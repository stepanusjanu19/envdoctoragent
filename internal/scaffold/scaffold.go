package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stepanusjanu19/envdoctoragent/internal/executor"
)

const (
	SourceAuto     = "auto"
	SourceInternal = "internal"
	SourceOfficial = "official"
	SourceManual   = "manual"
)

// Options controls scaffold template selection and file safety.
type Options struct {
	Name          string
	Module        string
	PackageName   string
	Source        string
	Force         bool
	CreateDir     bool
	AllowNonEmpty bool
}

// FileMetadata is public scaffold metadata without file contents.
type FileMetadata struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

// Template describes one scaffold entry.
type Template struct {
	ID               string         `json:"id"`
	Language         string         `json:"language"`
	Framework        string         `json:"framework,omitempty"`
	Ecosystem        string         `json:"ecosystem"`
	PackageManager   string         `json:"package_manager,omitempty"`
	Source           string         `json:"source"`
	AvailableSources []string       `json:"available_sources"`
	RequiresNetwork  bool           `json:"requires_network"`
	Files            []FileMetadata `json:"files,omitempty"`
	Summary          string         `json:"summary"`
}

// Plan is produced by the scaffold registry and embedded by projectops.
type Plan struct {
	Directory       string         `json:"directory"`
	Template        Template       `json:"template"`
	SelectedSource  string         `json:"selected_source"`
	RequiresNetwork bool           `json:"requires_network"`
	Files           []FileMetadata `json:"files,omitempty"`
	Actions         []executor.Action
	Summary         string `json:"summary"`
}

type fileTemplate struct {
	Path    string
	Content string
}

type context struct {
	Name        string
	Slug        string
	Snake       string
	Pascal      string
	Module      string
	PackageName string
	PackagePath string
}

type templateDef struct {
	ID              string
	Language        string
	Framework       string
	Ecosystem       string
	PackageManager  string
	DefaultSource   string
	RequiresNetwork bool
	Summary         string
	Internal        func(context) []fileTemplate
	Official        func(context) []executor.Action
	ManualSteps     string
}

// List returns supported scaffold templates with default metadata.
func List() []Template {
	defs := registry()
	keys := make([]string, 0, len(defs))
	for key := range defs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	templates := make([]Template, 0, len(keys))
	for _, key := range keys {
		def := defs[key]
		ctx := defaultContext(def.ID, "")
		templates = append(templates, def.metadata(ctx, def.DefaultSource))
	}
	return templates
}

