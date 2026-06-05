package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/stepanusjanu19/envdoctoragent/internal/bootstrap"
	"github.com/stepanusjanu19/envdoctoragent/internal/fixplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/installplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/snapshot"
	"github.com/stepanusjanu19/envdoctoragent/internal/version"
)

const (
	DefaultTimeout = 2 * time.Minute
	previewLimit   = 4096
)

// Action is the normalized execution contract used by approval-gated apply commands.
type Action struct {
	ID               string   `json:"id"`
	Source           string   `json:"source"`
	Category         string   `json:"category"`
	Title            string   `json:"title"`
	Description      string   `json:"description,omitempty"`
	Command          string   `json:"command,omitempty"`
	Args             []string `json:"args,omitempty"`
	SuggestedCommand string   `json:"suggested_command,omitempty"`
	ManualSteps      string   `json:"manual_steps,omitempty"`
	WorkingDir       string   `json:"working_dir,omitempty"`
	Risk             string   `json:"risk"`
	RequiresAdmin    bool     `json:"requires_admin"`
	SafeToRun        bool     `json:"safe_to_run"`
	Timeout          string   `json:"timeout"`
	RollbackHint     string   `json:"rollback_hint,omitempty"`
	Status           string   `json:"status"`
}

// Options configures one apply run.
type Options struct {
	DryRun   bool
	Approved bool
	BaseDir  string
	AuditLog string
	Timeout  time.Duration
	Now      time.Time
}

// Result describes one action execution or dry-run decision.
type Result struct {
	Action        Action `json:"action"`
	Status        string `json:"status"`
	ExitCode      int    `json:"exit_code,omitempty"`
	DurationMS    int64  `json:"duration_ms,omitempty"`
	StdoutPreview string `json:"stdout_preview,omitempty"`
	StderrPreview string `json:"stderr_preview,omitempty"`
	Error         string `json:"error,omitempty"`
	Message       string `json:"message,omitempty"`
}

