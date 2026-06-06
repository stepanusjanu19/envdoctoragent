package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stepanusjanu19/envdoctoragent/internal/agent"
	"github.com/stepanusjanu19/envdoctoragent/internal/analyzer"
	"github.com/stepanusjanu19/envdoctoragent/internal/automation"
	"github.com/stepanusjanu19/envdoctoragent/internal/bootstrap"
	"github.com/stepanusjanu19/envdoctoragent/internal/cliui"
	"github.com/stepanusjanu19/envdoctoragent/internal/container"
	"github.com/stepanusjanu19/envdoctoragent/internal/dependencies"
	"github.com/stepanusjanu19/envdoctoragent/internal/diagnose"
	"github.com/stepanusjanu19/envdoctoragent/internal/dockerize"
	"github.com/stepanusjanu19/envdoctoragent/internal/executor"
	"github.com/stepanusjanu19/envdoctoragent/internal/fixplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/installplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/projectops"
	"github.com/stepanusjanu19/envdoctoragent/internal/rag"
	"github.com/stepanusjanu19/envdoctoragent/internal/recommendation"
	"github.com/stepanusjanu19/envdoctoragent/internal/scaffold"
	"github.com/stepanusjanu19/envdoctoragent/internal/scanner"
	"github.com/stepanusjanu19/envdoctoragent/internal/service"
	"github.com/stepanusjanu19/envdoctoragent/internal/snapshot"
	"github.com/stepanusjanu19/envdoctoragent/internal/system"
	"github.com/stepanusjanu19/envdoctoragent/internal/terminalui"
	versionpkg "github.com/stepanusjanu19/envdoctoragent/internal/version"

	"github.com/spf13/cobra"
)

var plainOutput bool

