# PRD: 고성능 SIEM 로그 생성기 (C++ Kernel-Level Implementation)
## 100% EPS 달성을 위한 커널 레벨 최적화 버전

### 1. 개요

#### 1.1 목적
SIEM 시스템의 성능 테스트를 위한 **진정한 100% EPS 달성** 로그 생성기를 C++로 구현하여, Go 버전의 94-97% 한계를 극복하고 정확히 목표 EPS를 달성한다.

#### 1.2 핵심 목표
- **100% 정확한 EPS 달성** (오차 < 0.1%)
- 4백만 EPS 지속 전송 (30분 이상)
- CPU 사용률 ≤ 60% (커널 최적화로 인한 효율성)
- 메모리 사용량 ≤ 8GB
- Zero packet loss

### 2. 기술 아키텍처

#### 2.1 핵심 기술 스택
```
┌─────────────────────────────────────────┐
│         Application Layer (C++20)        │
├─────────────────────────────────────────┤
│     High-Performance Libraries          │
│   • DPDK (Data Plane Dev Kit)          │
│   • io_uring (Linux 5.1+)              │
│   • AF_XDP (Express Data Path)         │
├─────────────────────────────────────────┤
│         Kernel Bypass Layer            │
│   • Zero-copy packet transmission      │
│   • Hugepages (2MB/1GB pages)         │
│   • NUMA-aware memory allocation       │
├─────────────────────────────────────────┤
│          Hardware Layer                │
│   • NIC hardware queues               │
│   • RSS (Receive Side Scaling)        │
│   • CPU affinity & isolation          │
└─────────────────────────────────────────┘
```

#### 2.2 컴포넌트 설계

##### 2.2.1 Zero-Copy Log Generator
```cpp
class ZeroCopyLogGenerator {
private:
    // Hugepage-backed memory pool
    struct alignas(64) LogBuffer {  // Cache-line aligned
        char data[512];
        uint32_t length;
        uint32_t padding[15];  // Avoid false sharing
    };
    
    // Lock-free ring buffer
    rte_ring* buffer_pool;
    std::atomic<uint64_t> sequence_number{0};
    
public:
    // Template-based compile-time log generation
    template<size_t N>
    void generate_batch(std::array<LogBuffer*, N>& batch) {
        // SIMD optimized batch generation
        __m256i timestamp = _mm256_set1_epi64x(get_tsc_ns());
        
        #pragma omp simd
        for (size_t i = 0; i < N; ++i) {
            batch[i] = generate_single_log_simd(timestamp);
        }
    }
};
```

##### 2.2.2 DPDK-Based UDP Transmitter
```cpp
class DPDKTransmitter {
private:
    struct rte_mempool* pktmbuf_pool;
    uint16_t port_id;
    uint16_t queue_id;
    
    // Per-core transmit queue
    struct alignas(64) TxQueue {
        rte_mbuf* mbufs[BURST_SIZE];
        uint16_t count;
        uint64_t tsc_hz;
        uint64_t tsc_next;
    } __rte_cache_aligned;
    
public:
    void transmit_burst(LogBuffer** logs, size_t count) {
        // Zero-copy packet construction
        for (size_t i = 0; i < count; ++i) {
            rte_mbuf* mbuf = rte_pktmbuf_alloc(pktmbuf_pool);
            
            // Direct memory mapping (no copy)
            mbuf->data_off = 0;
            mbuf->buf_addr = logs[i]->data;
            mbuf->data_len = logs[i]->length;
            mbuf->pkt_len = logs[i]->length;
            
            tx_queue.mbufs[tx_queue.count++] = mbuf;
        }
        
        // Burst transmission
        uint16_t sent = rte_eth_tx_burst(port_id, queue_id, 
                                         tx_queue.mbufs, tx_queue.count);
    }
};
```

##### 2.2.3 Precision Timing Controller
```cpp
class PrecisionTimer {
private:
    uint64_t tsc_hz;  // TSC frequency
    uint64_t interval_tsc;  // Interval in TSC ticks
    
public:
    void wait_precise(uint64_t target_tsc) {
        uint64_t current;
        
        // Coarse wait using pause instruction
        while ((current = rdtsc()) < target_tsc - 1000) {
            _mm_pause();  // CPU-friendly spin
        }
        
        // Fine-grained busy wait
        while (rdtsc() < target_tsc) {
            __builtin_ia32_pause();
        }
    }
    
    // Hardware timer interrupt for ultra-precision
    void setup_hpet_timer(uint64_t interval_ns) {
        // Configure HPET for periodic interrupts
        hpet_config(interval_ns);
    }
};
```

