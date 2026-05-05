package management

import (
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var serverStartedAt = time.Now()

type serverStatusResource struct {
	UsagePercent *float64 `json:"usage_percent,omitempty"`
	TotalBytes   *uint64  `json:"total_bytes,omitempty"`
	UsedBytes    *uint64  `json:"used_bytes,omitempty"`
	FreeBytes    *uint64  `json:"free_bytes,omitempty"`
}

type serverStatusCPU struct {
	UsagePercent *float64  `json:"usage_percent,omitempty"`
	Cores        int       `json:"cores"`
	LoadAverage  []float64 `json:"load_average"`
}

type serverStatusProcess struct {
	Goroutines int `json:"goroutines"`
}

type serverStatusResponse struct {
	CPU           serverStatusCPU       `json:"cpu"`
	Memory        serverStatusResource  `json:"memory"`
	Disk          *serverStatusResource `json:"disk,omitempty"`
	Process       serverStatusProcess   `json:"process"`
	UptimeSeconds int64                 `json:"uptime_seconds"`
	UpdatedAt     string                `json:"updated_at"`
	Hostname      string                `json:"hostname,omitempty"`
	OS            string                `json:"os"`
	Arch          string                `json:"arch"`
}

type cpuTimes struct {
	idle  uint64
	total uint64
}

var cpuSampleState struct {
	sync.Mutex
	previous cpuTimes
	hasPrev  bool
}

// GetServerStatus returns lightweight host and runtime statistics for the management UI.
func (h *Handler) GetServerStatus(c *gin.Context) {
	hostname, _ := os.Hostname()
	diskPath := "."
	if h != nil && strings.TrimSpace(h.configFilePath) != "" {
		diskPath = filepath.Dir(h.configFilePath)
	}
	disk, hasDisk := readDiskStatus(diskPath)
	if !hasDisk {
		disk = nil
	}

	c.JSON(http.StatusOK, serverStatusResponse{
		CPU: serverStatusCPU{
			UsagePercent: currentCPUUsagePercent(),
			Cores:        runtime.NumCPU(),
			LoadAverage:  readLoadAverage(),
		},
		Memory:        readSystemMemoryStatus(),
		Disk:          disk,
		Process:       serverStatusProcess{Goroutines: runtime.NumGoroutine()},
		UptimeSeconds: int64(time.Since(serverStartedAt).Seconds()),
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		Hostname:      hostname,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
	})
}

func currentCPUUsagePercent() *float64 {
	now, ok := readCPUTimes()
	if !ok {
		return nil
	}

	cpuSampleState.Lock()
	if cpuSampleState.hasPrev {
		previous := cpuSampleState.previous
		cpuSampleState.previous = now
		cpuSampleState.Unlock()
		return calculateCPUUsagePercent(previous, now)
	}
	cpuSampleState.previous = now
	cpuSampleState.hasPrev = true
	cpuSampleState.Unlock()

	time.Sleep(100 * time.Millisecond)
	later, ok := readCPUTimes()
	if !ok {
		return nil
	}

	cpuSampleState.Lock()
	cpuSampleState.previous = later
	cpuSampleState.Unlock()
	return calculateCPUUsagePercent(now, later)
}

func calculateCPUUsagePercent(previous, current cpuTimes) *float64 {
	if current.total <= previous.total || current.idle < previous.idle {
		return nil
	}
	totalDelta := current.total - previous.total
	idleDelta := current.idle - previous.idle
	if totalDelta == 0 || idleDelta > totalDelta {
		return nil
	}
	usage := (float64(totalDelta-idleDelta) / float64(totalDelta)) * 100
	return float64Pointer(clampPercent(usage))
}

func resourceFromTotalFree(total, free uint64) serverStatusResource {
	used := uint64(0)
	if total > free {
		used = total - free
	}
	var percent *float64
	if total > 0 {
		percent = float64Pointer(clampPercent((float64(used) / float64(total)) * 100))
	}
	return serverStatusResource{
		UsagePercent: percent,
		TotalBytes:   uint64Pointer(total),
		UsedBytes:    uint64Pointer(used),
		FreeBytes:    uint64Pointer(free),
	}
}

func clampPercent(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return math.Round(value*100) / 100
}

func float64Pointer(value float64) *float64 {
	return &value
}

func uint64Pointer(value uint64) *uint64 {
	return &value
}
