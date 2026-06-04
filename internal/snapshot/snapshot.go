package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"envdoctor/internal/common"
	"envdoctor/internal/scanner"
	"envdoctor/internal/system"
)

// Snapshot represents a point-in-time capture of the environment
type Snapshot struct {
	Timestamp time.Time          `json:"timestamp"`
	System    *common.SystemInfo `json:"system"`
	Tools     []common.ToolInfo  `json:"tools"`
	Path      *common.PathReport `json:"path"`
}

func DefaultFilename(now time.Time) string {
	return fmt.Sprintf("snapshot-%s.json", now.Format("2006-01-02-150405"))
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
	differences, err := DiffSnapshots(a, b)
	if err != nil {
		return err
	}

	fmt.Println("Comparing snapshots...")
	if len(differences) == 0 {
		fmt.Println("No differences found.")
		return nil
	}

	for _, difference := range differences {
		fmt.Printf("  %s\n", difference)
	}

	return nil
}

// DiffSnapshots returns human-readable differences between two snapshots.
func DiffSnapshots(a, b *Snapshot) ([]string, error) {
	if a == nil || b == nil {
		return nil, fmt.Errorf("snapshots must not be nil")
	}

	var differences []string

	// Compare system information
	switch {
	case a.System == nil && b.System != nil:
		differences = append(differences, "System section was added")
	case a.System != nil && b.System == nil:
		differences = append(differences, "System section was removed")
	case a.System != nil && b.System != nil:
		if a.System.OS != b.System.OS {
			differences = append(differences, fmt.Sprintf("System changed from %s to %s", a.System.OS, b.System.OS))
		}
		if a.System.Kernel != b.System.Kernel {
			differences = append(differences, fmt.Sprintf("Kernel changed from %s to %s", a.System.Kernel, b.System.Kernel))
		}
		if a.System.Arch != b.System.Arch {
			differences = append(differences, fmt.Sprintf("Architecture changed from %s to %s", a.System.Arch, b.System.Arch))
		}
	}

	if len(a.Tools) == 0 && len(b.Tools) > 0 {
		differences = append(differences, "Toolchain section was added")
	}
	if len(a.Tools) > 0 && len(b.Tools) == 0 {
		differences = append(differences, "Toolchain section was removed")
	}

	toolsA := make(map[string]common.ToolInfo, len(a.Tools))
	for _, toolA := range a.Tools {
		toolsA[toolA.Name] = toolA
	}

	toolsB := make(map[string]common.ToolInfo, len(b.Tools))
	for _, toolB := range b.Tools {
		toolsB[toolB.Name] = toolB
	}

	for _, toolA := range a.Tools {
		toolB, found := toolsB[toolA.Name]
		if !found {
			differences = append(differences, fmt.Sprintf("%s was removed", toolA.Name))
			continue
		}
		if toolA.Found != toolB.Found {
			differences = append(differences, fmt.Sprintf("%s installed state changed from %t to %t", toolA.Name, toolA.Found, toolB.Found))
		}
		if toolA.Version != toolB.Version {
			differences = append(differences, fmt.Sprintf("%s version changed from %s to %s", toolA.Name, toolA.Version, toolB.Version))
		}
	}

	for _, toolB := range b.Tools {
		if _, found := toolsA[toolB.Name]; !found {
			differences = append(differences, fmt.Sprintf("%s was added", toolB.Name))
		}
	}

	switch {
	case a.Path == nil && b.Path != nil:
		differences = append(differences, "PATH section was added")
	case a.Path != nil && b.Path == nil:
		differences = append(differences, "PATH section was removed")
	case a.Path != nil && b.Path != nil:
		if a.Path.IssueCount != b.Path.IssueCount {
			differences = append(differences, fmt.Sprintf("PATH issue count changed from %d to %d", a.Path.IssueCount, b.Path.IssueCount))
		}
		differences = append(differences, diffPathIssues(a.Path.Issues, b.Path.Issues)...)
	}

	return differences, nil
}

func diffPathIssues(a, b []common.PathIssue) []string {
	issuesA := issueSet(a)
	issuesB := issueSet(b)

	var differences []string
	for key, issue := range issuesA {
		if _, found := issuesB[key]; !found {
			differences = append(differences, fmt.Sprintf("PATH issue removed: [%s] %s: %s", issue.Type, issue.Entry, issue.Description))
		}
	}
	for key, issue := range issuesB {
		if _, found := issuesA[key]; !found {
			differences = append(differences, fmt.Sprintf("PATH issue added: [%s] %s: %s", issue.Type, issue.Entry, issue.Description))
		}
	}

	sort.Strings(differences)
	return differences
}

func issueSet(issues []common.PathIssue) map[string]common.PathIssue {
	set := make(map[string]common.PathIssue, len(issues))
	for _, issue := range issues {
		key := fmt.Sprintf("%s\x00%s\x00%s", issue.Type, issue.Entry, issue.Description)
		set[key] = issue
	}
	return set
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
