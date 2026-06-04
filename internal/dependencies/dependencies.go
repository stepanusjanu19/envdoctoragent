package dependencies

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"envdoctor/internal/command"

	"github.com/BurntSushi/toml"
)

// Dependency represents a project dependency
type Dependency struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	Required         bool   `json:"required"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installed_version"`
}

// ProjectDependencies holds information about project dependencies
type ProjectDependencies struct {
	SourceFile   string       `json:"source_file,omitempty"`
	Language     string       `json:"language"`
	Dependencies []Dependency `json:"dependencies"`
}

// AnalyzeDependencies analyzes project dependencies from various file formats
func AnalyzeDependencies(dir string) (*ProjectDependencies, error) {
	results, err := AnalyzeAllDependencies(dir)
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// AnalyzeAllDependencies analyzes every supported dependency file found in a directory.
func AnalyzeAllDependencies(dir string) ([]*ProjectDependencies, error) {
	if info, err := os.Stat(dir); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}

	analyzers := []struct {
		file string
		fn   func(string) (*ProjectDependencies, error)
	}{
		{file: "requirements.txt", fn: analyzePythonRequirements},
		{file: "pyproject.toml", fn: analyzePythonPyproject},
		{file: "package.json", fn: analyzeNodePackage},
		{file: "Cargo.toml", fn: analyzeRustCargo},
		{file: "go.mod", fn: analyzeGoMod},
		{file: "composer.json", fn: analyzePHPComposer},
	}

	var results []*ProjectDependencies
	for _, analyzer := range analyzers {
		path := filepath.Join(dir, analyzer.file)
		if _, err := os.Stat(path); err == nil {
			deps, err := analyzer.fn(path)
			if err != nil {
				return nil, err
			}
			deps.SourceFile = analyzer.file
			results = append(results, validateDependencies(deps, dir))
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no supported dependency file found in directory: %s", dir)
	}

	return results, nil
}

// --- Python: requirements.txt ---
func analyzePythonRequirements(path string) (*ProjectDependencies, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	deps := &ProjectDependencies{Language: "Python"}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if shouldSkipRequirementLine(line) {
			continue
		}
		// Basic handling: split by operators to get name and version spec
		dep := parseRequirementLine(line)
		if dep != nil {
			deps.Dependencies = append(deps.Dependencies, *dep)
		}
	}
	return deps, scanner.Err()
}

func parseRequirementLine(line string) *Dependency {
	line = strings.TrimSpace(line)
	if line == "" || shouldSkipRequirementLine(line) {
		return nil
	}

	if editable := parseEditableRequirement(line); editable != nil {
		return editable
	}

	if idx := strings.Index(line, "#"); idx != -1 {
		line = strings.TrimSpace(line[:idx])
	}

	if idx := strings.Index(line, ";"); idx != -1 {
		line = strings.TrimSpace(line[:idx])
	}

	if idx := strings.Index(line, " @ "); idx != -1 {
		return &Dependency{
			Name:     strings.TrimSpace(line[:idx]),
			Version:  strings.TrimSpace(line[idx+1:]),
			Required: true,
		}
	}

	// Simple heuristic to split name and version spec
	separators := []string{"==", ">=", "<=", "!=", "~=", ">", "<"}
	for _, sep := range separators {
		if idx := strings.Index(line, sep); idx != -1 {
			return &Dependency{
				Name:     strings.TrimSpace(line[:idx]),
				Version:  strings.TrimSpace(line[idx:]),
				Required: true,
			}
		}
	}
	// No version specified
	return &Dependency{Name: line, Required: true}
}

func shouldSkipRequirementLine(line string) bool {
	skipPrefixes := []string{
		"-r ", "--requirement ",
		"-c ", "--constraint ",
		"--index-url", "--extra-index-url", "--find-links",
		"--trusted-host", "--no-index", "--pre",
	}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "-e ") && !strings.HasPrefix(line, "--editable ")
}

