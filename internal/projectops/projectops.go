package projectops

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stepanusjanu19/envdoctoragent/internal/dependencies"
	"github.com/stepanusjanu19/envdoctoragent/internal/executor"
)

const (
	OperationInit    = "init"
	OperationSync    = "sync"
	OperationInstall = "install"
	OperationUpdate  = "update"
	OperationRemove  = "remove"
)

// Options controls project lifecycle planning.
type Options struct {
	Ecosystem     string
	Manager       string
	Dev           bool
	Version       string
	All           bool
	AllowNonEmpty bool
}

// ScanReport summarizes project manifests for lifecycle operations.
type ScanReport struct {
	Directory  string                              `json:"directory"`
	Status     string                              `json:"status"`
	Ecosystems []string                            `json:"ecosystems"`
	Managers   []string                            `json:"package_managers"`
	Manifests  []*dependencies.ProjectDependencies `json:"manifests,omitempty"`
	Summary    string                              `json:"summary"`
}

// Plan is the project lifecycle plan output.
type Plan struct {
	Directory      string            `json:"directory"`
	Operation      string            `json:"operation"`
	Template       string            `json:"template,omitempty"`
	Ecosystem      string            `json:"ecosystem,omitempty"`
	PackageManager string            `json:"package_manager,omitempty"`
	Package        string            `json:"package,omitempty"`
	Actions        []executor.Action `json:"actions"`
	Status         string            `json:"status"`
	Summary        string            `json:"summary"`
}

type projectTarget struct {
	Ecosystem      string
	PackageManager string
	Manifest       string
	Report         *dependencies.ProjectDependencies
}

type templateDef struct {
	Template       string
	Ecosystem      string
	PackageManager string
	Actions        func(string) []executor.Action
}

// Scan detects project manifests without failing on empty/new projects.
func Scan(dir string) (*ScanReport, error) {
	absDir, err := existingDirectory(dir)
	if err != nil {
		return nil, err
	}
	reports, err := dependencies.AnalyzeAllDependencies(absDir)
	if err != nil {
		status := "not detected"
		if isDirectoryEmpty(absDir) {
			status = "empty"
		}
		return &ScanReport{
			Directory: absDir,
			Status:    status,
			Summary:   fmt.Sprintf("No supported project manifest detected in %s", absDir),
		}, nil
	}

	ecosystems := uniqueReports(reports, func(report *dependencies.ProjectDependencies) string { return report.Ecosystem })
	managers := uniqueReports(reports, func(report *dependencies.ProjectDependencies) string { return report.PackageManager })
	return &ScanReport{
		Directory:  absDir,
		Status:     "detected",
		Ecosystems: ecosystems,
		Managers:   managers,
		Manifests:  reports,
		Summary:    fmt.Sprintf("Detected %d project manifest(s) across %d ecosystem(s)", len(reports), len(ecosystems)),
	}, nil
}

// GenerateInitPlan creates a project initialization plan for an existing directory.
func GenerateInitPlan(template, dir string, options Options) (*Plan, error) {
	template = normalize(template)
	def, ok := templates()[template]
	if !ok {
		return nil, fmt.Errorf("unsupported project init template: %s", template)
	}
	absDir, err := existingDirectory(dir)
	if err != nil {
		return nil, err
	}
	if !options.AllowNonEmpty && !isDirectoryEmpty(absDir) {
		return nil, fmt.Errorf("project init is blocked for non-empty directory %s; use --allow-non-empty to override", absDir)
	}

	actions := def.Actions(absDir)
	for i := range actions {
		actions[i] = finalizeAction(actions[i], absDir, OperationInit, def.Ecosystem, def.PackageManager, nil, true, true)
	}
	return &Plan{
		Directory:      absDir,
		Operation:      OperationInit,
		Template:       template,
		Ecosystem:      def.Ecosystem,
		PackageManager: def.PackageManager,
		Actions:        actions,
		Status:         "safe-execution-preview",
		Summary:        fmt.Sprintf("Generated %d project init action(s) for %s. No commands were executed.", len(actions), template),
	}, nil
}

