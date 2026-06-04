package container

import (
	"fmt"
	"os/exec"
	"strings"

	"envdoctor/internal/command"
)

// ContainerInfo holds information about container environments
type ContainerInfo struct {
	Docker     *DockerInfo     `json:"docker,omitempty"`
	Podman     *PodmanInfo     `json:"podman,omitempty"`
	Kubernetes *KubernetesInfo `json:"kubernetes,omitempty"`
}

// DockerInfo holds Docker-specific information
type DockerInfo struct {
	Installed         bool   `json:"installed"`
	Version           string `json:"version"`
	DaemonStatus      string `json:"daemon_status"`
	SocketAccess      bool   `json:"socket_access"`
	ConfigConsistency string `json:"config_consistency"`
}

// PodmanInfo holds Podman-specific information
type PodmanInfo struct {
	Installed         bool   `json:"installed"`
	Version           string `json:"version"`
	Status            string `json:"status"`
	ConfigConsistency string `json:"config_consistency"`
}

// KubernetesInfo holds Kubernetes-specific information
type KubernetesInfo struct {
	Installed    bool   `json:"installed"`
	Version      string `json:"version"`
	Context      string `json:"context"`
	ConfigStatus string `json:"config_status"`
}

// CheckContainerEnvironments checks the status of container environments
func CheckContainerEnvironments() (*ContainerInfo, error) {
	info := &ContainerInfo{}

	// Check Docker
	dockerInfo, err := checkDocker()
	if err == nil {
		info.Docker = dockerInfo
	}

	// Check Podman
	podmanInfo, err := checkPodman()
	if err == nil {
		info.Podman = podmanInfo
	}

	// Check Kubernetes
	k8sInfo, err := checkKubernetes()
	if err == nil {
		info.Kubernetes = k8sInfo
	}

	return info, nil
}

// checkDocker checks Docker installation and status
func checkDocker() (*DockerInfo, error) {
	info := &DockerInfo{
		Installed:         false,
		DaemonStatus:      "not installed",
		ConfigConsistency: "not installed",
	}

	// Check if Docker is installed
	_, err := exec.LookPath("docker")
	if err != nil {
		return info, nil // Docker not installed, but not an error
	}

	info.Installed = true

	// Get Docker version
	versionOut, err := command.Output("docker", "--version")
	if err == nil {
		info.Version = strings.TrimSpace(string(versionOut))
	}

	// Use `docker version` instead of `docker info` for a cleaner permission/daemon check.
	// `docker version` works without daemon permissions if the user is in the docker group,
	// but if the daemon is down, it will clearly state it.
	out, err := command.CombinedOutput("docker", "version")
	if err == nil {
		info.DaemonStatus = "running"
		info.SocketAccess = true
		info.ConfigConsistency = checkDockerConfigConsistency()
	} else {
		outputStr := strings.ToLower(string(out) + " " + err.Error())
		// Robust checking for common error messages
		if strings.Contains(outputStr, "permission denied") ||
			strings.Contains(outputStr, "got permission denied") ||
			strings.Contains(outputStr, "dial unix") {
			info.DaemonStatus = "permission denied"
			info.SocketAccess = false
			info.ConfigConsistency = "not available"
		} else if strings.Contains(outputStr, "cannot connect to the docker daemon") ||
			strings.Contains(outputStr, "is the docker daemon running") {
			info.DaemonStatus = "not running"
			info.SocketAccess = false
			info.ConfigConsistency = "not available"
		} else if strings.Contains(outputStr, "command timed out") {
			info.DaemonStatus = "timeout"
			info.SocketAccess = false
			info.ConfigConsistency = "not available"
		} else {
			info.DaemonStatus = "unknown error"
			info.SocketAccess = false
			info.ConfigConsistency = "not available"
		}
	}

	return info, nil
}

// checkPodman checks Podman installation and status
func checkPodman() (*PodmanInfo, error) {
	info := &PodmanInfo{
		Installed:         false,
		Status:            "not installed",
		ConfigConsistency: "not installed",
	}

	// Check if Podman is installed
	_, err := exec.LookPath("podman")
	if err != nil {
		return info, nil // Podman not installed, but not an error
	}

	info.Installed = true

	// Get Podman version
	versionOut, err := command.Output("podman", "--version")
	if err == nil {
		info.Version = strings.TrimSpace(string(versionOut))
	}

	// Check configuration consistency
	info.ConfigConsistency = checkPodmanConfigConsistency()
	if info.ConfigConsistency == "consistent" {
		info.Status = "available"
	} else {
		info.Status = "not available"
	}

	return info, nil
}

