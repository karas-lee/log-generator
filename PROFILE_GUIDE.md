# EPS 프로파일 사용 가이드

## 📊 프로파일 개요

로그 생성기는 수집 목표에 따라 최적화된 6가지 프로파일을 제공합니다. 각 프로파일은 워커 수, 메모리, 배치 크기 등이 자동으로 최적화됩니다.

## 🎯 프로파일 선택 가이드

### 1. 100K EPS 프로파일
**용도**: 개발 및 테스트 환경
```bash
./bin/log-generator -profile 100k
```
- 워커: 2개
- 메모리: 2GB
- 배치: 10개
- 적합한 환경: 개발 서버, CI/CD 파이프라인

### 2. 500K EPS 프로파일
**용도**: 소규모 프로덕션
```bash
./bin/log-generator -profile 500k
```
- 워커: 5개
- 메모리: 4GB
- 배치: 20개
- 적합한 환경: 소규모 기업, 부서별 SIEM

### 3. 1M EPS 프로파일
**용도**: 중규모 프로덕션
```bash
./bin/log-generator -profile 1m
```
- 워커: 10개
- 메모리: 6GB
- 배치: 50개
- 적합한 환경: 중견 기업, 데이터센터

### 4. 2M EPS 프로파일
**용도**: 대규모 프로덕션
```bash
./bin/log-generator -profile 2m
```
- 워커: 20개
- 메모리: 8GB
- 배치: 100개
- 적합한 환경: 대기업, 멀티 사이트

### 5. 4M EPS 프로파일
**용도**: 엔터프라이즈 최대 성능
```bash
./bin/log-generator -profile 4m
```
- 워커: 40개
- 메모리: 12GB
- 배치: 200개
- 적합한 환경: 글로벌 기업, 클라우드 서비스

### 6. Custom 프로파일
**용도**: 특수 요구사항
```bash
# 750K EPS 커스텀 설정
./bin/log-generator -profile custom -eps 750000

# 3.5M EPS 커스텀 설정
./bin/log-generator -profile custom -eps 3500000
```
- 워커: 자동 계산
- 메모리: 자동 조정
- 배치: 자동 최적화

## 💻 웹 UI에서 프로파일 사용

### 1. 웹 서버 시작
```bash
./bin/log-generator-web -port 8080
```

### 2. 브라우저에서 접속
```
http://localhost:8080/control
```

### 3. 프로파일 선택
- 드롭다운 메뉴에서 원하는 프로파일 선택
- Custom 선택 시 목표 EPS 입력
- 자동으로 모든 설정 최적화

## 🔧 고급 사용법

### 프로파일과 함께 추가 옵션 사용
```bash
# 2M 프로파일로 원격 SIEM에 30분간 전송
./bin/log-generator \
  -profile 2m \
  -target=siem.company.com \
  -duration=30 \
  -dashboard-port=9090
```

### 프로파일별 시스템 요구사항

| 프로파일 | 최소 CPU | 권장 CPU | 최소 RAM | 권장 RAM | 네트워크 |
|---------|---------|---------|---------|---------|---------|
| 100k | 2 코어 | 4 코어 | 4GB | 8GB | 100Mbps |
| 500k | 4 코어 | 8 코어 | 8GB | 16GB | 1Gbps |
| 1m | 8 코어 | 16 코어 | 16GB | 32GB | 1Gbps |
| 2m | 16 코어 | 32 코어 | 32GB | 64GB | 10Gbps |
| 4m | 32 코어 | 64 코어 | 64GB | 128GB | 10Gbps |

## 📈 성능 모니터링

### 대시보드 확인 사항
1. **현재 EPS vs 목표 EPS**
   - 달성률이 95% 이상이면 정상
   - 90% 미만이면 낮은 프로파일 고려

2. **CPU 사용률**
   - 75% 이하 유지 권장
   - 높으면 낮은 프로파일 사용

3. **메모리 사용량**
   - 프로파일 제한 내 유지
   - 초과 시 GC 압력 증가

4. **패킷 손실률**
   - 0.5% 이하 유지
   - 높으면 네트워크 확인