// GenerateDepsPlan creates dependency lifecycle actions for an existing project.
func GenerateDepsPlan(operation, packageName, dir string, options Options) (*Plan, error) {
	operation = normalize(operation)
	if !validDependencyOperation(operation) {
		return nil, fmt.Errorf("unsupported dependency operation: %s", operation)
	}
	if (operation == OperationInstall || operation == OperationRemove) && strings.TrimSpace(packageName) == "" {
		return nil, fmt.Errorf("project deps %s requires a package name", operation)
	}
	if operation == OperationUpdate && strings.TrimSpace(packageName) == "" && !options.All {
		return nil, fmt.Errorf("project deps update requires a package name or --all")
	}

	target, err := resolveTarget(dir, options)
	if err != nil {
		return nil, err
	}
	actions := dependencyActions(operation, packageName, target, options)
	for i := range actions {
		actions[i] = finalizeAction(actions[i], targetDir(dir), operation, target.Ecosystem, target.PackageManager, packageList(packageName), false, true)
	}
	return &Plan{
		Directory:      targetDir(dir),
		Operation:      operation,
		Ecosystem:      target.Ecosystem,
		PackageManager: target.PackageManager,
		Package:        packageName,
		Actions:        actions,
		Status:         "safe-execution-preview",
		Summary:        fmt.Sprintf("Generated %d project dependency action(s) for %s/%s. No commands were executed.", len(actions), target.Ecosystem, operation),
	}, nil
}

func templates() map[string]templateDef {
	return map[string]templateDef{
		"node":       {Template: "node", Ecosystem: "node", PackageManager: "npm", Actions: simpleInit("npm", "init", "-y")},
		"react-vite": {Template: "react-vite", Ecosystem: "node", PackageManager: "npm", Actions: simpleInit("npm", "create", "vite@latest", ".", "--", "--template", "react")},
		"next":       {Template: "next", Ecosystem: "node", PackageManager: "npm", Actions: simpleInit("npx", "create-next-app@latest", ".")},
		"vue-vite":   {Template: "vue-vite", Ecosystem: "node", PackageManager: "npm", Actions: simpleInit("npm", "create", "vite@latest", ".", "--", "--template", "vue")},
		"sveltekit":  {Template: "sveltekit", Ecosystem: "node", PackageManager: "npm", Actions: simpleInit("npm", "create", "svelte@latest", ".")},
		"nestjs":     {Template: "nestjs", Ecosystem: "node", PackageManager: "npm", Actions: simpleInit("npx", "@nestjs/cli", "new", ".")},
		"python":     {Template: "python", Ecosystem: "python", PackageManager: "pip", Actions: simpleInit("python", "-m", "venv", ".venv")},
		"go": {Template: "go", Ecosystem: "go", PackageManager: "go modules", Actions: func(dir string) []executor.Action {
			return []executor.Action{commandAction("Initialize Go module", "go", []string{"mod", "init", "example.com/" + slug(filepath.Base(dir))})}
		}},
		"rust":       {Template: "rust", Ecosystem: "rust", PackageManager: "cargo", Actions: simpleInit("cargo", "init", ".")},
		"php":        {Template: "php", Ecosystem: "php", PackageManager: "composer", Actions: composerInit},
		"java-maven": {Template: "java-maven", Ecosystem: "jvm", PackageManager: "maven", Actions: manualInit("Initialize Maven project", "Run Maven archetype generation manually after choosing groupId, artifactId, and archetype.")},
		"java-gradle": {Template: "java-gradle", Ecosystem: "jvm", PackageManager: "gradle", Actions: func(dir string) []executor.Action {
			return []executor.Action{commandAction("Initialize Gradle Java application", "gradle", []string{"init", "--type", "java-application", "--dsl", "groovy", "--test-framework", "junit", "--project-name", slug(filepath.Base(dir))})}
		}},
		"dotnet":  {Template: "dotnet", Ecosystem: "dotnet", PackageManager: "dotnet", Actions: simpleInit("dotnet", "new", "console")},
		"dart":    {Template: "dart", Ecosystem: "dart", PackageManager: "pub", Actions: simpleInit("dart", "create", ".")},
		"flutter": {Template: "flutter", Ecosystem: "dart", PackageManager: "flutter", Actions: simpleInit("flutter", "create", ".")},
		"swift":   {Template: "swift", Ecosystem: "swift", PackageManager: "swiftpm", Actions: simpleInit("swift", "package", "init", "--type", "executable")},
		"elixir":  {Template: "elixir", Ecosystem: "elixir", PackageManager: "mix", Actions: simpleInit("mix", "new", ".")},
		"express": {Template: "express", Ecosystem: "node", PackageManager: "npm", Actions: func(string) []executor.Action {
			return []executor.Action{commandAction("Initialize Node.js project", "npm", []string{"init", "-y"}), commandAction("Install Express", "npm", []string{"install", "express"})}
		}},
	}
}

