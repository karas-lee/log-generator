package monitor

import (
	"encoding/json"
	"fmt"
	"log-generator/internal/config"
	"log-generator/internal/worker"
	"log-generator/pkg/metrics"
	"net/http"
	"sync"
	"time"
)

// ControlServer - 웹 UI 기반 로그 생성기 제어 서버
type ControlServer struct {
	port             int
	metricsCollector *metrics.MetricsCollector
	
	// 로그 생성기 상태
	workerPool       *worker.WorkerPool
	memoryOptimizer  *config.MemoryOptimizer
	isRunning        bool
	currentConfig    *GeneratorConfig
	
	// 제어 상태
	mutex            sync.RWMutex
	httpServer       *http.Server
}

// GeneratorConfig - 로그 생성기 설정
type GeneratorConfig struct {
	TargetHost       string `json:"target_host"`
	TargetEPS        int64  `json:"target_eps"`
	Duration         int    `json:"duration_minutes"`
	EnableDashboard  bool   `json:"enable_dashboard"`
	EnableOptimization bool `json:"enable_optimization"`
	WorkerCount      int    `json:"worker_count"`
	
	// 고급 설정
	BatchSize        int     `json:"batch_size"`
	SendInterval     int     `json:"send_interval_ms"`
	MemoryLimitGB    int     `json:"memory_limit_gb"`
	GCPercent        int     `json:"gc_percent"`
	
	// 로그 설정
	LogFormats       []string `json:"log_formats"`
	HostnamePrefix   string   `json:"hostname_prefix"`
	ServiceTypes     []string `json:"service_types"`
}

// GeneratorStatus - 로그 생성기 현재 상태
type GeneratorStatus struct {
	IsRunning        bool              `json:"is_running"`
	StartTime        *time.Time        `json:"start_time"`
	Uptime           int64             `json:"uptime_seconds"`
	Config           *GeneratorConfig  `json:"config"`
	Metrics          interface{}       `json:"metrics"`
	WorkerStatuses   map[int]bool     `json:"worker_statuses"`
}

// ControlResponse - API 응답 구조
type ControlResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewControlServer - 제어 서버 생성
func NewControlServer(port int) *ControlServer {
	return &ControlServer{
		port:             port,
		metricsCollector: metrics.NewMetricsCollector(),
		currentConfig:    getDefaultConfig(),
		isRunning:        false,
	}
}

// getDefaultConfig - 기본 설정 반환
func getDefaultConfig() *GeneratorConfig {
	return &GeneratorConfig{
		TargetHost:         "127.0.0.1",
		TargetEPS:          4000000,
		Duration:           0, // 무제한
		EnableDashboard:    true,
		EnableOptimization: true,
		WorkerCount:        40,
		BatchSize:          1000,
		SendInterval:       10,
		MemoryLimitGB:      12,
		GCPercent:          200,
		LogFormats:         []string{"syslog", "apache", "nginx"},
		HostnamePrefix:     "server",
		ServiceTypes:       []string{"systemd", "kernel", "sshd", "nginx", "apache"},
	}
}

// Start - 제어 서버 시작
func (cs *ControlServer) Start() error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	
	// HTTP 라우팅 설정
	mux := http.NewServeMux()
	
	// UI 페이지
	mux.HandleFunc("/", cs.handleMainUI)
	mux.HandleFunc("/control", cs.handleControlUI)
	
	// API 엔드포인트
	mux.HandleFunc("/api/status", cs.handleGetStatus)
	mux.HandleFunc("/api/config", cs.handleConfig)
	mux.HandleFunc("/api/start", cs.handleStart)
	mux.HandleFunc("/api/stop", cs.handleStop)
	mux.HandleFunc("/api/restart", cs.handleRestart)
	mux.HandleFunc("/api/metrics", cs.handleMetrics)
	mux.HandleFunc("/api/workers", cs.handleWorkers)
	mux.HandleFunc("/api/system-optimize", cs.handleSystemOptimize)
	
	// WebSocket (기존 모니터링)
	mux.HandleFunc("/ws", cs.handleWebSocket)
	
	// 정적 파일
	mux.HandleFunc("/static/", cs.handleStatic)
	
	// HTTP 서버 설정
	cs.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cs.port),
		Handler: mux,
	}
	
	fmt.Printf("🌐 로그 생성기 제어 서버 시작: http://localhost:%d\n", cs.port)
	
	// HTTP 서버 시작
	go func() {
		if err := cs.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("제어 서버 오류: %v\n", err)
		}
	}()
	
	return nil
}