func parseEditableRequirement(line string) *Dependency {
	if strings.HasPrefix(line, "-e ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "-e "))
	} else if strings.HasPrefix(line, "--editable ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "--editable "))
	} else {
		return nil
	}

	if idx := strings.LastIndex(line, "#egg="); idx != -1 {
		name := strings.TrimSpace(line[idx+5:])
		if name != "" {
			return &Dependency{Name: name, Version: "editable", Required: true}
		}
	}
	return nil
}

// --- Python: pyproject.toml ---
func analyzePythonPyproject(path string) (*ProjectDependencies, error) {
	type PyprojectToml struct {
		Project struct {
			Dependencies         []string            `toml:"dependencies"`
			OptionalDependencies map[string][]string `toml:"optional-dependencies"`
		} `toml:"project"`
		Tool struct {
			Poetry struct {
				Dependencies    map[string]interface{}           `toml:"dependencies"`
				DevDependencies map[string]interface{}           `toml:"dev-dependencies"`
				Group           map[string]poetryDependencyGroup `toml:"group"`
			} `toml:"poetry"`
		} `toml:"tool"`
	}

	var pyproject PyprojectToml
	if _, err := toml.DecodeFile(path, &pyproject); err != nil {
		return nil, err
	}

	deps := &ProjectDependencies{Language: "Python"}

	// PEP 621 style dependencies
	for _, depStr := range pyproject.Project.Dependencies {
		if dep := parseRequirementLine(depStr); dep != nil {
			deps.Dependencies = append(deps.Dependencies, *dep)
		}
	}

	for _, optionalDeps := range pyproject.Project.OptionalDependencies {
		for _, depStr := range optionalDeps {
			if dep := parseRequirementLine(depStr); dep != nil {
				dep.Required = false
				deps.Dependencies = append(deps.Dependencies, *dep)
			}
		}
	}

	// Poetry style dependencies
	for name, ver := range pyproject.Tool.Poetry.Dependencies {
		if name == "python" {
			continue
		}
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  parseTomlVersion(ver),
			Required: true,
		})
	}

	for name, ver := range pyproject.Tool.Poetry.DevDependencies {
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  parseTomlVersion(ver),
			Required: false,
		})
	}

	for _, group := range pyproject.Tool.Poetry.Group {
		for name, ver := range group.Dependencies {
			deps.Dependencies = append(deps.Dependencies, Dependency{
				Name:     name,
				Version:  parseTomlVersion(ver),
				Required: false,
			})
		}
	}

	return deps, nil
}

type poetryDependencyGroup struct {
	Dependencies map[string]interface{} `toml:"dependencies"`
}

// --- Node.js: package.json ---
func analyzeNodePackage(path string) (*ProjectDependencies, error) {
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		OptionalDeps    map[string]string `json:"optionalDependencies"`
		PeerDeps        map[string]string `json:"peerDependencies"`
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	deps := &ProjectDependencies{Language: "Node.js"}
	for name, version := range pkg.Dependencies {
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  version,
			Required: true,
		})
	}
	for name, version := range pkg.DevDependencies {
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  version,
			Required: false,
		})
	}
	for name, version := range pkg.OptionalDeps {
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  version,
			Required: false,
		})
	}
	for name, version := range pkg.PeerDeps {
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  version,
			Required: false,
		})
	}
	return deps, nil
}

// --- Rust: Cargo.toml ---
func analyzeRustCargo(path string) (*ProjectDependencies, error) {
	type CargoToml struct {
		Dependencies      map[string]interface{} `toml:"dependencies"`
		DevDependencies   map[string]interface{} `toml:"dev-dependencies"`
		BuildDependencies map[string]interface{} `toml:"build-dependencies"`
	}

	var cargo CargoToml
	if _, err := toml.DecodeFile(path, &cargo); err != nil {
		return nil, err
	}

	deps := &ProjectDependencies{Language: "Rust"}
	for name, ver := range cargo.Dependencies {
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  parseTomlVersion(ver),
			Required: true,
		})
	}
	for name, ver := range cargo.BuildDependencies {
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  parseTomlVersion(ver),
			Required: true,
		})
	}
	for name, ver := range cargo.DevDependencies {
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  parseTomlVersion(ver),
			Required: false,
		})
	}
	return deps, nil
}

