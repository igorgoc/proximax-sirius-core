//go:build !darwin

package supervisor

import (
	"path/filepath"
	"syscall"
)

func getDiskSpaceBytes(path string) (freeBytes, totalBytes, usedBytes uint64, err error) {
	p := path
	if p == "" {
		p = "."
	}
	for {
		var stat syscall.Statfs_t
		if err = syscall.Statfs(p, &stat); err == nil {
			blockSize := uint64(stat.Bsize)
			if stat.Frsize > 0 {
				blockSize = uint64(stat.Frsize)
			}
			freeBytes = stat.Bavail * blockSize
			totalBytes = stat.Blocks * blockSize
			usedBytes = totalBytes - (stat.Bfree * blockSize)
			return freeBytes, totalBytes, usedBytes, nil
		}
		parent := filepath.Dir(p)
		if parent == p || parent == "" || parent == "." {
			break
		}
		p = parent
	}

	var stat syscall.Statfs_t
	if err = syscall.Statfs(".", &stat); err == nil {
		blockSize := uint64(stat.Bsize)
		if stat.Frsize > 0 {
			blockSize = uint64(stat.Frsize)
		}
		freeBytes = stat.Bavail * blockSize
		totalBytes = stat.Blocks * blockSize
		usedBytes = totalBytes - (stat.Bfree * blockSize)
		return freeBytes, totalBytes, usedBytes, nil
	}
	return 0, 0, 0, err
}