func dependencyActions(operation, packageName string, target projectTarget, options Options) []executor.Action {
	manager := normalize(valueOrDefault(options.Manager, target.PackageManager))
	pkg := packageWithVersion(packageName, options.Version, target.Ecosystem)
	devFlag := options.Dev

	switch target.Ecosystem {
	case "node":
		return nodeActions(operation, pkg, manager, devFlag, options.All)
	case "python":
		return pythonActions(operation, pkg, target.Manifest)
	case "go":
		return goActions(operation, pkg, options.All)
	case "rust":
		return rustActions(operation, pkg, devFlag, options.All)
	case "php":
		return composerActions(operation, pkg, devFlag, options.All)
	case "dotnet":
		return dotnetActions(operation, packageName, options.Version)
	case "dart":
		return dartActions(operation, pkg, devFlag, options.All)
	case "ruby":
		return rubyActions(operation, pkg, options.Version, options.All)
	case "swift":
		if operation == OperationSync {
			return []executor.Action{commandAction("Resolve Swift packages", "swift", []string{"package", "resolve"})}
		}
	case "elixir":
		if operation == OperationSync {
			return []executor.Action{commandAction("Fetch Elixir dependencies", "mix", []string{"deps.get"})}
		}
	case "jvm":
		if operation == OperationSync {
			if manager == "gradle" {
				return []executor.Action{commandAction("Review Gradle dependencies", "gradle", []string{"dependencies"})}
			}
			return []executor.Action{commandAction("Resolve Maven dependencies", "mvn", []string{"dependency:resolve"})}
		}
	}
	return []executor.Action{manualAction("Manual dependency operation required", fmt.Sprintf("Envdoctor does not have a safe %s command mapping for %s/%s yet.", operation, target.Ecosystem, manager))}
}

func nodeActions(operation, pkg, manager string, dev, all bool) []executor.Action {
	if manager == "" || manager == "package.json" {
		manager = "npm"
	}
	switch manager {
	case "yarn":
		return yarnActions(operation, pkg, dev, all)
	case "pnpm":
		return pnpmActions(operation, pkg, dev, all)
	case "bun":
		return bunActions(operation, pkg, dev, all)
	default:
		switch operation {
		case OperationSync:
			return []executor.Action{commandAction("Install Node.js dependencies", "npm", []string{"install"})}
		case OperationInstall:
			args := []string{"install"}
			if dev {
				args = append(args, "--save-dev")
			}
			return []executor.Action{commandAction("Install Node.js package", "npm", append(args, pkg))}
		case OperationUpdate:
			args := []string{"update"}
			if !all {
				args = append(args, pkg)
			}
			return []executor.Action{commandAction("Update Node.js package", "npm", args)}
		case OperationRemove:
			return []executor.Action{commandAction("Remove Node.js package", "npm", []string{"uninstall", pkg})}
		}
	}
	return nil
}

