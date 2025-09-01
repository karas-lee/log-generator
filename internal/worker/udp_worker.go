package worker

import (
	"context"
	"fmt"
	"log-generator/internal/generator"
	"math"
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

// UDPWorker - 고성능 UDP 로그 전송 워커 (프로파일 기반 EPS)
type UDPWorker struct {
	// 워커 식별 정보
	ID          int
	Port        int
	TargetHost  string
	
	// 성능 설정
	batchSize      int
	tickerInterval int  // microseconds
	sendBufferSize int
	recvBufferSize int
	
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
	lastTotalSent  int64    // EPS 계산을 위한 이전 totalSent 값
	epsCounts      []int64  // 1초 간격 EPS 측정용
	epsIndex       int
	
	// 동기화
	stopChan    chan struct{}
	wg          sync.WaitGroup
	mutex       sync.RWMutex
	
	// CPU 최적화를 위한 고정밀 타이머
	ticker      *time.Ticker
	
	// PID Controller for Precise EPS Control
	targetEPS      int64           // 목표 EPS
	adaptiveControl bool           // adaptive control 활성화
	pidKp          float64         // Proportional gain
	pidKi          float64         // Integral gain  
	pidKd          float64         // Derivative gain
	pidIntegral    float64         // Integral term accumulator
	pidLastError   float64         // Previous error for derivative
	controlInterval time.Duration  // Control loop interval
	lastControlTime time.Time      // Last control adjustment time
	batchInterval   time.Duration  // Current batch send interval
	precisionMode   string         // "high", "medium", "performance"
}

// NewUDPWorker - 워커 생성 및 초기화 (기본 설정)
func NewUDPWorker(id, port int, targetHost string, metricsChannel chan WorkerMetrics) (*UDPWorker, error) {
	return NewUDPWorkerWithConfig(id, port, targetHost, metricsChannel, BATCH_SIZE, SEND_INTERVAL*1000)
}

// NewUDPWorkerWithConfig - 커스텀 설정으로 워커 생성
func NewUDPWorkerWithConfig(id, port int, targetHost string, metricsChannel chan WorkerMetrics, 
	batchSize int, tickerInterval int) (*UDPWorker, error) {
	
	worker := &UDPWorker{
		ID:             id,
		Port:           port,
		TargetHost:     targetHost,
		batchSize:      batchSize,
		tickerInterval: tickerInterval,
		sendBufferSize: UDP_SEND_BUFFER_SIZE,
		recvBufferSize: UDP_RECV_BUFFER_SIZE,
		generator:      generator.NewSystemLogGenerator(),
		batchBuffer:    make([][]byte, 0, batchSize),
		sendBuffer:     make([]byte, 0, UDP_SEND_BUFFER_SIZE),
		metricsChannel: metricsChannel,
		stopChan:       make(chan struct{}),
		epsCounts:      make([]int64, 60), // 1분간 EPS 이력
		lastMetricTime: time.Now(),
		lastTotalSent:  0,
	}
	
	// UDP 연결 설정
	err := worker.setupUDPConnection()
	if err != nil {
		return nil, fmt.Errorf("UDP 연결 설정 실패 (워커 %d): %v", id, err)
	}
	
	// 프로파일 기반 타이머 설정
	if tickerInterval < 1000 {
		// 마이크로초 단위 타이머
		worker.ticker = time.NewTicker(time.Duration(tickerInterval) * time.Microsecond)
	} else {
		// 밀리초 단위로 변환
		worker.ticker = time.NewTicker(time.Duration(tickerInterval/1000) * time.Millisecond)
	}
	
	return worker, nil
}

func (w *UDPWorker) setupUDPConnection() error {
	// 원격 주소 설정 (SIEM 시스템) - 포트 514는 표준 syslog 포트
	remotePort := 514
	var err error
	w.remoteAddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", w.TargetHost, remotePort))
	if err != nil {
		return fmt.Errorf("원격 주소 해결 실패: %v", err)
	}
	
	// UDP 클라이언트 소켓 생성 (바인딩하지 않음 - 송신 전용)
	// 로컬 포트는 OS가 자동 할당
	w.conn, err = net.DialUDP("udp", nil, w.remoteAddr)
	if err != nil {
		return fmt.Errorf("UDP 연결 생성 실패: %v", err)
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
		syscallErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, w.sendBufferSize)
		if syscallErr != nil {
			return
		}
		syscallErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, w.recvBufferSize)
	})
	
	if err != nil {
		return err
	}
	if syscallErr != nil {
		return syscallErr
	}
	
	return nil
}

