//go:build !windows

package main

import "syscall"

func raiseFileDescriptorLimit() {
	var rLimit syscall.Rlimit
	rLimit.Cur = 65536
	rLimit.Max = 65536
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		rLimit.Cur = 10240
		rLimit.Max = 10240
		_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	}
}
