package monitor

import (
	"encoding/json"
	"fmt"
	"log-generator/pkg/metrics"
	"net/http"
	"sync"
	"time"
	
	"github.com/gorilla/websocket"
)

// DashboardServer - 400만 EPS 실시간 모니터링 대시보드 서버
type DashboardServer struct {
	port            int
	metricsCollector *metrics.MetricsCollector
	
	// WebSocket 연결 관리
	clients         map[*websocket.Conn]bool
	clientsMutex    sync.RWMutex
	upgrader        websocket.Upgrader
	
	// 브로드캐스트 채널
	broadcast       chan []byte
	
	// HTTP 서버
	httpServer      *http.Server
	
	// 상태 관리
	isRunning       bool
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewDashboardServer - 대시보드 서버 생성
func NewDashboardServer(port int, metricsCollector *metrics.MetricsCollector) *DashboardServer {
	return &DashboardServer{
		port:            port,
		metricsCollector: metricsCollector,
		clients:         make(map[*websocket.Conn]bool),
		broadcast:       make(chan []byte, 100),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 개발용, 프로덕션에서는 제한 필요
			},
		},
		stopChan: make(chan struct{}),
	}
}

// Start - 대시보드 서버 시작
func (ds *DashboardServer) Start() error {
	if ds.isRunning {
		return fmt.Errorf("대시보드 서버가 이미 실행 중입니다")
	}
	
	// HTTP 라우팅 설정
	mux := http.NewServeMux()
	mux.HandleFunc("/", ds.handleDashboard)
	mux.HandleFunc("/ws", ds.handleWebSocket)
	mux.HandleFunc("/api/metrics", ds.handleAPIMetrics)
	mux.HandleFunc("/api/summary", ds.handleAPISummary)
	mux.HandleFunc("/static/", ds.handleStatic)
	
	// HTTP 서버 설정
	ds.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", ds.port),
		Handler: mux,
	}
	
	// 백그라운드 작업 시작
	ds.wg.Add(1)
	go ds.broadcastLoop()
	
	ds.wg.Add(1)
	go ds.metricsStreamer()
	
	ds.isRunning = true
	
	fmt.Printf("🌐 대시보드 서버 시작: http://localhost:%d\n", ds.port)
	
	// HTTP 서버 시작
	go func() {
		if err := ds.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("대시보드 서버 오류: %v\n", err)
		}
	}()
	
	return nil
}

// handleDashboard - 대시보드 메인 페이지
func (ds *DashboardServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	html := ds.generateDashboardHTML()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// handleWebSocket - WebSocket 연결 처리
func (ds *DashboardServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ds.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket 업그레이드 실패: %v\n", err)
		return
	}
	
	// 클라이언트 등록
	ds.clientsMutex.Lock()
	ds.clients[conn] = true
	ds.clientsMutex.Unlock()
	
	fmt.Printf("새 클라이언트 연결: %s\n", conn.RemoteAddr())
	
	// 연결 해제 처리
	defer func() {
		ds.clientsMutex.Lock()
		delete(ds.clients, conn)
		ds.clientsMutex.Unlock()
		conn.Close()
		fmt.Printf("클라이언트 연결 해제: %s\n", conn.RemoteAddr())
	}()
	
	// 초기 메트릭 전송
	initialMetrics := ds.metricsCollector.GetCurrentMetrics()
	initialData, _ := json.Marshal(initialMetrics)
	conn.WriteMessage(websocket.TextMessage, initialData)
	
	// 연결 유지를 위한 핑-퐁 처리
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// handleAPIMetrics - 메트릭 API
func (ds *DashboardServer) handleAPIMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	metrics := ds.metricsCollector.GetCurrentMetrics()
	json.NewEncoder(w).Encode(metrics)
}

// handleAPISummary - 요약 API
func (ds *DashboardServer) handleAPISummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	summary := ds.metricsCollector.GetSummaryReport()
	json.NewEncoder(w).Encode(summary)
}

// handleStatic - 정적 파일 처리 (향후 확장용)
func (ds *DashboardServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// broadcastLoop - 클라이언트들에게 메트릭 브로드캐스트
func (ds *DashboardServer) broadcastLoop() {
	defer ds.wg.Done()
	
	for {
		select {
		case <-ds.stopChan:
			return
		case message := <-ds.broadcast:
			ds.clientsMutex.RLock()
			for client := range ds.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					// 연결 오류시 클라이언트 제거
					delete(ds.clients, client)
					client.Close()
				}
			}
			ds.clientsMutex.RUnlock()
		}
	}
}

