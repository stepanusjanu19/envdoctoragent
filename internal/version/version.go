package version

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"envdoctor/internal/command"
)

// ManagerInfo describes a detected runtime version manager.
type ManagerInfo struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Source  string `json:"source,omitempty"`
}

// RuntimeInfo describes an installed runtime.
type RuntimeInfo struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Manager string `json:"manager,omitempty"`
}

// Requirement is a project runtime version requirement.
type Requirement struct {
	Runtime    string `json:"runtime"`
	Version    string `json:"version"`
	SourceFile string `json:"source_file"`
	Source     string `json:"source"`
	Raw        string `json:"raw,omitempty"`
}

// Mismatch describes a runtime requirement that needs review.
type Mismatch struct {
	Runtime        string `json:"runtime"`
	Required       string `json:"required"`
	Active         string `json:"active,omitempty"`
	SourceFile     string `json:"source_file"`
	Status         string `json:"status"`
	Description    string `json:"description"`
	RequiresReview bool   `json:"requires_review"`
}

// ScanReport is the read-only output of version scan.
type ScanReport struct {
	Directory    string        `json:"directory"`
	Managers     []ManagerInfo `json:"managers"`
	Runtimes     []RuntimeInfo `json:"runtimes"`
	Requirements []Requirement `json:"requirements"`
	Mismatches   []Mismatch    `json:"mismatches"`
	Summary      string        `json:"summary"`
}

// PlanAction is a non-mutating version manager suggestion.
type PlanAction struct {
	Runtime       string `json:"runtime"`
	Title         string `json:"title"`
	Command       string `json:"command,omitempty"`
	ManualSteps   string `json:"manual_steps,omitempty"`
	Risk          string `json:"risk"`
	RequiresAdmin bool   `json:"requires_admin"`
	Platform      string `json:"platform"`
	Manager       string `json:"manager,omitempty"`
	SafeToRun     bool   `json:"safe_to_run"`
}

// PlanReport contains version scan data and suggested actions.
type PlanReport struct {
	Scan    *ScanReport  `json:"scan"`
	Actions []PlanAction `json:"actions"`
	Summary string       `json:"summary"`
}

type managerDef struct {
	Name       string
	Command    string
	Args       []string
	EnvVar     string
	EnvSubPath string
}

type runtimeDef struct {
	Name    string
	Command string
	Args    []string
}

var managerDefs = []managerDef{
	{Name: "nvm", Command: "nvm", Args: []string{"--version"}, EnvVar: "NVM_DIR", EnvSubPath: "nvm.sh"},
	{Name: "fnm", Command: "fnm", Args: []string{"--version"}},
	{Name: "pyenv", Command: "pyenv", Args: []string{"--version"}},
	{Name: "rustup", Command: "rustup", Args: []string{"--version"}},
	{Name: "asdf", Command: "asdf", Args: []string{"--version"}},
	{Name: "sdkman", Command: "sdk", Args: []string{"version"}, EnvVar: "SDKMAN_DIR", EnvSubPath: filepath.Join("bin", "sdkman-init.sh")},
	{Name: "goenv", Command: "goenv", Args: []string{"--version"}},
}

var runtimeDefs = []runtimeDef{
	{Name: "node", Command: "node", Args: []string{"--version"}},
	{Name: "python", Command: "python3", Args: []string{"--version"}},
	{Name: "rust", Command: "rustc", Args: []string{"--version"}},
	{Name: "go", Command: "go", Args: []string{"version"}},
	{Name: "java", Command: "java", Args: []string{"-version"}},
}

// Scan detects version managers, installed runtimes, and project requirements.
func Scan(dir string) (*ScanReport, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	report := &ScanReport{
		Directory:    absDir,
		Managers:     detectManagers(),
		Runtimes:     detectRuntimes(),
		Requirements: detectRequirements(absDir),
	}
	report.Mismatches = detectMismatches(report.Requirements, report.Runtimes)
	report.Summary = summarizeScan(report)
	return report, nil
}

