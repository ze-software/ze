package sourcerewrite

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	ruleH1           = regexp.MustCompile(`^#\s+(.+?)\s*$`)
	blockingMark     = regexp.MustCompile(`(?i)^\*\*BLOCKING[:.]?\*\*\s*`)
	blockingAnywhere = regexp.MustCompile(`(?i)\*\*BLOCKING[:.]?\*\*`)
	whenMark         = regexp.MustCompile(`^\*\*When:\*\*\s*(.*)$`)
	severityMark     = regexp.MustCompile(`^\*\*Severity:\*\*`)
	extendsLine      = regexp.MustCompile(`(?i)^(?:Extends|Related):\s*(.*)$`)
	slugRef          = regexp.MustCompile("`?ai/rules/([a-z0-9]+(?:-[a-z0-9]+)*)\\.md`?")
	whenTrigger      = regexp.MustCompile(`(?i)^when\b(.*?)[,:]`)
	spaceRun         = regexp.MustCompile(`\s+`)
)

var skippedRuleNames = map[string]bool{
	"INDEX.md": true, "CONDENSED.md": true, "rule-format.md": true,
}

// rulesReformatReport counts migrated and skipped rule files.
type rulesReformatReport struct {
	DryRun  bool     `json:"dry_run"`
	Changed int      `json:"changed"`
	Skipped int      `json:"skipped"`
	Files   []string `json:"files"`
}

func (r rulesReformatReport) Text() string {
	var b strings.Builder
	verb := "migrated"
	if r.DryRun {
		verb = "WOULD migrate"
	}
	for _, file := range r.Files {
		fmt.Fprintf(&b, "%s %s\n", verb, file)
	}
	fmt.Fprintf(&b, "\n%d migrated, %d already conform / skipped\n", r.Changed, r.Skipped)
	return b.String()
}

// reformatRules applies the rules_reformat.py migration to ai/rules/*.md.
func reformatRules(root string, dryRun bool) (rulesReformatReport, error) {
	dir := filepath.Join(root, "ai", "rules")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return rulesReformatReport{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	report := rulesReformatReport{DryRun: dryRun, Files: []string{}}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".md" || skippedRuleNames[name] {
			continue
		}
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return report, err
		}
		text := string(raw)
		if !utf8.Valid(raw) {
			text = strings.ToValidUTF8(text, "\uFFFD")
		}
		content, changed, _ := migrateRule(text, strings.TrimSuffix(name, ".md"))
		if !changed {
			report.Skipped++
			continue
		}
		report.Changed++
		rel := filepath.ToSlash(filepath.Join("ai", "rules", name))
		report.Files = append(report.Files, rel)
		if dryRun {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return report, err
		}
		if err := os.WriteFile(path, []byte(content), info.Mode().Perm()); err != nil {
			return report, err
		}
	}
	return report, nil
}

