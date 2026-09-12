//go:build !windows

package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (dc *ProcessSupervisor) ProbeWSLStatus() WSLStatus {
	return WSLStatus{
		State:          WSL2Ready,
		IsWindows:      false,
		DefaultVersion: 2,
		DistroName:     "native",
	}
}

func (dc *ProcessSupervisor) InitiateWSLInstall() error {
	return nil
}

func (dc *ProcessSupervisor) SetupWSLDistro(distroName string) error {
	return nil
}

func (dc *ProcessSupervisor) executeWSL(ctx context.Context, siriusBin string, chainConfigPath string, localDataDir string, libEnvList []string) error {
	return nil
}

func (dc *ProcessSupervisor) stopWSL() error {
	return nil
}

func (dc *ProcessSupervisor) SetupPortProxy() error {
	return nil
}

// ToWSLPath is a no-op on non-Windows platforms.
func ToWSLPath(path string) string {
	return path
}

// FromWSLPath is a no-op on non-Windows platforms.
func FromWSLPath(path string) string {
	return path
}

func (dc *ProcessSupervisor) runCatapultRecovery(localDataDir string) {
	recoveryBin := filepath.Join(dc.binPath, "catapult.recovery")
	if _, err := os.Stat(recoveryBin); err != nil {
		return
	}

	dc.broadcastLog("[Supervisor] Executing native catapult.recovery reconciliation...")
	recCmd := exec.Command(recoveryBin, dc.chainConfigPath)
	recCmd.Dir = filepath.Dir(dc.chainConfigPath)
	out, recErr := recCmd.CombinedOutput()
	if recErr != nil {
		dc.broadcastLog(fmt.Sprintf("<warning> [Supervisor] catapult.recovery returned: %v (details: %s)", recErr, strings.TrimSpace(string(out))))
	} else {
		dc.broadcastLog("[Supervisor] catapult.recovery reconciliation completed successfully.")
	}

	_ = exec.Command("sync").Run()
	dc.clearLocks(localDataDir)
}

