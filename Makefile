# 400만 EPS 로그 전송기 Makefile

BINARY_NAME=log-generator
BINARY_PATH=bin/$(BINARY_NAME)
CMD_PATH=cmd/main.go

# Go 설정
GOARCH=amd64
GOOS=linux
GO_VERSION=1.21

# 빌드 플래그
LDFLAGS=-ldflags="-s -w"
BUILD_FLAGS=-trimpath

# 테스트 설정
TEST_TIMEOUT=30m
BENCH_TIME=10s

.PHONY: all build run test bench clean install deps help

# 기본 빌드
all: clean deps build build-web

# CLI 버전 빌드
build:
	@echo "🔨 CLI 애플리케이션 빌드 중..."
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BINARY_PATH) $(CMD_PATH)
	@echo "✅ CLI 빌드 완료: $(BINARY_PATH)"

# 웹 버전 빌드
build-web:
	@echo "🌐 웹 애플리케이션 빌드 중..."
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) $(LDFLAGS) -o bin/log-generator-web cmd/web_main.go
	@echo "✅ 웹 빌드 완료: bin/log-generator-web"

# 개발용 빌드 (디버그 정보 포함)
build-dev:
	@echo "🔨 개발용 빌드 중..."
	@mkdir -p bin
	@go build -race -o $(BINARY_PATH) $(CMD_PATH)
	@echo "✅ 개발용 빌드 완료: $(BINARY_PATH)"

# CLI 실행
run: build
	@echo "🚀 CLI 로그 전송기 실행..."
	@./$(BINARY_PATH)

# 웹 실행
run-web: build-web
	@echo "🌐 웹 로그 전송기 실행..."
	@./bin/log-generator-web

# 웹 데모 실행 (다른 포트)
run-web-demo: build-web
	@echo "🌐 웹 데모 실행 (포트 9090)..."
	@./bin/log-generator-web -port=9090

# 테스트 실행
run-test: build
	@echo "🧪 테스트 모드 실행 (5분)..."
	@./$(BINARY_PATH) -duration=5

# 고성능 테스트
test-performance:
	@echo "⚡ 고성능 테스트 실행..."
	@chmod +x scripts/test.sh
	@sudo ./scripts/test.sh

# 단위 테스트
test:
	@echo "🧪 단위 테스트 실행..."
	@go test -v -timeout=$(TEST_TIMEOUT) ./...

# 벤치마크 테스트
bench:
	@echo "📊 벤치마크 테스트 실행..."
	@go test -bench=. -benchtime=$(BENCH_TIME) -benchmem ./...

# 커버리지 테스트
test-coverage:
	@echo "📊 테스트 커버리지 측정..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "커버리지 리포트: coverage.html"

# 정적 분석
lint:
	@echo "🔍 코드 정적 분석..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint가 설치되지 않았습니다. 설치: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# 의존성 설치
deps:
	@echo "📦 의존성 설치 중..."
	@go mod download
	@go mod verify
	@echo "✅ 의존성 설치 완료"

# 의존성 업데이트
deps-update:
	@echo "📦 의존성 업데이트 중..."
	@go get -u ./...
	@go mod tidy
	@echo "✅ 의존성 업데이트 완료"

# 시스템 설치
install: build
	@echo "📋 시스템 설치 중..."
	@sudo cp $(BINARY_PATH) /usr/local/bin/$(BINARY_NAME)
	@sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "✅ 설치 완료: /usr/local/bin/$(BINARY_NAME)"

# 시스템에서 제거
uninstall:
	@echo "🗑️  시스템에서 제거 중..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "✅ 제거 완료"

# 시스템 최적화 (루트 권한 필요)
optimize-system:
	@echo "🔧 시스템 네트워크 최적화..."
	@if [ "$$(id -u)" -eq 0 ]; then \
		sysctl -w net.core.wmem_max=268435456; \
		sysctl -w net.core.rmem_max=268435456; \
		sysctl -w net.core.wmem_default=1048576; \
		sysctl -w net.core.rmem_default=1048576; \
		sysctl -w net.core.netdev_max_backlog=30000; \
		sysctl -w net.ipv4.udp_mem="102400 873800 16777216"; \
		sysctl -w net.ipv4.ip_local_port_range="1024 65535"; \
		echo "✅ 시스템 최적화 완료"; \
	else \
		echo "❌ 루트 권한이 필요합니다. 'sudo make optimize-system' 실행"; \
		exit 1; \
	fi