// migrateRule returns the canonical rule bytes and whether the producer would write them.
func migrateRule(raw, stem string) (string, bool, string) {
	lines := pythonSplitLines(raw)
	idx := 0
	for idx < len(lines) && strings.TrimSpace(lines[idx]) == "" {
		idx++
	}
	if idx >= len(lines) {
		return "", false, "no H1 title"
	}
	match := ruleH1.FindStringSubmatch(strings.TrimSpace(lines[idx]))
	if match == nil {
		return "", false, "no H1 title"
	}
	title := match[1]
	body := append([]string(nil), lines[idx+1:]...)
	probe := make([]string, 0, 4)
	for _, line := range body {
		if strings.TrimSpace(line) != "" {
			probe = append(probe, line)
			if len(probe) == 4 {
				break
			}
		}
	}
	hasWhen, hasSeverity := false, false
	for _, line := range probe {
		s := strings.TrimSpace(line)
		hasWhen = hasWhen || whenMark.MatchString(s)
		hasSeverity = hasSeverity || severityMark.MatchString(s)
	}
	if hasWhen && hasSeverity {
		return "", false, "already conforms"
	}

	severity := "advisory"
	if blockingAnywhere.MatchString(raw) {
		severity = "blocking"
	}
	when := ""
	related := []string{}
	kept := make([]string, 0, len(body))
	for i := 0; i < len(body); {
		line := body[i]
		s := strings.TrimSpace(line)
		if wm := whenMark.FindStringSubmatch(s); wm != nil && when == "" {
			parts := []string{strings.TrimSpace(wm[1])}
			i++
			for i < len(body) {
				next := strings.TrimSpace(body[i])
				if next == "" || strings.HasPrefix(next, "**") || strings.HasPrefix(next, "#") ||
					strings.HasPrefix(next, "|") || strings.HasPrefix(next, ">") ||
					strings.HasPrefix(next, "-") || strings.HasPrefix(next, "<!--") {
					break
				}
				parts = append(parts, next)
				i++
			}
			when = strings.TrimSpace(spaceRun.ReplaceAllString(strings.Join(parts, " "), " "))
			continue
		}
		if em := extendsLine.FindStringSubmatch(s); em != nil {
			for _, ref := range slugRef.FindAllStringSubmatch(em[1], -1) {
				related = append(related, ref[1])
			}
			i++
			continue
		}
		if bm := blockingMark.FindStringIndex(s); bm != nil {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t\r\n"))]
			kept = append(kept, indent+s[bm[1]:])
			i++
			continue
		}
		kept = append(kept, line)
		i++
	}
	if when == "" {
		when = deriveWhen(kept)
		if when == "" {
			when = "working on " + strings.ToLower(title)
		}
	}
	seen := map[string]bool{}
	unique := related[:0]
	for _, ref := range related {
		if ref != stem && !seen[ref] {
			seen[ref] = true
			unique = append(unique, ref)
		}
	}
	related = unique
	for len(kept) > 0 && strings.TrimSpace(kept[0]) == "" {
		kept = kept[1:]
	}
	if len(kept) > 0 && !strings.HasPrefix(strings.TrimLeft(kept[0], " \t"), "##") {
		leadEnd := len(kept)
		for i, line := range kept {
			if strings.HasPrefix(strings.TrimLeft(line, " \t"), "## ") {
				leadEnd = i
				break
			}
		}
		nonblank := false
		for _, line := range kept[:leadEnd] {
			nonblank = nonblank || strings.TrimSpace(line) != ""
		}
		if nonblank {
			wrapped := []string{"## Directives", ""}
			wrapped = append(wrapped, kept[:leadEnd]...)
			wrapped = append(wrapped, kept[leadEnd:]...)
			kept = wrapped
		}
	}
	meta := []string{"**When:** " + when, "**Severity:** " + severity}
	if len(related) > 0 {
		meta = append(meta, "**Related:** "+strings.Join(related, ", "))
	}
	out := []string{"# " + title, ""}
	out = append(out, meta...)
	out = append(out, "")
	out = append(out, kept...)
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n") + "\n", true, "migrated"
}

func deriveWhen(lines []string) string {
	inFence := false
	blockingText, proseText := "", ""
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "```") || strings.HasPrefix(s, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "|") ||
			strings.HasPrefix(s, ">") || strings.HasPrefix(s, "<!--") {
			continue
		}
		if m := blockingMark.FindStringIndex(s); m != nil && blockingText == "" {
			blockingText = strings.TrimSpace(s[m[1]:])
		}
		if proseText == "" && !strings.HasPrefix(s, "**") && !strings.HasPrefix(s, "-") &&
			!strings.HasPrefix(s, "*") && !strings.HasPrefix(s, "[") {
			lower := strings.ToLower(s)
			skip := false
			for _, prefix := range []string{"rationale", "principle", "extends", "related", "see"} {
				if strings.HasPrefix(lower, prefix) {
					skip = true
					break
				}
			}
			if !skip {
				proseText = s
			}
		}
	}
	chosen := blockingText
	if chosen == "" {
		chosen = proseText
	}
	if chosen == "" {
		return ""
	}
	if match := whenTrigger.FindStringSubmatch(chosen); match != nil {
		return "when " + strings.TrimSpace(match[1])
	}
	return firstSentence(chosen, 180)
}

func firstSentence(text string, limit int) string {
	text = strings.TrimSpace(spaceRun.ReplaceAllString(text, " "))
	end := -1
	if loc := regexp.MustCompile(`\.(\s|$)`).FindStringIndex(text); loc != nil && utf8.RuneCountInString(text[:loc[0]]) <= limit {
		end = loc[0]
	} else if loc := regexp.MustCompile(`:(\s|$)`).FindStringIndex(text); loc != nil {
		position := utf8.RuneCountInString(text[:loc[0]])
		if position >= 40 && position <= limit {
			end = loc[0]
		}
	}
	if end >= 0 {
		return strings.TrimSpace(text[:end])
	}
	runes := []rune(text)
	if len(runes) > limit {
		cut := strings.LastIndex(string(runes[:limit-1]), " ")
		if cut <= 0 {
			return string(runes[:limit-1])
		}
		return strings.TrimRight(string(runes[:limit-1])[:cut], " \t\r\n")
	}
	return strings.TrimSuffix(text, ".")
}

func pythonSplitLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	if raw == "" {
		return []string{}
	}
	lines := strings.Split(raw, "\n")
	if strings.HasSuffix(raw, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}
