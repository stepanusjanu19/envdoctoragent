package executor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/stepanusjanu19/envdoctoragent/internal/bootstrap"
	"github.com/stepanusjanu19/envdoctoragent/internal/fixplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/installplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/service"
	"github.com/stepanusjanu19/envdoctoragent/internal/snapshot"
	"github.com/stepanusjanu19/envdoctoragent/internal/version"
)

const (
	DefaultTimeout = 2 * time.Minute
	previewLimit   = 4096

	ProfileDevelopment = "development"
	ProfileProduction  = "production"

	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

// Action is the normalized execution contract used by approval-gated apply commands.
type Action struct {
	ID               string   `json:"id"`
	Type             string   `json:"type,omitempty"`
	Source           string   `json:"source"`
	Category         string   `json:"category"`
	Operation        string   `json:"operation,omitempty"`
	Ecosystem        string   `json:"ecosystem,omitempty"`
	PackageManager   string   `json:"package_manager,omitempty"`
	Packages         []string `json:"packages,omitempty"`
	Title            string   `json:"title"`
	Description      string   `json:"description,omitempty"`
	Command          string   `json:"command,omitempty"`
	Args             []string `json:"args,omitempty"`
	SuggestedCommand string   `json:"suggested_command,omitempty"`
	ManualSteps      string   `json:"manual_steps,omitempty"`
	Path             string   `json:"path,omitempty"`
	Content          string   `json:"-"`
	ContentBytes     int      `json:"content_bytes,omitempty"`
	Overwrite        bool     `json:"overwrite,omitempty"`
	WorkingDir       string   `json:"working_dir,omitempty"`
	Risk             string   `json:"risk"`
	RequiresAdmin    bool     `json:"requires_admin"`
	SafeToRun        bool     `json:"safe_to_run"`
	MutatesProject   bool     `json:"mutates_project,omitempty"`
	CreatesProject   bool     `json:"creates_project,omitempty"`
	Timeout          string   `json:"timeout"`
	RollbackHint     string   `json:"rollback_hint,omitempty"`
	Status           string   `json:"status"`
}

// Options configures one apply run.
type Options struct {
	DryRun     bool
	Approved   bool
	BaseDir    string
	AuditLog   string
	Timeout    time.Duration
	Now        time.Time
	Profile    string
	MaxRisk    string
	PolicyFile string
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
	Mode                string           `json:"mode"`
	Platform            string           `json:"platform"`
	Profile             string           `json:"profile"`
	MaxRisk             string           `json:"max_risk"`
	PolicyFile          string           `json:"policy_file,omitempty"`
	WorkingDir          string           `json:"working_dir"`
	AuditLog            string           `json:"audit_log"`
	SnapshotFile        string           `json:"snapshot_file,omitempty"`
	ProjectSnapshotFile string           `json:"project_snapshot_file,omitempty"`
	ProjectChanges      []string         `json:"project_changes,omitempty"`
	PolicyDecisions     []PolicyDecision `json:"policy_decisions,omitempty"`
	Results             []Result         `json:"results"`
	Summary             string           `json:"summary"`
	StartedAt           time.Time        `json:"started_at"`
	FinishedAt          time.Time        `json:"finished_at"`
}

// PolicyDecision records why an action is selected or blocked before execution.
type PolicyDecision struct {
	ActionID string `json:"action_id"`
	Profile  string `json:"profile"`
	MaxRisk  string `json:"max_risk"`
	Risk     string `json:"risk"`
	Mutating bool   `json:"mutating"`
	Allowed  bool   `json:"allowed"`
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

type auditRecord struct {
	Timestamp time.Time       `json:"timestamp"`
	Type      string          `json:"type"`
	Mode      string          `json:"mode"`
	Profile   string          `json:"profile,omitempty"`
	ActionID  string          `json:"action_id,omitempty"`
	Policy    *PolicyDecision `json:"policy,omitempty"`
	Result    *Result         `json:"result,omitempty"`
	Message   string          `json:"message,omitempty"`
}

type policyFileConfig struct {
	Profile                 string `json:"profile"`
	MaxRisk                 string `json:"max_risk"`
	AllowProductionMutation bool   `json:"allow_production_mutation"`
}

type resolvedPolicy struct {
	Profile                 string
	MaxRisk                 string
	PolicyFile              string
	AllowProductionMutation bool
}

type projectFileInfo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	ModTime string `json:"mod_time"`
}

type projectSnapshotManifest struct {
	Timestamp time.Time         `json:"timestamp"`
	Root      string            `json:"root"`
	Files     []projectFileInfo `json:"files"`
}

var projectFileNames = map[string]bool{
	".node-version": true, ".nvmrc": true, ".python-version": true, "Cargo.lock": true, "Cargo.toml": true,
	"Directory.Packages.props": true, "Gemfile": true, "Gemfile.lock": true, "Package.resolved": true,
	"Package.swift": true, "Pipfile": true, "Pipfile.lock": true, "Project.toml": true, "bun.lock": true,
	"bun.lockb": true, "build.gradle": true, "build.gradle.kts": true, "composer.json": true, "composer.lock": true,
	"conanfile.py": true, "conanfile.txt": true, "go.mod": true, "go.sum": true, "gradle.properties": true,
	"mix.exs": true, "mix.lock": true, "npm-shrinkwrap.json": true, "package-lock.json": true, "package.json": true,
	"packages.config": true, "pnpm-lock.yaml": true, "poetry.lock": true, "pom.xml": true, "pubspec.lock": true,
	"pubspec.yaml": true, "pubspec.yml": true, "requirements.txt": true, "runtime.txt": true, "rust-toolchain": true,
	"rust-toolchain.toml": true, "settings.gradle": true, "settings.gradle.kts": true, "vcpkg.json": true, "yarn.lock": true,
}

var projectIgnoredDirs = map[string]bool{
	".cache": true, ".envdoctor": true, ".git": true, ".venv": true, "build": true, "dist": true,
	"node_modules": true, "target": true, "vendor": true, "venv": true,
}

// Execute runs or previews normalized actions. Mutating execution requires Approved=true and DryRun=false.
func Execute(actions []Action, options Options) (*Report, error) {
	started := nowOrDefault(options.Now)
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	policy, err := resolvePolicy(options)
	if err != nil {
		return nil, err
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
		Profile:    policy.Profile,
		MaxRisk:    policy.MaxRisk,
		PolicyFile: policy.PolicyFile,
		WorkingDir: baseDir,
		AuditLog:   auditPath,
		Results:    []Result{},
		StartedAt:  started,
	}

	normalizedActions := make([]Action, 0, len(actions))
	decisions := make([]PolicyDecision, 0, len(actions))
	for i, action := range actions {
		normalized := normalizeAction(action, i, baseDir, timeout)
		decision := evaluatePolicy(normalized, mode, policy)
		normalizedActions = append(normalizedActions, normalized)
		decisions = append(decisions, decision)
		report.PolicyDecisions = append(report.PolicyDecisions, decision)
	}

	writeAudit(auditFile, auditRecord{Timestamp: started, Type: "session_start", Mode: mode, Profile: policy.Profile, Message: fmt.Sprintf("received %d actions", len(actions))})
	for _, decision := range decisions {
		decisionCopy := decision
		writeAudit(auditFile, auditRecord{Timestamp: time.Now(), Type: "policy_decision", Mode: mode, Profile: policy.Profile, ActionID: decision.ActionID, Policy: &decisionCopy})
	}

	if mode == "apply" && hasAllowedMutation(normalizedActions, decisions) {
		snapshotFile, err := createPreApplySnapshot(baseDir, started)
		if err != nil {
			return nil, err
		}
		report.SnapshotFile = snapshotFile
		writeAudit(auditFile, auditRecord{Timestamp: time.Now(), Type: "snapshot", Mode: mode, Profile: policy.Profile, Message: snapshotFile})
	}

	var projectBefore map[string]projectFileInfo
	if mode == "apply" && hasAllowedProjectMutation(normalizedActions, decisions) {
		projectBefore = collectProjectState(baseDir)
		projectSnapshotFile, err := saveProjectSnapshot(baseDir, started, projectBefore)
		if err != nil {
			return nil, err
		}
		report.ProjectSnapshotFile = projectSnapshotFile
		writeAudit(auditFile, auditRecord{Timestamp: time.Now(), Type: "project_snapshot", Mode: mode, Profile: policy.Profile, Message: projectSnapshotFile})
	}

	for i, normalized := range normalizedActions {
		result := policyBlockedResult(normalized, decisions[i])
		if decisions[i].Allowed {
			result = executeOne(normalized, mode, timeout)
		}
		report.Results = append(report.Results, result)
		writeAudit(auditFile, auditRecord{Timestamp: time.Now(), Type: "action_result", Mode: mode, Profile: policy.Profile, ActionID: result.Action.ID, Result: &result})
	}

	if mode == "apply" && hasAllowedProjectMutation(normalizedActions, decisions) {
		report.ProjectChanges = diffProjectStates(projectBefore, collectProjectState(baseDir))
	}

	report.FinishedAt = time.Now()
	report.Summary = summarize(report)
	writeAudit(auditFile, auditRecord{Timestamp: report.FinishedAt, Type: "session_finish", Mode: mode, Profile: policy.Profile, Message: report.Summary})
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
			ID: action.ID, Source: valueOrDefault(action.Source, "fixplan"), Category: action.Category,
			Operation: action.Operation, Ecosystem: action.Ecosystem, PackageManager: action.PackageManager, Packages: action.Packages,
			Title: action.Title, Description: action.Description, SuggestedCommand: action.Command, ManualSteps: action.ManualSteps,
			WorkingDir: valueOrDefault(action.WorkingDir, baseDir), Risk: valueOrDefault(action.Risk, "medium"),
			RequiresAdmin: action.RequiresAdmin, SafeToRun: action.SafeToRun, MutatesProject: action.MutatesProject,
			CreatesProject: action.CreatesProject, Timeout: action.Timeout, RollbackHint: action.RollbackHint, Status: action.Status,
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
		ID: action.ID, Source: valueOrDefault(action.Source, "installplan"), Category: "Install",
		Operation: action.Operation, Ecosystem: action.Ecosystem, PackageManager: action.PackageManager, Packages: action.Packages,
		Title: fmt.Sprintf("Install %s", action.Tool), Command: action.Command, Args: action.Args, ManualSteps: action.ManualSteps,
		WorkingDir: valueOrDefault(action.WorkingDir, baseDir), Risk: valueOrDefault(action.Risk, "medium"),
		RequiresAdmin: action.RequiresAdmin, SafeToRun: action.SafeToRun, MutatesProject: action.MutatesProject,
		CreatesProject: action.CreatesProject, Timeout: action.Timeout, RollbackHint: action.RollbackHint, Status: action.Status,
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
			ID: action.ID, Source: valueOrDefault(action.Source, "version"), Category: "Version",
			Operation: action.Operation, Ecosystem: action.Ecosystem, PackageManager: action.PackageManager, Packages: action.Packages,
			Title: action.Title, SuggestedCommand: action.Command, ManualSteps: action.ManualSteps,
			WorkingDir: valueOrDefault(action.WorkingDir, baseDir), Risk: valueOrDefault(action.Risk, "medium"),
			RequiresAdmin: action.RequiresAdmin, SafeToRun: action.SafeToRun, MutatesProject: action.MutatesProject,
			CreatesProject: action.CreatesProject, Timeout: action.Timeout, RollbackHint: action.RollbackHint, Status: "plan-only",
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
			ID: action.ID, Source: valueOrDefault(action.Source, "bootstrap"), Category: action.Category,
			Operation: action.Operation, Ecosystem: action.Ecosystem, PackageManager: action.PackageManager, Packages: action.Packages,
			Title: action.Title, SuggestedCommand: action.Command, ManualSteps: action.ManualSteps,
			WorkingDir: valueOrDefault(action.WorkingDir, baseDir), Risk: valueOrDefault(action.Risk, "medium"),
			RequiresAdmin: action.RequiresAdmin, SafeToRun: action.SafeToRun, MutatesProject: action.MutatesProject,
			CreatesProject: action.CreatesProject, Timeout: action.Timeout, RollbackHint: action.RollbackHint, Status: action.Status,
		})
	}
	for _, action := range plan.InstallPlans {
		actions = append(actions, Action{
			ID: action.ID, Source: valueOrDefault(action.Source, "bootstrap-install"), Category: "Install",
			Operation: action.Operation, Ecosystem: action.Ecosystem, PackageManager: action.PackageManager, Packages: action.Packages,
			Title: fmt.Sprintf("Install %s", action.Tool), SuggestedCommand: action.Command, ManualSteps: action.ManualSteps,
			WorkingDir: valueOrDefault(action.WorkingDir, baseDir), Risk: valueOrDefault(action.Risk, "medium"),
			RequiresAdmin: action.RequiresAdmin, SafeToRun: action.SafeToRun, MutatesProject: action.MutatesProject,
			CreatesProject: action.CreatesProject, Timeout: action.Timeout, RollbackHint: action.RollbackHint, Status: action.Status,
		})
	}
	return actions
}

