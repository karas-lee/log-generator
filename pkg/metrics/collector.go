package metrics

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// PerformanceMetrics - 400만 EPS 성능 메트릭
type PerformanceMetrics struct {
	// 핵심 성능 지표
	CurrentEPS          int64     `json:"current_eps"`
	TotalSent           int64     `json:"total_sent"`
	TotalErrors         int64     `json:"total_errors"`
	PacketLoss          float64   `json:"packet_loss"`
	
	// 시스템 리소스
	CPUUsagePercent     float64   `json:"cpu_usage_percent"`
	MemoryUsageMB       float64   `json:"memory_usage_mb"`
	NetworkTxMBps       float64   `json:"network_tx_mbps"`
	
	// 워커 상태
	ActiveWorkers       int       `json:"active_workers"`
	TotalWorkers        int       `json:"total_workers"`
	
	// 시간 정보
	Timestamp           time.Time `json:"timestamp"`
	UptimeSeconds       int64     `json:"uptime_seconds"`
	
	// 목표 대비 성과
	TargetEPS           int64     `json:"target_eps"`
	AchievementPercent  float64   `json:"achievement_percent"`
	
	// 품질 지표
	ConsistencyScore    float64   `json:"consistency_score"`
	EfficiencyScore     float64   `json:"efficiency_score"`
	
	// 워커별 상세 메트릭
	WorkerDetails       []WorkerMetric `json:"worker_details"`
}

// WorkerMetric - 워커별 메트릭
type WorkerMetric struct {
	WorkerID        int     `json:"worker_id"`
	Port           int     `json:"port"`
	CurrentEPS     int64   `json:"current_eps"`
	TotalSent      int64   `json:"total_sent"`
	ErrorCount     int64   `json:"error_count"`
	PacketLoss     float64 `json:"packet_loss"`
	IsActive       bool    `json:"is_active"`
	CPUUsage       float64 `json:"cpu_usage"`
}

// MetricsCollector - 실시간 메트릭 수집기
type MetricsCollector struct {
	// 기본 설정
	collectInterval     time.Duration
	historySize        int
	startTime          time.Time
	
	// 메트릭 저장
	currentMetrics     atomic.Value  // PerformanceMetrics
	metricsHistory     []PerformanceMetrics
	historyMutex       sync.RWMutex
	
	// EPS 계산용
	lastTotalSent      int64
	lastTimestamp      time.Time
	epsBuffer          []int64
	epsBufferIndex     int
	
	// 통계 계산용
	statisticsMutex    sync.RWMutex
	
	// 알림 시스템
	alertThresholds    AlertThresholds
	alertHandlers      []AlertHandler
	
	// 상태 관리
	isCollecting       atomic.Bool
	stopChan          chan struct{}
	wg                sync.WaitGroup
}

// AlertThresholds - 알림 임계값
type AlertThresholds struct {
	MinEPS            int64   // 최소 EPS (400만의 95% = 380만)
	MaxPacketLoss     float64 // 최대 패킷 손실률 (0.5%)
	MaxCPUUsage       float64 // 최대 CPU 사용률 (75%)
	MaxMemoryUsageMB  float64 // 최대 메모리 사용량 (12GB)
}

// AlertHandler - 알림 핸들러 인터페이스
type AlertHandler interface {
	HandleAlert(alertType string, message string, metrics PerformanceMetrics)
}

// NewMetricsCollector - 메트릭 수집기 생성
func NewMetricsCollector() *MetricsCollector {
	collector := &MetricsCollector{
		collectInterval: time.Second,     // 1초마다 수집
		historySize:    1800,            // 30분간 이력 보관
		startTime:      time.Now(),
		epsBuffer:      make([]int64, 60), // 1분간 EPS 버퍼
		stopChan:       make(chan struct{}),
		alertThresholds: AlertThresholds{
			MinEPS:           3800000,  // 380만 EPS (95% of target)
			MaxPacketLoss:    0.5,      // 0.5%
			MaxCPUUsage:      75.0,     // 75%
			MaxMemoryUsageMB: 12 * 1024, // 12GB
		},
	}
	
	// 초기 메트릭 설정
	collector.currentMetrics.Store(PerformanceMetrics{
		TargetEPS:  4000000, // 400만 EPS 목표
		Timestamp:  time.Now(),
	})
	
	return collector
}

// Start - 메트릭 수집 시작
func (mc *MetricsCollector) Start() {
	if !mc.isCollecting.CompareAndSwap(false, true) {
		return
	}
	
	mc.wg.Add(1)
	go mc.collectLoop()
}

// collectLoop - 메트릭 수집 루프
func (mc *MetricsCollector) collectLoop() {
	defer mc.wg.Done()
	
	ticker := time.NewTicker(mc.collectInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-mc.stopChan:
			return
		case <-ticker.C:
			metrics := mc.collectCurrentMetrics()
			mc.updateMetrics(metrics)
			mc.checkAlerts(metrics)
		}
	}
}

