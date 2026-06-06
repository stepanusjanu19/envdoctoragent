package terminalui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const width = 72

// Options controls human-readable terminal rendering.
type Options struct {
	Plain   bool
	Version string
	Profile string
}

// Presenter renders lightweight ANSI output for non-JSON CLI views.
type Presenter struct {
	out   io.Writer
	opts  Options
	color bool
}

type palette struct {
	reset  string
	bold   string
	dim    string
	cyan   string
	green  string
	yellow string
	red    string
}

// New creates a terminal presenter. It never writes outside the provided writer.
func New(out io.Writer, options Options) *Presenter {
	if out == nil {
		out = io.Discard
	}
	return &Presenter{
		out:   out,
		opts:  options,
		color: !options.Plain && os.Getenv("NO_COLOR") == "" && isTerminal(out),
	}
}

// About renders a compact product/safety summary.
func (p *Presenter) About() {
	p.Header("Envdoctor", "Environment Doctor Agent")
	p.Section("About")
	p.Row("Version", valueOrDefault(p.opts.Version, "dev"))
	p.Row("Automation", "read-only, plan-only, and approval-gated apply")
	p.Row("Safety", "dry-run default, policy checks, audit log, snapshot before mutation")
	p.Row("Project starters", "official generator actions only")
	p.Row("Future", "AI troubleshooting and autonomous production mutation are not active")
	p.NextSteps(
		"Run envdoctor diagnose for a workstation summary.",
		"Run envdoctor ui for the guided terminal dashboard.",
		"Use --json when another tool needs a stable contract.",
	)
}

// Header renders a command title.
func (p *Presenter) Header(title, subtitle string) {
	c := p.colors()
	fmt.Fprintf(p.out, "%s%s%s\n", c.cyan, strings.Repeat("=", width), c.reset)
	fmt.Fprintf(p.out, "%s%s%s", c.bold, title, c.reset)
	if subtitle != "" {
		fmt.Fprintf(p.out, "  %s%s%s", c.dim, subtitle, c.reset)
	}
	fmt.Fprintln(p.out)
	if p.opts.Version != "" {
		fmt.Fprintf(p.out, "%sVersion: %s%s\n", c.dim, p.opts.Version, c.reset)
	}
	fmt.Fprintf(p.out, "%s%s%s\n", c.cyan, strings.Repeat("=", width), c.reset)
}

// Section renders a section heading.
func (p *Presenter) Section(title string) {
	c := p.colors()
	fmt.Fprintf(p.out, "\n%s%s%s\n", c.bold, title, c.reset)
	fmt.Fprintf(p.out, "%s%s%s\n", c.dim, strings.Repeat("-", min(width, len(title)+18)), c.reset)
}

// Row renders aligned key/value detail.
func (p *Presenter) Row(label, value string) {
	fmt.Fprintf(p.out, "%-22s %s\n", label, valueOrDefault(value, "-"))
}

// StatusRow renders a status with lightweight coloring.
func (p *Presenter) StatusRow(label, status, detail string) {
	c := p.colors()
	fmt.Fprintf(p.out, "%-22s %s%-18s%s %s%s%s\n", label, p.statusColor(status), valueOrDefault(status, "-"), c.reset, c.dim, detail, c.reset)
}

// Bullet renders a bullet item.
func (p *Presenter) Bullet(status, title string) {
	fmt.Fprintf(p.out, "- [%s] %s\n", valueOrDefault(status, "-"), title)
}

// Detail renders an indented detail line.
func (p *Presenter) Detail(label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(p.out, "  %s: %s\n", label, value)
}

// Progress renders deterministic stage progress. It is hidden for --plain.
func (p *Presenter) Progress(step, total int, label string) {
	if p.opts.Plain {
		return
	}
	if total <= 0 {
		total = 1
	}
	if step < 0 {
		step = 0
	}
	if step > total {
		step = total
	}
	percent := step * 100 / total
	filled := percent / 10
	if filled > 10 {
		filled = 10
	}
	bar := strings.Repeat("#", filled) + strings.Repeat("-", 10-filled)
	fmt.Fprintf(p.out, "Progress: [%s] %3d%%  %s\n", bar, percent, label)
}

// NextSteps renders high-signal follow-up hints.
func (p *Presenter) NextSteps(steps ...string) {
	filtered := make([]string, 0, len(steps))
	for _, step := range steps {
		step = strings.TrimSpace(step)
		if step != "" {
			filtered = append(filtered, step)
		}
	}
	if len(filtered) == 0 {
		return
	}
	p.Section("Next steps")
	for _, step := range filtered {
		fmt.Fprintf(p.out, "- %s\n", step)
	}
}

func (p *Presenter) colors() palette {
	if !p.color {
		return palette{}
	}
	return palette{
		reset:  "\033[0m",
		bold:   "\033[1m",
		dim:    "\033[2m",
		cyan:   "\033[36m",
		green:  "\033[32m",
		yellow: "\033[33m",
		red:    "\033[31m",
	}
}

func (p *Presenter) statusColor(status string) string {
	if !p.color {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "implemented", "available", "read-only", "completed":
		return p.colors().green
	case "plan-only", "safe apply preview", "safe-execution-preview", "metadata-ready", "metadata-only", "dry-run":
		return p.colors().yellow
	case "blocked", "error", "not found", "not supported", "permission denied", "timeout":
		return p.colors().red
	default:
		return p.colors().cyan
	}
}

func isTerminal(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func valueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