// SetBufferSizes - 버퍼 크기 설정
func (w *UDPWorker) SetBufferSizes(sendSize, recvSize int) {
	w.sendBufferSize = sendSize
	w.recvBufferSize = recvSize
}

// Start - 워커 시작 (프로파일 기반 EPS 달성)
func (w *UDPWorker) Start(ctx context.Context) error {
	if !w.isRunning.CompareAndSwap(false, true) {
		return fmt.Errorf("워커 %d가 이미 실행 중입니다", w.ID)
	}
	
	// 메트릭 수집 고루틴 시작
	w.wg.Add(1)
	go w.metricsCollector(ctx)
	
	// 메인 전송 루프
	w.wg.Add(1)
	go w.sendLoop(ctx)
	
	// 워커가 성공적으로 시작됨
	return nil
}

// sendLoop - 핵심 전송 루프 (정밀 레이트 제어)
func (w *UDPWorker) sendLoop(ctx context.Context) {
	defer w.wg.Done()
	
	// generator 초기화 확인
	if w.generator == nil {
		return
	}
	
	// 목표 EPS가 있으면 정밀도 모드에 따라 선택
	if w.targetEPS > 0 && w.adaptiveControl {
		switch w.precisionMode {
		case "ultra":
			w.sendLoopUltra(ctx)         // 초고성능 100% 달성 모드
		case "realtime":
			w.sendLoopRealtime(ctx)      // 실시간 스케줄링 모드
		case "performance":
			w.sendLoopPerformance(ctx)  // 성능 우선 모드
		case "high":
			w.sendLoopPrecise(ctx)       // 높은 정밀도 모드
		default:
			w.sendLoopMedium(ctx)        // 중간 정밀도 모드
		}
		return
	}
	
	// 기본 모드 (ticker 사용)
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-w.ticker.C:
			// 배치 버퍼 초기화 (중요: 배치 생성 전에 먼저 초기화)
			w.batchBuffer = w.batchBuffer[:0]
			
			// 프로파일 기반 배치 크기까지 로그 생성
			for i := 0; i < w.batchSize; i++ {
				logData := w.generator.GenerateSystemLog()
				w.batchBuffer = append(w.batchBuffer, logData)
			}
			
			// 배치 전송
			err := w.sendBatch()
			if err != nil {
				w.errorCount.Add(1)
			} else {
				w.totalSent.Add(int64(w.batchSize))
			}
			
			// 주기적으로 EPS 업데이트
			w.updateEPSMetrics()
		}
	}
}

