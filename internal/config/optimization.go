package config

import (
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// OptimizationConfig - 400만 EPS 달성을 위한 최적화 설정
type OptimizationConfig struct {
	// 메모리 최적화
	MaxMemoryMB        int64   // 최대 메모리 사용량 (MB)
	GCTargetPercent    int     // GC 목표 퍼센트 (기본 100 → 200으로 증가)
	MemoryLimitMB      int64   // 메모리 제한 (12GB = PRD 요구사항)
	
	// CPU 최적화
	MaxProcs           int     // 최대 프로세서 수
	EnableProfiling    bool    // 프로파일링 활성화
	CPUAffinity        bool    // CPU 친화성 설정
	
	// GC 최적화
	GCForceInterval    time.Duration // 강제 GC 실행 간격
	DisableGC          bool          // GC 완전 비활성화 (위험)
	
	// 네트워크 최적화
	SocketBufferSize   int     // 소켓 버퍼 크기
	BatchSize          int     // 배치 전송 크기
	
	// 모니터링
	EnableRuntimeStats bool    // 런타임 통계 활성화
}

// DefaultOptimizationConfig - PRD 명세에 따른 기본 최적화 설정
func DefaultOptimizationConfig() *OptimizationConfig {
	return &OptimizationConfig{
		MaxMemoryMB:        12 * 1024,    // 12GB (PRD 요구사항)
		GCTargetPercent:    200,          // GC 압박 감소
		MemoryLimitMB:      12 * 1024,    // 12GB 제한
		MaxProcs:           runtime.NumCPU(), 
		EnableProfiling:    false,
		CPUAffinity:        true,
		GCForceInterval:    30 * time.Second, // 30초마다 강제 GC
		DisableGC:          false,
		SocketBufferSize:   2 * 1024 * 1024, // 2MB
		BatchSize:          1000,
		EnableRuntimeStats: true,
	}
}

// MemoryOptimizer - 메모리 최적화 관리자
type MemoryOptimizer struct {
	config          *OptimizationConfig
	isActive        bool
	stopChan        chan struct{}
	wg              sync.WaitGroup
	
	// 통계
	gcCount         int64
	lastGCTime      time.Time
	memoryPressure  float64
}

// NewMemoryOptimizer - 메모리 최적화 관리자 생성
func NewMemoryOptimizer(config *OptimizationConfig) *MemoryOptimizer {
	if config == nil {
		config = DefaultOptimizationConfig()
	}
	
	return &MemoryOptimizer{
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// Initialize - 메모리 최적화 초기화
func (mo *MemoryOptimizer) Initialize() error {
	// GOMAXPROCS 설정
	if mo.config.MaxProcs > 0 {
		runtime.GOMAXPROCS(mo.config.MaxProcs)
	}
	
	// GC 목표 퍼센트 설정 (메모리 압박 감소)
	debug.SetGCPercent(mo.config.GCTargetPercent)
	
	// 메모리 제한 설정 (Go 1.19+)
	if mo.config.MemoryLimitMB > 0 {
		memLimit := mo.config.MemoryLimitMB * 1024 * 1024 // MB to bytes
		debug.SetMemoryLimit(memLimit)
	}
	
	// GC 완전 비활성화 (극도의 성능이 필요한 경우만)
	if mo.config.DisableGC {
		debug.SetGCPercent(-1)
	}
	
	return nil
}

// Start - 메모리 최적화 시작
func (mo *MemoryOptimizer) Start() {
	if mo.isActive {
		return
	}
	
	mo.isActive = true
	
	// 주기적 GC 강제 실행
	if mo.config.GCForceInterval > 0 && !mo.config.DisableGC {
		mo.wg.Add(1)
		go mo.gcForcer()
	}
	
	// 메모리 압박 모니터링
	if mo.config.EnableRuntimeStats {
		mo.wg.Add(1)
		go mo.memoryMonitor()
	}
}

// gcForcer - 주기적 강제 GC 실행
func (mo *MemoryOptimizer) gcForcer() {
	defer mo.wg.Done()
	
	ticker := time.NewTicker(mo.config.GCForceInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-mo.stopChan:
			return
		case <-ticker.C:
			start := time.Now()
			runtime.GC()
			debug.FreeOSMemory()
			
			mo.gcCount++
			mo.lastGCTime = time.Now()
			gcDuration := time.Since(start)
			
			if gcDuration > time.Millisecond*100 {
				// GC가 100ms 이상 소요되면 경고
				runtime.GC() // 한 번 더 실행
			}
		}
	}
}

// memoryMonitor - 메모리 압박 모니터링
func (mo *MemoryOptimizer) memoryMonitor() {
	defer mo.wg.Done()
	
	ticker := time.NewTicker(time.Second * 5) // 5초마다 체크
	defer ticker.Stop()
	
	for {
		select {
		case <-mo.stopChan:
			return
		case <-ticker.C:
			mo.checkMemoryPressure()
		}
	}
}

// checkMemoryPressure - 메모리 압박 확인 및 대응
func (mo *MemoryOptimizer) checkMemoryPressure() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	currentMemoryMB := float64(m.Alloc) / 1024 / 1024
	maxMemoryMB := float64(mo.config.MaxMemoryMB)
	
	mo.memoryPressure = currentMemoryMB / maxMemoryMB
	
	// 메모리 사용량이 90% 초과시 적극적 GC
	if mo.memoryPressure > 0.9 {
		runtime.GC()
		debug.FreeOSMemory()
	}
	
	// 메모리 사용량이 95% 초과시 연속 GC
	if mo.memoryPressure > 0.95 {
		for i := 0; i < 3; i++ {
			runtime.GC()
			time.Sleep(time.Millisecond * 10)
		}
		debug.FreeOSMemory()
	}
}

// Stop - 메모리 최적화 중지
func (mo *MemoryOptimizer) Stop() {
	if !mo.isActive {
		return
	}
	
	mo.isActive = false
	close(mo.stopChan)
	mo.wg.Wait()
	
	// 최종 메모리 정리
	runtime.GC()
	debug.FreeOSMemory()
}

// GetStats - 메모리 최적화 통계 반환
func (mo *MemoryOptimizer) GetStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return map[string]interface{}{
		"memory_alloc_mb":    float64(m.Alloc) / 1024 / 1024,
		"memory_sys_mb":      float64(m.Sys) / 1024 / 1024,
		"gc_count":           mo.gcCount,
		"last_gc_time":       mo.lastGCTime,
		"memory_pressure":    mo.memoryPressure,
		"heap_objects":       m.HeapObjects,
		"gc_cpu_fraction":    m.GCCPUFraction,
		"gc_pause_ns":        m.PauseNs[(m.NumGC+255)%256],
		"num_goroutines":     runtime.NumGoroutine(),
		"gomaxprocs":         runtime.GOMAXPROCS(0),
	}
}

