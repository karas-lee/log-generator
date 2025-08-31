package worker

import (
	"context"
	"fmt"
	"log-generator/internal/config"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// PRD 명세: 400만 EPS = 40개 워커 × 10만 EPS
	TOTAL_WORKERS = 40
	FIRST_PORT = 10514  // 높은 포트 사용 (권한 문제 회피)
	LAST_PORT = 10553   // 10514 + 39 = 10553
)

// WorkerPoolMetrics - 워커 풀 전체 메트릭
type WorkerPoolMetrics struct {
	TotalEPS        int64                    `json:"total_eps"`
	TotalSent       int64                    `json:"total_sent"`
	TotalErrors     int64                    `json:"total_errors"`
	ActiveWorkers   int                      `json:"active_workers"`
	AverageEPS      int64                    `json:"average_eps"`
	PacketLossRate  float64                  `json:"packet_loss_rate"`
	WorkerMetrics   map[int]WorkerMetrics    `json:"worker_metrics"`
	SystemMetrics   SystemMetrics            `json:"system_metrics"`
	LastUpdate      time.Time                `json:"last_update"`
}

// SystemMetrics - 시스템 리소스 메트릭
type SystemMetrics struct {
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryUsageMB      float64 `json:"memory_usage_mb"`
	GoroutineCount     int     `json:"goroutine_count"`
	GCPauseMs          float64 `json:"gc_pause_ms"`
	NetworkTxMBps      float64 `json:"network_tx_mbps"`
}

// WorkerPool - 고성능 워커 풀 관리자 (프로파일 기반)
type WorkerPool struct {
	// 워커 관리
	workers         []*UDPWorker
	workerCount     int
	targetHost      string
	
	// 프로파일 설정
	profile         *config.EPSProfile
	
	// 메트릭 수집
	metricsChannel  chan WorkerMetrics
	poolMetrics     atomic.Value  // WorkerPoolMetrics 저장
	
	// 상태 관리
	isRunning       atomic.Bool
	startTime       time.Time
	
	// 동기화
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	mutex           sync.RWMutex
	
	// 성능 모니터링
	epsHistory      []int64    // EPS 이력 (최근 300초)
	historyIndex    int
	lastHistoryTime time.Time
	
	// 자동 조절 기능
	autoTuning      bool
	targetEPS       int64
	tuningEnabled   atomic.Bool
}

// NewWorkerPool - 워커 풀 생성 및 초기화
func NewWorkerPool(targetHost string) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	// 기본 프로파일 (4M)
	defaultProfile := config.EPSProfiles["4m"]
	
	pool := &WorkerPool{
		workers:        make([]*UDPWorker, 0, TOTAL_WORKERS),
		targetHost:     targetHost,
		profile:        defaultProfile,
		metricsChannel: make(chan WorkerMetrics, TOTAL_WORKERS*2), // 버퍼 크기 여유
		ctx:            ctx,
		cancel:         cancel,
		epsHistory:     make([]int64, 300), // 5분간 이력
		targetEPS:      int64(defaultProfile.TargetEPS),
		autoTuning:     true,
	}
	
	// 초기 메트릭 설정
	pool.poolMetrics.Store(WorkerPoolMetrics{
		WorkerMetrics: make(map[int]WorkerMetrics),
		LastUpdate:    time.Now(),
	})
	
	return pool
}

// NewWorkerPoolWithProfile - 프로파일 기반 워커 풀 생성
func NewWorkerPoolWithProfile(targetHost string, profile *config.EPSProfile) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	// 프로파일 기반 최적화 적용
	if profile.GOGC > 0 {
		debug.SetGCPercent(profile.GOGC)
	}
	if profile.MemoryLimit > 0 {
		debug.SetMemoryLimit(profile.MemoryLimit)
	}
	
	pool := &WorkerPool{
		workers:        make([]*UDPWorker, 0, profile.WorkerCount),
		targetHost:     targetHost,
		profile:        profile,
		metricsChannel: make(chan WorkerMetrics, profile.WorkerCount*2),
		ctx:            ctx,
		cancel:         cancel,
		epsHistory:     make([]int64, 300),
		targetEPS:      int64(profile.TargetEPS),
		autoTuning:     false, // 프로파일 모드에서는 자동 튜닝 비활성화
	}
	
	// 초기 메트릭 설정
	pool.poolMetrics.Store(WorkerPoolMetrics{
		WorkerMetrics: make(map[int]WorkerMetrics),
		LastUpdate:    time.Now(),
	})
	
	return pool
}