// sendLoopPrecise - 오차 없는 정밀한 EPS 제어를 위한 전송 루프
func (w *UDPWorker) sendLoopPrecise_OLD(ctx context.Context) {
	// 프로파일 배치 크기 사용
	logsPerBatch := int64(w.batchSize)
	fmt.Printf("Worker %d: Initial w.batchSize=%d\n", w.ID, w.batchSize)
	if logsPerBatch == 0 {
		// 목표 EPS
		targetEPS := w.targetEPS
		if targetEPS == 0 {
			targetEPS = 25000
		}
		// 더 작은 배치로 더 자주 전송 (정밀도 향상)
		batchesPerSecond := int64(100)
		logsPerBatch = targetEPS / batchesPerSecond
		if logsPerBatch < 1 {
			logsPerBatch = 1
		}
		fmt.Printf("Worker %d: Calculated logsPerBatch=%d from targetEPS=%d\n", w.ID, logsPerBatch, targetEPS)
	}
	
	// 타이머 간격 (프로파일 기반)
	intervalMs := w.tickerInterval / 1000
	if intervalMs < 1 {
		intervalMs = 10 // 기본 10ms
	}
	
	// 디버그 로깅
	fmt.Printf("Worker %d: targetEPS=%d, logsPerBatch=%d, intervalMs=%d, tickerInterval=%d\n", 
		w.ID, w.targetEPS, logsPerBatch, intervalMs, w.tickerInterval)
	
	// 나노초 단위 정밀 타이밍
	intervalNanos := int64(intervalMs) * 1000000
	
	// 예상 배치/초 계산 및 로깅
	expectedBatchesPerSec := 1000.0 / float64(intervalMs)
	expectedEPS := float64(logsPerBatch) * expectedBatchesPerSec
	fmt.Printf("Worker %d: Expected %d logs every %dms = %.0f batches/sec = %.0f EPS\n",
		w.ID, logsPerBatch, intervalMs, expectedBatchesPerSec, expectedEPS)
	
	// 실제 배치 전송 횟수 추적
	batchSentCount := int64(0)
	trackingStartTime := time.Now()
	
	// 시작 시간과 누적 카운터
	startTime := time.Now()
	totalSentInWindow := int64(0)
	windowStartTime := startTime
	
	// 피드백 제어를 위한 변수
	adjustmentFactor := float64(1.0)
	loopCount := 0
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		default:
			loopStart := time.Now()
			loopCount++
			
			// 실제 배치 크기 (피드백 조정 적용)
			actualBatchSize := int(float64(logsPerBatch) * adjustmentFactor)
			if actualBatchSize < 1 {
				actualBatchSize = 1
			}
			
			// 배치 버퍼 초기화 (중요: 배치 생성 전에 먼저 초기화)
			w.batchBuffer = w.batchBuffer[:0]
			
			// 배치 생성
			for i := 0; i < actualBatchSize; i++ {
				logData := w.generator.GenerateSystemLog()
				w.batchBuffer = append(w.batchBuffer, logData)
			}
			
			// 전송
			err := w.sendBatch()
			if err != nil {
				w.errorCount.Add(1)
			} else {
				sent := int64(actualBatchSize)
				w.totalSent.Add(sent)
				totalSentInWindow += sent
				batchSentCount++
				
				// 1초마다 실제 배치 전송률 출력
				if time.Since(trackingStartTime) >= time.Second && w.ID == 1 {
					actualBatchesPerSec := float64(batchSentCount)
					actualEPS := actualBatchesPerSec * float64(logsPerBatch)
					fmt.Printf("Worker 1: Actual batch rate: %.0f batches/sec = %.0f EPS (expected %.0f EPS)\n",
						actualBatchesPerSec, actualEPS, expectedEPS)
					batchSentCount = 0
					trackingStartTime = time.Now()
				}
			}
			
			// 50ms마다 빠른 피드백 제어 (정밀 모드)
			elapsed := time.Since(windowStartTime)
			if elapsed >= 50*time.Millisecond {
				// 실제 EPS 계산
				actualEPS := float64(totalSentInWindow) * 20 // 50ms를 1초로 환산
				targetEPSFloat := float64(w.targetEPS)
				
				// 오차율 계산
				errorPercent := (actualEPS - targetEPSFloat) / targetEPSFloat
				
				// 0.5% 이상 오차시 즐각 조정 (높은 정밀도)
				if math.Abs(errorPercent) > 0.005 {
					// 비례 제어 강화
					pAdjustment := -errorPercent * 0.8
					
					// 조정 범위 제한
					if pAdjustment > 0.02 {
						pAdjustment = 0.02
					} else if pAdjustment < -0.02 {
						pAdjustment = -0.02
					}
					
					adjustmentFactor *= (1.0 + pAdjustment)
					
					// 계수 범위 제한
					if adjustmentFactor < 0.95 {
						adjustmentFactor = 0.95
					} else if adjustmentFactor > 1.05 {
						adjustmentFactor = 1.05
					}
				}
				
				// 윈도우 리셋
				totalSentInWindow = 0
				windowStartTime = time.Now()
			}
			
			// 다음 전송까지 정밀 대기
			processingTime := time.Since(loopStart)
			sleepTime := time.Duration(intervalNanos) - processingTime
			
			if sleepTime > 0 {
				// 정밀한 sleep (busy-wait 혼합)
				if sleepTime > time.Millisecond {
					time.Sleep(sleepTime - time.Microsecond*100)
				}
				// 마지막 100us는 busy-wait로 정밀 제어
				targetTime := loopStart.Add(time.Duration(intervalNanos))
				for time.Now().Before(targetTime) {
					// busy-wait
				}
			}
			
			// 주기적으로 EPS 업데이트
			w.updateEPSMetrics()
		}
	}
}