// FromServicePlan converts service operation plans into executor actions.
func FromServicePlan(plan *service.PlanReport, baseDir string) []Action {
	if plan == nil {
		return nil
	}
	actions := make([]Action, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		actions = append(actions, Action{
			ID: action.ID, Source: valueOrDefault(action.Source, "service"), Category: action.Category,
			Operation: action.Operation, Title: action.Title, Description: action.Description,
			Command: action.Command, Args: action.Args, ManualSteps: action.ManualSteps,
			WorkingDir: baseDir, Risk: valueOrDefault(action.Risk, "high"), RequiresAdmin: action.RequiresAdmin,
			SafeToRun: action.SafeToRun, Timeout: action.Timeout, RollbackHint: action.RollbackHint,
			Status: action.Status,
		})
	}
	return actions
}

func executeOne(action Action, mode string, timeout time.Duration) Result {
	switch action.Type {
	case "manual":
		action.Status = "skipped"
		return Result{Action: action, Status: "skipped", Message: "manual-only action; no command to execute"}
	case "mkdir", "write_file":
		return executeFileAction(action, mode)
	}

	if action.SuggestedCommand == "" && action.Command == "" {
		action.Status = "skipped"
		return Result{Action: action, Status: "skipped", Message: "manual-only action; no command to execute"}
	}
	if action.Command == "" {
		commandName, args, err := parseSuggestedCommand(action.SuggestedCommand)
		if err != nil {
			action.Status = "blocked"
			return Result{Action: action, Status: "blocked", Error: err.Error()}
		}
		action.Command = commandName
		action.Args = args
	}
	if err := validateAction(action); err != nil {
		action.Status = "blocked"
		return Result{Action: action, Status: "blocked", Error: err.Error()}
	}
	if mode == "dry-run" {
		action.Status = "dry-run"
		return Result{Action: action, Status: "dry-run", Message: "action validated but not executed; pass --yes to apply"}
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
	result := Result{Action: action, DurationMS: time.Since(started).Milliseconds(), StdoutPreview: preview(stdout.String()), StderrPreview: preview(stderr.String())}
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
	if action.Type == "" {
		switch {
		case action.Path != "":
			action.Type = "write_file"
		case action.Command != "" || action.SuggestedCommand != "":
			action.Type = "command"
		default:
			action.Type = "manual"
		}
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
	action.Packages = dedupe(action.Packages)
	if action.WorkingDir == "" {
		action.WorkingDir = baseDir
	} else if absDir, err := filepath.Abs(action.WorkingDir); err == nil {
		action.WorkingDir = absDir
	}
	if action.Timeout == "" {
		action.Timeout = timeout.String()
	}
	if action.Content != "" && action.ContentBytes == 0 {
		action.ContentBytes = len([]byte(action.Content))
	}
	if action.RollbackHint == "" && (action.Command != "" || action.SuggestedCommand != "") {
		action.RollbackHint = rollbackHint(action.Category)
	}
	if action.Status == "" {
		action.Status = "pending"
	}
	return action
}

func executeFileAction(action Action, mode string) Result {
	if err := validateFileAction(action); err != nil {
		action.Status = "blocked"
		return Result{Action: action, Status: "blocked", Error: err.Error()}
	}
	if mode == "dry-run" {
		action.Status = "dry-run"
		return Result{Action: action, Status: "dry-run", Message: "file action validated but not executed; pass --yes to apply"}
	}

	started := time.Now()
	target, err := safeProjectPath(action.WorkingDir, action.Path)
	if err != nil {
		action.Status = "blocked"
		return Result{Action: action, Status: "blocked", Error: err.Error()}
	}
	switch action.Type {
	case "mkdir":
		err = os.MkdirAll(target, 0755)
	case "write_file":
		if err = os.MkdirAll(filepath.Dir(target), 0755); err == nil {
			flags := os.O_CREATE | os.O_WRONLY
			if action.Overwrite {
				flags |= os.O_TRUNC
			} else {
				flags |= os.O_EXCL
			}
			var file *os.File
			file, err = os.OpenFile(target, flags, 0644)
			if err == nil {
				_, err = file.WriteString(action.Content)
				closeErr := file.Close()
				if err == nil {
					err = closeErr
				}
			}
		}
	default:
		err = fmt.Errorf("unsupported file action type %q", action.Type)
	}

	result := Result{Action: action, DurationMS: time.Since(started).Milliseconds()}
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		result.Action.Status = result.Status
		return result
	}
	result.Status = "executed"
	result.Action.Status = result.Status
	if action.Type == "mkdir" {
		result.Message = "directory created"
	} else {
		result.Message = "file written"
	}
	return result
}

func validateFileAction(action Action) error {
	if action.WorkingDir == "" {
		return fmt.Errorf("missing working directory for file action")
	}
	if action.Path == "" {
		return fmt.Errorf("missing project-relative path for file action")
	}
	if _, err := safeProjectPath(action.WorkingDir, action.Path); err != nil {
		return err
	}
	switch action.Type {
	case "mkdir":
		return validateMkdirTarget(action.WorkingDir, action.Path)
	case "write_file":
		return validateWriteTarget(action.WorkingDir, action.Path, action.Overwrite)
	default:
		return fmt.Errorf("unsupported file action type %q", action.Type)
	}
}

func validateMkdirTarget(root, rel string) error {
	target, err := safeProjectPath(root, rel)
	if err != nil {
		return err
	}
	if err := ensureNoSymlinkEscape(root, rel, true); err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err == nil && !info.IsDir() {
		return fmt.Errorf("mkdir target exists and is not a directory: %s", rel)
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func validateWriteTarget(root, rel string, overwrite bool) error {
	if strings.TrimSpace(filepath.Clean(filepath.FromSlash(rel))) == "." {
		return fmt.Errorf("write_file target must be a file path")
	}
	target, err := safeProjectPath(root, rel)
	if err != nil {
		return err
	}
	if err := ensureNoSymlinkEscape(root, rel, true); err != nil {
		return err
	}
	info, err := os.Lstat(target)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("write_file target is a symlink: %s", rel)
		}
		if info.IsDir() {
			return fmt.Errorf("write_file target is a directory: %s", rel)
		}
		if !overwrite {
			return fmt.Errorf("write_file target exists; pass --force to overwrite: %s", rel)
		}
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func safeProjectPath(root, rel string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if info, err := os.Lstat(rootAbs); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("project directory symlink is blocked for file actions: %s", rootAbs)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	clean, err := cleanProjectRelativePath(rel)
	if err != nil {
		return "", err
	}
	target := filepath.Join(rootAbs, clean)
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("file action path escapes project directory: %s", rel)
	}
	return targetAbs, nil
}

func cleanProjectRelativePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty project-relative path")
	}
	if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", fmt.Errorf("absolute paths are blocked for file actions: %s", path)
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal is blocked for file actions: %s", path)
	}
	for _, part := range strings.Split(filepath.ToSlash(clean), "/") {
		if part == ".." {
			return "", fmt.Errorf("path traversal is blocked for file actions: %s", path)
		}
	}
	return clean, nil
}

