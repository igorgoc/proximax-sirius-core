//go:build !windows

package supervisor

import (
	"os/exec"
	"syscall"
)

// setCmdSysProcAttr isolates the child engine process in its own process group (PGID).
// This ensures terminal signals like Ctrl+C are caught exclusively by the supervisor,
// preventing premature/duplicate SIGINT aborts on the C++ Catapult engine.
func setCmdSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
