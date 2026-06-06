package cliui

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/stepanusjanu19/envdoctoragent/internal/agent"
	"github.com/stepanusjanu19/envdoctoragent/internal/analyzer"
	"github.com/stepanusjanu19/envdoctoragent/internal/bootstrap"
	"github.com/stepanusjanu19/envdoctoragent/internal/diagnose"
	"github.com/stepanusjanu19/envdoctoragent/internal/executor"
	"github.com/stepanusjanu19/envdoctoragent/internal/fixplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/installplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/projectops"
	"github.com/stepanusjanu19/envdoctoragent/internal/service"
	"github.com/stepanusjanu19/envdoctoragent/internal/snapshot"
	"github.com/stepanusjanu19/envdoctoragent/internal/terminalui"
	"github.com/stepanusjanu19/envdoctoragent/internal/version"
)

// Options configures the interactive CLI UI.
type Options struct {
	In      io.Reader
	Out     io.Writer
	Script  string
	Version string
	Plain   bool
}

// Run starts the CLI UI. It only runs read-only or plan-only workflows.
func Run(options Options) error {
	in := options.In
	if in == nil {
		in = strings.NewReader("")
	}
	out := options.Out
	if out == nil {
		out = io.Discard
	}

	ui := &session{
		in:        bufio.NewReader(in),
		out:       out,
		presenter: terminalui.New(out, terminalui.Options{Version: options.Version, Plain: options.Plain}),
	}

	if strings.TrimSpace(options.Script) != "" {
		return ui.runScript(options.Script)
	}
	return ui.runInteractive()
}

type session struct {
	in        *bufio.Reader
	out       io.Writer
	presenter *terminalui.Presenter
}

func (s *session) runInteractive() error {
	for {
		s.printMenu()
		choice, err := s.prompt("Select")
		if err != nil {
			return err
		}
		switch strings.TrimSpace(choice) {
		case "1":
			s.runDiagnose()
		case "2":
			s.runService()
		case "3":
			s.runVersion()
		case "4":
			s.runInstall()
		case "5":
			s.runFix()
		case "6":
			s.runBootstrap()
		case "7":
			s.runSnapshot()
		case "8":
			s.runLogs()
		case "9":
			s.runProjectScan()
		case "a", "agent":
			s.runAgentPlan()
		case "b", "about":
			s.printAbout()
		case "0", "q", "quit", "exit":
			fmt.Fprintln(s.out, "Exiting envdoctor UI.")
			return nil
		default:
			fmt.Fprintln(s.out, "Unknown selection.")
		}
		fmt.Fprintln(s.out)
	}
}

func (s *session) runScript(script string) error {
	s.printDashboard()
	s.printMenuItems()
	fmt.Fprintln(s.out, "Script mode: read-only and plan-only actions")
	for _, action := range strings.Split(script, ",") {
		action = strings.TrimSpace(action)
		if action == "" {
			continue
		}
		name, arg, _ := strings.Cut(action, ":")
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "diagnose":
			s.runDiagnose()
		case "service":
			s.runServiceList()
		case "version":
			s.runVersionFor(defaultValue(arg, "."))
		case "install":
			s.runInstallFor(defaultValue(arg, "python"))
		case "fix":
			s.runFixFor(defaultValue(arg, "."))
		case "bootstrap":
			s.runBootstrapFor(defaultValue(arg, "."))
		case "snapshot":
			s.runSnapshot()
		case "logs":
			if strings.TrimSpace(arg) == "" {
				fmt.Fprintln(s.out, "Logs: skipped, no log file provided.")
			} else {
				s.runLogsFor(arg)
			}
		case "project":
			s.runProjectScanFor(defaultValue(arg, "."))
		case "agent":
			s.runAgentPlanFor(defaultValue(arg, "."))
		case "about":
			s.printAbout()
		case "exit", "quit":
			fmt.Fprintln(s.out, "Exiting envdoctor UI.")
			return nil
		default:
			fmt.Fprintf(s.out, "Unknown UI script action: %s\n", action)
		}
	}
	return nil
}

func (s *session) printMenu() {
	s.printDashboard()
	s.printMenuItems()
}

func (s *session) printDashboard() {
	s.presenter.Header("Envdoctor Dashboard", "guided local environment review")
	s.presenter.Section("About")
	s.presenter.Row("Mode", "read-only and plan-only workflows")
	s.presenter.Row("Safety", "no install, restart, runtime switch, or autonomous repair")
	s.presenter.Row("UI style", "friendly ANSI layout with deterministic stage progress")
	s.presenter.Section("Coverage")
	s.presenter.StatusRow("Environment", "implemented", "system, toolchain, PATH, containers")
	s.presenter.StatusRow("Dependency Coverage", "metadata-ready", "multi-ecosystem manifest registry")
	s.presenter.StatusRow("Service", "read-only", "native manager inspection")
	s.presenter.StatusRow("Version", "plan-only", "runtime requirement scan and switch suggestions")
	s.presenter.StatusRow("Fix Plan", "plan-only", "safe action list")
	s.presenter.StatusRow("Bootstrap Plan", "plan-only", "project setup suggestions")
	s.presenter.StatusRow("Project", "safe apply preview", "official starters and dependency lifecycle")
	s.presenter.StatusRow("Agent", "plan-only", "local deterministic orchestration")
	s.presenter.StatusRow("Logs", "implemented", "rule-based explanation")
}