func ensureNoSymlinkEscape(root, rel string, includeTarget bool) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	clean, err := cleanProjectRelativePath(rel)
	if err != nil {
		return err
	}
	current := rootAbs
	parts := strings.Split(filepath.ToSlash(clean), "/")
	limit := len(parts)
	if !includeTarget && limit > 0 {
		limit--
	}
	for i := 0; i < limit; i++ {
		part := parts[i]
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink path component is blocked for file actions: %s", part)
		}
	}
	return nil
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
	if isDeleteCommand(action.Command) {
		return fmt.Errorf("delete commands are blocked by envdoctor: %s", action.Command)
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
	case "apt-get":
		return hasPrefix(args, "install")
	case "dnf", "yum", "zypper":
		return hasPrefix(args, "install")
	case "pacman":
		return hasPrefix(args, "-S")
	case "brew":
		return hasPrefix(args, "install")
	case "winget":
		return hasPrefix(args, "install")
	case "choco":
		return hasPrefix(args, "install")
	case "npm", "yarn", "pnpm", "bun":
		return allowNodePackageCommand(name, args)
	case "npx":
		return len(args) > 0 && (strings.HasPrefix(args[0], "create-") || strings.Contains(args[0], "@nestjs/cli"))
	case "python", "python3":
		return allowPythonCommand(args)
	case "go":
		return allowGoCommand(args)
	case "cargo":
		return hasAnyPrefix(args, "fetch", "init", "add", "update", "remove")
	case "composer":
		return hasAnyPrefix(args, "install", "init", "require", "update", "remove")
	case "mvn":
		return hasAnyPrefix(args, "dependency:resolve", "archetype:generate")
	case "gradle":
		return hasAnyPrefix(args, "dependencies", "init")
	case "dotnet":
		return allowDotnetCommand(args)
	case "bundle":
		return hasAnyPrefix(args, "install", "add", "update", "remove")
	case "dart":
		return allowDartCommand(args)
	case "flutter":
		return hasPrefix(args, "create")
	case "swift":
		return allowSwiftCommand(args)
	case "mix":
		return hasAnyPrefix(args, "deps.get", "new")
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
	case "systemctl":
		return len(args) == 2 && hasAnyPrefix(args, "start", "stop", "restart")
	case "sc.exe", "sc":
		return len(args) == 2 && hasAnyPrefix(args, "start", "stop")
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
	return fmt.Sprintf("%s completed with profile %s: %d actions (dry-run: %d, executed: %d, blocked: %d, skipped: %d, failed: %d, timeout: %d). Audit log: %s",
		report.Mode, report.Profile, len(report.Results), counts["dry-run"], counts["executed"], counts["blocked"], counts["skipped"], counts["failed"], counts["timeout"], report.AuditLog)
}