func yarnActions(operation, pkg string, dev, all bool) []executor.Action {
	switch operation {
	case OperationSync:
		return []executor.Action{commandAction("Install Yarn dependencies", "yarn", []string{"install"})}
	case OperationInstall:
		args := []string{"add"}
		if dev {
			args = append(args, "-D")
		}
		return []executor.Action{commandAction("Install Yarn package", "yarn", append(args, pkg))}
	case OperationUpdate:
		args := []string{"upgrade"}
		if !all {
			args = append(args, pkg)
		}
		return []executor.Action{commandAction("Update Yarn package", "yarn", args)}
	case OperationRemove:
		return []executor.Action{commandAction("Remove Yarn package", "yarn", []string{"remove", pkg})}
	}
	return nil
}

func pnpmActions(operation, pkg string, dev, all bool) []executor.Action {
	switch operation {
	case OperationSync:
		return []executor.Action{commandAction("Install pnpm dependencies", "pnpm", []string{"install"})}
	case OperationInstall:
		args := []string{"add"}
		if dev {
			args = append(args, "-D")
		}
		return []executor.Action{commandAction("Install pnpm package", "pnpm", append(args, pkg))}
	case OperationUpdate:
		args := []string{"update"}
		if !all {
			args = append(args, pkg)
		}
		return []executor.Action{commandAction("Update pnpm package", "pnpm", args)}
	case OperationRemove:
		return []executor.Action{commandAction("Remove pnpm package", "pnpm", []string{"remove", pkg})}
	}
	return nil
}

func bunActions(operation, pkg string, dev, all bool) []executor.Action {
	switch operation {
	case OperationSync:
		return []executor.Action{commandAction("Install Bun dependencies", "bun", []string{"install"})}
	case OperationInstall:
		args := []string{"add"}
		if dev {
			args = append(args, "-d")
		}
		return []executor.Action{commandAction("Install Bun package", "bun", append(args, pkg))}
	case OperationUpdate:
		args := []string{"update"}
		if !all {
			args = append(args, pkg)
		}
		return []executor.Action{commandAction("Update Bun package", "bun", args)}
	case OperationRemove:
		return []executor.Action{commandAction("Remove Bun package", "bun", []string{"remove", pkg})}
	}
	return nil
}

func pythonActions(operation, pkg, manifest string) []executor.Action {
	switch operation {
	case OperationSync:
		if manifest == "requirements.txt" {
			return []executor.Action{commandAction("Install Python requirements", "python", []string{"-m", "pip", "install", "-r", "requirements.txt"})}
		}
		return []executor.Action{commandAction("Install Python project", "python", []string{"-m", "pip", "install", "-e", "."})}
	case OperationInstall:
		return []executor.Action{commandAction("Install Python package", "python", []string{"-m", "pip", "install", pkg})}
	case OperationUpdate:
		return []executor.Action{commandAction("Update Python package", "python", []string{"-m", "pip", "install", "--upgrade", pkg})}
	case OperationRemove:
		return []executor.Action{commandAction("Remove Python package", "python", []string{"-m", "pip", "uninstall", "-y", pkg})}
	}
	return nil
}

func goActions(operation, pkg string, all bool) []executor.Action {
	switch operation {
	case OperationSync:
		return []executor.Action{commandAction("Download Go modules", "go", []string{"mod", "download"})}
	case OperationInstall:
		return []executor.Action{commandAction("Add Go module", "go", []string{"get", pkg})}
	case OperationUpdate:
		if all {
			pkg = "./..."
		}
		return []executor.Action{commandAction("Update Go module", "go", []string{"get", "-u", pkg})}
	case OperationRemove:
		return []executor.Action{commandAction("Remove Go module", "go", []string{"get", pkg + "@none"}), commandAction("Tidy Go modules", "go", []string{"mod", "tidy"})}
	}
	return nil
}

