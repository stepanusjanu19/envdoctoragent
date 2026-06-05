package dependencies

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"envdoctor/internal/command"

	"github.com/BurntSushi/toml"
)

const (
	validationValidated    = "validated"
	validationMetadataOnly = "metadata-only"
	statusDetected         = "detected"
	statusNoDependencies   = "no-dependencies"
)

// Dependency represents a project dependency.
type Dependency struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	Required         bool   `json:"required"`
	Scope            string `json:"scope,omitempty"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installed_version"`
}

// ProjectDependencies holds information about one dependency manifest.
type ProjectDependencies struct {
	SourceFile       string       `json:"source_file,omitempty"`
	Language         string       `json:"language"`
	Ecosystem        string       `json:"ecosystem"`
	PackageManager   string       `json:"package_manager"`
	ManifestType     string       `json:"manifest_type"`
	Scope            string       `json:"scope,omitempty"`
	ValidationStatus string       `json:"validation_status"`
	Status           string       `json:"status"`
	Frameworks       []string     `json:"frameworks,omitempty"`
	Dependencies     []Dependency `json:"dependencies"`
}

type manifestAnalyzer struct {
	Language       string
	Ecosystem      string
	PackageManager string
	ManifestType   string
	Match          func(string) bool
	Analyze        func(string) (*ProjectDependencies, error)
	Validate       func(*ProjectDependencies, string) *ProjectDependencies
}

var ignoredDependencyDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".venv":        true,
	"venv":         true,
	".cache":       true,
}

var registry = []manifestAnalyzer{
	{Language: "Python", Ecosystem: "python", PackageManager: "pip", ManifestType: "requirements.txt", Match: baseIs("requirements.txt"), Analyze: analyzePythonRequirements, Validate: validatePythonDeps},
	{Language: "Python", Ecosystem: "python", PackageManager: "pyproject", ManifestType: "pyproject.toml", Match: baseIs("pyproject.toml"), Analyze: analyzePythonPyproject, Validate: validatePythonDeps},
	{Language: "Node.js", Ecosystem: "node", PackageManager: "npm", ManifestType: "package.json", Match: baseIs("package.json"), Analyze: analyzeNodePackage, Validate: validateNodeDeps},
	{Language: "Go", Ecosystem: "go", PackageManager: "go modules", ManifestType: "go.mod", Match: baseIs("go.mod"), Analyze: analyzeGoMod, Validate: validateGoDeps},
	{Language: "Rust", Ecosystem: "rust", PackageManager: "cargo", ManifestType: "Cargo.toml", Match: baseIs("Cargo.toml"), Analyze: analyzeRustCargo, Validate: validateRustDeps},
	{Language: "PHP", Ecosystem: "php", PackageManager: "composer", ManifestType: "composer.json", Match: baseIs("composer.json"), Analyze: analyzePHPComposer, Validate: validatePHPDeps},
	{Language: "Java/Kotlin", Ecosystem: "jvm", PackageManager: "maven", ManifestType: "pom.xml", Match: baseIs("pom.xml"), Analyze: analyzeMavenPom},
	{Language: "Java/Kotlin", Ecosystem: "jvm", PackageManager: "gradle", ManifestType: "build.gradle", Match: baseAny("build.gradle", "build.gradle.kts"), Analyze: analyzeGradleBuild},
	{Language: ".NET", Ecosystem: "dotnet", PackageManager: "nuget", ManifestType: ".csproj", Match: suffixIs(".csproj"), Analyze: analyzeDotnetProject},
	{Language: ".NET", Ecosystem: "dotnet", PackageManager: "nuget", ManifestType: "packages.config", Match: baseIs("packages.config"), Analyze: analyzeNugetPackagesConfig},
	{Language: ".NET", Ecosystem: "dotnet", PackageManager: "nuget", ManifestType: "Directory.Packages.props", Match: baseIs("Directory.Packages.props"), Analyze: analyzeDirectoryPackagesProps},
	{Language: "Ruby", Ecosystem: "ruby", PackageManager: "bundler", ManifestType: "Gemfile", Match: baseIs("Gemfile"), Analyze: analyzeGemfile},
	{Language: "Dart/Flutter", Ecosystem: "dart", PackageManager: "pub", ManifestType: "pubspec.yaml", Match: baseAny("pubspec.yaml", "pubspec.yml"), Analyze: analyzePubspec},
	{Language: "Swift", Ecosystem: "swift", PackageManager: "swiftpm", ManifestType: "Package.swift", Match: baseIs("Package.swift"), Analyze: analyzeSwiftPackage},
	{Language: "Elixir", Ecosystem: "elixir", PackageManager: "mix", ManifestType: "mix.exs", Match: baseIs("mix.exs"), Analyze: analyzeMixExs},
	{Language: "Lua", Ecosystem: "lua", PackageManager: "luarocks", ManifestType: ".rockspec", Match: suffixIs(".rockspec"), Analyze: analyzeRockspec},
	{Language: "R", Ecosystem: "r", PackageManager: "cran", ManifestType: "DESCRIPTION", Match: baseIs("DESCRIPTION"), Analyze: analyzeRDescription},
	{Language: "Julia", Ecosystem: "julia", PackageManager: "pkg", ManifestType: "Project.toml", Match: baseIs("Project.toml"), Analyze: analyzeJuliaProject},
	{Language: "Haskell", Ecosystem: "haskell", PackageManager: "cabal", ManifestType: ".cabal", Match: suffixIs(".cabal"), Analyze: analyzeCabal},
	{Language: "Haskell", Ecosystem: "haskell", PackageManager: "stack", ManifestType: "stack.yaml", Match: baseIs("stack.yaml"), Analyze: analyzeStackYaml},
	{Language: "Perl", Ecosystem: "perl", PackageManager: "cpanm", ManifestType: "cpanfile", Match: baseIs("cpanfile"), Analyze: analyzeCpanfile},
	{Language: "C/C++", Ecosystem: "cpp", PackageManager: "vcpkg", ManifestType: "vcpkg.json", Match: baseIs("vcpkg.json"), Analyze: analyzeVcpkgJSON},
	{Language: "C/C++", Ecosystem: "cpp", PackageManager: "conan", ManifestType: "conanfile.txt", Match: baseIs("conanfile.txt"), Analyze: analyzeConanfileTxt},
	{Language: "C/C++", Ecosystem: "cpp", PackageManager: "conan", ManifestType: "conanfile.py", Match: baseIs("conanfile.py"), Analyze: analyzeConanfilePy},
}

