//go:build windows

package supervisor

import "os/exec"

// setCmdSysProcAttr is a no-op on Windows because the engine runs inside WSL2 or via dedicated wrappers.
func setCmdSysProcAttr(cmd *exec.Cmd) {
}