// collectCurrentMetrics - 현재 메트릭 수집
func (mc *MetricsCollector) collectCurrentMetrics() PerformanceMetrics {
	now := time.Now()
	uptime := now.Sub(mc.startTime)
	
	// 기존 메트릭 로드
	current := mc.GetCurrentMetrics()
	
	// EPS 계산
	var currentEPS int64
	if !mc.lastTimestamp.IsZero() && mc.lastTotalSent > 0 {
		duration := now.Sub(mc.lastTimestamp)
		if duration > 0 {
			sentSinceLastUpdate := current.TotalSent - mc.lastTotalSent
			currentEPS = int64(float64(sentSinceLastUpdate) / duration.Seconds())
		}
	}
	
	// EPS 버퍼 업데이트
	mc.epsBuffer[mc.epsBufferIndex] = currentEPS
	mc.epsBufferIndex = (mc.epsBufferIndex + 1) % len(mc.epsBuffer)
	
	// 목표 달성률 계산
	achievementPercent := float64(currentEPS) / float64(current.TargetEPS) * 100
	
	// 일관성 점수 계산 (최근 1분간 EPS 변동성)
	consistencyScore := mc.calculateConsistencyScore()
	
	// 효율성 점수 계산
	efficiencyScore := mc.calculateEfficiencyScore(current)
	
	// 새로운 메트릭 구성
	newMetrics := PerformanceMetrics{
		CurrentEPS:         currentEPS,
		TotalSent:         current.TotalSent,
		TotalErrors:       current.TotalErrors,
		PacketLoss:        current.PacketLoss,
		CPUUsagePercent:   current.CPUUsagePercent,
		MemoryUsageMB:     current.MemoryUsageMB,
		NetworkTxMBps:     current.NetworkTxMBps,
		ActiveWorkers:     current.ActiveWorkers,
		TotalWorkers:      current.TotalWorkers,
		Timestamp:         now,
		UptimeSeconds:     int64(uptime.Seconds()),
		TargetEPS:         4000000,
		AchievementPercent: achievementPercent,
		ConsistencyScore:  consistencyScore,
		EfficiencyScore:   efficiencyScore,
		WorkerDetails:     current.WorkerDetails,
	}
	
	// 다음 계산을 위한 상태 업데이트
	mc.lastTotalSent = current.TotalSent
	mc.lastTimestamp = now
	
	return newMetrics
}

// calculateConsistencyScore - 일관성 점수 계산 (0-100)
func (mc *MetricsCollector) calculateConsistencyScore() float64 {
	var sum, sumSquares float64
	var count int
	
	for _, eps := range mc.epsBuffer {
		if eps > 0 {
			sum += float64(eps)
			sumSquares += float64(eps) * float64(eps)
			count++
		}
	}
	
	if count < 2 {
		return 100.0 // 데이터가 충분하지 않으면 최고 점수
	}
	
	mean := sum / float64(count)
	variance := (sumSquares - sum*mean) / float64(count-1)
	stddev := variance
	if variance > 0 {
		stddev = variance // 간소화된 표준편차 계산
	}
	
	// 변동 계수 계산 (CV = stddev / mean)
	var cv float64
	if mean > 0 {
		cv = stddev / mean
	}
	
	// 일관성 점수 = 100 - (CV * 100), 최소 0점
	score := 100.0 - (cv * 100)
	if score < 0 {
		score = 0
	}
	
	return score
}

// calculateEfficiencyScore - 효율성 점수 계산 (0-100)
func (mc *MetricsCollector) calculateEfficiencyScore(metrics PerformanceMetrics) float64 {
	// CPU 효율성 (낮은 CPU 사용률로 높은 EPS 달성)
	var cpuEfficiency float64 = 100
	if metrics.CPUUsagePercent > 0 {
		cpuEfficiency = float64(metrics.CurrentEPS) / (metrics.CPUUsagePercent * 1000)
	}
	
	// 메모리 효율성
	var memEfficiency float64 = 100
	if metrics.MemoryUsageMB > 0 {
		memEfficiency = float64(metrics.CurrentEPS) / (metrics.MemoryUsageMB * 10)
	}
	
	// 전체 효율성 점수 (가중 평균)
	efficiency := (cpuEfficiency*0.6 + memEfficiency*0.4)
	
	// 0-100 범위로 정규화
	if efficiency > 100 {
		efficiency = 100
	}
	if efficiency < 0 {
		efficiency = 0
	}
	
	return efficiency
}

// updateMetrics - 메트릭 업데이트 및 이력 저장
func (mc *MetricsCollector) updateMetrics(metrics PerformanceMetrics) {
	// 현재 메트릭 저장
	mc.currentMetrics.Store(metrics)
	
	// 이력에 추가
	mc.historyMutex.Lock()
	mc.metricsHistory = append(mc.metricsHistory, metrics)
	
	// 이력 크기 제한
	if len(mc.metricsHistory) > mc.historySize {
		mc.metricsHistory = mc.metricsHistory[1:]
	}
	mc.historyMutex.Unlock()
}

