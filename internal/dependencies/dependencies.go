package dependencies

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	Language     string       `json:"language"`
	Dependencies []Dependency `json:"dependencies"`
}

// AnalyzeDependencies analyzes project dependencies from various file formats
func AnalyzeDependencies(dir string) (*ProjectDependencies, error) {
	// mapping file -> analyzer
	files := map[string]func(string) (*ProjectDependencies, error){
		"requirements.txt": analyzePythonRequirements,
		"pyproject.toml":   analyzePythonPyproject,
		"package.json":     analyzeNodePackage,
		"Cargo.toml":       analyzeRustCargo,
		"go.mod":           analyzeGoMod,
		"composer.json":    analyzePHPComposer,
	}

	for file, fn := range files {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			deps, err := fn(path)
			if err != nil {
				return nil, err
			}
			// Now validate which dependencies are actually installed
			deps = validateDependencies(deps)
			return deps, nil
		}
	}

	return nil, fmt.Errorf("no supported dependency file found in directory: %s", dir)
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
		// Handle inline comments
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
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
	// Simple heuristic to split name and version spec
	separators := []string{"==", ">=", "<=", ">", "<", "!=", "~="}
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

// --- Python: pyproject.toml ---
func analyzePythonPyproject(path string) (*ProjectDependencies, error) {
	type PyprojectToml struct {
		Project struct {
			Dependencies []string `toml:"dependencies"`
		} `toml:"project"`
		Tool struct {
			Poetry struct {
				Dependencies map[string]interface{} `toml:"dependencies"`
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

	// Poetry style dependencies
	for name, ver := range pyproject.Tool.Poetry.Dependencies {
		var versionStr string
		switch v := ver.(type) {
		case string:
			versionStr = v
		case map[string]interface{}:
			if verStr, ok := v["version"].(string); ok {
				versionStr = verStr
			}
		}
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  versionStr,
			Required: true,
		})
	}

	return deps, nil
}

// --- Node.js: package.json ---
func analyzeNodePackage(path string) (*ProjectDependencies, error) {
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
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
	return deps, nil
}

// --- Rust: Cargo.toml ---
func analyzeRustCargo(path string) (*ProjectDependencies, error) {
	type CargoToml struct {
		Dependencies map[string]interface{} `toml:"dependencies"`
	}

	var cargo CargoToml
	if _, err := toml.DecodeFile(path, &cargo); err != nil {
		return nil, err
	}

	deps := &ProjectDependencies{Language: "Rust"}
	for name, ver := range cargo.Dependencies {
		var versionStr string
		switch v := ver.(type) {
		case string:
			versionStr = v
		case map[string]interface{}:
			if verStr, ok := v["version"].(string); ok {
				versionStr = verStr
			}
		}
		deps.Dependencies = append(deps.Dependencies, Dependency{
			Name:     name,
			Version:  versionStr,
			Required: true,
		})
	}
	return deps, nil
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
			// validateDependencies checks which dependencies are installed in the environment
func validateDependencies(proj *ProjectDependencies) *ProjectDependencies {
	switch proj.Language {
	case "Python":
		proj = validatePythonDeps(proj)
	case "Node.js":
		proj = validateNodeDeps(proj)
	case "Go":
		proj = validateGoDeps(proj)
	case "Rust":
		proj = validateRustDeps(proj)
	case "PHP":
		proj = validatePHPDeps(proj)
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
		out, err := exec.Command("pip3", "show", pkgName).Output()
		if err != nil {
			out, err = exec.Command("pip", "show", pkgName).Output()
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
func validateNodeDeps(proj *ProjectDependencies) *ProjectDependencies {
	for i, dep := range proj.Dependencies {
		out, err := exec.Command("npm", "ls", dep.Name, "--json", "--depth=0").Output()
		if err == nil {
			var npmLsOutput struct {
				Dependencies map[string]struct {
					Version string `json:"version"`
				} `json:"dependencies"`
			}
			if err := json.Unmarshal(out, &npmLsOutput); err == nil {
				if v, ok := npmLsOutput.Dependencies[dep.Name]; ok {
					proj.Dependencies[i].Installed = true
					// Parse version from npm ls output
					proj.Dependencies[i].InstalledVersion = v.Version
				}
			}
		}
	}
	return proj
}

// --- Go Validation ---
func validateGoDeps(proj *ProjectDependencies) *ProjectDependencies {
	// Use go list -m to check if modules are cached/downloaded
	for i, dep := range proj.Dependencies {
		out, err := exec.Command("go", "list", "-m", dep.Name).Output()
		if err == nil {
			proj.Dependencies[i].Installed = true
			// Parse installed version from output
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) > 0 {
				// go list -m output: module v0.0.0
				parts := strings.Fields(lines[0])
				if len(parts) >= 2 {
					proj.Dependencies[i].InstalledVersion = parts[1]
				}
			}
		}
	}
	return proj
}

// --- Rust Validation ---
func validateRustDeps(proj *ProjectDependencies) *ProjectDependencies {
	// cargo metadata can show resolved dependencies
	out, err := exec.Command("cargo", "metadata", "--format-version=1", "--no-deps").Output()
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
func validatePHPDeps(proj *ProjectDependencies) *ProjectDependencies {
	for i, dep := range proj.Dependencies {
		out, err := exec.Command("composer", "show", dep.Name).Output()
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
	fmt.Println("Dependencies:")
	for _, dep := range deps.Dependencies {
		if dep.Required {
			fmt.Printf("  [required] %s%s\n", dep.Name, dep.Version)
		} else {
			fmt.Printf("  [optional] %s%s\n", dep.Name, dep.Version)
		}
		if dep.Installed {
			fmt.Printf("    ✓ Installed: %s\n", dep.InstalledVersion)
		} else {
			fmt.Printf("    ✗ Not installed\n")
		}
	}
}