// Report is the JSON/text output for apply commands.
type Report struct {
	Mode         string    `json:"mode"`
	Platform     string    `json:"platform"`
	WorkingDir   string    `json:"working_dir"`
	AuditLog     string    `json:"audit_log"`
	SnapshotFile string    `json:"snapshot_file,omitempty"`
	Results      []Result  `json:"results"`
	Summary      string    `json:"summary"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
}

type auditRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Mode      string    `json:"mode"`
	ActionID  string    `json:"action_id,omitempty"`
	Result    *Result   `json:"result,omitempty"`
	Message   string    `json:"message,omitempty"`
}

// Execute runs or previews normalized actions. Mutating execution requires Approved=true and DryRun=false.
func Execute(actions []Action, options Options) (*Report, error) {
	started := nowOrDefault(options.Now)
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	baseDir, err := baseDirectory(options.BaseDir)
	if err != nil {
		return nil, err
	}

	mode := "dry-run"
	if options.Approved && !options.DryRun {
		mode = "apply"
	}

	auditPath := options.AuditLog
	if auditPath == "" {
		auditPath = defaultAuditLog(baseDir, started)
	}
	auditPath, err = filepath.Abs(auditPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(auditPath), 0755); err != nil {
		return nil, err
	}
	auditFile, err := os.OpenFile(auditPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer auditFile.Close()

	report := &Report{
		Mode:       mode,
		Platform:   runtime.GOOS,
		WorkingDir: baseDir,
		AuditLog:   auditPath,
		Results:    []Result{},
		StartedAt:  started,
	}

	writeAudit(auditFile, auditRecord{
		Timestamp: started,
		Type:      "session_start",
		Mode:      mode,
		Message:   fmt.Sprintf("received %d actions", len(actions)),
	})

	if mode == "apply" {
		snapshotFile, err := createPreApplySnapshot(baseDir, started)
		if err != nil {
			return nil, err
		}
		report.SnapshotFile = snapshotFile
		writeAudit(auditFile, auditRecord{
			Timestamp: time.Now(),
			Type:      "snapshot",
			Mode:      mode,
			Message:   snapshotFile,
		})
	}

	for i, action := range actions {
		normalized := normalizeAction(action, i, baseDir, timeout)
		result := executeOne(normalized, mode, timeout)
		report.Results = append(report.Results, result)
		writeAudit(auditFile, auditRecord{
			Timestamp: time.Now(),
			Type:      "action_result",
			Mode:      mode,
			ActionID:  result.Action.ID,
			Result:    &result,
		})
	}

	report.FinishedAt = time.Now()
	report.Summary = summarize(report)
	writeAudit(auditFile, auditRecord{
		Timestamp: report.FinishedAt,
		Type:      "session_finish",
		Mode:      mode,
		Message:   report.Summary,
	})

	return report, nil
}

// FromFixPlan converts a fix plan into executor actions.
func FromFixPlan(report *fixplan.Report, baseDir string) []Action {
	if report == nil {
		return nil
	}
	actions := make([]Action, 0, len(report.Actions))
	for _, action := range report.Actions {
		actions = append(actions, Action{
			ID:               action.ID,
			Source:           valueOrDefault(action.Source, "fixplan"),
			Category:         action.Category,
			Title:            action.Title,
			Description:      action.Description,
			SuggestedCommand: action.Command,
			ManualSteps:      action.ManualSteps,
			WorkingDir:       valueOrDefault(action.WorkingDir, baseDir),
			Risk:             valueOrDefault(action.Risk, "medium"),
			RequiresAdmin:    action.RequiresAdmin,
			SafeToRun:        action.SafeToRun,
			Timeout:          action.Timeout,
			RollbackHint:     action.RollbackHint,
			Status:           action.Status,
		})
	}
	return actions
}

// FromInstallPlan converts an installation plan into executor actions.
func FromInstallPlan(plan *installplan.Plan, baseDir string) []Action {
	if plan == nil {
		return nil
	}
	action := plan.Action
	return []Action{{
		ID:               action.ID,
		Source:           valueOrDefault(action.Source, "installplan"),
		Category:         "Install",
		Title:            fmt.Sprintf("Install %s", action.Tool),
		SuggestedCommand: action.Command,
		ManualSteps:      action.ManualSteps,
		WorkingDir:       valueOrDefault(action.WorkingDir, baseDir),
		Risk:             valueOrDefault(action.Risk, "medium"),
		RequiresAdmin:    action.RequiresAdmin,
		SafeToRun:        action.SafeToRun,
		Timeout:          action.Timeout,
		RollbackHint:     action.RollbackHint,
		Status:           action.Status,
	}}
}

// FromVersionPlan converts a version plan into executor actions.
func FromVersionPlan(report *version.PlanReport, baseDir string) []Action {
	if report == nil {
		return nil
	}
	actions := make([]Action, 0, len(report.Actions))
	for _, action := range report.Actions {
		actions = append(actions, Action{
			ID:               action.ID,
			Source:           valueOrDefault(action.Source, "version"),
			Category:         "Version",
			Title:            action.Title,
			SuggestedCommand: action.Command,
			ManualSteps:      action.ManualSteps,
			WorkingDir:       valueOrDefault(action.WorkingDir, baseDir),
			Risk:             valueOrDefault(action.Risk, "medium"),
			RequiresAdmin:    action.RequiresAdmin,
			SafeToRun:        action.SafeToRun,
			Timeout:          action.Timeout,
			RollbackHint:     action.RollbackHint,
			Status:           "plan-only",
		})
	}
	return actions
}

// FromBootstrapPlan converts bootstrap actions into executor actions.
func FromBootstrapPlan(plan *bootstrap.Plan, baseDir string) []Action {
	if plan == nil {
		return nil
	}
	actions := make([]Action, 0, len(plan.Actions)+len(plan.InstallPlans))
	for _, action := range plan.Actions {
		actions = append(actions, Action{
			ID:               action.ID,
			Source:           valueOrDefault(action.Source, "bootstrap"),
			Category:         action.Category,
			Title:            action.Title,
			SuggestedCommand: action.Command,
			ManualSteps:      action.ManualSteps,
			WorkingDir:       valueOrDefault(action.WorkingDir, baseDir),
			Risk:             valueOrDefault(action.Risk, "medium"),
			RequiresAdmin:    action.RequiresAdmin,
			SafeToRun:        action.SafeToRun,
			Timeout:          action.Timeout,
			RollbackHint:     action.RollbackHint,
			Status:           action.Status,
		})
	}
	for _, action := range plan.InstallPlans {
		actions = append(actions, Action{
			ID:               action.ID,
			Source:           valueOrDefault(action.Source, "bootstrap-install"),
			Category:         "Install",
			Title:            fmt.Sprintf("Install %s", action.Tool),
			SuggestedCommand: action.Command,
			ManualSteps:      action.ManualSteps,
			WorkingDir:       valueOrDefault(action.WorkingDir, baseDir),
			Risk:             valueOrDefault(action.Risk, "medium"),
			RequiresAdmin:    action.RequiresAdmin,
			SafeToRun:        action.SafeToRun,
			Timeout:          action.Timeout,
			RollbackHint:     action.RollbackHint,
			Status:           action.Status,
		})
	}
	return actions
}

func executeOne(action Action, mode string, timeout time.Duration) Result {
	if action.SuggestedCommand == "" && action.Command == "" {
		action.Status = "skipped"
		return Result{
			Action:  action,
			Status:  "skipped",
			Message: "manual-only action; no command to execute",
		}
	}

	if action.Command == "" {
		commandName, args, err := parseSuggestedCommand(action.SuggestedCommand)
		if err != nil {
			action.Status = "blocked"
			return Result{
				Action: action,
				Status: "blocked",
				Error:  err.Error(),
			}
		}
		action.Command = commandName
		action.Args = args
	}

	if err := validateAction(action); err != nil {
		action.Status = "blocked"
		return Result{
			Action: action,
			Status: "blocked",
			Error:  err.Error(),
		}
	}

	if mode == "dry-run" {
		action.Status = "dry-run"
		return Result{
			Action:  action,
			Status:  "dry-run",
			Message: "action validated but not executed; pass --yes to apply",
		}
	}

	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, action.Command, action.Args...)
	if action.WorkingDir != "" {
		cmd.Dir = action.WorkingDir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(started)
	result := Result{
		Action:        action,
		DurationMS:    duration.Milliseconds(),
		StdoutPreview: preview(stdout.String()),
		StderrPreview: preview(stderr.String()),
	}
	if ctx.Err() == context.DeadlineExceeded {
		result.Status = "timeout"
		result.Error = fmt.Sprintf("command timed out after %s", timeout)
		result.Action.Status = result.Status
		return result
	}
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Action.Status = result.Status
		return result
	}
	result.Status = "executed"
	result.Action.Status = result.Status
	return result
}

func normalizeAction(action Action, index int, baseDir string, timeout time.Duration) Action {
	if action.ID == "" {
		action.ID = fmt.Sprintf("action-%03d-%s", index+1, slug(action.Category+"-"+action.Title))
	}
	if action.Source == "" {
		action.Source = "executor"
	}
	if action.Category == "" {
		action.Category = "General"
	}
	if action.Risk == "" {
		action.Risk = "medium"
	}
	if action.WorkingDir == "" {
		action.WorkingDir = baseDir
	} else if absDir, err := filepath.Abs(action.WorkingDir); err == nil {
		action.WorkingDir = absDir
	}
	if action.Timeout == "" {
		action.Timeout = timeout.String()
	}
	if action.RollbackHint == "" && (action.Command != "" || action.SuggestedCommand != "") {
		action.RollbackHint = rollbackHint(action.Category)
	}
	if action.Status == "" {
		action.Status = "pending"
	}
	return action
}

func parseSuggestedCommand(command string) (string, []string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", nil, fmt.Errorf("empty command")
	}
	for _, token := range []string{"&&", "||", ";", "|", ">", "<", "$(", "`", "\n", "\r"} {
		if strings.Contains(command, token) {
			return "", nil, fmt.Errorf("blocked shell operator or expansion %q in suggested command", token)
		}
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", nil, fmt.Errorf("empty command")
	}
	for _, field := range fields {
		if strings.Contains(field, "$") {
			return "", nil, fmt.Errorf("blocked shell variable expansion in argument %q", field)
		}
	}
	return fields[0], fields[1:], nil
}

