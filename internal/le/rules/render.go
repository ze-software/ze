// Design: docs/architecture/core-design.md -- rendering a rule from its points
// Overview: points.go -- the split and the manifest this reads back
// Detail: difflib.go -- the drift page a stale rule is reported as
//
// render.go reads the point tree. It converts a point directory into the
// rendered `ai/rules/<rule>.md` that agents read. Three gates use it.
//
// Every walk fails CLOSED. Each refusal prevents one way to lose an instruction:
// an unlisted point, a listed but missing slug, a duplicate slug, or a misplaced
// point. Other refusals cover a nested directory, a `##` heading that no manifest
// section names, and a slug that is not a bare path component. Partial rendering
// is invalid.
// `./le rules condensed-update` uses WRITE mode, so partial output would delete
// an instruction while every gate stayed green.

package rules

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// renderDir renders the rule text from a point directory on disk.
func renderDir(ruleDir string) (string, error) {
	stem := filepath.Base(ruleDir)
	var tb textbuf.Buffer

	manifestPath := filepath.Join(ruleDir, manifestName)
	raw, err := os.ReadFile(manifestPath) // #nosec G304 -- a path derived from the checkout
	if err != nil {
		return "", errors.New(tb.Str(stem).Str(": no manifest at ").Str(manifestPath).String())
	}
	header, listed, err := parseManifest(string(raw), stem)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(ruleDir)
	if err != nil {
		return "", err
	}
	var loose, onDisk []string
	for _, entry := range entries {
		if entry.IsDir() {
			onDisk = append(onDisk, entry.Name())
			continue
		}
		if strings.HasSuffix(entry.Name(), ".md") && entry.Name() != manifestName {
			loose = append(loose, entry.Name())
		}
	}
	if len(loose) > 0 {
		sort.Strings(loose)
		tb.Reset()
		return "", errors.New(tb.Str(stem).Str(": ").Str(pyListRepr(loose)).
			Str(" sit(s) directly in the rule directory; every point lives in a `##` section directory, so the id is always <rule>/<section>/<slug>. Move it into its section").String())
	}

	// Depth is FIXED at two, and every reader downstream is written to that
	// shape. A point one level deeper is read by nothing, rendered into
	// nothing, and named by no gate, while all three gates exit 0.
	deep, nested, err := deepAndNested(ruleDir)
	if err != nil {
		return "", err
	}
	if len(deep) > 0 {
		tb.Reset()
		return "", errors.New(tb.Str(stem).Str(": ").Str(pyListRepr(deep)).
			Str(" sit(s) below its `##` section directory; the tree is at a fixed depth of two, so nothing reads a point there and its instruction reaches no rendered rule. Move it up to <rule>/<section>/<slug>.md and list the slug in the manifest").String())
	}
	if len(nested) > 0 {
		tb.Reset()
		return "", errors.New(tb.Str(stem).Str(": ").Str(pyListRepr(nested)).
			Str(" is/are directories inside a `##` section directory; a section holds point FILES only, and a directory there is where a point goes to be read by nothing. Remove it").String())
	}

	seenSections := map[string]bool{}
	for _, section := range listed {
		if err := safeSlug(stem, section.Slug, "section"); err != nil {
			return "", err
		}
		if seenSections[section.Slug] {
			tb.Reset()
			return "", errors.New(tb.Str(stem).Str(": duplicate section slug ").
				Str(pyRepr(section.Slug)).String())
		}
		seenSections[section.Slug] = true
	}

	var unlisted []string
	for _, name := range onDisk {
		if !seenSections[name] {
			unlisted = append(unlisted, name)
		}
	}
	if len(unlisted) > 0 {
		sort.Strings(unlisted)
		tb.Reset()
		return "", errors.New(tb.Str(stem).Str(": section directory/ies ").Str(pyListRepr(unlisted)).
			Str(" exist but the manifest does not list them; add them to the reading order or delete them").String())
	}

	sections := make([]Section, 0, len(listed))
	for _, listedSection := range listed {
		section, err := readSection(ruleDir, stem, listedSection)
		if err != nil {
			return "", err
		}
		sections = append(sections, section)
	}

	text := RenderText(header, sections)
	seen := sectionHeadings(strings.Split(strings.TrimSuffix(text, "\n"), "\n"))
	want := make([]string, 0, len(sections))
	for _, section := range sections {
		want = append(want, section.Heading)
	}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		var orphan []string
		for _, h := range seen {
			if !slices.Contains(want, h) {
				orphan = append(orphan, h)
			}
		}
		if len(orphan) == 0 {
			orphan = []string{"(none)"}
		}
		tb.Reset()
		return "", errors.New(tb.Str(stem).Str(": the rendered rule carries ").Int(int64(len(seen))).
			Str(" `##` heading(s) and the manifest names ").Int(int64(len(want))).Str("; ").
			Str(pyListRepr(orphan)).
			Str(" sit(s) inside a point BODY. A `##` opens a section, and a section is a directory: a heading inside a body renders identically and names no directory, so every point after it carries an id naming a section no reader ever sees. Split the point at the heading and add the section to the manifest").String())
	}
	return text, nil
}

