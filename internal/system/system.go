package system

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"

	"envdoctor/internal/command"
	"envdoctor/internal/common"
)

// Detect collects system information and returns a SystemInfo struct
func Detect() (*common.SystemInfo, error) {
	info := &common.SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	info.Kernel = detectKernel(runtime.GOOS)
	info.Distribution = detectDistribution()
	info.Shell = detectShell(runtime.GOOS)

	return info, nil
}

func detectKernel(goos string) string {
	switch goos {
	case "windows":
		out, err := command.Output("cmd", "/C", "ver")
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	default:
		out, err := command.Output("uname", "-r")
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return "unknown"
}

func detectDistribution() string {
	switch runtime.GOOS {
	case "linux":
		return detectLinuxDistribution()
	case "darwin":
		return detectMacOSDistribution()
	case "windows":
		out, err := command.Output("cmd", "/C", "ver")
		if err == nil {
			return strings.TrimSpace(string(out))
		}
		return "Windows"
	default:
		out, err := command.Output("uname", "-o")
		if err == nil {
			return strings.TrimSpace(string(out))
		}
		return "unknown"
	}
}

func detectLinuxDistribution() string {
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		lines := bytes.Split(data, []byte("\n"))
		for _, key := range []string{"PRETTY_NAME", "NAME"} {
			for _, line := range lines {
				parts := bytes.SplitN(line, []byte("="), 2)
				if len(parts) != 2 || string(parts[0]) != key {
					continue
				}
				value := strings.Trim(string(parts[1]), `"`)
				if value != "" {
					return value
				}
			}
		}
	}

	out, err := command.Output("uname", "-o")
	if err == nil {
		return strings.TrimSpace(string(out))
	}

	return "unknown"
}

func detectMacOSDistribution() string {
	nameOut, nameErr := command.Output("sw_vers", "-productName")
	versionOut, versionErr := command.Output("sw_vers", "-productVersion")
	if nameErr == nil && versionErr == nil {
		name := strings.TrimSpace(string(nameOut))
		version := strings.TrimSpace(string(versionOut))
		if name != "" && version != "" {
			return fmt.Sprintf("%s %s", name, version)
		}
	}

	out, err := command.Output("uname", "-s")
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return "macOS"
}

func detectShell(goos string) string {
	if goos == "windows" {
		if shell := os.Getenv("COMSPEC"); shell != "" {
			return shell
		}
		return "cmd.exe"
	}

	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

// Print prints system information to stdout
func Print(info *common.SystemInfo) {
	fmt.Printf("Operating System: %s\n", info.OS)
	fmt.Printf("Kernel Version:   %s\n", info.Kernel)
	fmt.Printf("Architecture:     %s\n", info.Arch)
	fmt.Printf("Distribution:     %s\n", info.Distribution)
	fmt.Printf("Shell:            %s\n", info.Shell)
}
