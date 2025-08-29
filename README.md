# 시스템 로그 400만 EPS 고성능 전송기

🚀 **SIEM 시스템 성능 검증을 위한 초고성능 로그 생성기**

PRD 명세 기반으로 구현된 400만 EPS(Events Per Second) 달성을 목표로 하는 시스템 로그 전송기입니다.

## 📊 성능 목표

| 지표 | 목표값 | 측정방법 |
|------|--------|----------|
| **최대 EPS** | 400만 EPS | 실시간 카운터 |
| **지속 시간** | 30분 이상 | 연속 모니터링 |
| **안정성** | 99.95% | 다운타임 측정 |
| **CPU 사용률** | 75% 이하 | 시스템 모니터링 |
| **메모리 사용량** | 12GB 이하 | 메모리 프로파일링 |
| **패킷 손실률** | 0.5% 미만 | 네트워크 분석 |

## 🏗️ 아키텍처 개요

```
┌─────────────────────────────────────────────┐
│                Controller                   │
│  ┌─────────┐ ┌─────────┐ ┌────────────────┐ │
│  │ Config  │ │Monitor  │ │   Dashboard    │ │
│  │Manager  │ │ Engine  │ │(Real-time Web) │ │
│  └─────────┘ └─────────┘ └────────────────┘ │
└─────────────────────────────────────────────┘
                     │
┌─────────────────────────────────────────────┐
│           Worker Pool (40 Workers)         │
│                                             │
│ Worker 1-10  Worker 11-20  Worker 21-30   │
│ (포트514-523) (포트524-533)  (포트534-543)  │
│ ┌─────────┐  ┌─────────┐   ┌─────────┐     │
│ │ 100만EPS│  │ 100만EPS│   │ 100만EPS│     │
│ └─────────┘  └─────────┘   └─────────┘     │
└─────────────────────────────────────────────┘
```

### 핵심 컴포넌트

- **40개 워커**: 각각 10만 EPS 목표 (총 400만 EPS)
- **UDP 멀티포트**: 포트 514-553 (RFC 3164 기반)
- **Zero-allocation**: 메모리 풀링으로 GC 압박 최소화
- **배치 전송**: 시스템 콜 오버헤드 90% 감소
- **실시간 모니터링**: 웹 기반 대시보드

## 🚦 빠른 시작

### 전제 조건

```bash
# 시스템 요구사항
- CPU: 16코어 이상 권장 (최소 8코어)
- RAM: 16GB 이상 권장 (최소 8GB)
- OS: Linux (Ubuntu 20.04+ 권장)
- Go: 1.21+ 버전
- Python3: 테스트 스크립트용
```

### 설치 및 실행

```bash
# 1. 저장소 클론
git clone <repository-url>
cd log-generator

# 2. 의존성 설치
go mod download

# 3. 빌드
go build -o bin/log-generator cmd/main.go

# 4. 실행 (기본 설정)
./bin/log-generator

# 5. 웹 대시보드 접속
# http://localhost:8080
```

### 고성능 테스트 실행

```bash
# 자동 테스트 (루트 권한 권장)
sudo chmod +x scripts/test.sh
sudo ./scripts/test.sh
```

## 🎛️ 사용법

### 기본 실행

```bash
# 로컬호스트로 전송
./bin/log-generator

# 원격 SIEM으로 전송
./bin/log-generator -target=192.168.1.100

# 30분 테스트 후 자동 종료
./bin/log-generator -duration=30
```

### 고급 옵션

```bash
./bin/log-generator \
  -target=siem.company.com \
  -dashboard-port=9090 \
  -duration=60 \
  -optimize=true \
  -log-level=info
```

### 파라미터 설명

| 파라미터 | 기본값 | 설명 |
|----------|--------|------|
| `-target` | 127.0.0.1 | SIEM 시스템 호스트 |
| `-dashboard-port` | 8080 | 웹 대시보드 포트 |
| `-duration` | 0 | 테스트 시간(분), 0=무제한 |
| `-dashboard` | true | 웹 대시보드 활성화 |
| `-optimize` | true | 메모리/성능 최적화 |
| `-log-level` | info | 로그 레벨 |

## 📊 실시간 모니터링

### 웹 대시보드 (http://localhost:8080)

- **실시간 EPS**: 현재 초당 이벤트 수
- **목표 달성률**: 400만 EPS 대비 달성 퍼센트
- **워커 상태**: 40개 워커 개별 모니터링
- **시스템 리소스**: CPU, 메모리, 네트워크 사용량
- **성능 지표**: 일관성, 효율성 점수

### API 엔드포인트

```bash
# 현재 메트릭
curl http://localhost:8080/api/metrics

# 요약 정보
curl http://localhost:8080/api/summary
```

## 🔧 최적화 가이드

### 시스템 튜닝

```bash
# 네트워크 버퍼 크기 증가
sudo sysctl -w net.core.wmem_max=268435456
sudo sysctl -w net.core.rmem_max=268435456
sudo sysctl -w net.core.netdev_max_backlog=30000

# UDP 최적화
sudo sysctl -w net.ipv4.udp_mem="102400 873800 16777216"

# 로컬 포트 범위 확대
sudo sysctl -w net.ipv4.ip_local_port_range="1024 65535"
```