func resolvePolicy(options Options) (resolvedPolicy, error) {
	policy := resolvedPolicy{
		Profile: valueOrDefault(options.Profile, ProfileDevelopment),
		MaxRisk: valueOrDefault(options.MaxRisk, RiskHigh),
	}
	if strings.TrimSpace(options.PolicyFile) != "" {
		abs, err := filepath.Abs(options.PolicyFile)
		if err != nil {
			return resolvedPolicy{}, err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return resolvedPolicy{}, err
		}
		var config policyFileConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return resolvedPolicy{}, fmt.Errorf("invalid policy file %s: %w", abs, err)
		}
		policy.PolicyFile = abs
		policy.AllowProductionMutation = config.AllowProductionMutation
		if options.Profile == "" && config.Profile != "" {
			policy.Profile = config.Profile
		}
		if options.MaxRisk == "" && config.MaxRisk != "" {
			policy.MaxRisk = config.MaxRisk
		}
	}
	var err error
	policy.Profile, err = normalizeProfile(policy.Profile)
	if err != nil {
		return resolvedPolicy{}, err
	}
	policy.MaxRisk, err = normalizeRisk(policy.MaxRisk)
	if err != nil {
		return resolvedPolicy{}, err
	}
	return policy, nil
}

func evaluatePolicy(action Action, mode string, policy resolvedPolicy) PolicyDecision {
	decision := PolicyDecision{
		ActionID: action.ID,
		Profile:  policy.Profile,
		MaxRisk:  policy.MaxRisk,
		Risk:     normalizeRiskOrDefault(action.Risk),
		Mutating: actionMutates(action),
		Allowed:  true,
		Decision: "allowed",
	}
	if riskRank(decision.Risk) > riskRank(policy.MaxRisk) {
		decision.Allowed = false
		decision.Decision = "blocked"
		decision.Reason = fmt.Sprintf("action risk %s exceeds --max-risk %s", decision.Risk, policy.MaxRisk)
		return decision
	}
	if mode == "apply" && policy.Profile == ProfileProduction && decision.Mutating && !policy.AllowProductionMutation {
		decision.Allowed = false
		decision.Decision = "blocked"
		decision.Reason = "production profile blocks mutating apply actions"
		return decision
	}
	if mode == "apply" && action.RequiresAdmin && policy.Profile == ProfileProduction {
		decision.Allowed = false
		decision.Decision = "blocked"
		decision.Reason = "production profile blocks admin-required apply actions"
		return decision
	}
	return decision
}