func rustActions(operation, pkg string, dev, all bool) []executor.Action {
	switch operation {
	case OperationSync:
		return []executor.Action{commandAction("Fetch Rust crates", "cargo", []string{"fetch"})}
	case OperationInstall:
		args := []string{"add", pkg}
		if dev {
			args = append(args, "--dev")
		}
		return []executor.Action{commandAction("Add Rust crate", "cargo", args)}
	case OperationUpdate:
		args := []string{"update"}
		if !all {
			args = append(args, pkg)
		}
		return []executor.Action{commandAction("Update Rust crate", "cargo", args)}
	case OperationRemove:
		return []executor.Action{commandAction("Remove Rust crate", "cargo", []string{"remove", pkg})}
	}
	return nil
}

func composerActions(operation, pkg string, dev, all bool) []executor.Action {
	switch operation {
	case OperationSync:
		return []executor.Action{commandAction("Install Composer dependencies", "composer", []string{"install"})}
	case OperationInstall:
		args := []string{"require"}
		if dev {
			args = append(args, "--dev")
		}
		return []executor.Action{commandAction("Require Composer package", "composer", append(args, pkg))}
	case OperationUpdate:
		args := []string{"update"}
		if !all {
			args = append(args, pkg)
		}
		return []executor.Action{commandAction("Update Composer package", "composer", args)}
	case OperationRemove:
		return []executor.Action{commandAction("Remove Composer package", "composer", []string{"remove", pkg})}
	}
	return nil
}

func dotnetActions(operation, pkg, version string) []executor.Action {
	switch operation {
	case OperationSync:
		return []executor.Action{commandAction("Restore .NET packages", "dotnet", []string{"restore"})}
	case OperationInstall, OperationUpdate:
		args := []string{"add", "package", pkg}
		if version != "" {
			args = append(args, "--version", version)
		}
		return []executor.Action{commandAction("Add .NET package", "dotnet", args)}
	case OperationRemove:
		return []executor.Action{commandAction("Remove .NET package", "dotnet", []string{"remove", "package", pkg})}
	}
	return nil
}

func dartActions(operation, pkg string, dev, all bool) []executor.Action {
	switch operation {
	case OperationSync:
		return []executor.Action{commandAction("Fetch Dart packages", "dart", []string{"pub", "get"})}
	case OperationInstall:
		if dev {
			pkg = "dev:" + pkg
		}
		return []executor.Action{commandAction("Add Dart package", "dart", []string{"pub", "add", pkg})}
	case OperationUpdate:
		args := []string{"pub", "upgrade"}
		if !all {
			args = append(args, pkg)
		}
		return []executor.Action{commandAction("Upgrade Dart package", "dart", args)}
	case OperationRemove:
		return []executor.Action{commandAction("Remove Dart package", "dart", []string{"pub", "remove", pkg})}
	}
	return nil
}

func rubyActions(operation, pkg, version string, all bool) []executor.Action {
	switch operation {
	case OperationSync:
		return []executor.Action{commandAction("Install Ruby gems", "bundle", []string{"install"})}
	case OperationInstall:
		args := []string{"add", pkg}
		if version != "" {
			args = append(args, "--version", version)
		}
		return []executor.Action{commandAction("Add Ruby gem", "bundle", args)}
	case OperationUpdate:
		args := []string{"update"}
		if !all {
			args = append(args, pkg)
		}
		return []executor.Action{commandAction("Update Ruby gem", "bundle", args)}
	case OperationRemove:
		return []executor.Action{commandAction("Remove Ruby gem", "bundle", []string{"remove", pkg})}
	}
	return nil
}

func resolveTarget(dir string, options Options) (projectTarget, error) {
	scan, err := Scan(dir)
	if err != nil {
		return projectTarget{}, err
	}
	if len(scan.Manifests) == 0 {
		return projectTarget{}, fmt.Errorf("no supported project manifest found in %s; choose project init first", scan.Directory)
	}
	var candidates []*dependencies.ProjectDependencies
	for _, report := range scan.Manifests {
		if options.Ecosystem == "" || normalize(report.Ecosystem) == normalize(options.Ecosystem) {
			candidates = append(candidates, report)
		}
	}
	if len(candidates) == 0 {
		return projectTarget{}, fmt.Errorf("ecosystem %q was not detected in %s", options.Ecosystem, scan.Directory)
	}
	if options.Ecosystem == "" && len(uniqueReports(candidates, func(report *dependencies.ProjectDependencies) string { return report.Ecosystem })) > 1 {
		return projectTarget{}, fmt.Errorf("multiple ecosystems detected; pass --ecosystem to select one")
	}
	selected := candidates[0]
	manager := selected.PackageManager
	if options.Manager != "" {
		manager = options.Manager
	}
	return projectTarget{Ecosystem: selected.Ecosystem, PackageManager: manager, Manifest: selected.ManifestType, Report: selected}, nil
}

