# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a high-performance log generator designed to achieve 4 million EPS (Events Per Second) for SIEM system performance testing. The system generates RFC 3164 compliant system logs and transmits them via UDP to test SIEM ingestion capabilities.

**Key Performance Targets:**
- 4M EPS sustained for 30+ minutes
- CPU usage ≤75%, Memory ≤12GB
- Packet loss <0.5%

## Architecture

The system uses a **40-worker architecture** where each worker targets 100K EPS (40 × 100K = 4M EPS total).

**Core Components:**
- **SystemLogGenerator** (`internal/generator/`): Zero-allocation log generation with memory pooling
- **WorkerPool** (`internal/worker/`): Manages 40 UDP workers across ports 514-553
- **MetricsCollector** (`pkg/metrics/`): Real-time performance monitoring
- **DashboardServer** (`internal/monitor/`): Web-based monitoring at :8080

**Critical Performance Techniques:**
- Global `sync.Pool` for memory reuse
- Timestamp caching (1-second intervals)
- UDP batch transmission via `sendmmsg()`
- GC tuning (GOGC=200, 12GB memory limit)
- Pre-allocated component pools (hostnames, services, PIDs)

## Common Commands

### Build & Run
```bash
make build          # Build optimized binary
make run            # Build and run with defaults
make run-test       # 5-minute test run
```

### Testing
```bash
make test                    # Unit tests
make test-performance       # Full 30-minute performance test (requires sudo)
make bench                  # Benchmark tests
make test-coverage         # Coverage analysis
```

### System Optimization
```bash
sudo make optimize-system   # Apply kernel network tuning
sudo make restore-system    # Restore default settings
```

### Development
```bash
make build-dev      # Build with race detection
make lint           # Static analysis (requires golangci-lint)
make clean          # Clean build artifacts
```

## Development Notes

### Performance-Critical Areas

**Memory Management:**
- All log generation uses `sync.Pool` to avoid allocations
- The `GenerateSystemLog()` method is the hot path - modifications must maintain zero-allocation design
- String builders are pooled and reused

**Worker Coordination:**
- Each worker binds to a specific UDP port (514-553 range)
- Workers use atomic counters for metrics to avoid lock contention
- The 10ms ticker interval in workers is calibrated for 100K EPS target

**Metrics Collection:**
- Metrics flow: Worker → WorkerPool → MetricsCollector → Dashboard
- Real-time updates via WebSocket to dashboard
- EPS calculation uses sliding window averaging

### Key Configuration

**Network Settings** (applied by `make optimize-system`):
```bash
net.core.wmem_max=268435456      # 256MB send buffer
net.core.netdev_max_backlog=30000
net.ipv4.ip_local_port_range="1024 65535"
```

**Runtime Tuning:**
- `GOGC=200` - Reduces GC pressure
- Memory limit: 12GB via `debug.SetMemoryLimit()`
- CPU binding available via `SetCPUAffinity()` (Linux only)

### Testing Infrastructure

The `scripts/test.sh` provides automated testing:
- Launches dummy SIEM receiver on ports 514-553
- Monitors performance in real-time
- Generates comprehensive performance reports
- Handles system optimization and cleanup

### Command Line Options

```bash
./bin/log-generator \
  -target=<siem-host>     # Target SIEM system (default: 127.0.0.1)
  -dashboard-port=<port>  # Dashboard port (default: 8080)
  -duration=<minutes>     # Test duration, 0=unlimited (default: 0)
  -optimize=<bool>        # Enable memory optimization (default: true)
```

## Monitoring

**Web Dashboard:** `http://localhost:8080`
- Real-time EPS tracking with 40 individual worker status indicators
- System resource monitoring (CPU, memory, goroutines)
- Achievement percentage vs 4M EPS target

**API Endpoints:**
- `GET /api/metrics` - Current performance metrics
- `GET /api/summary` - Aggregated summary statistics
- `WebSocket /ws` - Real-time metric streaming

## Performance Debugging

**Common Issues:**
- Port binding failures: Check for existing syslog services on ports 514-553
- Low EPS: Verify network buffer tuning and CPU core availability
- High memory usage: Monitor GC metrics via dashboard

**Profiling:**
- Built-in metrics collection tracks CPU/memory usage
- Use `runtime/pprof` endpoints if enabled in development builds
- Monitor goroutine counts - each worker creates ~3 goroutines

The system is designed to be self-monitoring with extensive real-time metrics, making performance issues immediately visible through the web dashboard.