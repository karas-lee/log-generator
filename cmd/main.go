package main

import (
	"context"
	"flag"
	"fmt"
	"log-generator/internal/config"
	"log-generator/internal/monitor"
	"log-generator/internal/worker"
	"log-generator/pkg/metrics"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// AppConfig - 애플리케이션 설정
type AppConfig struct {
	TargetHost        string
	DashboardPort     int
	TestDurationMin   int
	EnableDashboard   bool
	EnableOptimization bool
	LogLevel          string
	Profile           string  // EPS 프로파일
	TargetEPS         int     // 커스텀 EPS
}

// LogGenerator - 400만 EPS 로그 생성기 메인 애플리케이션
type LogGenerator struct {
	config           *AppConfig
	workerPool       *worker.WorkerPool
	metricsCollector *metrics.MetricsCollector
	dashboard        *monitor.DashboardServer
	memoryOptimizer  *config.MemoryOptimizer
	
	// 상태 관리
	ctx              context.Context
	cancel           context.CancelFunc
	startTime        time.Time
	isRunning        bool
}

func main() {
	// 명령행 파라미터 파싱
	appConfig := parseFlags()
	
	// 애플리케이션 생성
	app, err := NewLogGenerator(appConfig)
	if err != nil {
		fmt.Printf("❌ 애플리케이션 초기화 실패: %v\n", err)
		os.Exit(1)
	}
	
	// 시작 메시지
	printWelcomeMessage()
	
	// 시그널 핸들링 설정
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	// 애플리케이션 시작
	err = app.Start()
	if err != nil {
		fmt.Printf("❌ 애플리케이션 시작 실패: %v\n", err)
		os.Exit(1)
	}
	
	// 테스트 지속 시간 체크
	var testTimer <-chan time.Time
	if appConfig.TestDurationMin > 0 {
		testTimer = time.After(time.Duration(appConfig.TestDurationMin) * time.Minute)
		fmt.Printf("⏰ %d분 후 자동 종료 예정\n", appConfig.TestDurationMin)
	}
	
	// 종료 신호 대기
	select {
	case <-sigChan:
		fmt.Println("\n🛑 종료 신호 수신, 애플리케이션 종료 중...")
	case <-testTimer:
		fmt.Println("\n⏰ 테스트 시간 만료, 애플리케이션 종료 중...")
	}
	
	// 애플리케이션 정지
	err = app.Stop()
	if err != nil {
		fmt.Printf("⚠️  애플리케이션 정지 중 오류: %v\n", err)
	}
	
	fmt.Println("✅ 애플리케이션 정상 종료")
}

// parseFlags - 명령행 파라미터 파싱
func parseFlags() *AppConfig {
	config := &AppConfig{}
	
	flag.StringVar(&config.TargetHost, "target", "127.0.0.1", 
		"SIEM 시스템 호스트 주소")
	flag.IntVar(&config.DashboardPort, "dashboard-port", 8080, 
		"대시보드 웹 서버 포트")
	flag.IntVar(&config.TestDurationMin, "duration", 0, 
		"테스트 실행 시간 (분, 0=무제한)")
	flag.BoolVar(&config.EnableDashboard, "dashboard", true, 
		"웹 대시보드 활성화")
	flag.BoolVar(&config.EnableOptimization, "optimize", true, 
		"메모리/성능 최적화 활성화")
	flag.StringVar(&config.LogLevel, "log-level", "info", 
		"로그 레벨 (debug, info, warn, error)")
	flag.StringVar(&config.Profile, "profile", "4m",
		"EPS 프로파일 (100k, 500k, 1m, 2m, 4m, custom)")
	flag.IntVar(&config.TargetEPS, "eps", 0,
		"커스텀 목표 EPS (profile=custom일 때 사용)")
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Available EPS Profiles:\n")
		fmt.Fprintf(os.Stderr, "  100k: Light load (Workers: 2, Batch: 10)\n")
		fmt.Fprintf(os.Stderr, "  500k: Medium load (Workers: 5, Batch: 20)\n")
		fmt.Fprintf(os.Stderr, "  1m: Standard load (Workers: 10, Batch: 50)\n")
		fmt.Fprintf(os.Stderr, "  2m: High load (Workers: 20, Batch: 100)\n")
		fmt.Fprintf(os.Stderr, "  4m: Maximum load (Workers: 40, Batch: 200)\n")
		fmt.Fprintf(os.Stderr, "  custom: Specify custom EPS target with -eps flag\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}
	
	flag.Parse()
	
	// 커스텀 프로파일 검증
	if config.Profile == "custom" && config.TargetEPS == 0 {
		fmt.Println("⚠️  custom 프로파일에는 -eps 플래그가 필요합니다")
		os.Exit(1)
	}
	
	return config
}

// NewLogGenerator - 로그 생성기 애플리케이션 생성
func NewLogGenerator(appConfig *AppConfig) (*LogGenerator, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	app := &LogGenerator{
		config: appConfig,
		ctx:    ctx,
		cancel: cancel,
	}
	
	// 메트릭 수집기 초기화
	app.metricsCollector = metrics.NewMetricsCollector()
	
	// 프로파일 설정
	var profile *config.EPSProfile
	if appConfig.Profile == "custom" {
		profile = config.CalculateCustomProfile(appConfig.TargetEPS)
	} else {
		var err error
		profile, err = config.GetProfile(appConfig.Profile)
		if err != nil {
			return nil, fmt.Errorf("프로파일 로드 실패: %v", err)
		}
	}
	
	// 프로파일 기반 워커 풀 초기화
	app.workerPool = worker.NewWorkerPoolWithProfile(appConfig.TargetHost, profile)
	
	// 대시보드 초기화 (옵션)
	if appConfig.EnableDashboard {
		app.dashboard = monitor.NewDashboardServer(
			appConfig.DashboardPort, app.metricsCollector)
		// 프로파일 정보 설정
		app.dashboard.SetProfile(profile.Name, int64(profile.TargetEPS))
	}
	
	// 메모리 최적화 초기화 (옵션)
	if appConfig.EnableOptimization {
		optimizationConfig := config.DefaultOptimizationConfig()
		app.memoryOptimizer = config.NewMemoryOptimizer(optimizationConfig)
	}
	
	return app, nil
}

// Start - 애플리케이션 시작
func (lg *LogGenerator) Start() error {
	if lg.isRunning {
		return fmt.Errorf("애플리케이션이 이미 실행 중입니다")
	}
	
	lg.startTime = time.Now()
	lg.isRunning = true
	
	profile := lg.workerPool.GetProfile()
	fmt.Printf("🚀 %s 프로파일 로그 전송기 시작 (목표: %s EPS)\n", profile.Name, formatNumber(int64(profile.TargetEPS)))
	fmt.Println("=" + repeatString("=", 60))
	
	// 시스템 정보 출력
	lg.printSystemInfo()
	
	// 1. 메모리 최적화 시작
	if lg.memoryOptimizer != nil {
		err := lg.memoryOptimizer.Initialize()
		if err != nil {
			return fmt.Errorf("메모리 최적화 초기화 실패: %v", err)
		}
		lg.memoryOptimizer.Start()
		fmt.Println("✅ 메모리 최적화 활성화")
		
		// 전역 메모리 풀 초기화
		config.InitializeGlobalPools()
		fmt.Println("✅ 전역 메모리 풀 초기화 완료")
	}
	
	// 2. 메트릭 수집 시작
	lg.metricsCollector.Start()
	fmt.Println("✅ 메트릭 수집기 시작")
	
	// 3. 워커 풀 초기화 및 시작
	err := lg.workerPool.Initialize()
	if err != nil {
		return fmt.Errorf("워커 풀 초기화 실패: %v", err)
	}
	
	err = lg.workerPool.Start()
	if err != nil {
		return fmt.Errorf("워커 풀 시작 실패: %v", err)
	}
	fmt.Printf("✅ 워커 풀 시작 (%d개 워커)\n", lg.workerPool.GetWorkerCount())
	
	// 4. 웹 대시보드 시작
	if lg.dashboard != nil {
		err = lg.dashboard.Start()
		if err != nil {
			return fmt.Errorf("대시보드 시작 실패: %v", err)
		}
		fmt.Printf("✅ 웹 대시보드: http://localhost:%d\n", lg.config.DashboardPort)
	}
	
	// 5. 메트릭 업데이트 루프 시작
	go lg.metricsUpdateLoop()
	
	fmt.Println("=" + repeatString("=", 60))
	fmt.Printf("🎯 목표: %s EPS 달성\n", formatNumber(int64(profile.TargetEPS)))
	fmt.Println("📊 실시간 모니터링 시작...")
	fmt.Println()
	
	return nil
}

// metricsUpdateLoop - 메트릭 업데이트 루프
func (lg *LogGenerator) metricsUpdateLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-lg.ctx.Done():
			return
		case <-ticker.C:
			lg.updateMetrics()
		}
	}
}

