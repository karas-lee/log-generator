package monitor

// generateControlUI - 완전한 제어 대시보드 UI 생성
func (cs *ControlServer) generateControlUI() string {
	return `<!DOCTYPE html>
<html lang="ko">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>로그 생성기 제어 대시보드</title>
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
        }
        
        .header {
            background: rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
            padding: 20px;
            text-align: center;
            border-bottom: 1px solid rgba(255, 255, 255, 0.2);
        }
        
        .header h1 {
            font-size: 2.2em;
            font-weight: 300;
            color: #00d4ff;
            margin-bottom: 5px;
        }
        
        .status-bar {
            display: flex;
            justify-content: space-between;
            align-items: center;
            background: rgba(255, 255, 255, 0.05);
            padding: 15px 20px;
            border-bottom: 1px solid rgba(255, 255, 255, 0.1);
        }
        
        .status-indicator {
            display: flex;
            align-items: center;
            gap: 10px;
        }
        
        .status-dot {
            width: 12px;
            height: 12px;
            border-radius: 50%;
            animation: pulse 2s infinite;
        }
        
        .status-running { background: #00ff88; }
        .status-stopped { background: #ff4444; }
        .status-starting { background: #ffaa00; }
        
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
        
        .main-content {
            display: grid;
            grid-template-columns: 350px 1fr;
            gap: 20px;
            padding: 20px;
            max-width: 1600px;
            margin: 0 auto;
        }
        
        .control-panel {
            background: rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
            border-radius: 15px;
            padding: 25px;
            border: 1px solid rgba(255, 255, 255, 0.2);
            height: fit-content;
        }
        
        .monitoring-area {
            display: grid;
            gap: 20px;
        }
        
        .section-title {
            font-size: 1.3em;
            color: #00d4ff;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 2px solid #00d4ff;
        }
        
        .control-buttons {
            display: grid;
            gap: 10px;
            margin-bottom: 30px;
        }
        
        .btn {
            padding: 12px 20px;
            border: none;
            border-radius: 8px;
            font-size: 1em;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s ease;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        
        .btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }
        
        .btn-start {
            background: linear-gradient(135deg, #00ff88, #00cc66);
            color: #000;
        }
        
        .btn-start:hover:not(:disabled) {
            transform: translateY(-2px);
            box-shadow: 0 5px 15px rgba(0, 255, 136, 0.4);
        }
        
        .btn-stop {
            background: linear-gradient(135deg, #ff4444, #cc2222);
            color: #fff;
        }
        
        .btn-stop:hover:not(:disabled) {
            transform: translateY(-2px);
            box-shadow: 0 5px 15px rgba(255, 68, 68, 0.4);
        }
        
        .btn-restart {
            background: linear-gradient(135deg, #ffaa00, #cc8800);
            color: #000;
        }
        
        .btn-restart:hover:not(:disabled) {
            transform: translateY(-2px);
            box-shadow: 0 5px 15px rgba(255, 170, 0, 0.4);
        }
        
        .config-form {
            display: grid;
            gap: 15px;
        }
        
        .form-group {
            display: flex;
            flex-direction: column;
            gap: 5px;
        }
        
        .form-group label {
            font-size: 0.9em;
            color: #ccc;
            font-weight: 500;
        }
        
        .form-group input,
        .form-group select {
            padding: 10px;
            border: 1px solid rgba(255, 255, 255, 0.3);
            border-radius: 5px;
            background: rgba(255, 255, 255, 0.1);
            color: #fff;
            font-size: 1em;
        }
        
        .form-group input:focus,
        .form-group select:focus {
            outline: none;
            border-color: #00d4ff;
            box-shadow: 0 0 0 2px rgba(0, 212, 255, 0.2);
        }
        
        .checkbox-group {
            display: flex;
            align-items: center;
            gap: 10px;
        }
        
        .checkbox-group input[type="checkbox"] {
            width: 18px;
            height: 18px;
        }
        
        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 15px;
        }
        
        .metric-card {
            background: rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
            border-radius: 10px;
            padding: 20px;
            border: 1px solid rgba(255, 255, 255, 0.2);
            text-align: center;
        }
        
        .metric-card h4 {
            font-size: 0.9em;
            color: #ccc;
            margin-bottom: 10px;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        
        .metric-value {
            font-size: 2em;
            font-weight: bold;
            color: #00d4ff;
        }
        
        .worker-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(50px, 1fr));
            gap: 8px;
            margin-top: 15px;
        }
        
        .worker-status {
            padding: 10px 8px;
            border-radius: 5px;
            text-align: center;
            font-size: 0.8em;
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
        
        .logs-panel {
            background: rgba(0, 0, 0, 0.5);
            border-radius: 10px;
            padding: 20px;
            font-family: 'Courier New', monospace;
            font-size: 0.9em;
            max-height: 300px;
            overflow-y: auto;
            border: 1px solid rgba(255, 255, 255, 0.2);
        }
        
        .log-entry {
            padding: 5px 0;
            border-bottom: 1px solid rgba(255, 255, 255, 0.1);
        }
        
        .log-timestamp {
            color: #00d4ff;
        }
        
        .log-info { color: #00ff88; }
        .log-warning { color: #ffaa00; }
        .log-error { color: #ff4444; }
        
        .notification {
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 15px 20px;
            border-radius: 8px;
            font-weight: 500;
            z-index: 1000;
            animation: slideIn 0.3s ease;
            max-width: 300px;
        }
        
        .notification.success {
            background: linear-gradient(135deg, #00ff88, #00cc66);
            color: #000;
        }
        
        .notification.error {
            background: linear-gradient(135deg, #ff4444, #cc2222);
            color: #fff;
        }
        
        @keyframes slideIn {
            from {
                transform: translateX(100%);
                opacity: 0;
            }
            to {
                transform: translateX(0);
                opacity: 1;
            }
        }
        
        @media (max-width: 1024px) {
            .main-content {
                grid-template-columns: 1fr;
            }
            
            .metrics-grid {
                grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            }
        }
        
        .advanced-toggle {
            cursor: pointer;
            padding: 10px;
            background: rgba(255, 255, 255, 0.1);
            border-radius: 5px;
            margin: 15px 0;
            text-align: center;
            transition: background 0.3s ease;
        }
        
        .advanced-toggle:hover {
            background: rgba(255, 255, 255, 0.2);
        }
        
        .advanced-config {
            display: none;
            animation: fadeIn 0.3s ease;
        }
        
        .advanced-config.show {
            display: block;
        }
        
        @keyframes fadeIn {
            from { opacity: 0; }
            to { opacity: 1; }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚀 로그 생성기 제어 대시보드</h1>
        <div>프로파일 기반 고성능 EPS 로그 전송기 - 웹 기반 완전 제어</div>
    </div>
    
    <div class="status-bar">
        <div class="status-indicator">
            <div class="status-dot status-stopped" id="statusDot"></div>
            <span id="statusText">정지됨</span>
        </div>
        <div id="uptimeDisplay">업타임: 00:00:00</div>
        <div>
            <button class="btn btn-start" id="systemOptimizeBtn" onclick="optimizeSystem()">
                시스템 최적화
            </button>
        </div>
    </div>
    
    <div class="main-content">
        <!-- 제어 패널 -->
        <div class="control-panel">
            <h2 class="section-title">🎛️ 제어 패널</h2>
            
            <!-- 제어 버튼 -->
            <div class="control-buttons">
                <button class="btn btn-start" id="startBtn" onclick="startGenerator()">
                    ▶️ 시작
                </button>
                <button class="btn btn-stop" id="stopBtn" onclick="stopGenerator()" disabled>
                    ⏹️ 정지
                </button>
                <button class="btn btn-restart" id="restartBtn" onclick="restartGenerator()" disabled>
                    🔄 재시작
                </button>
            </div>
            
            <!-- 기본 설정 -->
            <h3 class="section-title">⚙️ 기본 설정</h3>
            <form class="config-form" id="configForm">
                <div class="form-group">
                    <label>목표 호스트</label>
                    <input type="text" id="targetHost" value="127.0.0.1" placeholder="SIEM 서버 주소">
                </div>
                
                <div class="form-group">
                    <label>EPS 프로파일</label>
                    <select id="epsProfile" onchange="onProfileChange()">
                        <option value="100k">100K EPS (가벼운 부하)</option>
                        <option value="500k">500K EPS (중간 부하)</option>
                        <option value="1m">1M EPS (표준 부하)</option>
                        <option value="2m">2M EPS (높은 부하)</option>
                        <option value="4m" selected>4M EPS (최대 부하)</option>
                        <option value="custom">Custom (사용자 정의)</option>
                    </select>
                </div>
                
                <div class="form-group" id="customEPSGroup" style="display:none;">
                    <label>커스텀 목표 EPS</label>
                    <input type="number" id="targetEPS" value="4000000" min="1000" max="10000000" step="1000">
                </div>
                
                <div class="form-group">
                    <label>실행 시간 (분, 0=무제한)</label>
                    <input type="number" id="duration" value="0" min="0" max="1440">
                </div>
                
                <div class="form-group">
                    <label>워커 수 <span id="workerRecommend" style="color: #00d4ff; font-size: 0.9em;">(고정: 160개)</span></label>
                    <input type="number" id="workerCount" value="160" min="1" max="200" readonly>
                </div>
                
                <div class="checkbox-group">
                    <input type="checkbox" id="enableOptimization" checked>
                    <label for="enableOptimization">메모리 최적화</label>
                </div>
                
                <div class="checkbox-group">
                    <input type="checkbox" id="enableDashboard" checked>
                    <label for="enableDashboard">대시보드</label>
                </div>
            </form>
            
            <!-- 고급 설정 -->
            <div class="advanced-toggle" onclick="toggleAdvanced()">
                🔧 고급 설정 표시/숨김
            </div>
            
            <div class="advanced-config" id="advancedConfig">
                <div class="form-group">
                    <label>배치 크기</label>
                    <input type="number" id="batchSize" value="1000" min="100" max="10000">
                </div>
                
                <div class="form-group">
                    <label>전송 간격 (ms)</label>
                    <input type="number" id="sendInterval" value="10" min="1" max="1000">
                </div>
                
                <div class="form-group">
                    <label>메모리 제한 (GB)</label>
                    <input type="number" id="memoryLimit" value="12" min="1" max="64">
                </div>
                
                <div class="form-group">
                    <label>GC 퍼센트</label>
                    <input type="number" id="gcPercent" value="200" min="50" max="500">
                </div>
                
                <div class="form-group">
                    <label>호스트명 접두사</label>
                    <input type="text" id="hostnamePrefix" value="server" placeholder="server">
                </div>
            </div>
            
            <button class="btn" onclick="saveConfig()" style="background: #00d4ff; color: #000; margin-top: 15px;">
                💾 설정 저장
            </button>
        </div>
        
        <!-- 모니터링 영역 -->
        <div class="monitoring-area">
            <!-- 성능 메트릭 -->
            <div>
                <h2 class="section-title">📊 실시간 성능</h2>
                <div class="metrics-grid">
                    <div class="metric-card">
                        <h4>현재 EPS</h4>
                        <div class="metric-value" id="currentEPS">0</div>
                    </div>
                    <div class="metric-card">
                        <h4>달성률</h4>
                        <div class="metric-value" id="achievementRate">0%</div>
                    </div>
                    <div class="metric-card">
                        <h4>총 전송 로그</h4>
                        <div class="metric-value" id="totalSent">0</div>
                    </div>
                    <div class="metric-card">
                        <h4>활성 워커</h4>
                        <div class="metric-value" id="activeWorkers">0/0</div>
                    </div>
                    <div class="metric-card">
                        <h4>CPU 사용률</h4>
                        <div class="metric-value" id="cpuUsage">0%</div>
                    </div>
                    <div class="metric-card">
                        <h4>메모리</h4>
                        <div class="metric-value" id="memoryUsage">0MB</div>
                    </div>
                    <div class="metric-card">
                        <h4>총 전송 패킷</h4>
                        <div class="metric-value" id="txPackets">0</div>
                    </div>
                    <div class="metric-card">
                        <h4>TX 속도</h4>
                        <div class="metric-value" id="txRate">0 Mbps</div>
                    </div>
                    <div class="metric-card">
                        <h4>현재 패킷/초</h4>
                        <div class="metric-value" id="txPacketsPerSec">0</div>
                    </div>
                    <div class="metric-card">
                        <h4>총 전송 데이터</h4>
                        <div class="metric-value" id="networkThroughput">0 MB</div>
                    </div>
                </div>
            </div>
            
            <!-- 워커 상태 -->
            <div>
                <h2 class="section-title">🔧 워커 상태 (<span id="workerStatusTitle">0</span>개)</h2>
                <div class="worker-grid" id="workerGrid">
                    <!-- 워커 상태가 동적으로 생성됩니다 -->
                </div>
            </div>
            
            <!-- 시스템 로그 -->
            <div>
                <h2 class="section-title">📝 시스템 로그</h2>
                <div class="logs-panel" id="logsPanel">
                    <div class="log-entry">
                        <span class="log-timestamp">[시스템]</span>
                        <span class="log-info">로그 생성기 제어 시스템이 준비되었습니다.</span>
                    </div>
                </div>
            </div>
        </div>
    </div>
    
    <script>
        class LogGeneratorController {
            constructor() {
                this.isRunning = false;
                this.startTime = null;
                this.ws = null;
                this.pollInterval = null;
                
                this.initializeUI();
                this.connectWebSocket();
                this.startPolling();
                this.initializeWorkerGrid();
            }
            
            getProfileSettings() {
                return {
                    '100k': {
                        workers: 10,     // 워커당 10K EPS
                        batchSize: 100,  // 100 logs * 100회/초
                        sendInterval: 10,// 10ms
                        memoryLimit: 2,
                        gcPercent: 100
                    },
                    '500k': {
                        workers: 50,     // 워커당 10K EPS
                        batchSize: 100,  // 100 logs * 100회/초
                        sendInterval: 10,// 10ms
                        memoryLimit: 4,
                        gcPercent: 150
                    },
                    '1m': {
                        workers: 100,    // 워커당 10K EPS
                        batchSize: 100,  // 100 logs * 100회/초
                        sendInterval: 10,// 10ms
                        memoryLimit: 6,
                        gcPercent: 200
                    },
                    '2m': {
                        workers: 100,    // 워커당 20K EPS
                        batchSize: 200,  // 200 logs * 100회/초
                        sendInterval: 10,// 10ms
                        memoryLimit: 8,
                        gcPercent: 200
                    },
                    '4m': {
                        workers: 200,    // 워커당 20K EPS
                        batchSize: 200,  // 200 logs * 100회/초
                        sendInterval: 10,// 10ms
                        memoryLimit: 12,
                        gcPercent: 200
                    },
                    'custom': {
                        workers: 0,
                        batchSize: 100,
                        sendInterval: 50,
                        memoryLimit: 8,
                        gcPercent: 200
                    }
                };
            }
            
            initializeUI() {
                this.loadConfig();
                this.updateUI();
            }
            
            initializeWorkerGrid(workerCount) {
                const grid = document.getElementById('workerGrid');
                grid.innerHTML = '';
                
                // 프로파일에 따른 동적 워커 수
                if (!workerCount) {
                    // 현재 선택된 프로파일에서 워커 수 가져오기
                    const profile = document.getElementById('epsProfile').value;
                    const profileSettings = this.getProfileSettings();
                    const settings = profileSettings[profile] || profileSettings['4m'];
                    workerCount = settings.workers;
                }
                
                for (let i = 1; i <= workerCount; i++) {
                    const worker = document.createElement('div');
                    worker.className = 'worker-status worker-inactive';
                    worker.id = 'worker-' + i;
                    worker.textContent = i;
                    worker.title = '워커 ' + i + ' (포트 ' + (513 + i) + ')';
                    grid.appendChild(worker);
                }
                
                // 워커 상태 타이틀 업데이트
                const workerStatusTitle = document.getElementById('workerStatusTitle');
                if (workerStatusTitle) {
                    workerStatusTitle.textContent = workerCount;
                }
            }
            
            connectWebSocket() {
                const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
                const wsUrl = protocol + '//' + window.location.host + '/ws';
                
                try {
                    this.ws = new WebSocket(wsUrl);
                    
                    this.ws.onopen = () => {
                        this.addLog('WebSocket 연결됨', 'info');
                    };
                    
                    this.ws.onmessage = (event) => {
                        const metrics = JSON.parse(event.data);
                        this.updateMetrics(metrics);
                    };
                    
                    this.ws.onclose = () => {
                        this.addLog('WebSocket 연결 해제됨', 'warning');
                        setTimeout(() => this.connectWebSocket(), 5000);
                    };
                    
                    this.ws.onerror = (error) => {
                        this.addLog('WebSocket 오류: ' + error, 'error');
                    };
                } catch (error) {
                    this.addLog('WebSocket 연결 실패: ' + error, 'error');
                }
            }
            
            startPolling() {
                this.pollInterval = setInterval(() => {
                    this.checkStatus();
                }, 2000);
            }
            
            async checkStatus() {
                try {
                    const response = await fetch('/api/status');
                    const result = await response.json();
                    
                    if (result.success) {
                        this.updateStatus(result.data);
                    }
                } catch (error) {
                    console.error('상태 확인 오류:', error);
                }
            }
            
            updateStatus(status) {
                this.isRunning = status.is_running;
                
                // 상태 표시기 업데이트
                const statusDot = document.getElementById('statusDot');
                const statusText = document.getElementById('statusText');
                
                if (this.isRunning) {
                    statusDot.className = 'status-dot status-running';
                    statusText.textContent = '실행 중';
                    this.startTime = status.start_time ? new Date(status.start_time) : new Date();
                } else {
                    statusDot.className = 'status-dot status-stopped';
                    statusText.textContent = '정지됨';
                    this.startTime = null;
                }
                
                // 업타임 업데이트
                if (status.uptime_seconds) {
                    this.updateUptime(status.uptime_seconds);
                }
                
                // 워커 상태 업데이트
                if (status.worker_statuses) {
                    this.updateWorkerStatuses(status.worker_statuses);
                }
                
                this.updateUI();
            }
            
            updateMetrics(metrics) {
                document.getElementById('currentEPS').textContent = 
                    this.formatNumber(metrics.current_eps || 0);
                
                document.getElementById('achievementRate').textContent = 
                    (metrics.achievement_percent || 0).toFixed(1) + '%';
                
                document.getElementById('totalSent').textContent = 
                    this.formatNumber(metrics.total_sent || 0);
                
                document.getElementById('activeWorkers').textContent = 
                    (metrics.active_workers || 0) + '/' + (document.getElementById('workerCount').value || '0');
                
                // CPU와 메모리 메트릭 업데이트
                document.getElementById('cpuUsage').textContent = 
                    (metrics.cpu_usage_percent || 0).toFixed(1) + '%';
                
                // 메모리를 자동 단위 변환하여 표시 (MB를 bytes로 변환 후 formatDataSize 적용)
                const memoryBytes = (metrics.memory_usage_mb || 0) * 1024 * 1024;
                document.getElementById('memoryUsage').textContent = 
                    this.formatDataSize(memoryBytes);
                
                // 총 전송 패킷 수 (= 총 전송 로그 수)
                document.getElementById('txPackets').textContent = 
                    this.formatNumber(metrics.network_tx_packets || 0);
                
                // 현재 패킷/초 (= 현재 EPS)
                document.getElementById('txPacketsPerSec').textContent = 
                    this.formatNumber(metrics.current_eps || 0);
                
                // TX 속도 (현재 네트워크 전송 속도)
                document.getElementById('txRate').textContent = 
                    this.formatNetworkSpeed(metrics.network_tx_mbps || 0);
                
                // 총 전송 데이터 크기 (누적된 모든 로그의 바이트 합계)
                const totalBytes = (metrics.network_tx_bytes || 0) + (metrics.network_rx_bytes || 0);
                document.getElementById('networkThroughput').textContent = 
                    this.formatDataSize(totalBytes);
            }
            
            updateWorkerStatuses(statuses) {
                // 현재 표시된 워커 수 가져오기
                const workerElements = document.querySelectorAll('[id^="worker-"]');
                const totalWorkers = workerElements.length;
                
                for (let i = 1; i <= totalWorkers; i++) {
                    const workerElement = document.getElementById('worker-' + i);
                    if (!workerElement) continue;
                    
                    const isActive = statuses[i] || false;
                    
                    if (isActive) {
                        workerElement.className = 'worker-status worker-active';
                        workerElement.textContent = i;
                    } else {
                        workerElement.className = 'worker-status worker-inactive';
                        workerElement.textContent = i;
                    }
                }
            }
            
            updateUptime(seconds) {
                const hours = Math.floor(seconds / 3600);
                const minutes = Math.floor((seconds % 3600) / 60);
                const secs = Math.floor(seconds % 60);
                
                document.getElementById('uptimeDisplay').textContent = 
                    '업타임: ' + String(hours).padStart(2, '0') + ':' + 
                    String(minutes).padStart(2, '0') + ':' + 
                    String(secs).padStart(2, '0');
            }
            
            updateUI() {
                const startBtn = document.getElementById('startBtn');
                const stopBtn = document.getElementById('stopBtn');
                const restartBtn = document.getElementById('restartBtn');
                
                startBtn.disabled = this.isRunning;
                stopBtn.disabled = !this.isRunning;
                restartBtn.disabled = !this.isRunning;
            }
            
            async start() {
                const config = this.getConfigFromForm();
                
                try {
                    // 설정 저장
                    await this.saveConfig(config);
                    
                    // 시작
                    const response = await fetch('/api/start', {
                        method: 'POST'
                    });
                    const result = await response.json();
                    
                    if (result.success) {
                        this.showNotification(result.message, 'success');
                        this.addLog('로그 생성기 시작됨', 'info');
                    } else {
                        this.showNotification('시작 실패: ' + result.error, 'error');
                        this.addLog('시작 실패: ' + result.error, 'error');
                    }
                } catch (error) {
                    this.showNotification('시작 요청 실패: ' + error, 'error');
                    this.addLog('시작 요청 실패: ' + error, 'error');
                }
            }
            
            async stop() {
                try {
                    const response = await fetch('/api/stop', {
                        method: 'POST'
                    });
                    const result = await response.json();
                    
                    if (result.success) {
                        this.showNotification(result.message, 'success');
                        this.addLog('로그 생성기 정지됨', 'info');
                    } else {
                        this.showNotification('정지 실패: ' + result.error, 'error');
                        this.addLog('정지 실패: ' + result.error, 'error');
                    }
                } catch (error) {
                    this.showNotification('정지 요청 실패: ' + error, 'error');
                    this.addLog('정지 요청 실패: ' + error, 'error');
                }
            }
            
            async restart() {
                try {
                    const response = await fetch('/api/restart', {
                        method: 'POST'
                    });
                    const result = await response.json();
                    
                    if (result.success) {
                        this.showNotification(result.message, 'success');
                        this.addLog('로그 생성기 재시작됨', 'info');
                    } else {
                        this.showNotification('재시작 실패: ' + result.error, 'error');
                        this.addLog('재시작 실패: ' + result.error, 'error');
                    }
                } catch (error) {
                    this.showNotification('재시작 요청 실패: ' + error, 'error');
                    this.addLog('재시작 요청 실패: ' + error, 'error');
                }
            }
            
            getConfigFromForm() {
                const profile = document.getElementById('epsProfile').value;
                let targetEPS = 0;
                
                if (profile === 'custom') {
                    targetEPS = parseInt(document.getElementById('targetEPS').value);
                } else {
                    // 프로파일별 기본 EPS
                    const profileEPS = {
                        '100k': 100000,
                        '500k': 500000,
                        '1m': 1000000,
                        '2m': 2000000,
                        '4m': 4000000
                    };
                    targetEPS = profileEPS[profile] || 4000000;
                }
                
                return {
                    target_host: document.getElementById('targetHost').value,
                    profile: profile,
                    target_eps: targetEPS,
                    duration_minutes: parseInt(document.getElementById('duration').value),
                    worker_count: parseInt(document.getElementById('workerCount').value),
                    enable_optimization: document.getElementById('enableOptimization').checked,
                    enable_dashboard: document.getElementById('enableDashboard').checked,
                    batch_size: parseInt(document.getElementById('batchSize').value),
                    send_interval_ms: parseInt(document.getElementById('sendInterval').value),
                    memory_limit_gb: parseInt(document.getElementById('memoryLimit').value),
                    gc_percent: parseInt(document.getElementById('gcPercent').value),
                    hostname_prefix: document.getElementById('hostnamePrefix').value,
                    log_formats: ['syslog'],
                    service_types: ['systemd', 'kernel', 'sshd', 'nginx']
                };
            }
            
            async saveConfig(config = null) {
                if (!config) {
                    config = this.getConfigFromForm();
                }
                
                try {
                    const response = await fetch('/api/config', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json'
                        },
                        body: JSON.stringify(config)
                    });
                    const result = await response.json();
                    
                    if (result.success) {
                        this.showNotification('설정이 저장되었습니다', 'success');
                        this.addLog('설정 저장됨', 'info');
                        return true;
                    } else {
                        this.showNotification('설정 저장 실패: ' + result.error, 'error');
                        this.addLog('설정 저장 실패: ' + result.error, 'error');
                        return false;
                    }
                } catch (error) {
                    this.showNotification('설정 저장 요청 실패: ' + error, 'error');
                    this.addLog('설정 저장 요청 실패: ' + error, 'error');
                    return false;
                }
            }
            
            async loadConfig() {
                try {
                    const response = await fetch('/api/config');
                    const result = await response.json();
                    
                    if (result.success && result.data) {
                        const config = result.data;
                        
                        document.getElementById('targetHost').value = config.target_host || '127.0.0.1';
                        
                        // 프로파일 설정
                        if (config.profile) {
                            document.getElementById('epsProfile').value = config.profile;
                            onProfileChange();
                        }
                        
                        if (config.profile === 'custom') {
                            document.getElementById('targetEPS').value = config.target_eps || 4000000;
                        }
                        
                        document.getElementById('duration').value = config.duration_minutes || 0;
                        
                        // 프로파일에 맞는 워커 수 설정
                        const profileSettings = this.getProfileSettings();
                        const currentProfile = document.getElementById('epsProfile').value;
                        const profileConfig = profileSettings[currentProfile] || profileSettings['4m'];
                        document.getElementById('workerCount').value = profileConfig.workers;
                        
                        document.getElementById('enableOptimization').checked = config.enable_optimization !== false;
                        document.getElementById('enableDashboard').checked = config.enable_dashboard !== false;
                        document.getElementById('batchSize').value = config.batch_size || 1000;
                        document.getElementById('sendInterval').value = config.send_interval_ms || 10;
                        document.getElementById('memoryLimit').value = config.memory_limit_gb || 12;
                        document.getElementById('gcPercent').value = config.gc_percent || 200;
                        document.getElementById('hostnamePrefix').value = config.hostname_prefix || 'server';
                    }
                } catch (error) {
                    this.addLog('설정 로딩 실패: ' + error, 'error');
                }
            }
            
            async optimizeSystem() {
                try {
                    const response = await fetch('/api/system-optimize', {
                        method: 'POST'
                    });
                    const result = await response.json();
                    
                    if (result.success) {
                        this.showNotification(result.message, 'success');
                        this.addLog('시스템 최적화 적용됨', 'info');
                    } else {
                        this.showNotification('최적화 실패: ' + result.error, 'error');
                        this.addLog('최적화 실패: ' + result.error, 'error');
                    }
                } catch (error) {
                    this.showNotification('최적화 요청 실패: ' + error, 'error');
                    this.addLog('최적화 요청 실패: ' + error, 'error');
                }
            }
            
            showNotification(message, type) {
                const notification = document.createElement('div');
                notification.className = 'notification ' + type;
                notification.textContent = message;
                
                document.body.appendChild(notification);
                
                setTimeout(() => {
                    notification.remove();
                }, 4000);
            }
            
            addLog(message, level = 'info') {
                const logsPanel = document.getElementById('logsPanel');
                const logEntry = document.createElement('div');
                logEntry.className = 'log-entry';
                
                const timestamp = new Date().toLocaleTimeString();
                logEntry.innerHTML = 
                    '<span class="log-timestamp">[' + timestamp + ']</span> ' +
                    '<span class="log-' + level + '">' + message + '</span>';
                
                logsPanel.appendChild(logEntry);
                logsPanel.scrollTop = logsPanel.scrollHeight;
                
                // 로그 제한 (최대 100개)
                const logs = logsPanel.children;
                if (logs.length > 100) {
                    logsPanel.removeChild(logs[0]);
                }
            }
            
            formatNumber(num) {
                if (num >= 1000000000) {
                    return (num / 1000000000).toFixed(2) + 'B';
                } else if (num >= 1000000) {
                    return (num / 1000000).toFixed(2) + 'M';
                } else if (num >= 1000) {
                    return (num / 1000).toFixed(1) + 'K';
                }
                return num.toString();
            }
            
            // 네트워크 속도 자동 단위 변환 (bps, Kbps, Mbps, Gbps, Tbps)
            formatNetworkSpeed(mbps) {
                if (!mbps || mbps === 0) return '0 bps';
                
                const bps = mbps * 1024 * 1024; // Mbps를 bps로 변환
                
                if (bps >= 1e12) {
                    return (bps / 1e12).toFixed(2) + ' Tbps';
                } else if (bps >= 1e9) {
                    return (bps / 1e9).toFixed(2) + ' Gbps';
                } else if (bps >= 1e6) {
                    return (bps / 1e6).toFixed(2) + ' Mbps';
                } else if (bps >= 1e3) {
                    return (bps / 1e3).toFixed(2) + ' Kbps';
                }
                return bps.toFixed(0) + ' bps';
            }
            
            // 데이터 크기 자동 단위 변환 (B, KB, MB, GB, TB, PB)
            formatDataSize(bytes) {
                if (!bytes || bytes === 0) return '0 B';
                
                const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
                const k = 1024;
                const i = Math.floor(Math.log(bytes) / Math.log(k));
                
                if (i >= units.length) {
                    return (bytes / Math.pow(k, units.length - 1)).toFixed(2) + ' ' + units[units.length - 1];
                }
                
                return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + units[i];
            }
        }
        
        // 전역 함수들
        let controller;
        
        function startGenerator() {
            controller.start();
        }
        
        function stopGenerator() {
            controller.stop();
        }
        
        function restartGenerator() {
            controller.restart();
        }
        
        function saveConfig() {
            controller.saveConfig();
        }
        
        function optimizeSystem() {
            controller.optimizeSystem();
        }
        
        function toggleAdvanced() {
            const advancedConfig = document.getElementById('advancedConfig');
            advancedConfig.classList.toggle('show');
        }
        
        function onProfileChange() {
            const profile = document.getElementById('epsProfile').value;
            const customEPSGroup = document.getElementById('customEPSGroup');
            const workerCountInput = document.getElementById('workerCount');
            const workerRecommend = document.getElementById('workerRecommend');
            const batchSizeInput = document.getElementById('batchSize');
            const sendIntervalInput = document.getElementById('sendInterval');
            const memoryLimitInput = document.getElementById('memoryLimit');
            const gcPercentInput = document.getElementById('gcPercent');
            
            // controller가 있으면 그에서 프로파일 설정 가져오기
            const profileSettings = controller ? controller.getProfileSettings() : {
                '100k': { workers: 10, batchSize: 100, sendInterval: 10, memoryLimit: 2, gcPercent: 100 },
                '500k': { workers: 50, batchSize: 100, sendInterval: 10, memoryLimit: 4, gcPercent: 150 },
                '1m': { workers: 100, batchSize: 100, sendInterval: 10, memoryLimit: 6, gcPercent: 200 },
                '2m': { workers: 100, batchSize: 200, sendInterval: 10, memoryLimit: 8, gcPercent: 200 },
                '4m': { workers: 200, batchSize: 200, sendInterval: 10, memoryLimit: 12, gcPercent: 200 },
                'custom': { workers: 0, batchSize: 100, sendInterval: 50, memoryLimit: 8, gcPercent: 200 }
            };
            
            const settings = profileSettings[profile] || profileSettings['4m'];
            
            if (profile === 'custom') {
                customEPSGroup.style.display = 'block';
                workerCountInput.readOnly = false;
                workerRecommend.textContent = '(사용자 설정)';
            } else {
                customEPSGroup.style.display = 'none';
                workerCountInput.readOnly = true;
                workerCountInput.value = settings.workers;
                workerRecommend.textContent = '(자동 설정: ' + settings.workers + '개)';
                
                // 워커 그리드 업데이트
                if (controller && controller.initializeWorkerGrid) {
                    controller.initializeWorkerGrid(settings.workers);
                }
                
                // 워커 상태 타이틀 업데이트
                const workerStatusTitle = document.getElementById('workerStatusTitle');
                if (workerStatusTitle) {
                    workerStatusTitle.textContent = settings.workers;
                }
                
                // 고급 설정도 자동 업데이트
                batchSizeInput.value = settings.batchSize;
                sendIntervalInput.value = settings.sendInterval;
                memoryLimitInput.value = settings.memoryLimit;
                gcPercentInput.value = settings.gcPercent;
            }
        }
        
        // 초기화
        document.addEventListener('DOMContentLoaded', () => {
            controller = new LogGeneratorController();
        });
    </script>
</body>
</html>`
}

