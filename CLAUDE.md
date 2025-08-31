# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a profile-based adaptive log generator designed to achieve 100K to 4M EPS (Events Per Second) for SIEM system performance testing. The system automatically optimizes settings based on the selected EPS profile and generates RFC 3164 compliant system logs transmitted via UDP.

**Available EPS Profiles:**
- **100k**: Light load (2 workers, 2GB memory) - Development/Testing
- **500k**: Medium load (5 workers, 4GB memory) - Small systems
- **1m**: Standard load (10 workers, 6GB memory) - Medium systems
- **2m**: High load (20 workers, 8GB memory) - Large systems
- **4m**: Maximum load (40 workers, 12GB memory) - Enterprise
- **custom**: User-defined target EPS with automatic optimization

**Key Performance Targets (per profile):**
- Target EPS sustained for 30+ minutes
- CPU usage ≤75%, Memory within profile limits
- Packet loss <0.5%

## Architecture

The system uses an **adaptive worker architecture** that automatically scales based on the selected EPS profile:
- **100k profile**: 2 workers × 50K EPS each
- **500k profile**: 5 workers × 100K EPS each
- **1m profile**: 10 workers × 100K EPS each
- **2m profile**: 20 workers × 100K EPS each
- **4m profile**: 40 workers × 100K EPS each
- **custom**: Automatic worker calculation based on target

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

### Build & Run with Profiles
```bash
make build                      # Build optimized binary

# Run with different profiles
./bin/log-generator -profile 100k     # 100K EPS profile
./bin/log-generator -profile 500k     # 500K EPS profile
./bin/log-generator -profile 1m       # 1M EPS profile
./bin/log-generator -profile 2m       # 2M EPS profile
./bin/log-generator -profile 4m       # 4M EPS profile (default)
./bin/log-generator -profile custom -eps 750000  # Custom 750K EPS

# Web interface with profile selection
./bin/log-generator-web -port 8080    # Access at http://localhost:8080/control
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

**Profile-Based Optimization:**
- Each profile has pre-tuned settings for optimal performance
- Worker count, batch size, and timing are automatically adjusted
- Memory limits and GC settings are profile-specific

**Memory Management:**
- All log generation uses `sync.Pool` to avoid allocations
- The `GenerateSystemLog()` method is the hot path - modifications must maintain zero-allocation design
- String builders are pooled and reused

**Worker Coordination:**
- Workers dynamically scale based on profile (2-40 workers)
- Each worker binds to a specific UDP port (514-553 range)
- Workers use atomic counters for metrics to avoid lock contention
- Ticker intervals are profile-optimized (40-100μs based on target EPS)

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
  -profile=<profile>      # EPS profile: 100k/500k/1m/2m/4m/custom (default: 4m)
  -eps=<number>           # Custom target EPS (required when profile=custom)
  -target=<siem-host>     # Target SIEM system (default: 127.0.0.1)
  -dashboard-port=<port>  # Dashboard port (default: 8080)
  -duration=<minutes>     # Test duration, 0=unlimited (default: 0)
  -optimize=<bool>        # Enable memory optimization (default: true)
```

## Monitoring

**Web Dashboard:** `http://localhost:8080`
- Real-time EPS tracking with profile-based worker status indicators
- Current profile display and target EPS
- System resource monitoring (CPU, memory, goroutines)
- Achievement percentage vs profile target

**Web Control Panel:** `http://localhost:8080/control`
- Profile selection dropdown (100k/500k/1m/2m/4m/custom)
- Real-time start/stop/restart controls
- Automatic settings optimization based on profile
- Live system logs and metrics

**API Endpoints:**
- `GET /api/metrics` - Current performance metrics
- `GET /api/summary` - Aggregated summary statistics
- `WebSocket /ws` - Real-time metric streaming

## Performance Debugging

**Common Issues:**
- Port binding failures: Check for existing syslog services on ports 514-553
- Low EPS: Try a lower profile or verify network buffer tuning
- High memory usage: Use a lower profile or monitor GC metrics
- Profile not working: Ensure correct syntax (-profile 1m, not -profile 1M)

**Profiling:**
- Built-in metrics collection tracks CPU/memory usage
- Use `runtime/pprof` endpoints if enabled in development builds
- Monitor goroutine counts - each worker creates ~3 goroutines

The system is designed to be self-monitoring with extensive real-time metrics, making performance issues immediately visible through the web dashboard. The profile-based architecture ensures optimal performance for any target EPS from 100K to 4M, automatically adjusting all settings for best results.