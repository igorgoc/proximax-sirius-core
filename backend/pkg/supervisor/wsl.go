package supervisor

import (
	"bytes"
	"os"
	"strings"
	"unicode/utf16"
)

func isPathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func cleanWSLOutput(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	// Check for UTF-16LE BOM (0xFF, 0xFE)
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		u16 := make([]uint16, (len(data)-2)/2)
		for i := 0; i < len(u16); i++ {
			u16[i] = uint16(data[2+2*i]) | (uint16(data[3+2*i]) << 8)
		}
		return strings.TrimSpace(string(utf16.Decode(u16)))
	}
	// Check if null-interleaved UTF-16LE without BOM
	if len(data) >= 4 && data[1] == 0x00 && data[3] == 0x00 {
		var u16 []uint16
		for i := 0; i+1 < len(data); i += 2 {
			u16 = append(u16, uint16(data[i])|(uint16(data[i+1])<<8))
		}
		return strings.TrimSpace(string(utf16.Decode(u16)))
	}
	// Fallback standard UTF-8 / ASCII, stripping any stray null bytes
	cleaned := bytes.ReplaceAll(data, []byte{0x00}, []byte{})
	return strings.TrimSpace(string(cleaned))
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
