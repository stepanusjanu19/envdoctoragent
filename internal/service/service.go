package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/stepanusjanu19/envdoctoragent/internal/command"
)

// ServiceInfo describes a service discovered by the platform service manager.
type ServiceInfo struct {
	Name           string `json:"name"`
	Manager        string `json:"manager"`
	Platform       string `json:"platform"`
	Status         string `json:"status"`
	State          string `json:"state,omitempty"`
	Description    string `json:"description,omitempty"`
	Raw            string `json:"raw,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
}

// ListReport summarizes read-only service discovery.
type ListReport struct {
	Platform string        `json:"platform"`
	Manager  string        `json:"manager"`
	Status   string        `json:"status"`
	Message  string        `json:"message,omitempty"`
	Services []ServiceInfo `json:"services"`
}

// DiagnoseReport contains a read-only service diagnosis.
type DiagnoseReport struct {
	Service         ServiceInfo `json:"service"`
	Recommendations []string    `json:"recommendations"`
}

// Action describes one service operation candidate. Execution is delegated to
// the shared executor package by the CLI layer.
type Action struct {
	ID            string   `json:"id"`
	Source        string   `json:"source"`
	Category      string   `json:"category"`
	Operation     string   `json:"operation"`
	Title         string   `json:"title"`
	Description   string   `json:"description,omitempty"`
	Command       string   `json:"command,omitempty"`
	Args          []string `json:"args,omitempty"`
	ManualSteps   string   `json:"manual_steps,omitempty"`
	Risk          string   `json:"risk"`
	RequiresAdmin bool     `json:"requires_admin"`
	SafeToRun     bool     `json:"safe_to_run"`
	Timeout       string   `json:"timeout,omitempty"`
	RollbackHint  string   `json:"rollback_hint,omitempty"`
	Status        string   `json:"status"`
}

// PlanReport contains an approval-gated service operation plan.
type PlanReport struct {
	Service   string   `json:"service"`
	Operation string   `json:"operation"`
	Platform  string   `json:"platform"`
	Manager   string   `json:"manager"`
	Status    string   `json:"status"`
	Actions   []Action `json:"actions"`
	Summary   string   `json:"summary"`
}

// ListServices lists services using the native service manager when available.
func ListServices() (*ListReport, error) {
	switch runtime.GOOS {
	case "linux":
		return listLinuxServices(), nil
	case "darwin":
		return listLaunchdServices(), nil
	case "windows":
		return listWindowsServices(), nil
	default:
		return unsupportedReport(), nil
	}
}

// Status returns a read-only status for one service.
func Status(name string) (*ServiceInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("service name is required")
	}

	switch runtime.GOOS {
	case "linux":
		return linuxServiceStatus(name), nil
	case "darwin":
		return launchdServiceStatus(name), nil
	case "windows":
		return windowsServiceStatus(name), nil
	default:
		info := unsupportedService(name)
		return &info, nil
	}
}

// Diagnose returns one service status plus non-mutating recovery guidance.
func Diagnose(name string) (*DiagnoseReport, error) {
	info, err := Status(name)
	if err != nil {
		return nil, err
	}

	recs := []string{}
	switch info.Status {
	case "available":
		switch strings.ToLower(info.State) {
		case "running", "active":
			recs = append(recs, "Service is visible and running; inspect application logs if failures continue.")
		case "inactive", "stopped", "failed", "exited":
			recs = append(recs, "Service is visible but not running; review logs and restart manually if appropriate.")
		default:
			recs = append(recs, "Service is visible; inspect the native service manager for detailed state.")
		}
	case "not found":
		recs = append(recs, "Confirm the service name and whether the service is installed on this machine.")
	case "permission denied":
		recs = append(recs, "Re-run the native status command with appropriate permissions or inspect service manager policy.")
	case "timeout":
		recs = append(recs, "Run the native service manager command manually; the status command did not complete in time.")
	case "not supported":
		recs = append(recs, "This platform or service manager is not supported by envdoctor yet.")
	}

	if info.Recommendation != "" {
		recs = append(recs, info.Recommendation)
	}

	return &DiagnoseReport{
		Service:         *info,
		Recommendations: dedupeStrings(recs),
	}, nil
}

// Plan creates structured service actions for safe dry-run/apply execution.
func Plan(operation, name string) (*PlanReport, error) {
	operation, err := normalizeServiceOperation(operation)
	if err != nil {
		return nil, err
	}
	name, err = validateServiceName(name)
	if err != nil {
		return nil, err
	}

	report := &PlanReport{
		Service:   name,
		Operation: operation,
		Platform:  runtime.GOOS,
		Status:    "safe-execution-preview",
	}

	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("systemctl"); err == nil {
			report.Manager = "systemd"
			report.Actions = systemdActions(operation, name)
		} else {
			report.Manager = "linux-service"
			report.Actions = []Action{manualServiceAction(operation, name, "No supported Linux service executor was found. Use the native service manager manually after reviewing logs.")}
		}
	case "windows":
		if _, err := exec.LookPath("sc.exe"); err == nil {
			report.Manager = "windows-service"
			report.Actions = windowsServiceActions(operation, name)
		} else {
			report.Manager = "windows-service"
			report.Actions = []Action{manualServiceAction(operation, name, "sc.exe was not found. Use Windows Services or PowerShell manually after reviewing logs.")}
		}
	case "darwin":
		report.Manager = "launchd"
		report.Actions = []Action{manualServiceAction(operation, name, "launchctl requires an explicit service domain. envdoctor keeps macOS service mutation manual in this preview.")}
	default:
		report.Manager = "none"
		report.Actions = []Action{manualServiceAction(operation, name, "Service mutation is not supported on this platform.")}
	}

	report.Summary = fmt.Sprintf("Generated %d service %s action(s) for %s. Apply still requires executor policy approval.", len(report.Actions), operation, name)
	return report, nil
}

func listLinuxServices() *ListReport {
	if _, err := exec.LookPath("systemctl"); err == nil {
		out, err := command.CombinedOutput("systemctl", "list-units", "--type=service", "--all", "--no-pager", "--no-legend")
		if err != nil {
			return reportFromCommandError("systemd", err, string(out))
		}

		services := parseSystemctlList(string(out))
		return &ListReport{
			Platform: runtime.GOOS,
			Manager:  "systemd",
			Status:   "available",
			Services: services,
		}
	}

	if hasInitD() {
		return &ListReport{
			Platform: runtime.GOOS,
			Manager:  "init.d",
			Status:   "available",
			Services: listInitDServices(),
		}
	}

	return &ListReport{
		Platform: runtime.GOOS,
		Manager:  "none",
		Status:   "not supported",
		Message:  "No supported Linux service manager found.",
	}
}

func normalizeServiceOperation(operation string) (string, error) {
	operation = strings.ToLower(strings.TrimSpace(operation))
	switch operation {
	case "start", "stop", "restart":
		return operation, nil
	case "enable", "disable", "delete", "remove", "fix":
		return "", fmt.Errorf("service operation %q is intentionally blocked; use start, stop, or restart", operation)
	default:
		return "", fmt.Errorf("unsupported service operation %q; use start, stop, or restart", operation)
	}
}

func validateServiceName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("service name is required")
	}
	if strings.ContainsAny(name, `/\*?[]{};|&<>$`+"`") || strings.Contains(name, "..") {
		return "", fmt.Errorf("unsafe service name %q; wildcards, paths, and shell operators are blocked", name)
	}
	if strings.ContainsAny(name, "\n\r\t ") {
		return "", fmt.Errorf("unsafe service name %q; whitespace is blocked in service operations", name)
	}
	return name, nil
}