func policyBlockedResult(action Action, decision PolicyDecision) Result {
	action.Status = "blocked"
	return Result{
		Action:  action,
		Status:  "blocked",
		Error:   decision.Reason,
		Message: "blocked by envdoctor policy profile",
	}
}

func hasAllowedMutation(actions []Action, decisions []PolicyDecision) bool {
	for i, action := range actions {
		if i < len(decisions) && decisions[i].Allowed && actionMutates(action) {
			return true
		}
	}
	return false
}

func hasAllowedProjectMutation(actions []Action, decisions []PolicyDecision) bool {
	for i, action := range actions {
		if i < len(decisions) && decisions[i].Allowed && (action.MutatesProject || action.CreatesProject || action.Type == "mkdir" || action.Type == "write_file") {
			return true
		}
	}
	return false
}

func actionMutates(action Action) bool {
	if action.Type == "manual" {
		return false
	}
	if action.MutatesProject || action.CreatesProject || action.Type == "mkdir" || action.Type == "write_file" {
		return true
	}
	return action.Command != "" || action.SuggestedCommand != ""
}

func normalizeProfile(profile string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", ProfileDevelopment, "dev":
		return ProfileDevelopment, nil
	case ProfileProduction, "prod":
		return ProfileProduction, nil
	default:
		return "", fmt.Errorf("unsupported profile %q; use development or production", profile)
	}
}