// sendLoopPrecise - Simple ticker-based precise loop
func (w *UDPWorker) sendLoopPrecise(ctx context.Context) {
	// Calculate logs per batch
	logsPerBatch := int64(w.batchSize)
	if logsPerBatch == 0 {
		targetEPS := w.targetEPS
		if targetEPS == 0 {
			targetEPS = 25000
		}
		// 100 batches per second for precision
		logsPerBatch = targetEPS / 100
		if logsPerBatch < 1 {
			logsPerBatch = 1
		}
	}
	
	// Use ticker with interval from profile
	intervalMs := w.tickerInterval / 1000
	if intervalMs < 1 {
		intervalMs = 5 // minimum 5ms
	}
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()
	
	batchesPerSec := 1000 / intervalMs
	expectedEPS := int64(batchesPerSec) * logsPerBatch
	fmt.Printf("Worker %d: Ticker mode - %d logs every %dms = %d EPS\n", 
		w.ID, logsPerBatch, intervalMs, expectedEPS)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-ticker.C:
			// Clear and create batch
			w.batchBuffer = w.batchBuffer[:0]
			for i := int64(0); i < logsPerBatch; i++ {
				log := w.generator.GenerateSystemLog()
				w.batchBuffer = append(w.batchBuffer, log)
			}
			
			// Send batch
			if err := w.sendBatch(); err == nil {
				w.totalSent.Add(logsPerBatch)
			} else {
				w.errorCount.Add(1)
			}
			
			// Update EPS metrics periodically
			w.updateEPSMetrics()
		}
	}
}

// sendLoopMedium - 중간 정밀도 모드 (오차 <5%, 균형잡힌 성능)
func (w *UDPWorker) sendLoopMedium(ctx context.Context) {
	// 워커당 목표 EPS
	targetEPS := w.targetEPS
	if targetEPS == 0 {
		targetEPS = 25000 // 기본값
	}
	
	// 전송 빈도 계산 (10ms 간격)
	sendFrequency := int64(100) // 초당 100회
	logsPerBatch := targetEPS / sendFrequency
	if logsPerBatch < 1 {
		logsPerBatch = 1
	}
	
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	
	windowStartTime := time.Now()
	totalSentInWindow := int64(0)
	adjustmentFactor := 1.0
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 배치 크기 계산 (조정 계수 적용)
			currentBatchSize := int(float64(logsPerBatch) * adjustmentFactor)
			if currentBatchSize <= 0 {
				currentBatchSize = 1
			}
			
			// 배치 생성 및 전송
			w.batchBuffer = w.batchBuffer[:0]
			for i := 0; i < currentBatchSize; i++ {
				log := w.generator.GenerateSystemLog()
				w.batchBuffer = append(w.batchBuffer, log)
			}
			
			if err := w.sendBatch(); err == nil {
				sent := int64(currentBatchSize)
				w.totalSent.Add(sent)
				totalSentInWindow += sent
			}
			
			// 200ms마다 피드백 조정
			elapsed := time.Since(windowStartTime)
			if elapsed >= 200*time.Millisecond {
				actualEPS := float64(totalSentInWindow) * 5 // 200ms를 1초로 환산
				targetEPSFloat := float64(targetEPS)
				errorPercent := (actualEPS - targetEPSFloat) / targetEPSFloat
				
				// 2% 이상 오차시 조정
				if math.Abs(errorPercent) > 0.02 {
					adjustment := -errorPercent * 0.4
					if adjustment > 0.08 {
						adjustment = 0.08
					} else if adjustment < -0.08 {
						adjustment = -0.08
					}
					adjustmentFactor *= (1.0 + adjustment)
					if adjustmentFactor < 0.7 {
						adjustmentFactor = 0.7
					} else if adjustmentFactor > 1.3 {
						adjustmentFactor = 1.3
					}
				}
				
				// 윈도우 리셋
				windowStartTime = time.Now()
				totalSentInWindow = 0
			}
			
			w.updateEPSMetrics()
		}
	}
}