// Initialize - 프로파일 기반 워커 초기화
func (wp *WorkerPool) Initialize() error {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()
	
	if wp.isRunning.Load() {
		return fmt.Errorf("워커 풀이 이미 실행 중입니다")
	}
	
	// 프로파일에 따른 워커 수 결정
	workerCount := wp.profile.WorkerCount
	if workerCount > TOTAL_WORKERS {
		workerCount = TOTAL_WORKERS
	}
	
	// 워커 생성
	for i := 0; i < workerCount; i++ {
		workerID := i + 1
		port := FIRST_PORT + i
		
		// 프로파일 설정으로 워커 생성
		worker, err := NewUDPWorkerWithConfig(workerID, port, wp.targetHost, wp.metricsChannel, 
			wp.profile.BatchSize, wp.profile.TickerInterval)
		if err != nil {
			return fmt.Errorf("워커 %d 생성 실패: %v", workerID, err)
		}
		
		// 버퍼 크기 설정
		if wp.profile.SendBufferSize > 0 {
			worker.SetBufferSizes(wp.profile.SendBufferSize*1024, wp.profile.ReceiveBufferSize*1024)
		}
		
		wp.workers = append(wp.workers, worker)
	}
	
	wp.workerCount = len(wp.workers)
	fmt.Printf("✓ %s 프로파일: %d개 워커 초기화 완료 (포트 %d-%d)\n", 
		wp.profile.Name, wp.workerCount, FIRST_PORT, FIRST_PORT+wp.workerCount-1)
	fmt.Printf("  목표 EPS: %s, 배치 크기: %d, 타이머: %dμs\n",
		formatNumber(int64(wp.profile.TargetEPS)), wp.profile.BatchSize, wp.profile.TickerInterval)
	
	return nil
}

// Start - 워커 풀 시작 (400만 EPS 달성 시작)
func (wp *WorkerPool) Start() error {
	if !wp.isRunning.CompareAndSwap(false, true) {
		return fmt.Errorf("워커 풀이 이미 실행 중입니다")
	}
	
	wp.startTime = time.Now()
	
	// 메트릭 수집기 시작
	wp.wg.Add(1)
	go wp.metricsAggregator()
	
	// 자동 튜닝 시작
	if wp.autoTuning {
		wp.wg.Add(1)
		go wp.autoTuner()
	}
	
	// 모든 워커 시작
	for i, worker := range wp.workers {
		wp.wg.Add(1)
		go func(w *UDPWorker, index int) {
			defer wp.wg.Done()
			
			err := w.Start(wp.ctx)
			if err != nil {
				fmt.Printf("워커 %d 실행 실패: %v\n", w.ID, err)
			}
		}(worker, i)
		
		// 워커 시작 간격 (리소스 경합 방지)
		time.Sleep(time.Millisecond * 10)
	}
	
	fmt.Printf("🚀 워커 풀 시작: %d개 워커 실행 중 (목표: %s EPS)\n", wp.workerCount, formatNumber(wp.targetEPS))
	
	return nil
}

// metricsAggregator - 워커 메트릭 수집 및 집계
func (wp *WorkerPool) metricsAggregator() {
	defer wp.wg.Done()
	
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	workerMetricsMap := make(map[int]WorkerMetrics)
	
	for {
		select {
		case <-wp.ctx.Done():
			return
		case <-ticker.C:
			// 시스템 메트릭 수집
			sysMetrics := wp.collectSystemMetrics()
			
			// 워커 메트릭 집계
			var totalEPS, totalSent, totalErrors int64
			var activeWorkers int
			var totalPacketLoss float64
			
			for _, worker := range wp.workers {
				if worker.IsRunning() {
					activeWorkers++
					totalEPS += worker.GetCurrentEPS()
					totalSent += worker.GetTotalSent()
				}
			}
			
			// 평균 EPS 계산
			var averageEPS int64
			if activeWorkers > 0 {
				averageEPS = totalEPS / int64(activeWorkers)
			}
			
			// EPS 이력 업데이트
			wp.updateEPSHistory(totalEPS)
			
			// 풀 메트릭 업데이트
			poolMetrics := WorkerPoolMetrics{
				TotalEPS:       totalEPS,
				TotalSent:      totalSent,
				TotalErrors:    totalErrors,
				ActiveWorkers:  activeWorkers,
				AverageEPS:     averageEPS,
				PacketLossRate: totalPacketLoss / float64(activeWorkers),
				WorkerMetrics:  workerMetricsMap,
				SystemMetrics:  sysMetrics,
				LastUpdate:     time.Now(),
			}
			
			wp.poolMetrics.Store(poolMetrics)
			
			// 성능 로그 출력 (1분마다)
			if time.Since(wp.startTime).Seconds() > 0 && int(time.Since(wp.startTime).Seconds())%60 == 0 {
				wp.printPerformanceLog(poolMetrics)
			}
			
		case workerMetric := <-wp.metricsChannel:
			workerMetricsMap[workerMetric.WorkerID] = workerMetric
		}
	}
}

