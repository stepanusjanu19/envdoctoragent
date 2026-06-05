package fixplan

import (
	"fmt"
	"runtime"

	"github.com/stepanusjanu19/envdoctoragent/internal/dependencies"
	"github.com/stepanusjanu19/envdoctoragent/internal/recommendation"
	"github.com/stepanusjanu19/envdoctoragent/internal/service"
	"github.com/stepanusjanu19/envdoctoragent/internal/version"
)

// Action is a non-mutating repair suggestion.
type Action struct {
	ID             string   `json:"id,omitempty"`
	Category       string   `json:"category"`
	Source         string   `json:"source"`
	Operation      string   `json:"operation,omitempty"`
	Ecosystem      string   `json:"ecosystem,omitempty"`
	PackageManager string   `json:"package_manager,omitempty"`
	Packages       []string `json:"packages,omitempty"`
	Title          string   `json:"title"`
	Description    string   `json:"description,omitempty"`
	Command        string   `json:"command,omitempty"`
	Args           []string `json:"args,omitempty"`
	ManualSteps    string   `json:"manual_steps,omitempty"`
	WorkingDir     string   `json:"working_dir,omitempty"`
	Risk           string   `json:"risk"`
	RequiresAdmin  bool     `json:"requires_admin"`
	SafeToRun      bool     `json:"safe_to_run"`
	MutatesProject bool     `json:"mutates_project,omitempty"`
	CreatesProject bool     `json:"creates_project,omitempty"`
	Timeout        string   `json:"timeout,omitempty"`
	RollbackHint   string   `json:"rollback_hint,omitempty"`
	Confidence     float64  `json:"confidence"`
	Status         string   `json:"status"`
}

// Report contains safe fix plan actions.
type Report struct {
	Platform string   `json:"platform"`
	Actions  []Action `json:"actions"`
	Summary  string   `json:"summary"`
}

// Generate creates a plan-only fix report. It never executes commands.
func Generate(dir string) (*Report, error) {
	var actions []Action

	recReport, err := recommendation.GenerateRecommendations()
	if err == nil && recReport != nil {
		for _, rec := range recReport.Recommendations {
			actions = append(actions, fromRecommendation(rec))
		}
	}

	versionPlan, err := version.Plan(dir)
	if err == nil && versionPlan != nil {
		for _, action := range versionPlan.Actions {
			actions = append(actions, fromVersionAction(action))
		}
	}

	dependencyReports, err := dependencies.AnalyzeAllDependencies(dir)
	if err == nil {
		for _, report := range dependencyReports {
			actions = append(actions, fromDependencyReport(report)...)
		}
	}

	serviceReport, err := service.ListServices()
	if err == nil && serviceReport != nil && serviceReport.Status != "available" {
		actions = append(actions, Action{
			Category:    "Service",
			Source:      "service",
			Title:       "Service manager is not available",
			Description: serviceReport.Message,
			ManualSteps: "Inspect the native service manager manually or run envdoctor service list on a supported platform.",
			Risk:        "low",
			SafeToRun:   false,
			Confidence:  0.80,
			Status:      serviceReport.Status,
		})
	}

	actions = finalizeActions(actions)
	return &Report{
		Platform: runtime.GOOS,
		Actions:  actions,
		Summary:  fmt.Sprintf("Generated %d safe fix plan actions. No commands were executed.", len(actions)),
	}, nil
}

func fromRecommendation(rec recommendation.Recommendation) Action {
	return Action{
		Category:      rec.Category,
		Source:        rec.Source,
		Title:         rec.Title,
		Description:   rec.Description,
		Command:       rec.Command,
		ManualSteps:   rec.ManualSteps,
		Risk:          rec.Risk,
		RequiresAdmin: commandRequiresAdmin(rec.Command),
		SafeToRun:     false,
		Confidence:    rec.Confidence,
		Status:        "plan-only",
	}
}

func fromDependencyReport(report *dependencies.ProjectDependencies) []Action {
	var actions []Action
	if report.ValidationStatus == "metadata-only" {
		actions = append(actions, Action{
			Category:    "Dependencies",
			Source:      report.SourceFile,
			Title:       fmt.Sprintf("Review %s dependency metadata", report.Language),
			Description: fmt.Sprintf("%s is detected from %s using %s. Envdoctor records metadata for this ecosystem but does not validate installed package versions yet.", report.ManifestType, report.SourceFile, report.PackageManager),
			ManualSteps: "Run envdoctor bootstrap plan for setup suggestions, then use the project's package manager manually if changes are needed.",
			Risk:        "low",
			SafeToRun:   false,
			Confidence:  0.70,
			Status:      "metadata-only",
		})
		return actions
	}

	missing := 0
	for _, dep := range report.Dependencies {
		if dep.Required && !dep.Installed {
			missing++
			if missing > 5 {
				continue
			}
			actions = append(actions, Action{
				Category:    "Dependencies",
				Source:      report.SourceFile,
				Title:       fmt.Sprintf("Dependency may be missing: %s", dep.Name),
				Description: fmt.Sprintf("%s dependency from %s was not detected in the local environment.", report.Language, report.SourceFile),
				ManualSteps: fmt.Sprintf("Review %s and install dependencies with %s outside envdoctor.", report.SourceFile, report.PackageManager),
				Risk:        "medium",
				SafeToRun:   false,
				Confidence:  0.75,
				Status:      "plan-only",
			})
		}
	}
	if missing > 5 {
		actions = append(actions, Action{
			Category:    "Dependencies",
			Source:      report.SourceFile,
			Title:       "Additional dependencies may be missing",
			Description: fmt.Sprintf("%d required dependencies were not detected; only the first 5 are listed individually.", missing),
			ManualSteps: fmt.Sprintf("Run envdoctor scan dependencies --json for full detail, then use %s manually if changes are needed.", report.PackageManager),
			Risk:        "medium",
			SafeToRun:   false,
			Confidence:  0.70,
			Status:      "plan-only",
		})
	}
	return actions
}

func fromVersionAction(action version.PlanAction) Action {
	return Action{
		Category:      "Version",
		Source:        "version",
		Title:         action.Title,
		Command:       action.Command,
		ManualSteps:   action.ManualSteps,
		Risk:          action.Risk,
		RequiresAdmin: action.RequiresAdmin,
		SafeToRun:     false,
		Confidence:    0.80,
		Status:        "plan-only",
	}
}

func commandRequiresAdmin(command string) bool {
	return len(command) >= 5 && command[:5] == "sudo "
}

func finalizeActions(actions []Action) []Action {
	for i := range actions {
		if actions[i].ID == "" {
			actions[i].ID = fmt.Sprintf("fix-%03d", i+1)
		}
		if actions[i].Status == "" {
			actions[i].Status = "plan-only"
		}
		if actions[i].Timeout == "" && actions[i].Command != "" {
			actions[i].Timeout = "2m"
		}
		if actions[i].RollbackHint == "" && actions[i].Command != "" {
			actions[i].RollbackHint = "Use the pre-apply snapshot and version control to inspect and manually revert changes."
		}
	}
	return actions
}