func systemdActions(operation, name string) []Action {
	return []Action{commandServiceAction(operation, name, "systemctl", []string{operation, name})}
}

func windowsServiceActions(operation, name string) []Action {
	switch operation {
	case "restart":
		return []Action{
			commandServiceAction("stop", name, "sc.exe", []string{"stop", name}),
			commandServiceAction("start", name, "sc.exe", []string{"start", name}),
		}
	default:
		return []Action{commandServiceAction(operation, name, "sc.exe", []string{operation, name})}
	}
}

func commandServiceAction(operation, name, commandName string, args []string) Action {
	return Action{
		ID:            fmt.Sprintf("service-%s-%s", operation, serviceSlug(name)),
		Source:        "service",
		Category:      "Service",
		Operation:     operation,
		Title:         fmt.Sprintf("%s service %s", titleOperation(operation), name),
		Description:   "Structured service operation generated by envdoctor. Execution requires --yes and policy approval.",
		Command:       commandName,
		Args:          args,
		Risk:          "high",
		RequiresAdmin: true,
		SafeToRun:     true,
		Timeout:       "2m",
		RollbackHint:  serviceRollbackHint(operation, name),
		Status:        "safe-execution-preview",
	}
}

func manualServiceAction(operation, name, steps string) Action {
	return Action{
		ID:            fmt.Sprintf("service-%s-%s-manual", operation, serviceSlug(name)),
		Source:        "service",
		Category:      "Service",
		Operation:     operation,
		Title:         fmt.Sprintf("Review service %s for %s", name, operation),
		ManualSteps:   steps,
		Risk:          "high",
		RequiresAdmin: true,
		SafeToRun:     false,
		Timeout:       "2m",
		RollbackHint:  serviceRollbackHint(operation, name),
		Status:        "manual",
	}
}

