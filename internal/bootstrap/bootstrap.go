package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stepanusjanu19/envdoctoragent/internal/dependencies"
	"github.com/stepanusjanu19/envdoctoragent/internal/installplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/version"
)

// Action is a plan-only bootstrap step.
type Action struct {
	ID            string   `json:"id,omitempty"`
	Category      string   `json:"category"`
	Title         string   `json:"title"`
	Command       string   `json:"command,omitempty"`
	Args          []string `json:"args,omitempty"`
	ManualSteps   string   `json:"manual_steps,omitempty"`
	WorkingDir    string   `json:"working_dir,omitempty"`
	Risk          string   `json:"risk"`
	RequiresAdmin bool     `json:"requires_admin"`
	SafeToRun     bool     `json:"safe_to_run"`
	Timeout       string   `json:"timeout,omitempty"`
	RollbackHint  string   `json:"rollback_hint,omitempty"`
	Source        string   `json:"source"`
	Status        string   `json:"status,omitempty"`
}

// ServiceHint describes an inferred service/runtime need for bootstrap.
type ServiceHint struct {
	Name        string `json:"name"`
	SourceFile  string `json:"source_file"`
	Description string `json:"description"`
}

// Plan is the bootstrap planner output.
type Plan struct {
	Directory    string                              `json:"directory"`
	VersionPlan  *version.PlanReport                 `json:"version_plan,omitempty"`
	Dependencies []*dependencies.ProjectDependencies `json:"dependencies,omitempty"`
	InstallPlans []installplan.Action                `json:"install_plans,omitempty"`
	ServiceHints []ServiceHint                       `json:"service_hints,omitempty"`
	Actions      []Action                            `json:"actions"`
	Summary      string                              `json:"summary"`
}

// GeneratePlan builds a one-command setup plan without executing setup actions.
func GeneratePlan(dir string) (*Plan, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	versionPlan, _ := version.Plan(absDir)
	dependencyReports, _ := dependencies.AnalyzeAllDependencies(absDir)
	serviceHints := detectServiceHints(absDir)

	plan := &Plan{
		Directory:    absDir,
		VersionPlan:  versionPlan,
		Dependencies: dependencyReports,
		ServiceHints: serviceHints,
		InstallPlans: installPlansForProject(dependencyReports, serviceHints),
	}
	plan.Actions = buildActions(versionPlan, dependencyReports, serviceHints)
	plan.Summary = fmt.Sprintf("Generated %d bootstrap actions. No commands were executed.", len(plan.Actions))
	return plan, nil
}

func installPlansForProject(reports []*dependencies.ProjectDependencies, hints []ServiceHint) []installplan.Action {
	tools := map[string]bool{}
	for _, report := range reports {
		for _, tool := range installToolsForReport(report) {
			tools[tool] = true
		}
	}
	for _, hint := range hints {
		if hint.Name == "docker" {
			tools["docker"] = true
		}
	}

	var actions []installplan.Action
	for tool := range tools {
		actions = append(actions, installplan.Generate(tool).Action)
	}
	return actions
}

func buildActions(versionPlan *version.PlanReport, reports []*dependencies.ProjectDependencies, hints []ServiceHint) []Action {
	var actions []Action
	if versionPlan != nil {
		for _, action := range versionPlan.Actions {
			actions = append(actions, Action{
				Category:      "Version",
				Title:         action.Title,
				Command:       action.Command,
				ManualSteps:   action.ManualSteps,
				Risk:          action.Risk,
				RequiresAdmin: action.RequiresAdmin,
				SafeToRun:     false,
				Source:        "version",
			})
		}
	}

	for _, report := range reports {
		if action, ok := dependencyAction(report); ok {
			actions = append(actions, action)
		}
	}

	for _, hint := range hints {
		actions = append(actions, Action{
			Category:    "Service",
			Title:       fmt.Sprintf("Verify %s service", hint.Name),
			ManualSteps: fmt.Sprintf("%s Run envdoctor service status %s if it is managed by the OS.", hint.Description, hint.Name),
			Risk:        "low",
			SafeToRun:   false,
			Source:      hint.SourceFile,
		})
	}

	return finalizeActions(dedupeActions(actions))
}