// AnalyzeDependencies analyzes project dependencies from the first supported manifest found.
func AnalyzeDependencies(dir string) (*ProjectDependencies, error) {
	results, err := AnalyzeAllDependencies(dir)
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// AnalyzeAllDependencies analyzes every supported dependency manifest found in a directory tree.
func AnalyzeAllDependencies(dir string) ([]*ProjectDependencies, error) {
	if info, err := os.Stat(dir); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	var results []*ProjectDependencies
	err = filepath.WalkDir(absDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if path != absDir && ignoredDependencyDirs[entry.Name()] {
				return filepath.SkipDir
			}
			if path != absDir && pathDepth(absDir, path) > 3 {
				return filepath.SkipDir
			}
			return nil
		}

		for _, analyzer := range registry {
			if !analyzer.Match(path) {
				continue
			}
			report, err := analyzer.Analyze(path)
			if err != nil {
				return err
			}
			applyMetadata(report, analyzer, absDir, path)
			if analyzer.Validate != nil {
				report = analyzer.Validate(report, filepath.Dir(path))
				report.ValidationStatus = validationValidated
			} else if report.ValidationStatus == "" {
				report.ValidationStatus = validationMetadataOnly
			}
			finalizeReport(report)
			results = append(results, report)
			break
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no supported dependency file found in directory: %s", dir)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].SourceFile < results[j].SourceFile
	})
	return results, nil
}