func serviceRollbackHint(operation, name string) string {
	switch operation {
	case "start":
		return "If the start causes issues, stop the service again after collecting logs: " + name
	case "stop":
		return "If the stop causes issues, start the service again after collecting logs: " + name
	case "restart":
		return "If restart causes issues, inspect service status and logs before further changes: " + name
	default:
		return "Inspect service status and logs before further changes: " + name
	}
}

func serviceSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('-')
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "service"
	}
	return result
}

func titleOperation(operation string) string {
	if operation == "" {
		return "Service"
	}
	return strings.ToUpper(operation[:1]) + operation[1:]
}

func linuxServiceStatus(name string) *ServiceInfo {
	if _, err := exec.LookPath("systemctl"); err == nil {
		return systemdStatus(name)
	}

	if hasInitD() {
		return initDStatus(name)
	}

	info := unsupportedService(name)
	info.Platform = runtime.GOOS
	return &info
}

func systemdStatus(name string) *ServiceInfo {
	info := &ServiceInfo{
		Name:     name,
		Manager:  "systemd",
		Platform: runtime.GOOS,
	}

	out, err := command.CombinedOutput("systemctl", "show", name, "--property=LoadState,ActiveState,SubState", "--no-pager")
	raw := strings.TrimSpace(string(out))
	info.Raw = raw
	if err != nil {
		return classifyServiceCommandError(info, err, raw)
	}

	fields := parseSystemdShow(raw)
	if fields["LoadState"] == "not-found" || fields["LoadState"] == "" {
		info.Status = "not found"
		info.Description = "Service was not found by systemd."
		return info
	}

	info.Status = "available"
	info.State = firstNonEmpty(fields["ActiveState"], fields["SubState"])
	info.Description = fmt.Sprintf("systemd load state: %s", fields["LoadState"])
	return info
}

func initDStatus(name string) *ServiceInfo {
	info := &ServiceInfo{
		Name:     name,
		Manager:  "init.d",
		Platform: runtime.GOOS,
	}

	path := filepath.Join("/etc/init.d", name)
	if _, err := os.Stat(path); err != nil {
		info.Status = "not found"
		info.Description = "No init.d script exists for this service."
		return info
	}

	out, err := command.CombinedOutput(path, "status")
	raw := strings.TrimSpace(string(out))
	info.Raw = raw
	if err != nil {
		if raw == "" {
			return classifyServiceCommandError(info, err, raw)
		}
	}

	info.Status = "available"
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "running") || strings.Contains(lower, "started"):
		info.State = "running"
	case strings.Contains(lower, "stopped") || strings.Contains(lower, "not running"):
		info.State = "stopped"
	default:
		info.State = "unknown"
	}
	return info
}

