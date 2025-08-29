#!/bin/bash

# 400만 EPS 로그 전송기 성능 테스트 스크립트
# PRD 요구사항 기반 검증 자동화

set -e

# 컬러 출력
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}"
echo "=================================================================="
echo "   400만 EPS 로그 전송기 성능 테스트 도구"
echo "   PRD 명세 기반 자동 검증 시스템"
echo "=================================================================="
echo -e "${NC}"

# 테스트 설정
TARGET_EPS=4000000        # 목표 EPS
MIN_ACHIEVEMENT=95        # 최소 달성률 (95%)
TEST_DURATION=1800        # 테스트 지속 시간 (30분)
DASHBOARD_PORT=8080       # 대시보드 포트
LOG_DIR="./test_logs"     # 로그 디렉토리

# 디렉토리 생성
mkdir -p $LOG_DIR

echo -e "${YELLOW}📋 테스트 설정:${NC}"
echo "   목표 EPS: $TARGET_EPS"
echo "   최소 달성률: $MIN_ACHIEVEMENT%"
echo "   테스트 시간: $(($TEST_DURATION / 60))분"
echo "   대시보드: http://localhost:$DASHBOARD_PORT"
echo ""

# 1. 시스템 사전 점검
echo -e "${BLUE}🔍 1. 시스템 사전 점검${NC}"

# Go 설치 확인
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go가 설치되지 않았습니다.${NC}"
    exit 1
fi
echo "✅ Go $(go version | cut -d' ' -f3) 확인"

# CPU 코어 수 확인
CPU_CORES=$(nproc)
echo "✅ CPU 코어: $CPU_CORES개"

if [ $CPU_CORES -lt 16 ]; then
    echo -e "${YELLOW}⚠️  권장 최소 CPU 코어: 16개 (현재: $CPU_CORES개)${NC}"
fi

# 메모리 확인
MEMORY_GB=$(free -g | awk 'NR==2{print $2}')
echo "✅ 시스템 메모리: ${MEMORY_GB}GB"

if [ $MEMORY_GB -lt 16 ]; then
    echo -e "${YELLOW}⚠️  권장 최소 메모리: 16GB (현재: ${MEMORY_GB}GB)${NC}"
fi

# 2. 네트워크 최적화 적용
echo -e "${BLUE}🔧 2. 네트워크 최적화 적용${NC}"

# 루트 권한 확인
if [[ $EUID -eq 0 ]]; then
    echo "✅ 루트 권한으로 실행 중 - 네트워크 최적화 적용"
    
    # 소켓 버퍼 크기 증가
    sysctl -w net.core.wmem_max=268435456      # 256MB
    sysctl -w net.core.rmem_max=268435456      # 256MB
    sysctl -w net.core.wmem_default=1048576    # 1MB
    sysctl -w net.core.rmem_default=1048576    # 1MB
    
    # 네트워크 백로그 큐 크기
    sysctl -w net.core.netdev_max_backlog=30000
    
    # UDP 버퍼 설정
    sysctl -w net.ipv4.udp_mem="102400 873800 16777216"
    
    # 로컬 포트 범위 확대
    sysctl -w net.ipv4.ip_local_port_range="1024 65535"
    
    echo "✅ 커널 파라미터 최적화 완료"
else
    echo -e "${YELLOW}⚠️  루트 권한이 아닙니다. 네트워크 최적화를 위해 sudo로 실행을 권장합니다.${NC}"
fi

# 3. 빌드
echo -e "${BLUE}🔨 3. 애플리케이션 빌드${NC}"
cd "$(dirname "$0")/.."

if ! go build -o bin/log-generator -ldflags="-s -w" cmd/main.go; then
    echo -e "${RED}❌ 빌드 실패${NC}"
    exit 1
fi
echo "✅ 빌드 완료: bin/log-generator"

# 4. 더미 SIEM 서버 시작 (UDP 수신)
echo -e "${BLUE}📡 4. 더미 SIEM 서버 시작${NC}"

# 더미 서버 스크립트 생성
cat > $LOG_DIR/dummy_siem.py << 'EOF'
#!/usr/bin/env python3
import socket
import threading
import time
import sys