// readSection reads one section directory into the points the manifest lists,
// refusing every shape that would drop or duplicate an instruction.
func readSection(ruleDir, stem string, listed manifestSectionSpec) (Section, error) {
	var tb textbuf.Buffer
	sectionDir := filepath.Join(ruleDir, listed.Slug)
	info, err := os.Stat(sectionDir)
	if err != nil || !info.IsDir() {
		return Section{}, errors.New(tb.Str(stem).Str(": manifest lists section ").
			Str(pyRepr(listed.Slug)).Str(" with no directory on disk").String())
	}

	seen := map[string]bool{}
	for _, slug := range listed.Points {
		if err := safeSlug(stem, slug, "point"); err != nil {
			return Section{}, err
		}
		if seen[slug] {
			tb.Reset()
			return Section{}, errors.New(tb.Str(stem).Byte('/').Str(listed.Slug).
				Str(": duplicate slug ").Str(pyRepr(slug)).Str(" in the manifest").String())
		}
		seen[slug] = true
	}

	entries, err := os.ReadDir(sectionDir)
	if err != nil {
		return Section{}, err
	}
	var extra []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if stem := strings.TrimSuffix(entry.Name(), ".md"); !seen[stem] {
			extra = append(extra, stem)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		tb.Reset()
		return Section{}, errors.New(tb.Str(stem).Byte('/').Str(listed.Slug).
			Str(": point file(s) ").Str(pyListRepr(extra)).
			Str(" exist but the manifest does not list them; add them to the reading order or delete them").String())
	}

	var missing []string
	points := make([]Point, 0, len(listed.Points))
	for _, slug := range listed.Points {
		tb.Reset()
		path := filepath.Join(sectionDir, tb.Str(slug).Str(".md").String())
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			missing = append(missing, slug)
		}
	}
	if len(missing) > 0 {
		tb.Reset()
		return Section{}, errors.New(tb.Str(stem).Byte('/').Str(listed.Slug).
			Str(": manifest lists ").Str(pyListRepr(missing)).Str(" with no file on disk").String())
	}
	if len(listed.Points) == 0 {
		tb.Reset()
		return Section{}, errors.New(tb.Str(stem).Str(": section ").Str(pyRepr(listed.Slug)).
			Str(" lists no point; a section directory with nothing in it carries no instruction").String())
	}

	for _, slug := range listed.Points {
		tb.Reset()
		raw, err := os.ReadFile(filepath.Join(sectionDir, tb.Str(slug).Str(".md").String())) // #nosec G304 -- a slug this walk already refused unless it is a bare path component
		if err != nil {
			return Section{}, err
		}
		point, err := parsePoint(string(raw), slug)
		if err != nil {
			return Section{}, err
		}
		points = append(points, point)
	}
	return Section{Slug: listed.Slug, Heading: listed.Heading, Tight: listed.Tight, Points: points}, nil
}