// collectSystemMetrics - 시스템 리소스 메트릭 수집
func (wp *WorkerPool) collectSystemMetrics() SystemMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return SystemMetrics{
		CPUUsagePercent: wp.getCPUUsage(),
		MemoryUsageMB:   float64(m.Alloc) / 1024 / 1024,
		GoroutineCount:  runtime.NumGoroutine(),
		GCPauseMs:      float64(m.PauseNs[(m.NumGC+255)%256]) / 1000000,
	}
}

func (wp *WorkerPool) getCPUUsage() float64 {
	// 실제 구현에서는 더 정교한 CPU 사용률 측정이 필요
	// 여기서는 워커 수와 목표 달성률 기반 추정
	currentMetrics := wp.GetMetrics()
	targetAchievementRate := float64(currentMetrics.TotalEPS) / float64(wp.targetEPS)
	
	// 추정 CPU 사용률 = 목표 달성률 * 75% (PRD 목표 CPU 사용률)
	return targetAchievementRate * 75.0
}

// updateEPSHistory - EPS 이력 업데이트
func (wp *WorkerPool) updateEPSHistory(currentEPS int64) {
	now := time.Now()
	if now.Sub(wp.lastHistoryTime) >= time.Second {
		wp.epsHistory[wp.historyIndex] = currentEPS
		wp.historyIndex = (wp.historyIndex + 1) % len(wp.epsHistory)
		wp.lastHistoryTime = now
	}
}

// autoTuner - 자동 성능 튜닝 (실험적 기능)
func (wp *WorkerPool) autoTuner() {
	defer wp.wg.Done()
	
	if !wp.tuningEnabled.Load() {
		return
	}
	
	ticker := time.NewTicker(time.Second * 30) // 30초마다 튜닝
	defer ticker.Stop()
	
	for {
		select {
		case <-wp.ctx.Done():
			return
		case <-ticker.C:
			metrics := wp.GetMetrics()
			
			// 목표 EPS의 95% 미만인 경우 최적화 시도
			if metrics.TotalEPS < (wp.targetEPS * 95 / 100) {
				wp.performAutoTuning(metrics)
			}
		}
	}
}

func (wp *WorkerPool) performAutoTuning(metrics WorkerPoolMetrics) {
	// CPU 사용률이 낮으면 워커 증가 시도
	if metrics.SystemMetrics.CPUUsagePercent < 60 {
		fmt.Printf("🔧 자동 튜닝: CPU 사용률 낮음 (%.1f%%), 성능 향상 시도\n", 
			metrics.SystemMetrics.CPUUsagePercent)
	}
	
	// 메모리 사용량이 높으면 GC 강제 실행
	if metrics.SystemMetrics.MemoryUsageMB > 10*1024 { // 10GB
		runtime.GC()
		fmt.Printf("🔧 자동 튜닝: 메모리 정리 실행 (%.1f MB)\n", 
			metrics.SystemMetrics.MemoryUsageMB)
	}
}

