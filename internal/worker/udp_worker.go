package worker

import (
	"context"
	"fmt"
	"log-generator/internal/generator"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	// PRD 명세: 각 워커 목표 = 10만 EPS
	TARGET_EPS_PER_WORKER = 100000
	
	// 배치 전송 최적화 설정
	BATCH_SIZE = 1000     // 1000개씩 배치 전송
	SEND_INTERVAL = 10    // 10ms마다 전송 (1000 logs/10ms = 100k/sec)
	
	// UDP 버퍼 크기 (PRD 명세 기반)
	UDP_SEND_BUFFER_SIZE = 2 * 1024 * 1024  // 2MB
	UDP_RECV_BUFFER_SIZE = 1 * 1024 * 1024  // 1MB
)

// WorkerMetrics - 워커별 성능 메트릭
type WorkerMetrics struct {
	WorkerID        int           `json:"worker_id"`
	Port            int           `json:"port"`
	CurrentEPS      int64         `json:"current_eps"`
	TotalSent       int64         `json:"total_sent"`
	ErrorCount      int64         `json:"error_count"`
	PacketLoss      float64       `json:"packet_loss"`
	LastSentTime    time.Time     `json:"last_sent_time"`
	CPUUsage        float64       `json:"cpu_usage"`
	GoroutineCount  int           `json:"goroutine_count"`
}

// UDPWorker - 고성능 UDP 로그 전송 워커 (10만 EPS 목표)
type UDPWorker struct {
	// 워커 식별 정보
	ID          int
	Port        int
	TargetHost  string
	
	// 네트워크 연결
	conn        *net.UDPConn
	remoteAddr  *net.UDPAddr
	
	// 로그 생성기
	generator   *generator.SystemLogGenerator
	
	// 성능 최적화 필드
	batchBuffer [][]byte
	sendBuffer  []byte
	
	// 상태 관리 (atomic 사용)
	isRunning   atomic.Bool
	currentEPS  atomic.Int64
	totalSent   atomic.Int64
	errorCount  atomic.Int64
	
	// 메트릭 및 모니터링
	metricsChannel chan WorkerMetrics
	lastMetricTime time.Time
	epsCounts      []int64  // 1초 간격 EPS 측정용
	epsIndex       int
	
	// 동기화
	stopChan    chan struct{}
	wg          sync.WaitGroup
	mutex       sync.RWMutex
	
	// CPU 최적화를 위한 고정밀 타이머
	ticker      *time.Ticker
}

// NewUDPWorker - 워커 생성 및 초기화
func NewUDPWorker(id, port int, targetHost string, metricsChannel chan WorkerMetrics) (*UDPWorker, error) {
	worker := &UDPWorker{
		ID:             id,
		Port:           port,
		TargetHost:     targetHost,
		generator:      generator.NewSystemLogGenerator(),
		batchBuffer:    make([][]byte, 0, BATCH_SIZE),
		sendBuffer:     make([]byte, 0, UDP_SEND_BUFFER_SIZE),
		metricsChannel: metricsChannel,
		stopChan:       make(chan struct{}),
		epsCounts:      make([]int64, 60), // 1분간 EPS 이력
		lastMetricTime: time.Now(),
	}
	
	// UDP 연결 설정
	err := worker.setupUDPConnection()
	if err != nil {
		return nil, fmt.Errorf("UDP 연결 설정 실패 (워커 %d): %v", id, err)
	}
	
	// 10ms마다 배치 전송하는 타이머 설정
	worker.ticker = time.NewTicker(time.Millisecond * SEND_INTERVAL)
	
	return worker, nil
}

