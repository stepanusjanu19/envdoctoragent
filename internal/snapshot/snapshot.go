package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"envdoctor/internal/common"
	"envdoctor/internal/scanner"
	"envdoctor/internal/system"
)

// Snapshot represents a point-in-time capture of the environment
type Snapshot struct {
	Timestamp time.Time         `json:"timestamp"`
	System    *common.SystemInfo  `json:"system"`
	Tools     []common.ToolInfo   `json:"tools"`
	Path      *common.PathReport  `json:"path"`
}

// CreateSnapshot creates a new environment snapshot by collecting actual system data
func CreateSnapshot() (*Snapshot, error) {
	snap := &Snapshot{
		Timestamp: time.Now(),
	}

	// Collect system info
	sysInfo, err := system.Detect()
	if err == nil {
		snap.System = sysInfo
	}

	// Collect toolchain info
	tools, err := scanner.ScanToolchain()
	if err == nil {
		snap.Tools = tools
	}

	// Collect path report
	pathReport, err := scanner.ScanPath()
	if err == nil {
		snap.Path = pathReport
	}

	return snap, nil
}

// SaveSnapshot saves a snapshot to a file
func SaveSnapshot(snapshot *Snapshot, filename string) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// LoadSnapshot loads a snapshot from a file
func LoadSnapshot(filename string) (*Snapshot, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// CompareSnapshots compares two environment snapshots
func CompareSnapshots(a, b *Snapshot) error {
	fmt.Println("Comparing snapshots...")

	// Compare system information
	if a.System != nil && b.System != nil {
		fmt.Printf("System: %s vs %s\n", a.System.OS, b.System.OS)
		fmt.Printf("Kernel: %s vs %s\n", a.System.Kernel, b.System.Kernel)
		fmt.Printf("Architecture: %s vs %s\n", a.System.Arch, b.System.Arch)
	}

	// Compare toolchain
	fmt.Println("Toolchain differences:")
	for _, toolA := range a.Tools {
		found := false
		for _, toolB := range b.Tools {
			if toolA.Name == toolB.Name {
				found = true
				if toolA.Version != toolB.Version {
					fmt.Printf("  %s version changed from %s to %s\n", toolA.Name, toolA.Version, toolB.Version)
				}
				break
			}
		}
		if !found {
			fmt.Printf("  %s was removed\n", toolA.Name)
		}
	}

	return nil
}

// PrintSnapshot prints snapshot information
func PrintSnapshot(snap *Snapshot) {
	fmt.Printf("Snapshot taken at: %s\n", snap.Timestamp.Format("2006-01-02 15:04:05"))

	if snap.System != nil {
		fmt.Printf("System: %s (%s) %s\n", snap.System.OS, snap.System.Distribution, snap.System.Kernel)
	}

	fmt.Println("Installed tools:")
	for _, tool := range snap.Tools {
		if tool.Found {
			fmt.Printf("  %s: %s\n", tool.Name, tool.Version)
		} else {
			fmt.Printf("  %s: not found\n", tool.Name)
		}
	}
}
