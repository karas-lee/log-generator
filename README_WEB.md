# 🌐 웹 UI 기반 로그 생성기 제어 시스템

**400만 EPS 로그 전송기를 웹 브라우저로 완전 제어하세요!**

## 🚀 빠른 시작

### 1. 웹 애플리케이션 빌드 및 실행

```bash
# 웹 버전 빌드
make build-web

# 웹 서버 실행 
make run-web

# 브라우저에서 접속
open http://localhost:8080
```

### 2. 즉시 사용 가능한 기능

✅ **원클릭 시작/정지**: 버튼 하나로 400만 EPS 로그 전송 시작  
✅ **실시간 설정**: 목표 EPS, 워커 수, 호스트 변경 즉시 적용  
✅ **시각적 모니터링**: 40개 워커 상태를 실시간 그래프로 확인  
✅ **고급 튜닝**: 메모리, GC, 배치 크기 등 전문가급 설정  
✅ **시스템 최적화**: 네트워크 커널 파라미터 자동 적용  

## 🎛️ 웹 UI 주요 기능

### 메인 제어 대시보드
![대시보드 예시](http://localhost:8080)

**좌측 제어 패널:**
- 🟢 **시작/정지/재시작** 버튼
- ⚙️ **기본 설정**: 목표 호스트, EPS, 실행시간, 워커수
- 🔧 **고급 설정**: 배치크기, 메모리제한, GC 튜닝

**우측 모니터링 영역:**
- 📊 **실시간 성능 메트릭**: 현재 EPS, 달성률, 총 전송량
- 🔧 **워커 상태 그리드**: 40개 워커 개별 상태 (활성/비활성)
- 📝 **시스템 로그**: 실시간 이벤트 및 오류 로그

### API 엔드포인트

```bash
# 현재 상태 조회
curl http://localhost:8080/api/status

# 설정 조회/저장
curl http://localhost:8080/api/config
curl -X POST -d '{"target_eps": 5000000}' http://localhost:8080/api/config

# 제어 명령
curl -X POST http://localhost:8080/api/start
curl -X POST http://localhost:8080/api/stop
curl -X POST http://localhost:8080/api/restart

# 실시간 메트릭
curl http://localhost:8080/api/metrics

# 워커 상태
curl http://localhost:8080/api/workers

# 시스템 최적화
curl -X POST http://localhost:8080/api/system-optimize
```

## 🎯 사용 시나리오

### 시나리오 1: 빠른 테스트
1. 웹 브라우저로 `http://localhost:8080` 접속
2. 기본 설정 확인 (127.0.0.1, 400만 EPS)
3. **▶️ 시작** 버튼 클릭
4. 실시간 성능 그래프 확인
5. **⏹️ 정지** 버튼으로 중지

### 시나리오 2: 원격 SIEM 테스트
1. **목표 호스트**를 실제 SIEM 서버로 변경
2. **워커 수** 및 **목표 EPS** 조정
3. **💾 설정 저장** 클릭
4. **시스템 최적화** 버튼으로 네트워크 튜닝
5. **▶️ 시작** 버튼으로 테스트 시작

### 시나리오 3: 성능 튜닝
1. **🔧 고급 설정 표시/숨김** 클릭
2. **배치 크기** (기본 1000) 조정
3. **메모리 제한** (기본 12GB) 설정
4. **GC 퍼센트** (기본 200) 튜닝
5. 실시간으로 성능 변화 모니터링

## 📱 간편 제어 UI

임베드 가능한 경량화 버전: `http://localhost:8080/control`

```html
<iframe src="http://localhost:8080/control" 
        width="450" height="600" 
        frameborder="0">
</iframe>
```

**간편 UI 기능:**
- ✓ 기본 설정 (호스트, EPS, 시간)
- ✓ 시작/정지/재시작 버튼
- ✓ 현재 상태 표시
- ✓ 실시간 EPS 및 워커 수 표시

## 🔧 고급 설정 가이드

### 성능 최적화 설정

| 항목 | 기본값 | 추천값 | 설명 |
|------|--------|--------|------|
| **목표 EPS** | 4,000,000 | 1M-10M | 전체 시스템 목표 |
| **워커 수** | 40 | 10-100 | CPU 코어 수 고려 |
| **배치 크기** | 1000 | 500-5000 | 네트워크 효율성 |
| **전송 간격** | 10ms | 1-100ms | 정밀도 vs 성능 |
| **메모리 제한** | 12GB | 8-64GB | 시스템 메모리 고려 |
| **GC 퍼센트** | 200 | 100-500 | GC 압박 vs 메모리 |

### 네트워크 최적화

웹 UI의 **시스템 최적화** 버튼은 다음 커널 파라미터를 자동 적용:

```bash
net.core.wmem_max=268435456      # 256MB 송신 버퍼
net.core.rmem_max=268435456      # 256MB 수신 버퍼
net.core.netdev_max_backlog=30000
net.ipv4.udp_mem="102400 873800 16777216"
```

## 🌟 웹 UI vs CLI 버전 비교

| 기능 | 웹 UI | CLI |
|------|-------|-----|
| **사용 편의성** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ |
| **실시간 모니터링** | ⭐⭐⭐⭐⭐ | ⭐⭐ |
| **설정 변경** | ⭐⭐⭐⭐⭐ | ⭐⭐ |
| **원격 제어** | ⭐⭐⭐⭐⭐ | ⭐ |
| **스크립팅** | ⭐⭐ | ⭐⭐⭐⭐⭐ |
| **자동화** | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **시각화** | ⭐⭐⭐⭐⭐ | ⭐ |

## 🚨 문제 해결

### 자주 발생하는 문제

#### 1. 웹 서버 접속 불가
```bash
# 포트 사용 확인
netstat -tuln | grep :8080

# 다른 포트로 실행
./bin/log-generator-web -port=9090
```

#### 2. 로그 생성기 시작 실패
- 웹 UI 하단 **시스템 로그** 패널 확인
- 포트 514-553 사용 여부 점검
- **시스템 최적화** 버튼 클릭 (sudo 권한 필요)

#### 3. 성능이 목표에 못미침
- **고급 설정**에서 **워커 수** 증가
- **배치 크기** 조정 (1000 → 2000)
- **전송 간격** 단축 (10ms → 5ms)

### 디버깅 팁

```bash
# 웹 서버 로그 확인
./bin/log-generator-web 2>&1 | tee web-server.log

# API 응답 확인
curl -v http://localhost:8080/api/status

# 브라우저 개발자 도구에서 네트워크 탭 확인
# WebSocket 연결 상태 모니터링
```

## 📈 모니터링 가이드

### 실시간 대시보드 읽는 법

**성능 메트릭 카드:**
- 🟢 **초록색**: 목표 달성 (95% 이상)
- 🟡 **노란색**: 부분 달성 (70-95%)
- 🔴 **빨간색**: 성능 부족 (70% 미만)

**워커 상태 그리드:**
- 🟢 **활성** (밝은 초록): EPS > 0, 정상 동작
- ⚫ **비활성** (회색): EPS = 0, 대기 또는 오류

**시스템 로그 색상:**
- 🔵 **파란색**: 정보 메시지
- 🟡 **노란색**: 경고 메시지  
- 🔴 **빨간색**: 오류 메시지

### 성능 지표 해석

```json
{
  "current_eps": 4200000,        // 현재 EPS (목표: 4M)
  "achievement_percent": 105.0,   // 달성률 (105%)
  "total_sent": 252000000,       // 총 전송 로그 수
  "active_workers": 40,          // 활성 워커 수 (40/40)
  "cpu_usage_percent": 68.5,     // CPU 사용률 (목표: <75%)
  "memory_usage_mb": 10240,      // 메모리 사용량 (목표: <12GB)
  "packet_loss": 0.02           // 패킷 손실률 (목표: <0.5%)
}
```

## 🔗 통합 및 확장

### 외부 시스템 연동

```javascript
// JavaScript에서 API 호출 예시
async function startLogGenerator(config) {
    const response = await fetch('/api/config', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(config)
    });
    
    if (response.ok) {
        return await fetch('/api/start', {method: 'POST'});
    }
}

// WebSocket으로 실시간 모니터링
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onmessage = (event) => {
    const metrics = JSON.parse(event.data);
    updateDashboard(metrics);
};
```

### Docker 컨테이너 배포

```dockerfile
FROM golang:1.21-alpine AS builder
COPY . /app
WORKDIR /app
RUN make build-web

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/bin/log-generator-web /usr/local/bin/
EXPOSE 8080
CMD ["log-generator-web"]
```

```bash
# Docker 빌드 및 실행
docker build -t log-generator-web .
docker run -p 8080:8080 log-generator-web
```

---

🌐 **웹 UI로 400만 EPS를 손쉽게 달성하세요!**  
📊 **브라우저만 있으면 어디서든 완전 제어 가능합니다.**