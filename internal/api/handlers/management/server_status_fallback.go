//go:build !linux && !windows

package management

func readCPUTimes() (cpuTimes, bool) {
	return cpuTimes{}, false
}

func readLoadAverage() []float64 {
	return []float64{}
}

func readSystemMemoryStatus() serverStatusResource {
	return serverStatusResource{}
}

func readDiskStatus(string) (*serverStatusResource, bool) {
	return nil, false
}