func finalizeAction(action executor.Action, dir, operation, ecosystem, manager string, packages []string, createsProject, mutatesProject bool) executor.Action {
	if action.ID == "" {
		action.ID = fmt.Sprintf("project-%s-%s-%s", operation, ecosystem, slug(action.Title))
	}
	action.Source = valueOrDefault(action.Source, "projectops")
	action.Category = valueOrDefault(action.Category, "Project")
	action.Operation = operation
	action.Ecosystem = ecosystem
	action.PackageManager = manager
	action.Packages = packages
	action.WorkingDir = dir
	action.Risk = valueOrDefault(action.Risk, "medium")
	action.SafeToRun = false
	action.CreatesProject = createsProject
	action.MutatesProject = mutatesProject
	action.Status = valueOrDefault(action.Status, "plan-only")
	action.Timeout = valueOrDefault(action.Timeout, "5m")
	if action.RollbackHint == "" {
		action.RollbackHint = "Use the project snapshot and version control to inspect and manually revert manifest or lockfile changes."
	}
	return action
}

func simpleInit(command string, args ...string) func(string) []executor.Action {
	return func(string) []executor.Action {
		return []executor.Action{commandAction("Initialize project", command, args)}
	}
}

func manualInit(title, steps string) func(string) []executor.Action {
	return func(string) []executor.Action {
		return []executor.Action{manualAction(title, steps)}
	}
}

func composerInit(dir string) []executor.Action {
	name := "envdoctor/" + slug(filepath.Base(dir))
	return []executor.Action{commandAction("Initialize Composer project", "composer", []string{"init", "--no-interaction", "--name", name})}
}

func commandAction(title, command string, args []string) executor.Action {
	return executor.Action{Title: title, Command: command, Args: args, Source: "projectops"}
}

func manualAction(title, steps string) executor.Action {
	return executor.Action{Title: title, ManualSteps: steps, Source: "projectops", Status: "metadata-only", Risk: "low"}
}

func existingDirectory(dir string) (string, error) {
	if dir == "" {
		dir = "."
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", absDir)
	}
	return absDir, nil
}

func targetDir(dir string) string {
	abs, err := filepath.Abs(valueOrDefault(dir, "."))
	if err != nil {
		return dir
	}
	return abs
}

func isDirectoryEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() == ".envdoctor" {
			continue
		}
		return false
	}
	return true
}

func validDependencyOperation(operation string) bool {
	switch operation {
	case OperationSync, OperationInstall, OperationUpdate, OperationRemove:
		return true
	default:
		return false
	}
}

func packageWithVersion(pkg, version, ecosystem string) string {
	pkg = strings.TrimSpace(pkg)
	version = strings.TrimSpace(version)
	if pkg == "" || version == "" {
		return pkg
	}
	switch ecosystem {
	case "node":
		return pkg + "@" + version
	case "python":
		return pkg + "==" + version
	case "php":
		return pkg + ":" + version
	default:
		return pkg
	}
}

func packageList(pkg string) []string {
	if strings.TrimSpace(pkg) == "" {
		return nil
	}
	return []string{strings.TrimSpace(pkg)}
}

func uniqueReports(reports []*dependencies.ProjectDependencies, value func(*dependencies.ProjectDependencies) string) []string {
	seen := map[string]bool{}
	var result []string
	for _, report := range reports {
		item := strings.TrimSpace(value(report))
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
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
		return "project"
	}
	return result
}
