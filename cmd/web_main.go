package main

import (
	"flag"
	"fmt"
	"log-generator/internal/monitor"
	"os"
	"os/signal"
	"syscall"
)

// WebApp - 웹 기반 로그 생성기 애플리케이션
type WebApp struct {
	controlServer *monitor.ControlServer
	port          int
}

func main() {
	// 명령행 파라미터 파싱
	var port int
	flag.IntVar(&port, "port", 8080, "웹 서버 포트")
	flag.Parse()

	fmt.Println(`
 ██╗     ██╗███████╗██████╗      ██████╗ ███████╗███╗   ██╗███████╗██████╗  █████╗ ████████╗ ██████╗ ██████╗ 
 ██║     ██║██╔════╝██╔══██╗    ██╔════╝ ██╔════╝████╗  ██║██╔════╝██╔══██╗██╔══██╗╚══██╔══╝██╔═══██╗██╔══██╗
 ██║  █╗ ██║█████╗  ██████╔╝    ██║  ███╗█████╗  ██╔██╗ ██║█████╗  ██████╔╝███████║   ██║   ██║   ██║██████╔╝
 ██║ ███╗██║██╔══╝  ██╔══██╗    ██║   ██║██╔══╝  ██║╚██╗██║██╔══╝  ██╔══██╗██╔══██║   ██║   ██║   ██║██╔══██╗
 ╚███╔███╔╝███████╗██████╔╝    ╚██████╔╝███████╗██║ ╚████║███████╗██║  ██║██║  ██║   ██║   ╚██████╔╝██║  ██║
  ╚══╝╚══╝ ╚══════╝╚═════╝      ╚═════╝ ╚══════╝╚═╝  ╚═══╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚═╝  ╚═╝
`)
	fmt.Println("🌐 웹 기반 400만 EPS 로그 생성기 제어 시스템")
	fmt.Println("📊 완전한 웹 UI로 설정/실행/모니터링 통합 관리")
	fmt.Println()

	// 웹 애플리케이션 생성
	app := &WebApp{
		controlServer: monitor.NewControlServer(port),
		port:          port,
	}

	// 서버 시작
	err := app.Start()
	if err != nil {
		fmt.Printf("❌ 웹 애플리케이션 시작 실패: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 웹 제어 시스템이 시작되었습니다!\n")
	fmt.Printf("🌐 브라우저에서 접속: http://localhost:%d\n", port)
	fmt.Printf("📱 간편 제어 UI: http://localhost:%d/control\n", port)
	fmt.Printf("🔧 API 문서: http://localhost:%d/api/\n", port)
	fmt.Println()
	fmt.Println("🎛️  웹 UI 기능:")
	fmt.Println("   ✓ 실시간 설정 변경")
	fmt.Println("   ✓ 원클릭 시작/정지/재시작")
	fmt.Println("   ✓ 실시간 성능 모니터링")
	fmt.Println("   ✓ 워커 상태 시각화")
	fmt.Println("   ✓ 시스템 로그 실시간 표시")
	fmt.Println("   ✓ 고급 설정 및 최적화")
	fmt.Println()

	// 시그널 핸들링
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 종료 신호 대기
	<-sigChan
	fmt.Println("\n🛑 종료 신호 수신, 웹 애플리케이션 종료 중...")

	// 애플리케이션 정지
	err = app.Stop()
	if err != nil {
		fmt.Printf("⚠️  웹 애플리케이션 정지 중 오류: %v\n", err)
	}

	fmt.Println("✅ 웹 애플리케이션 정상 종료")
}

// Start - 웹 애플리케이션 시작
func (wa *WebApp) Start() error {
	return wa.controlServer.Start()
}

// Stop - 웹 애플리케이션 정지
func (wa *WebApp) Stop() error {
	return wa.controlServer.Stop()
}