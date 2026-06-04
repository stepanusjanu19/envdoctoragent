package recommendation

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Recommendation represents a single actionable fix suggestion
type Recommendation struct {
	Category    string `json:"category"`
	Severity    string `json:"severity"` // critical, warning, info
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command"`    // suggested fix command
	ManualSteps string `json:"manual_steps"` // if automatic fix is not possible
}

// Report holds all recommendations
type Report struct {
	Recommendations []Recommendation `json:"recommendations"`
	Summary         string           `json:"summary"`
}

// GenerateRecommendations analyzes all available scan results and produces recommendations
func GenerateRecommendations() (*Report, error) {
	var recs []Recommendation

	// 1. System-level recommendations
	recs = append(recs, analyzeSystem()...)

	// 2. PATH recommendations
	recs = append(recs, analyzePath()...)

	// 3. Toolchain recommendations
	recs = append(recs, analyzeToolchain()...)

	// 4. Container recommendations
	recs = append(recs, analyzeContainer()...)

	// Build summary
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

	summary := fmt.Sprintf("Found %d recommendations (critical: %d, warning: %d, info: %d)", len(recs), critical, warning, info)

	return &Report{
		Recommendations: recs,
		Summary:         summary,
	}, nil
}

// analyzeSystem checks for common system-level issues
func analyzeSystem() []Recommendation {
	var recs []Recommendation

	// Check if running on outdated OS (simple heuristic)
	if runtime.GOOS == "linux" {
		// Check if system is up to date using package manager
		if cmdExists("dnf") {
			out, err := exec.Command("dnf", "check-update", "-q").Output()
			if err == nil && len(out) > 0 {
				recs = append(recs, Recommendation{
					Category:    "System",
					Severity:    "warning",
					Title:       "System packages are outdated",
					Description: "Your system has available package updates. Keeping packages updated ensures security and stability.",
					Command:     "sudo dnf update -y",
				})
			}
		} else if cmdExists("apt") {
			out, err := exec.Command("apt", "list", "--upgradable", "-qq").Output()
			if err == nil && len(out) > 0 {
				recs = append(recs, Recommendation{
					Category:    "System",
					Severity:    "warning",
					Title:       "System packages are outdated",
					Description: "Your system has available package updates. Keeping packages updated ensures security and stability.",
					Command:     "sudo apt update && sudo apt upgrade -y",
				})
			}
		}
	}

	return recs
}

// analyzePath checks PATH issues
func analyzePath() []Recommendation {
	var recs []Recommendation

	// This will be called by the diagnose command
	// Placeholder for PATH-specific recommendations

	return recs
}

// analyzeToolchain checks for missing or outdated tools
func analyzeToolchain() []Recommendation {
	var recs []Recommendation

	// Check for missing Git
	if !cmdExists("git") {
		recs = append(recs, Recommendation{
			Category:    "Toolchain",
			Severity:    "critical",
			Title:       "Git is not installed",
			Description: "Git is required for most development workflows including cloning repositories and version control.",
			ManualSteps: "Install Git using your package manager (e.g., sudo dnf install git or sudo apt install git)",
		})
	}

	// Check for missing Python
	if !cmdExists("python") && !cmdExists("python3") {
		recs = append(recs, Recommendation{
			Category:    "Toolchain",
			Severity:    "warning",
			Title:       "Python is not installed",
			Description: "Python is commonly used for scripting, data science, and many development tools.",
			ManualSteps: "Install Python 3 using your package manager (e.g., sudo dnf install python3 or sudo apt install python3)",
		})
	}

	// Check for missing Node.js
	if !cmdExists("node") {
		recs = append(recs, Recommendation{
			Category:    "Toolchain",
			Severity:    "info",
			Title:       "Node.js is not installed",
			Description: "Node.js is required for JavaScript/TypeScript development and modern front-end tooling.",
			ManualSteps: "Install Node.js from https://nodejs.org or use your package manager",
		})
	}

	return recs
}

// analyzeContainer checks container environment for issues
func analyzeContainer() []Recommendation {
	var recs []Recommendation

	// Check Docker daemon status
	if cmdExists("docker") {
		out, err := exec.Command("docker", "version").CombinedOutput()
		if err != nil {
			output := string(out)
			if strings.Contains(output, "permission denied") {
				recs = append(recs, Recommendation{
					Category:    "Container",
					Severity:    "critical",
					Title:       "Docker daemon permission denied",
					Description: "Your user does not have permission to access the Docker daemon. This prevents running Docker commands without sudo.",
					Command:     "sudo usermod -aG docker $USER",
					ManualSteps: "Log out and log back in for the group change to take effect, or run 'newgrp docker'",
				})
			} else if strings.Contains(output, "Cannot connect") {
				recs = append(recs, Recommendation{
					Category:    "Container",
					Severity:    "critical",
					Title:       "Docker daemon is not running",
					Description: "The Docker daemon is not running. Docker commands will fail until the daemon is started.",
					Command:     "sudo systemctl start docker",
				})
			}
		}
	}

	return recs
}

// PrintReport prints the recommendation report
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
		fmt.Printf("Description: %s\n", rec.Description)
		if rec.Command != "" {
			fmt.Printf("Fix: %s\n", rec.Command)
		}
		if rec.ManualSteps != "" {
			fmt.Printf("Manual steps: %s\n", rec.ManualSteps)
		}
		fmt.Println()
	}
}

func cmdExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
