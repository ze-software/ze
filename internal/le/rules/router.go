// Design: docs/architecture/core-design.md -- the routing risk, measured
// Overview: digest.go -- the tokenizer and the core this report subtracts
// Overview: actions.go -- the action that runs this
//
// router.go ports `rules_router.py`. It reports which rules a `**When:**`
// trigger index surfaces for past work. It also reports blocking rules that NO
// corpus task surfaces.
//
// A trigger index saves tokens but risks hiding a blocking rule from a session.
// This report measures that risk with past work.
//
// An always-loaded digest contains every rule for EVERY task. Under a literal
// comparison, any rule that one task does not surface is "missed". That list
// only measures rule-set size. Instead, a blocking rule is MISSED when NO corpus
// task surfaces it. Such a rule is dark across the corpus. The always-on core
// protects that population.
//
// Core rules are excluded because routing cannot miss an always-loaded rule.
//
// The scoring models a keyword reader, which is WEAKER than a model that reads
// trigger meaning. It therefore over-reports misses and gives a risk floor.
//
// The report computes the subtracted core WITHOUT the corpus. Passing the corpus
// would remove every corpus-derived member from the missed set. The report would
// then print "MISSED: none" because its output had already changed its input.

package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// taskSections names the `##` sections that contain work descriptions from
// either lifecycle half. It matches the heading's FIRST word, so it includes
// `## Task (MANDATORY)`.
var taskSections = [...]string{"context", "task"}

// loadCorpus answers the task descriptions in one directory of specs.
//
// A missing directory gives no tasks, as in the script. The derivation that
// needs the corpus reports an empty result (digest.go, unreachableBlocking).
// That layer knows which answer the empty read prevented.
func loadCorpus(dir string) ([]Task, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		if name == "TEMPLATE.md" || isUpperStem(strings.TrimSuffix(name, ".md")) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	var corpus []Task
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name)) // #nosec G304 -- a path derived from the checkout
		if err != nil {
			return nil, err
		}
		text := taskSectionText(splitLines(string(raw)))
		if text == "" {
			continue
		}
		corpus = append(corpus, Task{Source: name, Text: text})
	}
	return corpus, nil
}

