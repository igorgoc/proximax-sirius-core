//go:build !windows

package supervisor

import "context"

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