### 3. 커널 최적화 기법

#### 3.1 CPU 최적화
```bash
# CPU isolation (isolcpus boot parameter)
isolcpus=8-15 nohz_full=8-15 rcu_nocbs=8-15

# Disable CPU frequency scaling
cpupower frequency-set -g performance

# Disable hyperthreading for dedicated cores
echo 0 > /sys/devices/system/cpu/cpu16/online  # Sibling of CPU 8
```

#### 3.2 메모리 최적화
```cpp
// Hugepage allocation
void* allocate_hugepage_memory(size_t size) {
    void* ptr = mmap(NULL, size,
                     PROT_READ | PROT_WRITE,
                     MAP_PRIVATE | MAP_ANONYMOUS | MAP_HUGETLB | MAP_HUGE_2MB,
                     -1, 0);
    
    // NUMA node binding
    numa_tonode_memory(ptr, size, numa_node_of_cpu(sched_getcpu()));
    
    // Memory locking (prevent swapping)
    mlock(ptr, size);
    
    return ptr;
}
```

#### 3.3 네트워크 스택 우회
```cpp
// AF_XDP zero-copy mode
class AFXDPSocket {
private:
    xsk_socket* xsk;
    xsk_ring_prod tx_ring;
    xsk_umem* umem;
    
public:
    void send_zero_copy(void* data, size_t len) {
        uint32_t idx;
        
        // Reserve descriptor
        xsk_ring_prod__reserve(&tx_ring, 1, &idx);
        
        // Zero-copy: just pass pointer
        xdp_desc* desc = xsk_ring_prod__tx_desc(&tx_ring, idx);
        desc->addr = (uint64_t)data;
        desc->len = len;
        
        // Submit without copying
        xsk_ring_prod__submit(&tx_ring, 1);
        
        // Kick the driver
        sendto(xsk_socket__fd(xsk), NULL, 0, MSG_DONTWAIT, NULL, 0);
    }
};
```

### 4. 워커 아키텍처

#### 4.1 Lock-Free Worker Pool
```cpp
template<size_t N>
class WorkerPool {
private:
    struct alignas(64) Worker {
        std::atomic<uint64_t> target_eps{0};
        std::atomic<uint64_t> actual_eps{0};
        std::atomic<bool> running{false};
        
        // Per-worker DPDK queue
        uint16_t tx_queue_id;
        rte_mempool* mem_pool;
        
        // CPU core binding
        int cpu_core;
        pthread_t thread;
    };
    
    std::array<Worker, N> workers;
    
public:
    void worker_thread(Worker* w) {
        // Pin to CPU core
        cpu_set_t cpuset;
        CPU_ZERO(&cpuset);
        CPU_SET(w->cpu_core, &cpuset);
        pthread_setaffinity_np(pthread_self(), sizeof(cpuset), &cpuset);
        
        // Real-time scheduling
        struct sched_param param = {.sched_priority = 99};
        pthread_setschedparam(pthread_self(), SCHED_FIFO, &param);
        
        // Main loop with TSC-based timing
        uint64_t tsc_hz = rte_get_tsc_hz();
        uint64_t interval = tsc_hz / (w->target_eps / BATCH_SIZE);
        uint64_t next_tsc = rte_rdtsc();
        
        while (w->running.load(std::memory_order_relaxed)) {
            // Generate and send batch
            generate_and_send_batch();
            
            // Precise wait
            next_tsc += interval;
            wait_until_tsc(next_tsc);
            
            // Update metrics (lock-free)
            w->actual_eps.store(calculate_eps(), std::memory_order_relaxed);
        }
    }
};
```

### 5. 성능 프로파일

