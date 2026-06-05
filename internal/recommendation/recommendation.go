package recommendation

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/stepanusjanu19/envdoctoragent/internal/common"
	"github.com/stepanusjanu19/envdoctoragent/internal/container"
	"github.com/stepanusjanu19/envdoctoragent/internal/scanner"
	"github.com/stepanusjanu19/envdoctoragent/internal/system"
)

// Recommendation represents a single actionable fix suggestion.
type Recommendation struct {
	Category    string  `json:"category"`
	Severity    string  `json:"severity"` // critical, warning, info
	Source      string  `json:"source"`
	Risk        string  `json:"risk"`
	Confidence  float64 `json:"confidence"`
	SafeToRun   bool    `json:"safe_to_run"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Command     string  `json:"command"`      // suggested fix command, never executed by envdoctor
	ManualSteps string  `json:"manual_steps"` // if automatic fix is not possible
}

// Report holds all recommendations.
type Report struct {
	Recommendations []Recommendation `json:"recommendations"`
	Summary         string           `json:"summary"`
}

// GenerateRecommendations gathers Phase 1/2 scan data and produces recommendations.
func GenerateRecommendations() (*Report, error) {
	sysInfo, _ := system.Detect()
	tools, _ := scanner.ScanToolchain()
	pathReport, _ := scanner.ScanPath()
	containerInfo, _ := container.CheckContainerEnvironments()

	return GenerateRecommendationsFromScans(sysInfo, tools, pathReport, containerInfo), nil
}

// GenerateRecommendationsFromScans produces recommendations from already-collected scan data.
func GenerateRecommendationsFromScans(sysInfo *common.SystemInfo, tools []common.ToolInfo, pathReport *common.PathReport, containerInfo *container.ContainerInfo) *Report {
	var recs []Recommendation

	recs = append(recs, analyzeSystem(sysInfo)...)
	recs = append(recs, analyzePath(pathReport)...)
	recs = append(recs, analyzeToolchain(tools)...)
	recs = append(recs, analyzeContainer(containerInfo)...)

	return &Report{
		Recommendations: recs,
		Summary:         summarize(recs),
	}
}

func analyzeSystem(info *common.SystemInfo) []Recommendation {
	if info == nil {
		return []Recommendation{
			{
				Category:    "System",
				Severity:    "warning",
				Source:      "system",
				Risk:        "low",
				Confidence:  0.70,
				SafeToRun:   true,
				Title:       "System information could not be collected",
				Description: "Environment diagnosis is less complete without OS, kernel, architecture, and shell information.",
				ManualSteps: "Run envdoctor system and inspect any command or permission error.",
			},
		}
	}

	var recs []Recommendation
	if info.Kernel == "" || info.Kernel == "unknown" {
		recs = append(recs, Recommendation{
			Category:    "System",
			Severity:    "info",
			Source:      "system",
			Risk:        "low",
			Confidence:  0.65,
			SafeToRun:   true,
			Title:       "Kernel version is unknown",
			Description: "Some environment recommendations may be less precise when the kernel version cannot be detected.",
			ManualSteps: "Check whether uname or platform-specific system commands are available.",
		})
	}
	if info.Shell == "" {
		recs = append(recs, Recommendation{
			Category:    "System",
			Severity:    "info",
			Source:      "system",
			Risk:        "low",
			Confidence:  0.65,
			SafeToRun:   true,
			Title:       "Shell could not be detected",
			Description: "PATH and environment configuration fixes may require knowing the active shell.",
			ManualSteps: "Inspect SHELL or COMSPEC and your terminal profile configuration.",
		})
	}
	return recs
}

func analyzePath(report *common.PathReport) []Recommendation {
	if report == nil {
		return nil
	}

	var recs []Recommendation
	for _, issue := range report.Issues {
		switch issue.Type {
		case "missing", "not-directory", "broken-symlink":
			recs = append(recs, Recommendation{
				Category:    "PATH",
				Severity:    "warning",
				Source:      "path",
				Risk:        "medium",
				Confidence:  0.90,
				SafeToRun:   false,
				Title:       fmt.Sprintf("Invalid PATH entry: %s", issue.Entry),
				Description: issue.Description,
				ManualSteps: "Remove or fix this PATH entry in your shell profile or environment configuration.",
			})
		case "duplicate":
			recs = append(recs, Recommendation{
				Category:    "PATH",
				Severity:    "info",
				Source:      "path",
				Risk:        "low",
				Confidence:  0.85,
				SafeToRun:   false,
				Title:       fmt.Sprintf("Duplicate PATH entry: %s", issue.Entry),
				Description: issue.Description,
				ManualSteps: "Remove duplicate PATH entries to keep command lookup predictable.",
			})
		case "empty":
			recs = append(recs, Recommendation{
				Category:    "PATH",
				Severity:    "warning",
				Source:      "path",
				Risk:        "medium",
				Confidence:  0.85,
				SafeToRun:   false,
				Title:       "Empty PATH entry detected",
				Description: issue.Description,
				ManualSteps: "Remove empty separators from PATH to avoid implicitly searching the current directory.",
			})
		}
	}
	return recs
}

func analyzeToolchain(tools []common.ToolInfo) []Recommendation {
	byCommand := make(map[string]common.ToolInfo, len(tools))
	for _, tool := range tools {
		byCommand[tool.Name] = tool
	}

	var recs []Recommendation
	if !toolFound(byCommand, "Git") {
		recs = append(recs, missingToolRecommendation("Git", "Version control and repository workflows usually require Git.", "Install Git from your OS package manager or https://git-scm.com."))
	}
	if !toolFound(byCommand, "Python") && !toolFound(byCommand, "Python3") {
		recs = append(recs, missingToolRecommendation("Python", "Python is required by many development tools and project scripts.", "Install Python 3 and verify python3 --version."))
	}
	if !toolFound(byCommand, "Node.js") {
		recs = append(recs, missingToolRecommendation("Node.js", "Node.js is required for JavaScript, TypeScript, and many frontend toolchains.", "Install Node.js or use a version manager such as nvm."))
	}
	return recs
}

func missingToolRecommendation(name, description, manual string) Recommendation {
	return Recommendation{
		Category:    "Toolchain",
		Severity:    "warning",
		Source:      "toolchain",
		Risk:        "medium",
		Confidence:  0.90,
		SafeToRun:   false,
		Title:       fmt.Sprintf("%s is not installed", name),
		Description: description,
		ManualSteps: manual,
	}
}

func analyzeContainer(info *container.ContainerInfo) []Recommendation {
	if info == nil {
		return nil
	}

	var recs []Recommendation
	if info.Docker != nil && info.Docker.Installed {
		switch info.Docker.DaemonStatus {
		case "permission denied":
			recs = append(recs, Recommendation{
				Category:    "Container",
				Severity:    "critical",
				Source:      "container",
				Risk:        "high",
				Confidence:  0.90,
				SafeToRun:   false,
				Title:       "Docker daemon permission denied",
				Description: "The current user cannot access the Docker daemon.",
				Command:     "sudo usermod -aG docker $USER",
				ManualSteps: "On Linux, add the user to the docker group and log out/in. On macOS or Windows, check Docker Desktop permissions and context.",
			})
		case "not running":
			recs = append(recs, Recommendation{
				Category:    "Container",
				Severity:    "critical",
				Source:      "container",
				Risk:        "medium",
				Confidence:  0.90,
				SafeToRun:   false,
				Title:       "Docker daemon is not running",
				Description: "Docker commands will fail until the daemon is started.",
				Command:     dockerStartCommand(),
				ManualSteps: "Start Docker Desktop or the Docker service, then re-run envdoctor scan container.",
			})
		case "timeout":
			recs = append(recs, Recommendation{
				Category:    "Container",
				Severity:    "warning",
				Source:      "container",
				Risk:        "low",
				Confidence:  0.75,
				SafeToRun:   true,
				Title:       "Docker check timed out",
				Description: "Docker CLI did not return before the diagnostic timeout.",
				ManualSteps: "Run docker version manually and inspect Docker Desktop or daemon startup state.",
			})
		}
	}

	if info.Podman != nil && info.Podman.Installed && info.Podman.Status == "not available" {
		recs = append(recs, Recommendation{
			Category:    "Container",
			Severity:    "warning",
			Source:      "container",
			Risk:        "low",
			Confidence:  0.80,
			SafeToRun:   false,
			Title:       "Podman is installed but not available",
			Description: "Podman exists on PATH, but podman info did not report a healthy runtime.",
			ManualSteps: "Start or initialize the Podman machine/runtime, then run podman info.",
		})
	}

	if info.Kubernetes != nil && info.Kubernetes.Installed && info.Kubernetes.ConfigStatus == "not configured" {
		recs = append(recs, Recommendation{
			Category:    "Kubernetes",
			Severity:    "info",
			Source:      "container",
			Risk:        "low",
			Confidence:  0.80,
			SafeToRun:   true,
			Title:       "kubectl is installed but not configured",
			Description: "kubectl is present, but no current context was detected.",
			ManualSteps: "Configure kubeconfig or select a context with kubectl config use-context.",
		})
	}

	return recs
}

func dockerStartCommand() string {
	if runtime.GOOS == "linux" {
		return "sudo systemctl start docker"
	}
	return ""
}

func summarize(recs []Recommendation) string {
	critical := 0
	warning := 0
	info := 0
	for _, r := range recs {
		switch r.Severity {
		case "critical":
			critical++
		case "warning":
			warning++
		case "info":
			info++
		}
	}
	return fmt.Sprintf("Found %d recommendations (critical: %d, warning: %d, info: %d)", len(recs), critical, warning, info)
}

// PrintReport prints the recommendation report.
func PrintReport(report *Report) {
	fmt.Println(report.Summary)
	fmt.Println()

	if len(report.Recommendations) == 0 {
		fmt.Println("No recommendations at this time. Your environment looks healthy!")
		return
	}

	for i, rec := range report.Recommendations {
		fmt.Printf("--- Recommendation %d ---\n", i+1)
		fmt.Printf("[%s] [%s] %s\n", rec.Severity, rec.Category, rec.Title)
		fmt.Printf("Source: %s | Risk: %s | Confidence: %.2f | Safe to run: %t\n", rec.Source, rec.Risk, rec.Confidence, rec.SafeToRun)
		fmt.Printf("Description: %s\n", rec.Description)
		if rec.Command != "" {
			fmt.Printf("Suggested command: %s\n", rec.Command)
		}
		if rec.ManualSteps != "" {
			fmt.Printf("Manual steps: %s\n", rec.ManualSteps)
		}
		fmt.Println()
	}
}

func toolFound(tools map[string]common.ToolInfo, name string) bool {
	tool, ok := tools[name]
	return ok && tool.Found
}

func cmdExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