func listLaunchdServices() *ListReport {
	if _, err := exec.LookPath("launchctl"); err != nil {
		return &ListReport{
			Platform: runtime.GOOS,
			Manager:  "launchd",
			Status:   "not supported",
			Message:  "launchctl was not found.",
		}
	}

	out, err := command.CombinedOutput("launchctl", "list")
	if err != nil {
		return reportFromCommandError("launchd", err, string(out))
	}

	return &ListReport{
		Platform: runtime.GOOS,
		Manager:  "launchd",
		Status:   "available",
		Services: parseLaunchctlList(string(out)),
	}
}

func launchdServiceStatus(name string) *ServiceInfo {
	info := &ServiceInfo{
		Name:     name,
		Manager:  "launchd",
		Platform: runtime.GOOS,
	}

	if _, err := exec.LookPath("launchctl"); err != nil {
		info.Status = "not supported"
		info.Description = "launchctl was not found."
		return info
	}

	out, err := command.CombinedOutput("launchctl", "list")
	raw := strings.TrimSpace(string(out))
	info.Raw = raw
	if err != nil {
		return classifyServiceCommandError(info, err, raw)
	}

	for _, svc := range parseLaunchctlList(raw) {
		if svc.Name == name {
			info.Status = "available"
			info.State = svc.State
			info.Description = "Service label was found in launchctl list."
			return info
		}
	}

	info.Status = "not found"
	info.Description = "Service label was not found in launchctl list."
	return info
}

func listWindowsServices() *ListReport {
	if _, err := exec.LookPath("sc.exe"); err != nil {
		return &ListReport{
			Platform: runtime.GOOS,
			Manager:  "windows-service",
			Status:   "not supported",
			Message:  "sc.exe was not found.",
		}
	}

	out, err := command.CombinedOutput("sc.exe", "query", "type=", "service", "state=", "all")
	if err != nil {
		return reportFromCommandError("windows-service", err, string(out))
	}

	return &ListReport{
		Platform: runtime.GOOS,
		Manager:  "windows-service",
		Status:   "available",
		Services: parseSCQuery(string(out)),
	}
}

func windowsServiceStatus(name string) *ServiceInfo {
	info := &ServiceInfo{
		Name:     name,
		Manager:  "windows-service",
		Platform: runtime.GOOS,
	}

	if _, err := exec.LookPath("sc.exe"); err != nil {
		info.Status = "not supported"
		info.Description = "sc.exe was not found."
		return info
	}

	out, err := command.CombinedOutput("sc.exe", "query", name)
	raw := strings.TrimSpace(string(out))
	info.Raw = raw
	if err != nil {
		return classifyServiceCommandError(info, err, raw)
	}

	parsed := parseSCQuery(raw)
	if len(parsed) == 0 {
		info.Status = "not found"
		info.Description = "Service was not found by Windows Service Control."
		return info
	}

	info.Status = "available"
	info.State = parsed[0].State
	info.Description = "Service was found by Windows Service Control."
	return info
}

func unsupportedReport() *ListReport {
	return &ListReport{
		Platform: runtime.GOOS,
		Manager:  "none",
		Status:   "not supported",
		Message:  "Service discovery is not supported on this platform.",
	}
}

func unsupportedService(name string) ServiceInfo {
	return ServiceInfo{
		Name:        name,
		Manager:     "none",
		Platform:    runtime.GOOS,
		Status:      "not supported",
		Description: "Service discovery is not supported on this platform.",
	}
}

