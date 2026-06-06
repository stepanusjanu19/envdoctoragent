package rag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/stepanusjanu19/envdoctoragent/internal/automation"
	"github.com/stepanusjanu19/envdoctoragent/internal/bootstrap"
	"github.com/stepanusjanu19/envdoctoragent/internal/container"
	"github.com/stepanusjanu19/envdoctoragent/internal/dependencies"
	"github.com/stepanusjanu19/envdoctoragent/internal/diagnose"
	"github.com/stepanusjanu19/envdoctoragent/internal/fixplan"
	"github.com/stepanusjanu19/envdoctoragent/internal/projectops"
	"github.com/stepanusjanu19/envdoctoragent/internal/recommendation"
	"github.com/stepanusjanu19/envdoctoragent/internal/scanner"
	"github.com/stepanusjanu19/envdoctoragent/internal/system"
	versionpkg "github.com/stepanusjanu19/envdoctoragent/internal/version"
)

const (
	indexVersion     = "envdoctor-rag-v1"
	defaultIndexFile = ".envdoctor/rag/index.json"
	maxFileBytes     = 256 * 1024
	maxDocumentBytes = 64 * 1024
	snippetRadius    = 120
)

// DefaultTopK is the default number of ranked matches returned by RAG queries.
const DefaultTopK = 5

// Options configures local RAG collection and querying.
type Options struct {
	Directory    string
	Output       string
	Index        string
	Query        string
	Goal         string
	TopK         int
	IncludeAudit bool
	Logs         []string
}

// Safety documents the hard boundaries of the local RAG layer.
type Safety struct {
	ReadOnly        bool     `json:"read_only"`
	ExecutorAccess  bool     `json:"executor_access"`
	MutatingActions bool     `json:"mutating_actions"`
	Notes           []string `json:"notes"`
}