// checkKubernetes checks Kubernetes installation and status
func checkKubernetes() (*KubernetesInfo, error) {
	info := &KubernetesInfo{
		Installed:    false,
		ConfigStatus: "not installed",
	}

	// Check if kubectl is installed
	_, err := exec.LookPath("kubectl")
	if err != nil {
		return info, nil // kubectl not installed, but not an error
	}

	info.Installed = true

	// Get kubectl version
	versionOut, err := command.Output("kubectl", "version", "--client")
	if err == nil {
		info.Version = strings.TrimSpace(string(versionOut))
	}

	// Get current context
	contextOut, err := command.Output("kubectl", "config", "current-context")
	if err == nil {
		info.Context = strings.TrimSpace(string(contextOut))
		if info.Context != "" {
			info.ConfigStatus = "configured"
		} else {
			info.ConfigStatus = "not configured"
		}
	} else {
		info.ConfigStatus = "not configured"
	}

	return info, nil
}

// checkDockerConfigConsistency checks Docker configuration consistency
func checkDockerConfigConsistency() string {
	// Check if docker info can be retrieved
	_, err := command.CombinedOutput("docker", "info")
	if err != nil {
		return "inconsistent"
	}

	// Check if docker version matches client and server versions
	versionOut, err := command.CombinedOutput("docker", "version", "--format", "{{.Client.Version}}")
	if err != nil {
		return "inconsistent"
	}

	clientVersion := strings.TrimSpace(string(versionOut))

	serverOut, err := command.CombinedOutput("docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return "inconsistent"
	}

	serverVersion := strings.TrimSpace(string(serverOut))

	if clientVersion != serverVersion {
		return "version_mismatch"
	}

	return "consistent"
}

// checkPodmanConfigConsistency checks Podman configuration consistency
func checkPodmanConfigConsistency() string {
	// Check if podman info can be retrieved
	_, err := command.CombinedOutput("podman", "info")
	if err != nil {
		return "inconsistent"
	}

	// Check if podman version is available
	versionOut, err := command.CombinedOutput("podman", "version", "--format", "{{.Version}}")
	if err != nil {
		return "inconsistent"
	}

	version := strings.TrimSpace(string(versionOut))
	if version == "" {
		return "inconsistent"
	}

	return "consistent"
}

// PrintContainerInfo prints container environment information
func PrintContainerInfo(info *ContainerInfo) {
	fmt.Println("Container Environment Check:")

	if info.Docker != nil {
		fmt.Println("  Docker:")
		fmt.Printf("    Installed: %t\n", info.Docker.Installed)
		if info.Docker.Installed {
			fmt.Printf("    Version: %s\n", info.Docker.Version)
			fmt.Printf("    Daemon Status: %s\n", info.Docker.DaemonStatus)
			fmt.Printf("    Socket Access: %t\n", info.Docker.SocketAccess)
			fmt.Printf("    Config Consistency: %s\n", info.Docker.ConfigConsistency)
		}
	}

	if info.Podman != nil {
		fmt.Println("  Podman:")
		fmt.Printf("    Installed: %t\n", info.Podman.Installed)
		if info.Podman.Installed {
			fmt.Printf("    Version: %s\n", info.Podman.Version)
			fmt.Printf("    Status: %s\n", info.Podman.Status)
			fmt.Printf("    Config Consistency: %s\n", info.Podman.ConfigConsistency)
		}
	}

	if info.Kubernetes != nil {
		fmt.Println("  Kubernetes:")
		fmt.Printf("    Installed: %t\n", info.Kubernetes.Installed)
		if info.Kubernetes.Installed {
			fmt.Printf("    Version: %s\n", info.Kubernetes.Version)
			fmt.Printf("    Context: %s\n", info.Kubernetes.Context)
			fmt.Printf("    Config Status: %s\n", info.Kubernetes.ConfigStatus)
		}
	}
}