class DummySIEM:
    def __init__(self, start_port=514, end_port=553):
        self.start_port = start_port
        self.end_port = end_port
        self.sockets = []
        self.running = True
        self.total_received = 0
        self.last_count = 0
        self.last_time = time.time()
    
    def create_udp_server(self, port):
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.settimeout(1.0)  # 1초 타임아웃
        try:
            sock.bind(('127.0.0.1', port))
            return sock
        except:
            print(f"포트 {port} 바인딩 실패")
            return None
    
    def receiver_thread(self, sock, port):
        buffer = bytearray(65536)  # 64KB 버퍼
        while self.running:
            try:
                data, addr = sock.recvfrom_into(buffer)
                if data:
                    # 로그 개수 계산 (개행 문자로 구분)
                    log_count = buffer[:data].count(b'\n') + 1
                    self.total_received += log_count
            except socket.timeout:
                continue
            except:
                break
    
    def stats_thread(self):
        while self.running:
            time.sleep(1)
            current_time = time.time()
            current_count = self.total_received
            
            eps = (current_count - self.last_count) / (current_time - self.last_time)
            
            print(f"수신 EPS: {eps:,.0f} | 총 수신: {current_count:,}")
            
            self.last_count = current_count
            self.last_time = current_time
    
    def start(self):
        print(f"더미 SIEM 서버 시작 (포트 {self.start_port}-{self.end_port})")
        
        # UDP 서버들 생성
        for port in range(self.start_port, self.end_port + 1):
            sock = self.create_udp_server(port)
            if sock:
                self.sockets.append(sock)
                thread = threading.Thread(target=self.receiver_thread, args=(sock, port))
                thread.daemon = True
                thread.start()
        
        print(f"✅ {len(self.sockets)}개 UDP 서버 시작")
        
        # 통계 스레드 시작
        stats_thread = threading.Thread(target=self.stats_thread)
        stats_thread.daemon = True
        stats_thread.start()
        
        try:
            while self.running:
                time.sleep(1)
        except KeyboardInterrupt:
            self.stop()
    
    def stop(self):
        print("\n더미 SIEM 서버 중지 중...")
        self.running = False
        for sock in self.sockets:
            sock.close()
        print(f"총 수신 로그: {self.total_received:,}개")

if __name__ == "__main__":
    siem = DummySIEM()
    siem.start()
EOF

# Python3 확인
if ! command -v python3 &> /dev/null; then
    echo -e "${RED}❌ Python3이 설치되지 않았습니다.${NC}"
    exit 1
fi

# 더미 서버 백그라운드 실행
python3 $LOG_DIR/dummy_siem.py > $LOG_DIR/siem_output.log 2>&1 &
SIEM_PID=$!
echo "✅ 더미 SIEM 서버 시작됨 (PID: $SIEM_PID)"

# 서버 준비 대기
sleep 3

# 5. 로그 전송기 실행
echo -e "${BLUE}🚀 5. 로그 전송기 실행 (테스트 시간: $(($TEST_DURATION / 60))분)${NC}"

LOG_FILE="$LOG_DIR/generator_output.log"
./bin/log-generator \
    -target=127.0.0.1 \
    -dashboard-port=$DASHBOARD_PORT \
    -duration=$(($TEST_DURATION / 60)) \
    -optimize=true \
    > $LOG_FILE 2>&1 &

GENERATOR_PID=$!
echo "✅ 로그 전송기 시작됨 (PID: $GENERATOR_PID)"
echo "📊 대시보드: http://localhost:$DASHBOARD_PORT"
echo "📝 로그 파일: $LOG_FILE"

# 6. 실시간 모니터링
echo -e "${BLUE}📊 6. 실시간 성능 모니터링${NC}"

START_TIME=$(date +%s)
MONITOR_LOG="$LOG_DIR/monitor.log"

