# 시스템 로그 400만 EPS 전송 PRD
## Product Requirements Document

---

## 1. 프로젝트 개요

### 1.1 프로젝트 명
**시스템 로그 기반 초고성능 로그 생성기 - SIEM 400만 EPS 성능 검증**

### 1.2 목적
- 단일 시스템 로그 포맷으로 **400만 EPS (Events Per Second)** 달성
- SIEM 제품의 시스템 로그 수집 성능 한계 측정
- 실제 운영 환경과 동일한 시스템 로그 패턴으로 현실적 성능 테스트
- 로그 다양성 제거를 통한 순수 처리량 성능 집중 측정

### 1.3 배경 및 근거
- **단순화의 이점**: 로그 파싱 오버헤드 최소화로 순수 I/O 성능 측정
- **운영 현실성**: 대규모 서버 팜에서 시스템 로그가 전체 로그의 60-70% 차지
- **벤치마크 표준화**: 동일한 로그 구조로 정확한 성능 비교 가능
- **리소스 최적화**: 단일 로그 타입으로 메모리 및 CPU 효율성 극대화

### 1.4 성공 기준
- 시스템 로그 형태로 400만 EPS를 24시간간 이상 지속 전송
- 패킷 손실률 0.5% 미만 유지 (단일 로그 타입의 이점 활용)
- CPU 사용률 75% 이하에서 목표 성능 달성
- 메모리 사용량 12GB 이하로 최적화

---

## 2. 비즈니스 요구사항

### 2.1 핵심 목표
| 목표 | 수치 | 측정 방법 |
|------|------|-----------|
| 최대 처리량 | 400만 EPS | 실시간 카운터 |
| 지속 시간 | 30분 이상 | 연속 모니터링 |
| 안정성 | 99.95% | 다운타임 측정 |
| 효율성 | CPU 75% 이하 | 시스템 모니터링 |

### 2.2 비즈니스 가치
- **성능 기준 정립**: 업계 표준 벤치마크 제시
- **비용 효율성**: 단일 로그 타입으로 개발/운영 비용 절감  
- **확장성 검증**: 향후 800만 EPS 확장 가능성 검토
- **경쟁력 확보**: 타 SIEM 대비 성능 우위 입증

---

## 3. 기술 요구사항

### 3.1 시스템 로그 명세

#### 3.1.1 표준 시스템 로그 포맷
```bash
# RFC 3164 기반 시스템 로그 표준
<PRIORITY>TIMESTAMP HOSTNAME TAG[PID]: MESSAGE

# 실제 예시
<6>2025-08-30T10:15:30.123Z server01 systemd[1234]: Starting nginx.service
<4>2025-08-30T10:15:30.456Z server02 kernel[0]: CPU0: temperature above threshold
<5>2025-08-30T10:15:30.789Z server03 sshd[5678]: Accepted password for admin from 192.168.1.100
```

#### 3.1.2 로그 구성 요소 세부 명세
| 필드 | 크기 | 예시 | 설명 |
|------|------|------|------|
| Priority | 3-4 bytes | `<6>` | Facility(16) + Severity(0-7) |
| Timestamp | 29 bytes | `2025-08-30T10:15:30.123Z` | ISO 8601 형식 |
| Hostname | 8-12 bytes | `server01` | 서버 식별자 |
| Tag | 6-15 bytes | `systemd[1234]` | 프로세스명[PID] |
| Message | 50-200 bytes | 시스템 이벤트 내용 | 가변 길이 |
| **총 크기** | **96-260 bytes** | **평균 150 bytes** | 네트워크 효율성 고려 |

### 3.2 로그 생성 전략

#### 3.2.1 시스템 로그 카테고리 (가중치 적용)
```go
// 실제 시스템 환경 반영한 로그 분포
var SystemLogTemplates = []LogTemplate{
    // systemd 관련 (40%)
    {"<6>%s %s systemd[1]: Starting %s", 40},
    {"<6>%s %s systemd[1]: Started %s", 30}, 
    {"<6>%s %s systemd[1]: Stopping %s", 10},
    
    // 커널 메시지 (25%)
    {"<4>%s %s kernel: CPU%d: temperature %s", 15},
    {"<3>%s %s kernel: Out of memory: Kill process %d", 5},
    {"<6>%s %s kernel: device eth0: link up", 5},
    
    // SSH 관련 (20%)
    {"<6>%s %s sshd[%d]: Accepted password for %s from %s", 15},
    {"<4>%s %s sshd[%d]: Failed password for %s from %s", 5},
    
    // 기타 시스템 프로세스 (15%)
    {"<5>%s %s cron[%d]: (%s) CMD (%s)", 8},
    {"<6>%s %s rsyslog[%d]: action 'action 17' suspended", 4},
    {"<6>%s %s NetworkManager[%d]: device (eth0): state change", 3}
}
```

#### 3.2.2 최적화된 로그 생성 알고리즘
```go
type OptimizedLogGenerator struct {
    templates    []string          // 사전 컴파일된 템플릿
    hostnames    []string          // 재사용 가능한 호스트명 풀
    services     []string          // 서비스명 풀
    timestamps   chan string       // 타임스탬프 생성기
    logPool      sync.Pool         // 메모리 풀링
    buffer       []byte           // 재사용 버퍼
}

// 고성능 로그 생성 (Zero-allocation)
func (g *OptimizedLogGenerator) GenerateLog() []byte {
    buf := g.logPool.Get().([]byte)
    defer g.logPool.Put(buf[:0])
    
    // 템플릿 기반 고속 생성 (sprintf 회피)
    template := g.templates[fastRand()%len(g.templates)]
    hostname := g.hostnames[fastRand()%len(g.hostnames)]
    timestamp := <-g.timestamps
    
    return g.fastFormat(buf, template, timestamp, hostname)
}
```

### 3.3 네트워크 아키텍처

#### 3.3.1 멀티포트 UDP 전송 설계
```
포트 분산 전략 (400만 EPS ÷ 40포트 = 10만 EPS/포트)

포트 범위: 514-553 (40개 포트)
├── 포트 514-523: Worker Group 1 (100만 EPS)
├── 포트 524-533: Worker Group 2 (100만 EPS)  
├── 포트 534-543: Worker Group 3 (100만 EPS)
└── 포트 544-553: Worker Group 4 (100만 EPS)

각 Worker Group = 10개 포트 × 10만 EPS = 100만 EPS
```

#### 3.3.2 고성능 UDP 송신 최적화
```go
type UDPSender struct {
    conn        *net.UDPConn
    batch       [][]byte         // 배치 전송용 버퍼
    batchSize   int             // 배치 크기 (1000개)
    sendBuffer  []byte          // 송신 버퍼 (2MB)
}

// 배치 전송으로 시스템 콜 최소화
func (s *UDPSender) SendBatch(logs [][]byte) error {
    // sendmmsg() 활용으로 시스템 콜 오버헤드 90% 감소
    return s.conn.WriteMsgUDP(s.combineLogs(logs), nil, nil)
}
```

---

## 4. 시스템 아키텍처

### 4.1 전체 시스템 구조도
```
                    시스템 로그 생성기 (400만 EPS)
    ┌─────────────────────────────────────────────────────────────┐
    │                     Controller                               │
    │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐ │
    │  │   Config    │ │  Monitor    │ │      Dashboard          │ │  
    │  │  Manager    │ │   Engine    │ │   (Real-time Metrics)   │ │
    │  └─────────────┘ └─────────────┘ └─────────────────────────┘ │
    └─────────────────────────────────────────────────────────────┘
                                 │
    ┌─────────────────────────────────────────────────────────────┐
    │                  Worker Pool (40 Workers)                   │
    │                                                             │
    │  Worker 1-10     Worker 11-20    Worker 21-30   Worker 31-40│
    │  (포트514-523)   (포트524-533)   (포트534-543)  (포트544-553)│
    │  ┌─────────┐     ┌─────────┐     ┌─────────┐    ┌─────────┐ │
    │  │100만EPS │     │100만EPS │     │100만EPS │    │100만EPS │ │
    │  └─────────┘     └─────────┘     └─────────┘    └─────────┘ │
    └─────────────────────────────────────────────────────────────┘
                                 │
                        UDP 멀티포트 전송
                                 │
    ┌─────────────────────────────────────────────────────────────┐
    │                    SIEM 시스템                               │
    │  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────┐ │
    │  │   Log Receiver   │  │  Parser Engine  │  │   Storage    │ │
    │  │  (40 포트 수신)   │  │  (시스템 로그)   │  │   Engine     │ │
    │  └─────────────────┘  └─────────────────┘  └──────────────┘ │
    └─────────────────────────────────────────────────────────────┘
```

### 4.2 핵심 컴포넌트 설계

#### 4.2.1 LogWorker (단위: 10만 EPS)
```go
type LogWorker struct {
    ID              int
    Port            int
    TargetEPS       int
    Connection      *net.UDPConn
    LogGenerator    *SystemLogGenerator
    MetricsChannel  chan WorkerMetrics
    
    // 성능 최적화 필드
    sendBuffer      []byte
    batchBuffer     [][]byte
    ticker          *time.Ticker
    
    // 상태 관리
    isRunning       atomic.Bool
    currentEPS      atomic.Int64
    totalSent       atomic.Int64
    errors          atomic.Int64
}

func (w *LogWorker) Start() error {
    // 10만 EPS = 100,000 events/second
    // = 100 events per 1ms
    interval := time.Microsecond * 10  // 10μs마다 1개 로그
    
    w.ticker = time.NewTicker(interval)
    go w.sendLoop()
    return nil
}

func (w *LogWorker) sendLoop() {
    batch := make([][]byte, 0, 100)  // 100개씩 배치 전송
    
    for range w.ticker.C {
        if len(batch) < 100 {
            log := w.LogGenerator.GenerateSystemLog()
            batch = append(batch, log)
        } else {
            w.sendBatch(batch)
            batch = batch[:0]  // 슬라이스 재사용
        }
    }
}
```