func normalizeRisk(risk string) (string, error) {
	switch normalizeRiskOrDefault(risk) {
	case RiskLow:
		return RiskLow, nil
	case RiskMedium:
		return RiskMedium, nil
	case RiskHigh:
		return RiskHigh, nil
	default:
		return "", fmt.Errorf("unsupported risk %q; use low, medium, or high", risk)
	}
}

func normalizeRiskOrDefault(risk string) string {
	risk = strings.ToLower(strings.TrimSpace(risk))
	switch risk {
	case RiskLow, RiskMedium, RiskHigh:
		return risk
	default:
		return RiskMedium
	}
}

func riskRank(risk string) int {
	switch normalizeRiskOrDefault(risk) {
	case RiskLow:
		return 1
	case RiskMedium:
		return 2
	case RiskHigh:
		return 3
	default:
		return 2
	}
}

func hasProjectMutation(actions []Action) bool {
	for _, action := range actions {
		if action.MutatesProject || action.CreatesProject {
			return true
		}
	}
	return false
}

func collectProjectState(root string) map[string]projectFileInfo {
	state := map[string]projectFileInfo{}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && projectIgnoredDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !isProjectSnapshotFile(path) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		info, err := projectFile(path, filepath.ToSlash(rel))
		if err == nil {
			state[info.Path] = info
		}
		return nil
	})
	return state
}