// GlobalMemoryPools - 전역 메모리 풀 모음
var GlobalMemoryPools = struct {
	// 로그 관련 풀
	LogBuffer    sync.Pool
	StringBuffer sync.Pool
	ByteSlice    sync.Pool
	
	// 네트워크 관련 풀  
	UDPPacket    sync.Pool
	BatchBuffer  sync.Pool
}{
	LogBuffer: sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 512) // 평균 로그 크기 + 여유
		},
	},
	StringBuffer: sync.Pool{
		New: func() interface{} {
			return make([]string, 0, 100)
		},
	},
	ByteSlice: sync.Pool{
		New: func() interface{} {
			return make([][]byte, 0, 1000) // 배치 크기
		},
	},
	UDPPacket: sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 1500) // 일반적인 MTU 크기
		},
	},
	BatchBuffer: sync.Pool{
		New: func() interface{} {
			return make([][]byte, 0, 1000) // 1000개 로그 배치
		},
	},
}

// InitializeGlobalPools - 전역 풀 초기화
func InitializeGlobalPools() {
	// 풀 사전 워밍업 (초기 객체 생성)
	for i := 0; i < 100; i++ {
		// 각 풀에 객체 생성 후 반납 (워밍업)
		logBuf := GlobalMemoryPools.LogBuffer.Get().([]byte)
		GlobalMemoryPools.LogBuffer.Put(logBuf[:0])
		
		strBuf := GlobalMemoryPools.StringBuffer.Get().([]string)
		GlobalMemoryPools.StringBuffer.Put(strBuf[:0])
		
		byteBuf := GlobalMemoryPools.ByteSlice.Get().([][]byte)
		GlobalMemoryPools.ByteSlice.Put(byteBuf[:0])
		
		udpBuf := GlobalMemoryPools.UDPPacket.Get().([]byte)
		GlobalMemoryPools.UDPPacket.Put(udpBuf[:0])
		
		batchBuf := GlobalMemoryPools.BatchBuffer.Get().([][]byte)
		GlobalMemoryPools.BatchBuffer.Put(batchBuf[:0])
	}
}

// PerformanceHints - 성능 향상 힌트
type PerformanceHints struct {
	EnabledOptimizations []string
	Recommendations      []string
	CurrentBottlenecks   []string
}

// AnalyzePerformance - 성능 분석 및 최적화 제안
func AnalyzePerformance() *PerformanceHints {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	hints := &PerformanceHints{
		EnabledOptimizations: []string{},
		Recommendations:      []string{},
		CurrentBottlenecks:   []string{},
	}
	
	// GC 압박 분석
	if m.GCCPUFraction > 0.05 { // GC가 CPU의 5% 이상 사용
		hints.CurrentBottlenecks = append(hints.CurrentBottlenecks, 
			"High GC CPU usage: consider increasing GC target percent")
		hints.Recommendations = append(hints.Recommendations, 
			"Set GOGC=200 or higher to reduce GC frequency")
	}
	
	// 메모리 사용량 분석
	memUsageMB := float64(m.Alloc) / 1024 / 1024
	if memUsageMB > 8*1024 { // 8GB 이상
		hints.CurrentBottlenecks = append(hints.CurrentBottlenecks, 
			"High memory usage: consider memory pool optimization")
		hints.Recommendations = append(hints.Recommendations, 
			"Implement more aggressive memory pooling")
	}
	
	// 고루틴 수 분석
	numGoroutines := runtime.NumGoroutine()
	if numGoroutines > 10000 {
		hints.CurrentBottlenecks = append(hints.CurrentBottlenecks, 
			"High goroutine count: potential goroutine leak")
		hints.Recommendations = append(hints.Recommendations, 
			"Review goroutine lifecycle management")
	}
	
	return hints
}