// sendLoopUltraPerformance - 초고성능 모드 (100% 목표 달성을 위한 정밀 제어)
func (w *UDPWorker) sendLoopUltraPerformance(ctx context.Context) {
	// 목표 EPS와 배치 설정
	targetEPS := w.targetEPS
	if targetEPS == 0 {
		targetEPS = 20000
	}
	
	// 프로파일 기반 배치 크기 (보정값 적용)
	baseLogsPerBatch := int64(w.batchSize)
	if baseLogsPerBatch == 0 {
		baseLogsPerBatch = 200
	}
	
	// 배치 크기 그대로 사용
	logsPerBatch := baseLogsPerBatch
	
	// 나노초 단위 간격 (더 정밀한 제어)
	intervalNanos := int64(w.tickerInterval) * 1000 // 마이크로초 -> 나노초
	if intervalNanos == 0 {
		intervalNanos = 10000000 // 10ms
	}
	
	// 버퍼 사전 할당 (GC 압력 감소)
	buffer1 := make([][]byte, 0, logsPerBatch)
	buffer2 := make([][]byte, 0, logsPerBatch)
	currentBuffer := &buffer1
	nextBuffer := &buffer2
	
	// 시작 시간과 다음 전송 시간
	startTime := time.Now()
	nextSendTime := startTime
	batchNumber := int64(0)
	
	// 적응형 보정
	windowStart := startTime
	windowSent := int64(0)
	adjustmentFactor := 1.0
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		default:
			// 배치 준비 (다음 버퍼에 미리 생성)
			*nextBuffer = (*nextBuffer)[:0]
			actualBatchSize := int64(float64(logsPerBatch) * adjustmentFactor)
			for i := int64(0); i < actualBatchSize; i++ {
				log := w.generator.GenerateSystemLog()
				*nextBuffer = append(*nextBuffer, log)
			}
			
			// 정확한 시간까지 대기
			now := time.Now()
			if now.Before(nextSendTime) {
				sleepDuration := nextSendTime.Sub(now)
				
				// 2ms 이상은 Sleep
				if sleepDuration > 2*time.Millisecond {
					time.Sleep(sleepDuration - time.Millisecond)
				}
				
				// 나머지는 정밀 스핀 대기
				for time.Now().Before(nextSendTime) {
					runtime.Gosched() // 다른 고루틴에게 양보
				}
			}
			
			// 버퍼 스왑 및 전송
			currentBuffer, nextBuffer = nextBuffer, currentBuffer
			w.batchBuffer = *currentBuffer
			
			if err := w.sendBatch(); err == nil {
				sent := int64(len(w.batchBuffer))
				w.totalSent.Add(sent)
				windowSent += sent
			} else {
				w.errorCount.Add(1)
			}
			
			// 다음 전송 시간 계산 (드리프트 방지)
			batchNumber++
			nextSendTime = startTime.Add(time.Duration(batchNumber * intervalNanos))
			
			// 250ms마다 적응형 조정
			if time.Since(windowStart) >= 250*time.Millisecond {
				actualEPS := float64(windowSent) * 4 // 250ms를 1초로 환산
				targetEPSFloat := float64(targetEPS)
				errorRate := (actualEPS - targetEPSFloat) / targetEPSFloat
				
				// 오차가 1% 이상이면 조정
				if math.Abs(errorRate) > 0.01 {
					// PD 제어 (비례 + 미분)
					adjustment := -errorRate * 0.3 // P 게인
					
					// 조정 범위 제한
					if adjustment > 0.05 {
						adjustment = 0.05
					} else if adjustment < -0.05 {
						adjustment = -0.05
					}
					
					adjustmentFactor *= (1.0 + adjustment)
					
					// 팩터 범위 제한
					if adjustmentFactor > 1.2 {
						adjustmentFactor = 1.2
					} else if adjustmentFactor < 0.8 {
						adjustmentFactor = 0.8
					}
				}
				
				windowStart = time.Now()
				windowSent = 0
			}
			
			// 메트릭 업데이트
			if batchNumber%50 == 0 {
				w.updateEPSMetrics()
			}
		}
	}
}