#### 4.2.2 SystemLogGenerator (최적화된 로그 생성)
```go
type SystemLogGenerator struct {
    templates       []string
    hostPool        []string      // 사전 생성된 호스트명
    servicePool     []string      // 사전 생성된 서비스명  
    pidPool         []int         // PID 풀
    timestampCache  string        // 1초마다 갱신되는 타임스탬프
    randomSource    *rand.Rand    // 전용 랜덤 생성기
    
    // 메모리 최적화
    logBuffer       []byte        // 재사용 버퍼 (256 bytes)
    stringBuilder   strings.Builder
}

// 제로 할당 로그 생성
func (g *SystemLogGenerator) GenerateSystemLog() []byte {
    g.stringBuilder.Reset()
    
    // 사전 생성된 컴포넌트 조합 (할당 없음)
    priority := priorities[g.randomSource.Intn(len(priorities))]
    hostname := g.hostPool[g.randomSource.Intn(len(g.hostPool))]
    service := g.servicePool[g.randomSource.Intn(len(g.servicePool))]
    pid := g.pidPool[g.randomSource.Intn(len(g.pidPool))]
    
    // 고속 문자열 조립
    g.stringBuilder.WriteString(priority)
    g.stringBuilder.WriteString(g.timestampCache)  // 캐시된 타임스탬프
    g.stringBuilder.WriteByte(' ')
    g.stringBuilder.WriteString(hostname)
    g.stringBuilder.WriteByte(' ')
    g.stringBuilder.WriteString(service)
    g.stringBuilder.WriteByte('[')
    g.stringBuilder.WriteString(strconv.Itoa(pid))
    g.stringBuilder.WriteString("]: ")
    g.stringBuilder.WriteString(g.getRandomMessage())
    
    return []byte(g.stringBuilder.String())
}
```

### 4.3 메모리 최적화 전략

#### 4.3.1 객체 풀링 패턴
```go
// 글로벌 메모리 풀
var (
    logBufferPool = sync.Pool{
        New: func() interface{} {
            return make([]byte, 256)  // 평균 로그 크기
        },
    }
    
    batchPool = sync.Pool{
        New: func() interface{} {
            return make([][]byte, 0, 1000)  // 배치 크기
        },
    }
    
    builderPool = sync.Pool{
        New: func() interface{} {
            return &strings.Builder{}
        },
    }
)

// 메모리 재사용 패턴
func processLogs() {
    buffer := logBufferPool.Get().([]byte)
    defer logBufferPool.Put(buffer[:0])
    
    batch := batchPool.Get().([][]byte)
    defer batchPool.Put(batch[:0])
    
    // 로그 처리 로직...
}
```

#### 4.3.2 가비지 컬렉션 최적화
```go
// GC 튜닝 설정
func optimizeGC() {
    // GC 목표 백분율 설정 (기본 100 → 200으로 증가)
    debug.SetGCPercent(200)
    
    // 메모리 제한 설정 (12GB)
    debug.SetMemoryLimit(12 << 30)  // 12GB
    
    // 주기적 GC 강제 실행 (30초마다)
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        for range ticker.C {
            runtime.GC()
            debug.FreeOSMemory()
        }
    }()
}
```

---

## 5. 성능 최적화 상세 설계

### 5.1 CPU 최적화

#### 5.1.1 CPU 코어 바인딩 전략
```bash
# 40개 워커를 40개 CPU 코어에 1:1 매핑
# NUMA 노드 고려한 최적 배치

# NUMA Node 0 (코어 0-19): Worker 1-20
# NUMA Node 1 (코어 20-39): Worker 21-40

taskset -c 0 ./log-worker --id=1 --port=514 --eps=100000
taskset -c 1 ./log-worker --id=2 --port=515 --eps=100000
...
taskset -c 39 ./log-worker --id=40 --port=553 --eps=100000
```

#### 5.1.2 고성능 타이머 구현
```go
// 기존 time.Ticker 대신 고정밀 타이머 사용
type HighPrecisionTimer struct {
    interval    time.Duration
    callback    func()
    stopChan    chan struct{}
}

func (t *HighPrecisionTimer) Start() {
    go func() {
        next := time.Now().Add(t.interval)
        for {
            select {
            case <-t.stopChan:
                return
            default:
                now := time.Now()
                if now.After(next) {
                    t.callback()
                    next = next.Add(t.interval)
                }
                // CPU 사용률 조절을 위한 마이크로 슬립
                time.Sleep(time.Nanosecond * 100)
            }
        }
    }()
}
```

### 5.2 네트워크 최적화

#### 5.2.1 커널 파라미터 최적화
```bash
#!/bin/bash
# 400만 EPS를 위한 네트워크 커널 튜닝

# 송신 버퍼 크기 증대
echo 'net.core.wmem_max = 268435456' >> /etc/sysctl.conf          # 256MB
echo 'net.core.wmem_default = 1048576' >> /etc/sysctl.conf       # 1MB

# 수신 버퍼 크기 증대  
echo 'net.core.rmem_max = 268435456' >> /etc/sysctl.conf          # 256MB
echo 'net.core.rmem_default = 1048576' >> /etc/sysctl.conf       # 1MB

# 네트워크 백로그 큐 크기
echo 'net.core.netdev_max_backlog = 30000' >> /etc/sysctl.conf    # 기본 1000 → 30000

# UDP 버퍼 크기
echo 'net.ipv4.udp_mem = 102400 873800 16777216' >> /etc/sysctl.conf

# 로컬 포트 범위 확대
echo 'net.ipv4.ip_local_port_range = 1024 65535' >> /etc/sysctl.conf

# 적용
sysctl -p
```

#### 5.2.2 배치 전송 최적화
```go
// sendmmsg() 시스템 콜을 활용한 배치 전송
type BatchSender struct {
    conn        *net.UDPConn
    batchSize   int           // 1000개씩 배치
    messages    [][]byte
    iovecs      []syscall.Iovec
    msghdrs     []syscall.Msghdr
}

func (b *BatchSender) SendBatch(logs [][]byte) error {
    // 여러 로그를 하나의 시스템 콜로 전송
    batchCount := (len(logs) + b.batchSize - 1) / b.batchSize
    
    for i := 0; i < batchCount; i++ {
        start := i * b.batchSize
        end := start + b.batchSize
        if end > len(logs) {
            end = len(logs)
        }
        
        // sendmmsg로 배치 전송 (시스템 콜 오버헤드 90% 감소)
        err := b.sendmmsg(logs[start:end])
        if err != nil {
            return err
        }
    }
    return nil
}
```

### 5.3 메모리 최적화

#### 5.3.1 사전 할당 및 재사용
```go
type PreAllocatedResources struct {
    // 시스템 로그 구성 요소 사전 생성
    priorities   []string      // 8개 우선순위
    hostnames    []string      // 1000개 호스트명
    services     []string      // 100개 서비스명
    pids         []int         // 10000개 PID
    messages     []string      // 500개 메시지 템플릿
    
    // 시간 관련 캐시
    timestampCache   string        // 1초마다 갱신
    timestampTicker  *time.Ticker
    
    // 버퍼 풀
    logBuffers      sync.Pool
    batchBuffers    sync.Pool
    stringBuilders  sync.Pool
}

func NewPreAllocatedResources() *PreAllocatedResources {
    r := &PreAllocatedResources{}
    
    // 우선순위 사전 생성
    r.priorities = []string{
        "<0>", "<1>", "<2>", "<3>", "<4>", "<5>", "<6>", "<7>",
    }
    
    // 호스트명 사전 생성 (server001~server999)
    r.hostnames = make([]string, 1000)
    for i := 0; i < 1000; i++ {
        r.hostnames[i] = fmt.Sprintf("server%03d", i+1)
    }
    
    // 서비스명 사전 생성
    r.services = []string{
        "systemd", "kernel", "sshd", "nginx", "apache2", "mysql", 
        "postgresql", "redis", "docker", "kubelet", "cron", "rsyslog",
        // ... 더 많은 서비스명
    }
    
    // 타임스탬프 캐시 초기화
    r.updateTimestamp()
    r.timestampTicker = time.NewTicker(time.Second)
    go r.timestampUpdater()
    
    return r
}

func (r *PreAllocatedResources) timestampUpdater() {
    for range r.timestampTicker.C {
        r.updateTimestamp()
    }
}

func (r *PreAllocatedResources) updateTimestamp() {
    r.timestampCache = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}
```

---

## 6. 모니터링 및 관리

### 6.1 실시간 성능 지표

#### 6.1.1 핵심 KPI 대시보드
```go
type PerformanceMetrics struct {
    // 처리량 지표
    CurrentEPS      int64   `json:"current_eps"`
    PeakEPS         int64   `json:"peak_eps"`
    AverageEPS      int64   `json:"average_eps"`
    TotalEvents     int64   `json:"total_events"`
    
    // 시스템 리소스
    CPUUsage        float64 `json:"cpu_usage"`
    MemoryUsage     int64   `json:"memory_usage_bytes"`
    NetworkTx       int64   `json:"network_tx_bytes"`
    NetworkPackets  int64   `json:"network_packets"`
    
    // 안정성 지표
    PacketLoss      float64 `json:"packet_loss_percent"`
    ErrorRate       float64 `json:"error_rate"`
    UptimeSeconds   int64   `json:"uptime_seconds"`
    
    // 워커별 상세 정보
    WorkerMetrics   map[int]*WorkerStatus `json:"worker_metrics"`
}

type WorkerStatus struct {
    ID              int     `json:"id"`
    Port            int     `json:"port"`
    EPS             int64   `json:"eps"`
    TotalSent       int64   `json:"total_sent"`
    Errors          int64   `json:"errors"`
    CPUCore         int     `json:"cpu_core"`
    Status          string  `json:"status"` // running, stopped, error
}
```

