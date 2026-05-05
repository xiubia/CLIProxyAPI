//go:build linux

package management

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

func readCPUTimes() (cpuTimes, bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, false
	}
	lines := strings.SplitN(string(data), "\n", 2)
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, false
	}

	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, errParse := strconv.ParseUint(field, 10, 64)
		if errParse != nil {
			return cpuTimes{}, false
		}
		values = append(values, value)
	}

	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	return cpuTimes{idle: idle, total: total}, total > 0
}

func readLoadAverage() []float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return []float64{}
	}
	fields := strings.Fields(string(data))
	loads := make([]float64, 0, 3)
	for i := 0; i < len(fields) && i < 3; i++ {
		value, errParse := strconv.ParseFloat(fields[i], 64)
		if errParse != nil {
			return []float64{}
		}
		loads = append(loads, value)
	}
	return loads
}

func readSystemMemoryStatus() serverStatusResource {
	if resource, ok := readMeminfoStatus(); ok {
		return resource
	}

	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		return serverStatusResource{}
	}
	unit := uint64(info.Unit)
	if unit == 0 {
		unit = 1
	}
	total := info.Totalram * unit
	free := (info.Freeram + info.Bufferram) * unit
	return resourceFromTotalFree(total, free)
}

func readMeminfoStatus() (serverStatusResource, bool) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return serverStatusResource{}, false
	}

	var total uint64
	var available uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, errParse := strconv.ParseUint(fields[1], 10, 64)
		if errParse != nil {
			continue
		}
		bytes := value * 1024
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			total = bytes
		case "MemAvailable":
			available = bytes
		}
	}
	if total == 0 {
		return serverStatusResource{}, false
	}
	return resourceFromTotalFree(total, available), true
}

func readDiskStatus(path string) (*serverStatusResource, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, false
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	resource := resourceFromTotalFree(total, free)
	return &resource, true
}
