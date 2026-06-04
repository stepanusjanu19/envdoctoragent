package main

import (
	"fmt"
	"os"
	"time"

	"envdoctor/internal/analyzer"
	"envdoctor/internal/container"
	"envdoctor/internal/dependencies"
	"envdoctor/internal/diagnose"
	"envdoctor/internal/dockerize"
	"envdoctor/internal/recommendation"
	"envdoctor/internal/scanner"
	"envdoctor/internal/snapshot"
	"envdoctor/internal/system"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "envdoctor",
		Short: "Environment Doctor Agent - detect, analyze, and resolve dev env issues",
		Long: `An intelligent cross-platform environment diagnostic tool.
Supports Linux, Windows, and macOS.`,
	}

	// system command
	var systemCmd = &cobra.Command{
		Use:   "system",
		Short: "Display system information",
		Run: func(cmd *cobra.Command, args []string) {
			info, err := system.Detect()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error detecting system: %v\n", err)
				os.Exit(1)
			}
			system.Print(info)
		},
	}

	// scan command
	var scanCmd = &cobra.Command{
		Use:   "scan",
		Short: "Scan various aspects of the environment",
	}

	// scan toolchain
	var scanToolchainCmd = &cobra.Command{
		Use:   "toolchain",
		Short: "Detect installed development tools",
		Run: func(cmd *cobra.Command, args []string) {
			tools, err := scanner.ScanToolchain()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error scanning toolchain: %v\n", err)
				os.Exit(1)
			}
			scanner.PrintToolchain(tools)
		},
	}

	// scan path
	var scanPathCmd = &cobra.Command{
		Use:   "path",
		Short: "Analyze PATH environment variable",
		Run: func(cmd *cobra.Command, args []string) {
			report, err := scanner.ScanPath()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error scanning PATH: %v\n", err)
				os.Exit(1)
			}
			scanner.PrintPathReport(report)
		},
	}

	// scan container
	var scanContainerCmd = &cobra.Command{
		Use:   "container",
		Short: "Validate container environments",
		Run: func(cmd *cobra.Command, args []string) {
			info, err := container.CheckContainerEnvironments()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error checking container environments: %v\n", err)
				os.Exit(1)
			}
			container.PrintContainerInfo(info)
		},
	}

	// scan dependencies
	var scanDependenciesCmd = &cobra.Command{
		Use:   "dependencies [directory]",
		Short: "Analyze project dependencies",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			deps, err := dependencies.AnalyzeDependencies(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error analyzing dependencies: %v\n", err)
				os.Exit(1)
			}
			dependencies.PrintDependencies(deps)
		},
	}

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
			analyzer.PrintAnalysis(result)
		},
	}

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
			
			// Check for --save flag
			save, _ := cmd.Flags().GetBool("save")
			if save {
				err := dockerize.SaveDockerfile(content, dir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error saving Dockerfile: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("Dockerfile saved successfully.")
			} else {
				fmt.Println(content)
			}
		},
	}
	dockerizeCmd.Flags().Bool("save", false, "Save the generated Dockerfile to disk")

	// snapshot command
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
			if save {
				filename := fmt.Sprintf("snapshot-%s.json", time.Now().Format("2006-01-02"))
				err := snapshot.SaveSnapshot(snap, filename)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error saving snapshot: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Snapshot saved to %s\n", filename)
			} else {
				snapshot.PrintSnapshot(snap)
			}
		},
	}
	snapshotCmd.Flags().Bool("save", false, "Save the snapshot to a file")

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
	var recommendCmd = &cobra.Command{
		Use:   "recommend",
		Short: "Generate actionable recommendations for detected environment issues",
		Run: func(cmd *cobra.Command, args []string) {
			report, err := recommendation.GenerateRecommendations()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating recommendations: %v\n", err)
				os.Exit(1)
			}
			recommendation.PrintReport(report)
		},
	}

	rootCmd.AddCommand(systemCmd, scanCmd, diagnoseCmd, snapshotCmd, explainCmd, compareCmd, dockerizeCmd, recommendCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}