func (s *session) printMenuItems() {
	s.presenter.Section("Menu")
	fmt.Fprintln(s.out, "  > 1  Diagnose")
	fmt.Fprintln(s.out, "    2  Service")
	fmt.Fprintln(s.out, "    3  Version")
	fmt.Fprintln(s.out, "    4  Install Plan")
	fmt.Fprintln(s.out, "    5  Fix Plan")
	fmt.Fprintln(s.out, "    6  Bootstrap Plan")
	fmt.Fprintln(s.out, "    7  Snapshot")
	fmt.Fprintln(s.out, "    8  Logs")
	fmt.Fprintln(s.out, "    9  Project Scan")
	fmt.Fprintln(s.out, "    A  Agent Plan")
	fmt.Fprintln(s.out, "    B  About")
	fmt.Fprintln(s.out, "    0  Exit")
}

func (s *session) summaryRow(label, status, detail string) {
	s.presenter.StatusRow(label, status, detail)
}

func (s *session) prompt(label string) (string, error) {
	fmt.Fprintf(s.out, "%s: ", label)
	value, err := s.in.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (s *session) runDiagnose() {
	s.section("Diagnose")
	s.progress("Prepare diagnosis", "Collect system/toolchain/PATH/container data", "Summarize recommendations")
	report, err := diagnose.Run()
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.presenter.StatusRow("Status", "implemented", "local environment summary")
	s.presenter.Row("Toolchain entries", fmt.Sprint(len(report.Toolchain)))
	if report.PathReport != nil {
		s.presenter.Row("PATH issues", fmt.Sprint(report.PathReport.IssueCount))
	}
	s.presenter.Row("Recommendations", fmt.Sprint(len(report.Recommendations)))
	s.presenter.NextSteps("Review recommendations before using any apply command.", "Run envdoctor diagnose --json for automation-safe output.")
}

func (s *session) runService() {
	name, err := s.prompt("Service name (blank to list)")
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	if name == "" {
		s.runServiceList()
		return
	}
	s.section("Service Status")
	s.progress("Select service manager", "Read service status", "Prepare guidance")
	info, err := service.Status(name)
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.presenter.StatusRow("Status", "read-only", "native service status")
	s.presenter.Row("Manager", info.Manager)
	s.presenter.Row("Service status", info.Status)
	s.presenter.Row("State", valueOrDash(info.State))
	s.presenter.NextSteps("Use envdoctor service plan for structured start/stop/restart previews.")
}

func (s *session) runServiceList() {
	s.section("Service List")
	s.progress("Detect service manager", "List known services", "Summarize availability")
	report, err := service.ListServices()
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.presenter.StatusRow("Status", "read-only", "native manager inspection")
	s.presenter.Row("Manager", report.Manager)
	s.presenter.Row("Manager status", report.Status)
	s.presenter.Row("Services", fmt.Sprint(len(report.Services)))
	s.presenter.NextSteps("Run envdoctor service status <name> for a single service.")
}

func (s *session) runVersion() {
	dir, err := s.prompt("Directory (default .)")
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.runVersionFor(defaultValue(dir, "."))
}

func (s *session) runVersionFor(dir string) {
	s.section("Version Scan")
	s.progress("Detect managers", "Read project requirements", "Compare active runtimes")
	report, err := version.Scan(dir)
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.presenter.StatusRow("Status", "read-only", "runtime and version-manager scan")
	s.presenter.Row("Managers", fmt.Sprint(len(report.Managers)))
	s.presenter.Row("Runtimes", fmt.Sprint(len(report.Runtimes)))
	s.presenter.Row("Requirements", fmt.Sprint(len(report.Requirements)))
	s.presenter.Row("Items needing review", fmt.Sprint(len(report.Mismatches)))
	s.presenter.NextSteps("Run envdoctor version plan for switch/install suggestions.")
}

func (s *session) runInstall() {
	tool, err := s.prompt("Tool (default python)")
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.runInstallFor(defaultValue(tool, "python"))
}

func (s *session) runInstallFor(tool string) {
	s.section("Install Plan")
	s.progress("Detect platform", "Resolve package manager", "Build advisory command")
	plan := installplan.Generate(tool)
	s.presenter.StatusRow("Status", "plan-only", "advisory only")
	s.presenter.Row("Tool", plan.Action.Tool)
	s.presenter.Row("Manager", valueOrDash(plan.Action.Manager))
	if plan.Action.Command != "" {
		s.presenter.Detail("Suggested command", plan.Action.Command)
	}
	s.presenter.NextSteps("Use install apply --dry-run to inspect executor policy before any mutation.")
}

func (s *session) runFix() {
	dir, err := s.prompt("Directory (default .)")
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.runFixFor(defaultValue(dir, "."))
}

func (s *session) runFixFor(dir string) {
	s.section("Fix Plan")
	s.progress("Collect recommendations", "Scan project signals", "Build safe action list")
	report, err := fixplan.Generate(dir)
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.presenter.StatusRow("Status", "plan-only", "safe action candidates")
	s.presenter.Row("Actions", fmt.Sprint(len(report.Actions)))
	s.presenter.NextSteps("Run envdoctor fix apply --dry-run before considering --yes.")
}

func (s *session) runBootstrap() {
	dir, err := s.prompt("Directory (default .)")
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.runBootstrapFor(defaultValue(dir, "."))
}

func (s *session) runBootstrapFor(dir string) {
	s.section("Bootstrap Plan")
	s.progress("Read project metadata", "Infer runtime and service needs", "Build setup plan")
	plan, err := bootstrap.GeneratePlan(dir)
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.presenter.StatusRow("Status", "plan-only", "project onboarding guidance")
	s.presenter.Row("Actions", fmt.Sprint(len(plan.Actions)))
	s.presenter.Row("Service hints", fmt.Sprint(len(plan.ServiceHints)))
	s.presenter.NextSteps("Run envdoctor bootstrap apply --dry-run to inspect allowlisted setup actions.")
}

func (s *session) runSnapshot() {
	s.section("Snapshot")
	s.progress("Capture toolchain", "Analyze PATH", "Build snapshot summary")
	snap, err := snapshot.CreateSnapshot()
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.presenter.StatusRow("Status", "read-only", "environment snapshot preview")
	s.presenter.Row("Tools", fmt.Sprint(len(snap.Tools)))
	if snap.Path != nil {
		s.presenter.Row("PATH issues", fmt.Sprint(snap.Path.IssueCount))
	}
	s.presenter.NextSteps("Use envdoctor snapshot --save when you want a comparison baseline.")
}

func (s *session) runLogs() {
	path, err := s.prompt("Log file")
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	if path == "" {
		fmt.Fprintln(s.out, "Logs: skipped, no log file provided.")
		return
	}
	s.runLogsFor(path)
}

func (s *session) runLogsFor(path string) {
	s.section("Logs")
	s.progress("Read log file", "Match known patterns", "Summarize likely causes")
	result, err := analyzer.AnalyzeLog(path)
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.presenter.StatusRow("Status", "implemented", "rule-based explanation")
	s.presenter.Row("Issues", fmt.Sprint(len(result.Issues)))
	s.presenter.NextSteps("Run envdoctor explain --json <logfile> for structured log findings.")
}

func (s *session) runProjectScan() {
	dir, err := s.prompt("Directory (default .)")
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.runProjectScanFor(defaultValue(dir, "."))
}

func (s *session) runProjectScanFor(dir string) {
	s.section("Project Scan")
	s.progress("Search manifests", "Identify ecosystems", "Summarize dependency managers")
	report, err := projectops.Scan(dir)
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.presenter.StatusRow("Status", report.Status, "project lifecycle metadata")
	s.presenter.Row("Ecosystems", strings.Join(report.Ecosystems, ", "))
	s.presenter.Row("Package managers", strings.Join(report.Managers, ", "))
	s.presenter.Row("Manifests", fmt.Sprint(len(report.Manifests)))
	s.presenter.NextSteps("Run envdoctor project deps plan sync --json for machine-readable dependency actions.")
}

func (s *session) runAgentPlan() {
	dir, err := s.prompt("Directory (default .)")
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.runAgentPlanFor(defaultValue(dir, "."))
}

func (s *session) runAgentPlanFor(dir string) {
	s.section("Agent Plan")
	s.progress("Set goal", "Run local orchestrator", "Collect action candidates")
	report, err := agent.Plan(agent.Options{
		Directory: dir,
		Goal:      agent.GoalDiagnose,
		Profile:   executor.ProfileDevelopment,
		MaxRisk:   executor.RiskHigh,
	})
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	s.presenter.StatusRow("Status", report.Status, "deterministic local agent")
	s.presenter.Row("Goal", report.Goal)
	s.presenter.Row("Actions", fmt.Sprint(len(report.Actions)))
	s.presenter.NextSteps("Use envdoctor agent plan --goal onboard or --goal repair for broader orchestration.")
}

func (s *session) printAbout() {
	s.presenter.About()
}

func defaultValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func valueOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func (s *session) section(title string) {
	s.presenter.Section(title)
}

func (s *session) progress(stages ...string) {
	total := len(stages)
	for i, stage := range stages {
		s.presenter.Progress(i+1, total, stage)
	}
}