#### 6.1.2 웹 기반 실시간 대시보드
```html
<!-- 실시간 모니터링 대시보드 -->
<!DOCTYPE html>
<html>
<head>
    <title>시스템 로그 400만 EPS 모니터링</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body>
    <div class="dashboard">
        <header>
            <h1>시스템 로그 생성기 - 400만 EPS</h1>
            <div class="status-bar">
                <span class="metric">현재 EPS: <span id="current-eps">0</span></span>
                <span class="metric">목표 달성률: <span id="target-rate">0%</span></span>
                <span class="metric">패킷 손실: <span id="packet-loss">0%</span></span>
                <span class="metric">가동시간: <span id="uptime">00:00:00</span></span>
            </div>
        </header>
        
        <main>
            <section class="charts">
                <div class="chart-container">
                    <canvas id="eps-chart"></canvas>
                    <h3>EPS 실시간 추이</h3>
                </div>
                
                <div class="chart-container">
                    <canvas id="resource-chart"></canvas>
                    <h3>시스템 리소스 사용률</h3>
                </div>
            </section>
            
            <section class="worker-grid">
                <h3>워커 상태 (40개)</h3>
                <div id="worker-grid">
                    <!-- 40개 워커 상태가 동적으로 생성됨 -->
                </div>
            </section>
        </main>
    </div>
    
    <script>
        // WebSocket을 통한 실시간 데이터 수신
        const ws = new WebSocket('ws://localhost:8080/metrics');
        
        ws.onmessage = function(event) {
            const metrics = JSON.parse(event.data);
            updateDashboard(metrics);
        };
        
        function updateDashboard(metrics) {
            document.getElementById('current-eps').textContent = 
                metrics.current_eps.toLocaleString();
                
            const targetRate = (metrics.current_eps / 4000000 * 100).toFixed(1);
            document.getElementById('target-rate').textContent = targetRate + '%';
            
            document.getElementById('packet-loss').textContent = 
                metrics.packet_loss.toFixed(2) + '%';
                
            updateWorkerGrid(metrics.worker_metrics);
        }
        
        function updateWorkerGrid(workers) {
            const grid = document.getElementById('worker-grid');
            grid.innerHTML = '';
            
            for (const [id, worker] of Object.entries(workers)) {
                const workerElement = createWorkerElement(worker);
                grid.appendChild(workerElement);
            }
        }
        
        // 1초마다 업데이트
        setInterval(() => {
            // WebSocket을 통해 실시간 업데이트됨
        }, 1000);
    </script>
</body>
</html>
```

### 6.2 알림 및 자동 대응

#### 6.2.1 임계값 기반 알림 시스템
```go
type AlertConfig struct {
    EPSThreshold        int64   `yaml:"eps_threshold"`        // 3,800,000 (목표의 95%)
    PacketLossThreshold float64 `yaml:"packet_loss_threshold"` // 0.5%
    CPUThreshold        float64 `yaml:"cpu_threshold"`        // 80%
    MemoryThreshold     int64   `yaml:"memory_threshold"`     // 14GB
}

type AlertManager struct {
    config      *AlertConfig
    alerts      chan Alert
    webhookURL  string
    emailSMTP   SMTPConfig
}

func (am *AlertManager) CheckAndAlert(metrics *PerformanceMetrics) {
    // EPS 성능 저하 알림
    if metrics.CurrentEPS < am.config.EPSThreshold {
        alert := Alert{
            Level:   "WARNING",
            Message: fmt.Sprintf("EPS 성능 저하: %d (목표: 4,000,000)", metrics.CurrentEPS),
            Time:    time.Now(),
            Metrics: metrics,
        }
        am.sendAlert(alert)
    }
    
    // 패킷 손실 임계값 초과
    if metrics.PacketLoss > am.config.PacketLossThreshold {
        alert := Alert{
            Level:   "CRITICAL",
            Message: fmt.Sprintf("패킷 손실 임계값 초과: %.2f%%", metrics.PacketLoss),
            Time:    time.Now(),
            Action:  "네트워크 버퍼 크기 자동 조정",
        }
        am.sendAlert(alert)
        am.autoTuneNetwork()  // 자동 복구 시도
    }
}
```

---

## 7. 테스트 계획 및 검증

### 7.1 성능 테스트 시나리오

#### 7.1.1 단계별 성능 검증
```yaml
# 테스트 시나리오 정의
test_scenarios:
  - name: "점진적 부하 증가"
    duration: "60분"
    steps:
      - eps: 1000000
        duration: "10분"
        success_criteria: "안정적 동작"
      - eps: 2000000  
        duration: "10분"
        success_criteria: "CPU 70% 이하"
      - eps: 3000000
        duration: "15분" 
        success_criteria: "패킷손실 0.3% 이하"
      - eps: 4000000
        duration: "25분"
        success_criteria: "목표 성능 달성"
        
  - name: "지속성 테스트"
    duration: "24시간"
    eps: 4000000
    success_criteria:
      - "메모리 사용량 안정"
      - "패킷 손실률 0.5% 이하"
      - "CPU 사용률 75% 이하"
      - "무중단 동작"
      
  - name: "부하 급증 테스트"
    duration: "30분"
    pattern: "burst"
    steps:
      - eps: 2000000
        duration: "5분"
      - eps: 5000000  # 목표 초과 부하
        duration: "2분"
      - eps: 4000000
        duration: "23분"
    success_criteria: "시스템 복구 및 안정화"
```

#### 7.1.2 자동화된 테스트 스크립트
```bash
#!/bin/bash
# 자동화된 성능 테스트 실행

LOG_GENERATOR_HOST="192.168.1.100"
SIEM_HOST="192.168.1.200"
TEST_DURATION="1800"  # 30분

echo "=== 시스템 로그 400만 EPS 테스트 시작 ==="

# 1. 시스템 사전 점검
echo "1. 시스템 사전 점검..."
ssh root@$LOG_GENERATOR_HOST "free -h && df -h && nproc"
ssh root@$SIEM_HOST "netstat -tuln | grep -E ':(51[4-9]|5[2-4][0-9]|55[0-3])'"

# 2. 네트워크 최적화 설정 적용
echo "2. 네트워크 튜닝 적용..."
ssh root@$LOG_GENERATOR_HOST "./scripts/network-tuning.sh"
ssh root@$SIEM_HOST "./scripts/network-tuning.sh"

# 3. SIEM 시스템 시작
echo "3. SIEM 로그 수집기 시작..."
ssh root@$SIEM_HOST "./siem-collector --ports=514-553 --max-eps=4500000 &"
sleep 10

# 4. 로그 생성기 시작 (점진적 증가)
echo "4. 로그 생성기 시작..."
ssh root@$LOG_GENERATOR_HOST "./log-generator \
    --target=$SIEM_HOST \
    --log-type=system \
    --eps=4000000 \
    --ports=40 \
    --duration=$TEST_DURATION \
    --ramp-up=600 \
    --monitor-port=8080 &"

# 5. 실시간 모니터링
echo "5. 실시간 모니터링 시작..."
./monitor-test.sh $LOG_GENERATOR_HOST $SIEM_HOST $TEST_DURATION

# 6. 결과 수집 및 분석
echo "6. 테스트 결과 수집..."
./collect-results.sh $LOG_GENERATOR_HOST $SIEM_HOST

echo "=== 테스트 완료 ==="
```

### 7.2 성능 측정 및 분석

#### 7.2.1 측정 도구 및 지표
```go
type TestMetrics struct {
    // 기본 성능 지표
    TestStartTime       time.Time       `json:"test_start_time"`
    TestDuration        time.Duration   `json:"test_duration"`
    TargetEPS           int64          `json:"target_eps"`
    AchievedEPS         int64          `json:"achieved_eps"`
    PeakEPS             int64          `json:"peak_eps"`
    
    // 상세 분석 지표
    EPSVariance         float64        `json:"eps_variance"`         // EPS 변동성
    PacketLossRate      float64        `json:"packet_loss_rate"`     // 패킷 손실률
    Latency95th         time.Duration  `json:"latency_95th"`         // 95% 지연시간
    ThroughputMbps      float64        `json:"throughput_mbps"`      // 네트워크 처리량
    
    // 시스템 리소스 사용률
    CPUUsageAvg         float64        `json:"cpu_usage_avg"`
    CPUUsagePeak        float64        `json:"cpu_usage_peak"`
    MemoryUsageAvg      int64          `json:"memory_usage_avg"`
    MemoryUsagePeak     int64          `json:"memory_usage_peak"`
    NetworkUtilization  float64        `json:"network_utilization"`
    
    // 안정성 지표
    ErrorCount          int64          `json:"error_count"`
    RecoveryTime        time.Duration  `json:"recovery_time"`        // 장애 복구 시간
    UptimePercentage    float64        `json:"uptime_percentage"`
    
    // 워커별 세부 통계
    WorkerStats         map[int]*WorkerTestStats `json:"worker_stats"`
}

type WorkerTestStats struct {
    WorkerID           int            `json:"worker_id"`
    Port               int            `json:"port"`
    TotalSent          int64          `json:"total_sent"`
    ErrorCount         int64          `json:"error_count"`
    AverageEPS         float64        `json:"average_eps"`
    CPUCoreUsage       float64        `json:"cpu_core_usage"`
    MaxLatency         time.Duration  `json:"max_latency"`
}
```