// updateMetrics - 워커 풀로부터 메트릭 업데이트
func (lg *LogGenerator) updateMetrics() {
	poolMetrics := lg.workerPool.GetMetrics()
	
	// 워커별 메트릭 변환
	var workerMetrics []metrics.WorkerMetric
	for _, wm := range poolMetrics.WorkerMetrics {
		workerMetrics = append(workerMetrics, metrics.WorkerMetric{
			WorkerID:   wm.WorkerID,
			Port:       wm.Port,
			CurrentEPS: wm.CurrentEPS,
			TotalSent:  wm.TotalSent,
			ErrorCount: wm.ErrorCount,
			PacketLoss: wm.PacketLoss,
			IsActive:   wm.CurrentEPS > 0, // EPS가 있으면 활성상태로 간주
			CPUUsage:   wm.CPUUsage,
		})
	}
	
	// 메트릭 컬렉터 업데이트
	lg.metricsCollector.UpdateWorkerMetrics(workerMetrics)
	
	// 현재 메트릭 가져와서 시스템 메트릭 업데이트
	current := lg.metricsCollector.GetCurrentMetrics()
	current.CPUUsagePercent = poolMetrics.SystemMetrics.CPUUsagePercent
	current.MemoryUsageMB = poolMetrics.SystemMetrics.MemoryUsageMB
	
	// 간단한 성능 로그 출력 (10초마다)
	if int(time.Since(lg.startTime).Seconds())%10 == 0 {
		lg.printQuickStats(current)
	}
}

