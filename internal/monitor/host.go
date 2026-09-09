package monitor

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// HostMetrics holds aggregated host-level resources.
type HostMetrics struct {
	Load1  float64
	Load5  float64
	Load15 float64

	MemTotal uint64 // In bytes
	MemUsed  uint64 // In bytes

	DiskTotal uint64 // In bytes
	DiskUsed  uint64 // In bytes
}

// GetHostMetrics reads low-overhead kernel and syscall data to populate metrics.
func GetHostMetrics() (HostMetrics, error) {
	if runtime.GOOS != "linux" {
		// Graceful degradation for macOS/Windows during development
		return HostMetrics{}, nil
	}

	metrics := HostMetrics{}

	// Load Average
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) >= 3 {
			metrics.Load1, _ = strconv.ParseFloat(fields[0], 64)
			metrics.Load5, _ = strconv.ParseFloat(fields[1], 64)
			metrics.Load15, _ = strconv.ParseFloat(fields[2], 64)
		}
	}

	// Memory (RAM)
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		ParseMemInfo(string(b), &metrics)
	}

	// Disk
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		// Bsize is int64 on macOS, uint32 on Linux. We cast to uint64 for safety.
		bsize := uint64(stat.Bsize)
		metrics.DiskTotal = stat.Blocks * bsize
		metrics.DiskUsed = (stat.Blocks - stat.Bfree) * bsize
	}

	return metrics, nil
}

// ParseMemInfo parses the raw string of /proc/meminfo. Exported for tests.
func ParseMemInfo(content string, metrics *HostMetrics) {
	lines := strings.Split(content, "\n")
	var memTotal, memAvailable uint64
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			memTotal = extractKB(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvailable = extractKB(line)
		}
	}
	if memAvailable == 0 { // Fallback heurístico
		var memFree, buffers, cached uint64
		for _, line := range lines {
			if strings.HasPrefix(line, "MemFree:") {
				memFree = extractKB(line)
			} else if strings.HasPrefix(line, "Buffers:") {
				buffers = extractKB(line)
			} else if strings.HasPrefix(line, "Cached:") {
				cached = extractKB(line)
			}
		}
		memAvailable = memFree + buffers + cached
	}
	
	metrics.MemTotal = memTotal * 1024
	if memTotal > memAvailable {
		metrics.MemUsed = (memTotal - memAvailable) * 1024
	} else {
		metrics.MemUsed = 0
	}
}

func extractKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		return val
	}
	return 0
}

// FormatBytes converts raw bytes to readable MB/GB string.
func FormatBytes(bytes uint64) string {
	if bytes == 0 {
		return "N/A"
	}
	mb := float64(bytes) / 1024 / 1024
	if mb >= 1024 {
		gb := mb / 1024
		return fmt.Sprintf("%.1f GB", gb)
	}
	return fmt.Sprintf("%.0f MB", mb)
}