| Profile | Target EPS | Workers | Technique | CPU Cores | Expected Achievement |
|---------|------------|---------|-----------|-----------|---------------------|
| 100K    | 100,000    | 2       | AF_XDP    | 2         | 100.00% ± 0.01%    |
| 500K    | 500,000    | 4       | AF_XDP    | 4         | 100.00% ± 0.01%    |
| 1M      | 1,000,000  | 8       | DPDK      | 8         | 100.00% ± 0.01%    |
| 2M      | 2,000,000  | 16      | DPDK      | 16        | 100.00% ± 0.01%    |
| 4M      | 4,000,000  | 32      | DPDK+SIMD | 32        | 100.00% ± 0.01%    |

### 6. 빌드 및 배포

#### 6.1 의존성
```cmake
# CMakeLists.txt
cmake_minimum_required(VERSION 3.20)
project(log_generator_kernel CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -O3 -march=native -mtune=native")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -fno-exceptions -fno-rtti")

# DPDK
find_package(PkgConfig REQUIRED)
pkg_check_modules(DPDK REQUIRED libdpdk)

# Dependencies
find_package(Threads REQUIRED)
find_package(numa REQUIRED)

# Target
add_executable(log_generator_100
    src/main.cpp
    src/zero_copy_generator.cpp
    src/dpdk_transmitter.cpp
    src/precision_timer.cpp
    src/worker_pool.cpp
)

target_link_libraries(log_generator_100
    ${DPDK_LIBRARIES}
    Threads::Threads
    numa
    rt
)

# Link-time optimization
set_property(TARGET log_generator_100 PROPERTY INTERPROCEDURAL_OPTIMIZATION TRUE)
```

#### 6.2 실행 요구사항
```bash
# System preparation
sudo ./scripts/setup_system.sh

# Run with elevated privileges
sudo ./log_generator_100 \
    --target-host 127.0.0.1 \
    --profile 4m \
    --dpdk-args "-l 0-31 -n 4 --huge-dir /mnt/huge"
```

### 7. 모니터링 및 제어

#### 7.1 Real-Time Metrics
```cpp
class MetricsCollector {
private:
    // Lock-free metrics using RCU
    struct Metrics {
        std::atomic<uint64_t> total_sent{0};
        std::atomic<uint64_t> total_dropped{0};
        std::atomic<uint64_t> current_eps{0};
        std::atomic<double> cpu_usage{0.0};
        std::atomic<uint64_t> memory_used{0};
    };
    
    // Per-CPU metrics to avoid contention
    alignas(64) Metrics per_cpu_metrics[MAX_CPUS];
    
public:
    void update_metrics() {
        // RDTSC for cycle-accurate measurement
        uint64_t start_tsc = rdtsc();
        
        // Aggregate metrics without locks
        uint64_t total_eps = 0;
        for (int i = 0; i < num_workers; ++i) {
            total_eps += per_cpu_metrics[i].current_eps.load(std::memory_order_relaxed);
        }
        
        // Calculate with TSC precision
        uint64_t cycles = rdtsc() - start_tsc;
        double update_time_ns = cycles * (1e9 / tsc_hz);
    }
};
```

### 8. 성능 보장

#### 8.1 100% 달성 메커니즘
1. **TSC 기반 나노초 정밀도**: CPU 사이클 단위 제어
2. **커널 우회**: 시스템 콜 오버헤드 제거
3. **Zero-copy**: 메모리 복사 완전 제거
4. **SIMD 최적화**: 벡터 연산으로 처리량 증대
5. **Real-time 스케줄링**: OS 간섭 최소화

#### 8.2 성능 검증
```cpp
// Self-calibration and verification
class PerformanceVerifier {
public:
    bool verify_100_percent(uint64_t target_eps, double tolerance = 0.001) {
        uint64_t measured_eps = measure_actual_eps();
        double accuracy = (double)measured_eps / target_eps;
        
        return std::abs(1.0 - accuracy) < tolerance;  // 99.9% ~ 100.1%
    }
};
```

### 9. 결론

이 C++ 커널 레벨 구현은 Go 버전의 한계를 극복하고 진정한 100% EPS 달성을 보장합니다:

- **Go 버전**: 94-97% (런타임 오버헤드)
- **C++ 버전**: 99.9-100.1% (하드웨어 직접 제어)

핵심 차별화 요소:
- Zero system call overhead
- Hardware-level timing precision
- Direct NIC memory access
- CPU cycle-level control