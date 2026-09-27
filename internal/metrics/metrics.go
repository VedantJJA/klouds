// Package metrics collects system and container metrics.
// On Linux, reads directly from /proc for zero-dependency monitoring.
// On non-Linux (e.g., Windows dev), returns mock data.
package metrics

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SystemMetrics represents a snapshot of host system metrics.
type SystemMetrics struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryTotal   int64   `json:"memory_total"`    // bytes
	MemoryUsed    int64   `json:"memory_used"`     // bytes
	MemoryPercent float64 `json:"memory_percent"`
	DiskTotal     int64   `json:"disk_total"`      // bytes
	DiskUsed      int64   `json:"disk_used"`       // bytes
	DiskPercent   float64 `json:"disk_percent"`
	Uptime        int64   `json:"uptime"`          // seconds
	LoadAvg1      float64 `json:"load_avg_1"`
	LoadAvg5      float64 `json:"load_avg_5"`
	LoadAvg15     float64 `json:"load_avg_15"`
	NumCPUs       int     `json:"num_cpus"`
	GoRoutines    int     `json:"go_routines"`
	Timestamp     int64   `json:"timestamp"`
}

// Collector gathers system metrics from /proc.
type Collector struct {
	prevCPUIdle  uint64
	prevCPUTotal uint64
}

// NewCollector creates a new metrics collector.
func NewCollector() *Collector {
	return &Collector{}
}

// Collect gathers current system metrics.
func (c *Collector) Collect() SystemMetrics {
	m := SystemMetrics{
		NumCPUs:    runtime.NumCPU(),
		GoRoutines: runtime.NumGoroutine(),
		Timestamp:  time.Now().Unix(),
	}

	if runtime.GOOS == "linux" {
		c.collectLinux(&m)
	} else {
		// On non-Linux, provide Go runtime stats at minimum
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		m.MemoryUsed = int64(mem.Sys)
		m.MemoryTotal = int64(mem.Sys) * 2 // rough estimate
		if m.MemoryTotal > 0 {
			m.MemoryPercent = float64(m.MemoryUsed) / float64(m.MemoryTotal) * 100
		}
	}

	return m
}

// collectLinux reads from /proc and /sys on Linux.
func (c *Collector) collectLinux(m *SystemMetrics) {
	// --- Memory from /proc/meminfo ---
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		memInfo := make(map[string]int64)
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				key := strings.TrimSuffix(parts[0], ":")
				val, _ := strconv.ParseInt(parts[1], 10, 64)
				memInfo[key] = val * 1024 // kB → bytes
			}
		}
		m.MemoryTotal = memInfo["MemTotal"]
		available := memInfo["MemAvailable"]
		m.MemoryUsed = m.MemoryTotal - available
		if m.MemoryTotal > 0 {
			m.MemoryPercent = float64(m.MemoryUsed) / float64(m.MemoryTotal) * 100
		}
	}

	// --- CPU from /proc/stat ---
	if data, err := os.ReadFile("/proc/stat"); err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "cpu ") {
			fields := strings.Fields(lines[0])
			if len(fields) >= 8 {
				var total, idle uint64
				for i := 1; i < len(fields); i++ {
					v, _ := strconv.ParseUint(fields[i], 10, 64)
					total += v
					if i == 4 { // idle is the 4th value
						idle = v
					}
				}

				if c.prevCPUTotal > 0 {
					totalDelta := float64(total - c.prevCPUTotal)
					idleDelta := float64(idle - c.prevCPUIdle)
					if totalDelta > 0 {
						m.CPUPercent = (1.0 - idleDelta/totalDelta) * 100
					}
				}

				c.prevCPUTotal = total
				c.prevCPUIdle = idle
			}
		}
	}

	// --- Uptime from /proc/uptime ---
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 1 {
			uptime, _ := strconv.ParseFloat(fields[0], 64)
			m.Uptime = int64(uptime)
		}
	}

	// --- Load averages from /proc/loadavg ---
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			m.LoadAvg1, _ = strconv.ParseFloat(fields[0], 64)
			m.LoadAvg5, _ = strconv.ParseFloat(fields[1], 64)
			m.LoadAvg15, _ = strconv.ParseFloat(fields[2], 64)
		}
	}
}