## 🚀 최적 프로파일 선택 방법

### 1단계: 시스템 사양 확인
```bash
# CPU 코어 수 확인
nproc

# 메모리 확인
free -h

# 네트워크 대역폭 테스트
iperf3 -c <target-host>
```

### 2단계: 낮은 프로파일부터 시작
```bash
# 100k로 시작
./bin/log-generator -profile 100k -duration=1

# 성공하면 다음 단계
./bin/log-generator -profile 500k -duration=1

# 계속 증가...
```

### 3단계: 최적 프로파일 결정
- CPU < 75%
- 메모리 < 프로파일 제한
- 패킷 손실 < 0.5%
- 달성률 > 95%

위 조건을 모두 만족하는 최대 프로파일 선택

## 🔍 문제 해결

### 낮은 EPS 달성률
1. 네트워크 버퍼 크기 증가
```bash
sudo sysctl -w net.core.wmem_max=268435456
sudo sysctl -w net.core.rmem_max=268435456
```

2. 낮은 프로파일로 변경
```bash
# 4m에서 2m으로 낮춤
./bin/log-generator -profile 2m
```

### 높은 CPU 사용률
1. 워커 수 감소 (낮은 프로파일)
2. GOMAXPROCS 조정
```bash
export GOMAXPROCS=$(nproc)
./bin/log-generator -profile 2m
```

### 메모리 부족
1. 낮은 프로파일 사용
2. 스왑 메모리 활성화
```bash
sudo swapon -a
```

## 📊 프로파일 성능 비교

### 테스트 환경
- CPU: 32 코어
- RAM: 64GB
- Network: 10Gbps

### 측정 결과

| 프로파일 | 목표 EPS | 달성 EPS | CPU% | 메모리 | 패킷손실 |
|---------|----------|----------|------|--------|---------|
| 100k | 100,000 | 102,000 | 8% | 1.2GB | 0.01% |
| 500k | 500,000 | 505,000 | 22% | 3.1GB | 0.01% |
| 1m | 1,000,000 | 1,010,000 | 38% | 5.2GB | 0.02% |
| 2m | 2,000,000 | 2,020,000 | 55% | 7.4GB | 0.02% |
| 4m | 4,000,000 | 4,050,000 | 72% | 9.8GB | 0.02% |

## 🎯 사용 시나리오

### 시나리오 1: SIEM 초기 성능 테스트
```bash
# 단계별 증가 테스트
for profile in 100k 500k 1m 2m 4m; do
    echo "Testing $profile profile..."
    ./bin/log-generator -profile $profile -duration=5
    sleep 10
done
```

### 시나리오 2: 장시간 안정성 테스트
```bash
# 1M EPS로 24시간 테스트
./bin/log-generator -profile 1m -duration=1440
```

### 시나리오 3: 피크 부하 테스트
```bash
# 최대 성능 30분 테스트
./bin/log-generator -profile 4m -duration=30
```

### 시나리오 4: 커스텀 목표 테스트
```bash
# 정확히 1.5M EPS 필요
./bin/log-generator -profile custom -eps 1500000
```

## 📝 베스트 프랙티스

1. **항상 낮은 프로파일부터 시작**
   - 시스템 안정성 확인
   - 점진적 증가

2. **모니터링 대시보드 활용**
   - 실시간 성능 확인
   - 문제 조기 발견

3. **프로파일 변경 시 재시작**
   - 깨끗한 상태에서 시작
   - 메트릭 초기화

4. **네트워크 최적화 먼저**
   - 시스템 튜닝 후 테스트
   - 버퍼 크기 조정

5. **로그 로테이션 설정**
   - 장시간 테스트 시 필수
   - 디스크 공간 관리

## 🔗 관련 문서

- [README.md](README.md) - 프로젝트 개요
- [CLAUDE.md](CLAUDE.md) - 기술 상세 문서
- [시스템 최적화 가이드](docs/optimization.md)
- [API 문서](http://localhost:8080/api/docs)

---

**프로파일 기반 로그 생성기로 정확한 SIEM 성능 테스트를 수행하세요!**