func dependencyAction(report *dependencies.ProjectDependencies) (Action, bool) {
	source := report.SourceFile
	switch report.Ecosystem {
	case "python":
		if strings.Contains(source, "requirements.txt") {
			return Action{
				Category:    "Dependencies",
				Title:       "Install Python dependencies",
				Command:     "python -m pip install -r requirements.txt",
				ManualSteps: "Review virtual environment selection before running pip.",
				Risk:        "medium",
				SafeToRun:   false,
				Source:      source,
			}, true
		}
		return Action{
			Category:    "Dependencies",
			Title:       "Install Python project dependencies",
			Command:     "python -m pip install -e .",
			ManualSteps: "Review pyproject.toml before installing editable dependencies.",
			Risk:        "medium",
			SafeToRun:   false,
			Source:      source,
		}, true
	case "node":
		command := "npm install"
		switch report.PackageManager {
		case "yarn":
			command = "yarn install"
		case "pnpm":
			command = "pnpm install"
		case "bun":
			command = "bun install"
		}
		return Action{
			Category:    "Dependencies",
			Title:       "Install Node.js dependencies",
			Command:     command,
			ManualSteps: "Use a lockfile-aware install mode when reproducible install is required.",
			Risk:        "medium",
			SafeToRun:   false,
			Source:      source,
		}, true
	case "go":
		return Action{
			Category:    "Dependencies",
			Title:       "Download Go modules",
			Command:     "go mod download",
			ManualSteps: "Review go.mod and go.sum changes after dependency operations.",
			Risk:        "low",
			SafeToRun:   false,
			Source:      source,
		}, true
	case "rust":
		return Action{
			Category:    "Dependencies",
			Title:       "Fetch Rust crates",
			Command:     "cargo fetch",
			ManualSteps: "Review Cargo.lock changes after dependency operations.",
			Risk:        "low",
			SafeToRun:   false,
			Source:      source,
		}, true
	case "php":
		return Action{
			Category:    "Dependencies",
			Title:       "Install PHP Composer dependencies",
			Command:     "composer install",
			ManualSteps: "Review composer.lock and PHP extension requirements before running composer.",
			Risk:        "medium",
			SafeToRun:   false,
			Source:      source,
		}, true
	case "jvm":
		if report.PackageManager == "gradle" {
			return metadataDependencyAction(report, "Review Gradle dependencies", "gradle dependencies"), true
		}
		return metadataDependencyAction(report, "Resolve Maven dependencies", "mvn dependency:resolve"), true
	case "dotnet":
		return metadataDependencyAction(report, "Restore .NET dependencies", "dotnet restore"), true
	case "ruby":
		return metadataDependencyAction(report, "Install Ruby gems", "bundle install"), true
	case "dart":
		return metadataDependencyAction(report, "Fetch Dart or Flutter packages", "dart pub get"), true
	case "swift":
		return metadataDependencyAction(report, "Resolve Swift packages", "swift package resolve"), true
	case "elixir":
		return metadataDependencyAction(report, "Fetch Elixir dependencies", "mix deps.get"), true
	case "lua":
		return metadataDependencyAction(report, "Review LuaRocks dependencies", "luarocks install --deps-only *.rockspec"), true
	case "r":
		return Action{
			Category:    "Dependencies",
			Title:       "Review R package dependencies",
			ManualSteps: "Review DESCRIPTION and restore packages with the project's preferred R dependency workflow.",
			Risk:        "medium",
			SafeToRun:   false,
			Source:      source,
		}, true
	case "julia":
		return metadataDependencyAction(report, "Instantiate Julia project", "julia --project=. -e 'using Pkg; Pkg.instantiate()'"), true
	case "haskell":
		if report.PackageManager == "stack" {
			return metadataDependencyAction(report, "Review Stack dependencies", "stack build --dry-run"), true
		}
		return metadataDependencyAction(report, "Review Cabal dependencies", "cabal build --dry-run"), true
	case "perl":
		return metadataDependencyAction(report, "Install Perl dependencies", "cpanm --installdeps ."), true
	case "cpp":
		if report.PackageManager == "vcpkg" {
			return metadataDependencyAction(report, "Install vcpkg dependencies", "vcpkg install"), true
		}
		return metadataDependencyAction(report, "Install Conan dependencies", "conan install . --build=missing"), true
	}
	return Action{}, false
}