#### 7.2.2 결과 분석 리포트
```go
func GenerateTestReport(metrics *TestMetrics) *TestReport {
    report := &TestReport{
        Summary: TestSummary{
            TestPassed:      metrics.AchievedEPS >= 4000000,
            EfficiencyScore: calculateEfficiencyScore(metrics),
            Grade:           calculateGrade(metrics),
        },
    }
    
    // 성능 분석
    report.PerformanceAnalysis = PerformanceAnalysis{
        TargetAchievement: float64(metrics.AchievedEPS) / float64(metrics.TargetEPS) * 100,
        ConsistencyScore:  calculateConsistencyScore(metrics.EPSVariance),
        ScalabilityScore:  calculateScalabilityScore(metrics.WorkerStats),
    }
    
    // 리소스 효율성 분석
    report.ResourceEfficiency = ResourceEfficiency{
        CPUEfficiency:     metrics.AchievedEPS / int64(metrics.CPUUsageAvg * 1000),
        MemoryEfficiency:  metrics.AchievedEPS / (metrics.MemoryUsageAvg >> 20), // MB 단위
        NetworkEfficiency: metrics.ThroughputMbps / metrics.NetworkUtilization,
    }
    
    // 권장사항 생성
    report.Recommendations = generateRecommendations(metrics)
    
    return report
}
```

---

## 8. 위험 관리 및 대응 방안

### 8.1 기술적 위험 분석

#### 8.1.1 성능 관련 위험
| 위험 | 확률 | 영향도 | 대응 방안 |
|------|------|--------|-----------|
| 400만 EPS 미달 | 중간 | 높음 | 하드웨어 업그레이드, 알고리즘 최적화 |
| 높은 패킷 손실률 | 높음 | 중간 | 네트워크 버퍼 증대, 배치 크기 조정 |
| 메모리 누수 | 낮음 | 높음 | 메모리 프로파일링, 자동 GC 튜닝 |
| CPU 병목 현상 | 중간 | 중간 | NUMA 최적화, 코어 바인딩 |

#### 8.1.2 위험 대응 전략
```go
// 자동 성능 조정 시스템
type AutoTuner struct {
    metrics         *MetricsCollector
    currentConfig   *Config
    adjustments     chan ConfigAdjustment
}

func (at *AutoTuner) MonitorAndAdjust() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        current := at.metrics.GetCurrentMetrics()
        
        // EPS 목표 미달 시 자동 조정
        if current.CurrentEPS < 3800000 {  // 목표의 95%
            adjustment := at.calculateAdjustment(current)
            select {
            case at.adjustments <- adjustment:
                log.Printf("성능 조정 적용: %+v", adjustment)
            default:
                log.Printf("조정 채널이 가득참, 건너뛰기")
            }
        }
        
        // 패킷 손실률 높을 때 배치 크기 조정
        if current.PacketLoss > 0.3 {
            at.increaseBatchSize()
        }
        
        // 메모리 사용률 높을 때 GC 강제 실행
        if current.MemoryUsage > 14*1024*1024*1024 {  // 14GB
            runtime.GC()
            debug.FreeOSMemory()
        }
    }
}

type ConfigAdjustment struct {
    WorkerCount     int    `json:"worker_count"`
    BatchSize       int    `json:"batch_size"`
    SendInterval    int    `json:"send_interval_us"`
    BufferSize      int    `json:"buffer_size"`
    GCPercent       int    `json:"gc_percent"`
}
```

### 8.2 운영상 위험 대응

#### 8.2.1 장애 감지 및 자동 복구
```go
type HealthChecker struct {
    workers         map[int]*LogWorker
    unhealthyLimit  int
    recoveryActions []RecoveryAction
}

func (hc *HealthChecker) CheckHealth() {
    unhealthyCount := 0
    
    for id, worker := range hc.workers {
        if !worker.IsHealthy() {
            unhealthyCount++
            log.Printf("워커 %d 비정상 상태 감지", id)
            
            // 자동 복구 시도
            go hc.recoverWorker(worker)
        }
    }
    
    // 전체 시스템 위험 상황
    if unhealthyCount > hc.unhealthyLimit {
        hc.executeEmergencyProcedure()
    }
}

func (hc *HealthChecker) recoverWorker(worker *LogWorker) {
    // 1단계: 소프트 재시작
    if err := worker.SoftRestart(); err == nil {
        return
    }
    
    // 2단계: 하드 재시작
    if err := worker.HardRestart(); err == nil {
        return
    }
    
    // 3단계: 다른 코어로 이전
    hc.migrateWorkerToNewCore(worker)
}
```

---

## 9. 구현 로드맵

### 9.1 개발 일정 (6주)

| 주차 | 작업 내용 | 담당자 | 산출물 | 성공 기준 |
|------|-----------|--------|--------|-----------|
| **1주** | 아키텍처 설계 및 환경 구축 | 시스템 아키텍트 | 상세 설계서, 개발 환경 | 개발 환경 완료 |
| **2주** | 핵심 로그 생성 엔진 개발 | 백엔드 개발자 | LogGenerator 모듈 | 100만 EPS 달성 |
| **3주** | 멀티워커 및 네트워크 최적화 | 성능 엔지니어 | 멀티포트 전송 모듈 | 200만 EPS 달성 |
| **4주** | 400만 EPS 최적화 | 전체 팀 | 최적화된 시스템 | 400만 EPS 달성 |
| **5주** | 모니터링 및 관리 도구 | 프론트엔드 개발자 | 웹 대시보드 | 실시간 모니터링 |
| **6주** | 통합 테스트 및 문서화 | 전체 팀 | 최종 제품, 매뉴얼 | 모든 요구사항 충족 |

### 9.2 주요 마일스톤

#### M1 (1주차 말): 기반 아키텍처 완성
- [x] 전체 시스템 아키텍처 확정
- [x] 개발 환경 및 CI/CD 파이프라인 구축
- [x] 기본 프로젝트 구조 생성
- [x] 네트워크 최적화 스크립트 준비

#### M2 (2주차 말): 기본 기능 구현
- [ ] 시스템 로그 생성 엔진 완성
- [ ] UDP 전송 기본 기능
- [ ] 단일 워커 100만 EPS 달성
- [ ] 기본 메트릭 수집 기능

#### M3 (3주차 말): 확장성 구현  
- [ ] 40개 워커 멀티프로세싱
- [ ] 멀티포트 분산 전송
- [ ] 200만 EPS 안정적 달성
- [ ] CPU 코어 바인딩 최적화

#### M4 (4주차 말): 목표 성능 달성
- [ ] 400만 EPS 달성 및 검증
- [ ] 패킷 손실률 0.5% 이하
- [ ] CPU 사용률 75% 이하
- [ ] 메모리 사용량 12GB 이하

#### M5 (5주차 말): 관리 도구 완성
- [ ] 실시간 웹 대시보드
- [ ] 자동 알림 시스템
- [ ] 성능 자동 조정 기능
- [ ] 장애 감지 및 복구 시스템

#### M6 (6주차 말): 프로덕션 준비
- [ ] 24시간 지속성 테스트 통과
- [ ] 모든 성능 요구사항 검증
- [ ] 사용자 매뉴얼 및 기술 문서
- [ ] 배포 스크립트 및 설치 가이드

---

## 10. 리소스 및 예산

### 10.1 인적 자원

| 역할 | 인원 | 기간 | 주요 업무 |
|------|------|------|-----------|
| 프로젝트 매니저 | 1명 | 6주 | 전체 일정 관리, 위험 관리 |
| 시스템 아키텍트 | 1명 | 2주 | 아키텍처 설계, 기술 검토 |
| 백엔드 개발자 (Go) | 2명 | 6주 | 핵심 엔진 개발, 성능 최적화 |
| 성능 엔지니어 | 1명 | 4주 | 네트워크/시스템 최적화 |
| 프론트엔드 개발자 | 1명 | 2주 | 모니터링 대시보드 개발 |
| QA 엔지니어 | 1명 | 3주 | 테스트 계획 수립, 검증 |

### 10.2 하드웨어 리소스

#### 10.2.1 로그 생성기 서버
| 구성요소 | 사양 | 수량 | 예상 비용 |
|----------|------|------|-----------|
| 서버 | Dell R750 (Xeon 8380, 64GB RAM) | 1대 | $15,000 |
| 네트워크 카드 | Intel E810 100Gbps NIC | 1개 | $3,000 |
| 스토리지 | 2TB NVMe SSD | 1개 | $800 |
| **소계** | | | **$18,800** |

#### 10.2.2 SIEM 테스트 서버
| 구성요소 | 사양 | 수량 | 예상 비용 |
|----------|------|------|-----------|
| 서버 | Dell R750 (Dual Xeon 8380, 256GB RAM) | 1대 | $25,000 |
| 네트워크 카드 | Intel E810 100Gbps NIC | 2개 | $6,000 |
| 스토리지 | 10TB NVMe SSD RAID | 1세트 | $4,000 |
| **소계** | | | **$35,000** |

#### 10.2.3 네트워크 인프라
| 구성요소 | 사양 | 수량 | 예상 비용 |
|----------|------|------|-----------|
| 100Gbps 스위치 | Cisco Nexus 9000 | 1대 | $20,000 |
| 광케이블 | 100Gbps QSFP28 | 4개 | $2,000 |
| **소계** | | | **$22,000** |

### 10.3 소프트웨어 및 도구
| 항목 | 용도 | 수량 | 예상 비용 |
|------|------|------|-----------|
| Go 개발 도구 | 무료 (오픈소스) | - | $0 |
| 모니터링 도구 | Grafana Pro | 1년 | $2,000 |
| 성능 분석 도구 | Intel VTune | 2라이선스 | $1,500 |
| **소계** | | | **$3,500** |

