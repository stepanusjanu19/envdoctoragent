package diagnose

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/stepanusjanu19/envdoctoragent/internal/common"
	"github.com/stepanusjanu19/envdoctoragent/internal/container"
	"github.com/stepanusjanu19/envdoctoragent/internal/recommendation"
	"github.com/stepanusjanu19/envdoctoragent/internal/scanner"
	"github.com/stepanusjanu19/envdoctoragent/internal/system"
)

type Report struct {
	SystemInfo      *common.SystemInfo              `json:"system"`
	Toolchain       []common.ToolInfo               `json:"toolchain"`
	PathReport      *common.PathReport              `json:"path"`
	ContainerInfo   *container.ContainerInfo        `json:"container,omitempty"`
	Recommendations []recommendation.Recommendation `json:"recommendations,omitempty"`
}

func Run() (*Report, error) {
	report := &Report{}

	sysInfo, err := system.Detect()
	if err != nil {
		return nil, fmt.Errorf("failed to detect system: %v", err)
	}
	report.SystemInfo = sysInfo

	tools, err := scanner.ScanToolchain()
	if err != nil {
		return report, fmt.Errorf("failed to scan toolchain: %v", err)
	}
	report.Toolchain = tools

	pathReport, err := scanner.ScanPath()
	if err != nil {
		return report, fmt.Errorf("failed to scan path: %v", err)
	}
	report.PathReport = pathReport

	containerInfo, err := container.CheckContainerEnvironments()
	if err == nil {
		report.ContainerInfo = containerInfo
	}

	recReport := recommendation.GenerateRecommendationsFromScans(report.SystemInfo, report.Toolchain, report.PathReport, report.ContainerInfo)
	if recReport != nil {
		report.Recommendations = recReport.Recommendations
	}

	return report, nil
}

func Print(r *Report) {
	fmt.Println("=== System Information ===")
	if r.SystemInfo != nil {
		system.Print(r.SystemInfo)
	}

	fmt.Println()
	fmt.Println("=== Toolchain Scan ===")
	scanner.PrintToolchain(r.Toolchain)

	fmt.Println()
	fmt.Println("=== PATH Analysis ===")
	if r.PathReport != nil {
		scanner.PrintPathReport(r.PathReport)
	}

	fmt.Println()
	fmt.Println("=== Container Environment ===")
	if r.ContainerInfo != nil {
		container.PrintContainerInfo(r.ContainerInfo)
	} else {
		fmt.Println("  No container information available.")
	}

	fmt.Println()
	fmt.Println("=== Recommendations ===")
	if len(r.Recommendations) == 0 {
		fmt.Println("  No recommendations. Your environment looks healthy!")
	} else {
		for i, rec := range r.Recommendations {
			fmt.Printf("  %d. [%s] [%s] %s\n", i+1, rec.Severity, rec.Category, rec.Title)
			if rec.Command != "" {
				fmt.Printf("     Fix: %s\n", rec.Command)
			}
			if rec.ManualSteps != "" {
				fmt.Printf("     Manual: %s\n", rec.ManualSteps)
			}
		}
	}
}

func PrintJSON(r *Report) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling to JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
