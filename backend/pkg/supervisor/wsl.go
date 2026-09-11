package supervisor

import "os"

func isPathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

type WSLState string

const (
	WSLNotInstalled WSLState = "WSL_NOT_INSTALLED"
	WSLV1Only       WSLState = "WSL_V1_ONLY"
	WSL2NoDistro    WSLState = "WSL2_NO_DISTRO"
	WSL2Ready       WSLState = "WSL2_READY"
)

type WSLStatus struct {
	State               WSLState `json:"state"`
	IsWindows           bool     `json:"isWindows"`
	DefaultVersion      int      `json:"defaultVersion"`
	DistroName          string   `json:"distroName"`
	WSLInstallInitiated bool     `json:"wslInstallInitiated"`
	ErrorCode           string   `json:"errorCode,omitempty"`
	ErrorMessage        string   `json:"errorMessage,omitempty"`
	RawStatus           string   `json:"rawStatus,omitempty"`
}
