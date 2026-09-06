//go:build darwin

package supervisor

import "syscall"

func getDiskSpaceBytes(path string) (freeBytes, totalBytes, usedBytes uint64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, err
	}
	blockSize := uint64(stat.Bsize)
	freeBytes = stat.Bavail * blockSize
	totalBytes = stat.Blocks * blockSize
	usedBytes = totalBytes - (stat.Bfree * blockSize)
	return freeBytes, totalBytes, usedBytes, nil
}