### 10.4 총 예산 요약
| 카테고리 | 비용 |
|----------|------|
| 인건비 (6주) | $72,000 |
| 하드웨어 | $75,800 |
| 소프트웨어 | $3,500 |
| 기타 비용 (10%) | $15,130 |
| **총 예산** | **$166,430** |

---

## 11. 품질 보증 및 테스트

### 11.1 품질 기준

#### 11.1.1 성능 품질 기준
```yaml
performance_quality_criteria:
  throughput:
    target_eps: 4000000
    minimum_acceptable: 3800000
    measurement_duration: "30분"
    consistency_variance: "<5%"
    
  latency:
    average_latency: "<1ms"
    p95_latency: "<2ms" 
    p99_latency: "<5ms"
    
  reliability:
    packet_loss_rate: "<0.5%"
    uptime_percentage: ">99.95%"
    error_rate: "<0.01%"
    
  resource_efficiency:
    cpu_usage: "<75%"
    memory_usage: "<12GB"
    network_utilization: "<80%"
```

#### 11.1.2 코드 품질 기준
```yaml
code_quality_criteria:
  test_coverage: ">85%"
  cyclomatic_complexity: "<10"
  go_vet_issues: "0"
  go_fmt_compliance: "100%"
  security_vulnerabilities: "0"
  
  performance_benchmarks:
    log_generation_speed: ">1M logs/sec"
    memory_allocations: "<1000 allocs/sec"
    gc_pause_time: "<1ms"
```

### 11.2 테스트 전략

#### 11.2.1 단위 테스트
```go
// 로그 생성 성능 테스트
func BenchmarkSystemLogGeneration(b *testing.B) {
    generator := NewSystemLogGenerator()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        log := generator.GenerateSystemLog()
        if len(log) < 50 {
            b.Errorf("로그 크기가 너무 작음: %d", len(log))
        }
    }
    
    // 초당 100만 로그 생성 가능한지 검증
    logsPerSec := float64(b.N) / b.Elapsed().Seconds()
    if logsPerSec < 1000000 {
        b.Errorf("로그 생성 속도 부족: %.0f logs/sec", logsPerSec)
    }
}

// UDP 전송 성능 테스트
func BenchmarkUDPSend(b *testing.B) {
    sender := NewUDPSender("localhost:514")
    defer sender.Close()
    
    log := []byte("<6>2025-08-30T10:15:30.123Z server01 systemd[1234]: Starting nginx.service")
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        if err := sender.Send(log); err != nil {
            b.Fatal(err)
        }
    }
}

// 메모리 할당 최적화 테스트
func TestZeroAllocation(t *testing.T) {
    generator := NewSystemLogGenerator()
    
    allocs := testing.AllocsPerRun(1000, func() {
        _ = generator.GenerateSystemLog()
    })
    
    if allocs > 1.0 {
        t.Errorf("메모리 할당이 너무 많음: %.2f allocs per run", allocs)
    }
}
```

#### 11.2.2 통합 테스트
```go
// 전체 시스템 통합 테스트
func TestSystemIntegration(t *testing.T) {
    // SIEM 목 서버 시작
    mockSIEM := startMockSIEMServer(t)
    defer mockSIEM.Close()
    
    // 로그 생성기 시작 (소규모)
    config := &Config{
        TargetEPS:   100000,  // 테스트용 10만 EPS
        Ports:       []int{514, 515, 516, 517},
        Duration:    30 * time.Second,
        LogType:     "system",
    }
    
    generator := NewLogGenerator(config)
    ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
    defer cancel()
    
    // 테스트 실행
    metrics := generator.Run(ctx)
    
    // 결과 검증
    assert.True(t, metrics.AchievedEPS >= 95000, "EPS 목표의 95% 이상 달성")
    assert.True(t, metrics.PacketLoss < 0.01, "패킷 손실률 1% 미만")
    assert.True(t, mockSIEM.GetReceivedCount() > 2900000, "수신된 로그 수 검증")
}

// 부하 테스트
func TestLoadTest(t *testing.T) {
    if testing.Short() {
        t.Skip("부하 테스트는 -short 플래그로 건너뜀")
    }
    
    config := &Config{
        TargetEPS: 4000000,
        Duration:  10 * time.Minute,
    }
    
    generator := NewLogGenerator(config)
    metrics := generator.Run(context.Background())
    
    // 성능 검증
    assert.True(t, metrics.AchievedEPS >= 3800000, "400만 EPS의 95% 이상")
    assert.True(t, metrics.CPUUsage < 0.75, "CPU 사용률 75% 미만")
    assert.True(t, metrics.MemoryUsage < 12*1024*1024*1024, "메모리 12GB 미만")
}
```

#### 11.2.3 스트레스 테스트
```bash
#!/bin/bash
# 스트레스 테스트 스크립트

echo "=== 시스템 로그 스트레스 테스트 시작 ==="

# 테스트 환경 설정
export GOMAXPROCS=40
export GOGC=200

# 1단계: 점진적 부하 증가 (1시간)
echo "1단계: 점진적 부하 증가 테스트"
for eps in 1000000 2000000 3000000 4000000; do
    echo "현재 목표: ${eps} EPS"
    ./log-generator --eps=$eps --duration=15m --log-type=system
    if [ $? -ne 0 ]; then
        echo "ERROR: ${eps} EPS에서 실패"
        exit 1
    fi
    sleep 60  # 1분 휴식
done

# 2단계: 최대 부하 지속 테스트 (2시간) 
echo "2단계: 400만 EPS 지속 테스트 (2시간)"
./log-generator \
    --eps=4000000 \
    --duration=2h \
    --log-type=system \
    --monitor-interval=10s \
    --alert-threshold=3800000

if [ $? -eq 0 ]; then
    echo "SUCCESS: 스트레스 테스트 통과"
else
    echo "FAILED: 스트레스 테스트 실패"
    exit 1
fi

# 3단계: 복구 테스트
echo "3단계: 시스템 복구 능력 테스트"
./log-generator --eps=5000000 --duration=5m  # 과부하
sleep 30
./log-generator --eps=4000000 --duration=10m  # 정상 부하 복구

echo "=== 스트레스 테스트 완료 ==="
```

---

## 12. 운영 및 유지보수

### 12.1 배포 전략

#### 12.1.1 자동화된 배포 파이프라인
```yaml
# .github/workflows/deploy.yml
name: 배포 파이프라인

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: 코드 체크아웃
        uses: actions/checkout@v3
        
      - name: Go 설정
        uses: actions/setup-go@v3
        with:
          go-version: 1.21
          
      - name: 의존성 설치
        run: go mod download
        
      - name: 테스트 실행
        run: go test -v -race ./...
        
      - name: 벤치마크 테스트
        run: go test -bench=. -benchmem ./...
        
      - name: 빌드 (최적화)
        run: |
          CGO_ENABLED=0 GOOS=linux go build \
            -ldflags="-s -w -X main.Version=${{ github.ref_name }}" \
            -o bin/log-generator ./cmd/generator
            
      - name: Docker 이미지 빌드
        run: |
          docker build -t log-generator:${{ github.ref_name }} .
          docker tag log-generator:${{ github.ref_name }} log-generator:latest
          
  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: 프로덕션 서버 배포
        run: |
          scp bin/log-generator production-server:/opt/log-generator/
          ssh production-server 'systemctl restart log-generator'
```

#### 12.1.2 배포 스크립트
```bash
#!/bin/bash
# deploy.sh - 프로덕션 배포 스크립트

VERSION=${1:-latest}
TARGET_SERVER=${2:-production-server}
BACKUP_DIR="/opt/log-generator/backup"

echo "=== 로그 생성기 배포 시작 (버전: $VERSION) ==="

# 1. 현재 버전 백업
echo "1. 현재 버전 백업 중..."
ssh root@$TARGET_SERVER "
    mkdir -p $BACKUP_DIR
    cp /opt/log-generator/log-generator $BACKUP_DIR/log-generator.$(date +%Y%m%d_%H%M%S)
"

# 2. 새 버전 업로드
echo "2. 새 버전 업로드 중..."
scp bin/log-generator root@$TARGET_SERVER:/opt/log-generator/log-generator.new
scp config/production.yaml root@$TARGET_SERVER:/opt/log-generator/config/

# 3. 헬스체크 후 교체
echo "3. 서비스 중단 및 교체..."
ssh root@$TARGET_SERVER "
    systemctl stop log-generator
    mv /opt/log-generator/log-generator.new /opt/log-generator/log-generator
    chmod +x /opt/log-generator/log-generator
    systemctl start log-generator
    sleep 10
    systemctl status log-generator
"

# 4. 배포 후 검증
echo "4. 배포 검증 중..."
sleep 30
health_check=$(curl -s http://$TARGET_SERVER:8080/health | jq -r '.status')
if [ "$health_check" = "healthy" ]; then
    echo "SUCCESS: 배포 완료 및 헬스체크 통과"
else
    echo "ERROR: 헬스체크 실패, 롤백 실행"
    ssh root@$TARGET_SERVER "
        systemctl stop log-generator
        cp $BACKUP_DIR/log-generator.* /opt/log-generator/log-generator
        systemctl start log-generator
    "
    exit 1
fi

echo "=== 배포 완료 ==="
```

### 12.2 모니터링 및 알림