// printQuickStats - 간단한 상태 출력
func (lg *LogGenerator) printQuickStats(metrics metrics.PerformanceMetrics) {
	duration := time.Since(lg.startTime)
	profile := lg.workerPool.GetProfile()
	achievement := float64(metrics.CurrentEPS) / float64(profile.TargetEPS) * 100
	
	fmt.Printf("[%s] EPS: %s/%s (%.1f%%) | 워커: %d/%d | CPU: %.1f%% | 메모리: %.0fMB\n",
		duration.Round(time.Second).String(),
		formatNumber(metrics.CurrentEPS),
		formatNumber(int64(profile.TargetEPS)),
		achievement,
		metrics.ActiveWorkers,
		profile.WorkerCount,
		metrics.CPUUsagePercent,
		metrics.MemoryUsageMB)
}

// Stop - 애플리케이션 정지
func (lg *LogGenerator) Stop() error {
	if !lg.isRunning {
		return fmt.Errorf("애플리케이션이 실행되지 않고 있습니다")
	}
	
	fmt.Println()
	fmt.Println("🛑 시스템 종료 시작...")
	
	// 컨텍스트 취소
	lg.cancel()
	
	// 1. 워커 풀 정지
	if lg.workerPool != nil {
		err := lg.workerPool.Stop()
		if err != nil {
			fmt.Printf("⚠️  워커 풀 정지 오류: %v\n", err)
		} else {
			fmt.Println("✅ 워커 풀 정지 완료")
		}
	}
	
	// 2. 대시보드 정지
	if lg.dashboard != nil {
		err := lg.dashboard.Stop()
		if err != nil {
			fmt.Printf("⚠️  대시보드 정지 오류: %v\n", err)
		} else {
			fmt.Println("✅ 대시보드 정지 완료")
		}
	}
	
	// 3. 메트릭 수집 정지
	if lg.metricsCollector != nil {
		lg.metricsCollector.Stop()
		fmt.Println("✅ 메트릭 수집 정지 완료")
	}
	
	// 4. 메모리 최적화 정지
	if lg.memoryOptimizer != nil {
		lg.memoryOptimizer.Stop()
		fmt.Println("✅ 메모리 최적화 정지 완료")
	}
	
	// 5. 최종 성능 리포트 출력
	lg.printFinalReport()
	
	lg.isRunning = false
	return nil
}

