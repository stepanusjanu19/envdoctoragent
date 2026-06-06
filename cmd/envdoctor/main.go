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
	"github.com/stepanusjanu19/envdoctoragent/internal/recommendation"
	"github.com/stepanusjanu19/envdoctoragent/internal/scaffold"
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
			options, err := executionOptions(cmd, projectInitExecutionFlags, plan.Directory)
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

	rootCmd.AddCommand(systemCmd, scanCmd, diagnoseCmd, snapshotCmd, explainCmd, compareCmd, dockerizeCmd, recommendCmd, serviceCmd, versionCmd, installCmd, fixCmd, bootstrapCmd, projectCmd, agentCmd, uiCmd)

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
	fmt.Println(report.Summary)
	fmt.Printf("Mode: %s\n", report.Mode)
	fmt.Printf("Profile: %s\n", report.Profile)
	fmt.Printf("Max risk: %s\n", report.MaxRisk)
	if report.PolicyFile != "" {
		fmt.Printf("Policy file: %s\n", report.PolicyFile)
	}
	fmt.Printf("Audit log: %s\n", report.AuditLog)
	if report.SnapshotFile != "" {
		fmt.Printf("Pre-apply snapshot: %s\n", report.SnapshotFile)
	}
	if report.ProjectSnapshotFile != "" {
		fmt.Printf("Project snapshot: %s\n", report.ProjectSnapshotFile)
	}
	if len(report.PolicyDecisions) > 0 {
		fmt.Println("Policy decisions:")
		for _, decision := range report.PolicyDecisions {
			if decision.Allowed {
				fmt.Printf("  - %s: allowed (%s)\n", decision.ActionID, decision.Risk)
			} else {
				fmt.Printf("  - %s: blocked (%s)\n", decision.ActionID, decision.Reason)
			}
		}
	}
	if len(report.ProjectChanges) > 0 {
		fmt.Println("Project changes:")
		for _, change := range report.ProjectChanges {
			fmt.Printf("  - %s\n", change)
		}
	}
	if len(report.Results) == 0 {
		fmt.Println("No executable actions were generated.")
		return
	}
	for _, result := range report.Results {
		fmt.Printf("- [%s] %s\n", result.Status, result.Action.Title)
		if result.Action.SuggestedCommand != "" {
			fmt.Printf("  Suggested command: %s\n", result.Action.SuggestedCommand)
		} else if result.Action.Command != "" {
			fmt.Printf("  Command: %s\n", strings.Join(append([]string{result.Action.Command}, result.Action.Args...), " "))
		}
		if result.Action.Type == "mkdir" || result.Action.Type == "write_file" {
			fmt.Printf("  File action: %s %s\n", result.Action.Type, result.Action.Path)
			if result.Action.ContentBytes > 0 {
				fmt.Printf("  Content bytes: %d\n", result.Action.ContentBytes)
			}
		}
		if result.Action.ManualSteps != "" {
			fmt.Printf("  Manual steps: %s\n", result.Action.ManualSteps)
		}
		if result.Error != "" {
			fmt.Printf("  Error: %s\n", result.Error)
		}
		if result.Message != "" {
			fmt.Printf("  Message: %s\n", result.Message)
		}
		if result.Action.RollbackHint != "" {
			fmt.Printf("  Rollback hint: %s\n", result.Action.RollbackHint)
		}
	}
}

