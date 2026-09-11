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

func (dc *ProcessSupervisor) executeWSL(ctx context.Context, siriusBin string, chainConfigPath string, libEnvList []string) error {
	return nil
}

func (dc *ProcessSupervisor) stopWSL() error {
	return nil
}

func (dc *ProcessSupervisor) SetupPortProxy() error {
	return nil
}
