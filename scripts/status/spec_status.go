// Design: (none -- build tool)
//
// spec_status generates the spec inventory from metadata in plan/spec-*.md.
//
// Each spec file has a metadata table at the top with Status, Depends, Phase
// and Updated fields. This tool extracts those, joins git last-modified date,
// detects the spec set from the filename, sorts by status order then by
// updated date descending, and prints either a fixed-width table or JSON.
//
// Usage: go run scripts/status/spec_status.go [--json]
//
// Replaces the previous bash implementation (scripts/spec-status.sh).

//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type spec struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Depends     string `json:"depends"`
	Phase       string `json:"phase"`
	Set         string `json:"set"`
	Updated     string `json:"updated"`
	GitModified string `json:"git-modified"`
}

// statusOrder returns the sort key for a status (lower = sorted first).
func statusOrder(status string) int {
	switch status {
	case "in-progress":
		return 1
	case "ready":
		return 2
	case "design":
		return 3
	case "skeleton":
		return 4
	case "blocked":
		return 5
	case "deferred":
		return 6
	}
	return 9
}

// detectSet returns the spec set from the filename pattern
// "spec-<prefix>-<N>-<name>.md", or "-" if no match.
var setRE = regexp.MustCompile(`^spec-([a-z]+(?:-[a-z]+)*)-[0-9]+-.*\.md$`)

func detectSet(filename string) string {
	m := setRE.FindStringSubmatch(filename)
	if m == nil {
		return "-"
	}
	return m[1]
}

// extractField returns the value of a metadata field from a spec's table.
// The table has rows like "| Status | design |"; only the first 10 lines of
// the file are scanned to avoid matching unrelated tables further down.
func extractField(content, field string) string {
	pattern := regexp.MustCompile(`^\|\s*` + regexp.QuoteMeta(field) + `\s*\|\s*([^|]*?)\s*\|`)
	lines := strings.SplitN(content, "\n", 11)
	if len(lines) > 10 {
		lines = lines[:10]
	}
	for _, line := range lines {
		if m := pattern.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

// gitDate returns git's last-modified date for a file (YYYY-MM-DD), falling
// back to "unknown" if git is unavailable or the file is untracked.
func gitDate(path string) string {
	out, err := exec.Command("git", "log", "-1", "--format=%as", "--", path).Output()
	if err != nil {
		return "unknown"
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "unknown"
	}
	return s
}

// loadSpec reads metadata from a single spec file.
func loadSpec(path string) (spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return spec{}, err
	}
	content := string(data)
	base := filepath.Base(path)
	name := strings.TrimSuffix(strings.TrimPrefix(base, "spec-"), ".md")

	s := spec{
		Name:        name,
		Status:      extractField(content, "Status"),
		Depends:     extractField(content, "Depends"),
		Phase:       extractField(content, "Phase"),
		Updated:     extractField(content, "Updated"),
		Set:         detectSet(base),
		GitModified: gitDate(path),
	}
	if s.Status == "" {
		s.Status = "unknown"
	}
	if s.Depends == "" {
		s.Depends = "-"
	}
	if s.Phase == "" {
		s.Phase = "-"
	}
	if s.Updated == "" {
		s.Updated = s.GitModified
	}
	return s, nil
}

func loadAllSpecs() ([]spec, error) {
	matches, err := filepath.Glob("plan/spec-*.md")
	if err != nil {
		return nil, err
	}
	var specs []spec
	for _, path := range matches {
		if filepath.Base(path) == "spec-template.md" {
			continue
		}
		s, err := loadSpec(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		specs = append(specs, s)
	}
	sort.SliceStable(specs, func(i, j int) bool {
		oi, oj := statusOrder(specs[i].Status), statusOrder(specs[j].Status)
		if oi != oj {
			return oi < oj
		}
		// Updated date descending.
		return specs[i].Updated > specs[j].Updated
	})
	return specs, nil
}

func printTable(specs []spec) {
	counts := map[string]int{}
	for _, s := range specs {
		counts[s.Status]++
	}
	order := []string{"in-progress", "ready", "design", "skeleton", "blocked", "deferred", "unknown"}
	var parts []string
	for _, st := range order {
		if counts[st] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[st], st))
		}
	}
	fmt.Printf("Specs: %d total (%s)\n\n", len(specs), strings.Join(parts, ", "))

	const fmtRow = "%-12s  %-10s  %-34s  %-5s  %-10s  %s\n"
	fmt.Printf(fmtRow, "Status", "Updated", "Spec", "Phase", "Set", "Depends")
	fmt.Printf(fmtRow,
		strings.Repeat("─", 12),
		strings.Repeat("─", 10),
		strings.Repeat("─", 34),
		strings.Repeat("─", 5),
		strings.Repeat("─", 10),
		strings.Repeat("─", 10),
	)
	for _, s := range specs {
		fmt.Printf(fmtRow, s.Status, s.Updated, s.Name, s.Phase, s.Set, s.Depends)
	}
}

// printJSON writes the spec list as a JSON array, one record per line,
// matching the layout produced by the previous bash implementation.
// HTML escaping is disabled so `<` and `>` appear literally.
func printJSON(specs []spec) error {
	fmt.Println("[")
	for i, s := range specs {
		var buf strings.Builder
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(s); err != nil {
			return err
		}
		// Encoder appends a trailing newline; trim it.
		line := strings.TrimRight(buf.String(), "\n")
		sep := ","
		if i == len(specs)-1 {
			sep = ""
		}
		fmt.Printf("  %s%s\n", line, sep)
	}
	fmt.Println("]")
	return nil
}

func run() error {
	jsonMode := false
	for _, arg := range os.Args[1:] {
		if arg == "--json" {
			jsonMode = true
		}
	}

	specs, err := loadAllSpecs()
	if err != nil {
		return err
	}

	if jsonMode {
		return printJSON(specs)
	}
	printTable(specs)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "spec_status: %v\n", err)
		os.Exit(1)
	}
}