func main() {
	var rootCmd = &cobra.Command{
		Use:     "envdoctor",
		Short:   "Environment Doctor Agent - detect, analyze, and resolve dev env issues",
		Version: buildVersion(),
		Long: `An intelligent cross-platform environment diagnostic tool.
Supports Linux, Windows, and macOS.`,
	}
	rootCmd.PersistentFlags().BoolVar(&plainOutput, "plain", false, "Disable ANSI styling and progress bars for human-readable output")

	var aboutCmd = &cobra.Command{
		Use:   "about",
		Short: "Show envdoctor version, scope, and safety model",
		Run: func(cmd *cobra.Command, args []string) {
			newPresenter().About()
		},
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
			printDiagnoseReport(report)
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

	// service command (read-only / safe execution preview)
	var serviceCmd = &cobra.Command{
		Use:   "service",
		Short: "Inspect and safely plan/apply OS service operations",
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

	var servicePlanJSON bool
	var servicePlanCmd = &cobra.Command{
		Use:   "plan <start|stop|restart> <name>",
		Short: "Plan a service operation without executing it",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			report, err := service.Plan(args[0], args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating service plan: %v\n", err)
				os.Exit(1)
			}
			if servicePlanJSON {
				printJSON(report)
			} else {
				printServicePlan(report)
			}
		},
	}
	servicePlanCmd.Flags().BoolVar(&servicePlanJSON, "json", false, "Output in JSON format")

	var serviceApplyFlags executionFlags
	var serviceApplyCmd = &cobra.Command{
		Use:   "apply <start|stop|restart> <name>",
		Short: "Dry-run or apply a service operation with policy approval",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			report, err := service.Plan(args[0], args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating service plan: %v\n", err)
				os.Exit(1)
			}
			options, err := executionOptions(cmd, serviceApplyFlags, ".")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error preparing service apply: %v\n", err)
				os.Exit(1)
			}
			result, err := executor.Execute(executor.FromServicePlan(report, "."), options)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running service apply: %v\n", err)
				os.Exit(1)
			}
			if serviceApplyFlags.JSON {
				printJSON(result)
			} else {
				printExecutionReport(result)
			}
		},
	}
	addExecutionFlags(serviceApplyCmd, &serviceApplyFlags)
	serviceCmd.AddCommand(serviceListCmd, serviceStatusCmd, serviceDiagnoseCmd, servicePlanCmd, serviceApplyCmd)

	// version command (read-only / safe execution preview)
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

	var versionApplyFlags executionFlags
	var versionApplyCmd = &cobra.Command{
		Use:   "apply [directory]",
		Short: "Dry-run or apply allowlisted version-manager actions with approval",
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
			options, err := executionOptions(cmd, versionApplyFlags, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error preparing version apply: %v\n", err)
				os.Exit(1)
			}
			result, err := executor.Execute(executor.FromVersionPlan(report, dir), options)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running version apply: %v\n", err)
				os.Exit(1)
			}
			if versionApplyFlags.JSON {
				printJSON(result)
			} else {
				printExecutionReport(result)
			}
		},
	}
	addExecutionFlags(versionApplyCmd, &versionApplyFlags)
	versionCmd.AddCommand(versionScanCmd, versionPlanCmd, versionApplyCmd)

	// install command (plan/apply)
	var installCmd = &cobra.Command{
		Use:   "install",
		Short: "Generate install plans or apply allowlisted installs with approval",
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

	var installApplyFlags executionFlags
	var installApplyCmd = &cobra.Command{
		Use:   "apply <tool>",
		Short: "Dry-run or apply an allowlisted install plan with approval",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			plan := installplan.Generate(args[0])
			options, err := executionOptions(cmd, installApplyFlags, ".")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error preparing install apply: %v\n", err)
				os.Exit(1)
			}
			result, err := executor.Execute(executor.FromInstallPlan(plan, "."), options)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running install apply: %v\n", err)
				os.Exit(1)
			}
			if installApplyFlags.JSON {
				printJSON(result)
			} else {
				printExecutionReport(result)
			}
		},
	}
	addExecutionFlags(installApplyCmd, &installApplyFlags)
	installCmd.AddCommand(installPlanCmd, installApplyCmd)

	// fix command (plan/apply)
	var fixCmd = &cobra.Command{
		Use:   "fix",
		Short: "Generate safe fix plans or apply allowlisted fixes with approval",
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

	var fixApplyFlags executionFlags
	var fixApplyCmd = &cobra.Command{
		Use:   "apply [directory]",
		Short: "Dry-run or apply allowlisted fix actions with approval",
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
			options, err := executionOptions(cmd, fixApplyFlags, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error preparing fix apply: %v\n", err)
				os.Exit(1)
			}
			result, err := executor.Execute(executor.FromFixPlan(report, dir), options)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running fix apply: %v\n", err)
				os.Exit(1)
			}
			if fixApplyFlags.JSON {
				printJSON(result)
			} else {
				printExecutionReport(result)
			}
		},
	}
	addExecutionFlags(fixApplyCmd, &fixApplyFlags)
	fixCmd.AddCommand(fixPlanCmd, fixApplyCmd)

	// bootstrap command (plan/apply)
	var bootstrapCmd = &cobra.Command{
		Use:   "bootstrap",
		Short: "Generate project bootstrap plans or apply allowlisted setup actions with approval",
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

	var bootstrapApplyFlags executionFlags
	var bootstrapApplyCmd = &cobra.Command{
		Use:   "apply [directory]",
		Short: "Dry-run or apply allowlisted bootstrap actions with approval",
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
			options, err := executionOptions(cmd, bootstrapApplyFlags, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error preparing bootstrap apply: %v\n", err)
				os.Exit(1)
			}
			result, err := executor.Execute(executor.FromBootstrapPlan(plan, dir), options)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running bootstrap apply: %v\n", err)
				os.Exit(1)
			}
			if bootstrapApplyFlags.JSON {
				printJSON(result)
			} else {
				printExecutionReport(result)
			}
		},
	}
	addExecutionFlags(bootstrapApplyCmd, &bootstrapApplyFlags)
	bootstrapCmd.AddCommand(bootstrapPlanCmd, bootstrapApplyCmd)

	// project command (safe execution preview)
	var projectCmd = &cobra.Command{
		Use:   "project",
		Short: "Inspect, initialize, and manage project dependencies with approval-gated execution",
	}
	var projectScanJSON bool
	var projectScanCmd = &cobra.Command{
		Use:   "scan [directory]",
		Short: "Scan project lifecycle metadata",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			report, err := projectops.Scan(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error scanning project: %v\n", err)
				os.Exit(1)
			}
			if projectScanJSON {
				printJSON(report)
			} else {
				printProjectScan(report)
			}
		},
	}
	projectScanCmd.Flags().BoolVar(&projectScanJSON, "json", false, "Output in JSON format")

	var projectTemplatesJSON bool
	var projectTemplatesCmd = &cobra.Command{
		Use:   "templates",
		Short: "List project scaffold templates",
		Run: func(cmd *cobra.Command, args []string) {
			templates := projectops.ListTemplates()
			if projectTemplatesJSON {
				printJSON(templates)
			} else {
				printProjectTemplates(templates)
			}
		},
	}
	projectTemplatesCmd.Flags().BoolVar(&projectTemplatesJSON, "json", false, "Output in JSON format")

	var projectInitCmd = &cobra.Command{
		Use:   "init",
		Short: "Plan or apply project initialization templates",
	}
	var projectInitPlanFlags projectFlags
	var projectInitPlanCmd = &cobra.Command{
		Use:   "plan <template> [directory]",
		Short: "Plan initialization of a new project template",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 1 {
				dir = args[1]
			}
			plan, err := projectops.GenerateInitPlan(args[0], dir, projectOptions(projectInitPlanFlags))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating project init plan: %v\n", err)
				os.Exit(1)
			}
			if projectInitPlanFlags.JSON {
				printJSON(plan)
			} else {
				printProjectPlan(plan)
			}
		},
	}
	addScaffoldFlags(projectInitPlanCmd, &projectInitPlanFlags, true)

	var projectInitApplyFlags projectFlags
	var projectInitExecutionFlags executionFlags
	var projectInitApplyCmd = &cobra.Command{
		Use:   "apply <template> [directory]",
		Short: "Dry-run or apply a project initialization template with approval",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 1 {
				dir = args[1]
			}
			plan, err := projectops.GenerateInitPlan(args[0], dir, projectOptions(projectInitApplyFlags))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating project init plan: %v\n", err)
				os.Exit(1)
			}
			executionBaseDir := plan.Directory
			if plan.ExecutionBaseDir != "" {
				executionBaseDir = plan.ExecutionBaseDir
			}
			options, err := executionOptions(cmd, projectInitExecutionFlags, executionBaseDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error preparing project init apply: %v\n", err)
				os.Exit(1)
			}
			result, err := executor.Execute(plan.Actions, options)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running project init apply: %v\n", err)
				os.Exit(1)
			}
			if projectInitExecutionFlags.JSON {
				printJSON(result)
			} else {
				printExecutionReport(result)
			}
		},
	}
	addScaffoldFlags(projectInitApplyCmd, &projectInitApplyFlags, false)
	addExecutionFlags(projectInitApplyCmd, &projectInitExecutionFlags)
	projectInitCmd.AddCommand(projectInitPlanCmd, projectInitApplyCmd)

	var projectDepsCmd = &cobra.Command{
		Use:   "deps",
		Short: "Plan or apply project dependency operations",
	}
	var projectDepsPlanFlags projectFlags
	var projectDepsPlanCmd = &cobra.Command{
		Use:   "plan <sync|install|update|remove> [package] [directory]",
		Short: "Plan a project dependency operation",
		Args:  cobra.RangeArgs(1, 3),
		Run: func(cmd *cobra.Command, args []string) {
			operation, packageName, dir, err := parseProjectDepsArgs(args, projectDepsPlanFlags)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing project deps plan: %v\n", err)
				os.Exit(1)
			}
			plan, err := projectops.GenerateDepsPlan(operation, packageName, dir, projectOptions(projectDepsPlanFlags))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating project deps plan: %v\n", err)
				os.Exit(1)
			}
			if projectDepsPlanFlags.JSON {
				printJSON(plan)
			} else {
				printProjectPlan(plan)
			}
		},
	}
	addProjectFlags(projectDepsPlanCmd, &projectDepsPlanFlags, true)

	var projectDepsApplyFlags projectFlags
	var projectDepsExecutionFlags executionFlags
	var projectDepsApplyCmd = &cobra.Command{
		Use:   "apply <sync|install|update|remove> [package] [directory]",
		Short: "Dry-run or apply a project dependency operation with approval",
		Args:  cobra.RangeArgs(1, 3),
		Run: func(cmd *cobra.Command, args []string) {
			operation, packageName, dir, err := parseProjectDepsArgs(args, projectDepsApplyFlags)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing project deps apply: %v\n", err)
				os.Exit(1)
			}
			plan, err := projectops.GenerateDepsPlan(operation, packageName, dir, projectOptions(projectDepsApplyFlags))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating project deps plan: %v\n", err)
				os.Exit(1)
			}
			options, err := executionOptions(cmd, projectDepsExecutionFlags, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error preparing project deps apply: %v\n", err)
				os.Exit(1)
			}
			result, err := executor.Execute(plan.Actions, options)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running project deps apply: %v\n", err)
				os.Exit(1)
			}
			if projectDepsExecutionFlags.JSON {
				printJSON(result)
			} else {
				printExecutionReport(result)
			}
		},
	}
	addProjectFlags(projectDepsApplyCmd, &projectDepsApplyFlags, false)
	addExecutionFlags(projectDepsApplyCmd, &projectDepsExecutionFlags)
	projectDepsCmd.AddCommand(projectDepsPlanCmd, projectDepsApplyCmd)
	projectCmd.AddCommand(projectScanCmd, projectTemplatesCmd, projectInitCmd, projectDepsCmd)

	// agent command (deterministic autonomous preview)
	var agentCmd = &cobra.Command{
		Use:   "agent",
		Short: "Plan or run deterministic local autonomous workflows",
	}
	var agentPlanFlags agentCommandFlags
	var agentPlanCmd = &cobra.Command{
		Use:   "plan [directory]",
		Short: "Create an autonomous agent plan without executing actions",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			report, err := agent.Plan(agentOptions(agentPlanFlags, dir))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating agent plan: %v\n", err)
				os.Exit(1)
			}
			if agentPlanFlags.JSON {
				printJSON(report)
			} else {
				printAgentReport(report)
			}
		},
	}
	addAgentFlags(agentPlanCmd, &agentPlanFlags, false)

	var agentRunFlags agentCommandFlags
	var agentRunCmd = &cobra.Command{
		Use:   "run [directory]",
		Short: "Dry-run or apply an autonomous agent workflow with policy approval",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			execFlags := executionFlags{
				DryRun: agentRunFlags.DryRun, Yes: agentRunFlags.Yes, JSON: agentRunFlags.JSON,
				AuditLog: agentRunFlags.AuditLog, Timeout: agentRunFlags.Timeout,
				Profile: agentRunFlags.Profile, MaxRisk: agentRunFlags.MaxRisk, PolicyFile: agentRunFlags.PolicyFile,
			}
			options, err := executionOptions(cmd, execFlags, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error preparing agent run: %v\n", err)
				os.Exit(1)
			}
			report, err := agent.Run(agentOptions(agentRunFlags, dir), options)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running agent: %v\n", err)
				os.Exit(1)
			}
			if agentRunFlags.JSON {
				printJSON(report)
			} else {
				printAgentReport(report)
			}
		},
	}
	addAgentFlags(agentRunCmd, &agentRunFlags, true)
	agentCmd.AddCommand(agentPlanCmd, agentRunCmd)

	// automation command (controlled automation finalize)
	var automationCmd = &cobra.Command{
		Use:   "automation",
		Short: "Plan or run controlled automation workflows through policy-gated execution",
	}
	var automationPlanFlags automationCommandFlags
	var automationPlanCmd = &cobra.Command{
		Use:   "plan [directory]",
		Short: "Create a controlled automation plan without executing actions",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			report, err := automation.Plan(automationOptions(automationPlanFlags, dir))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating automation plan: %v\n", err)
				os.Exit(1)
			}
			if automationPlanFlags.JSON {
				printJSON(report)
			} else {
				printAutomationReport(report)
			}
		},
	}
	addAutomationFlags(automationPlanCmd, &automationPlanFlags, false)

	var automationRunFlags automationCommandFlags
	var automationRunCmd = &cobra.Command{
		Use:   "run [directory]",
		Short: "Dry-run or apply a controlled automation workflow with policy approval",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			execFlags := executionFlags{
				DryRun: automationRunFlags.DryRun, Yes: automationRunFlags.Yes, JSON: automationRunFlags.JSON,
				AuditLog: automationRunFlags.AuditLog, Timeout: automationRunFlags.Timeout,
				Profile: automationRunFlags.Profile, MaxRisk: automationRunFlags.MaxRisk, PolicyFile: automationRunFlags.PolicyFile,
			}
			options, err := executionOptions(cmd, execFlags, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error preparing automation run: %v\n", err)
				os.Exit(1)
			}
			report, err := automation.Run(automationOptions(automationRunFlags, dir), options)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running automation: %v\n", err)
				os.Exit(1)
			}
			if automationRunFlags.JSON {
				printJSON(report)
			} else {
				printAutomationReport(report)
			}
		},
	}
	addAutomationFlags(automationRunCmd, &automationRunFlags, true)
	automationCmd.AddCommand(automationPlanCmd, automationRunCmd)

	// rag command (local read-only knowledge layer)
	var ragCmd = &cobra.Command{
		Use:   "rag",
		Short: "Build and query a local read-only RAG knowledge layer",
	}
	var ragIndexFlags ragCommandFlags
	var ragIndexCmd = &cobra.Command{
		Use:   "index [directory]",
		Short: "Build a local lexical RAG index without executing actions",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			report, err := rag.CreateIndex(ragOptions(ragIndexFlags, dir, ""))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error building RAG index: %v\n", err)
				os.Exit(1)
			}
			if ragIndexFlags.JSON {
				printJSON(report)
			} else {
				printRAGIndexReport(report)
			}
		},
	}
	addRAGCollectionFlags(ragIndexCmd, &ragIndexFlags)
	ragIndexCmd.Flags().StringVar(&ragIndexFlags.Output, "output", "", "Write index JSON to this file")

	var ragQueryFlags ragCommandFlags
	var ragQueryCmd = &cobra.Command{
		Use:   "query <question> [directory]",
		Short: "Query local RAG context with lexical retrieval",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 1 {
				dir = args[1]
			}
			report, err := rag.Query(ragOptions(ragQueryFlags, dir, args[0]))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error querying RAG context: %v\n", err)
				os.Exit(1)
			}
			if ragQueryFlags.JSON {
				printJSON(report)
			} else {
				printRAGQueryReport(report)
			}
		},
	}
	addRAGCollectionFlags(ragQueryCmd, &ragQueryFlags)
	addRAGQueryFlags(ragQueryCmd, &ragQueryFlags)

	var ragContextFlags ragCommandFlags
	var ragContextCmd = &cobra.Command{
		Use:   "context [directory]",
		Short: "Build a goal-specific local RAG context pack",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			report, err := rag.Context(ragOptions(ragContextFlags, dir, ""))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating RAG context: %v\n", err)
				os.Exit(1)
			}
			if ragContextFlags.JSON {
				printJSON(report)
			} else {
				printRAGQueryReport(report)
			}
		},
	}
	addRAGCollectionFlags(ragContextCmd, &ragContextFlags)
	addRAGQueryFlags(ragContextCmd, &ragContextFlags)
	ragContextCmd.Flags().StringVar(&ragContextFlags.Goal, "goal", "diagnose", "Context goal: diagnose, onboard, repair, or maintain")
	ragCmd.AddCommand(ragIndexCmd, ragQueryCmd, ragContextCmd)

	// ui command (interactive, non-mutating)
	var uiScript string
	var uiCmd = &cobra.Command{
		Use:   "ui",
		Short: "Start an interactive CLI UI for read-only and plan-only workflows",
		Run: func(cmd *cobra.Command, args []string) {
			err := cliui.Run(cliui.Options{
				In:      os.Stdin,
				Out:     os.Stdout,
				Script:  uiScript,
				Version: buildVersion(),
				Plain:   plainOutput,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
				os.Exit(1)
			}
		},
	}
	uiCmd.Flags().StringVar(&uiScript, "script", "", "Run comma-separated UI actions for smoke checks, e.g. diagnose,fix,exit")

	rootCmd.AddCommand(aboutCmd, systemCmd, scanCmd, diagnoseCmd, snapshotCmd, explainCmd, compareCmd, dockerizeCmd, recommendCmd, serviceCmd, versionCmd, installCmd, fixCmd, bootstrapCmd, projectCmd, agentCmd, automationCmd, ragCmd, uiCmd)

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

func newPresenter() *terminalui.Presenter {
	return terminalui.New(os.Stdout, terminalui.Options{
		Plain:   plainOutput,
		Version: buildVersion(),
	})
}

func printDiagnoseReport(report *diagnose.Report) {
	ui := newPresenter()
	ui.Header("Envdoctor Diagnose", "workstation health summary")
	ui.Progress(1, 4, "System discovery")
	ui.Progress(2, 4, "Toolchain inventory")
	ui.Progress(3, 4, "PATH and container checks")
	ui.Progress(4, 4, "Recommendation summary")

	ui.Section("Summary")
	ui.StatusRow("Status", "implemented", "read-only diagnosis")
	if report.SystemInfo != nil {
		ui.Row("OS", report.SystemInfo.OS)
		ui.Row("Architecture", report.SystemInfo.Arch)
		ui.Row("Shell", report.SystemInfo.Shell)
	}
	ui.Row("Toolchain entries", fmt.Sprint(len(report.Toolchain)))
	if report.PathReport != nil {
		ui.Row("PATH entries", fmt.Sprint(len(report.PathReport.Entries)))
		ui.Row("PATH issues", fmt.Sprint(report.PathReport.IssueCount))
	}
	if report.ContainerInfo != nil {
		ui.Row("Container status", containerSummary(report.ContainerInfo))
	}
	ui.Row("Recommendations", fmt.Sprint(len(report.Recommendations)))

	if len(report.Recommendations) > 0 {
		ui.Section("Issues and actions")
		for _, rec := range report.Recommendations {
			ui.Bullet(rec.Severity, rec.Title)
			ui.Detail("Category", rec.Category)
			if rec.Command != "" {
				ui.Detail("Suggested command", rec.Command)
			}
			if rec.ManualSteps != "" {
				ui.Detail("Manual", rec.ManualSteps)
			}
		}
	}
	ui.NextSteps("Run envdoctor diagnose --json for stable automation output.", "Use plan/apply commands with --dry-run before considering --yes.")
}

func containerSummary(info *container.ContainerInfo) string {
	if info == nil {
		return "-"
	}
	var parts []string
	if info.Docker != nil {
		parts = append(parts, "docker="+info.Docker.DaemonStatus)
	}
	if info.Podman != nil {
		parts = append(parts, "podman="+info.Podman.Status)
	}
	if info.Kubernetes != nil {
		parts = append(parts, "kubernetes="+info.Kubernetes.ConfigStatus)
	}
	if len(parts) == 0 {
		return "no container tooling detected"
	}
	return strings.Join(parts, ", ")
}

type executionFlags struct {
	DryRun     bool
	Yes        bool
	JSON       bool
	AuditLog   string
	Timeout    string
	Profile    string
	MaxRisk    string
	PolicyFile string
}

type projectFlags struct {
	JSON          bool
	Ecosystem     string
	Manager       string
	Dev           bool
	Version       string
	All           bool
	AllowNonEmpty bool
	Name          string
	Module        string
	PackageName   string
	Source        string
	Force         bool
	CreateDir     bool
}

type agentCommandFlags struct {
	DryRun        bool
	Yes           bool
	JSON          bool
	AuditLog      string
	Timeout       string
	Profile       string
	MaxRisk       string
	PolicyFile    string
	Goal          string
	Template      string
	Name          string
	Module        string
	PackageName   string
	Source        string
	CreateDir     bool
	Force         bool
	AllowNonEmpty bool
}

type automationCommandFlags struct {
	DryRun        bool
	Yes           bool
	JSON          bool
	AuditLog      string
	Timeout       string
	Profile       string
	MaxRisk       string
	PolicyFile    string
	Goal          string
	Template      string
	Name          string
	Module        string
	PackageName   string
	Source        string
	CreateDir     bool
	Force         bool
	AllowNonEmpty bool
}

type ragCommandFlags struct {
	JSON         bool
	Output       string
	Index        string
	Query        string
	Goal         string
	TopK         int
	IncludeAudit bool
	Logs         []string
}

func addExecutionFlags(command *cobra.Command, flags *executionFlags) {
	command.Flags().BoolVar(&flags.DryRun, "dry-run", true, "Preview actions without executing them")
	command.Flags().BoolVar(&flags.Yes, "yes", false, "Approve execution of allowlisted actions")
	command.Flags().BoolVar(&flags.JSON, "json", false, "Output in JSON format")
	command.Flags().StringVar(&flags.AuditLog, "audit-log", "", "Write audit JSONL to the specified file")
	command.Flags().StringVar(&flags.Timeout, "timeout", executor.DefaultTimeout.String(), "Per-action timeout, for example 30s or 2m")
	command.Flags().StringVar(&flags.Profile, "profile", executor.ProfileDevelopment, "Execution policy profile: development or production")
	command.Flags().StringVar(&flags.MaxRisk, "max-risk", executor.RiskHigh, "Maximum action risk allowed by policy: low, medium, or high")
	command.Flags().StringVar(&flags.PolicyFile, "policy-file", "", "Optional JSON policy file")
}

func addRAGCollectionFlags(command *cobra.Command, flags *ragCommandFlags) {
	command.Flags().BoolVar(&flags.JSON, "json", false, "Output in JSON format")
	command.Flags().BoolVar(&flags.IncludeAudit, "include-audit", false, "Include redacted local audit summaries")
	command.Flags().StringArrayVar(&flags.Logs, "log", nil, "Add a specific log file to the local RAG context")
}

func addRAGQueryFlags(command *cobra.Command, flags *ragCommandFlags) {
	command.Flags().StringVar(&flags.Index, "index", "", "Read an existing RAG index JSON instead of building in memory")
	command.Flags().IntVar(&flags.TopK, "top-k", rag.DefaultTopK, "Number of ranked matches to return")
}

func addAgentFlags(command *cobra.Command, flags *agentCommandFlags, includeExecution bool) {
	command.Flags().BoolVar(&flags.JSON, "json", false, "Output in JSON format")
	command.Flags().StringVar(&flags.Profile, "profile", executor.ProfileDevelopment, "Policy profile: development or production")
	command.Flags().StringVar(&flags.MaxRisk, "max-risk", executor.RiskHigh, "Maximum action risk allowed by policy: low, medium, or high")
	command.Flags().StringVar(&flags.PolicyFile, "policy-file", "", "Optional JSON policy file")
	command.Flags().StringVar(&flags.Goal, "goal", agent.GoalDiagnose, "Agent goal: diagnose, onboard, repair, scaffold, or bootstrap")
	command.Flags().StringVar(&flags.Template, "template", "", "Scaffold template id for --goal scaffold")
	command.Flags().StringVar(&flags.Name, "name", "", "Project display/package name for scaffold templates")
	command.Flags().StringVar(&flags.Module, "module", "", "Module path for scaffold templates")
	command.Flags().StringVar(&flags.PackageName, "package", "", "Package or namespace for scaffold templates")
	command.Flags().StringVar(&flags.Source, "source", "auto", "Scaffold source: auto or official (official-only)")
	command.Flags().BoolVar(&flags.CreateDir, "create-dir", false, "Create the target project directory when it does not exist")
	command.Flags().BoolVar(&flags.Force, "force", false, "Allow scaffold file overwrite when safe")
	command.Flags().BoolVar(&flags.AllowNonEmpty, "allow-non-empty", false, "Allow scaffold planning/apply in a non-empty directory")
	if includeExecution {
		command.Flags().BoolVar(&flags.DryRun, "dry-run", true, "Preview actions without executing them")
		command.Flags().BoolVar(&flags.Yes, "yes", false, "Approve execution of allowlisted actions")
		command.Flags().StringVar(&flags.AuditLog, "audit-log", "", "Write audit JSONL to the specified file")
		command.Flags().StringVar(&flags.Timeout, "timeout", executor.DefaultTimeout.String(), "Per-action timeout, for example 30s or 2m")
	}
}

func addAutomationFlags(command *cobra.Command, flags *automationCommandFlags, includeExecution bool) {
	command.Flags().BoolVar(&flags.JSON, "json", false, "Output in JSON format")
	command.Flags().StringVar(&flags.Profile, "profile", executor.ProfileDevelopment, "Policy profile: development or production")
	command.Flags().StringVar(&flags.MaxRisk, "max-risk", executor.RiskHigh, "Maximum action risk allowed by policy: low, medium, or high")
	command.Flags().StringVar(&flags.PolicyFile, "policy-file", "", "Optional JSON policy file")
	command.Flags().StringVar(&flags.Goal, "goal", automation.GoalDiagnose, "Automation goal: diagnose, onboard, repair, scaffold, bootstrap, or maintain")
	command.Flags().StringVar(&flags.Template, "template", "", "Scaffold template id for --goal scaffold")
	command.Flags().StringVar(&flags.Name, "name", "", "Project display/package name for scaffold templates")
	command.Flags().StringVar(&flags.Module, "module", "", "Module path for scaffold templates")
	command.Flags().StringVar(&flags.PackageName, "package", "", "Package or namespace for scaffold templates")
	command.Flags().StringVar(&flags.Source, "source", "auto", "Scaffold source: auto or official (official-only)")
	command.Flags().BoolVar(&flags.CreateDir, "create-dir", false, "Create the target project directory when it does not exist")
	command.Flags().BoolVar(&flags.Force, "force", false, "Allow scaffold file overwrite when safe")
	command.Flags().BoolVar(&flags.AllowNonEmpty, "allow-non-empty", false, "Allow scaffold planning/apply in a non-empty directory")
	if includeExecution {
		command.Flags().BoolVar(&flags.DryRun, "dry-run", true, "Preview actions without executing them")
		command.Flags().BoolVar(&flags.Yes, "yes", false, "Approve execution of allowlisted actions")
		command.Flags().StringVar(&flags.AuditLog, "audit-log", "", "Write audit JSONL to the specified file")
		command.Flags().StringVar(&flags.Timeout, "timeout", executor.DefaultTimeout.String(), "Per-action timeout, for example 30s or 2m")
	}
}

func addProjectFlags(command *cobra.Command, flags *projectFlags, includeJSON bool) {
	if includeJSON {
		command.Flags().BoolVar(&flags.JSON, "json", false, "Output in JSON format")
	}
	command.Flags().StringVar(&flags.Ecosystem, "ecosystem", "", "Select ecosystem when multiple manifests are detected")
	command.Flags().StringVar(&flags.Manager, "manager", "", "Override package manager for the selected ecosystem")
	command.Flags().BoolVar(&flags.Dev, "dev", false, "Use development dependency scope when supported")
	command.Flags().StringVar(&flags.Version, "version", "", "Dependency version constraint for install/update operations")
	command.Flags().BoolVar(&flags.All, "all", false, "Apply update operation to all dependencies when supported")
	command.Flags().BoolVar(&flags.AllowNonEmpty, "allow-non-empty", false, "Allow project init planning/apply in a non-empty directory")
}

func addScaffoldFlags(command *cobra.Command, flags *projectFlags, includeJSON bool) {
	addProjectFlags(command, flags, includeJSON)
	command.Flags().StringVar(&flags.Name, "name", "", "Project display/package name for scaffold templates")
	command.Flags().StringVar(&flags.Module, "module", "", "Module path for Go/JVM-style scaffold templates")
	command.Flags().StringVar(&flags.PackageName, "package", "", "Package or namespace for scaffold templates")
	command.Flags().StringVar(&flags.Source, "source", "auto", "Scaffold source: auto or official (official-only)")
	command.Flags().BoolVar(&flags.Force, "force", false, "Allow scaffold file overwrite when safe")
	command.Flags().BoolVar(&flags.CreateDir, "create-dir", false, "Create the target project directory when it does not exist")
}

func projectOptions(flags projectFlags) projectops.Options {
	return projectops.Options{
		Ecosystem:     flags.Ecosystem,
		Manager:       flags.Manager,
		Dev:           flags.Dev,
		Version:       flags.Version,
		All:           flags.All,
		AllowNonEmpty: flags.AllowNonEmpty,
		Name:          flags.Name,
		Module:        flags.Module,
		PackageName:   flags.PackageName,
		Source:        flags.Source,
		Force:         flags.Force,
		CreateDir:     flags.CreateDir,
	}
}

func agentOptions(flags agentCommandFlags, dir string) agent.Options {
	return agent.Options{
		Directory: dir,
		Goal:      flags.Goal,
		Template:  flags.Template,
		Profile:   flags.Profile,
		MaxRisk:   flags.MaxRisk,
		Project: projectops.Options{
			Name:          flags.Name,
			Module:        flags.Module,
			PackageName:   flags.PackageName,
			Source:        flags.Source,
			Force:         flags.Force,
			CreateDir:     flags.CreateDir,
			AllowNonEmpty: flags.AllowNonEmpty,
		},
	}
}

func automationOptions(flags automationCommandFlags, dir string) automation.Options {
	return automation.Options{
		Directory: dir,
		Goal:      flags.Goal,
		Template:  flags.Template,
		Profile:   flags.Profile,
		MaxRisk:   flags.MaxRisk,
		Project: projectops.Options{
			Name:          flags.Name,
			Module:        flags.Module,
			PackageName:   flags.PackageName,
			Source:        flags.Source,
			Force:         flags.Force,
			CreateDir:     flags.CreateDir,
			AllowNonEmpty: flags.AllowNonEmpty,
		},
	}
}

func ragOptions(flags ragCommandFlags, dir, query string) rag.Options {
	return rag.Options{
		Directory:    dir,
		Output:       flags.Output,
		Index:        flags.Index,
		Query:        query,
		Goal:         flags.Goal,
		TopK:         flags.TopK,
		IncludeAudit: flags.IncludeAudit,
		Logs:         flags.Logs,
	}
}

func executionOptions(command *cobra.Command, flags executionFlags, baseDir string) (executor.Options, error) {
	dryRun := true
	if flags.Yes {
		dryRun = false
	}
	if command.Flags().Changed("dry-run") {
		dryRun = flags.DryRun
	}
	if !flags.Yes && command.Flags().Changed("dry-run") && !flags.DryRun {
		return executor.Options{}, fmt.Errorf("mutating apply requires --yes; omit --dry-run=false or pass --yes")
	}

	timeout, err := time.ParseDuration(flags.Timeout)
	if err != nil {
		return executor.Options{}, fmt.Errorf("invalid --timeout value %q: %w", flags.Timeout, err)
	}
	if timeout <= 0 {
		return executor.Options{}, fmt.Errorf("--timeout must be positive")
	}
	auditLog := flags.AuditLog
	if dryRun && auditLog == "" {
		auditLog = filepath.Join(os.TempDir(), "envdoctor", "audit", time.Now().Format("2006-01-02-150405")+".jsonl")
	}
	profile := flags.Profile
	if !command.Flags().Changed("profile") {
		profile = ""
	}
	maxRisk := flags.MaxRisk
	if !command.Flags().Changed("max-risk") {
		maxRisk = ""
	}

	return executor.Options{
		DryRun:     dryRun,
		Approved:   flags.Yes,
		BaseDir:    baseDir,
		AuditLog:   auditLog,
		Timeout:    timeout,
		Profile:    profile,
		MaxRisk:    maxRisk,
		PolicyFile: flags.PolicyFile,
	}, nil
}

func printExecutionReport(report *executor.Report) {
	ui := newPresenter()
	ui.Header("Envdoctor Apply Report", "approval-gated executor")
	ui.Progress(1, 3, "Load policy")
	ui.Progress(2, 3, "Evaluate actions")
	ui.Progress(3, 3, "Summarize results")
	ui.Section("Summary")
	ui.Row("Result", report.Summary)
	ui.StatusRow("Mode", report.Mode, "executor outcome")
	ui.Row("Profile", report.Profile)
	ui.Row("Max risk", report.MaxRisk)
	if report.PolicyFile != "" {
		ui.Row("Policy file", report.PolicyFile)
	}
	ui.Row("Audit log", report.AuditLog)
	if report.SnapshotFile != "" {
		ui.Row("Pre-apply snapshot", report.SnapshotFile)
	}
	if report.ProjectSnapshotFile != "" {
		ui.Row("Project snapshot", report.ProjectSnapshotFile)
	}
	if len(report.PolicyDecisions) > 0 {
		ui.Section("Policy decisions")
		for _, decision := range report.PolicyDecisions {
			if decision.Allowed {
				ui.Bullet("allowed", fmt.Sprintf("%s (%s)", decision.ActionID, decision.Risk))
			} else {
				ui.Bullet("blocked", fmt.Sprintf("%s (%s)", decision.ActionID, decision.Reason))
			}
		}
	}
	if len(report.ProjectChanges) > 0 {
		ui.Section("Project changes")
		for _, change := range report.ProjectChanges {
			ui.Bullet("changed", change)
		}
	}
	if len(report.Results) == 0 {
		ui.NextSteps("No executable actions were generated.", "Review the matching plan command for details.")
		return
	}
	ui.Section("Actions")
	for _, result := range report.Results {
		ui.Bullet(result.Status, result.Action.Title)
		if result.Action.SuggestedCommand != "" {
			ui.Detail("Suggested command", result.Action.SuggestedCommand)
		} else if result.Action.Command != "" {
			ui.Detail("Command", commandLine(result.Action.Command, result.Action.Args))
		}
		if result.Action.Type == "mkdir" || result.Action.Type == "write_file" {
			ui.Detail("File action", strings.TrimSpace(result.Action.Type+" "+result.Action.Path))
			if result.Action.ContentBytes > 0 {
				ui.Detail("Content bytes", fmt.Sprint(result.Action.ContentBytes))
			}
		}
		if result.Action.ManualSteps != "" {
			ui.Detail("Manual", result.Action.ManualSteps)
		}
		if result.Error != "" {
			ui.Detail("Error", result.Error)
		}
		if result.Message != "" {
			ui.Detail("Message", result.Message)
		}
		if result.Action.RollbackHint != "" {
			ui.Detail("Rollback hint", result.Action.RollbackHint)
		}
	}
	ui.NextSteps("Use --json for full structured audit details.", "Keep production profile non-mutating unless policy is explicitly changed.")
}

func printAgentReport(report *agent.Report) {
	ui := newPresenter()
	ui.Header("Envdoctor Agent", "deterministic local orchestration")
	ui.Progress(1, 4, "Resolve goal")
	ui.Progress(2, 4, "Collect context")
	ui.Progress(3, 4, "Build action plan")
	ui.Progress(4, 4, "Prepare report")
	ui.Section("Summary")
	ui.Row("Result", report.Summary)
	ui.Row("Goal", report.Goal)
	ui.Row("Profile", report.Profile)
	ui.Row("Max risk", report.MaxRisk)
	ui.Row("Directory", report.Directory)
	ui.StatusRow("Status", report.Status, "agent report")
	if report.Diagnostics != nil {
		ui.Row("Diagnostics", "available")
	}
	if report.ProjectScan != nil {
		ui.Row("Project scan", report.ProjectScan.Summary)
	}
	if report.FixPlan != nil {
		ui.Row("Fix plan", report.FixPlan.Summary)
	}
	if report.BootstrapPlan != nil {
		ui.Row("Bootstrap plan", report.BootstrapPlan.Summary)
	}
	if report.ProjectPlan != nil {
		ui.Row("Project plan", report.ProjectPlan.Summary)
	}
	if len(report.Actions) > 0 {
		ui.Section("Actions")
		for _, action := range report.Actions {
			ui.Bullet(valueOrDash(action.Category), action.Title)
			if action.Command != "" {
				ui.Detail("Command", commandLine(action.Command, action.Args))
			}
			if action.Path != "" {
				ui.Detail("File action", strings.TrimSpace(action.Type+" "+action.Path))
			}
			if action.ManualSteps != "" {
				ui.Detail("Manual", action.ManualSteps)
			}
		}
	}
	if report.Execution != nil {
		printExecutionReport(report.Execution)
		return
	}
	ui.NextSteps("Use agent run --dry-run before --yes.", "Use --json to pass the report into IDE or CI wrappers.")
}

func printAutomationReport(report *automation.Report) {
	ui := newPresenter()
	ui.Header("Envdoctor Automation", "controlled automation finalize")
	ui.Progress(1, 4, "Resolve goals")
	ui.Progress(2, 4, "Collect agent plans")
	ui.Progress(3, 4, "Summarize policy")
	ui.Progress(4, 4, "Prepare report")
	ui.Section("Summary")
	ui.Row("Result", report.Summary)
	ui.Row("Goal", report.Goal)
	ui.Row("Selected goals", strings.Join(report.SelectedGoals, ", "))
	ui.Row("Profile", report.Profile)
	ui.Row("Max risk", report.MaxRisk)
	ui.Row("Directory", report.Directory)
	ui.StatusRow("Status", report.Status, "controlled automation")
	ui.Section("Policy Summary")
	ui.Row("Selected actions", fmt.Sprint(report.PolicySummary.SelectedActions))
	ui.Row("Skipped actions", fmt.Sprint(report.PolicySummary.SkippedActions))
	ui.Row("Blocked actions", fmt.Sprint(report.PolicySummary.BlockedActions))
	ui.Row("Production mutation blocked", fmt.Sprint(report.PolicySummary.ProductionMutationBlocked))
	for _, approval := range report.PolicySummary.RequiredApprovals {
		ui.Bullet("Approval", approval)
	}
	for _, hint := range report.PolicySummary.RollbackHints {
		ui.Detail("Rollback hint", hint)
	}
	for _, note := range report.PolicySummary.Notes {
		ui.Detail("Note", note)
	}
	if len(report.Actions) > 0 {
		ui.Section("Actions")
		for _, action := range report.Actions {
			ui.Bullet(valueOrDash(action.Category), action.Title)
			if action.Command != "" {
				ui.Detail("Command", commandLine(action.Command, action.Args))
			}
			if action.Path != "" {
				ui.Detail("File action", strings.TrimSpace(action.Type+" "+action.Path))
			}
			if action.ManualSteps != "" {
				ui.Detail("Manual", action.ManualSteps)
			}
		}
	}
	if report.Execution != nil {
		printExecutionReport(report.Execution)
		return
	}
	ui.NextSteps("Use automation run --dry-run before --yes.", "Production profile remains non-mutating by default.")
}

func printRAGIndexReport(report *rag.IndexReport) {
	ui := newPresenter()
	ui.Header("Envdoctor RAG", "local read-only knowledge index")
	ui.Progress(1, 3, "Collect deterministic context")
	ui.Progress(2, 3, "Build lexical index")
	ui.Progress(3, 3, "Write local index")
	ui.Section("Summary")
	ui.Row("Result", report.Summary)
	ui.Row("Directory", report.Directory)
	ui.Row("Output", report.Output)
	ui.Row("Sources", fmt.Sprint(len(report.Sources)))
	ui.StatusRow("Status", report.Status, "read-only RAG")
	if len(report.Limitations) > 0 {
		ui.Section("Limitations")
		for _, limitation := range report.Limitations {
			ui.Bullet("note", limitation)
		}
	}
	printRAGSafety(ui, report.Safety)
	ui.NextSteps("Run envdoctor rag query \"<question>\" --index "+report.Output, "RAG is retrieval-only and cannot execute actions.")
}

func printRAGQueryReport(report *rag.QueryReport) {
	ui := newPresenter()
	ui.Header("Envdoctor RAG", "local read-only knowledge query")
	ui.Progress(1, 3, "Collect or load context")
	ui.Progress(2, 3, "Rank lexical matches")
	ui.Progress(3, 3, "Prepare safety-bounded answer")
	ui.Section("Summary")
	ui.Row("Answer", report.Answer)
	ui.Row("Directory", report.Directory)
	if report.Goal != "" {
		ui.Row("Goal", report.Goal)
	}
	ui.Row("Query", report.Query)
	ui.Row("Sources", fmt.Sprint(len(report.Sources)))
	ui.Row("Matches", fmt.Sprint(len(report.Matches)))
	ui.StatusRow("Status", report.Status, "read-only RAG")
	if len(report.Matches) > 0 {
		ui.Section("Ranked matches")
		for _, match := range report.Matches {
			ui.Bullet(fmt.Sprintf("%.2f", match.Score), match.Title)
			ui.Detail("Source", match.Source)
			ui.Detail("Type", match.SourceType)
			if match.Snippet != "" {
				ui.Detail("Snippet", match.Snippet)
			}
		}
	}
	if len(report.Limitations) > 0 {
		ui.Section("Limitations")
		for _, limitation := range report.Limitations {
			ui.Bullet("note", limitation)
		}
	}
	printRAGSafety(ui, report.Safety)
	if len(report.NextSteps) > 0 {
		ui.NextSteps(report.NextSteps...)
	}
}

func printRAGSafety(ui *terminalui.Presenter, safety rag.Safety) {
	ui.Section("Safety")
	ui.Row("Read only", fmt.Sprint(safety.ReadOnly))
	ui.Row("Executor access", fmt.Sprint(safety.ExecutorAccess))
	ui.Row("Mutating actions", fmt.Sprint(safety.MutatingActions))
	for _, note := range safety.Notes {
		ui.Detail("Note", note)
	}
}

func parseProjectDepsArgs(args []string, flags projectFlags) (string, string, string, error) {
	operation := strings.ToLower(strings.TrimSpace(args[0]))
	rest := args[1:]
	dir := "."
	packageName := ""

	switch operation {
	case "sync":
		if len(rest) > 1 {
			return "", "", "", fmt.Errorf("sync accepts at most one directory argument")
		}
		if len(rest) == 1 {
			dir = rest[0]
		}
	case "install", "remove":
		if len(rest) == 0 {
			return "", "", "", fmt.Errorf("%s requires a package name", operation)
		}
		packageName = rest[0]
		if len(rest) > 1 {
			dir = rest[1]
		}
	case "update":
		if flags.All {
			if len(rest) > 1 {
				return "", "", "", fmt.Errorf("update --all accepts at most one directory argument")
			}
			if len(rest) == 1 {
				dir = rest[0]
			}
			return operation, "", dir, nil
		}
		if len(rest) == 0 {
			return "", "", "", fmt.Errorf("update requires a package name or --all")
		}
		packageName = rest[0]
		if len(rest) > 1 {
			dir = rest[1]
		}
	default:
		return "", "", "", fmt.Errorf("unsupported operation %q", operation)
	}
	return operation, packageName, dir, nil
}

func printProjectScan(report *projectops.ScanReport) {
	ui := newPresenter()
	ui.Header("Envdoctor Project Scan", "project lifecycle metadata")
	ui.Progress(1, 3, "Search manifests")
	ui.Progress(2, 3, "Identify ecosystems")
	ui.Progress(3, 3, "Summarize managers")
	ui.Section("Summary")
	ui.Row("Result", report.Summary)
	ui.Row("Directory", report.Directory)
	ui.StatusRow("Status", report.Status, "project scan")
	if len(report.Ecosystems) > 0 {
		ui.Row("Ecosystems", strings.Join(report.Ecosystems, ", "))
	}
	if len(report.Managers) > 0 {
		ui.Row("Package managers", strings.Join(report.Managers, ", "))
	}
	ui.Row("Manifests", fmt.Sprint(len(report.Manifests)))
	if len(report.Manifests) > 0 {
		ui.Section("Manifests")
	}
	for _, manifest := range report.Manifests {
		ui.Bullet(manifest.ValidationStatus, manifest.SourceFile)
		ui.Detail("Ecosystem", manifest.Ecosystem)
		ui.Detail("Package manager", manifest.PackageManager)
	}
	ui.NextSteps("Run envdoctor project deps plan sync to inspect dependency actions.", "Use --json for stable project metadata output.")
}

func printProjectTemplates(templates []scaffold.Template) {
	ui := newPresenter()
	ui.Header("Envdoctor Project Templates", "official starter generators")
	if len(templates) == 0 {
		ui.NextSteps("No scaffold templates are registered.")
		return
	}
	ui.Section("Summary")
	ui.StatusRow("Status", "safe apply preview", "official-only starter registry")
	ui.Row("Templates", fmt.Sprint(len(templates)))
	ui.Section("Templates")
	fmt.Printf("%-18s %-12s %-18s %-10s %s\n", "Template", "Language", "Framework", "Source", "Summary")
	for _, template := range templates {
		fmt.Printf("%-18s %-12s %-18s %-10s %s\n", template.ID, valueOrDash(template.Language), valueOrDash(template.Framework), valueOrDash(template.Source), template.Summary)
	}
	ui.NextSteps("Run envdoctor project init plan <template> --create-dir <dir> to preview a starter.", "Missing generators are reported as blocked apply results with install plan hints.")
}

func printProjectPlan(plan *projectops.Plan) {
	ui := newPresenter()
	ui.Header("Envdoctor Project Plan", "lifecycle action preview")
	ui.Progress(1, 3, "Resolve project context")
	ui.Progress(2, 3, "Build structured actions")
	ui.Progress(3, 3, "Summarize safety metadata")
	ui.Section("Summary")
	ui.Row("Result", plan.Summary)
	ui.Row("Directory", plan.Directory)
	ui.Row("Operation", plan.Operation)
	if plan.Template != "" {
		ui.Row("Template", plan.Template)
	}
	if plan.Source != "" {
		ui.Row("Source", plan.Source)
	}
	if plan.Ecosystem != "" {
		ui.Row("Ecosystem", plan.Ecosystem)
	}
	if plan.PackageManager != "" {
		ui.Row("Package manager", plan.PackageManager)
	}
	if plan.RequiresNetwork {
		ui.Row("Requires network", "yes")
	}
	if len(plan.Files) > 0 {
		ui.Section("Files")
		for _, file := range plan.Files {
			ui.Bullet("file", fmt.Sprintf("%s (%d bytes)", file.Path, file.Bytes))
		}
	}
	if len(plan.Actions) > 0 {
		ui.Section("Actions")
	}
	for _, action := range plan.Actions {
		ui.Bullet(valueOrDash(action.Status), action.Title)
		if action.Command != "" {
			ui.Detail("Command", commandLine(action.Command, action.Args))
		}
		if action.Type == "mkdir" || action.Type == "write_file" {
			ui.Detail("File action", strings.TrimSpace(action.Type+" "+action.Path))
			if action.ContentBytes > 0 {
				ui.Detail("Content bytes", fmt.Sprint(action.ContentBytes))
			}
		}
		if action.ManualSteps != "" {
			ui.Detail("Manual", action.ManualSteps)
		}
	}
	ui.NextSteps("Use apply --dry-run to evaluate executor policy.", "Use --json for stable action contracts.")
}

func printServiceList(report *service.ListReport) {
	ui := newPresenter()
	ui.Header("Envdoctor Service List", "read-only native manager inspection")
	ui.Progress(1, 3, "Detect service manager")
	ui.Progress(2, 3, "Read service inventory")
	ui.Progress(3, 3, "Summarize availability")
	ui.Section("Summary")
	ui.Row("Service manager", report.Manager)
	ui.StatusRow("Status", report.Status, "service discovery")
	if report.Message != "" {
		ui.Row("Message", report.Message)
	}
	ui.Row("Services", fmt.Sprint(len(report.Services)))
	if len(report.Services) == 0 {
		ui.NextSteps("Run envdoctor service status <name> when you know the service name.")
		return
	}
	ui.Section("Services")
	fmt.Printf("%-48s %-14s %s\n", "Service", "State", "Description")
	for _, svc := range report.Services {
		fmt.Printf("%-48s %-14s %s\n", svc.Name, valueOrDash(svc.State), svc.Description)
	}
	ui.NextSteps("Run envdoctor service diagnose <name> for focused guidance.")
}

func printServiceInfo(info *service.ServiceInfo) {
	ui := newPresenter()
	ui.Header("Envdoctor Service", "read-only service status")
	ui.Progress(1, 3, "Select manager")
	ui.Progress(2, 3, "Read service state")
	ui.Progress(3, 3, "Prepare guidance")
	ui.Section("Summary")
	ui.Row("Service", info.Name)
	ui.Row("Manager", info.Manager)
	ui.Row("Platform", info.Platform)
	ui.StatusRow("Status", info.Status, "service status")
	if info.State != "" {
		ui.Row("State", info.State)
	}
	if info.Description != "" {
		ui.Row("Description", info.Description)
	}
	if info.Recommendation != "" {
		ui.Row("Recommendation", info.Recommendation)
	}
	ui.NextSteps("Run envdoctor service plan restart <name> to preview structured service actions.")
}

func printServicePlan(report *service.PlanReport) {
	ui := newPresenter()
	ui.Header("Envdoctor Service Plan", "safe service action preview")
	ui.Progress(1, 3, "Validate service request")
	ui.Progress(2, 3, "Build native manager actions")
	ui.Progress(3, 3, "Summarize policy status")
	ui.Section("Summary")
	ui.Row("Result", report.Summary)
	ui.Row("Service", report.Service)
	ui.Row("Operation", report.Operation)
	ui.Row("Manager", report.Manager)
	ui.StatusRow("Status", report.Status, "service operation preview")
	ui.Section("Actions")
	for _, action := range report.Actions {
		ui.Bullet(action.Status, action.Title)
		if action.Command != "" {
			ui.Detail("Command", commandLine(action.Command, action.Args))
		}
		if action.ManualSteps != "" {
			ui.Detail("Manual", action.ManualSteps)
		}
		if action.RollbackHint != "" {
			ui.Detail("Rollback hint", action.RollbackHint)
		}
	}
	ui.NextSteps("Use service apply --dry-run to evaluate policy.", "Production profile blocks service mutation in v1.")
}

func printVersionScan(report *versionpkg.ScanReport) {
	ui := newPresenter()
	ui.Header("Envdoctor Version Scan", "runtime and version-manager review")
	ui.Progress(1, 3, "Detect version managers")
	ui.Progress(2, 3, "Read project requirements")
	ui.Progress(3, 3, "Compare active runtimes")
	ui.Section("Summary")
	ui.Row("Result", report.Summary)
	ui.Row("Directory", report.Directory)
	ui.Row("Managers", fmt.Sprint(len(report.Managers)))
	ui.Row("Runtimes", fmt.Sprint(len(report.Runtimes)))
	ui.Row("Requirements", fmt.Sprint(len(report.Requirements)))
	ui.Row("Items needing review", fmt.Sprint(len(report.Mismatches)))
	ui.Section("Version managers")
	for _, manager := range report.Managers {
		ui.Bullet(foundStatus(manager.Found), manager.Name)
		ui.Detail("Version", valueOrDash(manager.Version))
		ui.Detail("Source", valueOrDash(manager.Source))
	}
	ui.Section("Runtimes")
	for _, runtimeInfo := range report.Runtimes {
		ui.Bullet(foundStatus(runtimeInfo.Found), runtimeInfo.Name)
		ui.Detail("Version", valueOrDash(runtimeInfo.Version))
		ui.Detail("Manager", valueOrDash(runtimeInfo.Manager))
	}
	if len(report.Requirements) > 0 {
		ui.Section("Project requirements")
		for _, req := range report.Requirements {
			ui.Bullet("required", req.Runtime+" "+req.Version)
			ui.Detail("Source", req.SourceFile)
		}
	}
	if len(report.Mismatches) > 0 {
		ui.Section("Items needing review")
		for _, mismatch := range report.Mismatches {
			ui.Bullet(mismatch.Status, mismatch.Runtime)
			ui.Detail("Required", mismatch.Required)
			ui.Detail("Active", valueOrDash(mismatch.Active))
			ui.Detail("Source", mismatch.SourceFile)
		}
	}
	ui.NextSteps("Run envdoctor version plan for non-mutating switch/install suggestions.", "Use --json for stable version scan output.")
}

func printVersionPlan(report *versionpkg.PlanReport) {
	ui := newPresenter()
	ui.Header("Envdoctor Version Plan", "runtime action suggestions")
	ui.Progress(1, 3, "Read version scan")
	ui.Progress(2, 3, "Build action candidates")
	ui.Progress(3, 3, "Summarize risk")
	ui.Section("Summary")
	ui.Row("Result", report.Summary)
	ui.Row("Actions", fmt.Sprint(len(report.Actions)))
	ui.Section("Actions")
	for _, action := range report.Actions {
		ui.Bullet(valueOrDash(action.Risk), action.Title)
		if action.Command != "" {
			ui.Detail("Suggested command", action.Command)
		}
		if action.ManualSteps != "" {
			ui.Detail("Manual", action.ManualSteps)
		}
	}
	ui.NextSteps("Use version apply --dry-run to evaluate executor policy.", "No runtime switch happens from plan output.")
}

func printInstallPlan(plan *installplan.Plan) {
	ui := newPresenter()
	ui.Header("Envdoctor Install Plan", "package-manager advisor")
	ui.Progress(1, 3, "Detect platform")
	ui.Progress(2, 3, "Resolve package manager")
	ui.Progress(3, 3, "Build advisory action")
	ui.Section("Summary")
	ui.Row("Result", plan.Summary)
	action := plan.Action
	ui.Row("Tool", action.Tool)
	ui.Row("Manager", valueOrDash(action.Manager))
	ui.Row("Risk", action.Risk)
	ui.Row("Requires admin", fmt.Sprint(action.RequiresAdmin))
	ui.Row("Safe to run", fmt.Sprint(action.SafeToRun))
	if action.Command != "" {
		ui.Detail("Suggested command", commandLine(action.Command, action.Args))
	}
	if action.ManualSteps != "" {
		ui.Detail("Manual", action.ManualSteps)
	}
	ui.NextSteps("Run envdoctor install apply <tool> --dry-run to inspect policy.", "Envdoctor does not auto-install tools from plan output.")
}

func printFixPlan(report *fixplan.Report) {
	ui := newPresenter()
	ui.Header("Envdoctor Fix Plan", "safe repair action candidates")
	ui.Progress(1, 3, "Collect recommendations")
	ui.Progress(2, 3, "Scan project and runtime signals")
	ui.Progress(3, 3, "Build action list")
	ui.Section("Summary")
	ui.Row("Result", report.Summary)
	ui.Row("Platform", report.Platform)
	ui.Row("Actions", fmt.Sprint(len(report.Actions)))
	ui.Section("Actions")
	for _, action := range report.Actions {
		ui.Bullet(action.Category, action.Title)
		ui.Detail("Status", action.Status)
		ui.Detail("Risk", action.Risk)
		if action.Command != "" {
			ui.Detail("Suggested command", action.Command)
		}
		if action.ManualSteps != "" {
			ui.Detail("Manual", action.ManualSteps)
		}
	}
	ui.NextSteps("Run envdoctor fix apply --dry-run before considering --yes.", "Use production profile to block mutation by default.")
}

func printBootstrapPlan(plan *bootstrap.Plan) {
	ui := newPresenter()
	ui.Header("Envdoctor Bootstrap Plan", "project onboarding setup preview")
	ui.Progress(1, 3, "Read project metadata")
	ui.Progress(2, 3, "Infer setup needs")
	ui.Progress(3, 3, "Build setup actions")
	ui.Section("Summary")
	ui.Row("Result", plan.Summary)
	ui.Row("Directory", plan.Directory)
	ui.Row("Dependencies", fmt.Sprint(len(plan.Dependencies)))
	ui.Row("Install plans", fmt.Sprint(len(plan.InstallPlans)))
	ui.Row("Service hints", fmt.Sprint(len(plan.ServiceHints)))
	ui.Row("Actions", fmt.Sprint(len(plan.Actions)))
	if len(plan.ServiceHints) > 0 {
		ui.Section("Service hints")
		for _, hint := range plan.ServiceHints {
			ui.Bullet("hint", hint.Name)
			ui.Detail("Source", hint.SourceFile)
			ui.Detail("Detail", hint.Description)
		}
	}
	if len(plan.Actions) > 0 {
		ui.Section("Actions")
		for _, action := range plan.Actions {
			ui.Bullet(action.Category, action.Title)
			if action.Command != "" {
				ui.Detail("Suggested command", action.Command)
			}
			if action.ManualSteps != "" {
				ui.Detail("Manual", action.ManualSteps)
			}
		}
	}
	ui.NextSteps("Run envdoctor bootstrap apply --dry-run to inspect allowlisted setup actions.", "No project setup action runs from plan output.")
}

func valueOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func commandLine(command string, args []string) string {
	if command == "" {
		return ""
	}
	return strings.Join(append([]string{command}, args...), " ")
}

func foundStatus(found bool) string {
	if found {
		return "available"
	}
	return "not found"
}