// Generate creates a scaffold plan. It never writes files or runs commands.
func Generate(template, dir string, options Options) (*Plan, error) {
	template = normalizeTemplate(template)
	if template == "" || strings.ContainsAny(template, `/\`) || strings.Contains(template, "..") {
		return nil, fmt.Errorf("invalid project template id: %s", template)
	}
	def, ok := registry()[template]
	if !ok {
		return nil, fmt.Errorf("unsupported project init template: %s", template)
	}

	absDir, exists, err := prepareDirectory(dir, options.CreateDir)
	if err != nil {
		return nil, err
	}
	if exists && !options.AllowNonEmpty && !isDirectoryEmpty(absDir) {
		return nil, fmt.Errorf("project init is blocked for non-empty directory %s; use --allow-non-empty to override", absDir)
	}

	ctx := defaultContext(absDir, options.Name)
	if strings.TrimSpace(options.Module) != "" {
		ctx.Module = strings.TrimSpace(options.Module)
	}
	if strings.TrimSpace(options.PackageName) != "" {
		ctx.PackageName = sanitizePackage(options.PackageName)
		ctx.PackagePath = strings.ReplaceAll(ctx.PackageName, ".", "/")
	}

	source, err := chooseSource(def, options.Source)
	if err != nil {
		return nil, err
	}

	var files []fileTemplate
	var actions []executor.Action
	if !exists && options.CreateDir {
		actions = append(actions, mkdirAction("Create project directory", "."))
	}

	switch source {
	case SourceInternal:
		files = def.Internal(ctx)
		if err := validateFiles(absDir, files, options.Force); err != nil {
			return nil, err
		}
		for _, file := range files {
			actions = append(actions, writeFileAction("Write "+file.Path, file.Path, file.Content, options.Force))
		}
	case SourceOfficial:
		actions = append(actions, def.Official(ctx)...)
	case SourceManual:
		actions = append(actions, manualAction("Manual scaffold required", manualSteps(def)))
	}

	for i := range actions {
		actions[i] = finalizeAction(actions[i], absDir, def, source)
	}

	metadata := def.metadata(ctx, source)
	metadata.Files = fileMetadata(files)
	return &Plan{
		Directory:       absDir,
		Template:        metadata,
		SelectedSource:  source,
		RequiresNetwork: source == SourceOfficial && def.RequiresNetwork,
		Files:           metadata.Files,
		Actions:         actions,
		Summary:         fmt.Sprintf("Generated %d scaffold action(s) for %s using %s source. No commands were executed.", len(actions), def.ID, source),
	}, nil
}

func registry() map[string]templateDef {
	defs := []templateDef{
		internalTemplate("node", "Node.js", "", "node", "npm", "Minimal Node.js application", nodeFiles),
		internalTemplate("express", "Node.js", "Express", "node", "npm", "Express HTTP API starter", expressFiles),
		internalTemplate("react-vite", "Node.js", "React + Vite", "node", "npm", "React Vite starter", reactViteFiles).withOfficial(true, official("Run Vite React generator", "npm", "create", "vite@latest", ".", "--", "--template", "react")),
		internalTemplate("vue-vite", "Node.js", "Vue + Vite", "node", "npm", "Vue Vite starter", vueViteFiles).withOfficial(true, official("Run Vite Vue generator", "npm", "create", "vite@latest", ".", "--", "--template", "vue")),
		internalTemplate("next", "Node.js", "Next.js", "node", "npm", "Next.js app router starter", nextFiles).withOfficial(true, official("Run Next.js generator", "npx", "create-next-app@latest", ".", "--yes", "--use-npm", "--typescript", "--eslint", "--app", "--src-dir")),
		internalTemplate("sveltekit", "Node.js", "SvelteKit", "node", "npm", "SvelteKit starter", svelteKitFiles),
		internalTemplate("nestjs", "Node.js", "NestJS", "node", "npm", "NestJS starter", nestFiles).withOfficial(true, official("Run NestJS generator", "npx", "@nestjs/cli", "new", ".", "--package-manager", "npm", "--skip-git", "--strict")),
		internalTemplate("python", "Python", "", "python", "pip", "Python package starter", pythonPackageFiles),
		internalTemplate("python-cli", "Python", "CLI", "python", "pip", "Python CLI starter", pythonCLIFiles),
		internalTemplate("fastapi", "Python", "FastAPI", "python", "pip", "FastAPI application starter", fastAPIFiles),
		internalTemplate("flask", "Python", "Flask", "python", "pip", "Flask application starter", flaskFiles),
		internalTemplate("go", "Go", "Module", "go", "go modules", "Go module starter", goModuleFiles),
		internalTemplate("go-module", "Go", "Module", "go", "go modules", "Go module starter", goModuleFiles),
		internalTemplate("go-cli", "Go", "CLI", "go", "go modules", "Go CLI starter", goCLIFiles),
		internalTemplate("go-web", "Go", "HTTP", "go", "go modules", "Go HTTP server starter", goWebFiles),
		internalTemplate("rust", "Rust", "CLI", "rust", "cargo", "Rust CLI starter", rustCLIFiles).withOfficial(false, official("Run Cargo init", "cargo", "init", ".")),
		internalTemplate("rust-cli", "Rust", "CLI", "rust", "cargo", "Rust CLI starter", rustCLIFiles).withOfficial(false, official("Run Cargo init", "cargo", "init", ".")),
		internalTemplate("rust-lib", "Rust", "Library", "rust", "cargo", "Rust library starter", rustLibFiles),
		internalTemplate("rust-web", "Rust", "Axum", "rust", "cargo", "Rust web service starter", rustWebFiles),
		internalTemplate("php", "PHP", "Composer", "php", "composer", "PHP Composer starter", phpComposerFiles),
		internalTemplate("php-composer", "PHP", "Composer", "php", "composer", "PHP Composer starter", phpComposerFiles),
		manualTemplate("laravel", "PHP", "Laravel", "php", "composer", "Run composer create-project laravel/laravel after confirming network and PHP extension requirements."),
		internalTemplate("java-maven", "Java", "Maven", "jvm", "maven", "Java Maven starter", javaMavenFiles),
		internalTemplate("java-gradle", "Java", "Gradle", "jvm", "gradle", "Java Gradle starter", javaGradleFiles),
		internalTemplate("kotlin-gradle", "Kotlin", "Gradle", "jvm", "gradle", "Kotlin Gradle starter", kotlinGradleFiles),
		internalTemplate("spring-boot", "Java", "Spring Boot", "jvm", "maven", "Spring Boot starter", springBootFiles),
		internalTemplate("dotnet", ".NET", "Console", "dotnet", "dotnet", ".NET console starter", dotnetConsoleFiles).withOfficial(false, official("Run dotnet console generator", "dotnet", "new", "console", "--output", ".")),
		internalTemplate("dotnet-console", ".NET", "Console", "dotnet", "dotnet", ".NET console starter", dotnetConsoleFiles).withOfficial(false, official("Run dotnet console generator", "dotnet", "new", "console", "--output", ".")),
		internalTemplate("dotnet-webapi", ".NET", "Web API", "dotnet", "dotnet", ".NET minimal API starter", dotnetWebAPIFiles).withOfficial(false, official("Run dotnet webapi generator", "dotnet", "new", "webapi", "--output", ".")),
		internalTemplate("dart", "Dart", "Console", "dart", "pub", "Dart console starter", dartConsoleFiles).withOfficial(false, official("Run Dart generator", "dart", "create", ".")),
		internalTemplate("dart-console", "Dart", "Console", "dart", "pub", "Dart console starter", dartConsoleFiles).withOfficial(false, official("Run Dart generator", "dart", "create", ".")),
		officialOnly("flutter", "Dart", "Flutter", "dart", "flutter", "Flutter app starter", true, official("Run Flutter generator", "flutter", "create", ".")),
		officialOnly("flutter-app", "Dart", "Flutter", "dart", "flutter", "Flutter app starter", true, official("Run Flutter generator", "flutter", "create", ".")),
		internalTemplate("swift", "Swift", "SwiftPM", "swift", "swiftpm", "Swift executable starter", swiftFiles).withOfficial(false, official("Run SwiftPM init", "swift", "package", "init", "--type", "executable")),
		internalTemplate("elixir", "Elixir", "Mix", "elixir", "mix", "Elixir Mix starter", elixirFiles).withOfficial(false, official("Run Mix generator", "mix", "new", ".")),
		internalTemplate("ruby", "Ruby", "Bundler", "ruby", "bundler", "Ruby starter", rubyFiles),
		internalTemplate("c-cli", "C", "CLI", "c", "make", "C CLI starter", cFiles),
		internalTemplate("cpp-cli", "C++", "CMake", "cpp", "cmake", "C++ CMake starter", cppFiles),
		internalTemplate("lua", "Lua", "LuaRocks", "lua", "luarocks", "Lua starter", luaFiles),
		internalTemplate("r", "R", "Package", "r", "r", "R package starter", rFiles),
		internalTemplate("julia", "Julia", "Package", "julia", "julia", "Julia package starter", juliaFiles),
		internalTemplate("haskell", "Haskell", "Cabal", "haskell", "cabal", "Haskell Cabal starter", haskellFiles),
		internalTemplate("perl", "Perl", "CPAN", "perl", "cpan", "Perl starter", perlFiles),
	}
	result := make(map[string]templateDef, len(defs))
	for _, def := range defs {
		result[def.ID] = def
	}
	return result
}

func internalTemplate(id, language, framework, ecosystem, manager, summary string, files func(context) []fileTemplate) templateDef {
	return templateDef{ID: id, Language: language, Framework: framework, Ecosystem: ecosystem, PackageManager: manager, DefaultSource: SourceInternal, Summary: summary, Internal: files}
}

func manualTemplate(id, language, framework, ecosystem, manager, steps string) templateDef {
	return templateDef{ID: id, Language: language, Framework: framework, Ecosystem: ecosystem, PackageManager: manager, DefaultSource: SourceManual, Summary: framework + " manual scaffold", ManualSteps: steps}
}

func officialOnly(id, language, framework, ecosystem, manager, summary string, network bool, actions func(context) []executor.Action) templateDef {
	return templateDef{ID: id, Language: language, Framework: framework, Ecosystem: ecosystem, PackageManager: manager, DefaultSource: SourceOfficial, RequiresNetwork: network, Summary: summary, Official: actions}
}

func (def templateDef) withOfficial(network bool, actions func(context) []executor.Action) templateDef {
	def.Official = actions
	def.RequiresNetwork = network
	return def
}

func (def templateDef) metadata(ctx context, source string) Template {
	files := []FileMetadata{}
	if def.Internal != nil {
		files = fileMetadata(def.Internal(ctx))
	}
	selectedSource := valueOrDefault(source, def.DefaultSource)
	return Template{
		ID:               def.ID,
		Language:         def.Language,
		Framework:        def.Framework,
		Ecosystem:        def.Ecosystem,
		PackageManager:   def.PackageManager,
		Source:           selectedSource,
		AvailableSources: availableSources(def),
		RequiresNetwork:  selectedSource == SourceOfficial && def.RequiresNetwork,
		Files:            files,
		Summary:          def.Summary,
	}
}

func chooseSource(def templateDef, requested string) (string, error) {
	requested = normalizeTemplate(valueOrDefault(requested, SourceAuto))
	if requested == SourceAuto {
		return def.DefaultSource, nil
	}
	switch requested {
	case SourceInternal:
		if def.Internal == nil {
			return "", fmt.Errorf("template %s does not provide an internal scaffold", def.ID)
		}
	case SourceOfficial:
		if def.Official == nil {
			return "", fmt.Errorf("template %s does not provide an official generator", def.ID)
		}
	case SourceManual:
		if def.ManualSteps == "" {
			return "", fmt.Errorf("template %s does not provide manual scaffold steps", def.ID)
		}
	default:
		return "", fmt.Errorf("unsupported scaffold source %q; use auto, internal, official, or manual", requested)
	}
	return requested, nil
}

func availableSources(def templateDef) []string {
	var sources []string
	if def.Internal != nil {
		sources = append(sources, SourceInternal)
	}
	if def.Official != nil {
		sources = append(sources, SourceOfficial)
	}
	if def.ManualSteps != "" {
		sources = append(sources, SourceManual)
	}
	return sources
}

func prepareDirectory(dir string, create bool) (string, bool, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(absDir)
	if err == nil {
		if !info.IsDir() {
			return "", false, fmt.Errorf("not a directory: %s", absDir)
		}
		return absDir, true, nil
	}
	if os.IsNotExist(err) && create {
		parent := filepath.Dir(absDir)
		if parentInfo, parentErr := os.Stat(parent); parentErr != nil || !parentInfo.IsDir() {
			return "", false, fmt.Errorf("parent directory does not exist: %s", parent)
		}
		return absDir, false, nil
	}
	if os.IsNotExist(err) {
		return "", false, fmt.Errorf("project directory does not exist: %s; pass --create-dir to create it", absDir)
	}
	return "", false, err
}

func validateFiles(root string, files []fileTemplate, force bool) error {
	seen := map[string]bool{}
	for _, file := range files {
		clean, err := cleanRelativePath(file.Path)
		if err != nil {
			return err
		}
		if seen[clean] {
			return fmt.Errorf("duplicate scaffold file path: %s", file.Path)
		}
		seen[clean] = true
		target := filepath.Join(root, filepath.FromSlash(clean))
		if _, err := os.Stat(target); err == nil && !force {
			return fmt.Errorf("scaffold file exists; pass --force to overwrite: %s", file.Path)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func finalizeAction(action executor.Action, dir string, def templateDef, source string) executor.Action {
	action.Source = valueOrDefault(action.Source, "scaffold")
	action.Category = valueOrDefault(action.Category, "Project")
	action.Operation = "init"
	action.Ecosystem = def.Ecosystem
	action.PackageManager = def.PackageManager
	action.WorkingDir = dir
	action.Risk = valueOrDefault(action.Risk, "medium")
	action.SafeToRun = false
	action.CreatesProject = true
	action.MutatesProject = true
	action.Status = valueOrDefault(action.Status, "plan-only")
	action.Timeout = valueOrDefault(action.Timeout, "5m")
	action.RollbackHint = valueOrDefault(action.RollbackHint, "Use the project snapshot and version control to inspect and manually revert scaffold changes.")
	if action.ID == "" {
		action.ID = fmt.Sprintf("scaffold-%s-%s-%s", def.ID, source, slug(action.Title))
	}
	return action
}

func mkdirAction(title, path string) executor.Action {
	return executor.Action{Type: "mkdir", Title: title, Path: path, Source: "scaffold"}
}

func writeFileAction(title, path, content string, overwrite bool) executor.Action {
	return executor.Action{Type: "write_file", Title: title, Path: path, Content: content, ContentBytes: len([]byte(content)), Overwrite: overwrite, Source: "scaffold"}
}

func manualAction(title, steps string) executor.Action {
	return executor.Action{Type: "manual", Title: title, ManualSteps: steps, Source: "scaffold", Risk: "low", Status: "metadata-only"}
}

func official(title, command string, args ...string) func(context) []executor.Action {
	return func(context) []executor.Action {
		return []executor.Action{{Type: "command", Title: title, Command: command, Args: args, Source: "scaffold"}}
	}
}

func fileMetadata(files []fileTemplate) []FileMetadata {
	result := make([]FileMetadata, 0, len(files))
	for _, file := range files {
		result = append(result, FileMetadata{Path: file.Path, Bytes: len([]byte(file.Content))})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

func defaultContext(dir, name string) context {
	base := filepath.Base(strings.TrimSpace(dir))
	if strings.TrimSpace(name) != "" {
		base = name
	}
	s := slug(base)
	snake := strings.ReplaceAll(s, "-", "_")
	pascal := pascalCase(s)
	pkg := "com.example." + snake
	return context{
		Name:        s,
		Slug:        s,
		Snake:       snake,
		Pascal:      pascal,
		Module:      "example.com/" + s,
		PackageName: pkg,
		PackagePath: strings.ReplaceAll(pkg, ".", "/"),
	}
}

func nodeFiles(ctx context) []fileTemplate {
	return files(
		"package.json", fmt.Sprintf("{\n  \"name\": %q,\n  \"version\": \"0.1.0\",\n  \"type\": \"module\",\n  \"scripts\": {\n    \"start\": \"node src/index.js\"\n  }\n}\n", ctx.Slug),
		"src/index.js", "console.log('Hello from "+ctx.Slug+"');\n",
		"README.md", "# "+ctx.Name+"\n\nNode.js starter generated by envdoctor.\n",
	)
}

func expressFiles(ctx context) []fileTemplate {
	return files(
		"package.json", fmt.Sprintf("{\n  \"name\": %q,\n  \"version\": \"0.1.0\",\n  \"type\": \"module\",\n  \"scripts\": {\n    \"start\": \"node src/server.js\",\n    \"dev\": \"node --watch src/server.js\"\n  },\n  \"dependencies\": {\n    \"express\": \"^5.0.0\"\n  }\n}\n", ctx.Slug),
		"src/server.js", "import express from 'express';\n\nconst app = express();\nconst port = process.env.PORT || 3000;\n\napp.get('/health', (req, res) => res.json({ status: 'ok' }));\napp.listen(port, () => console.log(`listening on ${port}`));\n",
	)
}

func reactViteFiles(ctx context) []fileTemplate {
	return files(
		"package.json", fmt.Sprintf("{\n  \"name\": %q,\n  \"version\": \"0.1.0\",\n  \"type\": \"module\",\n  \"scripts\": {\n    \"dev\": \"vite\",\n    \"build\": \"vite build\",\n    \"preview\": \"vite preview\"\n  },\n  \"dependencies\": {\n    \"@vitejs/plugin-react\": \"^5.0.0\",\n    \"vite\": \"^7.0.0\",\n    \"react\": \"^19.0.0\",\n    \"react-dom\": \"^19.0.0\"\n  }\n}\n", ctx.Slug),
		"index.html", "<div id=\"root\"></div><script type=\"module\" src=\"/src/main.jsx\"></script>\n",
		"src/main.jsx", "import React from 'react';\nimport { createRoot } from 'react-dom/client';\nimport App from './App.jsx';\nimport './style.css';\n\ncreateRoot(document.getElementById('root')).render(<App />);\n",
		"src/App.jsx", "export default function App() {\n  return <main><h1>"+ctx.Name+"</h1><p>React starter generated by envdoctor.</p></main>;\n}\n",
		"src/style.css", "body { font-family: system-ui, sans-serif; margin: 2rem; }\n",
	)
}

func vueViteFiles(ctx context) []fileTemplate {
	return files(
		"package.json", fmt.Sprintf("{\n  \"name\": %q,\n  \"version\": \"0.1.0\",\n  \"type\": \"module\",\n  \"scripts\": { \"dev\": \"vite\", \"build\": \"vite build\", \"preview\": \"vite preview\" },\n  \"dependencies\": { \"@vitejs/plugin-vue\": \"^6.0.0\", \"vite\": \"^7.0.0\", \"vue\": \"^3.5.0\" }\n}\n", ctx.Slug),
		"index.html", "<div id=\"app\"></div><script type=\"module\" src=\"/src/main.js\"></script>\n",
		"src/main.js", "import { createApp } from 'vue';\nimport App from './App.vue';\n\ncreateApp(App).mount('#app');\n",
		"src/App.vue", "<template><main><h1>"+ctx.Name+"</h1><p>Vue starter generated by envdoctor.</p></main></template>\n",
	)
}

func nextFiles(ctx context) []fileTemplate {
	return files(
		"package.json", fmt.Sprintf("{\n  \"name\": %q,\n  \"version\": \"0.1.0\",\n  \"scripts\": { \"dev\": \"next dev\", \"build\": \"next build\", \"start\": \"next start\" },\n  \"dependencies\": { \"next\": \"^15.0.0\", \"react\": \"^19.0.0\", \"react-dom\": \"^19.0.0\" },\n  \"devDependencies\": { \"typescript\": \"^5.0.0\" }\n}\n", ctx.Slug),
		"app/layout.tsx", "export default function RootLayout({ children }: { children: React.ReactNode }) {\n  return <html lang=\"en\"><body>{children}</body></html>;\n}\n",
		"app/page.tsx", "export default function Page() {\n  return <main><h1>"+ctx.Name+"</h1><p>Next.js starter generated by envdoctor.</p></main>;\n}\n",
		"tsconfig.json", "{\n  \"compilerOptions\": { \"jsx\": \"preserve\", \"strict\": true, \"moduleResolution\": \"bundler\" }\n}\n",
	)
}

func svelteKitFiles(ctx context) []fileTemplate {
	return files(
		"package.json", fmt.Sprintf("{\n  \"name\": %q,\n  \"version\": \"0.1.0\",\n  \"type\": \"module\",\n  \"scripts\": { \"dev\": \"vite\", \"build\": \"vite build\", \"preview\": \"vite preview\" },\n  \"devDependencies\": { \"@sveltejs/kit\": \"^2.0.0\", \"@sveltejs/adapter-auto\": \"^3.0.0\", \"svelte\": \"^5.0.0\", \"vite\": \"^7.0.0\" }\n}\n", ctx.Slug),
		"src/app.html", "<div>%sveltekit.body%</div>\n",
		"src/routes/+page.svelte", "<h1>"+ctx.Name+"</h1>\n<p>SvelteKit starter generated by envdoctor.</p>\n",
	)
}

func nestFiles(ctx context) []fileTemplate {
	return files(
		"package.json", fmt.Sprintf("{\n  \"name\": %q,\n  \"version\": \"0.1.0\",\n  \"scripts\": { \"start\": \"nest start\", \"start:dev\": \"nest start --watch\" },\n  \"dependencies\": { \"@nestjs/common\": \"^11.0.0\", \"@nestjs/core\": \"^11.0.0\", \"reflect-metadata\": \"^0.2.0\", \"rxjs\": \"^7.8.0\" },\n  \"devDependencies\": { \"@nestjs/cli\": \"^11.0.0\", \"typescript\": \"^5.0.0\" }\n}\n", ctx.Slug),
		"src/main.ts", "import { NestFactory } from '@nestjs/core';\nimport { AppModule } from './app.module';\n\nasync function bootstrap() {\n  const app = await NestFactory.create(AppModule);\n  await app.listen(process.env.PORT || 3000);\n}\nbootstrap();\n",
		"src/app.module.ts", "import { Module } from '@nestjs/common';\n\n@Module({})\nexport class AppModule {}\n",
		"tsconfig.json", "{\n  \"compilerOptions\": { \"module\": \"commonjs\", \"target\": \"ES2022\", \"experimentalDecorators\": true, \"emitDecoratorMetadata\": true }\n}\n",
	)
}

func pythonPackageFiles(ctx context) []fileTemplate {
	return files(
		"pyproject.toml", fmt.Sprintf("[project]\nname = %q\nversion = \"0.1.0\"\nrequires-python = \">=3.11\"\n\n[build-system]\nrequires = [\"setuptools>=68\"]\nbuild-backend = \"setuptools.build_meta\"\n", ctx.Slug),
		"src/"+ctx.Snake+"/__init__.py", "__version__ = '0.1.0'\n",
		"README.md", "# "+ctx.Name+"\n\nPython starter generated by envdoctor.\n",
	)
}

func pythonCLIFiles(ctx context) []fileTemplate {
	return files(
		"pyproject.toml", fmt.Sprintf("[project]\nname = %q\nversion = \"0.1.0\"\nrequires-python = \">=3.11\"\n\n[project.scripts]\n%s = \"%s.__main__:main\"\n", ctx.Slug, ctx.Slug, ctx.Snake),
		"src/"+ctx.Snake+"/__main__.py", "def main():\n    print('Hello from "+ctx.Slug+"')\n\nif __name__ == '__main__':\n    main()\n",
	)
}

func fastAPIFiles(ctx context) []fileTemplate {
	return files(
		"pyproject.toml", fmt.Sprintf("[project]\nname = %q\nversion = \"0.1.0\"\nrequires-python = \">=3.11\"\ndependencies = [\"fastapi>=0.115\", \"uvicorn[standard]>=0.30\"]\n", ctx.Slug),
		"src/app/main.py", "from fastapi import FastAPI\n\napp = FastAPI()\n\n@app.get('/health')\ndef health():\n    return {'status': 'ok'}\n",
	)
}

func flaskFiles(ctx context) []fileTemplate {
	return files(
		"pyproject.toml", fmt.Sprintf("[project]\nname = %q\nversion = \"0.1.0\"\nrequires-python = \">=3.11\"\ndependencies = [\"flask>=3.0\"]\n", ctx.Slug),
		"src/app.py", "from flask import Flask\n\napp = Flask(__name__)\n\n@app.get('/health')\ndef health():\n    return {'status': 'ok'}\n",
	)
}

func goModuleFiles(ctx context) []fileTemplate {
	return files("go.mod", "module "+ctx.Module+"\n\ngo 1.22\n", "README.md", "# "+ctx.Name+"\n\nGo module starter generated by envdoctor.\n")
}

func goCLIFiles(ctx context) []fileTemplate {
	return files("go.mod", "module "+ctx.Module+"\n\ngo 1.22\n", "main.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello from "+ctx.Slug+"\")\n}\n")
}

func goWebFiles(ctx context) []fileTemplate {
	return files("go.mod", "module "+ctx.Module+"\n\ngo 1.22\n", "main.go", "package main\n\nimport (\n\t\"encoding/json\"\n\t\"log\"\n\t\"net/http\"\n)\n\nfunc main() {\n\thttp.HandleFunc(\"/health\", func(w http.ResponseWriter, r *http.Request) {\n\t\t_ = json.NewEncoder(w).Encode(map[string]string{\"status\": \"ok\"})\n\t})\n\tlog.Println(\"listening on :8080\")\n\tlog.Fatal(http.ListenAndServe(\":8080\", nil))\n}\n")
}

func rustCLIFiles(ctx context) []fileTemplate {
	return files("Cargo.toml", "[package]\nname = \""+ctx.Slug+"\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\n", "src/main.rs", "fn main() {\n    println!(\"Hello from "+ctx.Slug+"\");\n}\n")
}

func rustLibFiles(ctx context) []fileTemplate {
	return files("Cargo.toml", "[package]\nname = \""+ctx.Slug+"\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\n", "src/lib.rs", "pub fn health() -> &'static str {\n    \"ok\"\n}\n")
}

func rustWebFiles(ctx context) []fileTemplate {
	return files("Cargo.toml", "[package]\nname = \""+ctx.Slug+"\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\naxum = \"0.8\"\ntokio = { version = \"1\", features = [\"full\"] }\n", "src/main.rs", "use axum::{routing::get, Json, Router};\nuse std::collections::HashMap;\n\n#[tokio::main]\nasync fn main() {\n    let app = Router::new().route(\"/health\", get(health));\n    let listener = tokio::net::TcpListener::bind(\"0.0.0.0:3000\").await.unwrap();\n    axum::serve(listener, app).await.unwrap();\n}\n\nasync fn health() -> Json<HashMap<&'static str, &'static str>> {\n    Json(HashMap::from([(\"status\", \"ok\")]))\n}\n")
}

func phpComposerFiles(ctx context) []fileTemplate {
	return files("composer.json", fmt.Sprintf("{\n  \"name\": %q,\n  \"type\": \"project\",\n  \"autoload\": { \"psr-4\": { \"App\\\\\\\\\": \"src/\" } },\n  \"require\": {}\n}\n", "envdoctor/"+ctx.Slug), "src/index.php", "<?php\n\necho \"Hello from "+ctx.Slug+"\\n\";\n")
}

func javaMavenFiles(ctx context) []fileTemplate {
	return files("pom.xml", "<project xmlns=\"http://maven.apache.org/POM/4.0.0\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xsi:schemaLocation=\"http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd\">\n  <modelVersion>4.0.0</modelVersion>\n  <groupId>com.example</groupId>\n  <artifactId>"+ctx.Slug+"</artifactId>\n  <version>0.1.0</version>\n  <properties><maven.compiler.release>21</maven.compiler.release></properties>\n</project>\n", "src/main/java/"+ctx.PackagePath+"/App.java", "package "+ctx.PackageName+";\n\npublic class App {\n  public static void main(String[] args) {\n    System.out.println(\"Hello from "+ctx.Slug+"\");\n  }\n}\n")
}

func javaGradleFiles(ctx context) []fileTemplate {
	return files("settings.gradle", "rootProject.name = '"+ctx.Slug+"'\n", "build.gradle", "plugins { id 'application' }\n\nrepositories { mavenCentral() }\n\napplication { mainClass = '"+ctx.PackageName+".App' }\n", "src/main/java/"+ctx.PackagePath+"/App.java", "package "+ctx.PackageName+";\n\npublic class App {\n  public static void main(String[] args) {\n    System.out.println(\"Hello from "+ctx.Slug+"\");\n  }\n}\n")
}

func kotlinGradleFiles(ctx context) []fileTemplate {
	return files("settings.gradle.kts", "rootProject.name = \""+ctx.Slug+"\"\n", "build.gradle.kts", "plugins { kotlin(\"jvm\") version \"2.0.0\" application }\n\nrepositories { mavenCentral() }\n\napplication { mainClass.set(\""+ctx.PackageName+".AppKt\") }\n", "src/main/kotlin/"+ctx.PackagePath+"/App.kt", "package "+ctx.PackageName+"\n\nfun main() {\n    println(\"Hello from "+ctx.Slug+"\")\n}\n")
}

func springBootFiles(ctx context) []fileTemplate {
	return files("pom.xml", "<project xmlns=\"http://maven.apache.org/POM/4.0.0\">\n  <modelVersion>4.0.0</modelVersion>\n  <groupId>com.example</groupId>\n  <artifactId>"+ctx.Slug+"</artifactId>\n  <version>0.1.0</version>\n  <parent><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-parent</artifactId><version>3.3.0</version></parent>\n  <dependencies><dependency><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-web</artifactId></dependency></dependencies>\n</project>\n", "src/main/java/"+ctx.PackagePath+"/Application.java", "package "+ctx.PackageName+";\n\nimport org.springframework.boot.SpringApplication;\nimport org.springframework.boot.autoconfigure.SpringBootApplication;\n\n@SpringBootApplication\npublic class Application {\n  public static void main(String[] args) {\n    SpringApplication.run(Application.class, args);\n  }\n}\n")
}

func dotnetConsoleFiles(ctx context) []fileTemplate {
	return files(ctx.Pascal+".csproj", "<Project Sdk=\"Microsoft.NET.Sdk\">\n  <PropertyGroup><OutputType>Exe</OutputType><TargetFramework>net8.0</TargetFramework><Nullable>enable</Nullable></PropertyGroup>\n</Project>\n", "Program.cs", "Console.WriteLine(\"Hello from "+ctx.Slug+"\");\n")
}

func dotnetWebAPIFiles(ctx context) []fileTemplate {
	return files(ctx.Pascal+".csproj", "<Project Sdk=\"Microsoft.NET.Sdk.Web\">\n  <PropertyGroup><TargetFramework>net8.0</TargetFramework><Nullable>enable</Nullable></PropertyGroup>\n</Project>\n", "Program.cs", "var builder = WebApplication.CreateBuilder(args);\nvar app = builder.Build();\napp.MapGet(\"/health\", () => Results.Ok(new { status = \"ok\" }));\napp.Run();\n")
}

func dartConsoleFiles(ctx context) []fileTemplate {
	return files("pubspec.yaml", "name: "+ctx.Snake+"\nversion: 0.1.0\nenvironment:\n  sdk: ^3.4.0\n", "bin/main.dart", "void main() {\n  print('Hello from "+ctx.Slug+"');\n}\n")
}

func swiftFiles(ctx context) []fileTemplate {
	return files("Package.swift", "// swift-tools-version: 5.10\nimport PackageDescription\n\nlet package = Package(name: \""+ctx.Pascal+"\", targets: [.executableTarget(name: \""+ctx.Pascal+"\")])\n", "Sources/"+ctx.Pascal+"/main.swift", "print(\"Hello from "+ctx.Slug+"\")\n")
}

func elixirFiles(ctx context) []fileTemplate {
	return files("mix.exs", "defmodule "+ctx.Pascal+".MixProject do\n  use Mix.Project\n  def project, do: [app: :"+ctx.Snake+", version: \"0.1.0\", elixir: \"~> 1.16\"]\n  def application, do: [extra_applications: [:logger]]\nend\n", "lib/"+ctx.Snake+".ex", "defmodule "+ctx.Pascal+" do\n  def hello, do: :world\nend\n")
}

func rubyFiles(ctx context) []fileTemplate {
	return files("Gemfile", "source 'https://rubygems.org'\n\ngem 'rack'\n", "app.rb", "puts 'Hello from "+ctx.Slug+"'\n")
}

func cFiles(ctx context) []fileTemplate {
	return files("Makefile", "CC ?= cc\nCFLAGS ?= -Wall -Wextra -std=c11\n\nrun: "+ctx.Slug+"\n\t./"+ctx.Slug+"\n\n"+ctx.Slug+": src/main.c\n\t$(CC) $(CFLAGS) -o $@ $<\n", "src/main.c", "#include <stdio.h>\n\nint main(void) {\n    puts(\"Hello from "+ctx.Slug+"\");\n    return 0;\n}\n")
}

func cppFiles(ctx context) []fileTemplate {
	return files("CMakeLists.txt", "cmake_minimum_required(VERSION 3.20)\nproject("+ctx.Pascal+" LANGUAGES CXX)\nset(CMAKE_CXX_STANDARD 20)\nadd_executable("+ctx.Slug+" src/main.cpp)\n", "src/main.cpp", "#include <iostream>\n\nint main() {\n    std::cout << \"Hello from "+ctx.Slug+"\\n\";\n}\n")
}

func luaFiles(ctx context) []fileTemplate {
	return files("init.lua", "print('Hello from "+ctx.Slug+"')\n", ctx.Slug+"-0.1.0-1.rockspec", "package = '"+ctx.Slug+"'\nversion = '0.1.0-1'\nsource = { url = 'git://example.invalid/"+ctx.Slug+"' }\ndescription = { summary = 'Lua starter' }\n")
}

func rFiles(ctx context) []fileTemplate {
	return files("DESCRIPTION", "Package: "+ctx.Pascal+"\nVersion: 0.1.0\nTitle: Envdoctor Starter\nDescription: R starter generated by envdoctor.\nLicense: MIT\nEncoding: UTF-8\n", "R/main.R", "hello <- function() {\n  print('Hello from "+ctx.Slug+"')\n}\n")
}

func juliaFiles(ctx context) []fileTemplate {
	return files("Project.toml", "name = \""+ctx.Pascal+"\"\nversion = \"0.1.0\"\n", "src/main.jl", "println(\"Hello from "+ctx.Slug+"\")\n")
}

func haskellFiles(ctx context) []fileTemplate {
	return files(ctx.Slug+".cabal", "cabal-version: 3.0\nname: "+ctx.Slug+"\nversion: 0.1.0.0\nexecutable "+ctx.Slug+"\n  main-is: Main.hs\n  hs-source-dirs: app\n  build-depends: base >=4.14\n  default-language: Haskell2010\n", "app/Main.hs", "main :: IO ()\nmain = putStrLn \"Hello from "+ctx.Slug+"\"\n")
}

func perlFiles(ctx context) []fileTemplate {
	return files("cpanfile", "requires 'perl', '5.030';\n", "script/app.pl", "#!/usr/bin/env perl\nuse strict;\nuse warnings;\n\nprint \"Hello from "+ctx.Slug+"\\n\";\n")
}

func files(values ...string) []fileTemplate {
	result := make([]fileTemplate, 0, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		result = append(result, fileTemplate{Path: values[i], Content: values[i+1]})
	}
	return result
}

func cleanRelativePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", fmt.Errorf("invalid scaffold file path: %s", path)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("path traversal is blocked for scaffold file: %s", path)
	}
	return clean, nil
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

func normalizeTemplate(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
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

func pascalCase(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '-' || r == '_' || r == '.' || r == ' ' })
	var builder strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			builder.WriteString(part[1:])
		}
	}
	if builder.Len() == 0 {
		return "Project"
	}
	return builder.String()
}

func sanitizePackage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var parts []string
	for _, part := range strings.Split(value, ".") {
		clean := strings.ReplaceAll(slug(part), "-", "_")
		if clean != "" {
			parts = append(parts, clean)
		}
	}
	if len(parts) == 0 {
		return "com.example.project"
	}
	return strings.Join(parts, ".")
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func manualSteps(def templateDef) string {
	return valueOrDefault(def.ManualSteps, "Review the framework documentation and create this project manually.")
}
