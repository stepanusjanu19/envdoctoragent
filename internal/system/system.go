package system

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"envdoctor/internal/common"
)

// Detect collects system information and returns a SystemInfo struct
func Detect() (*common.SystemInfo, error) {
	info := &common.SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	// Kernel version
	out, err := exec.Command("uname", "-r").Output()
	if err == nil {
		info.Kernel = strings.TrimSpace(string(out))
	}

	// Distribution
	info.Distribution = detectDistribution()

	// Shell
	info.Shell = os.Getenv("SHELL")
	if info.Shell == "" {
		info.Shell = "/bin/sh" // fallback
	}

	return info, nil
}

func detectDistribution() string {
	// Try /etc/os-release first (modern Linux distributions)
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		lines := bytes.Split(data, []byte("\n"))
		for _, line := range lines {
			if strings.HasPrefix(string(line), "PRETTY_NAME=\"") {
				return strings.TrimSuffix(strings.TrimPrefix(string(line), "PRETTY_NAME=\""), "\"")
			}
		}
	}

	// Fallback to uname
	out, err := exec.Command("uname", "-o").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}

	return "unknown"
}

// Print prints system information to stdout
func Print(info *common.SystemInfo) {
	fmt.Printf("Operating System: %s\n", info.OS)
	fmt.Printf("Kernel Version:   %s\n", info.Kernel)
	fmt.Printf("Architecture:     %s\n", info.Arch)
	fmt.Printf("Distribution:     %s\n", info.Distribution)
	fmt.Printf("Shell:            %s\n", info.Shell)
}