func printAgentReport(report *agent.Report) {
	fmt.Println(report.Summary)
	fmt.Printf("Goal: %s\n", report.Goal)
	fmt.Printf("Profile: %s\n", report.Profile)
	fmt.Printf("Max risk: %s\n", report.MaxRisk)
	fmt.Printf("Directory: %s\n", report.Directory)
	fmt.Printf("Status: %s\n", report.Status)
	if report.Diagnostics != nil {
		fmt.Println("Diagnostics: available")
	}
	if report.ProjectScan != nil {
		fmt.Printf("Project scan: %s\n", report.ProjectScan.Summary)
	}
	if report.FixPlan != nil {
		fmt.Printf("Fix plan: %s\n", report.FixPlan.Summary)
	}
	if report.BootstrapPlan != nil {
		fmt.Printf("Bootstrap plan: %s\n", report.BootstrapPlan.Summary)
	}
	if report.ProjectPlan != nil {
		fmt.Printf("Project plan: %s\n", report.ProjectPlan.Summary)
	}
	if len(report.Actions) > 0 {
		fmt.Println("Actions:")
		for _, action := range report.Actions {
			fmt.Printf("- [%s] %s\n", valueOrDash(action.Category), action.Title)
			if action.Command != "" {
				fmt.Printf("  Command: %s\n", strings.Join(append([]string{action.Command}, action.Args...), " "))
			}
			if action.Path != "" {
				fmt.Printf("  File action: %s %s\n", action.Type, action.Path)
			}
			if action.ManualSteps != "" {
				fmt.Printf("  Manual steps: %s\n", action.ManualSteps)
			}
		}
	}
	if report.Execution != nil {
		fmt.Println()
		printExecutionReport(report.Execution)
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
	fmt.Println(report.Summary)
	fmt.Printf("Directory: %s\n", report.Directory)
	fmt.Printf("Status: %s\n", report.Status)
	if len(report.Ecosystems) > 0 {
		fmt.Printf("Ecosystems: %s\n", strings.Join(report.Ecosystems, ", "))
	}
	if len(report.Managers) > 0 {
		fmt.Printf("Package managers: %s\n", strings.Join(report.Managers, ", "))
	}
	for _, manifest := range report.Manifests {
		fmt.Printf("- %s (%s/%s, %s)\n", manifest.SourceFile, manifest.Ecosystem, manifest.PackageManager, manifest.ValidationStatus)
	}
}

func printProjectTemplates(templates []scaffold.Template) {
	if len(templates) == 0 {
		fmt.Println("No scaffold templates are registered.")
		return
	}
	fmt.Printf("%-18s %-12s %-18s %-10s %s\n", "Template", "Language", "Framework", "Source", "Summary")
	for _, template := range templates {
		fmt.Printf("%-18s %-12s %-18s %-10s %s\n", template.ID, valueOrDash(template.Language), valueOrDash(template.Framework), valueOrDash(template.Source), template.Summary)
	}
}

func printProjectPlan(plan *projectops.Plan) {
	fmt.Println(plan.Summary)
	fmt.Printf("Directory: %s\n", plan.Directory)
	fmt.Printf("Operation: %s\n", plan.Operation)
	if plan.Template != "" {
		fmt.Printf("Template: %s\n", plan.Template)
	}
	if plan.Source != "" {
		fmt.Printf("Source: %s\n", plan.Source)
	}
	if plan.Ecosystem != "" {
		fmt.Printf("Ecosystem: %s\n", plan.Ecosystem)
	}
	if plan.PackageManager != "" {
		fmt.Printf("Package manager: %s\n", plan.PackageManager)
	}
	if plan.RequiresNetwork {
		fmt.Println("Requires network: yes")
	}
	if len(plan.Files) > 0 {
		fmt.Println("Files:")
		for _, file := range plan.Files {
			fmt.Printf("  - %s (%d bytes)\n", file.Path, file.Bytes)
		}
	}
	for _, action := range plan.Actions {
		fmt.Printf("- [%s] %s\n", valueOrDash(action.Status), action.Title)
		if action.Command != "" {
			fmt.Printf("  Command: %s\n", strings.Join(append([]string{action.Command}, action.Args...), " "))
		}
		if action.Type == "mkdir" || action.Type == "write_file" {
			fmt.Printf("  File action: %s %s\n", action.Type, action.Path)
			if action.ContentBytes > 0 {
				fmt.Printf("  Content bytes: %d\n", action.ContentBytes)
			}
		}
		if action.ManualSteps != "" {
			fmt.Printf("  Manual steps: %s\n", action.ManualSteps)
		}
	}
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

func printServicePlan(report *service.PlanReport) {
	fmt.Println(report.Summary)
	fmt.Printf("Service: %s\n", report.Service)
	fmt.Printf("Operation: %s\n", report.Operation)
	fmt.Printf("Manager: %s\n", report.Manager)
	fmt.Printf("Status: %s\n", report.Status)
	for _, action := range report.Actions {
		fmt.Printf("- [%s] %s\n", action.Status, action.Title)
		if action.Command != "" {
			fmt.Printf("  Command: %s\n", strings.Join(append([]string{action.Command}, action.Args...), " "))
		}
		if action.ManualSteps != "" {
			fmt.Printf("  Manual steps: %s\n", action.ManualSteps)
		}
		if action.RollbackHint != "" {
			fmt.Printf("  Rollback hint: %s\n", action.RollbackHint)
		}
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
		fmt.Printf("Suggested command: %s\n", strings.Join(append([]string{action.Command}, action.Args...), " "))
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