func applyMetadata(report *ProjectDependencies, analyzer manifestAnalyzer, root, path string) {
	if report.Language == "" {
		report.Language = analyzer.Language
	}
	if report.Ecosystem == "" {
		report.Ecosystem = analyzer.Ecosystem
	}
	if report.PackageManager == "" {
		report.PackageManager = analyzer.PackageManager
	}
	if report.ManifestType == "" {
		report.ManifestType = analyzer.ManifestType
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	report.SourceFile = filepath.ToSlash(rel)
}

func finalizeReport(report *ProjectDependencies) {
	report.Frameworks = dedupeStrings(report.Frameworks)
	for i := range report.Dependencies {
		if report.Dependencies[i].Scope == "" {
			if report.Dependencies[i].Required {
				report.Dependencies[i].Scope = "runtime"
			} else {
				report.Dependencies[i].Scope = "development"
			}
		}
	}
	if report.Scope == "" {
		report.Scope = inferReportScope(report.Dependencies)
	}
	if report.ValidationStatus == "" {
		report.ValidationStatus = validationMetadataOnly
	}
	if len(report.Dependencies) == 0 {
		report.Status = statusNoDependencies
	} else if report.Status == "" {
		report.Status = statusDetected
	}
}

func inferReportScope(deps []Dependency) string {
	if len(deps) == 0 {
		return "none"
	}
	hasRuntime := false
	hasDev := false
	for _, dep := range deps {
		if dep.Required {
			hasRuntime = true
		} else {
			hasDev = true
		}
	}
	switch {
	case hasRuntime && hasDev:
		return "mixed"
	case hasRuntime:
		return "runtime"
	default:
		return "development"
	}
}

func pathDepth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	return len(strings.Split(filepath.ToSlash(rel), "/"))
}

func baseIs(name string) func(string) bool {
	return func(path string) bool { return filepath.Base(path) == name }
}

func baseAny(names ...string) func(string) bool {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	return func(path string) bool { return allowed[filepath.Base(path)] }
}

func suffixIs(suffix string) func(string) bool {
	return func(path string) bool { return strings.HasSuffix(filepath.Base(path), suffix) }
}

func dependency(name, version, scope string, required bool) Dependency {
	return Dependency{Name: strings.TrimSpace(name), Version: strings.TrimSpace(version), Scope: scope, Required: required}
}

func addDependency(deps *[]Dependency, name, version, scope string, required bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	*deps = append(*deps, dependency(name, version, scope, required))
}

// --- Python: requirements.txt ---
func analyzePythonRequirements(path string) (*ProjectDependencies, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	report := &ProjectDependencies{Language: "Python", Ecosystem: "python", PackageManager: "pip"}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || shouldSkipRequirementLine(line) {
			continue
		}
		if dep := parseRequirementLine(line); dep != nil {
			report.Dependencies = append(report.Dependencies, *dep)
		}
	}
	return report, scanner.Err()
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
		return &Dependency{Name: strings.TrimSpace(line[:idx]), Version: strings.TrimSpace(line[idx+1:]), Required: true, Scope: "runtime"}
	}
	for _, sep := range []string{"==", ">=", "<=", "!=", "~=", ">", "<"} {
		if idx := strings.Index(line, sep); idx != -1 {
			return &Dependency{Name: strings.TrimSpace(line[:idx]), Version: strings.TrimSpace(line[idx:]), Required: true, Scope: "runtime"}
		}
	}
	return &Dependency{Name: line, Required: true, Scope: "runtime"}
}

func shouldSkipRequirementLine(line string) bool {
	skipPrefixes := []string{"-r ", "--requirement ", "-c ", "--constraint ", "--index-url", "--extra-index-url", "--find-links", "--trusted-host", "--no-index", "--pre"}
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
			return &Dependency{Name: name, Version: "editable", Required: true, Scope: "runtime"}
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

	report := &ProjectDependencies{Language: "Python", Ecosystem: "python", PackageManager: "pyproject"}
	for _, depStr := range pyproject.Project.Dependencies {
		if dep := parseRequirementLine(depStr); dep != nil {
			report.Dependencies = append(report.Dependencies, *dep)
		}
	}
	for _, optionalDeps := range pyproject.Project.OptionalDependencies {
		for _, depStr := range optionalDeps {
			if dep := parseRequirementLine(depStr); dep != nil {
				dep.Required = false
				dep.Scope = "optional"
				report.Dependencies = append(report.Dependencies, *dep)
			}
		}
	}
	for name, ver := range pyproject.Tool.Poetry.Dependencies {
		if name == "python" {
			continue
		}
		addDependency(&report.Dependencies, name, parseTomlVersion(ver), "runtime", true)
	}
	for name, ver := range pyproject.Tool.Poetry.DevDependencies {
		addDependency(&report.Dependencies, name, parseTomlVersion(ver), "development", false)
	}
	for _, group := range pyproject.Tool.Poetry.Group {
		for name, ver := range group.Dependencies {
			addDependency(&report.Dependencies, name, parseTomlVersion(ver), "development", false)
		}
	}
	return report, nil
}

type poetryDependencyGroup struct {
	Dependencies map[string]interface{} `toml:"dependencies"`
}

