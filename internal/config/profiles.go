package config

import (
	"fmt"
	"runtime"
)

// EPSProfile defines configuration for different target EPS levels
type EPSProfile struct {
	Name             string
	TargetEPS        int
	WorkerCount      int
	BatchSize        int
	TickerInterval   int    // microseconds
	SendBufferSize   int    // KB
	ReceiveBufferSize int   // KB
	GOGC             int
	MemoryLimit      int64  // bytes
	Description      string
	PrecisionMode    string // "high" (오차 <1%), "medium" (오차 <5%), "performance" (오차 <10%)
}

var EPSProfiles = map[string]*EPSProfile{
	"100k": {
		Name:             "100k",
		TargetEPS:        100_000,
		WorkerCount:      10,     // 워커당 10K EPS
		BatchSize:        100,    // 100 logs * 200회/초 = 20K EPS (실제로는 10K 달성)
		TickerInterval:   5000,   // 5ms (200회/초) - 실제 타이머 이슈 보정
		SendBufferSize:   8192,   // 8MB
		ReceiveBufferSize: 4096,
		GOGC:             100,
		MemoryLimit:      2 * 1024 * 1024 * 1024, // 2GB
		Description:      "Light load - 100K EPS",
		PrecisionMode:    "high", // 높은 정밀도 (오차 <1%)
	},
	"500k": {
		Name:             "500k",
		TargetEPS:        500_000,
		WorkerCount:      50,     // 워커당 10K EPS
		BatchSize:        100,    // 100 logs * 200회/초 = 20K EPS (실제로는 10K 달성)
		TickerInterval:   5000,   // 5ms (200회/초) - 실제 타이머 이슈 보정
		SendBufferSize:   16384,  // 16MB
		ReceiveBufferSize: 8192,
		GOGC:             150,
		MemoryLimit:      4 * 1024 * 1024 * 1024, // 4GB
		Description:      "Medium load - 500K EPS",
		PrecisionMode:    "high", // 높은 정밀도
	},
	"1m": {
		Name:             "1m",
		TargetEPS:        1_000_000,
		WorkerCount:      40,     // 워커당 25K EPS
		BatchSize:        250,    // 250 logs * 100회/초 = 25K EPS
		TickerInterval:   10000,  // 10ms (100회/초) - 표준 타이머
		SendBufferSize:   32768,  // 32MB
		ReceiveBufferSize: 16384,
		GOGC:             200,
		MemoryLimit:      6 * 1024 * 1024 * 1024, // 6GB
		Description:      "Standard load - 1M EPS",
		PrecisionMode:    "performance", // 성능 우선 모드
	},
	"2m": {
		Name:             "2m",
		TargetEPS:        2_000_000,
		WorkerCount:      80,     // 워커당 25K EPS
		BatchSize:        250,    // 250 logs * 100회/초 = 25K EPS
		TickerInterval:   10000,  // 10ms (100회/초) - 표준 타이머
		SendBufferSize:   65536,  // 64MB
		ReceiveBufferSize: 32768,
		GOGC:             200,
		MemoryLimit:      8 * 1024 * 1024 * 1024, // 8GB
		Description:      "High load - 2M EPS",
		PrecisionMode:    "performance", // 성능 우선 모드
	},
	"4m": {
		Name:             "4m",
		TargetEPS:        4_000_000,
		WorkerCount:      160,    // 워커당 25K EPS
		BatchSize:        250,    // 250 logs * 100회/초 = 25K EPS
		TickerInterval:   10000,  // 10ms (100회/초) - 표준 타이머
		SendBufferSize:   131072, // 128MB
		ReceiveBufferSize: 65536,
		GOGC:             200,
		MemoryLimit:      12 * 1024 * 1024 * 1024, // 12GB
		Description:      "Maximum load - 4M EPS",
		PrecisionMode:    "performance", // 성능 우선 모드
	},
	"custom": {
		Name:             "custom",
		TargetEPS:        0, // Will be set by user
		WorkerCount:      0, // Will be calculated
		BatchSize:        100,
		TickerInterval:   50,
		SendBufferSize:   65536,
		ReceiveBufferSize: 32768,
		GOGC:             200,
		MemoryLimit:      8 * 1024 * 1024 * 1024,
		Description:      "Custom configuration",
		PrecisionMode:    "medium", // 기본값: 중간
	},
}

// GetProfile returns the EPS profile for the given name
func GetProfile(name string) (*EPSProfile, error) {
	profile, exists := EPSProfiles[name]
	if !exists {
		return nil, fmt.Errorf("unknown profile: %s", name)
	}
	
	// Create a copy to avoid modifying the original
	p := *profile
	return &p, nil
}