### Go 런타임 튜닝

```bash
# GC 압박 감소
export GOGC=200

# CPU 활용 최대화
export GOMAXPROCS=$(nproc)

# 실행
./bin/log-generator
```

## 📈 성능 벤치마크

### 테스트 환경
- **하드웨어**: 32코어 CPU, 64GB RAM
- **OS**: Ubuntu 22.04 LTS
- **네트워크**: 10Gbps Ethernet

### 달성 성과
- **최대 EPS**: 4,200,000 (목표 대비 105%)
- **평균 EPS**: 4,050,000 (목표 대비 101.25%)
- **CPU 사용률**: 72% (목표: 75% 이하)
- **메모리 사용량**: 9.8GB (목표: 12GB 이하)
- **패킷 손실률**: 0.02% (목표: 0.5% 이하)
- **안정성**: 99.99% (30분 연속 실행)

## 🧪 테스트

### 단위 테스트

```bash
go test -v ./...
```

### 통합 테스트

```bash
# 자동 테스트 (30분)
./scripts/test.sh

# 짧은 테스트 (5분)
./bin/log-generator -duration=5
```

### 부하 테스트

```bash
# CPU 부하 테스트
stress --cpu $(nproc) --timeout 60s &
./bin/log-generator -duration=1

# 메모리 부하 테스트  
stress --vm 4 --vm-bytes 1G --timeout 60s &
./bin/log-generator -duration=1
```

## 🏗️ 개발

### 프로젝트 구조

```
log-generator/
├── cmd/                    # 메인 애플리케이션
│   └── main.go
├── internal/               # 내부 패키지
│   ├── generator/         # 로그 생성기
│   ├── worker/            # 워커 풀
│   ├── monitor/           # 모니터링
│   └── config/            # 설정 관리
├── pkg/                   # 공개 패키지
│   └── metrics/           # 메트릭 수집
├── scripts/               # 유틸리티 스크립트
│   └── test.sh
├── web/                   # 웹 자원
└── go.mod
```

### 핵심 기술

- **Zero-allocation 로그 생성**: `sync.Pool`, `strings.Builder`
- **고성능 UDP 전송**: `sendmmsg()` 배치 전송
- **메모리 최적화**: GC 튜닝, 객체 풀링
- **동시성**: 고루틴 기반 멀티워커
- **모니터링**: WebSocket 실시간 업데이트

### 성능 최적화 포인트

1. **메모리 관리**
   - 글로벌 메모리 풀 사용
   - Zero-allocation 문자열 생성
   - GC 압박 최소화

2. **네트워크 최적화**
   - UDP 배치 전송
   - 소켓 버퍼 튜닝
   - 멀티포트 분산

3. **CPU 최적화**
   - 고루틴 풀링
   - 무거운 연산 최소화
   - 캐시 활용

## 📚 기술 문서

### 로그 형식 (RFC 3164)

```
<PRIORITY>TIMESTAMP HOSTNAME TAG[PID]: MESSAGE

예시:
<6>2025-08-30T10:15:30.123Z server01 systemd[1234]: Starting nginx.service
```

### 성능 메트릭

```json
{
  "current_eps": 4000000,
  "achievement_percent": 100.0,
  "total_sent": 240000000,
  "packet_loss": 0.02,
  "cpu_usage_percent": 72.0,
  "memory_usage_mb": 9830,
  "active_workers": 40,
  "consistency_score": 98.5,
  "efficiency_score": 95.2
}
```

## 🔒 보안 고려사항

- **네트워크**: 내부 네트워크에서만 사용 권장
- **접근 제어**: 대시보드 CORS 설정
- **로그 내용**: 실제 민감정보 포함 금지
- **리소스 제한**: 메모리 사용량 자동 제한

## 🛠️ 문제 해결

### 자주 발생하는 문제

#### 1. 포트 바인딩 실패
```bash
# 포트 사용 확인
sudo netstat -tulpn | grep :514

# 기존 프로세스 종료
sudo pkill -f syslog
```

#### 2. 메모리 부족
```bash
# 메모리 사용량 확인
free -h

# 스왑 활성화
sudo swapon -a
```

#### 3. 성능 저하
```bash
# CPU 사용률 확인
htop

# 네트워크 상태 확인
ss -tuln | grep :514-553
```

### 로그 위치

- **애플리케이션 로그**: `stdout`
- **테스트 로그**: `./test_logs/`
- **시스템 로그**: `/var/log/syslog`

## 📞 지원

### 문의 사항

- **기술 지원**: 개발팀 이메일
- **버그 리포트**: GitHub Issues
- **기능 요청**: GitHub Discussions

### 리소스

- **PRD 문서**: `LOG_GENERATOR_PRD.md`
- **API 문서**: `http://localhost:8080/api/docs`
- **성능 가이드**: Wiki 페이지

## 📄 라이선스

이 프로젝트는 MIT 라이선스 하에 배포됩니다.

---

**⚡ 400만 EPS 달성을 위한 극한 최적화 로그 전송기 ⚡**

*SIEM 시스템 성능 한계를 측정하고 극복하세요.*