#### 12.2.1 Prometheus 메트릭 수집
```go
// metrics.go - Prometheus 메트릭 정의
package main

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // 처리량 메트릭
    currentEPS = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "log_generator_current_eps",
        Help: "현재 초당 이벤트 처리량",
    })
    
    totalEvents = promauto.NewCounter(prometheus.CounterOpts{
        Name: "log_generator_total_events",
        Help: "총 생성된 이벤트 수",
    })
    
    // 시스템 리소스 메트릭
    cpuUsage = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "log_generator_cpu_usage_percent",
        Help: "CPU 사용률 (퍼센트)",
    })
    
    memoryUsage = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "log_generator_memory_usage_bytes",
        Help: "메모리 사용량 (바이트)",
    })
    
    // 네트워크 메트릭
    networkTxBytes = promauto.NewCounter(prometheus.CounterOpts{
        Name: "log_generator_network_tx_bytes_total",
        Help: "네트워크 송신 바이트 수",
    })
    
    packetLoss = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "log_generator_packet_loss_percent",
        Help: "패킷 손실률 (퍼센트)",
    })
    
    // 워커별 메트릭
    workerEPS = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "log_generator_worker_eps",
            Help: "워커별 EPS",
        },
        []string{"worker_id", "port"},
    )
    
    workerErrors = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "log_generator_worker_errors_total", 
            Help: "워커별 오류 수",
        },
        []string{"worker_id", "port", "error_type"},
    )
)

// 메트릭 업데이트 함수
func UpdateMetrics(metrics *PerformanceMetrics) {
    currentEPS.Set(float64(metrics.CurrentEPS))
    totalEvents.Add(float64(metrics.EventsLastInterval))
    cpuUsage.Set(metrics.CPUUsage)
    memoryUsage.Set(float64(metrics.MemoryUsage))
    networkTxBytes.Add(float64(metrics.NetworkTxLastInterval))
    packetLoss.Set(metrics.PacketLoss)
    
    // 워커별 메트릭 업데이트
    for id, worker := range metrics.WorkerMetrics {
        workerEPS.WithLabelValues(
            strconv.Itoa(id), 
            strconv.Itoa(worker.Port),
        ).Set(float64(worker.EPS))
    }
}
```

#### 12.2.2 Grafana 대시보드 설정
```json
{
  "dashboard": {
    "title": "시스템 로그 생성기 - 400만 EPS 모니터링",
    "panels": [
      {
        "title": "실시간 EPS",
        "type": "stat",
        "targets": [
          {
            "expr": "log_generator_current_eps",
            "legendFormat": "현재 EPS"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "thresholds": {
              "steps": [
                {"color": "red", "value": 0},
                {"color": "yellow", "value": 3800000},
                {"color": "green", "value": 4000000}
              ]
            }
          }
        }
      },
      {
        "title": "EPS 추이 (5분간)",
        "type": "timeseries",
        "targets": [
          {
            "expr": "log_generator_current_eps",
            "legendFormat": "실시간 EPS"
          },
          {
            "expr": "4000000",
            "legendFormat": "목표 EPS"
          }
        ]
      },
      {
        "title": "시스템 리소스 사용률",
        "type": "timeseries", 
        "targets": [
          {
            "expr": "log_generator_cpu_usage_percent",
            "legendFormat": "CPU 사용률 (%)"
          },
          {
            "expr": "log_generator_memory_usage_bytes / 1024 / 1024 / 1024",
            "legendFormat": "메모리 사용량 (GB)"
          }
        ]
      },
      {
        "title": "워커별 성능",
        "type": "heatmap",
        "targets": [
          {
            "expr": "log_generator_worker_eps",
            "legendFormat": "워커 {{worker_id}}"
          }
        ]
      }
    ]
  }
}
```

### 12.3 장애 대응 매뉴얼

#### 12.3.1 일반적인 장애 시나리오
| 장애 유형 | 증상 | 원인 | 대응 방법 |
|----------|------|------|-----------|
| EPS 급격한 하락 | EPS < 3.8M | CPU/메모리 부족 | 워커 수 조정, 리소스 확인 |
| 패킷 손실 증가 | 손실률 > 1% | 네트워크 버퍼 부족 | 버퍼 크기 증대, 배치 크기 조정 |
| 메모리 누수 | 메모리 지속 증가 | GC 문제 | 강제 GC, 프로세스 재시작 |
| 워커 다운 | 특정 포트 무응답 | 네트워크/프로세스 오류 | 워커 재시작, 포트 변경 |

#### 12.3.2 자동 복구 스크립트
```bash
#!/bin/bash
# auto-recovery.sh - 자동 장애 복구

LOG_FILE="/var/log/log-generator-recovery.log"
SERVICE_NAME="log-generator"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> $LOG_FILE
}

# 1. 서비스 상태 확인
check_service_health() {
    response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health)
    if [ "$response" != "200" ]; then
        log "ERROR: 서비스 헬스체크 실패 (HTTP $response)"
        return 1
    fi
    return 0
}

# 2. EPS 성능 확인
check_performance() {
    current_eps=$(curl -s http://localhost:8080/metrics | grep current_eps | awk '{print $2}')
    if [ "$current_eps" -lt "3800000" ]; then
        log "WARNING: EPS 성능 저하 ($current_eps)"
        return 1
    fi
    return 0
}

# 3. 메모리 사용량 확인
check_memory() {
    memory_usage=$(free -b | awk 'NR==2{print $3}')
    memory_limit=$((14 * 1024 * 1024 * 1024))  # 14GB
    
    if [ "$memory_usage" -gt "$memory_limit" ]; then
        log "WARNING: 메모리 사용량 초과 ($((memory_usage/1024/1024/1024))GB)"
        return 1
    fi
    return 0
}

# 4. 복구 작업
recover_service() {
    log "INFO: 자동 복구 시작"
    
    # 소프트 복구 시도
    killall -USR1 log-generator  # 설정 리로드 시그널
    sleep 10
    
    if check_service_health && check_performance; then
        log "INFO: 소프트 복구 성공"
        return 0
    fi
    
    # 하드 복구 시도
    log "INFO: 하드 복구 시작 (서비스 재시작)"
    systemctl restart $SERVICE_NAME
    sleep 30
    
    if check_service_health && check_performance; then
        log "INFO: 하드 복구 성공"
        return 0
    fi
    
    # 복구 실패
    log "ERROR: 자동 복구 실패 - 수동 개입 필요"
    return 1
}

# 메인 로직
main() {
    if ! check_service_health; then
        recover_service
    elif ! check_performance; then
        log "WARNING: 성능 이슈 감지, 복구 시도"
        recover_service
    elif ! check_memory; then
        log "INFO: 메모리 정리 실행"
        killall -USR2 log-generator  # 메모리 정리 시그널
    else
        log "INFO: 시스템 정상 동작 중"
    fi
}

# crontab에서 5분마다 실행
# */5 * * * * /opt/log-generator/scripts/auto-recovery.sh
main
```

---

## 13. 보안 고려사항

### 13.1 네트워크 보안

#### 13.1.1 방화벽 설정
```bash
#!/bin/bash
# firewall-setup.sh - 보안 설정

# 기본 정책: 모든 트래픽 차단
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT ACCEPT

# 로컬호스트 트래픽 허용
iptables -A INPUT -i lo -j ACCEPT

# SSH 접근 허용 (관리용)
iptables -A INPUT -p tcp --dport 22 -m state --state NEW,ESTABLISHED -j ACCEPT

# 로그 전송 포트 허용 (514-553)
iptables -A OUTPUT -p udp --dport 514:553 -j ACCEPT

# 모니터링 포트 허용 (내부 네트워크에서만)
iptables -A INPUT -s 192.168.1.0/24 -p tcp --dport 8080 -j ACCEPT
iptables -A INPUT -s 192.168.1.0/24 -p tcp --dport 9090 -j ACCEPT  # Prometheus

# ICMP 허용 (제한적)
iptables -A INPUT -p icmp --icmp-type echo-request -m limit --limit 1/s -j ACCEPT

# 로그 남기기
iptables -A INPUT -m limit --limit 5/min -j LOG --log-prefix "iptables denied: "

# 설정 저장
iptables-save > /etc/iptables/rules.v4
```

#### 13.1.2 네트워크 격리
```yaml
# docker-compose.yml - 컨테이너 네트워크 격리
version: '3.8'

services:
  log-generator:
    build: .
    container_name: log-generator
    networks:
      - log-network
    ports:
      - "8080:8080"  # 모니터링 포트만 외부 노출
    environment:
      - GOMAXPROCS=40
      - GOGC=200
    ulimits:
      nofile: 1048576  # 파일 디스크립터 제한 증가
    cap_drop:
      - ALL
    cap_add:
      - NET_ADMIN  # 네트워크 최적화에 필요
    security_opt:
      - no-new-privileges:true

  monitoring:
    image: grafana/grafana:latest
    container_name: grafana
    networks:
      - log-network
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=secure_password_here

networks:
  log-network:
    driver: bridge
    internal: false  # 외부 통신 필요 (SIEM으로 로그 전송)
```

### 13.2 애플리케이션 보안

#### 13.2.1 보안 설정
```go
// security.go - 보안 관련 설정
package main

import (
    "crypto/rand"
    "crypto/tls"
    "log"
    "net/http"
    "time"
)

// 보안 HTTP 서버 설정
func NewSecureHTTPServer() *http.Server {
    // TLS 설정
    tlsConfig := &tls.Config{
        MinVersion:               tls.VersionTLS12,
        CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
        PreferServerCipherSuites: true,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
        },
    }
    
    server := &http.Server{
        Addr:         ":8443",  // HTTPS 포트
        TLSConfig:    tlsConfig,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    return server
}

// 보안 헤더 미들웨어
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 보안 헤더 설정
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        w.Header().Set("Content-Security-Policy", "default-src 'self'")
        
        // CORS 설정 (필요한 경우만)
        w.Header().Set("Access-Control-Allow-Origin", "https://monitoring.internal.com")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        
        next.ServeHTTP(w, r)
    })
}

// 인증 미들웨어
func AuthenticationMiddleware(validToken string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := r.Header.Get("Authorization")
            if token == "" {
                token = r.URL.Query().Get("token")
            }
            
            if token != "Bearer "+validToken {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}

// 보안 토큰 생성
func GenerateSecureToken() (string, error) {
    bytes := make([]byte, 32)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes), nil
}
```