func validateAction(action Action) error {
	if action.Command == "" {
		return fmt.Errorf("missing executable command")
	}
	if strings.ContainsAny(action.Command, `/\`) {
		return fmt.Errorf("command must be an executable name, not a path: %s", action.Command)
	}
	if !isAllowlisted(action.Command, action.Args) {
		return fmt.Errorf("command is not in the envdoctor execution allowlist: %s", commandLine(action.Command, action.Args))
	}
	if _, err := exec.LookPath(action.Command); err != nil {
		return fmt.Errorf("command is not available on PATH: %s", action.Command)
	}
	return nil
}

func isAllowlisted(commandName string, args []string) bool {
	name := strings.ToLower(filepath.Base(commandName))
	switch name {
	case "brew":
		return hasPrefix(args, "install")
	case "winget":
		return hasPrefix(args, "install")
	case "choco":
		return hasPrefix(args, "install")
	case "npm", "yarn", "pnpm", "bun":
		return hasPrefix(args, "install")
	case "python", "python3":
		return len(args) >= 4 && args[0] == "-m" && args[1] == "pip" && args[2] == "install"
	case "go":
		return len(args) >= 2 && args[0] == "mod" && args[1] == "download"
	case "cargo":
		return hasPrefix(args, "fetch")
	case "composer":
		return hasPrefix(args, "install")
	case "mvn":
		return hasPrefix(args, "dependency:resolve")
	case "gradle":
		return hasPrefix(args, "dependencies")
	case "dotnet":
		return hasPrefix(args, "restore")
	case "bundle":
		return hasPrefix(args, "install")
	case "dart":
		return len(args) >= 2 && args[0] == "pub" && args[1] == "get"
	case "swift":
		return len(args) >= 2 && args[0] == "package" && args[1] == "resolve"
	case "mix":
		return hasPrefix(args, "deps.get")
	case "cpanm":
		return hasPrefix(args, "--installdeps")
	case "vcpkg":
		return hasPrefix(args, "install")
	case "conan":
		return hasPrefix(args, "install")
	case "fnm":
		return hasPrefix(args, "install") || hasPrefix(args, "use")
	case "pyenv", "goenv":
		return hasPrefix(args, "install") || hasPrefix(args, "local")
	case "rustup":
		return len(args) >= 2 && args[0] == "toolchain" && args[1] == "install"
	case "asdf":
		return hasPrefix(args, "install") || hasPrefix(args, "local")
	default:
		return false
	}
}

func createPreApplySnapshot(baseDir string, now time.Time) (string, error) {
	snap, err := snapshot.CreateSnapshot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(baseDir, ".envdoctor", "snapshots")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "pre-apply-"+now.Format("2006-01-02-150405")+".json")
	if err := snapshot.SaveSnapshot(snap, path); err != nil {
		return "", err
	}
	return path, nil
}

func defaultAuditLog(baseDir string, now time.Time) string {
	return filepath.Join(baseDir, ".envdoctor", "audit", now.Format("2006-01-02-150405")+".jsonl")
}

func writeAudit(file *os.File, record auditRecord) {
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	_, _ = file.Write(append(data, '\n'))
}

func summarize(report *Report) string {
	counts := map[string]int{}
	for _, result := range report.Results {
		counts[result.Status]++
	}
	return fmt.Sprintf("%s completed: %d actions (dry-run: %d, executed: %d, blocked: %d, skipped: %d, failed: %d, timeout: %d). Audit log: %s",
		report.Mode,
		len(report.Results),
		counts["dry-run"],
		counts["executed"],
		counts["blocked"],
		counts["skipped"],
		counts["failed"],
		counts["timeout"],
		report.AuditLog,
	)
}

func baseDirectory(dir string) (string, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = wd
	}
	return filepath.Abs(dir)
}

func nowOrDefault(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now()
	}
	return now
}

func preview(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= previewLimit {
		return value
	}
	return value[:previewLimit] + "...[truncated]"
}

func rollbackHint(category string) string {
	switch strings.ToLower(category) {
	case "dependencies":
		return "Review dependency lockfiles and restore project files from version control if needed."
	case "install":
		return "Use the platform package manager to uninstall the package if the install is not desired."
	case "version":
		return "Use the version manager to switch back to the previous runtime version."
	default:
		return "Use the pre-apply snapshot and version control to inspect and manually revert changes."
	}
}

func hasPrefix(args []string, prefix string) bool {
	return len(args) > 0 && strings.EqualFold(args[0], prefix)
}

func commandLine(name string, args []string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			builder.WriteRune(ch)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "action"
	}
	if len(result) > 48 {
		return result[:48]
	}
	return result
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