// --- Node.js: package.json ---
func analyzeNodePackage(path string) (*ProjectDependencies, error) {
	var pkg struct {
		PackageManager  string            `json:"packageManager"`
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

	report := &ProjectDependencies{Language: "Node.js", Ecosystem: "node", PackageManager: detectNodePackageManager(path, pkg.PackageManager)}
	for name, version := range pkg.Dependencies {
		addDependency(&report.Dependencies, name, version, "runtime", true)
	}
	for name, version := range pkg.DevDependencies {
		addDependency(&report.Dependencies, name, version, "development", false)
	}
	for name, version := range pkg.OptionalDeps {
		addDependency(&report.Dependencies, name, version, "optional", false)
	}
	for name, version := range pkg.PeerDeps {
		addDependency(&report.Dependencies, name, version, "peer", false)
	}
	report.Frameworks = detectNodeFrameworks(report.Dependencies)
	return report, nil
}

func detectNodePackageManager(path, declared string) string {
	if declared != "" {
		return strings.Split(declared, "@")[0]
	}
	dir := filepath.Dir(path)
	switch {
	case fileExists(filepath.Join(dir, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(dir, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(dir, "bun.lock")) || fileExists(filepath.Join(dir, "bun.lockb")):
		return "bun"
	default:
		return "npm"
	}
}

func detectNodeFrameworks(deps []Dependency) []string {
	frameworkMap := map[string]string{
		"react":         "React",
		"vue":           "Vue",
		"@angular/core": "Angular",
		"next":          "Next.js",
		"nuxt":          "Nuxt",
		"svelte":        "Svelte",
		"vite":          "Vite",
		"@nestjs/core":  "NestJS",
		"express":       "Express",
	}
	var frameworks []string
	for _, dep := range deps {
		if framework, ok := frameworkMap[dep.Name]; ok {
			frameworks = append(frameworks, framework)
		}
	}
	return dedupeStrings(frameworks)
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
	report := &ProjectDependencies{Language: "Rust", Ecosystem: "rust", PackageManager: "cargo"}
	for name, ver := range cargo.Dependencies {
		addDependency(&report.Dependencies, name, parseTomlVersion(ver), "runtime", true)
	}
	for name, ver := range cargo.BuildDependencies {
		addDependency(&report.Dependencies, name, parseTomlVersion(ver), "build", true)
	}
	for name, ver := range cargo.DevDependencies {
		addDependency(&report.Dependencies, name, parseTomlVersion(ver), "development", false)
	}
	return report, nil
}

func parseTomlVersion(ver interface{}) string {
	switch v := ver.(type) {
	case string:
		return v
	case map[string]interface{}:
		if verStr, ok := v["version"].(string); ok {
			return verStr
		}
		if path, ok := v["path"].(string); ok {
			return "path:" + path
		}
		if git, ok := v["git"].(string); ok {
			return "git:" + git
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

	report := &ProjectDependencies{Language: "Go", Ecosystem: "go", PackageManager: "go modules"}
	scanner := bufio.NewScanner(file)
	foundRequire := false
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
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
		depStr := strings.TrimPrefix(trimmed, "require ")
		parts := strings.Fields(depStr)
		if len(parts) >= 2 {
			isIndirect := len(parts) >= 3 && parts[2] == "//" && strings.Contains(depStr, "indirect")
			scope := "runtime"
			if isIndirect {
				scope = "indirect"
			}
			addDependency(&report.Dependencies, parts[0], parts[1], scope, !isIndirect)
		}
	}
	return report, scanner.Err()
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
	report := &ProjectDependencies{Language: "PHP", Ecosystem: "php", PackageManager: "composer"}
	for name, version := range composer.Require {
		addDependency(&report.Dependencies, name, version, "runtime", true)
	}
	for name, version := range composer.RequireDev {
		addDependency(&report.Dependencies, name, version, "development", false)
	}
	return report, nil
}

// --- JVM: Maven pom.xml ---
func analyzeMavenPom(path string) (*ProjectDependencies, error) {
	type dependencyXML struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
		Scope      string `xml:"scope"`
		Optional   string `xml:"optional"`
	}
	type projectXML struct {
		Dependencies []dependencyXML `xml:"dependencies>dependency"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var project projectXML
	if err := xml.Unmarshal(data, &project); err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "Java/Kotlin", Ecosystem: "jvm", PackageManager: "maven"}
	for _, dep := range project.Dependencies {
		name := dep.ArtifactID
		if dep.GroupID != "" {
			name = dep.GroupID + ":" + dep.ArtifactID
		}
		scope := defaultValue(dep.Scope, "runtime")
		required := dep.Optional != "true" && scope != "test"
		addDependency(&report.Dependencies, name, dep.Version, scope, required)
	}
	return report, nil
}

// --- JVM: Gradle ---
func analyzeGradleBuild(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "Java/Kotlin", Ecosystem: "jvm", PackageManager: "gradle"}
	for _, line := range strings.Split(string(data), "\n") {
		scope, spec, ok := parseGradleDependencyLine(line)
		if !ok {
			continue
		}
		name, version := splitCoordinate(spec)
		addDependency(&report.Dependencies, name, version, scope, !strings.Contains(scope, "test"))
	}
	return report, nil
}

func parseGradleDependencyLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "//") {
		return "", "", false
	}
	scopes := []string{"implementation", "api", "compileOnly", "runtimeOnly", "testImplementation", "testRuntimeOnly", "kapt"}
	for _, scope := range scopes {
		if !strings.HasPrefix(line, scope) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, scope))
		rest = strings.Trim(rest, "() ")
		rest = strings.Trim(rest, `"'`)
		if rest != "" && strings.Contains(rest, ":") {
			return scope, rest, true
		}
	}
	return "", "", false
}

// --- .NET / NuGet ---
func analyzeDotnetProject(path string) (*ProjectDependencies, error) {
	type packageReference struct {
		Include string `xml:"Include,attr"`
		Update  string `xml:"Update,attr"`
		Version string `xml:"Version,attr"`
	}
	type itemGroup struct {
		Packages []packageReference `xml:"PackageReference"`
	}
	type project struct {
		ItemGroups []itemGroup `xml:"ItemGroup"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var proj project
	if err := xml.Unmarshal(data, &proj); err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: ".NET", Ecosystem: "dotnet", PackageManager: "nuget"}
	for _, group := range proj.ItemGroups {
		for _, pkg := range group.Packages {
			name := defaultValue(pkg.Include, pkg.Update)
			addDependency(&report.Dependencies, name, pkg.Version, "runtime", true)
		}
	}
	return report, nil
}

func analyzeNugetPackagesConfig(path string) (*ProjectDependencies, error) {
	type pkg struct {
		ID      string `xml:"id,attr"`
		Version string `xml:"version,attr"`
	}
	type packages struct {
		Packages []pkg `xml:"package"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var parsed packages
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: ".NET", Ecosystem: "dotnet", PackageManager: "nuget"}
	for _, pkg := range parsed.Packages {
		addDependency(&report.Dependencies, pkg.ID, pkg.Version, "runtime", true)
	}
	return report, nil
}

func analyzeDirectoryPackagesProps(path string) (*ProjectDependencies, error) {
	type packageVersion struct {
		Include string `xml:"Include,attr"`
		Version string `xml:"Version,attr"`
	}
	type itemGroup struct {
		Packages []packageVersion `xml:"PackageVersion"`
	}
	type project struct {
		ItemGroups []itemGroup `xml:"ItemGroup"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var proj project
	if err := xml.Unmarshal(data, &proj); err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: ".NET", Ecosystem: "dotnet", PackageManager: "nuget"}
	for _, group := range proj.ItemGroups {
		for _, pkg := range group.Packages {
			addDependency(&report.Dependencies, pkg.Include, pkg.Version, "runtime", true)
		}
	}
	return report, nil
}

// --- Ruby: Gemfile ---
func analyzeGemfile(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "Ruby", Ecosystem: "ruby", PackageManager: "bundler"}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "gem ") {
			continue
		}
		args := splitQuotedArgs(strings.TrimPrefix(line, "gem "))
		if len(args) == 0 {
			continue
		}
		version := ""
		if len(args) > 1 {
			version = args[1]
		}
		scope := "runtime"
		if strings.Contains(line, "group: :development") || strings.Contains(line, "group: :test") {
			scope = "development"
		}
		addDependency(&report.Dependencies, args[0], version, scope, scope == "runtime")
	}
	return report, nil
}

// --- Dart/Flutter: pubspec.yaml ---
func analyzePubspec(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "Dart/Flutter", Ecosystem: "dart", PackageManager: "pub"}
	current := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			current = strings.TrimSuffix(strings.TrimSpace(line), ":")
			continue
		}
		if current != "dependencies" && current != "dev_dependencies" {
			continue
		}
		name, version, ok := parseYAMLKeyValue(line)
		if ok && name != "sdk" && name != "flutter" {
			required := current == "dependencies"
			scope := "runtime"
			if !required {
				scope = "development"
			}
			addDependency(&report.Dependencies, name, version, scope, required)
		}
	}
	if containsFileText(path, "flutter:") {
		report.Frameworks = append(report.Frameworks, "Flutter")
	}
	return report, nil
}

// --- Swift: Package.swift ---
func analyzeSwiftPackage(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "Swift", Ecosystem: "swift", PackageManager: "swiftpm"}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, ".package(") {
			continue
		}
		name := swiftPackageName(line)
		version := swiftPackageVersion(line)
		addDependency(&report.Dependencies, name, version, "runtime", true)
	}
	return report, nil
}

// --- Elixir: mix.exs ---
func analyzeMixExs(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "Elixir", Ecosystem: "elixir", PackageManager: "mix"}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{:") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) == 0 {
			continue
		}
		name := strings.Trim(strings.TrimPrefix(strings.TrimSpace(parts[0]), "{:"), `" '`)
		version := ""
		if len(parts) > 1 {
			version = strings.Trim(strings.TrimSpace(parts[1]), `" '}`)
		}
		addDependency(&report.Dependencies, name, version, "runtime", true)
	}
	return report, nil
}

// --- Lua: rockspec ---
func analyzeRockspec(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "Lua", Ecosystem: "lua", PackageManager: "luarocks"}
	inDeps := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "dependencies") && strings.Contains(line, "{") {
			inDeps = true
			continue
		}
		if inDeps && strings.Contains(line, "}") {
			inDeps = false
		}
		if inDeps {
			name, version := splitNameVersion(strings.Trim(line, `"',`))
			addDependency(&report.Dependencies, name, version, "runtime", true)
		}
	}
	return report, nil
}