# 시스템 설정 복구
restore-system:
	@echo "🔧 시스템 설정 복구 중..."
	@if [ "$$(id -u)" -eq 0 ]; then \
		sysctl -w net.core.wmem_max=212992; \
		sysctl -w net.core.rmem_max=212992; \
		sysctl -w net.core.netdev_max_backlog=1000; \
		echo "✅ 시스템 설정 복구 완료"; \
	else \
		echo "❌ 루트 권한이 필요합니다. 'sudo make restore-system' 실행"; \
		exit 1; \
	fi

# Docker 빌드
docker-build:
	@echo "🐳 Docker 이미지 빌드 중..."
	@docker build -t log-generator:latest .
	@echo "✅ Docker 이미지 빌드 완료"

# Docker 실행
docker-run:
	@echo "🐳 Docker 컨테이너 실행..."
	@docker run -p 8080:8080 --name log-generator-container log-generator:latest

# 정리
clean:
	@echo "🧹 빌드 파일 정리 중..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@rm -rf test_logs/
	@go clean -cache
	@echo "✅ 정리 완료"

# 전체 정리
clean-all: clean
	@echo "🧹 전체 정리 중..."
	@go clean -modcache
	@docker rmi -f log-generator:latest 2>/dev/null || true
	@echo "✅ 전체 정리 완료"

# 시스템 정보
info:
	@echo "📋 시스템 정보:"
	@echo "   OS: $$(uname -s)"
	@echo "   아키텍처: $$(uname -m)"
	@echo "   CPU 코어: $$(nproc)"
	@echo "   메모리: $$(free -h | awk 'NR==2{print $$2}')"
	@echo "   Go 버전: $$(go version)"
	@echo "   빌드 대상: $(GOOS)/$(GOARCH)"

# 도움말
help:
	@echo "400만 EPS 로그 전송기 Makefile"
	@echo ""
	@echo "🚀 기본 명령어:"
	@echo "  make build        - CLI 애플리케이션 빌드"
	@echo "  make build-web    - 웹 애플리케이션 빌드"
	@echo "  make run          - CLI 애플리케이션 실행"
	@echo "  make run-web      - 웹 애플리케이션 실행 (http://localhost:8080)"
	@echo "  make run-web-demo - 웹 데모 실행 (http://localhost:9090)"
	@echo "  make run-test     - 5분 테스트 실행"
	@echo ""
	@echo "🧪 테스트:"
	@echo "  make test         - 단위 테스트"
	@echo "  make bench        - 벤치마크 테스트"
	@echo "  make test-performance - 고성능 테스트 (sudo 권한 필요)"
	@echo "  make test-coverage - 코드 커버리지"
	@echo ""
	@echo "🔧 시스템 관리:"
	@echo "  make install      - 시스템 설치"
	@echo "  make optimize-system - 시스템 최적화 (sudo 권한 필요)"
	@echo "  make restore-system - 시스템 설정 복구 (sudo 권한 필요)"
	@echo ""
	@echo "📦 의존성:"
	@echo "  make deps         - 의존성 설치"
	@echo "  make deps-update  - 의존성 업데이트"
	@echo ""
	@echo "🧹 정리:"
	@echo "  make clean        - 빌드 파일 정리"
	@echo "  make clean-all    - 전체 정리"
	@echo ""
	@echo "📋 정보:"
	@echo "  make info         - 시스템 정보"
	@echo "  make help         - 이 도움말"

# 빌드 대상별 설정
.PHONY: build-linux build-windows build-mac build-all

build-linux:
	@GOOS=linux GOARCH=amd64 make build
	@mv $(BINARY_PATH) bin/$(BINARY_NAME)-linux-amd64

build-windows:
	@GOOS=windows GOARCH=amd64 make build
	@mv $(BINARY_PATH) bin/$(BINARY_NAME)-windows-amd64.exe

build-mac:
	@GOOS=darwin GOARCH=amd64 make build
	@mv $(BINARY_PATH) bin/$(BINARY_NAME)-darwin-amd64

build-all: clean
	@echo "🔨 모든 플랫폼 빌드 중..."
	@make build-linux
	@make build-windows  
	@make build-mac
	@echo "✅ 모든 플랫폼 빌드 완료"
	@ls -la bin/