// generateEmbeddedControlUI - 임베드 가능한 간소화된 제어 UI
func (cs *ControlServer) generateEmbeddedControlUI() string {
	return `<!DOCTYPE html>
<html lang="ko">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>로그 생성기 간편 제어</title>
    <style>
        /* 간소화된 스타일 */
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background: #f5f5f5;
        }
        
        .control-panel {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            max-width: 400px;
        }
        
        .btn {
            padding: 10px 15px;
            margin: 5px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-weight: bold;
        }
        
        .btn-start { background: #4CAF50; color: white; }
        .btn-stop { background: #f44336; color: white; }
        .btn-restart { background: #ff9800; color: white; }
        .btn:disabled { opacity: 0.5; cursor: not-allowed; }
        
        .form-group {
            margin: 10px 0;
        }
        
        .form-group label {
            display: block;
            margin-bottom: 5px;
            font-weight: bold;
        }
        
        .form-group input {
            width: 100%;
            padding: 8px;
            border: 1px solid #ddd;
            border-radius: 4px;
        }
        
        .status {
            padding: 10px;
            margin: 10px 0;
            border-radius: 4px;
            text-align: center;
            font-weight: bold;
        }
        
        .status.running { background: #d4edda; color: #155724; }
        .status.stopped { background: #f8d7da; color: #721c24; }
    </style>
</head>
<body>
    <div class="control-panel">
        <h2>로그 생성기 제어</h2>
        
        <div class="status stopped" id="statusDisplay">정지됨</div>
        
        <div class="form-group">
            <label>목표 호스트:</label>
            <input type="text" id="targetHost" value="127.0.0.1">
        </div>
        
        <div class="form-group">
            <label>목표 EPS:</label>
            <input type="number" id="targetEPS" value="4000000">
        </div>
        
        <div class="form-group">
            <label>실행 시간 (분):</label>
            <input type="number" id="duration" value="0">
        </div>
        
        <div>
            <button class="btn btn-start" id="startBtn" onclick="start()">시작</button>
            <button class="btn btn-stop" id="stopBtn" onclick="stop()" disabled>정지</button>
            <button class="btn btn-restart" id="restartBtn" onclick="restart()" disabled>재시작</button>
        </div>
        
        <div style="margin-top: 20px; font-size: 0.9em; color: #666;">
            현재 EPS: <span id="currentEPS">0</span><br>
            활성 워커: <span id="activeWorkers">0</span>/40
        </div>
    </div>
    
    <script>
        // 간소화된 제어 스크립트
        let isRunning = false;
        
        async function start() {
            const config = {
                target_host: document.getElementById('targetHost').value,
                target_eps: parseInt(document.getElementById('targetEPS').value),
                duration_minutes: parseInt(document.getElementById('duration').value)
            };
            
            try {
                await fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(config)
                });
                
                const response = await fetch('/api/start', { method: 'POST' });
                const result = await response.json();
                
                if (result.success) {
                    updateStatus(true);
                    alert('시작됨: ' + result.message);
                } else {
                    alert('시작 실패: ' + result.error);
                }
            } catch (error) {
                alert('요청 실패: ' + error);
            }
        }
        
        async function stop() {
            try {
                const response = await fetch('/api/stop', { method: 'POST' });
                const result = await response.json();
                
                if (result.success) {
                    updateStatus(false);
                    alert('정지됨: ' + result.message);
                } else {
                    alert('정지 실패: ' + result.error);
                }
            } catch (error) {
                alert('요청 실패: ' + error);
            }
        }
        
        async function restart() {
            try {
                const response = await fetch('/api/restart', { method: 'POST' });
                const result = await response.json();
                
                if (result.success) {
                    alert('재시작됨: ' + result.message);
                } else {
                    alert('재시작 실패: ' + result.error);
                }
            } catch (error) {
                alert('요청 실패: ' + error);
            }
        }
        
        function updateStatus(running) {
            isRunning = running;
            const statusDisplay = document.getElementById('statusDisplay');
            const startBtn = document.getElementById('startBtn');
            const stopBtn = document.getElementById('stopBtn');
            const restartBtn = document.getElementById('restartBtn');
            
            if (running) {
                statusDisplay.textContent = '실행 중';
                statusDisplay.className = 'status running';
                startBtn.disabled = true;
                stopBtn.disabled = false;
                restartBtn.disabled = false;
            } else {
                statusDisplay.textContent = '정지됨';
                statusDisplay.className = 'status stopped';
                startBtn.disabled = false;
                stopBtn.disabled = true;
                restartBtn.disabled = true;
            }
        }
        
        // 주기적 상태 확인
        setInterval(async () => {
            try {
                const response = await fetch('/api/status');
                const result = await response.json();
                
                if (result.success) {
                    updateStatus(result.data.is_running);
                    
                    if (result.data.metrics) {
                        document.getElementById('currentEPS').textContent = 
                            (result.data.metrics.total_eps || 0).toLocaleString();
                        document.getElementById('activeWorkers').textContent = 
                            result.data.metrics.active_workers || 0;
                    }
                }
            } catch (error) {
                console.error('상태 확인 실패:', error);
            }
        }, 3000);
    </script>
</body>
</html>`
}