func (w *UDPWorker) setupUDPConnection() error {
	// 로컬 주소 설정 (포트 바인딩)
	localAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", w.Port))
	if err != nil {
		return fmt.Errorf("로컬 주소 해결 실패: %v", err)
	}
	
	// 원격 주소 설정 (SIEM 시스템)
	w.remoteAddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", w.TargetHost, w.Port))
	if err != nil {
		return fmt.Errorf("원격 주소 해결 실패: %v", err)
	}
	
	// UDP 소켓 생성 및 최적화 설정
	w.conn, err = net.ListenUDP("udp", localAddr)
	if err != nil {
		return fmt.Errorf("UDP 소켓 생성 실패: %v", err)
	}
	
	// 소켓 버퍼 크기 최적화 (PRD 명세 기반)
	err = w.optimizeSocketBuffers()
	if err != nil {
		return fmt.Errorf("소켓 최적화 실패: %v", err)
	}
	
	return nil
}

func (w *UDPWorker) optimizeSocketBuffers() error {
	// SO_SNDBUF 설정 (송신 버퍼)
	rawConn, err := w.conn.SyscallConn()
	if err != nil {
		return err
	}
	
	var syscallErr error
	err = rawConn.Control(func(fd uintptr) {
		syscallErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, UDP_SEND_BUFFER_SIZE)
		if syscallErr != nil {
			return
		}
		syscallErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, UDP_RECV_BUFFER_SIZE)
	})
	
	if err != nil {
		return err
	}
	if syscallErr != nil {
		return syscallErr
	}
	
	return nil
}

// Start - 워커 시작 (10만 EPS 달성을 위한 최적화된 루프)
func (w *UDPWorker) Start(ctx context.Context) error {
	if !w.isRunning.CompareAndSwap(false, true) {
		return fmt.Errorf("워커 %d가 이미 실행 중입니다", w.ID)
	}
	
	defer w.cleanup()
	
	// CPU 코어에 고루틴 바인딩 (가능한 경우)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	
	// 메트릭 수집 고루틴 시작
	w.wg.Add(1)
	go w.metricsCollector(ctx)
	
	// 메인 전송 루프
	w.wg.Add(1)
	go w.sendLoop(ctx)
	
	// 종료 대기
	w.wg.Wait()
	return nil
}

// sendLoop - 핵심 전송 루프 (10만 EPS 달성)
func (w *UDPWorker) sendLoop(ctx context.Context) {
	defer w.wg.Done()
	
	batchCount := 0
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-w.ticker.C:
			// 배치가 가득 찰 때까지 로그 생성
			for len(w.batchBuffer) < BATCH_SIZE {
				logData := w.generator.GenerateSystemLog()
				w.batchBuffer = append(w.batchBuffer, logData)
			}
			
			// 배치 전송 실행
			err := w.sendBatch()
			if err != nil {
				w.errorCount.Add(1)
			} else {
				w.totalSent.Add(int64(len(w.batchBuffer)))
				batchCount++
			}
			
			// 배치 버퍼 초기화 (메모리 재사용)
			w.batchBuffer = w.batchBuffer[:0]
			
			// EPS 계산을 위한 카운팅 (100ms마다)
			if batchCount%10 == 0 {
				w.updateEPSMetrics()
			}
		}
	}
}

// sendBatch - 배치 전송 (시스템 콜 최소화)
func (w *UDPWorker) sendBatch() error {
	if len(w.batchBuffer) == 0 {
		return nil
	}
	
	// 여러 로그를 하나의 패킷으로 결합 (네트워크 효율성 향상)
	w.sendBuffer = w.sendBuffer[:0]
	
	for i, logData := range w.batchBuffer {
		w.sendBuffer = append(w.sendBuffer, logData...)
		if i < len(w.batchBuffer)-1 {
			w.sendBuffer = append(w.sendBuffer, '\n')
		}
	}
	
	// UDP 전송
	_, err := w.conn.WriteToUDP(w.sendBuffer, w.remoteAddr)
	return err
}

// sendBatchIndividual - 개별 로그 전송 (높은 정확도가 필요한 경우)
func (w *UDPWorker) sendBatchIndividual() error {
	var errors int
	
	for _, logData := range w.batchBuffer {
		_, err := w.conn.WriteToUDP(logData, w.remoteAddr)
		if err != nil {
			errors++
		}
	}
	
	if errors > 0 {
		return fmt.Errorf("%d개 로그 전송 실패", errors)
	}
	
	return nil
}