// --- R: DESCRIPTION ---
func analyzeRDescription(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "R", Ecosystem: "r", PackageManager: "cran"}
	fields := parseContinuationFields(string(data))
	for _, key := range []string{"Depends", "Imports", "LinkingTo"} {
		for _, spec := range splitCommaList(fields[key]) {
			name, version := splitNameVersion(spec)
			if name != "R" {
				addDependency(&report.Dependencies, name, version, "runtime", true)
			}
		}
	}
	for _, spec := range splitCommaList(fields["Suggests"]) {
		name, version := splitNameVersion(spec)
		addDependency(&report.Dependencies, name, version, "development", false)
	}
	return report, nil
}

// --- Julia: Project.toml ---
func analyzeJuliaProject(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "Julia", Ecosystem: "julia", PackageManager: "pkg"}
	sections := parseTomlSections(string(data))
	compat := sections["compat"]
	for name := range sections["deps"] {
		addDependency(&report.Dependencies, name, compat[name], "runtime", true)
	}
	for name := range sections["extras"] {
		addDependency(&report.Dependencies, name, compat[name], "development", false)
	}
	return report, nil
}

// --- Haskell: cabal / stack ---
func analyzeCabal(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "Haskell", Ecosystem: "haskell", PackageManager: "cabal"}
	fields := parseContinuationFields(string(data))
	for _, spec := range splitCommaList(fields["build-depends"]) {
		name, version := splitNameVersion(spec)
		addDependency(&report.Dependencies, name, version, "runtime", true)
	}
	return report, nil
}