// printSystemInfo - 시스템 정보 출력
func (lg *LogGenerator) printSystemInfo() {
	fmt.Printf("🖥️  시스템 정보:\n")
	fmt.Printf("   OS: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("   CPU 코어: %d개\n", runtime.NumCPU())
	fmt.Printf("   Go 버전: %s\n", runtime.Version())
	fmt.Printf("   목표 호스트: %s\n", lg.config.TargetHost)
	profile := lg.workerPool.GetProfile()
	fmt.Printf("   EPS 프로파일: %s (%s)\n", profile.Name, profile.Description)
	fmt.Printf("   워커 수: %d, 배치 크기: %d, 타이머: %dμs\n", 
		profile.WorkerCount, profile.BatchSize, profile.TickerInterval)
	if lg.config.TestDurationMin > 0 {
		fmt.Printf("   테스트 시간: %d분\n", lg.config.TestDurationMin)
	}
	fmt.Println()
}

// printFinalReport - 최종 성능 리포트
func (lg *LogGenerator) printFinalReport() {
	duration := time.Since(lg.startTime)
	finalMetrics := lg.metricsCollector.GetCurrentMetrics()
	
	fmt.Println()
	fmt.Println("📊 최종 성능 리포트")
	fmt.Println("=" + repeatString("=", 60))
	fmt.Printf("   총 실행 시간: %s\n", duration.Round(time.Second))
	fmt.Printf("   총 전송 로그: %s개\n", formatNumber(finalMetrics.TotalSent))
	
	avgEPS := int64(0)
	if duration.Seconds() > 0 {
		avgEPS = int64(float64(finalMetrics.TotalSent) / duration.Seconds())
	}
	fmt.Printf("   평균 EPS: %s\n", formatNumber(avgEPS))
	fmt.Printf("   최종 EPS: %s\n", formatNumber(finalMetrics.CurrentEPS))
	
	achievement := float64(finalMetrics.CurrentEPS) / float64(finalMetrics.TargetEPS) * 100
	fmt.Printf("   목표 달성률: %.1f%%\n", achievement)
	fmt.Printf("   일관성 점수: %.0f/100\n", finalMetrics.ConsistencyScore)
	fmt.Printf("   효율성 점수: %.0f/100\n", finalMetrics.EfficiencyScore)
	fmt.Printf("   패킷 손실률: %.2f%%\n", finalMetrics.PacketLoss)
	profile := lg.workerPool.GetProfile()
	fmt.Printf("   활성 워커: %d/%d\n", finalMetrics.ActiveWorkers, profile.WorkerCount)
	
	// 성과 평가
	if achievement >= 95 {
		fmt.Println("🎉 우수! 목표 달성률 95% 이상")
	} else if achievement >= 80 {
		fmt.Println("👍 양호! 목표 달성률 80% 이상")
	} else if achievement >= 50 {
		fmt.Println("⚠️  개선 필요! 목표 달성률 50% 이상")
	} else {
		fmt.Println("❌ 성능 문제! 시스템 점검 필요")
	}
	
	fmt.Println("=" + repeatString("=", 60))
}

// printWelcomeMessage - 시작 메시지
func printWelcomeMessage() {
	fmt.Println(`
 ██╗      ██████╗  ██████╗      ██████╗ ███████╗███╗   ██╗███████╗██████╗  █████╗ ████████╗ ██████╗ ██████╗ 
 ██║     ██╔═══██╗██╔════╝     ██╔════╝ ██╔════╝████╗  ██║██╔════╝██╔══██╗██╔══██╗╚══██╔══╝██╔═══██╗██╔══██╗
 ██║     ██║   ██║██║  ███╗    ██║  ███╗█████╗  ██╔██╗ ██║█████╗  ██████╔╝███████║   ██║   ██║   ██║██████╔╝
 ██║     ██║   ██║██║   ██║    ██║   ██║██╔══╝  ██║╚██╗██║██╔══╝  ██╔══██╗██╔══██║   ██║   ██║   ██║██╔══██╗
 ███████╗╚██████╔╝╚██████╔╝    ╚██████╔╝███████╗██║ ╚████║███████╗██║  ██║██║  ██║   ██║   ╚██████╔╝██║  ██║
 ╚══════╝ ╚═════╝  ╚═════╝      ╚═════╝ ╚══════╝╚═╝  ╚═══╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚═╝  ╚═╝
`)
	fmt.Println("🚀 시스템 로그 고성능 EPS 전송기")
	fmt.Println("📋 프로파일 기반 SIEM 성능 검증 도구")
	fmt.Println("⚡ 선택 가능한 EPS 프로파일: 100K, 500K, 1M, 2M, 4M, Custom")
	fmt.Println()
}

// 유틸리티 함수들
func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}