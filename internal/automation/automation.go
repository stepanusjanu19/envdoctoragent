package automation

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/stepanusjanu19/envdoctoragent/internal/agent"
	"github.com/stepanusjanu19/envdoctoragent/internal/executor"
	"github.com/stepanusjanu19/envdoctoragent/internal/projectops"
)

const (
	GoalDiagnose  = agent.GoalDiagnose
	GoalOnboard   = agent.GoalOnboard
	GoalRepair    = agent.GoalRepair
	GoalScaffold  = agent.GoalScaffold
	GoalBootstrap = agent.GoalBootstrap
	GoalMaintain  = "maintain"
)

// Options configures controlled automation orchestration.
type Options struct {
	Directory string             `json:"directory"`
	Goal      string             `json:"goal"`
	Template  string             `json:"template,omitempty"`
	Profile   string             `json:"profile"`
	MaxRisk   string             `json:"max_risk"`
	Project   projectops.Options `json:"-"`
}

// PolicySummary describes the selected automation action set.
type PolicySummary struct {
	SelectedGoals             []string `json:"selected_goals"`
	SelectedActions           int      `json:"selected_actions"`
	SkippedActions            int      `json:"skipped_actions"`
	BlockedActions            int      `json:"blocked_actions"`
	RequiredApprovals         []string `json:"required_approvals,omitempty"`
	RollbackHints             []string `json:"rollback_hints,omitempty"`
	ProductionMutationBlocked bool     `json:"production_mutation_blocked"`
	Notes                     []string `json:"notes,omitempty"`
}

// Report is the controlled automation output.
type Report struct {
	Goal          string            `json:"goal"`
	Profile       string            `json:"profile"`
	MaxRisk       string            `json:"max_risk"`
	Directory     string            `json:"directory"`
	Status        string            `json:"status"`
	SelectedGoals []string          `json:"selected_goals"`
	AgentReports  []*agent.Report   `json:"agent_reports,omitempty"`
	Actions       []executor.Action `json:"actions,omitempty"`
	PolicySummary PolicySummary     `json:"policy_summary"`
	Execution     *executor.Report  `json:"execution,omitempty"`
	Summary       string            `json:"summary"`
}

// Plan builds an automation report without executing actions.
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

	selectedGoals, err := expandGoals(options.Goal)
	if err != nil {
		return nil, err
	}
	report.SelectedGoals = selectedGoals

	for _, goal := range selectedGoals {
		agentOptions := agent.Options{
			Directory: dir,
			Goal:      goal,
			Template:  options.Template,
			Profile:   options.Profile,
			MaxRisk:   options.MaxRisk,
			Project:   options.Project,
		}
		agentReport, err := agent.Plan(agentOptions)
		if err != nil {
			return nil, err
		}
		report.AgentReports = append(report.AgentReports, agentReport)
		report.Actions = append(report.Actions, agentReport.Actions...)
		if agentReport.ProjectPlan != nil && agentReport.ProjectPlan.Directory != "" {
			report.Directory = agentReport.ProjectPlan.Directory
		}
	}

	report.PolicySummary = summarizePolicy(report.SelectedGoals, report.Actions, nil, options.Profile)
	report.Summary = summarize(report)
	return report, nil
}

// Run creates an automation plan and routes actions through the shared executor.
func Run(options Options, execution executor.Options) (*Report, error) {
	report, err := Plan(options)
	if err != nil {
		return nil, err
	}
	if len(report.Actions) == 0 {
		report.Status = "completed"
		report.PolicySummary = summarizePolicy(report.SelectedGoals, report.Actions, nil, report.Profile)
		report.Summary = summarize(report)
		return report, nil
	}
	if baseDir := executionBaseDir(report); baseDir != "" {
		execution.BaseDir = baseDir
	} else if execution.BaseDir == "" {
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
	report.PolicySummary = summarizePolicy(report.SelectedGoals, report.Actions, executionReport, report.Profile)
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

func expandGoals(goal string) ([]string, error) {
	switch goal {
	case GoalDiagnose, GoalOnboard, GoalRepair, GoalScaffold, GoalBootstrap:
		return []string{goal}, nil
	case GoalMaintain:
		return []string{GoalDiagnose, GoalRepair}, nil
	default:
		return nil, fmt.Errorf("unsupported automation goal %q", goal)
	}
}

func executionBaseDir(report *Report) string {
	for _, agentReport := range report.AgentReports {
		if agentReport.ProjectPlan != nil && agentReport.ProjectPlan.ExecutionBaseDir != "" {
			return agentReport.ProjectPlan.ExecutionBaseDir
		}
	}
	return report.Directory
}

func summarizePolicy(goals []string, actions []executor.Action, execution *executor.Report, profile string) PolicySummary {
	summary := PolicySummary{
		SelectedGoals:   append([]string{}, goals...),
		SelectedActions: len(actions),
	}
	approvalSeen := map[string]bool{}
	rollbackSeen := map[string]bool{}
	for _, action := range actions {
		if action.Type == "manual" || action.Status == "skipped" || action.Status == "metadata-only" {
			summary.SkippedActions++
		}
		if action.Status == "blocked" {
			summary.BlockedActions++
		}
		if action.MutatesProject || action.CreatesProject || action.RequiresAdmin || action.Command != "" || action.Type == "mkdir" || action.Type == "write_file" {
			approval := fmt.Sprintf("%s: %s", valueOrDefault(action.Category, "Action"), action.Title)
			if !approvalSeen[approval] {
				summary.RequiredApprovals = append(summary.RequiredApprovals, approval)
				approvalSeen[approval] = true
			}
		}
		if strings.TrimSpace(action.RollbackHint) != "" && !rollbackSeen[action.RollbackHint] {
			summary.RollbackHints = append(summary.RollbackHints, action.RollbackHint)
			rollbackSeen[action.RollbackHint] = true
		}
	}
	if execution != nil {
		summary.BlockedActions = 0
		summary.SkippedActions = 0
		for _, result := range execution.Results {
			switch result.Status {
			case "blocked":
				summary.BlockedActions++
			case "skipped":
				summary.SkippedActions++
			}
		}
	}
	if profile == executor.ProfileProduction && hasMutation(actions) {
		summary.ProductionMutationBlocked = true
		summary.Notes = append(summary.Notes, "production profile blocks mutating automation by default")
	}
	if len(actions) == 0 {
		summary.Notes = append(summary.Notes, "no executable automation actions were selected")
	}
	return summary
}

func hasMutation(actions []executor.Action) bool {
	for _, action := range actions {
		if action.MutatesProject || action.CreatesProject || action.Type == "mkdir" || action.Type == "write_file" || action.Command != "" {
			return true
		}
	}
	return false
}

func summarize(report *Report) string {
	if report.Execution != nil {
		return fmt.Sprintf("Automation %s run completed with %d selected goal(s) and %d action(s). %s", report.Goal, len(report.SelectedGoals), len(report.Actions), report.Execution.Summary)
	}
	return fmt.Sprintf("Automation %s plan selected %d goal(s) and %d action(s). No commands were executed.", report.Goal, len(report.SelectedGoals), len(report.Actions))
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