// sendLoopPerformance - 성능 우선 모드 (오차 <10%, 최대 처리량)
func (w *UDPWorker) sendLoopPerformance(ctx context.Context) {
	// 워커당 목표 EPS에 따라 전송 간격과 배치 크기 계산
	targetEPS := w.targetEPS
	if targetEPS == 0 {
		targetEPS = 25000 // 기본값 (4M / 160)
	}
	
	// 프로파일에서 설정한 ticker 간격 사용
	intervalMs := int64(w.tickerInterval / 1000)
	if intervalMs < 1 {
		intervalMs = 10 // 기본 10ms
	}
	
	// 배치당 로그 수 계산: targetEPS * (intervalMs / 1000) * 1.064 (보정 계수)
	// 94% -> 100% 달성을 위한 정확한 보정
	logsPerBatch := int64(float64(targetEPS * intervalMs) / 1000.0 * 1.064)
	if logsPerBatch < 1 {
		logsPerBatch = 1
	}
	
	// 타이머 생성
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()
	
	// 버스트 전송을 위한 큰 버퍼 사전 할당
	maxBatchSize := int(logsPerBatch * 2)
	preallocBuffer := make([][]byte, 0, maxBatchSize)
	
	windowStartTime := time.Now()
	totalSentInWindow := int64(0)
	// 초기 부스트: 94% -> 100% 달성을 위해 6.4% 부스트 적용 (100/94 = 1.064)
	adjustmentFactor := 1.064
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 조정된 배치 크기
			batchSize := int(float64(logsPerBatch) * adjustmentFactor)
			if batchSize < 1 {
				batchSize = 1
			}
			preallocBuffer = preallocBuffer[:0]
			
			for i := 0; i < batchSize; i++ {
				log := w.generator.GenerateSystemLog()
				preallocBuffer = append(preallocBuffer, log)
			}
			
			// 배치 전송
			w.batchBuffer = preallocBuffer
			if err := w.sendBatch(); err == nil {
				sent := int64(batchSize)
				w.totalSent.Add(sent)
				totalSentInWindow += sent
			}
			
			// 100ms마다 체크 (빠른 피드백)
			elapsed := time.Since(windowStartTime)
			if elapsed >= 100*time.Millisecond {
				actualEPS := float64(totalSentInWindow) * 10 // 100ms를 1초로 환산
				targetEPSFloat := float64(targetEPS)
				errorPercent := (actualEPS - targetEPSFloat) / targetEPSFloat
				
				// PID 제어 방식으로 조정
				if math.Abs(errorPercent) > 0.01 { // 1% 이상 오차시 조정
					// P (비례) 제어: 오차에 비례하여 조정
					adjustment := -errorPercent * 0.5
					
					// 조정 범위 제한 (급격한 변화 방지)
					if adjustment > 0.1 {
						adjustment = 0.1
					} else if adjustment < -0.1 {
						adjustment = -0.1
					}
					
					adjustmentFactor *= (1.0 + adjustment)
					
					// 조정 계수 범위 제한
					if adjustmentFactor > 2.0 {
						adjustmentFactor = 2.0
					} else if adjustmentFactor < 0.5 {
						adjustmentFactor = 0.5
					}
				}
				
				// 윈도우 리셋
				windowStartTime = time.Now()
				totalSentInWindow = 0
			}
			
			// 메트릭 업데이트 (낮은 빈도)
			if time.Since(w.lastMetricTime) >= 100*time.Millisecond {
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
	
	// conn 상태 확인
	if w.conn == nil {
		return fmt.Errorf("worker %d: UDP connection is nil", w.ID)
	}
	
	// 여러 로그를 하나의 패킷으로 결합 (네트워크 효율성 향상)
	w.sendBuffer = w.sendBuffer[:0]
	
	for i, logData := range w.batchBuffer {
		w.sendBuffer = append(w.sendBuffer, logData...)
		if i < len(w.batchBuffer)-1 {
			w.sendBuffer = append(w.sendBuffer, '\n')
		}
	}
	
	// UDP 전송 (DialUDP 사용 시 Write 메서드 사용)
	_, err := w.conn.Write(w.sendBuffer)
	if err != nil && w.ID == 1 {
		fmt.Printf("Worker 1: Send error: %v\n", err)
	}
	return err
}

// sendBatchPartial - 부분 배치 전송 (정밀한 속도 제어)
func (w *UDPWorker) sendBatchPartial(batch [][]byte) error {
	if len(batch) == 0 {
		return nil
	}
	
	// 여러 로그를 하나의 패킷으로 결합
	w.sendBuffer = w.sendBuffer[:0]
	
	for i, logData := range batch {
		w.sendBuffer = append(w.sendBuffer, logData...)
		if i < len(batch)-1 {
			w.sendBuffer = append(w.sendBuffer, '\n')
		}
	}
	
	// UDP 전송 (DialUDP 사용 시 Write 메서드 사용)
	_, err := w.conn.Write(w.sendBuffer)
	return err
}

// sendBatchIndividual - 개별 로그 전송 (높은 정확도가 필요한 경우)
func (w *UDPWorker) sendBatchIndividual() error {
	var errors int
	
	for _, logData := range w.batchBuffer {
		_, err := w.conn.Write(logData)
		if err != nil {
			errors++
		}
	}
	
	if errors > 0 {
		return fmt.Errorf("%d개 로그 전송 실패", errors)
	}
	
	return nil
}

// updateEPSMetrics - EPS 메트릭 업데이트 (평활화 적용)
func (w *UDPWorker) updateEPSMetrics() {
	now := time.Now()
	duration := now.Sub(w.lastMetricTime)
	
	// 100ms마다 업데이트 (더 자주 측정)
	if duration >= time.Millisecond*100 {
		totalSent := w.totalSent.Load()
		
		if !w.lastMetricTime.IsZero() && w.lastTotalSent > 0 {
			// 순간 EPS 계산
			sentSinceLastUpdate := totalSent - w.lastTotalSent
			instantEPS := float64(sentSinceLastUpdate) / duration.Seconds()
			
			// 이동평균 필터 적용 (더 안정적인 값)
			smoothingFactor := 0.3 // 30% 새 값, 70% 기존 값
			currentEPS := w.currentEPS.Load()
			
			var smoothedEPS float64
			if currentEPS == 0 {
				smoothedEPS = instantEPS
			} else {
				smoothedEPS = float64(currentEPS)*(1-smoothingFactor) + instantEPS*smoothingFactor
			}
			
			w.currentEPS.Store(int64(smoothedEPS))
			
			// EPS 이력 업데이트
			w.epsCounts[w.epsIndex] = int64(smoothedEPS)
			w.epsIndex = (w.epsIndex + 1) % len(w.epsCounts)
		} else {
			w.currentEPS.Store(0)
		}
		
		w.lastTotalSent = totalSent
		w.lastMetricTime = now
	}
}

// GetMetrics - 워커 메트릭스 반환
func (w *UDPWorker) GetMetrics() WorkerMetrics {
	return w.collectMetrics()
}

// SetTargetEPS - 목표 EPS 설정 및 PID 컨트롤러 초기화
func (w *UDPWorker) SetTargetEPS(targetEPS int64) {
	w.targetEPS = targetEPS
	w.adaptiveControl = targetEPS > 0
	
	if targetEPS > 0 {
		// PID 파라미터 설정 (프로파일별 최적화)
		w.adjustPIDForProfile(targetEPS)
		
		// 초기 배치 전송 간격 계산
		batchesPerSecond := float64(targetEPS) / float64(w.batchSize)
		if batchesPerSecond > 0 {
			w.batchInterval = time.Duration(float64(time.Second) / batchesPerSecond)
		} else {
			w.batchInterval = time.Millisecond
		}
		
		// 제어 간격 설정 (100ms마다 조정)
		w.controlInterval = 100 * time.Millisecond
		w.lastControlTime = time.Now()
		
		// PID 상태 초기화
		w.pidIntegral = 0
		w.pidLastError = 0
	}
}

// SetPrecisionMode - 정밀도 모드 설정
func (w *UDPWorker) SetPrecisionMode(mode string) {
	w.precisionMode = mode
}

// adjustPIDForProfile - 목표 EPS에 따른 PID 파라미터 최적화
func (w *UDPWorker) adjustPIDForProfile(targetEPS int64) {
	if targetEPS <= 20_000 { // 워커당 20K (100K 프로파일)
		w.pidKp = 0.00005  // 매우 민감한 제어
		w.pidKi = 0.00001
		w.pidKd = 0.000005
	} else if targetEPS <= 50_000 { // 워커당 50K (500K, 1M 프로파일)
		w.pidKp = 0.00003
		w.pidKi = 0.000008
		w.pidKd = 0.000003
	} else if targetEPS <= 70_000 { // 워커당 67K (2M 프로파일)
		w.pidKp = 0.00002
		w.pidKi = 0.000005
		w.pidKd = 0.000002
	} else { // 워커당 100K (4M 프로파일)
		w.pidKp = 0.00001
		w.pidKi = 0.000003
		w.pidKd = 0.000001
	}
}

// runPIDControl - PID 제어 루프 실행
func (w *UDPWorker) runPIDControl() {
	if !w.adaptiveControl || w.targetEPS == 0 {
		return
	}
	
	now := time.Now()
	if now.Sub(w.lastControlTime) < w.controlInterval {
		return
	}
	
	// 현재 EPS와 목표 EPS 차이 계산
	currentEPS := w.currentEPS.Load()
	error := float64(w.targetEPS - currentEPS)
	
	// P (Proportional) 항
	P := w.pidKp * error
	
	// I (Integral) 항 - Anti-windup 적용
	w.pidIntegral += error
	maxIntegral := float64(w.targetEPS) * 10
	if w.pidIntegral > maxIntegral {
		w.pidIntegral = maxIntegral
	} else if w.pidIntegral < -maxIntegral {
		w.pidIntegral = -maxIntegral
	}
	I := w.pidKi * w.pidIntegral
	
	// D (Derivative) 항
	derivative := error - w.pidLastError
	D := w.pidKd * derivative
	w.pidLastError = error
	
	// PID 출력 계산
	pidOutput := P + I + D
	
	// 배치 간격 조정 (음수는 더 빠르게, 양수는 더 느리게)
	adjustment := 1.0 - pidOutput
	if adjustment < 0.5 {
		adjustment = 0.5  // 최대 2배 속도
	} else if adjustment > 2.0 {
		adjustment = 2.0  // 최소 0.5배 속도
	}
	
	w.batchInterval = time.Duration(float64(w.batchInterval) * adjustment)
	
	// 최소/최대 간격 제한
	minInterval := 10 * time.Microsecond
	maxInterval := 100 * time.Millisecond
	if w.batchInterval < minInterval {
		w.batchInterval = minInterval
	} else if w.batchInterval > maxInterval {
		w.batchInterval = maxInterval
	}
	
	w.lastControlTime = now
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
	
	// 종료 신호 전송
	close(w.stopChan)
	
	// 고루틴 종료 대기
	w.wg.Wait()
	
	// 리소스 정리
	w.cleanup()
	
	return nil
}

func (w *UDPWorker) cleanup() {
	if w.ticker != nil {
		w.ticker.Stop()
	}
	if w.conn != nil {
		w.conn.Close()
	}
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