func analyzeStackYaml(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "Haskell", Ecosystem: "haskell", PackageManager: "stack"}
	inExtraDeps := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "extra-deps:" {
			inExtraDeps = true
			continue
		}
		if inExtraDeps && strings.HasPrefix(trimmed, "- ") {
			spec := strings.TrimPrefix(trimmed, "- ")
			name, version := splitNameVersion(strings.ReplaceAll(spec, "-", " "))
			addDependency(&report.Dependencies, name, version, "runtime", true)
			continue
		}
		if inExtraDeps && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			inExtraDeps = false
		}
	}
	return report, nil
}

// --- Perl: cpanfile ---
func analyzeCpanfile(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "Perl", Ecosystem: "perl", PackageManager: "cpanm"}
	scope := "runtime"
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "on 'test'") || strings.HasPrefix(line, "on \"test\"") {
			scope = "development"
		}
		if strings.HasPrefix(line, "};") {
			scope = "runtime"
		}
		if !strings.HasPrefix(line, "requires ") {
			continue
		}
		args := splitQuotedArgs(strings.TrimPrefix(line, "requires "))
		if len(args) == 0 {
			continue
		}
		version := ""
		if len(args) > 1 {
			version = args[1]
		}
		addDependency(&report.Dependencies, args[0], version, scope, scope == "runtime")
	}
	return report, nil
}