#### 13.2.2 입력 검증 및 sanitization
```go
// validation.go - 입력 검증
package main

import (
    "errors"
    "net"
    "regexp"
    "strconv"
    "strings"
)

// 설정 검증
type ConfigValidator struct {
    maxEPS      int64
    maxPorts    int
    validHosts  []string
}

func NewConfigValidator() *ConfigValidator {
    return &ConfigValidator{
        maxEPS:   5000000,  // 최대 500만 EPS
        maxPorts: 100,      // 최대 100개 포트
        validHosts: []string{
            "192.168.1.0/24",
            "10.0.0.0/8",
            "172.16.0.0/12",
        },
    }
}

func (cv *ConfigValidator) ValidateConfig(config *Config) error {
    // EPS 검증
    if config.TargetEPS <= 0 || config.TargetEPS > cv.maxEPS {
        return errors.New("유효하지 않은 EPS 값")
    }
    
    // 포트 범위 검증
    if len(config.Ports) > cv.maxPorts {
        return errors.New("포트 수가 너무 많음")
    }
    
    for _, port := range config.Ports {
        if port < 514 || port > 65535 {
            return errors.New("유효하지 않은 포트 번호")
        }
    }
    
    // 대상 호스트 검증
    if err := cv.validateTargetHost(config.TargetHost); err != nil {
        return err
    }
    
    return nil
}

func (cv *ConfigValidator) validateTargetHost(host string) error {
    ip := net.ParseIP(host)
    if ip == nil {
        return errors.New("유효하지 않은 IP 주소")
    }
    
    // 허용된 네트워크 범위 확인
    for _, cidr := range cv.validHosts {
        _, network, err := net.ParseCIDR(cidr)
        if err != nil {
            continue
        }
        if network.Contains(ip) {
            return nil
        }
    }
    
    return errors.New("허용되지 않은 대상 호스트")
}

// 로그 내용 sanitization
func SanitizeLogContent(content string) string {
    // 제어 문자 제거
    reg := regexp.MustCompile(`[[:cntrl:]]`)
    content = reg.ReplaceAllString(content, "")
    
    // 길이 제한 (최대 1000자)
    if len(content) > 1000 {
        content = content[:1000] + "..."
    }
    
    // HTML 태그 제거 (로그 인젝션 방지)
    reg = regexp.MustCompile(`<[^>]*>`)
    content = reg.ReplaceAllString(content, "")
    
    return strings.TrimSpace(content)
}
```

---

## 14. 성과 측정 및 KPI

### 14.1 핵심 성과 지표

#### 14.1.1 기술적 KPI
```go
type TechnicalKPIs struct {
    // 처리 성능 지표
    AchievedEPS         int64     `json:"achieved_eps"`
    TargetAchievement   float64   `json:"target_achievement_percent"`
    PeakEPS             int64     `json:"peak_eps"`
    AverageEPS          int64     `json:"average_eps"`
    EPSConsistency      float64   `json:"eps_consistency_score"`    // 변동계수의 역수
    
    // 안정성 지표
    UptimePercentage    float64   `json:"uptime_percentage"`
    PacketLossRate      float64   `json:"packet_loss_rate"`
    ErrorRate           float64   `json:"error_rate"`
    MTBF                float64   `json:"mtbf_hours"`               // Mean Time Between Failures
    MTTR                float64   `json:"mttr_minutes"`             // Mean Time To Recovery
    
    // 효율성 지표
    CPUEfficiency       float64   `json:"cpu_efficiency"`           // EPS per CPU%
    MemoryEfficiency    float64   `json:"memory_efficiency"`        // EPS per GB
    NetworkUtilization  float64   `json:"network_utilization"`      // 대역폭 사용률
    PowerEfficiency     float64   `json:"power_efficiency"`         // EPS per Watt
    
    // 확장성 지표
    LinearScalability   float64   `json:"linear_scalability"`       // 워커 증가 대비 성능 증가율
    ResourceScalability float64   `json:"resource_scalability"`     // 리소스 증가 대비 성능 증가율
}
```

#### 14.1.2 비즈니스 KPI
```yaml
business_kpis:
  performance:
    target_eps_achievement: 
      target: 4000000
      minimum_acceptable: 3800000
      measurement_period: "30분 연속"
      
    consistency:
      eps_variance: "<5%"
      availability: ">99.95%"
      
  cost_efficiency:
    development_cost: "<$200,000"
    infrastructure_cost: "<$100,000"
    operational_cost_per_month: "<$10,000"
    
  competitive_advantage:
    industry_benchmark_comparison: ">120%"  # 업계 평균 대비 20% 우수
    time_to_market: "<6주"
    technical_debt_score: "<10%"
```

### 14.2 성과 측정 자동화

#### 14.2.1 KPI 자동 계산
```go
// kpi_calculator.go
package main

import (
    "math"
    "time"
)

type KPICalculator struct {
    startTime    time.Time
    metrics      []PerformanceMetrics
    targetEPS    int64
}

func NewKPICalculator(targetEPS int64) *KPICalculator {
    return &KPICalculator{
        startTime: time.Now(),
        metrics:   make([]PerformanceMetrics, 0),
        targetEPS: targetEPS,
    }
}

func (kc *KPICalculator) AddMetrics(metrics PerformanceMetrics) {
    kc.metrics = append(kc.metrics, metrics)
}

func (kc *KPICalculator) CalculateKPIs() TechnicalKPIs {
    if len(kc.metrics) == 0 {
        return TechnicalKPIs{}
    }
    
    kpis := TechnicalKPIs{}
    
    // 기본 통계 계산
    var sumEPS, maxEPS, minEPS int64
    var sumCPU, sumMemory, sumPacketLoss float64
    var errorCount, uptimeSeconds int64
    
    maxEPS = 0
    minEPS = math.MaxInt64
    
    for _, m := range kc.metrics {
        sumEPS += m.CurrentEPS
        if m.CurrentEPS > maxEPS {
            maxEPS = m.CurrentEPS
        }
        if m.CurrentEPS < minEPS {
            minEPS = m.CurrentEPS
        }
        
        sumCPU += m.CPUUsage
        sumMemory += float64(m.MemoryUsage)
        sumPacketLoss += m.PacketLoss
        errorCount += m.ErrorCount
        
        if m.CurrentEPS > int64(float64(kc.targetEPS)*0.95) {
            uptimeSeconds += 1  // 1초 간격 측정 가정
        }
    }
    
    metricsCount := float64(len(kc.metrics))
    totalTime := time.Since(kc.startTime).Seconds()
    
    // 성능 지표 계산
    kpis.AverageEPS = int64(float64(sumEPS) / metricsCount)
    kpis.AchievedEPS = kpis.AverageEPS
    kpis.PeakEPS = maxEPS
    kpis.TargetAchievement = float64(kpis.AverageEPS) / float64(kc.targetEPS) * 100
    
    // EPS 일관성 계산 (변동계수의 역수)
    variance := kc.calculateVariance(sumEPS, metricsCount)
    if variance > 0 {
        kpis.EPSConsistency = 100.0 / math.Sqrt(variance)
    } else {
        kpis.EPSConsistency = 100.0
    }
    
    // 안정성 지표
    kpis.UptimePercentage = float64(uptimeSeconds) / totalTime * 100
    kpis.PacketLossRate = sumPacketLoss / metricsCount
    kpis.ErrorRate = float64(errorCount) / metricsCount
    
    // 효율성 지표
    avgCPU := sumCPU / metricsCount
    avgMemory := sumMemory / metricsCount / 1024 / 1024 / 1024  // GB 변환
    
    if avgCPU > 0 {
        kpis.CPUEfficiency = float64(kpis.AverageEPS) / avgCPU
    }
    if avgMemory > 0 {
        kpis.MemoryEfficiency = float64(kpis.AverageEPS) / avgMemory
    }
    
    return kpis
}

func (kc *KPICalculator) calculateVariance(sum int64, count float64) float64 {
    mean := float64(sum) / count
    var sumSquares float64
    
    for _, m := range kc.metrics {
        diff := float64(m.CurrentEPS) - mean
        sumSquares += diff * diff
    }
    
    return sumSquares / count
}

// KPI 리포트 생성
func (kc *KPICalculator) GenerateReport() KPIReport {
    kpis := kc.CalculateKPIs()
    
    return KPIReport{
        TestPeriod:      time.Since(kc.startTime),
        TechnicalKPIs:   kpis,
        Grade:           kc.calculateGrade(kpis),
        Recommendations: kc.generateRecommendations(kpis),
        GeneratedAt:     time.Now(),
    }
}

func (kc *KPICalculator) calculateGrade(kpis TechnicalKPIs) string {
    score := 0.0
    
    // 성능 점수 (40%)
    if kpis.TargetAchievement >= 100 {
        score += 40
    } else if kpis.TargetAchievement >= 95 {
        score += 35
    } else if kpis.TargetAchievement >= 90 {
        score += 30
    }
    
    // 안정성 점수 (30%)
    if kpis.UptimePercentage >= 99.95 {
        score += 30
    } else if kpis.UptimePercentage >= 99.9 {
        score += 25
    } else if kpis.UptimePercentage >= 99.0 {
        score += 20
    }
    
    // 효율성 점수 (30%)
    if kpis.CPUEfficiency >= 50000 {  // 50K EPS per 1% CPU
        score += 30
    } else if kpis.CPUEfficiency >= 40000 {
        score += 25
    } else if kpis.CPUEfficiency >= 30000 {
        score += 20
    }
    
    // 등급 산정
    if score >= 90 {
        return "A+"
    } else if score >= 80 {
        return "A"
    } else if score >= 70 {
        return "B"
    } else if score >= 60 {
        return "C"
    } else {
        return "D"
    }
}
```