// checkAlerts - 알림 조건 검사
func (mc *MetricsCollector) checkAlerts(metrics PerformanceMetrics) {
	// EPS 성능 알림
	if metrics.CurrentEPS < mc.alertThresholds.MinEPS {
		mc.triggerAlert("LOW_EPS", 
			"EPS 성능 저하 감지", metrics)
	}
	
	// 패킷 손실 알림
	if metrics.PacketLoss > mc.alertThresholds.MaxPacketLoss {
		mc.triggerAlert("HIGH_PACKET_LOSS", 
			"패킷 손실률 임계값 초과", metrics)
	}
	
	// CPU 사용률 알림
	if metrics.CPUUsagePercent > mc.alertThresholds.MaxCPUUsage {
		mc.triggerAlert("HIGH_CPU", 
			"CPU 사용률 임계값 초과", metrics)
	}
	
	// 메모리 사용량 알림
	if metrics.MemoryUsageMB > mc.alertThresholds.MaxMemoryUsageMB {
		mc.triggerAlert("HIGH_MEMORY", 
			"메모리 사용량 임계값 초과", metrics)
	}
}

// triggerAlert - 알림 발생
func (mc *MetricsCollector) triggerAlert(alertType, message string, metrics PerformanceMetrics) {
	for _, handler := range mc.alertHandlers {
		go handler.HandleAlert(alertType, message, metrics)
	}
}

// Stop - 메트릭 수집 중지
func (mc *MetricsCollector) Stop() {
	if !mc.isCollecting.CompareAndSwap(true, false) {
		return
	}
	
	close(mc.stopChan)
	mc.wg.Wait()
}

// GetCurrentMetrics - 현재 메트릭 반환
func (mc *MetricsCollector) GetCurrentMetrics() PerformanceMetrics {
	if value := mc.currentMetrics.Load(); value != nil {
		return value.(PerformanceMetrics)
	}
	
	return PerformanceMetrics{
		TargetEPS: 4000000,
		Timestamp: time.Now(),
	}
}

// GetMetricsHistory - 메트릭 이력 반환
func (mc *MetricsCollector) GetMetricsHistory(duration time.Duration) []PerformanceMetrics {
	mc.historyMutex.RLock()
	defer mc.historyMutex.RUnlock()
	
	cutoff := time.Now().Add(-duration)
	var result []PerformanceMetrics
	
	for _, metrics := range mc.metricsHistory {
		if metrics.Timestamp.After(cutoff) {
			result = append(result, metrics)
		}
	}
	
	return result
}

// UpdateWorkerMetrics - 워커 메트릭 업데이트
func (mc *MetricsCollector) UpdateWorkerMetrics(workerMetrics []WorkerMetric) {
	current := mc.GetCurrentMetrics()
	
	var totalSent, totalErrors int64
	var activeWorkers int
	var totalPacketLoss float64
	
	for _, worker := range workerMetrics {
		if worker.IsActive {
			activeWorkers++
			totalSent += worker.TotalSent
			totalErrors += worker.ErrorCount
			totalPacketLoss += worker.PacketLoss
		}
	}
	
	// 평균 패킷 손실률 계산
	var avgPacketLoss float64
	if activeWorkers > 0 {
		avgPacketLoss = totalPacketLoss / float64(activeWorkers)
	}
	
	// 메트릭 업데이트
	current.TotalSent = totalSent
	current.TotalErrors = totalErrors
	current.PacketLoss = avgPacketLoss
	current.ActiveWorkers = activeWorkers
	current.TotalWorkers = len(workerMetrics)
	current.WorkerDetails = workerMetrics
	
	mc.currentMetrics.Store(current)
}

// AddAlertHandler - 알림 핸들러 추가
func (mc *MetricsCollector) AddAlertHandler(handler AlertHandler) {
	mc.alertHandlers = append(mc.alertHandlers, handler)
}

// GetSummaryReport - 요약 리포트 생성
func (mc *MetricsCollector) GetSummaryReport() map[string]interface{} {
	current := mc.GetCurrentMetrics()
	uptime := time.Since(mc.startTime)
	
	return map[string]interface{}{
		"current_eps":          current.CurrentEPS,
		"target_eps":          current.TargetEPS,
		"achievement_percent":  current.AchievementPercent,
		"total_sent":          current.TotalSent,
		"uptime_hours":        uptime.Hours(),
		"average_eps":         float64(current.TotalSent) / uptime.Seconds(),
		"consistency_score":   current.ConsistencyScore,
		"efficiency_score":    current.EfficiencyScore,
		"packet_loss_rate":    current.PacketLoss,
		"active_workers":      current.ActiveWorkers,
		"cpu_usage_percent":   current.CPUUsagePercent,
		"memory_usage_mb":     current.MemoryUsageMB,
	}
}

// ExportMetrics - 메트릭 JSON 내보내기
func (mc *MetricsCollector) ExportMetrics() ([]byte, error) {
	current := mc.GetCurrentMetrics()
	return json.MarshalIndent(current, "", "  ")
}