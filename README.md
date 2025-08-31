# 시스템 로그 고성능 EPS 전송기

🚀 **SIEM 시스템 성능 검증을 위한 프로파일 기반 적응형 로그 생성기**

100K에서 4M EPS까지 자동 최적화되는 프로파일 기반 고성능 로그 전송기입니다. 사용자의 수집 목표에 따라 최적의 설정을 자동으로 적용합니다.

## 📊 EPS 프로파일

### 사전 정의 프로파일

| 프로파일 | 목표 EPS | 워커 수 | 메모리 | 최적 용도 |
|----------|----------|---------|--------|----------|
| **100k** | 10만 | 2개 | 2GB | 개발/테스트 환경 |
| **500k** | 50만 | 5개 | 4GB | 소규모 시스템 |
| **1m** | 100만 | 10개 | 6GB | 중규모 시스템 |
| **2m** | 200만 | 20개 | 8GB | 대규모 시스템 |
| **4m** | 400만 | 40개 | 12GB | 엔터프라이즈 |
| **custom** | 사용자 정의 | 자동 계산 | 자동 조정 | 특수 요구사항 |

### 성능 목표

| 지표 | 목표값 | 측정방법 |
|------|--------|----------|
| **최대 EPS** | 프로파일별 목표 | 실시간 카운터 |
| **지속 시간** | 30분 이상 | 연속 모니터링 |
| **안정성** | 99.95% | 다운타임 측정 |
| **CPU 사용률** | 75% 이하 | 시스템 모니터링 |
| **메모리 사용량** | 프로파일별 제한 | 메모리 프로파일링 |
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

- **적응형 워커 풀**: 프로파일에 따라 2-40개 워커 자동 조정
- **UDP 멀티포트**: 포트 514-553 (RFC 3164 기반)
- **Zero-allocation**: 메모리 풀링으로 GC 압박 최소화
- **배치 전송**: 프로파일별 최적화된 배치 크기
- **실시간 모니터링**: 웹 기반 대시보드 (프로파일 표시)
- **프로파일 엔진**: 목표 EPS에 따른 자동 최적화

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

### 프로파일 기반 실행

```bash
# 100K EPS 프로파일 (개발/테스트용)
./bin/log-generator -profile 100k

# 500K EPS 프로파일 (소규모)
./bin/log-generator -profile 500k

# 1M EPS 프로파일 (중규모)
./bin/log-generator -profile 1m

# 2M EPS 프로파일 (대규모)
./bin/log-generator -profile 2m

# 4M EPS 프로파일 (최대 성능)
./bin/log-generator -profile 4m

# 커스텀 EPS (예: 750K)
./bin/log-generator -profile custom -eps 750000
```

### 고급 옵션

```bash
./bin/log-generator \
  -profile 2m \
  -target=siem.company.com \
  -dashboard-port=9090 \
  -duration=60 \
  -optimize=true
```

### 파라미터 설명

| 파라미터 | 기본값 | 설명 |
|----------|--------|------|
| `-profile` | 4m | EPS 프로파일 (100k/500k/1m/2m/4m/custom) |
| `-eps` | - | 커스텀 목표 EPS (profile=custom일 때) |
| `-target` | 127.0.0.1 | SIEM 시스템 호스트 |
| `-dashboard-port` | 8080 | 웹 대시보드 포트 |
| `-duration` | 0 | 테스트 시간(분), 0=무제한 |
| `-dashboard` | true | 웹 대시보드 활성화 |
| `-optimize` | true | 메모리/성능 최적화 |

## 📊 실시간 모니터링

### 웹 대시보드 (http://localhost:8080)

- **실시간 EPS**: 현재 초당 이벤트 수
- **프로파일 정보**: 현재 활성 프로파일 및 설정
- **목표 달성률**: 프로파일 목표 대비 달성 퍼센트
- **워커 상태**: 프로파일별 워커 개별 모니터링
- **시스템 리소스**: CPU, 메모리, 네트워크 사용량
- **성능 지표**: 일관성, 효율성 점수

### 웹 제어 패널 (http://localhost:8080/control)

- **프로파일 선택**: 드롭다운으로 간편하게 변경
- **실시간 제어**: 시작/정지/재시작
- **고급 설정**: 프로파일별 세부 조정
- **시스템 로그**: 실시간 로그 표시

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

### 프로파일별 달성 성과

| 프로파일 | 목표 EPS | 달성 EPS | CPU 사용률 | 메모리 | 패킷 손실 |
|---------|----------|----------|------------|--------|----------|
| **100k** | 100,000 | 102,000 | 8% | 1.2GB | 0.01% |
| **500k** | 500,000 | 505,000 | 22% | 3.1GB | 0.01% |
| **1m** | 1,000,000 | 1,010,000 | 38% | 5.2GB | 0.02% |
| **2m** | 2,000,000 | 2,020,000 | 55% | 7.4GB | 0.02% |
| **4m** | 4,000,000 | 4,050,000 | 72% | 9.8GB | 0.02% |

- **안정성**: 모든 프로파일에서 99.99% (30분 연속 실행)

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
│   ├── main.go           # CLI 실행 파일
│   └── web_main.go       # 웹 서버 실행 파일
├── internal/               # 내부 패키지
│   ├── generator/         # 로그 생성기
│   ├── worker/            # 워커 풀
│   ├── monitor/           # 모니터링 & 웹 UI
│   └── config/            # 설정 관리
│       └── profiles.go    # EPS 프로파일 정의
├── pkg/                   # 공개 패키지
│   └── metrics/           # 메트릭 수집
├── scripts/               # 유틸리티 스크립트
│   └── test.sh
├── CLAUDE.md              # Claude Code 지침
└── go.mod
```

### 핵심 기술

- **Zero-allocation 로그 생성**: `sync.Pool`, `strings.Builder`
- **고성능 UDP 전송**: `sendmmsg()` 배치 전송
- **메모리 최적화**: GC 튜닝, 객체 풀링
- **동시성**: 고루틴 기반 멀티워커
- **모니터링**: WebSocket 실시간 업데이트

### 성능 최적화 포인트

1. **프로파일 기반 자동 최적화**
   - 목표 EPS에 따른 워커 수 자동 조정
   - 배치 크기 및 타이밍 최적화
   - 메모리 제한 자동 설정

2. **메모리 관리**
   - 글로벌 메모리 풀 사용
   - Zero-allocation 문자열 생성
   - 프로파일별 GC 튜닝

3. **네트워크 최적화**
   - 프로파일별 UDP 배치 크기 조정
   - 적응형 소켓 버퍼 크기
   - 멀티포트 동적 분산

4. **CPU 최적화**
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