// deepAndNested answers the point files sitting BELOW a section directory, and
// the directories sitting INSIDE one. Both are shapes nothing reads.
func deepAndNested(ruleDir string) (deep, nested []string, err error) {
	err = filepath.WalkDir(ruleDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(ruleDir, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if entry.IsDir() {
			if len(parts) == 2 {
				nested = append(nested, filepath.ToSlash(rel))
			}
			return nil
		}
		if strings.HasSuffix(path, ".md") && len(parts) > 2 {
			deep = append(deep, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(deep)
	sort.Strings(nested)
	return deep, nested, nil
}

// drift answers the first unified-diff lines between one on-disk rule and its
// rendered points.
func drift(stem, want, have string) string {
	var tb textbuf.Buffer
	from := tb.Str("a/").Str(stem).Str(".md (on disk)").String()
	tb.Reset()
	to := tb.Str("b/").Str(stem).Str(".md (rendered)").String()
	lines := unifiedDiff(splitLines(have), splitLines(want), from, to)
	tb.Reset()
	return tb.Str(stem).Str(".md is stale\n").Str(diffHead(lines)).String()
}

// RenderReport is what the two render actions answer.
//
// Written distinguishes a fresh-tree `render-check` from a `render-update` that
// just made the tree fresh.
type RenderReport struct {
	Rules    int      `json:"rules"`
	Written  bool     `json:"written"`
	Failures []string `json:"failures"`
}

// Failed reports whether the render found drift or refused a point tree.
func (r RenderReport) Failed() bool { return len(r.Failures) > 0 }

// Text renders the verdict in the words the script printed.
func (r RenderReport) Text() string {
	var tb textbuf.Buffer
	if len(r.Failures) > 0 {
		for _, line := range r.Failures {
			tb.Str("rules-points: ").Str(line).Byte('\n')
		}
		return tb.Str("rules-points: ").Int(int64(len(r.Failures))).
			Str(" rule(s) are stale; run `./le rules render-update`\n").String()
	}
	verb := "are fresh"
	if r.Written {
		verb = "rendered"
	}
	return tb.Str("rules-points: ").Int(int64(r.Rules)).Str(" rules ").Str(verb).Byte('\n').String()
}

// RenderAll renders each point directory to `ai/rules/<stem>.md`. In check mode,
// it compares the content but does not write.
//
// It fails closed in four directions:
//   - An absent or empty point tree is an error, not a vacuous pass.
//   - A rule without a point directory is an error because nothing renders it.
//   - A generated-artifact point directory is an error because another generator
//     owns its output file.
//   - In check mode, any byte of drift fails instead of causing a silent rewrite.
func RenderAll(tree, rulesDir, pointsDir string, check bool) (RenderReport, error) {
	report := RenderReport{Written: !check}
	var tb textbuf.Buffer

	dirs := pointDirs(pointsDir)
	if len(dirs) == 0 {
		report.Failures = append(report.Failures, tb.Str("no rule point directories under ").
			Str(relTo(tree, pointsDir)).
			Str("; the render has nothing to read and must not report success").String())
		return report, nil
	}
	report.Rules = len(dirs)

	files, err := ruleFiles(rulesDir)
	if err != nil {
		return report, err
	}
	havePoints := map[string]bool{}
	for _, dir := range dirs {
		havePoints[filepath.Base(dir)] = true
	}
	for _, path := range files {
		stem := strings.TrimSuffix(filepath.Base(path), ".md")
		if havePoints[stem] {
			continue
		}
		tb.Reset()
		report.Failures = append(report.Failures, tb.Str(filepath.Base(path)).
			Str(": no point directory at ").Str(relTo(tree, filepath.Join(pointsDir, stem))).
			Str("; every rendered rule must be generated from points").String())
	}

	for _, ruleDir := range dirs {
		stem := filepath.Base(ruleDir)
		// pointDirs accepts any directory carrying a manifest, while ruleFiles
		// excludes an all-caps stem and the two named artifacts. Without this
		// the two disagree, and a `points/CORE/` directory would render over
		// ai/rules/CORE.md, which another generator owns.
		tb.Reset()
		if IsArtifact(tb.Str(stem).Str(".md").String()) {
			tb.Reset()
			report.Failures = append(report.Failures, tb.Str(stem).
				Str(": a point directory may not be named for a generated artifact; rendering it would overwrite ai/rules/").
				Str(stem).Str(".md, which another generator owns. Rename the directory").String())
			continue
		}
		tb.Reset()
		target := filepath.Join(rulesDir, tb.Str(stem).Str(".md").String())
		rendered, err := renderDir(ruleDir)
		if err != nil {
			tb.Reset()
			report.Failures = append(report.Failures, tb.Str(stem).Str(": ").Err(err).String())
			continue
		}
		current, readErr := os.ReadFile(target) // #nosec G304 -- a path derived from the checkout
		if readErr == nil && string(current) == rendered {
			continue
		}
		if check {
			if readErr != nil {
				tb.Reset()
				report.Failures = append(report.Failures, tb.Str(stem).
					Str(".md does not exist but its points do").String())
				continue
			}
			report.Failures = append(report.Failures, drift(stem, rendered, string(current)))
			continue
		}
		if err := os.WriteFile(target, []byte(rendered), 0o600); err != nil {
			return report, err
		}
	}
	return report, nil
}

// RoundTripReport is what the round-trip action answers.
type RoundTripReport struct {
	Rules    int      `json:"rules"`
	Failures []string `json:"failures"`
	// Empty names the population the round trip read nothing from. It is the
	// port's departure from the script, which reports success over a tree with
	// no rule in it at all.
	Empty []string `json:"empty"`
}

// Failed reports whether the round trip found a lossy split or read nothing.
func (r RoundTripReport) Failed() bool { return len(r.Failures) > 0 || len(r.Empty) > 0 }

// Text renders the verdict in the words the script printed.
func (r RoundTripReport) Text() string {
	var tb textbuf.Buffer
	if len(r.Empty) > 0 {
		for _, line := range r.Empty {
			tb.Str("rules-points: ").Str(line).Byte('\n')
		}
		return tb.String()
	}
	if len(r.Failures) > 0 {
		for _, line := range r.Failures {
			tb.Str("rules-points: ").Str(line).Byte('\n')
		}
		return tb.Str("rules-points: ").Int(int64(len(r.Failures))).Str(" of ").
			Int(int64(r.Rules)).Str(" rules do not round-trip\n").String()
	}
	return tb.Str("rules-points: all ").Int(int64(r.Rules)).
		Str(" rules round-trip byte-identical\n").String()
}

// RoundTrip splits and renders each rule in outDir, then compares the bytes.
//
// The split uses the FILESYSTEM, as the script does. formatPoint and parsePoint
// form half the partition. An in-memory round trip would skip them and prove
// only that the splitter agrees with itself.
func RoundTrip(rulesDir, outDir string) (RoundTripReport, error) {
	var report RoundTripReport
	files, err := ruleFiles(rulesDir)
	if err != nil {
		return report, err
	}
	report.Rules = len(files)
	if len(files) == 0 {
		report.Empty = append(report.Empty,
			"no rule file under ai/rules/; the round trip read nothing and must not report success")
		return report, nil
	}

	var tb textbuf.Buffer
	for _, path := range files {
		name := filepath.Base(path)
		stem := strings.TrimSuffix(name, ".md")
		raw, err := os.ReadFile(path) // #nosec G304 -- a path derived from the checkout
		if err != nil {
			return report, err
		}
		source := string(raw)

		rendered, err := roundTripOne(source, stem, outDir)
		if err != nil {
			tb.Reset()
			report.Failures = append(report.Failures, tb.Str(name).Str(": ").
				Str(pointsError).Str(": ").Err(err).String())
			continue
		}
		if rendered == source {
			continue
		}
		tb.Reset()
		from := tb.Str("a/").Str(name).String()
		tb.Reset()
		to := tb.Str("b/").Str(name).String()
		lines := unifiedDiff(splitLines(source), splitLines(rendered), from, to)
		tb.Reset()
		report.Failures = append(report.Failures, tb.Str(name).
			Str(": round trip is not byte-identical\n").Str(diffHead(lines)).String())
	}
	return report, nil
}

// roundTripOne splits one rule into outDir and renders it straight back.
func roundTripOne(source, stem, outDir string) (string, error) {
	split, err := splitRule(source, stem)
	if err != nil {
		return "", err
	}
	if err := writeSplit(split, outDir); err != nil {
		return "", err
	}
	return renderDir(filepath.Join(outDir, stem))
}
