package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stepanusjanu19/envdoctoragent/internal/analyzer"
	"github.com/stepanusjanu19/envdoctoragent/internal/bootstrap"
	"github.com/stepanusjanu19/envdoctoragent/internal/cliui"
	"github.com/stepanusjanu19/envdoctoragent/internal/container"
	"github.com/stepanusjanu19/envdoctoragent/internal/dependencies"
	"github.com/stepanusjanu19/envdoctoragent/internal/diagnose"
	"github.com/stepanusjanu19/envdoctoragent/internal/dockerize"
	"github.com/stepanusjanu19/envdoctoragent/internal/fixplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/installplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/recommendation"
	"github.com/stepanusjanu19/envdoctoragent/internal/scanner"
	"github.com/stepanusjanu19/envdoctoragent/internal/service"
	"github.com/stepanusjanu19/envdoctoragent/internal/snapshot"
	"github.com/stepanusjanu19/envdoctoragent/internal/system"
	versionpkg "github.com/stepanusjanu19/envdoctoragent/internal/version"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "envdoctor",
		Short:   "Environment Doctor Agent - detect, analyze, and resolve dev env issues",
		Version: buildVersion(),
		Long: `An intelligent cross-platform environment diagnostic tool.
Supports Linux, Windows, and macOS.`,
	}

	// system command
	var systemJSON bool
	var systemCmd = &cobra.Command{
		Use:   "system",
		Short: "Display system information",
		Run: func(cmd *cobra.Command, args []string) {
			info, err := system.Detect()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error detecting system: %v\n", err)
				os.Exit(1)
			}
			if systemJSON {
				printJSON(info)
			} else {
				system.Print(info)
			}
		},
	}
	systemCmd.Flags().BoolVar(&systemJSON, "json", false, "Output in JSON format")

	// scan command
	var scanCmd = &cobra.Command{
		Use:   "scan",
		Short: "Scan various aspects of the environment",
	}

	// scan toolchain
	var scanToolchainJSON bool
	var scanToolchainCmd = &cobra.Command{
		Use:   "toolchain",
		Short: "Detect installed development tools",
		Run: func(cmd *cobra.Command, args []string) {
			tools, err := scanner.ScanToolchain()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error scanning toolchain: %v\n", err)
				os.Exit(1)
			}
			if scanToolchainJSON {
				printJSON(tools)
			} else {
				scanner.PrintToolchain(tools)
			}
		},
	}
	scanToolchainCmd.Flags().BoolVar(&scanToolchainJSON, "json", false, "Output in JSON format")

	// scan path
	var scanPathJSON bool
	var scanPathCmd = &cobra.Command{
		Use:   "path",
		Short: "Analyze PATH environment variable",
		Run: func(cmd *cobra.Command, args []string) {
			report, err := scanner.ScanPath()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error scanning PATH: %v\n", err)
				os.Exit(1)
			}
			if scanPathJSON {
				printJSON(report)
			} else {
				scanner.PrintPathReport(report)
			}
		},
	}
	scanPathCmd.Flags().BoolVar(&scanPathJSON, "json", false, "Output in JSON format")

	// scan container
	var scanContainerJSON bool
	var scanContainerCmd = &cobra.Command{
		Use:   "container",
		Short: "Validate container environments",
		Run: func(cmd *cobra.Command, args []string) {
			info, err := container.CheckContainerEnvironments()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error checking container environments: %v\n", err)
				os.Exit(1)
			}
			if scanContainerJSON {
				printJSON(info)
			} else {
				container.PrintContainerInfo(info)
			}
		},
	}
	scanContainerCmd.Flags().BoolVar(&scanContainerJSON, "json", false, "Output in JSON format")

	// scan dependencies
	var scanDependenciesJSON bool
	var scanDependenciesCmd = &cobra.Command{
		Use:   "dependencies [directory]",
		Short: "Analyze project dependencies",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			deps, err := dependencies.AnalyzeAllDependencies(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error analyzing dependencies: %v\n", err)
				os.Exit(1)
			}
			if scanDependenciesJSON {
				printJSON(deps)
			} else {
				dependencies.PrintAllDependencies(deps)
			}
		},
	}
	scanDependenciesCmd.Flags().BoolVar(&scanDependenciesJSON, "json", false, "Output in JSON format")

	// diagnose command
	var diagnoseCmd = &cobra.Command{
		Use:   "diagnose",
		Short: "Run full environment diagnosis",
	}

	// Add JSON flag to diagnose command
	var jsonOutput bool
	diagnoseCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	diagnoseCmd.Run = func(cmd *cobra.Command, args []string) {
		report, err := diagnose.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running diagnosis: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			diagnose.PrintJSON(report)
		} else {
			diagnose.Print(report)
		}
	}

	// explain command (Log Analyzer)
	var explainJSON bool
	var explainCmd = &cobra.Command{
		Use:   "explain <logfile>",
		Short: "Analyze a log file and provide root-cause analysis",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			result, err := analyzer.AnalyzeLog(filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error analyzing log: %v\n", err)
				os.Exit(1)
			}
			if explainJSON {
				printJSON(result)
			} else {
				analyzer.PrintAnalysis(result)
			}
		},
	}
	explainCmd.Flags().BoolVar(&explainJSON, "json", false, "Output in JSON format")

	// dockerize command (Dockerfile Generator)
	var dockerizeCmd = &cobra.Command{
		Use:   "dockerize [directory]",
		Short: "Generate a Dockerfile for the project in the given directory",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			content, err := dockerize.GenerateDockerfile(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating Dockerfile: %v\n", err)
				os.Exit(1)
			}

			output, _ := cmd.Flags().GetString("output")
			force, _ := cmd.Flags().GetBool("force")
			save, _ := cmd.Flags().GetBool("save")
			if save || output != "" {
				target := output
				if target == "" {
					target = filepath.Join(dir, "Dockerfile")
				}
				err := dockerize.SaveDockerfileTo(content, target, force)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error saving Dockerfile: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Dockerfile saved to %s\n", target)
			} else {
				fmt.Println(content)
			}
		},
	}
	dockerizeCmd.Flags().Bool("save", false, "Save the generated Dockerfile to disk")
	dockerizeCmd.Flags().String("output", "", "Write the generated Dockerfile to the specified file")
	dockerizeCmd.Flags().Bool("force", false, "Overwrite an existing Dockerfile or output file")

	// snapshot command
	var snapshotJSON bool
	var snapshotCmd = &cobra.Command{
		Use:   "snapshot",
		Short: "Create environment snapshot",
		Run: func(cmd *cobra.Command, args []string) {
			snap, err := snapshot.CreateSnapshot()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating snapshot: %v\n", err)
				os.Exit(1)
			}

			// Check for --save flag
			save, _ := cmd.Flags().GetBool("save")
			output, _ := cmd.Flags().GetString("output")
			if save || output != "" {
				filename := output
				if filename == "" {
					filename = snapshot.DefaultFilename(time.Now())
				}
				err := snapshot.SaveSnapshot(snap, filename)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error saving snapshot: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Snapshot saved to %s\n", filename)
			} else if snapshotJSON {
				printJSON(snap)
			} else {
				snapshot.PrintSnapshot(snap)
			}
		},
	}
	snapshotCmd.Flags().Bool("save", false, "Save the snapshot to a file")
	snapshotCmd.Flags().String("output", "", "Write the snapshot to the specified file")
	snapshotCmd.Flags().BoolVar(&snapshotJSON, "json", false, "Output in JSON format")

	// compare command
	var compareCmd = &cobra.Command{
		Use:   "compare <snapshot-a.json> <snapshot-b.json>",
		Short: "Compare two environment snapshots",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			snapA, err := snapshot.LoadSnapshot(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading snapshot %s: %v\n", args[0], err)
				os.Exit(1)
			}
			snapB, err := snapshot.LoadSnapshot(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading snapshot %s: %v\n", args[1], err)
				os.Exit(1)
			}
			err = snapshot.CompareSnapshots(snapA, snapB)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error comparing snapshots: %v\n", err)
				os.Exit(1)
			}
		},
	}

	scanCmd.AddCommand(scanToolchainCmd, scanPathCmd, scanContainerCmd, scanDependenciesCmd)

	// recommend command
	var recommendJSON bool
	var recommendCmd = &cobra.Command{
		Use:   "recommend",
		Short: "Generate actionable recommendations for detected environment issues",
		Run: func(cmd *cobra.Command, args []string) {
			report, err := recommendation.GenerateRecommendations()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating recommendations: %v\n", err)
				os.Exit(1)
			}
			if recommendJSON {
				printJSON(report)
			} else {
				recommendation.PrintReport(report)
			}
		},
	}
	recommendCmd.Flags().BoolVar(&recommendJSON, "json", false, "Output in JSON format")

	// service command (read-only)
	var serviceCmd = &cobra.Command{
		Use:   "service",
		Short: "Inspect OS services without changing them",
	}
	var serviceListJSON bool
	var serviceListCmd = &cobra.Command{
		Use:   "list",
		Short: "List services using the native service manager",
		Run: func(cmd *cobra.Command, args []string) {
			report, err := service.ListServices()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing services: %v\n", err)
				os.Exit(1)
			}
			if serviceListJSON {
				printJSON(report)
			} else {
				printServiceList(report)
			}
		},
	}
	serviceListCmd.Flags().BoolVar(&serviceListJSON, "json", false, "Output in JSON format")

	var serviceStatusJSON bool
	var serviceStatusCmd = &cobra.Command{
		Use:   "status <name>",
		Short: "Show read-only status for one service",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			info, err := service.Status(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error checking service: %v\n", err)
				os.Exit(1)
			}
			if serviceStatusJSON {
				printJSON(info)
			} else {
				printServiceInfo(info)
			}
		},
	}
	serviceStatusCmd.Flags().BoolVar(&serviceStatusJSON, "json", false, "Output in JSON format")

	var serviceDiagnoseJSON bool
	var serviceDiagnoseCmd = &cobra.Command{
		Use:   "diagnose <name>",
		Short: "Diagnose one service without restart or fix actions",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			report, err := service.Diagnose(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error diagnosing service: %v\n", err)
				os.Exit(1)
			}
			if serviceDiagnoseJSON {
				printJSON(report)
			} else {
				printServiceInfo(&report.Service)
				if len(report.Recommendations) > 0 {
					fmt.Println("Recommendations:")
					for _, rec := range report.Recommendations {
						fmt.Printf("  - %s\n", rec)
					}
				}
			}
		},
	}
	serviceDiagnoseCmd.Flags().BoolVar(&serviceDiagnoseJSON, "json", false, "Output in JSON format")
	serviceCmd.AddCommand(serviceListCmd, serviceStatusCmd, serviceDiagnoseCmd)

	// version command (read-only / plan-only)
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Inspect version managers and project runtime requirements",
	}
	var versionScanJSON bool
	var versionScanCmd = &cobra.Command{
		Use:   "scan [directory]",
		Short: "Scan version managers, runtimes, and project requirements",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			report, err := versionpkg.Scan(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error scanning versions: %v\n", err)
				os.Exit(1)
			}
			if versionScanJSON {
				printJSON(report)
			} else {
				printVersionScan(report)
			}
		},
	}
	versionScanCmd.Flags().BoolVar(&versionScanJSON, "json", false, "Output in JSON format")

	var versionPlanJSON bool
	var versionPlanCmd = &cobra.Command{
		Use:   "plan [directory]",
		Short: "Generate version manager install/switch suggestions",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			report, err := versionpkg.Plan(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating version plan: %v\n", err)
				os.Exit(1)
			}
			if versionPlanJSON {
				printJSON(report)
			} else {
				printVersionPlan(report)
			}
		},
	}
	versionPlanCmd.Flags().BoolVar(&versionPlanJSON, "json", false, "Output in JSON format")
	versionCmd.AddCommand(versionScanCmd, versionPlanCmd)

	// install command (plan-only)
	var installCmd = &cobra.Command{
		Use:   "install",
		Short: "Generate install plans without installing tools",
	}
	var installPlanJSON bool
	var installPlanCmd = &cobra.Command{
		Use:   "plan <tool>",
		Short: "Suggest a platform package-manager install command",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			plan := installplan.Generate(args[0])
			if installPlanJSON {
				printJSON(plan)
			} else {
				printInstallPlan(plan)
			}
		},
	}
	installPlanCmd.Flags().BoolVar(&installPlanJSON, "json", false, "Output in JSON format")
	installCmd.AddCommand(installPlanCmd)

	// fix command (plan-only)
	var fixCmd = &cobra.Command{
		Use:   "fix",
		Short: "Generate safe fix plans without applying changes",
	}
	var fixPlanJSON bool
	var fixPlanCmd = &cobra.Command{
		Use:   "plan [directory]",
		Short: "Combine environment findings into a non-mutating fix plan",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			report, err := fixplan.Generate(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating fix plan: %v\n", err)
				os.Exit(1)
			}
			if fixPlanJSON {
				printJSON(report)
			} else {
				printFixPlan(report)
			}
		},
	}
	fixPlanCmd.Flags().BoolVar(&fixPlanJSON, "json", false, "Output in JSON format")
	fixCmd.AddCommand(fixPlanCmd)

	// bootstrap command (plan-only)
	var bootstrapCmd = &cobra.Command{
		Use:   "bootstrap",
		Short: "Generate project bootstrap plans without applying changes",
	}
	var bootstrapPlanJSON bool
	var bootstrapPlanCmd = &cobra.Command{
		Use:   "plan [directory]",
		Short: "Plan runtime, dependency, and service setup for a project",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			plan, err := bootstrap.GeneratePlan(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating bootstrap plan: %v\n", err)
				os.Exit(1)
			}
			if bootstrapPlanJSON {
				printJSON(plan)
			} else {
				printBootstrapPlan(plan)
			}
		},
	}
	bootstrapPlanCmd.Flags().BoolVar(&bootstrapPlanJSON, "json", false, "Output in JSON format")
	bootstrapCmd.AddCommand(bootstrapPlanCmd)

	// ui command (interactive, non-mutating)
	var uiScript string
	var uiCmd = &cobra.Command{
		Use:   "ui",
		Short: "Start an interactive CLI UI for read-only and plan-only workflows",
		Run: func(cmd *cobra.Command, args []string) {
			err := cliui.Run(cliui.Options{
				In:     os.Stdin,
				Out:    os.Stdout,
				Script: uiScript,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
				os.Exit(1)
			}
		},
	}
	uiCmd.Flags().StringVar(&uiScript, "script", "", "Run comma-separated UI actions for smoke checks, e.g. diagnose,fix,exit")

	rootCmd.AddCommand(systemCmd, scanCmd, diagnoseCmd, snapshotCmd, explainCmd, compareCmd, dockerizeCmd, recommendCmd, serviceCmd, versionCmd, installCmd, fixCmd, bootstrapCmd, uiCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printJSON(value interface{}) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

func printServiceList(report *service.ListReport) {
	fmt.Printf("Service manager: %s (%s)\n", report.Manager, report.Status)
	if report.Message != "" {
		fmt.Println(report.Message)
	}
	if len(report.Services) == 0 {
		return
	}
	fmt.Printf("%-48s %-14s %s\n", "Service", "State", "Description")
	for _, svc := range report.Services {
		fmt.Printf("%-48s %-14s %s\n", svc.Name, valueOrDash(svc.State), svc.Description)
	}
}

func printServiceInfo(info *service.ServiceInfo) {
	fmt.Printf("Service: %s\n", info.Name)
	fmt.Printf("Manager: %s\n", info.Manager)
	fmt.Printf("Platform: %s\n", info.Platform)
	fmt.Printf("Status: %s\n", info.Status)
	if info.State != "" {
		fmt.Printf("State: %s\n", info.State)
	}
	if info.Description != "" {
		fmt.Printf("Description: %s\n", info.Description)
	}
	if info.Recommendation != "" {
		fmt.Printf("Recommendation: %s\n", info.Recommendation)
	}
}

func printVersionScan(report *versionpkg.ScanReport) {
	fmt.Println(report.Summary)
	fmt.Println()
	fmt.Println("Version managers:")
	for _, manager := range report.Managers {
		fmt.Printf("  %-8s found=%t version=%s source=%s\n", manager.Name, manager.Found, valueOrDash(manager.Version), valueOrDash(manager.Source))
	}
	fmt.Println()
	fmt.Println("Runtimes:")
	for _, runtimeInfo := range report.Runtimes {
		fmt.Printf("  %-8s found=%t version=%s manager=%s\n", runtimeInfo.Name, runtimeInfo.Found, valueOrDash(runtimeInfo.Version), valueOrDash(runtimeInfo.Manager))
	}
	if len(report.Requirements) > 0 {
		fmt.Println()
		fmt.Println("Project requirements:")
		for _, req := range report.Requirements {
			fmt.Printf("  %s %s from %s\n", req.Runtime, req.Version, req.SourceFile)
		}
	}
	if len(report.Mismatches) > 0 {
		fmt.Println()
		fmt.Println("Items needing review:")
		for _, mismatch := range report.Mismatches {
			fmt.Printf("  [%s] %s required=%s active=%s source=%s\n", mismatch.Status, mismatch.Runtime, mismatch.Required, valueOrDash(mismatch.Active), mismatch.SourceFile)
		}
	}
}

func printVersionPlan(report *versionpkg.PlanReport) {
	fmt.Println(report.Summary)
	for _, action := range report.Actions {
		fmt.Printf("- %s\n", action.Title)
		if action.Command != "" {
			fmt.Printf("  Suggested command: %s\n", action.Command)
		}
		if action.ManualSteps != "" {
			fmt.Printf("  Manual steps: %s\n", action.ManualSteps)
		}
	}
}

func printInstallPlan(plan *installplan.Plan) {
	fmt.Println(plan.Summary)
	action := plan.Action
	fmt.Printf("Tool: %s\n", action.Tool)
	fmt.Printf("Manager: %s\n", valueOrDash(action.Manager))
	fmt.Printf("Risk: %s | Requires admin: %t | Safe to run: %t\n", action.Risk, action.RequiresAdmin, action.SafeToRun)
	if action.Command != "" {
		fmt.Printf("Suggested command: %s\n", action.Command)
	}
	if action.ManualSteps != "" {
		fmt.Printf("Manual steps: %s\n", action.ManualSteps)
	}
}

func printFixPlan(report *fixplan.Report) {
	fmt.Println(report.Summary)
	for _, action := range report.Actions {
		fmt.Printf("- [%s] %s\n", action.Category, action.Title)
		if action.Command != "" {
			fmt.Printf("  Suggested command: %s\n", action.Command)
		}
		if action.ManualSteps != "" {
			fmt.Printf("  Manual steps: %s\n", action.ManualSteps)
		}
	}
}

func printBootstrapPlan(plan *bootstrap.Plan) {
	fmt.Println(plan.Summary)
	if len(plan.ServiceHints) > 0 {
		fmt.Println("Service hints:")
		for _, hint := range plan.ServiceHints {
			fmt.Printf("  - %s from %s\n", hint.Name, hint.SourceFile)
		}
	}
	if len(plan.Actions) > 0 {
		fmt.Println("Actions:")
		for _, action := range plan.Actions {
			fmt.Printf("  - [%s] %s\n", action.Category, action.Title)
			if action.Command != "" {
				fmt.Printf("    Suggested command: %s\n", action.Command)
			}
			if action.ManualSteps != "" {
				fmt.Printf("    Manual steps: %s\n", action.ManualSteps)
			}
		}
	}
}

func valueOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}