// updateEPSMetrics - EPS 메트릭 업데이트
func (w *UDPWorker) updateEPSMetrics() {
	now := time.Now()
	duration := now.Sub(w.lastMetricTime)
	
	if duration >= time.Second {
		// 현재 EPS 계산
		totalSent := w.totalSent.Load()
		if w.lastMetricTime.IsZero() {
			w.currentEPS.Store(0)
		} else {
			eps := int64(float64(totalSent) / duration.Seconds())
			w.currentEPS.Store(eps)
		}
		
		// EPS 이력 업데이트
		w.epsCounts[w.epsIndex] = w.currentEPS.Load()
		w.epsIndex = (w.epsIndex + 1) % len(w.epsCounts)
		
		w.lastMetricTime = now
	}
}

// metricsCollector - 메트릭 수집 및 전송
func (w *UDPWorker) metricsCollector(ctx context.Context) {
	defer w.wg.Done()
	
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-ticker.C:
			metrics := w.collectMetrics()
			select {
			case w.metricsChannel <- metrics:
			default:
				// 채널이 가득 찬 경우 블로킹 방지
			}
		}
	}
}

func (w *UDPWorker) collectMetrics() WorkerMetrics {
	// 패킷 손실률 계산 (간소화된 추정)
	totalSent := w.totalSent.Load()
	errorCount := w.errorCount.Load()
	var packetLoss float64
	if totalSent > 0 {
		packetLoss = float64(errorCount) / float64(totalSent) * 100
	}
	
	return WorkerMetrics{
		WorkerID:       w.ID,
		Port:          w.Port,
		CurrentEPS:    w.currentEPS.Load(),
		TotalSent:     totalSent,
		ErrorCount:    errorCount,
		PacketLoss:    packetLoss,
		LastSentTime:  time.Now(),
		CPUUsage:      w.getCPUUsage(),
		GoroutineCount: runtime.NumGoroutine(),
	}
}

func (w *UDPWorker) getCPUUsage() float64 {
	// 간소화된 CPU 사용률 측정 (실제로는 더 정교한 측정이 필요)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(runtime.NumCPU()) * 0.1 // 임시값
}

// Stop - 워커 정지
func (w *UDPWorker) Stop() error {
	if !w.isRunning.CompareAndSwap(true, false) {
		return fmt.Errorf("워커 %d가 실행되지 않고 있습니다", w.ID)
	}
	
	close(w.stopChan)
	w.wg.Wait()
	return nil
}

func (w *UDPWorker) cleanup() {
	if w.ticker != nil {
		w.ticker.Stop()
	}
	if w.conn != nil {
		w.conn.Close()
	}
	w.isRunning.Store(false)
}

// GetCurrentEPS - 현재 EPS 반환
func (w *UDPWorker) GetCurrentEPS() int64 {
	return w.currentEPS.Load()
}

// GetTotalSent - 총 전송 로그 수 반환
func (w *UDPWorker) GetTotalSent() int64 {
	return w.totalSent.Load()
}

// IsRunning - 실행 상태 확인
func (w *UDPWorker) IsRunning() bool {
	return w.isRunning.Load()
}

// SetCPUAffinity - CPU 친화성 설정 (Linux 전용)
func (w *UDPWorker) SetCPUAffinity(cpuID int) error {
	// 이 기능은 runtime/cgo와 syscall을 사용하여 구현 가능
	// 여기서는 인터페이스만 제공
	return nil
}

// GetAverageEPS - 최근 1분간 평균 EPS
func (w *UDPWorker) GetAverageEPS() int64 {
	var sum int64
	var count int64
	
	for _, eps := range w.epsCounts {
		if eps > 0 {
			sum += eps
			count++
		}
	}
	
	if count == 0 {
		return 0
	}
	
	return sum / count
}