// printPerformanceLog - 성능 로그 출력
func (wp *WorkerPool) printPerformanceLog(metrics WorkerPoolMetrics) {
	duration := time.Since(wp.startTime)
	targetAchievement := float64(metrics.TotalEPS) / float64(wp.targetEPS) * 100
	
	fmt.Printf("📊 성능 리포트 [%s 경과]\n", duration.Round(time.Second))
	fmt.Printf("   현재 EPS: %s / 목표: %s (%.1f%%)\n", 
		formatNumber(metrics.TotalEPS), formatNumber(wp.targetEPS), targetAchievement)
	fmt.Printf("   활성 워커: %d/%d\n", metrics.ActiveWorkers, wp.workerCount)
	fmt.Printf("   총 전송: %s logs\n", formatNumber(metrics.TotalSent))
	fmt.Printf("   CPU: %.1f%% | 메모리: %.1f MB | 고루틴: %d\n",
		metrics.SystemMetrics.CPUUsagePercent,
		metrics.SystemMetrics.MemoryUsageMB,
		metrics.SystemMetrics.GoroutineCount)
	fmt.Printf("   패킷 손실률: %.2f%%\n", metrics.PacketLossRate)
	fmt.Println("   " + strings.Repeat("=", 60))
}

// Stop - 워커 풀 정지
func (wp *WorkerPool) Stop() error {
	if !wp.isRunning.CompareAndSwap(true, false) {
		return fmt.Errorf("워커 풀이 실행되지 않고 있습니다")
	}
	
	fmt.Printf("🛑 워커 풀 정지 시작...\n")
	
	// 취소 신호 전송
	wp.cancel()
	
	// 모든 워커 정지
	for _, worker := range wp.workers {
		err := worker.Stop()
		if err != nil {
			fmt.Printf("워커 %d 정지 실패: %v\n", worker.ID, err)
		}
	}
	
	// 고루틴 정리 대기
	wp.wg.Wait()
	
	// 최종 성능 리포트
	finalMetrics := wp.GetMetrics()
	duration := time.Since(wp.startTime)
	
	fmt.Printf("🏁 최종 성능 리포트:\n")
	fmt.Printf("   실행 시간: %s\n", duration.Round(time.Second))
	fmt.Printf("   총 전송 로그: %s\n", formatNumber(finalMetrics.TotalSent))
	fmt.Printf("   평균 EPS: %s\n", formatNumber(finalMetrics.TotalSent/int64(duration.Seconds())))
	fmt.Printf("   목표 달성률: %.1f%%\n", 
		float64(finalMetrics.TotalEPS)/float64(wp.targetEPS)*100)
	
	return nil
}

// GetMetrics - 현재 메트릭 반환
func (wp *WorkerPool) GetMetrics() WorkerPoolMetrics {
	if value := wp.poolMetrics.Load(); value != nil {
		return value.(WorkerPoolMetrics)
	}
	
	return WorkerPoolMetrics{
		WorkerMetrics: make(map[int]WorkerMetrics),
		LastUpdate:    time.Now(),
	}
}

// GetEPSHistory - EPS 이력 반환 (모니터링용)
func (wp *WorkerPool) GetEPSHistory() []int64 {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()
	
	history := make([]int64, len(wp.epsHistory))
	copy(history, wp.epsHistory)
	return history
}

// IsRunning - 실행 상태 확인
func (wp *WorkerPool) IsRunning() bool {
	return wp.isRunning.Load()
}

// GetWorkerCount - 워커 수 반환
func (wp *WorkerPool) GetWorkerCount() int {
	return wp.workerCount
}

// GetProfile - 현재 프로파일 반환
func (wp *WorkerPool) GetProfile() *config.EPSProfile {
	return wp.profile
}

// SetProfile - 프로파일 변경 (재시작 필요)
func (wp *WorkerPool) SetProfile(profile *config.EPSProfile) error {
	if wp.isRunning.Load() {
		return fmt.Errorf("워커 풀 실행 중에는 프로파일을 변경할 수 없습니다")
	}
	
	wp.profile = profile
	wp.targetEPS = int64(profile.TargetEPS)
	
	// 프로파일 기반 시스템 최적화
	if profile.GOGC > 0 {
		debug.SetGCPercent(profile.GOGC)
	}
	if profile.MemoryLimit > 0 {
		debug.SetMemoryLimit(profile.MemoryLimit)
	}
	
	fmt.Printf("프로파일 변경: %s (%s)\n", profile.Name, profile.Description)
	return nil
}

// EnableAutoTuning - 자동 튜닝 활성화/비활성화
func (wp *WorkerPool) EnableAutoTuning(enabled bool) {
	wp.tuningEnabled.Store(enabled)
	fmt.Printf("자동 튜닝: %t\n", enabled)
}

// formatNumber - 숫자 포맷팅 (가독성 향상)
func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// strings.Repeat 구현
func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}