// Plan returns non-mutating install/switch suggestions for runtime requirements.
func Plan(dir string) (*PlanReport, error) {
	scan, err := Scan(dir)
	if err != nil {
		return nil, err
	}

	actions := planActions(scan)
	return &PlanReport{
		Scan:    scan,
		Actions: actions,
		Summary: fmt.Sprintf("Generated %d version plan actions", len(actions)),
	}, nil
}

func detectManagers() []ManagerInfo {
	managers := make([]ManagerInfo, 0, len(managerDefs))
	for _, def := range managerDefs {
		info := ManagerInfo{Name: def.Name, Command: def.Command}
		if path, err := exec.LookPath(def.Command); err == nil {
			info.Found = true
			info.Path = path
			info.Source = "path"
			if out, err := command.CombinedOutput(def.Command, def.Args...); err == nil || len(out) > 0 {
				info.Version = firstLineWithText(string(out))
			}
		} else if def.EnvVar != "" {
			base := os.Getenv(def.EnvVar)
			if base != "" && fileExists(filepath.Join(base, def.EnvSubPath)) {
				info.Found = true
				info.Path = filepath.Join(base, def.EnvSubPath)
				info.Source = def.EnvVar
				info.Version = "detected"
			}
		}
		managers = append(managers, info)
	}
	return managers
}

func detectRuntimes() []RuntimeInfo {
	managers := detectManagers()
	managerByRuntime := inferRuntimeManagers(managers)
	runtimes := make([]RuntimeInfo, 0, len(runtimeDefs))

	for _, def := range runtimeDefs {
		info := RuntimeInfo{Name: def.Name, Command: def.Command}
		if path, err := exec.LookPath(def.Command); err == nil {
			info.Found = true
			info.Path = path
			if out, err := command.CombinedOutput(def.Command, def.Args...); err == nil || len(out) > 0 {
				info.Version = cleanRuntimeVersion(def.Name, string(out))
			}
		}
		info.Manager = managerByRuntime[def.Name]
		runtimes = append(runtimes, info)
	}

	return runtimes
}

func inferRuntimeManagers(managers []ManagerInfo) map[string]string {
	found := make(map[string]bool, len(managers))
	for _, manager := range managers {
		found[manager.Name] = manager.Found
	}

	result := make(map[string]string)
	if found["nvm"] {
		result["node"] = "nvm"
	} else if found["fnm"] {
		result["node"] = "fnm"
	} else if found["asdf"] {
		result["node"] = "asdf"
	}
	if found["pyenv"] {
		result["python"] = "pyenv"
	} else if found["asdf"] {
		result["python"] = "asdf"
	}
	if found["rustup"] {
		result["rust"] = "rustup"
	} else if found["asdf"] {
		result["rust"] = "asdf"
	}
	if found["goenv"] {
		result["go"] = "goenv"
	} else if found["asdf"] {
		result["go"] = "asdf"
	}
	if found["sdkman"] {
		result["java"] = "sdkman"
	} else if found["asdf"] {
		result["java"] = "asdf"
	}
	return result
}