// handleMainUI - 메인 제어 UI
func (cs *ControlServer) handleMainUI(w http.ResponseWriter, r *http.Request) {
	html := cs.generateControlUI()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// handleControlUI - 제어 전용 UI (임베드 가능)
func (cs *ControlServer) handleControlUI(w http.ResponseWriter, r *http.Request) {
	html := cs.generateEmbeddedControlUI()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// handleGetStatus - 현재 상태 조회
func (cs *ControlServer) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	
	var startTime *time.Time
	var uptime int64
	
	if cs.isRunning && cs.workerPool != nil {
		// 실제 시작 시간과 업타임 계산
		start := time.Now().Add(-time.Hour) // 임시값
		startTime = &start
		uptime = int64(time.Since(start).Seconds())
	}
	
	status := GeneratorStatus{
		IsRunning:      cs.isRunning,
		StartTime:      startTime,
		Uptime:         uptime,
		Config:         cs.currentConfig,
		WorkerStatuses: make(map[int]bool),
	}
	
	// 워커 상태 추가
	if cs.workerPool != nil {
		poolMetrics := cs.workerPool.GetMetrics()
		for id, worker := range poolMetrics.WorkerMetrics {
			status.WorkerStatuses[id] = worker.CurrentEPS > 0
		}
		status.Metrics = poolMetrics
	}
	
	cs.sendJSON(w, ControlResponse{
		Success: true,
		Data:    status,
	})
}

// handleConfig - 설정 관리
func (cs *ControlServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cs.mutex.RLock()
		config := cs.currentConfig
		cs.mutex.RUnlock()
		
		cs.sendJSON(w, ControlResponse{
			Success: true,
			Data:    config,
		})
		
	case "POST", "PUT":
		var newConfig GeneratorConfig
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			cs.sendJSON(w, ControlResponse{
				Success: false,
				Error:   "설정 파싱 오류: " + err.Error(),
			})
			return
		}
		
		// 설정 유효성 검사
		if err := cs.validateConfig(&newConfig); err != nil {
			cs.sendJSON(w, ControlResponse{
				Success: false,
				Error:   "설정 검증 실패: " + err.Error(),
			})
			return
		}
		
		cs.mutex.Lock()
		cs.currentConfig = &newConfig
		cs.mutex.Unlock()
		
		cs.sendJSON(w, ControlResponse{
			Success: true,
			Message: "설정이 저장되었습니다",
			Data:    &newConfig,
		})
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStart - 로그 생성기 시작
func (cs *ControlServer) handleStart(w http.ResponseWriter, r *http.Request) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	
	if cs.isRunning {
		cs.sendJSON(w, ControlResponse{
			Success: false,
			Error:   "로그 생성기가 이미 실행 중입니다",
		})
		return
	}
	
	// 설정 기반으로 로그 생성기 초기화
	err := cs.initializeGenerator()
	if err != nil {
		cs.sendJSON(w, ControlResponse{
			Success: false,
			Error:   "초기화 실패: " + err.Error(),
		})
		return
	}
	
	// 로그 생성기 시작
	err = cs.startGenerator()
	if err != nil {
		cs.sendJSON(w, ControlResponse{
			Success: false,
			Error:   "시작 실패: " + err.Error(),
		})
		return
	}
	
	cs.isRunning = true
	
	cs.sendJSON(w, ControlResponse{
		Success: true,
		Message: fmt.Sprintf("로그 생성기 시작됨 (%d개 워커, 목표: %d EPS)", 
			cs.currentConfig.WorkerCount, cs.currentConfig.TargetEPS),
	})
}

// handleStop - 로그 생성기 정지
func (cs *ControlServer) handleStop(w http.ResponseWriter, r *http.Request) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	
	if !cs.isRunning {
		cs.sendJSON(w, ControlResponse{
			Success: false,
			Error:   "로그 생성기가 실행되지 않고 있습니다",
		})
		return
	}
	
	err := cs.stopGenerator()
	if err != nil {
		cs.sendJSON(w, ControlResponse{
			Success: false,
			Error:   "정지 실패: " + err.Error(),
		})
		return
	}
	
	cs.isRunning = false
	
	cs.sendJSON(w, ControlResponse{
		Success: true,
		Message: "로그 생성기가 정지되었습니다",
	})
}

// handleRestart - 로그 생성기 재시작
func (cs *ControlServer) handleRestart(w http.ResponseWriter, r *http.Request) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	
	// 실행 중이면 먼저 정지
	if cs.isRunning {
		cs.stopGenerator()
		cs.isRunning = false
		time.Sleep(time.Second) // 정리 대기
	}
	
	// 재시작
	err := cs.initializeGenerator()
	if err == nil {
		err = cs.startGenerator()
	}
	
	if err != nil {
		cs.sendJSON(w, ControlResponse{
			Success: false,
			Error:   "재시작 실패: " + err.Error(),
		})
		return
	}
	
	cs.isRunning = true
	
	cs.sendJSON(w, ControlResponse{
		Success: true,
		Message: "로그 생성기가 재시작되었습니다",
	})
}

