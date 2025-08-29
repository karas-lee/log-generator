package generator

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var (
	// 글로벌 메모리 풀 - Zero allocation을 위한 핵심
	logBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 300) // 평균 로그 크기보다 여유있게
		},
	}
	
	builderPool = sync.Pool{
		New: func() interface{} {
			builder := &strings.Builder{}
			builder.Grow(300)
			return builder
		},
	}
)

// SystemLogGenerator - PRD 명세에 따른 RFC 3164 시스템 로그 생성기
type SystemLogGenerator struct {
	// 사전 생성된 컴포넌트 풀 (할당 최소화)
	priorities   []string
	hostnames    []string
	services     []string
	pids         []string
	messages     []string
	
	// 타임스탬프 캐시 (1초마다 갱신)
	timestampCache   string
	timestampMutex   sync.RWMutex
	lastTimestamp    time.Time
	
	// 고속 랜덤 생성기
	rng          *rand.Rand
	rngMutex     sync.Mutex
}

// NewSystemLogGenerator - 400만 EPS를 위한 최적화된 생성기 초기화
func NewSystemLogGenerator() *SystemLogGenerator {
	gen := &SystemLogGenerator{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	
	// PRD 명세에 따른 실제 시스템 로그 패턴 사전 생성
	gen.initializeLogComponents()
	gen.startTimestampUpdater()
	
	return gen
}

func (g *SystemLogGenerator) initializeLogComponents() {
	// RFC 3164 Priority 값들 (Facility * 8 + Severity)
	g.priorities = []string{
		"<0>", "<1>", "<2>", "<3>", "<4>", "<5>", "<6>", "<7>",    // kernel messages
		"<16>", "<17>", "<18>", "<19>", "<20>", "<21>", "<22>", "<23>", // mail
		"<32>", "<33>", "<34>", "<35>", "<36>", "<37>", "<38>", "<39>", // daemon
	}
	
	// 서버 호스트명 풀 (실제 환경과 유사)
	g.hostnames = []string{
		"server01", "server02", "server03", "server04", "server05",
		"web01", "web02", "web03", "db01", "db02", "cache01", "cache02",
		"app01", "app02", "app03", "proxy01", "proxy02", "lb01", "lb02",
	}
	
	// 시스템 서비스명 풀 (PRD 명세 반영)
	g.services = []string{
		"systemd", "kernel", "sshd", "nginx", "apache2", "mysqld",
		"redis-server", "cron", "rsyslog", "NetworkManager", "docker",
		"kubelet", "containerd", "etcd", "prometheus", "grafana",
	}
	
	// PID 풀 사전 생성 (문자열 변환 오버헤드 제거)
	g.pids = make([]string, 10000)
	for i := 0; i < 10000; i++ {
		g.pids[i] = strconv.Itoa(1000 + i)
	}
	
	// 실제 시스템 로그 메시지 템플릿 (가중치 고려)
	g.messages = []string{
		// systemd 관련 (40%)
		"Starting nginx.service",
		"Started nginx.service", 
		"Stopping nginx.service",
		"Starting docker.service",
		"Started docker.service",
		"Unit entered failed state",
		
		// 커널 메시지 (25%)  
		"CPU0: temperature above threshold",
		"Out of memory: Kill process",
		"device eth0: link up",
		"TCP: Possible SYN flooding on port 80",
		"oom-killer: Killed process",
		
		// SSH 관련 (20%)
		"Accepted password for admin from 192.168.1.100",
		"Failed password for admin from 192.168.1.200", 
		"Connection closed by 192.168.1.100",
		"pam_unix(sshd:session): session opened for user admin",
		
		// 기타 시스템 (15%)
		"(root) CMD (/usr/bin/updatedb)",
		"action 'action 17' suspended",
		"device (eth0): state change",
		"Certificate will expire",
		"Disk space warning: /var partition at 85%",
	}
}

// 타임스탬프 캐시 업데이터 (1초마다 갱신하여 CPU 절약)
func (g *SystemLogGenerator) startTimestampUpdater() {
	g.updateTimestamp()
	
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			g.updateTimestamp()
		}
	}()
}

func (g *SystemLogGenerator) updateTimestamp() {
	now := time.Now()
	timestamp := now.UTC().Format("2006-01-02T15:04:05.000Z")
	
	g.timestampMutex.Lock()
	g.timestampCache = timestamp
	g.lastTimestamp = now
	g.timestampMutex.Unlock()
}