func parseTomlVersion(ver interface{}) string {
	switch v := ver.(type) {
	case string:
		return v
	case map[string]interface{}:
		if verStr, ok := v["version"].(string); ok {
			return verStr
		}
	}
	return ""
}

// --- Go: go.mod ---
func analyzeGoMod(path string) (*ProjectDependencies, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	deps := &ProjectDependencies{Language: "Go"}
	scanner := bufio.NewScanner(file)
	foundRequire := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// skip comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// detect start of require block
		if strings.HasPrefix(trimmed, "require (") {
			foundRequire = true
			continue
		}
		if trimmed == ")" {
			foundRequire = false
			continue
		}

		if !foundRequire && !strings.HasPrefix(trimmed, "require ") {
			continue
		}

		// parse line: e.g., "require github.com/spf13/cobra v1.10.2"
		// or inside a block: "github.com/spf13/cobra v1.10.2"
		var depStr string
		if strings.HasPrefix(trimmed, "require ") {
			depStr = strings.TrimPrefix(trimmed, "require ")
		} else {
			depStr = trimmed
		}

		parts := strings.Fields(depStr)
		if len(parts) >= 2 {
			isIndirect := false
			if len(parts) >= 3 && parts[2] == "//" && strings.Contains(depStr, "indirect") {
				isIndirect = true
			}
			deps.Dependencies = append(deps.Dependencies, Dependency{
				Name:     parts[0],
				Version:  parts[1],
				Required: !isIndirect,
			})
		}
	}

	return deps, scanner.Err()
}

// --- PHP: composer.json ---
func analyzePHPComposer(path string) (*ProjectDependencies, error) {
	var composer struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &composer); err != nil {
		return nil, err
	}

	deps := &ProjectDependencies{Language: "PHP"}
	for name, version := range composer.Require {
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  version,
			Required: true,
		})
	}
	for name, version := range composer.RequireDev {
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  version,
			Required: false,
		})
	}
	return deps, nil
}

// validateDependencies checks which dependencies are installed in the environment.
func validateDependencies(proj *ProjectDependencies, dir string) *ProjectDependencies {
	switch proj.Language {
	case "Python":
		proj = validatePythonDeps(proj)
	case "Node.js":
		proj = validateNodeDeps(proj, dir)
	case "Go":
		proj = validateGoDeps(proj, dir)
	case "Rust":
		proj = validateRustDeps(proj, dir)
	case "PHP":
		proj = validatePHPDeps(proj, dir)
	}
	return proj
}

// stripPythonConstraint removes version constraints from package names
func stripPythonConstraint(name string) string {
	// Remove version constraints like package>=1.0, package==1.0
	for _, sep := range []string{">=", "<=", ">", "<", "==", "!=", "~=", " ", "[", ";", "@"} {
		if idx := strings.Index(name, sep); idx != -1 {
			name = strings.TrimSpace(name[:idx])
			break
		}
	}
	return name
}

// --- Python Validation ---
func validatePythonDeps(proj *ProjectDependencies) *ProjectDependencies {
	for i, dep := range proj.Dependencies {
		// Check via pip show (strip version specifiers like >=, ==)
		pkgName := stripPythonConstraint(dep.Name)
		out, err := command.Output("pip3", "show", pkgName)
		if err != nil {
			out, err = command.Output("pip", "show", pkgName)
		}
		if err == nil {
			proj.Dependencies[i].Installed = true
			// Parse Version: line from pip show output
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "Version: ") {
					proj.Dependencies[i].InstalledVersion = strings.TrimPrefix(line, "Version: ")
					break
				}
			}
		}
	}
	return proj
}

