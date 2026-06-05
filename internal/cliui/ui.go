package cliui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/stepanusjanu19/envdoctoragent/internal/analyzer"
	"github.com/stepanusjanu19/envdoctoragent/internal/bootstrap"
	"github.com/stepanusjanu19/envdoctoragent/internal/diagnose"
	"github.com/stepanusjanu19/envdoctoragent/internal/fixplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/installplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/service"
	"github.com/stepanusjanu19/envdoctoragent/internal/snapshot"
	"github.com/stepanusjanu19/envdoctoragent/internal/version"
)

// Options configures the interactive CLI UI.
type Options struct {
	In     io.Reader
	Out    io.Writer
	Script string
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
		in:  bufio.NewReader(in),
		out: out,
	}

	if strings.TrimSpace(options.Script) != "" {
		return ui.runScript(options.Script)
	}
	return ui.runInteractive()
}

type session struct {
	in  *bufio.Reader
	out io.Writer
}

type palette struct {
	reset  string
	bold   string
	dim    string
	cyan   string
	green  string
	yellow string
}

func colors() palette {
	if os.Getenv("NO_COLOR") != "" {
		return palette{}
	}
	return palette{
		reset:  "\033[0m",
		bold:   "\033[1m",
		dim:    "\033[2m",
		cyan:   "\033[36m",
		green:  "\033[32m",
		yellow: "\033[33m",
	}
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
	c := colors()
	fmt.Fprintf(s.out, "%s%s\n", c.cyan, strings.Repeat("=", 64))
	fmt.Fprintf(s.out, "%sEnvdoctor Dashboard%s\n", c.bold, c.reset)
	fmt.Fprintf(s.out, "%s%s\n", c.cyan, strings.Repeat("=", 64))
	s.summaryRow("Environment", "implemented", "system, toolchain, PATH, containers")
	s.summaryRow("Dependency Coverage", "metadata-ready", "multi-ecosystem manifest registry")
	s.summaryRow("Service", "read-only", "native manager inspection")
	s.summaryRow("Version", "plan-only", "runtime requirement scan and switch suggestions")
	s.summaryRow("Fix Plan", "plan-only", "safe action list")
	s.summaryRow("Bootstrap Plan", "plan-only", "project setup suggestions")
	s.summaryRow("Logs", "implemented", "rule-based explanation")
	fmt.Fprintf(s.out, "%s%s%s\n", c.dim, strings.Repeat("-", 64), c.reset)
}

func (s *session) printMenuItems() {
	c := colors()
	fmt.Fprintf(s.out, "%sMenu%s\n", c.bold, c.reset)
	fmt.Fprintln(s.out, "  > 1  Diagnose")
	fmt.Fprintln(s.out, "    2  Service")
	fmt.Fprintln(s.out, "    3  Version")
	fmt.Fprintln(s.out, "    4  Install Plan")
	fmt.Fprintln(s.out, "    5  Fix Plan")
	fmt.Fprintln(s.out, "    6  Bootstrap Plan")
	fmt.Fprintln(s.out, "    7  Snapshot")
	fmt.Fprintln(s.out, "    8  Logs")
	fmt.Fprintln(s.out, "    0  Exit")
}

func (s *session) summaryRow(label, status, detail string) {
	c := colors()
	fmt.Fprintf(s.out, "%-20s %s%-14s%s %s%s%s\n", label, statusColor(c, status), status, c.reset, c.dim, detail, c.reset)
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
	report, err := diagnose.Run()
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	fmt.Fprintln(s.out, "Status: implemented")
	fmt.Fprintf(s.out, "Toolchain entries: %d\n", len(report.Toolchain))
	if report.PathReport != nil {
		fmt.Fprintf(s.out, "PATH issues: %d\n", report.PathReport.IssueCount)
	}
	fmt.Fprintf(s.out, "Recommendations: %d\n", len(report.Recommendations))
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
	info, err := service.Status(name)
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	fmt.Fprintln(s.out, "Status: read-only")
	fmt.Fprintf(s.out, "Manager: %s\nService status: %s\nState: %s\n", info.Manager, info.Status, valueOrDash(info.State))
}

func (s *session) runServiceList() {
	s.section("Service List")
	report, err := service.ListServices()
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	fmt.Fprintln(s.out, "Status: read-only")
	fmt.Fprintf(s.out, "Manager: %s\nManager status: %s\nServices: %d\n", report.Manager, report.Status, len(report.Services))
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
	report, err := version.Scan(dir)
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	fmt.Fprintln(s.out, "Status: read-only")
	fmt.Fprintf(s.out, "Managers: %d\nRuntimes: %d\nRequirements: %d\nItems needing review: %d\n", len(report.Managers), len(report.Runtimes), len(report.Requirements), len(report.Mismatches))
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
	plan := installplan.Generate(tool)
	fmt.Fprintln(s.out, "Status: plan-only")
	fmt.Fprintf(s.out, "Tool: %s\nManager: %s\n", plan.Action.Tool, valueOrDash(plan.Action.Manager))
	if plan.Action.Command != "" {
		fmt.Fprintf(s.out, "Suggested command: %s\n", plan.Action.Command)
	}
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
	report, err := fixplan.Generate(dir)
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	fmt.Fprintln(s.out, "Status: plan-only")
	fmt.Fprintf(s.out, "Actions: %d\n", len(report.Actions))
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
	plan, err := bootstrap.GeneratePlan(dir)
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	fmt.Fprintln(s.out, "Status: plan-only")
	fmt.Fprintf(s.out, "Actions: %d\nService hints: %d\n", len(plan.Actions), len(plan.ServiceHints))
}

func (s *session) runSnapshot() {
	s.section("Snapshot")
	snap, err := snapshot.CreateSnapshot()
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	fmt.Fprintln(s.out, "Status: read-only")
	fmt.Fprintf(s.out, "Tools: %d\n", len(snap.Tools))
	if snap.Path != nil {
		fmt.Fprintf(s.out, "PATH issues: %d\n", snap.Path.IssueCount)
	}
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
	result, err := analyzer.AnalyzeLog(path)
	if err != nil {
		fmt.Fprintf(s.out, "Status: error\nError: %v\n", err)
		return
	}
	fmt.Fprintln(s.out, "Status: implemented")
	fmt.Fprintf(s.out, "Issues: %d\n", len(result.Issues))
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
	c := colors()
	fmt.Fprintf(s.out, "\n%s[%s]%s\n", c.cyan, title, c.reset)
}

func statusColor(c palette, status string) string {
	switch status {
	case "implemented", "read-only":
		return c.green
	case "plan-only", "metadata-ready":
		return c.yellow
	default:
		return c.cyan
	}
}