# 모니터링 함수
monitor_performance() {
    local max_eps=0
    local total_checks=0
    local success_checks=0
    
    while kill -0 $GENERATOR_PID 2>/dev/null; do
        sleep 10
        
        # API에서 메트릭 가져오기
        if metrics=$(curl -s http://localhost:$DASHBOARD_PORT/api/metrics); then
            current_eps=$(echo "$metrics" | python3 -c "import sys, json; data=json.load(sys.stdin); print(int(data.get('current_eps', 0)))" 2>/dev/null || echo "0")
            achievement=$(echo "$metrics" | python3 -c "import sys, json; data=json.load(sys.stdin); print('{:.1f}'.format(data.get('achievement_percent', 0)))" 2>/dev/null || echo "0.0")
            
            if [ "$current_eps" -gt "$max_eps" ]; then
                max_eps=$current_eps
            fi
            
            total_checks=$((total_checks + 1))
            if (( $(echo "$achievement >= $MIN_ACHIEVEMENT" | bc -l) )); then
                success_checks=$((success_checks + 1))
            fi
            
            current_time=$(date +%s)
            elapsed=$((current_time - START_TIME))
            
            printf "⏱️  %02d:%02d | EPS: %'d | 달성률: %s%% | 최대: %'d\n" \
                $((elapsed / 60)) $((elapsed % 60)) \
                $current_eps $achievement $max_eps
                
        else
            echo "⚠️  메트릭 API 연결 실패"
        fi
    done
    
    echo ""
    echo "최대 달성 EPS: $max_eps"
    echo "목표 달성 비율: $success_checks/$total_checks"
}

# 백그라운드 모니터링 시작
monitor_performance &
MONITOR_PID=$!

# 7. 테스트 완료 대기
wait $GENERATOR_PID
GENERATOR_EXIT_CODE=$?

# 8. 결과 분석
echo -e "${BLUE}📊 8. 테스트 결과 분석${NC}"

# 프로세스 정리
kill $SIEM_PID 2>/dev/null || true
kill $MONITOR_PID 2>/dev/null || true

# 결과 파일 생성
RESULT_FILE="$LOG_DIR/test_result_$(date +%Y%m%d_%H%M%S).json"

# API에서 최종 결과 가져오기
if final_metrics=$(curl -s http://localhost:$DASHBOARD_PORT/api/summary 2>/dev/null); then
    echo "$final_metrics" > $RESULT_FILE
    
    # 결과 출력
    max_eps=$(echo "$final_metrics" | python3 -c "import sys, json; data=json.load(sys.stdin); print(int(data.get('current_eps', 0)))" 2>/dev/null || echo "0")
    avg_eps=$(echo "$final_metrics" | python3 -c "import sys, json; data=json.load(sys.stdin); print(int(data.get('average_eps', 0)))" 2>/dev/null || echo "0")
    achievement=$(echo "$final_metrics" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('achievement_percent', 0))" 2>/dev/null || echo "0")
    
    echo "=================================================================="
    echo -e "${GREEN}🏁 최종 테스트 결과${NC}"
    echo "=================================================================="
    printf "목표 EPS:        %'14d\n" $TARGET_EPS
    printf "달성 EPS:        %'14d\n" $max_eps
    printf "평균 EPS:        %'14d\n" $avg_eps
    printf "달성률:          %13.1f%%\n" $achievement
    echo ""
    
    # 성과 평가
    if (( $(echo "$achievement >= 95" | bc -l) )); then
        echo -e "${GREEN}🎉 EXCELLENT! 목표의 95% 이상 달성!${NC}"
        TEST_RESULT="PASS"
    elif (( $(echo "$achievement >= 80" | bc -l) )); then
        echo -e "${YELLOW}👍 GOOD! 목표의 80% 이상 달성${NC}"
        TEST_RESULT="PARTIAL"
    else
        echo -e "${RED}❌ FAIL! 목표 달성률 부족${NC}"
        TEST_RESULT="FAIL"
    fi
    
else
    echo -e "${RED}❌ 최종 메트릭을 가져올 수 없습니다.${NC}"
    TEST_RESULT="ERROR"
fi

echo ""
echo "테스트 로그 위치: $LOG_DIR"
echo "결과 파일: $RESULT_FILE"

# 9. 시스템 복구
if [[ $EUID -eq 0 ]]; then
    echo -e "${BLUE}🔧 9. 시스템 설정 복구${NC}"
    sysctl -w net.core.wmem_max=212992
    sysctl -w net.core.rmem_max=212992
    sysctl -w net.core.netdev_max_backlog=1000
    echo "✅ 커널 파라미터 복구 완료"
fi

echo ""
echo "=================================================================="
case $TEST_RESULT in
    "PASS")
        echo -e "${GREEN}✅ 테스트 성공: 400만 EPS 목표 달성!${NC}"
        exit 0
        ;;
    "PARTIAL")
        echo -e "${YELLOW}⚠️  테스트 부분 성공: 성능 향상 필요${NC}"
        exit 1
        ;;
    "FAIL")
        echo -e "${RED}❌ 테스트 실패: 시스템 점검 필요${NC}"
        exit 2
        ;;
    *)
        echo -e "${RED}❌ 테스트 오류: 결과를 확인할 수 없음${NC}"
        exit 3
        ;;
esac