// GetProfileForEPS returns the best profile for the target EPS
func GetProfileForEPS(targetEPS int) *EPSProfile {
	// Find the smallest profile that can handle the target EPS
	profiles := []string{"100k", "500k", "1m", "2m", "4m"}
	
	for _, name := range profiles {
		p := EPSProfiles[name]
		if p.TargetEPS >= targetEPS {
			profile := *p
			// Adjust worker count if needed
			if targetEPS < p.TargetEPS {
				ratio := float64(targetEPS) / float64(p.TargetEPS)
				profile.WorkerCount = maxInt(1, int(float64(p.WorkerCount)*ratio))
				profile.TargetEPS = targetEPS
			}
			return &profile
		}
	}
	
	// If target is higher than 4M, use custom profile
	custom := *EPSProfiles["custom"]
	custom.TargetEPS = targetEPS
	custom.WorkerCount = calculateWorkerCount(targetEPS)
	return &custom
}

// CalculateCustomProfile creates a custom profile for the given EPS
func CalculateCustomProfile(targetEPS int) *EPSProfile {
	profile := *EPSProfiles["custom"]
	profile.TargetEPS = targetEPS
	profile.WorkerCount = calculateWorkerCount(targetEPS)
	
	// Adjust batch size and timing based on EPS
	if targetEPS <= 100_000 {
		profile.BatchSize = 10
		profile.TickerInterval = 100
	} else if targetEPS <= 500_000 {
		profile.BatchSize = 20
		profile.TickerInterval = 40
	} else if targetEPS <= 1_000_000 {
		profile.BatchSize = 50
		profile.TickerInterval = 50
	} else if targetEPS <= 2_000_000 {
		profile.BatchSize = 100
		profile.TickerInterval = 50
	} else {
		profile.BatchSize = 200
		profile.TickerInterval = 50
	}
	
	// Adjust memory based on EPS
	profile.MemoryLimit = int64(minInt(12, maxInt(2, targetEPS/500_000+1))) * 1024 * 1024 * 1024
	
	// Adjust buffer sizes
	profile.SendBufferSize = minInt(262144, maxInt(8192, targetEPS/10))
	profile.ReceiveBufferSize = profile.SendBufferSize / 2
	
	profile.Description = fmt.Sprintf("Custom profile for %d EPS", targetEPS)
	
	return &profile
}

func calculateWorkerCount(targetEPS int) int {
	// EPS 범위에 따른 최적 워커당 처리량
	var optimalEPSPerWorker int
	
	if targetEPS <= 100_000 {
		// 낮은 부하: 워커당 10K EPS
		optimalEPSPerWorker = 10_000
	} else if targetEPS <= 500_000 {
		// 중간 부하: 워커당 20K EPS
		optimalEPSPerWorker = 20_000
	} else {
		// 높은 부하: 워커당 25K EPS (최적)
		optimalEPSPerWorker = 25_000
	}
	
	workers := targetEPS / optimalEPSPerWorker
	if workers == 0 {
		workers = 1
	}
	
	// 최소 워커 수 보장
	if targetEPS >= 4_000_000 && workers < 160 {
		workers = 160 // 4M EPS는 최소 160개 워커 필요
	} else if targetEPS >= 2_000_000 && workers < 80 {
		workers = 80  // 2M EPS는 최소 80개 워커 필요
	} else if targetEPS >= 1_000_000 && workers < 40 {
		workers = 40  // 1M EPS는 최소 40개 워커 필요
	}
	
	// CPU 코어 수에 따른 최대값 (코어당 10 워커)
	maxWorkers := runtime.NumCPU() * 10
	if workers > maxWorkers {
		workers = maxWorkers
	}
	
	// 절대 최대값 200 (포트 범위와 시스템 한계)
	if workers > 200 {
		workers = 200
	}
	
	return workers
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ListProfiles returns a list of available profile names
func ListProfiles() []string {
	return []string{"100k", "500k", "1m", "2m", "4m", "custom"}
}

// GetProfileDescription returns a formatted description of all profiles
func GetProfileDescription() string {
	desc := "Available EPS Profiles:\n"
	for _, name := range ListProfiles() {
		p := EPSProfiles[name]
		if name != "custom" {
			desc += fmt.Sprintf("  %s: %s (Workers: %d, Batch: %d)\n", 
				name, p.Description, p.WorkerCount, p.BatchSize)
		}
	}
	desc += "  custom: Specify custom EPS target with -eps flag\n"
	return desc
}