// metricsStreamer - 메트릭 스트리밍
func (ds *DashboardServer) metricsStreamer() {
	defer ds.wg.Done()
	
	ticker := time.NewTicker(time.Second) // 1초마다 업데이트
	defer ticker.Stop()
	
	for {
		select {
		case <-ds.stopChan:
			return
		case <-ticker.C:
			metrics := ds.metricsCollector.GetCurrentMetrics()
			data, err := json.Marshal(metrics)
			if err != nil {
				continue
			}
			
			select {
			case ds.broadcast <- data:
			default:
				// 브로드캐스트 채널이 가득 찬 경우 건너뜀
			}
		}
	}
}

// Stop - 대시보드 서버 중지
func (ds *DashboardServer) Stop() error {
	if !ds.isRunning {
		return fmt.Errorf("대시보드 서버가 실행되지 않고 있습니다")
	}
	
	ds.isRunning = false
	
	// HTTP 서버 중지
	if ds.httpServer != nil {
		ds.httpServer.Close()
	}
	
	// 백그라운드 작업 중지
	close(ds.stopChan)
	ds.wg.Wait()
	
	// 모든 WebSocket 연결 해제
	ds.clientsMutex.Lock()
	for client := range ds.clients {
		client.Close()
	}
	ds.clients = make(map[*websocket.Conn]bool)
	ds.clientsMutex.Unlock()
	
	fmt.Println("대시보드 서버 중지 완료")
	return nil
}