#### 14.2.2 실시간 KPI 대시보드
```html
<!DOCTYPE html>
<html>
<head>
    <title>KPI 실시간 대시보드</title>
    <style>
        .kpi-grid {
            display: grid;
            grid-template-columns: repeat(4, 1fr);
            gap: 20px;
            margin: 20px;
        }
        
        .kpi-card {
            background: white;
            border-radius: 8px;
            padding: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            text-align: center;
        }
        
        .kpi-value {
            font-size: 2.5em;
            font-weight: bold;
            color: #2196F3;
        }
        
        .kpi-value.success { color: #4CAF50; }
        .kpi-value.warning { color: #FF9800; }
        .kpi-value.error { color: #F44336; }
        
        .kpi-label {
            font-size: 1.1em;
            color: #666;
            margin-top: 10px;
        }
        
        .grade-display {
            font-size: 4em;
            font-weight: bold;
            text-align: center;
            margin: 20px;
        }
        
        .grade-A { color: #4CAF50; }
        .grade-B { color: #2196F3; }
        .grade-C { color: #FF9800; }
        .grade-D { color: #F44336; }
    </style>
</head>
<body>
    <h1>시스템 로그 생성기 KPI 대시보드</h1>
    
    <div class="grade-display" id="overall-grade">
        Loading...
    </div>
    
    <div class="kpi-grid">
        <div class="kpi-card">
            <div class="kpi-value" id="current-eps">0</div>
            <div class="kpi-label">현재 EPS</div>
        </div>
        
        <div class="kpi-card">
            <div class="kpi-value" id="target-achievement">0%</div>
            <div class="kpi-label">목표 달성률</div>
        </div>
        
        <div class="kpi-card">
            <div class="kpi-value" id="uptime">0%</div>
            <div class="kpi-label">가동 시간</div>
        </div>
        
        <div class="kpi-card">
            <div class="kpi-value" id="packet-loss">0%</div>
            <div class="kpi-label">패킷 손실률</div>
        </div>
        
        <div class="kpi-card">
            <div class="kpi-value" id="cpu-efficiency">0</div>
            <div class="kpi-label">CPU 효율성</div>
        </div>
        
        <div class="kpi-card">
            <div class="kpi-value" id="memory-efficiency">0</div>
            <div class="kpi-label">메모리 효율성</div>
        </div>
        
        <div class="kpi-card">
            <div class="kpi-value" id="consistency-score">0</div>
            <div class="kpi-label">일관성 점수</div>
        </div>
        
        <div class="kpi-card">
            <div class="kpi-value" id="error-rate">0</div>
            <div class="kpi-label">오류율</div>
        </div>
    </div>
    
    <script>
        // WebSocket을 통한 실시간 KPI 업데이트
        const ws = new WebSocket('ws://localhost:8080/kpi-stream');
        
        ws.onmessage = function(event) {
            const kpis = JSON.parse(event.data);
            updateKPIDashboard(kpis);
        };
        
        function updateKPIDashboard(kpis) {
            // 기본 값들 업데이트
            updateKPIValue('current-eps', kpis.achieved_eps, formatNumber);
            updateKPIValue('target-achievement', kpis.target_achievement_percent, v => v.toFixed(1) + '%');
            updateKPIValue('uptime', kpis.uptime_percentage, v => v.toFixed(2) + '%');
            updateKPIValue('packet-loss', kpis.packet_loss_rate, v => v.toFixed(3) + '%');
            updateKPIValue('cpu-efficiency', kpis.cpu_efficiency, v => Math.round(v));
            updateKPIValue('memory-efficiency', kpis.memory_efficiency, v => Math.round(v));
            updateKPIValue('consistency-score', kpis.eps_consistency, v => v.toFixed(1));
            updateKPIValue('error-rate', kpis.error_rate, v => v.toFixed(4) + '%');
            
            // 전체 등급 업데이트
            const gradeElement = document.getElementById('overall-grade');
            gradeElement.textContent = kpis.grade || 'N/A';
            gradeElement.className = 'grade-display grade-' + (kpis.grade || 'D').charAt(0);
            
            // 색상 업데이트
            updateValueColor('target-achievement', kpis.target_achievement_percent, 95, 100);
            updateValueColor('uptime', kpis.uptime_percentage, 99.9, 99.95);
            updateValueColor('packet-loss', kpis.packet_loss_rate, 0.5, 0.1, true); // 역순
        }
        
        function updateKPIValue(elementId, value, formatter) {
            const element = document.getElementById(elementId);
            if (element && value !== undefined) {
                element.textContent = formatter ? formatter(value) : value;
            }
        }
        
        function updateValueColor(elementId, value, warningThreshold, successThreshold, reverse = false) {
            const element = document.getElementById(elementId);
            if (!element || value === undefined) return;
            
            element.className = 'kpi-value';
            
            if (reverse) {
                if (value <= successThreshold) {
                    element.classList.add('success');
                } else if (value <= warningThreshold) {
                    element.classList.add('warning');
                } else {
                    element.classList.add('error');
                }
            } else {
                if (value >= successThreshold) {
                    element.classList.add('success');
                } else if (value >= warningThreshold) {
                    element.classList.add('warning');
                } else {
                    element.classList.add('error');
                }
            }
        }
        
        function formatNumber(num) {
            return num.toLocaleString();
        }
        
        // 초기 연결 상태 표시
        ws.onopen = function() {
            console.log('KPI WebSocket 연결됨');
        };
        
        ws.onerror = function(error) {
            console.error('KPI WebSocket 오류:', error);
        };
        
        ws.onclose = function() {
            console.log('KPI WebSocket 연결 종료됨');
            // 재연결 시도
            setTimeout(() => {
                location.reload();
            }, 5000);
        };
    </script>
</body>
</html>
```

---

## 15. 결론 및 향후 계획

### 15.1 프로젝트 요약

이 PRD는 **시스템 로그를 사용한 400만 EPS 달성**이라는 명확한 목표를 가지고 설계되었습니다. 주요 특징은 다음과 같습니다:

- **단일 로그 타입 집중**: 시스템 로그만 사용하여 파싱 오버헤드 최소화
- **극한 성능 최적화**: Go언어 기반 고성능 아키텍처
- **멀티포트 분산**: 40개 포트를 활용한 부하 분산
- **실시간 모니터링**: 웹 기반 대시보드와 자동 알림
- **자동 복구**: 장애 감지 및 자동 대응 시스템

### 15.2 기대 효과

#### 15.2.1 기술적 성과
- **성능 벤치마크 수립**: 업계 최고 수준의 400만 EPS 달성
- **기술 역량 강화**: 고성능 시스템 개발 노하우 축적
- **인프라 최적화**: 하드웨어/소프트웨어 최적화 기법 확보

#### 15.2.2 비즈니스 가치
- **시장 경쟁력**: SIEM 성능 검증을 통한 제품 신뢰성 향상
- **고객 만족도**: 대용량 환경에서의 안정성 입증
- **기술 마케팅**: 400만 EPS 달성이라는 구체적 성과 활용

### 15.3 향후 발전 방향

#### 15.3.1 단기 계획 (6개월)
- **800만 EPS 도전**: 하드웨어 확장을 통한 성능 두 배 향상
- **다양한 로그 타입 지원**: 방화벽, 웹서버 로그 추가 구현
- **AI/ML 기반 로그 생성**: 더욱 현실적인 로그 패턴 구현

#### 15.3.2 중장기 계획 (1-2년)
- **클라우드 네이티브**: Kubernetes 기반 분산 아키텍처
- **실시간 분석**: 스트림 처리 기반 로그 분석 기능
- **상용화**: 독립적인 성능 테스트 도구로 제품화

### 15.4 리스크 및 대응

#### 15.4.1 주요 리스크
- **기술적 한계**: 현재 기술로 400만 EPS 달성 불가능
- **하드웨어 제약**: 예산 내에서 필요 성능의 하드웨어 확보 어려움
- **인력 부족**: 고성능 시스템 개발 경험 있는 개발자 부족

#### 15.4.2 대응 방안
- **단계적 접근**: 200만 → 300만 → 400만 EPS 순차 달성
- **외부 전문가**: 필요시 성능 최적화 전문 컨설턴트 활용
- **기술 파트너십**: 하드웨어/소프트웨어 벤더와의 협력

---

## 부록

### A. 참고 자료
- RFC 3164: The BSD Syslog Protocol
- RFC 5424: The Syslog Protocol
- Go High Performance Programming Best Practices
- Linux Network Performance Tuning Guide
- DPDK Programming Guide

### B. 용어 정의
- **EPS**: Events Per Second, 초당 이벤트 처리량
- **UDP**: User Datagram Protocol, 비연결형 전송 프로토콜
- **SIEM**: Security Information and Event Management
- **NUMA**: Non-Uniform Memory Access
- **GC**: Garbage Collection, 가비지 컬렉션

### C. 변경 이력
| 버전 | 날짜 | 변경 내용 | 작성자 |
|------|------|-----------|--------|
| 1.0 | 2025-08-30 | 초기 버전 작성 | 개발팀 |

---

**문서 상태**: 승인 대기  
**다음 검토일**: 2025-09-06  
**승인 필요**: CTO, 인프라팀장, 보안팀장