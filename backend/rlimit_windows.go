//go:build windows

package main

func raiseFileDescriptorLimit() {
	// On Windows, process file handles are dynamically allocated by the OS;
	// POSIX rlimit does not apply.
}