func detectRequirements(dir string) []Requirement {
	var requirements []Requirement
	add := func(runtimeName, versionValue, file, source, raw string) {
		versionValue = strings.TrimSpace(versionValue)
		if versionValue == "" {
			return
		}
		requirements = append(requirements, Requirement{
			Runtime:    runtimeName,
			Version:    versionValue,
			SourceFile: filepath.ToSlash(file),
			Source:     source,
			Raw:        raw,
		})
	}

	add("node", firstMeaningfulLine(filepath.Join(dir, ".nvmrc")), ".nvmrc", ".nvmrc", "")
	add("node", firstMeaningfulLine(filepath.Join(dir, ".node-version")), ".node-version", ".node-version", "")
	add("python", parseRuntimeTxt(filepath.Join(dir, "runtime.txt")), "runtime.txt", "runtime.txt", "")
	add("python", firstMeaningfulLine(filepath.Join(dir, ".python-version")), ".python-version", ".python-version", "")
	if versionValue := parseRustToolchain(filepath.Join(dir, "rust-toolchain.toml")); versionValue != "" {
		add("rust", versionValue, "rust-toolchain.toml", "rust-toolchain.toml", "")
	} else {
		add("rust", firstMeaningfulLine(filepath.Join(dir, "rust-toolchain")), "rust-toolchain", "rust-toolchain", "")
	}
	add("go", parseGoModVersion(filepath.Join(dir, "go.mod")), "go.mod", "go directive", "")

	if versionValue := parsePackageEngine(filepath.Join(dir, "package.json"), "node"); versionValue != "" {
		add("node", versionValue, "package.json", "engines.node", "")
	}
	if versionValue := parsePyprojectPython(filepath.Join(dir, "pyproject.toml")); versionValue != "" {
		add("python", versionValue, "pyproject.toml", "requires-python", "")
	}

	return dedupeRequirements(requirements)
}

func detectMismatches(requirements []Requirement, runtimes []RuntimeInfo) []Mismatch {
	runtimeByName := make(map[string]RuntimeInfo, len(runtimes))
	for _, runtimeInfo := range runtimes {
		runtimeByName[runtimeInfo.Name] = runtimeInfo
	}

	var mismatches []Mismatch
	for _, req := range requirements {
		runtimeInfo := runtimeByName[req.Runtime]
		if !runtimeInfo.Found {
			mismatches = append(mismatches, Mismatch{
				Runtime:        req.Runtime,
				Required:       req.Version,
				SourceFile:     req.SourceFile,
				Status:         "runtime missing",
				Description:    fmt.Sprintf("%s is required by %s but is not installed or not on PATH.", req.Runtime, req.SourceFile),
				RequiresReview: false,
			})
			continue
		}

		match, reviewOnly := requirementMatches(req.Runtime, req.Version, runtimeInfo.Version)
		if reviewOnly {
			mismatches = append(mismatches, Mismatch{
				Runtime:        req.Runtime,
				Required:       req.Version,
				Active:         runtimeInfo.Version,
				SourceFile:     req.SourceFile,
				Status:         "requires review",
				Description:    "Requirement uses a range or constraint that envdoctor does not evaluate yet.",
				RequiresReview: true,
			})
			continue
		}
		if !match {
			mismatches = append(mismatches, Mismatch{
				Runtime:        req.Runtime,
				Required:       req.Version,
				Active:         runtimeInfo.Version,
				SourceFile:     req.SourceFile,
				Status:         "version mismatch",
				Description:    fmt.Sprintf("Active %s version does not match project requirement.", req.Runtime),
				RequiresReview: false,
			})
		}
	}
	return mismatches
}

func planActions(scan *ScanReport) []PlanAction {
	managerByRuntime := make(map[string]string)
	for _, runtimeInfo := range scan.Runtimes {
		if runtimeInfo.Manager != "" {
			managerByRuntime[runtimeInfo.Name] = runtimeInfo.Manager
		}
	}

	var actions []PlanAction
	for _, mismatch := range scan.Mismatches {
		manager := managerByRuntime[mismatch.Runtime]
		action := PlanAction{
			Runtime:       mismatch.Runtime,
			Title:         fmt.Sprintf("Review %s runtime requirement from %s", mismatch.Runtime, mismatch.SourceFile),
			Risk:          "medium",
			RequiresAdmin: false,
			Platform:      runtime.GOOS,
			Manager:       manager,
			SafeToRun:     false,
			ManualSteps:   "Review the project runtime requirement and install or switch runtime manually.",
		}

		if mismatch.RequiresReview {
			action.Risk = "low"
			action.ManualSteps = fmt.Sprintf("Validate whether active version %q satisfies requirement %q.", mismatch.Active, mismatch.Required)
		} else if cmd := versionCommand(mismatch.Runtime, mismatch.Required, manager); cmd != "" {
			action.Command = cmd
			action.ManualSteps = "Command is a suggestion only; envdoctor does not run version manager commands."
		}

		actions = append(actions, action)
	}

	return actions
}

