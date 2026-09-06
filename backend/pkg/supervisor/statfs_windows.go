//go:build windows

package supervisor

import (
	"syscall"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func getDiskSpaceBytes(path string) (freeBytes, totalBytes, usedBytes uint64, err error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, 0, err
	}
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	r1, _, e1 := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)
	if r1 == 0 {
		return 0, 0, 0, e1
	}
	used := totalNumberOfBytes - totalNumberOfFreeBytes
	return totalNumberOfFreeBytes, totalNumberOfBytes, used, nil
}
