package agent

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/stepanusjanu19/envdoctoragent/internal/bootstrap"
	"github.com/stepanusjanu19/envdoctoragent/internal/diagnose"
	"github.com/stepanusjanu19/envdoctoragent/internal/executor"
	"github.com/stepanusjanu19/envdoctoragent/internal/fixplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/projectops"
)

const (
	GoalDiagnose  = "diagnose"
	GoalOnboard   = "onboard"
	GoalRepair    = "repair"
	GoalScaffold  = "scaffold"
	GoalBootstrap = "bootstrap"
)

// Options configures deterministic local agent orchestration.
type Options struct {
	Directory string             `json:"directory"`
	Goal      string             `json:"goal"`
	Template  string             `json:"template,omitempty"`
	Profile   string             `json:"profile"`
	MaxRisk   string             `json:"max_risk"`
	Project   projectops.Options `json:"-"`
}

// Report is the autonomous-agent preview output.
type Report struct {
	Goal          string                 `json:"goal"`
	Profile       string                 `json:"profile"`
	MaxRisk       string                 `json:"max_risk"`
	Directory     string                 `json:"directory"`
	Status        string                 `json:"status"`
	Diagnostics   *diagnose.Report       `json:"diagnostics,omitempty"`
	ProjectScan   *projectops.ScanReport `json:"project_scan,omitempty"`
	FixPlan       *fixplan.Report        `json:"fix_plan,omitempty"`
	BootstrapPlan *bootstrap.Plan        `json:"bootstrap_plan,omitempty"`
	ProjectPlan   *projectops.Plan       `json:"project_plan,omitempty"`
	Actions       []executor.Action      `json:"actions,omitempty"`
	Execution     *executor.Report       `json:"execution,omitempty"`
	Summary       string                 `json:"summary"`
}

// Plan builds an agent report without executing actions.
func Plan(options Options) (*Report, error) {
	options = normalizeOptions(options)
	dir, err := filepath.Abs(options.Directory)
	if err != nil {
		return nil, err
	}
	report := &Report{
		Goal:      options.Goal,
		Profile:   options.Profile,
		MaxRisk:   options.MaxRisk,
		Directory: dir,
		Status:    "plan-only",
	}

	switch options.Goal {
	case GoalDiagnose:
		diagnostics, err := diagnose.Run()
		if err != nil {
			return nil, err
		}
		report.Diagnostics = diagnostics
	case GoalOnboard:
		diagnostics, _ := diagnose.Run()
		projectScan, _ := projectops.Scan(dir)
		bootstrapPlan, _ := bootstrap.GeneratePlan(dir)
		report.Diagnostics = diagnostics
		report.ProjectScan = projectScan
		report.BootstrapPlan = bootstrapPlan
		report.Actions = executor.FromBootstrapPlan(bootstrapPlan, dir)
	case GoalRepair:
		fixPlan, err := fixplan.Generate(dir)
		if err != nil {
			return nil, err
		}
		report.FixPlan = fixPlan
		report.Actions = executor.FromFixPlan(fixPlan, dir)
	case GoalScaffold:
		if strings.TrimSpace(options.Template) == "" {
			return nil, fmt.Errorf("agent scaffold goal requires --template")
		}
		projectPlan, err := projectops.GenerateInitPlan(options.Template, dir, options.Project)
		if err != nil {
			return nil, err
		}
		report.ProjectPlan = projectPlan
		report.Directory = projectPlan.Directory
		report.Actions = projectPlan.Actions
	case GoalBootstrap:
		bootstrapPlan, err := bootstrap.GeneratePlan(dir)
		if err != nil {
			return nil, err
		}
		report.BootstrapPlan = bootstrapPlan
		report.Actions = executor.FromBootstrapPlan(bootstrapPlan, dir)
	default:
		return nil, fmt.Errorf("unsupported agent goal %q", options.Goal)
	}

	report.Summary = summarize(report)
	return report, nil
}

// Run creates an agent plan and sends allowed actions to the executor.
func Run(options Options, execution executor.Options) (*Report, error) {
	report, err := Plan(options)
	if err != nil {
		return nil, err
	}
	if len(report.Actions) == 0 {
		report.Status = "completed"
		report.Summary = summarize(report)
		return report, nil
	}
	if execution.BaseDir == "" {
		execution.BaseDir = report.Directory
	}
	if execution.Profile == "" && execution.PolicyFile == "" {
		execution.Profile = report.Profile
	}
	if execution.MaxRisk == "" && execution.PolicyFile == "" {
		execution.MaxRisk = report.MaxRisk
	}
	executionReport, err := executor.Execute(report.Actions, execution)
	if err != nil {
		return nil, err
	}
	report.Execution = executionReport
	report.Status = executionReport.Mode
	report.Summary = summarize(report)
	return report, nil
}

func normalizeOptions(options Options) Options {
	if strings.TrimSpace(options.Directory) == "" {
		options.Directory = "."
	}
	options.Goal = strings.ToLower(strings.TrimSpace(options.Goal))
	if options.Goal == "" {
		options.Goal = GoalDiagnose
	}
	if options.Profile == "" {
		options.Profile = executor.ProfileDevelopment
	}
	if options.MaxRisk == "" {
		options.MaxRisk = executor.RiskHigh
	}
	return options
}

func summarize(report *Report) string {
	if report.Execution != nil {
		return fmt.Sprintf("Agent %s run completed with %d planned action(s). %s", report.Goal, len(report.Actions), report.Execution.Summary)
	}
	return fmt.Sprintf("Agent %s plan generated %d action(s). No commands were executed.", report.Goal, len(report.Actions))
}