func versionCommand(runtimeName, required, manager string) string {
	versionValue := cleanRequiredVersion(required)
	if versionValue == "" {
		return ""
	}

	switch runtimeName {
	case "node":
		switch manager {
		case "fnm":
			return fmt.Sprintf("fnm install %s && fnm use %s", versionValue, versionValue)
		case "nvm":
			return fmt.Sprintf("nvm install %s && nvm use %s", versionValue, versionValue)
		case "asdf":
			return fmt.Sprintf("asdf install nodejs %s && asdf local nodejs %s", versionValue, versionValue)
		}
	case "python":
		switch manager {
		case "pyenv":
			return fmt.Sprintf("pyenv install %s && pyenv local %s", versionValue, versionValue)
		case "asdf":
			return fmt.Sprintf("asdf install python %s && asdf local python %s", versionValue, versionValue)
		}
	case "rust":
		if manager == "rustup" {
			return fmt.Sprintf("rustup toolchain install %s && rustup override set %s", versionValue, versionValue)
		}
		if manager == "asdf" {
			return fmt.Sprintf("asdf install rust %s && asdf local rust %s", versionValue, versionValue)
		}
	case "go":
		if manager == "goenv" {
			return fmt.Sprintf("goenv install %s && goenv local %s", versionValue, versionValue)
		}
		if manager == "asdf" {
			return fmt.Sprintf("asdf install golang %s && asdf local golang %s", versionValue, versionValue)
		}
	case "java":
		if manager == "sdkman" {
			return fmt.Sprintf("sdk install java %s", versionValue)
		}
		if manager == "asdf" {
			return fmt.Sprintf("asdf install java %s && asdf local java %s", versionValue, versionValue)
		}
	}

	return ""
}

func firstMeaningfulLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line
	}
	return ""
}

func parseRuntimeTxt(path string) string {
	line := firstMeaningfulLine(path)
	line = strings.TrimPrefix(line, "python-")
	return line
}

func parseRustToolchain(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "channel") {
			_, value, ok := strings.Cut(line, "=")
			if ok {
				return strings.Trim(strings.TrimSpace(value), `"`)
			}
		}
	}
	return ""
}

func parseGoModVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 2 && fields[0] == "go" {
			return fields[1]
		}
	}
	return ""
}

func parsePackageEngine(path, engine string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var pkg struct {
		Engines map[string]string `json:"engines"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Engines[engine]
}

func parsePyprojectPython(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	inProject := false
	inPoetryDeps := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "[project]":
			inProject = true
			inPoetryDeps = false
			continue
		case "[tool.poetry.dependencies]":
			inProject = false
			inPoetryDeps = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inProject = false
			inPoetryDeps = false
		}
		if inProject && strings.HasPrefix(trimmed, "requires-python") {
			return valueAfterEquals(trimmed)
		}
		if inPoetryDeps && strings.HasPrefix(trimmed, "python") {
			return valueAfterEquals(trimmed)
		}
	}
	return ""
}

func valueAfterEquals(line string) string {
	_, value, ok := strings.Cut(line, "=")
	if !ok {
		return ""
	}
	return strings.Trim(strings.TrimSpace(value), `"`)
}

func requirementMatches(runtimeName, required, active string) (bool, bool) {
	required = strings.TrimSpace(required)
	active = cleanRuntimeVersion(runtimeName, active)
	if required == "" || active == "" {
		return true, true
	}
	if isNamedChannel(required) {
		return false, true
	}
	if runtimeName == "go" {
		return goVersionSatisfies(active, required), false
	}

	if isConstraint(required) {
		return false, true
	}

	required = cleanRequiredVersion(required)
	active = cleanRequiredVersion(active)
	if required == "" || active == "" {
		return true, true
	}
	return strings.HasPrefix(active, required) || sameMajorMinor(active, required), false
}

func isConstraint(value string) bool {
	return strings.ContainsAny(value, "<>^~*=|, ") || strings.HasPrefix(value, "python")
}

func goVersionSatisfies(active, required string) bool {
	activeParts := numericParts(cleanRequiredVersion(active))
	requiredParts := numericParts(cleanRequiredVersion(required))
	if len(activeParts) < 2 || len(requiredParts) < 2 {
		return false
	}
	for i := 0; i < len(requiredParts) && i < len(activeParts); i++ {
		if activeParts[i] > requiredParts[i] {
			return true
		}
		if activeParts[i] < requiredParts[i] {
			return false
		}
	}
	return true
}

func sameMajorMinor(a, b string) bool {
	aParts := numericParts(a)
	bParts := numericParts(b)
	if len(aParts) < 2 || len(bParts) < 2 {
		return a == b
	}
	return aParts[0] == bParts[0] && aParts[1] == bParts[1]
}

func numericParts(value string) []int {
	cleaned := cleanRequiredVersion(value)
	var parts []int
	for _, part := range strings.Split(cleaned, ".") {
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			break
		}
		parts = append(parts, n)
	}
	return parts
}