func saveProjectSnapshot(root string, now time.Time, state map[string]projectFileInfo) (string, error) {
	dir := filepath.Join(root, ".envdoctor", "project-snapshots", now.Format("2006-01-02-150405"))
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return "", err
	}
	files := make([]projectFileInfo, 0, len(state))
	for _, info := range state {
		files = append(files, info)
		if err := copyFile(filepath.Join(root, filepath.FromSlash(info.Path)), filepath.Join(filesDir, filepath.FromSlash(info.Path))); err != nil {
			return "", err
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	data, err := json.MarshalIndent(projectSnapshotManifest{Timestamp: now, Root: root, Files: files}, "", "  ")
	if err != nil {
		return "", err
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return "", err
	}
	return manifestPath, nil
}

func diffProjectStates(before, after map[string]projectFileInfo) []string {
	var changes []string
	for path, beforeInfo := range before {
		afterInfo, ok := after[path]
		if !ok {
			changes = append(changes, "removed: "+path)
			continue
		}
		if beforeInfo.SHA256 != afterInfo.SHA256 {
			changes = append(changes, "changed: "+path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			changes = append(changes, "added: "+path)
		}
	}
	sort.Strings(changes)
	return changes
}

func projectFile(path, rel string) (projectFileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return projectFileInfo{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return projectFileInfo{}, err
	}
	stat, err := file.Stat()
	if err != nil {
		return projectFileInfo{}, err
	}
	return projectFileInfo{Path: rel, Size: stat.Size(), SHA256: hex.EncodeToString(hash.Sum(nil)), ModTime: stat.ModTime().UTC().Format(time.RFC3339)}, nil
}

func isProjectSnapshotFile(path string) bool {
	base := filepath.Base(path)
	return projectFileNames[base] || strings.HasSuffix(base, ".csproj") || strings.HasSuffix(base, ".fsproj") || strings.HasSuffix(base, ".vbproj") || strings.HasSuffix(base, ".rockspec") || strings.HasSuffix(base, ".cabal")
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
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

func hasAnyPrefix(args []string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if hasPrefix(args, prefix) {
			return true
		}
	}
	return false
}

func allowNodePackageCommand(name string, args []string) bool {
	switch name {
	case "npm":
		return hasAnyPrefix(args, "install", "init", "update", "uninstall", "create")
	case "yarn":
		return hasAnyPrefix(args, "install", "add", "upgrade", "remove", "create")
	case "pnpm":
		return hasAnyPrefix(args, "install", "add", "update", "remove", "create")
	case "bun":
		return hasAnyPrefix(args, "install", "add", "update", "remove", "create")
	default:
		return false
	}
}

func allowPythonCommand(args []string) bool {
	if len(args) < 2 || args[0] != "-m" {
		return false
	}
	switch args[1] {
	case "pip":
		return len(args) >= 3 && hasAnyPrefix(args[2:], "install", "uninstall")
	case "venv":
		return len(args) >= 3
	default:
		return false
	}
}

func allowGoCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}
	if args[0] == "mod" {
		return hasAnyPrefix(args[1:], "download", "init", "tidy")
	}
	return hasPrefix(args, "get")
}

func allowDotnetCommand(args []string) bool {
	if hasPrefix(args, "restore") || hasPrefix(args, "new") {
		return true
	}
	if len(args) >= 3 && args[0] == "add" && args[1] == "package" {
		return true
	}
	if len(args) >= 3 && args[0] == "remove" && args[1] == "package" {
		return true
	}
	return false
}

func allowDartCommand(args []string) bool {
	if hasPrefix(args, "create") {
		return true
	}
	return len(args) >= 2 && args[0] == "pub" && hasAnyPrefix(args[1:], "get", "add", "upgrade", "remove")
}

func allowSwiftCommand(args []string) bool {
	return len(args) >= 2 && args[0] == "package" && hasAnyPrefix(args[1:], "init", "resolve", "update")
}

func isDeleteCommand(commandName string) bool {
	switch strings.ToLower(filepath.Base(commandName)) {
	case "rm", "del", "erase", "remove-item", "rmdir", "rd":
		return true
	default:
		return false
	}
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

func dedupe(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