// --- Node.js Validation ---
func validateNodeDeps(proj *ProjectDependencies, dir string) *ProjectDependencies {
	for i, dep := range proj.Dependencies {
		out, _ := command.CombinedOutputInDir(dir, "npm", "ls", dep.Name, "--json", "--depth=0")
		var npmLsOutput struct {
			Dependencies map[string]struct {
				Version string `json:"version"`
			} `json:"dependencies"`
		}
		if err := json.Unmarshal(out, &npmLsOutput); err == nil {
			if v, ok := npmLsOutput.Dependencies[dep.Name]; ok {
				proj.Dependencies[i].Installed = true
				proj.Dependencies[i].InstalledVersion = v.Version
			}
		}
	}
	return proj
}

// --- Go Validation ---
func validateGoDeps(proj *ProjectDependencies, dir string) *ProjectDependencies {
	out, err := command.OutputInDir(dir, "go", "list", "-m", "-json", "all")
	if err != nil {
		return proj
	}

	installed := make(map[string]string)
	decoder := json.NewDecoder(bytes.NewReader(out))
	for {
		var module struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
		}
		err := decoder.Decode(&module)
		if err == io.EOF {
			break
		}
		if err != nil {
			return proj
		}
		if module.Path != "" {
			installed[module.Path] = module.Version
		}
	}

	for i, dep := range proj.Dependencies {
		if version, ok := installed[dep.Name]; ok {
			proj.Dependencies[i].Installed = true
			proj.Dependencies[i].InstalledVersion = version
		}
	}
	return proj
}

// --- Rust Validation ---
func validateRustDeps(proj *ProjectDependencies, dir string) *ProjectDependencies {
	// cargo metadata can show resolved dependencies
	out, err := command.OutputInDir(dir, "cargo", "metadata", "--format-version=1")
	if err != nil {
		return proj
	}
	var cargoMeta struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(out, &cargoMeta); err != nil {
		return proj
	}

	installed := make(map[string]string)
	for _, p := range cargoMeta.Packages {
		installed[p.Name] = p.Version
	}

	for i, dep := range proj.Dependencies {
		if ver, ok := installed[dep.Name]; ok {
			proj.Dependencies[i].Installed = true
			proj.Dependencies[i].InstalledVersion = ver
		}
	}
	return proj
}

// --- PHP Validation ---
func validatePHPDeps(proj *ProjectDependencies, dir string) *ProjectDependencies {
	for i, dep := range proj.Dependencies {
		out, err := command.OutputInDir(dir, "composer", "show", dep.Name)
		if err == nil {
			proj.Dependencies[i].Installed = true
			// Parse version from composer show output
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "versions : ") {
					proj.Dependencies[i].InstalledVersion = strings.TrimPrefix(line, "versions : ")
					break
				}
			}
		}
	}
	return proj
}

// PrintDependencies prints project dependencies in a readable format
func PrintDependencies(deps *ProjectDependencies) {
	fmt.Printf("Language: %s\n", deps.Language)
	if deps.SourceFile != "" {
		fmt.Printf("Source: %s\n", deps.SourceFile)
	}
	fmt.Println("Dependencies:")
	for _, dep := range deps.Dependencies {
		if dep.Required {
			fmt.Printf("  [required] %s\n", formatDependencyNameVersion(dep))
		} else {
			fmt.Printf("  [optional] %s\n", formatDependencyNameVersion(dep))
		}
		if dep.Installed {
			fmt.Printf("    ✓ Installed: %s\n", dep.InstalledVersion)
		} else {
			fmt.Printf("    ✗ Not installed\n")
		}
	}
}

func formatDependencyNameVersion(dep Dependency) string {
	if dep.Version == "" {
		return dep.Name
	}
	for _, prefix := range []string{"==", ">=", "<=", "!=", "~=", ">", "<"} {
		if strings.HasPrefix(dep.Version, prefix) {
			return dep.Name + dep.Version
		}
	}
	return fmt.Sprintf("%s %s", dep.Name, dep.Version)
}

// PrintAllDependencies prints multiple dependency reports in a readable format.
func PrintAllDependencies(reports []*ProjectDependencies) {
	for i, deps := range reports {
		if i > 0 {
			fmt.Println()
		}
		PrintDependencies(deps)
	}
}