// --- C/C++: vcpkg / conan ---
func analyzeVcpkgJSON(path string) (*ProjectDependencies, error) {
	var manifest struct {
		Dependencies []interface{} `json:"dependencies"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "C/C++", Ecosystem: "cpp", PackageManager: "vcpkg"}
	for _, raw := range manifest.Dependencies {
		switch dep := raw.(type) {
		case string:
			addDependency(&report.Dependencies, dep, "", "runtime", true)
		case map[string]interface{}:
			name, _ := dep["name"].(string)
			version, _ := dep["version>="].(string)
			addDependency(&report.Dependencies, name, version, "runtime", true)
		}
	}
	return report, nil
}

func analyzeConanfileTxt(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "C/C++", Ecosystem: "cpp", PackageManager: "conan"}
	inRequires := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "[requires]" {
			inRequires = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inRequires = false
		}
		if inRequires {
			name, version := splitSlashVersion(line)
			addDependency(&report.Dependencies, name, version, "runtime", true)
		}
	}
	return report, nil
}

func analyzeConanfilePy(path string) (*ProjectDependencies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &ProjectDependencies{Language: "C/C++", Ecosystem: "cpp", PackageManager: "conan"}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "requires ="):
			args := splitQuotedArgs(strings.TrimPrefix(line, "requires ="))
			for _, arg := range args {
				name, version := splitSlashVersion(arg)
				addDependency(&report.Dependencies, name, version, "runtime", true)
			}
		case strings.Contains(line, "self.requires("):
			start := strings.Index(line, "self.requires(")
			args := splitQuotedArgs(line[start:])
			for _, arg := range args {
				name, version := splitSlashVersion(arg)
				addDependency(&report.Dependencies, name, version, "runtime", true)
			}
		}
	}
	return report, nil
}

// --- Validation ---
func validatePythonDeps(proj *ProjectDependencies, dir string) *ProjectDependencies {
	for i, dep := range proj.Dependencies {
		pkgName := stripPythonConstraint(dep.Name)
		out, err := command.Output("pip3", "show", pkgName)
		if err != nil {
			out, err = command.Output("pip", "show", pkgName)
		}
		if err == nil {
			proj.Dependencies[i].Installed = true
			proj.Dependencies[i].InstalledVersion = lineValue(string(out), "Version: ")
		}
	}
	return proj
}

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

func validateRustDeps(proj *ProjectDependencies, dir string) *ProjectDependencies {
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

func validatePHPDeps(proj *ProjectDependencies, dir string) *ProjectDependencies {
	for i, dep := range proj.Dependencies {
		out, err := command.OutputInDir(dir, "composer", "show", dep.Name)
		if err == nil {
			proj.Dependencies[i].Installed = true
			proj.Dependencies[i].InstalledVersion = strings.TrimSpace(lineValue(string(out), "versions : "))
		}
	}
	return proj
}

// --- Formatting ---
func PrintDependencies(deps *ProjectDependencies) {
	fmt.Printf("Language: %s\n", deps.Language)
	fmt.Printf("Ecosystem: %s\n", deps.Ecosystem)
	fmt.Printf("Package manager: %s\n", deps.PackageManager)
	fmt.Printf("Manifest: %s (%s)\n", deps.SourceFile, deps.ManifestType)
	fmt.Printf("Validation: %s | Status: %s | Scope: %s\n", deps.ValidationStatus, deps.Status, deps.Scope)
	if len(deps.Frameworks) > 0 {
		fmt.Printf("Frameworks: %s\n", strings.Join(deps.Frameworks, ", "))
	}
	fmt.Println("Dependencies:")
	if len(deps.Dependencies) == 0 {
		fmt.Println("  none detected")
		return
	}
	for _, dep := range deps.Dependencies {
		required := "optional"
		if dep.Required {
			required = "required"
		}
		fmt.Printf("  [%s:%s] %s\n", required, defaultValue(dep.Scope, "-"), formatDependencyNameVersion(dep))
		if deps.ValidationStatus == validationMetadataOnly {
			fmt.Println("    metadata-only")
		} else if dep.Installed {
			fmt.Printf("    installed: %s\n", dep.InstalledVersion)
		} else {
			fmt.Println("    not installed")
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

func PrintAllDependencies(reports []*ProjectDependencies) {
	for i, deps := range reports {
		if i > 0 {
			fmt.Println()
		}
		PrintDependencies(deps)
	}
}

// --- Helpers ---
func stripPythonConstraint(name string) string {
	for _, sep := range []string{">=", "<=", ">", "<", "==", "!=", "~=", " ", "[", ";", "@"} {
		if idx := strings.Index(name, sep); idx != -1 {
			name = strings.TrimSpace(name[:idx])
			break
		}
	}
	return name
}

func lineValue(out, prefix string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

func splitCoordinate(spec string) (string, string) {
	parts := strings.Split(spec, ":")
	if len(parts) >= 3 {
		return parts[0] + ":" + parts[1], parts[2]
	}
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return spec, ""
}

func splitSlashVersion(spec string) (string, string) {
	spec = strings.TrimSpace(strings.Trim(spec, `"',`))
	parts := strings.Split(spec, "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return spec, ""
}

func splitNameVersion(spec string) (string, string) {
	spec = strings.TrimSpace(strings.Trim(spec, `"',`))
	for _, sep := range []string{" >=", " <=", " ==", " >", " <", " ~>", " ^", " ("} {
		if idx := strings.Index(spec, sep); idx != -1 {
			return strings.TrimSpace(spec[:idx]), strings.Trim(strings.TrimSpace(spec[idx:]), "()")
		}
	}
	parts := strings.Fields(spec)
	if len(parts) >= 2 {
		return parts[0], strings.Join(parts[1:], " ")
	}
	return spec, ""
}

func splitQuotedArgs(s string) []string {
	var args []string
	var current strings.Builder
	inQuote := rune(0)
	for _, ch := range s {
		switch {
		case ch == '\'' || ch == '"':
			if inQuote == 0 {
				inQuote = ch
				current.Reset()
			} else if inQuote == ch {
				args = append(args, current.String())
				current.Reset()
				inQuote = 0
			} else if inQuote != 0 {
				current.WriteRune(ch)
			}
		case inQuote != 0:
			current.WriteRune(ch)
		}
	}
	return args
}

func parseYAMLKeyValue(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "- ") {
		trimmed = strings.TrimPrefix(trimmed, "- ")
	}
	key, value, ok := strings.Cut(trimmed, ":")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.Trim(strings.TrimSpace(value), `"'`)
	if key == "" || strings.Contains(key, " ") {
		return "", "", false
	}
	return key, value, true
}

func swiftPackageName(line string) string {
	if idx := strings.Index(line, "name:"); idx != -1 {
		args := splitQuotedArgs(line[idx:])
		if len(args) > 0 {
			return args[0]
		}
	}
	if idx := strings.Index(line, "url:"); idx != -1 {
		args := splitQuotedArgs(line[idx:])
		if len(args) > 0 {
			base := filepath.Base(strings.TrimSuffix(args[0], ".git"))
			return base
		}
	}
	return ""
}

func swiftPackageVersion(line string) string {
	for _, marker := range []string{"from:", "exact:", "branch:", "revision:"} {
		if idx := strings.Index(line, marker); idx != -1 {
			args := splitQuotedArgs(line[idx:])
			if len(args) > 0 {
				return args[0]
			}
		}
	}
	return ""
}

func parseContinuationFields(content string) map[string]string {
	fields := make(map[string]string)
	current := ""
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if current != "" {
				fields[current] += " " + strings.TrimSpace(line)
			}
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		current = strings.TrimSpace(key)
		fields[current] = strings.TrimSpace(value)
	}
	return fields
}

func parseTomlSections(content string) map[string]map[string]string {
	sections := make(map[string]map[string]string)
	current := ""
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.Trim(line, "[]")
			if sections[current] == nil {
				sections[current] = make(map[string]string)
			}
			continue
		}
		if current == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			sections[current][strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	return sections
}

func splitCommaList(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func containsFileText(path, needle string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), needle)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func defaultValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