// generateDashboardHTML - 대시보드 HTML 생성
func (ds *DashboardServer) generateDashboardHTML() string {
	return `<!DOCTYPE html>
<html lang="ko">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>시스템 로그 400만 EPS 실시간 모니터링</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: linear-gradient(135deg, #1a1a2e, #16213e);
            color: #ffffff;
            min-height: 100vh;
            overflow-x: hidden;
        }
        
        .header {
            background: rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
            padding: 20px;
            text-align: center;
            border-bottom: 1px solid rgba(255, 255, 255, 0.2);
        }
        
        .header h1 {
            font-size: 2.5em;
            font-weight: 300;
            margin-bottom: 10px;
            color: #00d4ff;
        }
        
        .header .subtitle {
            font-size: 1.2em;
            opacity: 0.8;
        }
        
        .dashboard {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            padding: 20px;
            max-width: 1400px;
            margin: 0 auto;
        }
        
        .metric-card {
            background: rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
            border-radius: 15px;
            padding: 25px;
            border: 1px solid rgba(255, 255, 255, 0.2);
            transition: transform 0.3s ease, box-shadow 0.3s ease;
        }
        
        .metric-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 10px 30px rgba(0, 212, 255, 0.3);
        }
        
        .metric-card h3 {
            font-size: 1.3em;
            margin-bottom: 15px;
            color: #00d4ff;
            display: flex;
            align-items: center;
        }
        
        .metric-card .icon {
            font-size: 1.5em;
            margin-right: 10px;
        }
        
        .metric-value {
            font-size: 2.5em;
            font-weight: bold;
            margin-bottom: 10px;
            text-align: center;
        }
        
        .metric-target {
            font-size: 1em;
            opacity: 0.7;
            text-align: center;
        }
        
        .progress-bar {
            width: 100%;
            height: 10px;
            background: rgba(255, 255, 255, 0.2);
            border-radius: 5px;
            overflow: hidden;
            margin: 15px 0;
        }
        
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #00d4ff, #0099cc);
            transition: width 0.5s ease;
            border-radius: 5px;
        }
        
        .worker-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(60px, 1fr));
            gap: 5px;
            margin-top: 15px;
        }
        
        .worker-status {
            padding: 8px;
            border-radius: 5px;
            text-align: center;
            font-size: 0.9em;
            font-weight: bold;
            transition: all 0.3s ease;
        }
        
        .worker-active {
            background: linear-gradient(135deg, #00ff88, #00cc66);
            color: #000;
        }
        
        .worker-inactive {
            background: rgba(255, 255, 255, 0.2);
            color: #999;
        }
        
        .status-indicator {
            display: inline-block;
            width: 12px;
            height: 12px;
            border-radius: 50%;
            margin-right: 8px;
            animation: pulse 2s infinite;
        }
        
        .status-good { background: #00ff88; }
        .status-warning { background: #ffaa00; }
        .status-error { background: #ff4444; }
        
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
        
        .chart-container {
            grid-column: 1 / -1;
            height: 300px;
            background: rgba(255, 255, 255, 0.05);
            border-radius: 15px;
            padding: 20px;
            position: relative;
        }
        
        .connection-status {
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 10px 15px;
            border-radius: 20px;
            background: rgba(0, 255, 136, 0.2);
            border: 1px solid #00ff88;
            font-size: 0.9em;
        }
        
        .connection-disconnected {
            background: rgba(255, 68, 68, 0.2);
            border-color: #ff4444;
        }
        
        @media (max-width: 768px) {
            .dashboard {
                grid-template-columns: 1fr;
                padding: 10px;
            }
            
            .header h1 {
                font-size: 2em;
            }
            
            .metric-value {
                font-size: 2em;
            }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚀 시스템 로그 400만 EPS 모니터링</h1>
        <div class="subtitle">실시간 고성능 로그 전송기 대시보드</div>
    </div>
    
    <div class="connection-status" id="connectionStatus">
        <span class="status-indicator status-good"></span>
        연결됨
    </div>
    
    <div class="dashboard">
        <div class="metric-card">
            <h3><span class="icon">⚡</span>현재 EPS</h3>
            <div class="metric-value" id="currentEPS">0</div>
            <div class="metric-target">목표: 4,000,000 EPS</div>
            <div class="progress-bar">
                <div class="progress-fill" id="epsProgress" style="width: 0%"></div>
            </div>
        </div>
        
        <div class="metric-card">
            <h3><span class="icon">📊</span>달성률</h3>
            <div class="metric-value" id="achievementPercent">0%</div>
            <div class="metric-target">목표 대비 성과</div>
        </div>
        
        <div class="metric-card">
            <h3><span class="icon">📈</span>총 전송량</h3>
            <div class="metric-value" id="totalSent">0</div>
            <div class="metric-target">누적 로그 수</div>
        </div>
        
        <div class="metric-card">
            <h3><span class="icon">⏱️</span>실행 시간</h3>
            <div class="metric-value" id="uptime">00:00:00</div>
            <div class="metric-target">연속 실행 시간</div>
        </div>
        
        <div class="metric-card">
            <h3><span class="icon">🎯</span>일관성 점수</h3>
            <div class="metric-value" id="consistencyScore">100</div>
            <div class="metric-target">성능 안정성 지수</div>
        </div>
        
        <div class="metric-card">
            <h3><span class="icon">⚙️</span>효율성 점수</h3>
            <div class="metric-value" id="efficiencyScore">100</div>
            <div class="metric-target">리소스 활용 효율성</div>
        </div>
        
        <div class="metric-card">
            <h3><span class="icon">💻</span>시스템 리소스</h3>
            <div style="display: flex; justify-content: space-between; margin: 10px 0;">
                <span>CPU:</span>
                <span id="cpuUsage">0%</span>
            </div>
            <div style="display: flex; justify-content: space-between; margin: 10px 0;">
                <span>메모리:</span>
                <span id="memoryUsage">0 MB</span>
            </div>
            <div style="display: flex; justify-content: space-between; margin: 10px 0;">
                <span>고루틴:</span>
                <span id="goroutineCount">0</span>
            </div>
        </div>
        
        <div class="metric-card">
            <h3><span class="icon">📡</span>패킷 손실률</h3>
            <div class="metric-value" id="packetLoss">0.00%</div>
            <div class="metric-target">목표: < 0.5%</div>
        </div>
        
        <div class="metric-card" style="grid-column: 1 / -1;">
            <h3><span class="icon">🔧</span>워커 상태 (40개)</h3>
            <div style="display: flex; justify-content: space-between; margin: 10px 0;">
                <span>활성 워커: <span id="activeWorkers">0</span>/40</span>
                <span>포트 범위: 514-553</span>
            </div>
            <div class="worker-grid" id="workerGrid">
                <!-- 워커 상태가 여기에 동적으로 생성됩니다 -->
            </div>
        </div>
    </div>
    
    <script>
        class Dashboard {
            constructor() {
                this.ws = null;
                this.reconnectInterval = 5000;
                this.isConnected = false;
                this.connectWebSocket();
                this.initializeWorkerGrid();
            }
            
            connectWebSocket() {
                const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
                const wsUrl = protocol + '//' + window.location.host + '/ws';
                
                try {
                    this.ws = new WebSocket(wsUrl);
                    
                    this.ws.onopen = () => {
                        console.log('WebSocket 연결됨');
                        this.isConnected = true;
                        this.updateConnectionStatus();
                    };
                    
                    this.ws.onmessage = (event) => {
                        const metrics = JSON.parse(event.data);
                        this.updateDashboard(metrics);
                    };
                    
                    this.ws.onclose = () => {
                        console.log('WebSocket 연결 해제');
                        this.isConnected = false;
                        this.updateConnectionStatus();
                        setTimeout(() => this.connectWebSocket(), this.reconnectInterval);
                    };
                    
                    this.ws.onerror = (error) => {
                        console.error('WebSocket 오류:', error);
                        this.isConnected = false;
                        this.updateConnectionStatus();
                    };
                } catch (error) {
                    console.error('WebSocket 연결 실패:', error);
                    setTimeout(() => this.connectWebSocket(), this.reconnectInterval);
                }
            }
            
            updateConnectionStatus() {
                const status = document.getElementById('connectionStatus');
                const indicator = status.querySelector('.status-indicator');
                
                if (this.isConnected) {
                    status.className = 'connection-status';
                    indicator.className = 'status-indicator status-good';
                    status.innerHTML = '<span class="status-indicator status-good"></span>연결됨';
                } else {
                    status.className = 'connection-status connection-disconnected';
                    indicator.className = 'status-indicator status-error';
                    status.innerHTML = '<span class="status-indicator status-error"></span>연결 끊어짐';
                }
            }
            
            initializeWorkerGrid() {
                const grid = document.getElementById('workerGrid');
                for (let i = 1; i <= 40; i++) {
                    const worker = document.createElement('div');
                    worker.className = 'worker-status worker-inactive';
                    worker.id = 'worker-' + i;
                    worker.textContent = i;
                    worker.title = '워커 ' + i + ' (포트 ' + (513 + i) + ')';
                    grid.appendChild(worker);
                }
            }
            
            updateDashboard(metrics) {
                // 핵심 메트릭 업데이트
                document.getElementById('currentEPS').textContent = 
                    this.formatNumber(metrics.current_eps || 0);
                
                document.getElementById('achievementPercent').textContent = 
                    (metrics.achievement_percent || 0).toFixed(1) + '%';
                
                document.getElementById('totalSent').textContent = 
                    this.formatNumber(metrics.total_sent || 0);
                
                document.getElementById('uptime').textContent = 
                    this.formatUptime(metrics.uptime_seconds || 0);
                
                document.getElementById('consistencyScore').textContent = 
                    (metrics.consistency_score || 0).toFixed(0);
                
                document.getElementById('efficiencyScore').textContent = 
                    (metrics.efficiency_score || 0).toFixed(0);
                
                document.getElementById('packetLoss').textContent = 
                    (metrics.packet_loss || 0).toFixed(2) + '%';
                
                // 시스템 리소스 업데이트
                if (metrics.system_metrics) {
                    document.getElementById('cpuUsage').textContent = 
                        (metrics.system_metrics.cpu_usage_percent || 0).toFixed(1) + '%';
                    
                    document.getElementById('memoryUsage').textContent = 
                        this.formatBytes(metrics.system_metrics.memory_usage_mb * 1024 * 1024 || 0);
                    
                    document.getElementById('goroutineCount').textContent = 
                        metrics.system_metrics.goroutine_count || 0;
                }
                
                // 진행률 바 업데이트
                const progressPercent = Math.min(100, (metrics.achievement_percent || 0));
                document.getElementById('epsProgress').style.width = progressPercent + '%';
                
                // 워커 상태 업데이트
                document.getElementById('activeWorkers').textContent = 
                    metrics.active_workers || 0;
                
                if (metrics.worker_details) {
                    this.updateWorkerGrid(metrics.worker_details);
                }
            }
            
            updateWorkerGrid(workerDetails) {
                for (let i = 1; i <= 40; i++) {
                    const workerElement = document.getElementById('worker-' + i);
                    const worker = workerDetails.find(w => w.worker_id === i);
                    
                    if (worker && worker.is_active) {
                        workerElement.className = 'worker-status worker-active';
                        workerElement.title = 
                            '워커 ' + i + ' (포트 ' + worker.port + ')\n' +
                            'EPS: ' + this.formatNumber(worker.current_eps) + '\n' +
                            '전송: ' + this.formatNumber(worker.total_sent);
                    } else {
                        workerElement.className = 'worker-status worker-inactive';
                        workerElement.title = '워커 ' + i + ' (비활성)';
                    }
                }
            }
            
            formatNumber(num) {
                if (num >= 1000000) {
                    return (num / 1000000).toFixed(2) + 'M';
                } else if (num >= 1000) {
                    return (num / 1000).toFixed(1) + 'K';
                }
                return num.toString();
            }
            
            formatBytes(bytes) {
                if (bytes >= 1024 * 1024 * 1024) {
                    return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
                } else if (bytes >= 1024 * 1024) {
                    return (bytes / (1024 * 1024)).toFixed(0) + ' MB';
                } else if (bytes >= 1024) {
                    return (bytes / 1024).toFixed(0) + ' KB';
                }
                return bytes + ' B';
            }
            
            formatUptime(seconds) {
                const hours = Math.floor(seconds / 3600);
                const minutes = Math.floor((seconds % 3600) / 60);
                const secs = Math.floor(seconds % 60);
                
                return String(hours).padStart(2, '0') + ':' + 
                       String(minutes).padStart(2, '0') + ':' + 
                       String(secs).padStart(2, '0');
            }
        }
        
        // 대시보드 초기화
        document.addEventListener('DOMContentLoaded', () => {
            new Dashboard();
        });
    </script>
</body>
</html>`
}