// Source describes an indexed context source without embedding the full content.
type Source struct {
	ID         string            `json:"id"`
	SourceType string            `json:"source_type"`
	Source     string            `json:"source"`
	Title      string            `json:"title"`
	Bytes      int               `json:"bytes,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Document is a local indexed document. It is stored in the index file only.
type Document struct {
	ID         string            `json:"id"`
	SourceType string            `json:"source_type"`
	Source     string            `json:"source"`
	Title      string            `json:"title"`
	Content    string            `json:"content"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Index is the persisted local lexical index.
type Index struct {
	Version     string     `json:"version"`
	Status      string     `json:"status"`
	Directory   string     `json:"directory"`
	GeneratedAt string     `json:"generated_at"`
	Sources     []Source   `json:"sources"`
	Documents   []Document `json:"documents"`
	Limitations []string   `json:"limitations,omitempty"`
	Safety      Safety     `json:"safety"`
	Summary     string     `json:"summary"`
}

// IndexReport is returned by envdoctor rag index.
type IndexReport struct {
	Status      string   `json:"status"`
	Directory   string   `json:"directory"`
	Output      string   `json:"output"`
	GeneratedAt string   `json:"generated_at"`
	Sources     []Source `json:"sources"`
	Limitations []string `json:"limitations,omitempty"`
	Safety      Safety   `json:"safety"`
	Summary     string   `json:"summary"`
}

// Match is one ranked retrieval result.
type Match struct {
	SourceType string            `json:"source_type"`
	Source     string            `json:"source"`
	Title      string            `json:"title"`
	Score      float64           `json:"score"`
	Snippet    string            `json:"snippet"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// QueryReport is returned by envdoctor rag query/context.
type QueryReport struct {
	Status      string   `json:"status"`
	Directory   string   `json:"directory"`
	Query       string   `json:"query"`
	Goal        string   `json:"goal,omitempty"`
	GeneratedAt string   `json:"generated_at"`
	Sources     []Source `json:"sources"`
	Matches     []Match  `json:"matches"`
	Answer      string   `json:"answer"`
	Limitations []string `json:"limitations,omitempty"`
	NextSteps   []string `json:"next_steps"`
	Safety      Safety   `json:"safety"`
}

var secretValuePattern = regexp.MustCompile(`(?i)(password|passwd|token|secret|api[_-]?key|authorization|private[_-]?key)\s*[:=]\s*"?[^"\s,}]+`)

// CreateIndex builds and writes the local RAG index.
func CreateIndex(options Options) (*IndexReport, error) {
	index, err := Build(options)
	if err != nil {
		return nil, err
	}
	output, err := resolveOutputPath(index.Directory, options.Output)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(output, data, 0o644); err != nil {
		return nil, err
	}
	return &IndexReport{
		Status:      "indexed",
		Directory:   index.Directory,
		Output:      output,
		GeneratedAt: index.GeneratedAt,
		Sources:     index.Sources,
		Limitations: index.Limitations,
		Safety:      index.Safety,
		Summary:     fmt.Sprintf("Indexed %d local read-only source(s). RAG did not execute actions.", len(index.Sources)),
	}, nil
}

// Build collects deterministic Envdoctor context into an in-memory lexical index.
func Build(options Options) (*Index, error) {
	dir := strings.TrimSpace(options.Directory)
	if dir == "" {
		dir = "."
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(absDir); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, fmt.Errorf("rag directory is not a directory: %s", absDir)
	}

	documents, limitations := collectDocuments(absDir, options)
	sources := make([]Source, 0, len(documents))
	for _, doc := range documents {
		sources = append(sources, Source{
			ID:         doc.ID,
			SourceType: doc.SourceType,
			Source:     doc.Source,
			Title:      doc.Title,
			Bytes:      len([]byte(doc.Content)),
			Metadata:   doc.Metadata,
		})
	}

	index := &Index{
		Version:     indexVersion,
		Status:      "ready",
		Directory:   absDir,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Sources:     sources,
		Documents:   documents,
		Limitations: limitations,
		Safety:      defaultSafety(),
		Summary:     fmt.Sprintf("Built local lexical RAG index with %d source(s).", len(sources)),
	}
	if len(documents) == 0 {
		index.Status = "empty"
		index.Summary = "No local read-only RAG sources were collected."
	}
	return index, nil
}

// Query retrieves ranked local context for a question.
func Query(options Options) (*QueryReport, error) {
	query := strings.TrimSpace(options.Query)
	if query == "" {
		return nil, fmt.Errorf("rag query cannot be empty")
	}
	index, err := loadOrBuild(options)
	if err != nil {
		return nil, err
	}
	topK := options.TopK
	if topK <= 0 {
		topK = DefaultTopK
	}
	matches := rank(index.Documents, query, topK)
	report := &QueryReport{
		Status:      "ready",
		Directory:   index.Directory,
		Query:       query,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Sources:     index.Sources,
		Matches:     matches,
		Answer:      answer(query, matches),
		Limitations: index.Limitations,
		NextSteps:   nextSteps(),
		Safety:      defaultSafety(),
	}
	if len(matches) == 0 {
		report.Status = "no-match"
	}
	return report, nil
}

// Context creates a goal-specific retrieval context pack.
func Context(options Options) (*QueryReport, error) {
	goal := strings.ToLower(strings.TrimSpace(options.Goal))
	if goal == "" {
		goal = "diagnose"
	}
	query, err := queryForGoal(goal)
	if err != nil {
		return nil, err
	}
	options.Query = query
	report, err := Query(options)
	if err != nil {
		return nil, err
	}
	report.Goal = goal
	report.Answer = fmt.Sprintf("Prepared local read-only RAG context for %s. Review ranked matches before changing the environment.", goal)
	return report, nil
}

func collectDocuments(dir string, options Options) ([]Document, []string) {
	var documents []Document
	var limitations []string
	seen := map[string]bool{}

	add := func(sourceType, source, title string, value interface{}, metadata map[string]string) {
		content, err := encodeValue(value)
		if err != nil {
			limitations = append(limitations, fmt.Sprintf("could not encode %s: %v", title, err))
			return
		}
		addTextDocument(&documents, seen, sourceType, source, title, content, metadata)
	}
	addError := func(sourceType, title string, err error) {
		if err == nil {
			return
		}
		addTextDocument(&documents, seen, sourceType, sourceType, title, err.Error(), map[string]string{"status": "collector-error"})
		limitations = append(limitations, fmt.Sprintf("%s: %v", title, err))
	}

	sysInfo, err := system.Detect()
	if err != nil {
		addError("system", "System discovery issue", err)
	} else {
		add("system", "envdoctor system", "System discovery", sysInfo, map[string]string{"phase": "1", "command": "envdoctor system --json"})
	}

	tools, err := scanner.ScanToolchain()
	if err != nil {
		addError("toolchain", "Toolchain scan issue", err)
	} else {
		add("toolchain", "envdoctor scan toolchain", "Toolchain scan", tools, map[string]string{"phase": "1", "command": "envdoctor scan toolchain --json"})
	}

	pathReport, err := scanner.ScanPath()
	if err != nil {
		addError("path", "PATH scan issue", err)
	} else {
		add("path", "envdoctor scan path", "PATH scan", pathReport, map[string]string{"phase": "1", "command": "envdoctor scan path --json"})
	}

	containerInfo, err := container.CheckContainerEnvironments()
	if err != nil {
		addError("container", "Container scan issue", err)
	} else {
		add("container", "envdoctor scan container", "Container scan", containerInfo, map[string]string{"phase": "2", "command": "envdoctor scan container --json"})
	}

	dependencyReports, err := dependencies.AnalyzeAllDependencies(dir)
	if err != nil {
		addError("dependencies", "Dependency scan issue", err)
	} else {
		add("dependencies", "envdoctor scan dependencies", "Dependency scan", dependencyReports, map[string]string{"phase": "4H", "command": "envdoctor scan dependencies --json"})
		for _, report := range dependencyReports {
			if report != nil && report.SourceFile != "" {
				addFileDocument(&documents, &limitations, seen, dir, report.SourceFile, "manifest", "Project manifest")
			}
		}
	}

	diagnostics, err := diagnose.Run()
	if err != nil {
		addError("diagnose", "Diagnosis issue", err)
	} else {
		add("diagnose", "envdoctor diagnose", "Diagnosis report", diagnostics, map[string]string{"phase": "1", "command": "envdoctor diagnose --json"})
	}

	recommendations := recommendation.GenerateRecommendationsFromScans(sysInfo, tools, pathReport, containerInfo)
	add("recommendation", "envdoctor recommend", "Recommendation report", recommendations, map[string]string{"phase": "3", "command": "envdoctor recommend --json"})

	versionPlan, err := versionpkg.Plan(dir)
	if err != nil {
		addError("version", "Version plan issue", err)
	} else {
		add("version", "envdoctor version plan", "Version plan", versionPlan, map[string]string{"phase": "4C", "command": "envdoctor version plan --json"})
	}

	fixReport, err := fixplan.Generate(dir)
	if err != nil {
		addError("fix", "Fix plan issue", err)
	} else {
		add("fix", "envdoctor fix plan", "Fix plan", fixReport, map[string]string{"phase": "4E", "command": "envdoctor fix plan --json"})
	}

	bootstrapPlan, err := bootstrap.GeneratePlan(dir)
	if err != nil {
		addError("bootstrap", "Bootstrap plan issue", err)
	} else {
		add("bootstrap", "envdoctor bootstrap plan", "Bootstrap plan", bootstrapPlan, map[string]string{"phase": "4E", "command": "envdoctor bootstrap plan --json"})
	}

	projectScan, err := projectops.Scan(dir)
	if err != nil {
		addError("project", "Project scan issue", err)
	} else {
		add("project", "envdoctor project scan", "Project scan", projectScan, map[string]string{"phase": "5D", "command": "envdoctor project scan --json"})
	}

	automationPlan, err := automation.Plan(automation.Options{
		Directory: dir,
		Goal:      automation.GoalMaintain,
		Profile:   "development",
		MaxRisk:   "high",
	})
	if err != nil {
		addError("automation", "Automation plan issue", err)
	} else {
		add("automation", "envdoctor automation plan", "Automation maintain plan", automationPlan, map[string]string{"phase": "6B", "command": "envdoctor automation plan --json --goal maintain"})
	}

	addFileDocument(&documents, &limitations, seen, dir, filepath.Join(dir, "README.md"), "doc", "README")
	addFileDocument(&documents, &limitations, seen, dir, filepath.Join(dir, "PLAN.md"), "doc", "Canonical roadmap")
	addFileDocument(&documents, &limitations, seen, dir, filepath.Join(dir, "go.mod"), "manifest", "Go module manifest")
	addFileDocument(&documents, &limitations, seen, dir, filepath.Join(dir, ".goreleaser.yaml"), "release", "GoReleaser metadata")
	addFileDocument(&documents, &limitations, seen, dir, filepath.Join(dir, ".github", "workflows", "ci.yml"), "release", "GitHub Actions workflow")
	for _, path := range commonManifestFiles(dir) {
		addFileDocument(&documents, &limitations, seen, dir, path, "manifest", "Project manifest")
	}
	for _, logPath := range options.Logs {
		addFileDocument(&documents, &limitations, seen, dir, logPath, "log", "User-provided log")
	}
	if options.IncludeAudit {
		collectAuditDocuments(&documents, &limitations, seen, dir)
	}

	sort.SliceStable(documents, func(i, j int) bool {
		if documents[i].SourceType == documents[j].SourceType {
			return documents[i].Source < documents[j].Source
		}
		return documents[i].SourceType < documents[j].SourceType
	})
	for i := range documents {
		documents[i].ID = fmt.Sprintf("%s-%03d", normalizeID(documents[i].SourceType), i+1)
	}
	return documents, uniqueStrings(limitations)
}

func addTextDocument(documents *[]Document, seen map[string]bool, sourceType, source, title, content string, metadata map[string]string) {
	content = trimDocument(redact(content))
	key := sourceType + "\x00" + source + "\x00" + title
	if content == "" || seen[key] {
		return
	}
	seen[key] = true
	*documents = append(*documents, Document{
		SourceType: sourceType,
		Source:     source,
		Title:      title,
		Content:    content,
		Metadata:   metadata,
	})
}

func addFileDocument(documents *[]Document, limitations *[]string, seen map[string]bool, root, path, sourceType, title string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	content, err := readSafeText(path)
	if err != nil {
		if !os.IsNotExist(err) {
			*limitations = append(*limitations, fmt.Sprintf("skipped %s: %v", path, err))
		}
		return
	}
	display := path
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		display = rel
	}
	addTextDocument(documents, seen, sourceType, display, title+": "+filepath.Base(path), content, map[string]string{"file": display})
}

func collectAuditDocuments(documents *[]Document, limitations *[]string, seen map[string]bool, root string) {
	auditDir := filepath.Join(root, ".envdoctor", "audit")
	entries, err := os.ReadDir(auditDir)
	if err != nil {
		if !os.IsNotExist(err) {
			*limitations = append(*limitations, fmt.Sprintf("audit directory skipped: %v", err))
		}
		return
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jsonl") {
			continue
		}
		count++
		if count > 10 {
			*limitations = append(*limitations, "audit collection limited to 10 JSONL files")
			break
		}
		path := filepath.Join(auditDir, entry.Name())
		content, err := readSafeText(path)
		if err != nil {
			*limitations = append(*limitations, fmt.Sprintf("skipped audit %s: %v", entry.Name(), err))
			continue
		}
		content = stripPreviewFields(content)
		addTextDocument(documents, seen, "audit", filepath.Join(".envdoctor", "audit", entry.Name()), "Audit summary", content, map[string]string{"file": entry.Name()})
	}
}

func readSafeText(path string) (string, error) {
	if shouldSkipPath(path) {
		return "", fmt.Errorf("secret-prone or ignored path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("is a directory")
	}
	if info.Size() > maxFileBytes {
		return "", fmt.Errorf("file exceeds %d bytes", maxFileBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", fmt.Errorf("binary or non-UTF-8 content")
	}
	return string(data), nil
}

func loadOrBuild(options Options) (*Index, error) {
	if strings.TrimSpace(options.Index) == "" {
		return Build(options)
	}
	data, err := os.ReadFile(options.Index)
	if err != nil {
		return nil, err
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	if index.Version != indexVersion {
		return nil, fmt.Errorf("unsupported rag index version %q", index.Version)
	}
	if index.Safety.Notes == nil {
		index.Safety = defaultSafety()
	}
	return &index, nil
}

func rank(documents []Document, query string, topK int) []Match {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}
	df := documentFrequency(documents, queryTokens)
	totalDocs := float64(len(documents))
	lowerQuery := strings.ToLower(query)
	var matches []Match
	for _, doc := range documents {
		content := strings.ToLower(doc.Title + "\n" + doc.Content)
		frequencies := tokenFrequency(content)
		var score float64
		for _, token := range queryTokens {
			count := frequencies[token]
			if count == 0 {
				continue
			}
			idf := math.Log(1 + totalDocs/(1+float64(df[token])))
			score += float64(count) * (1 + idf)
			if strings.Contains(strings.ToLower(doc.Title), token) {
				score += 3
			}
		}
		if strings.Contains(content, lowerQuery) {
			score += 8
		}
		score *= sourceBoost(doc.SourceType)
		if score <= 0 {
			continue
		}
		matches = append(matches, Match{
			SourceType: doc.SourceType,
			Source:     doc.Source,
			Title:      doc.Title,
			Score:      math.Round(score*100) / 100,
			Snippet:    snippet(doc.Content, queryTokens, lowerQuery),
			Metadata:   doc.Metadata,
		})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			if matches[i].SourceType == matches[j].SourceType {
				return matches[i].Source < matches[j].Source
			}
			return matches[i].SourceType < matches[j].SourceType
		}
		return matches[i].Score > matches[j].Score
	})
	if len(matches) > topK {
		matches = matches[:topK]
	}
	return matches
}

func queryForGoal(goal string) (string, error) {
	switch goal {
	case "diagnose":
		return "diagnose system toolchain path container recommendation health issue", nil
	case "onboard":
		return "onboard bootstrap dependencies runtime version project setup install plan", nil
	case "repair":
		return "repair fix plan dependency issue version mismatch path container recommendation", nil
	case "maintain":
		return "maintain diagnose repair automation plan health drift dependency version", nil
	default:
		return "", fmt.Errorf("unsupported rag context goal %q", goal)
	}
}

func answer(query string, matches []Match) string {
	if len(matches) == 0 {
		return fmt.Sprintf("No strong local RAG match was found for %q. Run envdoctor diagnose --json and add relevant logs with --log for more context.", query)
	}
	top := matches[0]
	return fmt.Sprintf("Local RAG found the strongest evidence in %s (%s). This is retrieval-only context; use plan/apply --dry-run workflows before any environment change.", top.Title, top.SourceType)
}

func nextSteps() []string {
	return []string{
		"Review ranked matches and snippets before acting.",
		"Run envdoctor diagnose --json or envdoctor automation plan --json for fresh deterministic context.",
		"Use existing plan/apply --dry-run commands for any suggested mutation; RAG does not execute actions.",
	}
}

func defaultSafety() Safety {
	return Safety{
		ReadOnly:        true,
		ExecutorAccess:  false,
		MutatingActions: false,
		Notes: []string{
			"RAG is local lexical retrieval only.",
			"RAG does not import or call the executor.",
			"RAG suggestions must route through existing plan/apply dry-run flows before approval.",
		},
	}
}

func encodeValue(value interface{}) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func resolveOutputPath(dir, output string) (string, error) {
	if strings.TrimSpace(output) == "" {
		output = filepath.Join(dir, defaultIndexFile)
	}
	if !filepath.IsAbs(output) {
		output = filepath.Join(dir, output)
	}
	return filepath.Abs(output)
}

func shouldSkipPath(path string) bool {
	parts := strings.Split(filepath.Clean(path), string(os.PathSeparator))
	for _, part := range parts {
		switch strings.ToLower(part) {
		case ".git", ".cache", "node_modules", "vendor", "target", "dist", "build", ".venv", "venv":
			return true
		}
	}
	base := strings.ToLower(filepath.Base(path))
	if base == ".env" || base == ".env.local" || base == ".env.production" {
		return true
	}
	if strings.HasPrefix(base, "id_rsa") || strings.HasPrefix(base, "id_dsa") || strings.HasPrefix(base, "id_ed25519") || strings.HasPrefix(base, "id_ecdsa") {
		return true
	}
	if strings.Contains(base, "private") && strings.Contains(base, "key") {
		return true
	}
	switch filepath.Ext(base) {
	case ".pem", ".key", ".p12", ".pfx", ".crt", ".cer", ".der", ".exe", ".dll", ".so", ".dylib", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".zip", ".gz", ".zst":
		return true
	}
	return false
}

func commonManifestFiles(dir string) []string {
	names := []string{
		"package.json", "go.mod", "Cargo.toml", "composer.json", "requirements.txt", "pyproject.toml",
		"pom.xml", "build.gradle", "build.gradle.kts", "Gemfile", "pubspec.yaml", "Package.swift",
		"mix.exs", "Project.toml", "stack.yaml", "cpanfile", "vcpkg.json", "conanfile.txt", "conanfile.py",
	}
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths
}

func redact(content string) string {
	return secretValuePattern.ReplaceAllString(content, "$1: [redacted]")
}

func stripPreviewFields(content string) string {
	var sanitized []string
	for _, line := range strings.Split(content, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "stdout_preview") || strings.Contains(lower, "stderr_preview") || strings.Contains(lower, "content") {
			continue
		}
		sanitized = append(sanitized, line)
	}
	return strings.Join(sanitized, "\n")
}

func trimDocument(content string) string {
	content = strings.TrimSpace(content)
	if len(content) <= maxDocumentBytes {
		return content
	}
	return content[:maxDocumentBytes] + "\n...[truncated]"
}

func tokenize(value string) []string {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	var tokens []string
	seen := map[string]bool{}
	for _, field := range fields {
		if len(field) < 2 || stopWords[field] {
			continue
		}
		if !seen[field] {
			tokens = append(tokens, field)
			seen[field] = true
		}
	}
	return tokens
}

func tokenFrequency(value string) map[string]int {
	freq := map[string]int{}
	for _, token := range strings.FieldsFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	}) {
		token = strings.ToLower(token)
		if len(token) < 2 || stopWords[token] {
			continue
		}
		freq[token]++
	}
	return freq
}

func documentFrequency(documents []Document, queryTokens []string) map[string]int {
	result := map[string]int{}
	for _, doc := range documents {
		freq := tokenFrequency(doc.Title + "\n" + doc.Content)
		for _, token := range queryTokens {
			if freq[token] > 0 {
				result[token]++
			}
		}
	}
	return result
}

func snippet(content string, queryTokens []string, lowerQuery string) string {
	lower := strings.ToLower(content)
	pos := strings.Index(lower, lowerQuery)
	if pos < 0 {
		for _, token := range queryTokens {
			pos = strings.Index(lower, token)
			if pos >= 0 {
				break
			}
		}
	}
	if pos < 0 {
		return compact(content, snippetRadius*2)
	}
	start := pos - snippetRadius
	if start < 0 {
		start = 0
	}
	end := pos + snippetRadius
	if end > len(content) {
		end = len(content)
	}
	return compact(content[start:end], snippetRadius*2)
}

func compact(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func sourceBoost(sourceType string) float64 {
	switch sourceType {
	case "fix", "bootstrap", "automation", "recommendation":
		return 1.35
	case "diagnose", "dependencies", "version", "project":
		return 1.25
	case "log":
		return 1.20
	case "doc", "manifest":
		return 1.10
	default:
		return 1.0
	}
}

func normalizeID(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "source"
	}
	return result
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
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

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true, "this": true,
	"from": true, "into": true, "your": true, "you": true, "are": true, "was": true,
	"were": true, "why": true, "what": true, "when": true, "where": true, "how": true,
	"has": true, "have": true, "not": true, "but": true, "all": true, "any": true,
	"envdoctor": true,
}