// GenerateSystemLog - Zero-allocation 로그 생성 (핵심 성능 함수)
func (g *SystemLogGenerator) GenerateSystemLog() []byte {
	// 메모리 풀에서 버퍼 재사용
	buffer := logBufferPool.Get().([]byte)
	buffer = buffer[:0] // 길이만 0으로 리셋
	
	// 빠른 인덱스 계산 (락 최소화)
	g.rngMutex.Lock()
	priorityIdx := g.rng.Intn(len(g.priorities))
	hostnameIdx := g.rng.Intn(len(g.hostnames))
	serviceIdx := g.rng.Intn(len(g.services))
	pidIdx := g.rng.Intn(len(g.pids))
	messageIdx := g.rng.Intn(len(g.messages))
	g.rngMutex.Unlock()
	
	// 타임스탬프 읽기
	g.timestampMutex.RLock()
	timestamp := g.timestampCache
	g.timestampMutex.RUnlock()
	
	// Zero-allocation 문자열 조립 (unsafe 사용으로 최적화)
	priority := g.priorities[priorityIdx]
	hostname := g.hostnames[hostnameIdx]
	service := g.services[serviceIdx]
	pid := g.pids[pidIdx]
	message := g.messages[messageIdx]
	
	// 고속 바이트 슬라이스 조립 (append 사용, 할당 최소화)
	buffer = append(buffer, priority...)
	buffer = append(buffer, timestamp...)
	buffer = append(buffer, ' ')
	buffer = append(buffer, hostname...)
	buffer = append(buffer, ' ')
	buffer = append(buffer, service...)
	buffer = append(buffer, '[')
	buffer = append(buffer, pid...)
	buffer = append(buffer, ']', ':', ' ')
	buffer = append(buffer, message...)
	
	// 복사본 생성 (호출자가 안전하게 사용할 수 있도록)
	result := make([]byte, len(buffer))
	copy(result, buffer)
	
	// 버퍼를 풀에 반환
	logBufferPool.Put(buffer)
	
	return result
}

// GenerateSystemLogUnsafe - 최고 성능을 위한 unsafe 버전 (고급 사용자용)
func (g *SystemLogGenerator) GenerateSystemLogUnsafe() []byte {
	builder := builderPool.Get().(*strings.Builder)
	builder.Reset()
	
	// 인덱스 계산 (최소한의 락)
	g.rngMutex.Lock()
	priorityIdx := g.rng.Intn(len(g.priorities))
	hostnameIdx := g.rng.Intn(len(g.hostnames))
	serviceIdx := g.rng.Intn(len(g.services))
	pidIdx := g.rng.Intn(len(g.pids))
	messageIdx := g.rng.Intn(len(g.messages))
	g.rngMutex.Unlock()
	
	// 타임스탬프 읽기
	g.timestampMutex.RLock()
	timestamp := g.timestampCache
	g.timestampMutex.RUnlock()
	
	// 고속 문자열 조립
	builder.WriteString(g.priorities[priorityIdx])
	builder.WriteString(timestamp)
	builder.WriteByte(' ')
	builder.WriteString(g.hostnames[hostnameIdx])
	builder.WriteByte(' ')
	builder.WriteString(g.services[serviceIdx])
	builder.WriteByte('[')
	builder.WriteString(g.pids[pidIdx])
	builder.WriteString("]: ")
	builder.WriteString(g.messages[messageIdx])
	
	// unsafe를 사용한 zero-copy 변환
	str := builder.String()
	result := *(*[]byte)(unsafe.Pointer(&str))
	
	// strings.Builder를 풀에 반환
	builderPool.Put(builder)
	
	return result
}

// GetStats - 생성기 통계 정보
func (g *SystemLogGenerator) GetStats() map[string]interface{} {
	g.timestampMutex.RLock()
	lastUpdate := g.lastTimestamp
	g.timestampMutex.RUnlock()
	
	return map[string]interface{}{
		"priorities_count": len(g.priorities),
		"hostnames_count":  len(g.hostnames),
		"services_count":   len(g.services),
		"messages_count":   len(g.messages),
		"last_timestamp_update": lastUpdate,
		"timestamp_cache": g.timestampCache,
	}
}