func cleanRuntimeVersion(runtimeName, value string) string {
	line := firstLineWithText(value)
	line = strings.TrimSpace(line)
	switch runtimeName {
	case "node":
		return strings.TrimPrefix(line, "v")
	case "python":
		return strings.TrimPrefix(strings.TrimPrefix(line, "Python "), "python ")
	case "rust":
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			return fields[1]
		}
	case "go":
		fields := strings.Fields(line)
		for _, field := range fields {
			if strings.HasPrefix(field, "go") {
				versionValue := strings.TrimPrefix(field, "go")
				if versionValue != "" && hasDigit(versionValue) {
					return versionValue
				}
			}
		}
	case "java":
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.Contains(fields[2], ".") {
			return strings.Trim(fields[2], `"`)
		}
	}
	return cleanRequiredVersion(line)
}

func cleanRequiredVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	value = strings.TrimPrefix(value, "v")
	value = strings.TrimPrefix(value, "python-")
	for _, prefix := range []string{">=", "<=", "==", "~=", "!=", ">", "<", "^", "~", "="} {
		value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
	}
	value = strings.TrimPrefix(value, "go")
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '|' || r == ';'
	})
	if len(fields) > 0 {
		value = strings.TrimSpace(fields[0])
	}
	return value
}

func isNamedChannel(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "stable", "beta", "nightly", "latest", "lts":
		return true
	default:
		return false
	}
}

func hasDigit(value string) bool {
	for _, ch := range value {
		if ch >= '0' && ch <= '9' {
			return true
		}
	}
	return false
}

func firstLineWithText(value string) string {
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return strings.TrimSpace(value)
}

func summarizeScan(report *ScanReport) string {
	foundManagers := 0
	foundRuntimes := 0
	for _, manager := range report.Managers {
		if manager.Found {
			foundManagers++
		}
	}
	for _, runtimeInfo := range report.Runtimes {
		if runtimeInfo.Found {
			foundRuntimes++
		}
	}
	return fmt.Sprintf("Found %d version managers, %d runtimes, %d project requirements, %d items needing review", foundManagers, foundRuntimes, len(report.Requirements), len(report.Mismatches))
}

func dedupeRequirements(requirements []Requirement) []Requirement {
	seen := make(map[string]bool, len(requirements))
	var result []Requirement
	for _, req := range requirements {
		key := req.Runtime + "\x00" + req.Version + "\x00" + req.SourceFile
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, req)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Runtime == result[j].Runtime {
			return result[i].SourceFile < result[j].SourceFile
		}
		return result[i].Runtime < result[j].Runtime
	})
	return result
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
