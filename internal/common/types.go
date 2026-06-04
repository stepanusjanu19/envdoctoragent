package common

// SystemInfo holds basic system information
type SystemInfo struct {
	OS           string `json:"os"`
	Kernel       string `json:"kernel"`
	Arch         string `json:"arch"`
	Distribution string `json:"distribution"`
	Shell        string `json:"shell"`
}

// ToolInfo holds information about an installed tool
type ToolInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
	Found   bool   `json:"found"`
}

// PathIssue represents a problem found in the PATH
type PathIssue struct {
	Type        string `json:"type"`
	Entry       string `json:"entry"`
	Description string `json:"description"`
}

// PathReport summarizes PATH analysis
type PathReport struct {
	Entries   []string   `json:"entries"`
	Issues    []PathIssue `json:"issues"`
	IssueCount int       `json:"issue_count"`
}
