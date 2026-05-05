//go:build windows

package management

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpaceExW  = kernel32.NewProc("GetDiskFreeSpaceExW")
)

type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

func readCPUTimes() (cpuTimes, bool) {
	var idle syscall.Filetime
	var kernel syscall.Filetime
	var user syscall.Filetime
	ret, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if ret == 0 {
		return cpuTimes{}, false
	}
	idleValue := filetimeToUint64(idle)
	total := filetimeToUint64(kernel) + filetimeToUint64(user)
	return cpuTimes{idle: idleValue, total: total}, total > 0
}

func readLoadAverage() []float64 {
	return []float64{}
}

func readSystemMemoryStatus() serverStatusResource {
	status := memoryStatusEx{length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 {
		return serverStatusResource{}
	}
	return resourceFromTotalFree(status.totalPhys, status.availPhys)
}

func readDiskStatus(path string) (*serverStatusResource, bool) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, false
	}
	var freeAvailable uint64
	var total uint64
	var totalFree uint64
	ret, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeAvailable)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if ret == 0 {
		return nil, false
	}
	resource := resourceFromTotalFree(total, freeAvailable)
	return &resource, true
}

func filetimeToUint64(value syscall.Filetime) uint64 {
	return (uint64(value.HighDateTime) << 32) | uint64(value.LowDateTime)
}