// taskSectionText answers the prose of every task-stating `##` section.
func taskSectionText(lines []string) string {
	var out []string
	capturing := false
	for _, line := range lines {
		heading, ok := taskHeading(line)
		if ok {
			capturing = isTaskSection(heading)
			continue
		}
		if capturing {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// taskHeading answers the text of a `## ` heading, and only of a `## `
// heading: a deeper one is content of the section it sits in.
func taskHeading(line string) (string, bool) {
	if !strings.HasPrefix(line, "##") {
		return "", false
	}
	rest := line[2:]
	trimmed := strings.TrimLeft(rest, asciiSpace)
	if trimmed == rest {
		return "", false
	}
	return trimmed, true
}

// isTaskSection reports whether a heading's first word names a section that
// states the work.
func isTaskSection(heading string) bool {
	first := strings.ToLower(strings.TrimSpace(heading))
	if at := strings.IndexByte(first, ' '); at >= 0 {
		first = first[:at]
	}
	for _, want := range taskSections {
		if first == want {
			return true
		}
	}
	return false
}

// RouterTask is one past task description and the rules it surfaces.
type RouterTask struct {
	Source           string   `json:"source"`
	Surfaced         []string `json:"surfaced"`
	SurfacedBlocking []string `json:"surfaced-blocking"`
}

// RouterReport is the measurement: what routing reaches, and what it does not.
type RouterReport struct {
	Tasks           []RouterTask `json:"tasks"`
	CorpusSize      int          `json:"corpus-size"`
	RulesTotal      int          `json:"rules-total"`
	Core            []string     `json:"core"`
	Routed          int          `json:"routed"`
	BlockingRouted  int          `json:"blocking-routed"`
	SurfacedAny     []string     `json:"surfaced-any"`
	MissedBlocking  []string     `json:"missed-blocking"`
	UnroutableTerms []string     `json:"unroutable-terms"`
}

// Text renders the measurement for a person. The script's per-task `--verbose`
// list is now the payload's `tasks` rows. A single command answer therefore has
// multiple renderings through `| json`.
func (r RouterReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("corpus: ").Int(int64(r.CorpusSize)).Str(" past task descriptions\n")
	tb.Str("rules: ").Int(int64(r.RulesTotal)).Str(" (").Int(int64(len(r.Core))).
		Str(" always-on core, ").Int(int64(r.Routed)).Str(" routed, of which ").
		Int(int64(r.BlockingRouted)).Str(" blocking)\n\n")

	low, high, total := 0, 0, 0
	for i, task := range r.Tasks {
		count := len(task.Surfaced)
		if i == 0 || count < low {
			low = count
		}
		if count > high {
			high = count
		}
		total += count
	}
	mean := 0.0
	if len(r.Tasks) > 0 {
		mean = float64(total) / float64(len(r.Tasks))
	}
	tb.Str("surfaced per task: min ").Int(int64(low)).Str(", max ").Int(int64(high)).
		Str(", mean ").Float(mean, 1).Byte('\n')
	tb.Str("blocking rules surfaced by at least one task: ").
		Int(int64(r.BlockingRouted - len(r.MissedBlocking))).Str(" of ").
		Int(int64(r.BlockingRouted)).Str("\n\n")

	if len(r.MissedBlocking) > 0 {
		tb.Str("MISSED: ").Int(int64(len(r.MissedBlocking))).
			Str(" blocking rule(s) no task in the corpus surfaces. Each is a candidate for the always-on core:\n")
		for _, name := range r.MissedBlocking {
			tb.Str("    ").Str(rulesRel).Byte('/').Str(name).Byte('\n')
		}
	} else {
		tb.Str("MISSED: none. Every routed blocking rule is surfaced by some task\n")
	}
	if len(r.UnroutableTerms) > 0 {
		tb.Byte('\n')
		tb.Str("triggers with no distinctive term (they share every word with other triggers): ").
			Int(int64(len(r.UnroutableTerms))).Byte('\n')
		for _, name := range r.UnroutableTerms {
			tb.Str("    ").Str(rulesRel).Byte('/').Str(name).Byte('\n')
		}
	}
	return tb.String()
}

// Router measures which rules a trigger index would surface for past work.
func Router(tree string) (RouterReport, error) {
	var report RouterReport

	rules, err := loadRules(rulesDir(tree))
	if err != nil {
		return report, err
	}
	corpus, err := loadCorpus(filepath.Join(tree, "plan"))
	if err != nil {
		return report, err
	}
	core, err := coreMembers(rules, taskCorpus{}, nil)
	if err != nil {
		return report, err
	}
	return buildRouterReport(rules, corpus, core), nil
}

// buildRouterReport joins the corpus against the routed rules.
func buildRouterReport(rules []Rule, corpus []Task, core []Rule) RouterReport {
	inCore := coreNames(core)
	terms := distinctiveTerms(rules)

	routed := make([]Rule, 0, len(rules))
	for i := range rules {
		if !inCore[rules[i].Name] {
			routed = append(routed, rules[i])
		}
	}

	surfacedAny := make(map[string]bool)
	tasks := make([]RouterTask, 0, len(corpus))
	for _, entry := range corpus {
		taskTerms := significantTerms(entry.Text)
		task := RouterTask{Source: entry.Source, Surfaced: []string{}, SurfacedBlocking: []string{}}
		for j := range routed {
			rule := &routed[j]
			if overlap(terms[rule.Name], taskTerms) < minHits {
				continue
			}
			task.Surfaced = append(task.Surfaced, rule.Name)
			if rule.Severity == severityBlocking {
				task.SurfacedBlocking = append(task.SurfacedBlocking, rule.Name)
			}
			surfacedAny[rule.Name] = true
		}
		sort.Strings(task.Surfaced)
		sort.Strings(task.SurfacedBlocking)
		tasks = append(tasks, task)
	}

	report := RouterReport{
		Tasks:           tasks,
		CorpusSize:      len(corpus),
		RulesTotal:      len(rules),
		Core:            sortedNames(inCore),
		Routed:          len(routed),
		SurfacedAny:     sortedNames(surfacedAny),
		MissedBlocking:  []string{},
		UnroutableTerms: []string{},
	}
	for i := range routed {
		rule := &routed[i]
		if rule.Severity == severityBlocking {
			report.BlockingRouted++
			if !surfacedAny[rule.Name] {
				report.MissedBlocking = append(report.MissedBlocking, rule.Name)
			}
		}
		if len(terms[rule.Name]) == 0 {
			report.UnroutableTerms = append(report.UnroutableTerms, rule.Name)
		}
	}
	sort.Strings(report.MissedBlocking)
	sort.Strings(report.UnroutableTerms)
	return report
}