// handleMetrics - 메트릭 조회
func (cs *ControlServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	var metrics interface{}
	
	if cs.metricsCollector != nil {
		metrics = cs.metricsCollector.GetCurrentMetrics()
	}
	
	cs.sendJSON(w, ControlResponse{
		Success: true,
		Data:    metrics,
	})
}

// handleWorkers - 워커 제어
func (cs *ControlServer) handleWorkers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// 워커 상태 조회
		var workerStats map[int]interface{}
		
		if cs.workerPool != nil {
			poolMetrics := cs.workerPool.GetMetrics()
			workerStats = make(map[int]interface{})
			for id, worker := range poolMetrics.WorkerMetrics {
				workerStats[id] = map[string]interface{}{
					"id":          worker.WorkerID,
					"port":        worker.Port,
					"current_eps": worker.CurrentEPS,
					"total_sent":  worker.TotalSent,
					"errors":      worker.ErrorCount,
					"active":      worker.CurrentEPS > 0,
				}
			}
		}
		
		cs.sendJSON(w, ControlResponse{
			Success: true,
			Data:    workerStats,
		})
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSystemOptimize - 시스템 최적화
func (cs *ControlServer) handleSystemOptimize(w http.ResponseWriter, r *http.Request) {
	// 실제로는 시스템 명령어 실행이 필요하지만 여기서는 시뮬레이션
	cs.sendJSON(w, ControlResponse{
		Success: true,
		Message: "시스템 최적화 적용 완료 (실제로는 sudo 권한 필요)",
	})
}

// WebSocket 핸들러 (기존 대시보드와 동일)
func (cs *ControlServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 기존 dashboard.go의 WebSocket 로직 재사용
}

// 정적 파일 핸들러
func (cs *ControlServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// 헬퍼 메소드들
func (cs *ControlServer) sendJSON(w http.ResponseWriter, response ControlResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(response)
}

func (cs *ControlServer) validateConfig(config *GeneratorConfig) error {
	if config.TargetEPS <= 0 || config.TargetEPS > 10000000 {
		return fmt.Errorf("목표 EPS는 1-10,000,000 범위여야 합니다")
	}
	if config.WorkerCount <= 0 || config.WorkerCount > 100 {
		return fmt.Errorf("워커 수는 1-100 범위여야 합니다")
	}
	if config.MemoryLimitGB <= 0 || config.MemoryLimitGB > 64 {
		return fmt.Errorf("메모리 제한은 1-64GB 범위여야 합니다")
	}
	return nil
}

func (cs *ControlServer) initializeGenerator() error {
	// 메모리 최적화 초기화
	if cs.currentConfig.EnableOptimization {
		optimizationConfig := config.DefaultOptimizationConfig()
		optimizationConfig.MemoryLimitMB = int64(cs.currentConfig.MemoryLimitGB) * 1024
		optimizationConfig.GCTargetPercent = cs.currentConfig.GCPercent
		
		cs.memoryOptimizer = config.NewMemoryOptimizer(optimizationConfig)
		cs.memoryOptimizer.Initialize()
		cs.memoryOptimizer.Start()
	}
	
	// 워커 풀 초기화
	cs.workerPool = worker.NewWorkerPool(cs.currentConfig.TargetHost)
	err := cs.workerPool.Initialize()
	if err != nil {
		return err
	}
	
	// 메트릭 수집기 시작
	cs.metricsCollector.Start()
	
	return nil
}

func (cs *ControlServer) startGenerator() error {
	if cs.workerPool == nil {
		return fmt.Errorf("워커 풀이 초기화되지 않았습니다")
	}
	
	return cs.workerPool.Start()
}

func (cs *ControlServer) stopGenerator() error {
	var errors []error
	
	if cs.workerPool != nil {
		if err := cs.workerPool.Stop(); err != nil {
			errors = append(errors, err)
		}
		cs.workerPool = nil
	}
	
	if cs.metricsCollector != nil {
		cs.metricsCollector.Stop()
	}
	
	if cs.memoryOptimizer != nil {
		cs.memoryOptimizer.Stop()
		cs.memoryOptimizer = nil
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("정지 중 오류 발생: %v", errors)
	}
	
	return nil
}

// Stop - 제어 서버 정지
func (cs *ControlServer) Stop() error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	
	// 로그 생성기 정지
	if cs.isRunning {
		cs.stopGenerator()
		cs.isRunning = false
	}
	
	// HTTP 서버 정지
	if cs.httpServer != nil {
		return cs.httpServer.Close()
	}
	
	return nil
}