func installToolsForReport(report *dependencies.ProjectDependencies) []string {
	switch report.Ecosystem {
	case "python":
		return []string{"python"}
	case "node":
		return []string{"node"}
	case "go":
		return []string{"go"}
	case "rust":
		return []string{"rust"}
	case "php":
		return []string{"php", "composer"}
	case "jvm":
		if report.PackageManager == "gradle" {
			return []string{"java", "gradle"}
		}
		return []string{"java", "maven"}
	case "dotnet":
		return []string{"dotnet"}
	case "ruby":
		return []string{"ruby"}
	case "dart":
		return []string{"dart"}
	case "swift":
		return []string{"swift"}
	case "elixir":
		return []string{"elixir"}
	case "lua":
		return []string{"lua"}
	case "r":
		return []string{"r"}
	case "julia":
		return []string{"julia"}
	case "haskell":
		return []string{"haskell"}
	case "perl":
		return []string{"perl"}
	case "cpp":
		if report.PackageManager == "vcpkg" {
			return []string{"vcpkg"}
		}
		return []string{"conan"}
	default:
		return nil
	}
}

func metadataDependencyAction(report *dependencies.ProjectDependencies, title, command string) Action {
	return Action{
		Category:    "Dependencies",
		Title:       title,
		Command:     command,
		ManualSteps: fmt.Sprintf("Review %s first. This ecosystem is currently %s in envdoctor and commands are suggestions only.", report.SourceFile, report.ValidationStatus),
		Risk:        "medium",
		SafeToRun:   false,
		Source:      report.SourceFile,
	}
}

func detectServiceHints(dir string) []ServiceHint {
	candidates := map[string]string{
		"Dockerfile":          "Dockerfile indicates container build/runtime support may be needed.",
		"docker-compose.yml":  "Compose file indicates Docker or a compatible container runtime may be needed.",
		"docker-compose.yaml": "Compose file indicates Docker or a compatible container runtime may be needed.",
		"compose.yml":         "Compose file indicates Docker or a compatible container runtime may be needed.",
		"compose.yaml":        "Compose file indicates Docker or a compatible container runtime may be needed.",
	}

	var hints []ServiceHint
	for file, description := range candidates {
		if fileExists(filepath.Join(dir, file)) {
			hints = append(hints, ServiceHint{
				Name:        "docker",
				SourceFile:  file,
				Description: description,
			})
		}
	}
	return hints
}

func dedupeActions(actions []Action) []Action {
	seen := make(map[string]bool, len(actions))
	var result []Action
	for _, action := range actions {
		key := action.Category + "\x00" + action.Title + "\x00" + action.Command + "\x00" + action.Source
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, action)
	}
	return result
}

func finalizeActions(actions []Action) []Action {
	for i := range actions {
		if actions[i].ID == "" {
			actions[i].ID = fmt.Sprintf("bootstrap-%03d", i+1)
		}
		if actions[i].Status == "" {
			actions[i].Status = "plan-only"
		}
		if actions[i].Timeout == "" && actions[i].Command != "" {
			actions[i].Timeout = "5m"
		}
		if actions[i].RollbackHint == "" && actions[i].Command != "" {
			actions[i].RollbackHint = "Review project lockfiles and use version control to restore project files if needed."
		}
	}
	return actions
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