func reportFromCommandError(manager string, err error, raw string) *ListReport {
	status := "not supported"
	message := err.Error()
	lower := strings.ToLower(raw + " " + err.Error())
	switch {
	case strings.Contains(lower, "permission denied") || strings.Contains(lower, "access is denied"):
		status = "permission denied"
	case strings.Contains(lower, "command timed out"):
		status = "timeout"
	}
	return &ListReport{
		Platform: runtime.GOOS,
		Manager:  manager,
		Status:   status,
		Message:  strings.TrimSpace(firstNonEmpty(raw, message)),
	}
}

func classifyServiceCommandError(info *ServiceInfo, err error, raw string) *ServiceInfo {
	lower := strings.ToLower(raw + " " + err.Error())
	switch {
	case strings.Contains(lower, "permission denied") || strings.Contains(lower, "access is denied"):
		info.Status = "permission denied"
		info.Description = "Service manager denied access."
	case strings.Contains(lower, "command timed out"):
		info.Status = "timeout"
		info.Description = "Service status command timed out."
	case strings.Contains(lower, "not found") || strings.Contains(lower, "could not be found") || strings.Contains(lower, "does not exist"):
		info.Status = "not found"
		info.Description = "Service was not found."
	default:
		info.Status = "not found"
		info.Description = strings.TrimSpace(firstNonEmpty(raw, err.Error()))
	}
	return info
}

func parseSystemctlList(out string) []ServiceInfo {
	var services []ServiceInfo
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		services = append(services, ServiceInfo{
			Name:        fields[0],
			Manager:     "systemd",
			Platform:    runtime.GOOS,
			Status:      "available",
			State:       fields[2],
			Description: strings.Join(fields[4:], " "),
		})
	}
	sortServices(services)
	return services
}

func parseSystemdShow(raw string) map[string]string {
	fields := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok {
			fields[key] = value
		}
	}
	return fields
}

func hasInitD() bool {
	info, err := os.Stat("/etc/init.d")
	return err == nil && info.IsDir()
}

func listInitDServices() []ServiceInfo {
	entries, err := os.ReadDir("/etc/init.d")
	if err != nil {
		return nil
	}

	services := make([]ServiceInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		services = append(services, ServiceInfo{
			Name:     entry.Name(),
			Manager:  "init.d",
			Platform: runtime.GOOS,
			Status:   "available",
			State:    "unknown",
		})
	}
	sortServices(services)
	return services
}

func parseLaunchctlList(out string) []ServiceInfo {
	var services []ServiceInfo
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] == "PID" {
			continue
		}
		state := "stopped"
		if fields[0] != "-" {
			state = "running"
		} else if fields[1] != "0" {
			state = "exited"
		}
		services = append(services, ServiceInfo{
			Name:     fields[2],
			Manager:  "launchd",
			Platform: runtime.GOOS,
			Status:   "available",
			State:    state,
		})
	}
	sortServices(services)
	return services
}

func parseSCQuery(out string) []ServiceInfo {
	var services []ServiceInfo
	current := ServiceInfo{
		Manager:  "windows-service",
		Platform: runtime.GOOS,
		Status:   "available",
	}

	flush := func() {
		if current.Name == "" {
			return
		}
		services = append(services, current)
		current = ServiceInfo{
			Manager:  "windows-service",
			Platform: runtime.GOOS,
			Status:   "available",
		}
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SERVICE_NAME:") {
			flush()
			current.Name = strings.TrimSpace(strings.TrimPrefix(line, "SERVICE_NAME:"))
			continue
		}
		if strings.HasPrefix(line, "STATE") {
			_, value, ok := strings.Cut(line, ":")
			if ok {
				parts := strings.Fields(strings.TrimSpace(value))
				if len(parts) > 1 {
					current.State = strings.ToLower(parts[1])
				}
			}
		}
	}
	flush()
	sortServices(services)
	return services
}

func sortServices(services []ServiceInfo) {
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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
	return result
}
