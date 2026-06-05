package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/stepanusjanu19/envdoctoragent/internal/common"
)

// ScanPath analyzes the PATH environment variable and returns a report
func ScanPath() (*common.PathReport, error) {
	pathEnv := os.Getenv("PATH")
	entries := splitPath(pathEnv)

	report := &common.PathReport{
		Entries: entries,
		Issues:  []common.PathIssue{},
	}

	seen := make(map[string]bool)
	reported := make(map[string]bool) // Track which duplicates we've already reported

	for _, entry := range entries {
		if entry == "" {
			report.Issues = append(report.Issues, common.PathIssue{
				Type:        "empty",
				Entry:       entry,
				Description: "Empty PATH entry",
			})
			continue
		}

		normalized := normalizePathEntry(entry)

		// Check duplicates
		if seen[normalized] {
			// Only report each duplicate path once
			if !reported[normalized] {
				report.Issues = append(report.Issues, common.PathIssue{
					Type:        "duplicate",
					Entry:       entry,
					Description: "Duplicate PATH entry",
				})
				reported[normalized] = true
			}
			continue
		}
		seen[normalized] = true

		// Check broken symlinks before os.Stat, because os.Stat follows the
		// link and reports a missing target as a missing path.
		if isBrokenSymlink(entry) {
			report.Issues = append(report.Issues, common.PathIssue{
				Type:        "broken-symlink",
				Entry:       entry,
				Description: "Broken symbolic link",
			})
			continue
		}

		// Check existence
		info, err := os.Stat(entry)
		if err != nil {
			if os.IsNotExist(err) {
				report.Issues = append(report.Issues, common.PathIssue{
					Type:        "missing",
					Entry:       entry,
					Description: "Directory does not exist",
				})
			} else {
				report.Issues = append(report.Issues, common.PathIssue{
					Type:        "invalid",
					Entry:       entry,
					Description: fmt.Sprintf("Cannot access: %v", err),
				})
			}
			continue
		}

		// Check if directory
		if !info.IsDir() {
			report.Issues = append(report.Issues, common.PathIssue{
				Type:        "not-directory",
				Entry:       entry,
				Description: "Path is not a directory",
			})
			continue
		}
	}

	report.IssueCount = len(report.Issues)
	return report, nil
}

func splitPath(path string) []string {
	separator := string(os.PathListSeparator)
	parts := strings.Split(path, separator)
	return parts
}

func normalizePathEntry(entry string) string {
	normalized := filepath.Clean(entry)
	if resolved, err := filepath.EvalSymlinks(normalized); err == nil {
		normalized = resolved
	}
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}

func isBrokenSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		_, err := os.Stat(path)
		return err != nil
	}
	return false
}

// PrintPathReport prints the PATH analysis report
func PrintPathReport(report *common.PathReport) {
	fmt.Printf("PATH entries analyzed: %d\n", len(report.Entries))
	if report.IssueCount == 0 {
		fmt.Println("No issues found.")
		return
	}

	fmt.Printf("\nIssues found (%d):\n", report.IssueCount)
	for _, issue := range report.Issues {
		fmt.Printf("  [%s] %s: %s\n", issue.Type, issue.Entry, issue.Description)
	}
}
