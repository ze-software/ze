// Design: docs/architecture/core-design.md -- which point each hook check enforces
// Overview: points.go -- the point files this join reads
// Detail: coverage_report.go -- the answer this join is rendered as
// Detail: hooktable.go -- the published claim the join is compared against
//
// coverage.go implements the gate-map half of scripts/dev/rules_points.py. It
// joins PreToolUse `# ze point:` comments to the points on disk.
//
// The join produces five sets. GATED and UNGATED are measurements. An ungated
// point is a rule that no machine enforces. DANGLING fails because its check
// names a missing point. REGRESSED and DECLARED NONE fail because each moves a
// point from gated to ungated while every other gate stays green. SHRUNK fails
// because deletion of a point and its manifest line makes the corpus agree on a
// smaller set.
//
// An EMPTY result is never a pass. No point, or no binding at all, means the
// join read nothing and must say so (ai/rules/evidence.md).

package rules

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The dispatcher roster comes from settings.json. A fixed list can shrink with
// no failure. Removal of one dispatcher once retired seven checks from the join
// and the published table. Every gate stayed green. settings.json determines
// which dispatchers RUN, so it is the only valid source.
const (
	settingsRel    = ".claude/settings.json"
	hookDir        = ".claude/hooks"
	dispatcherGlob = "pretool-"
)

// noCheck names a binding that the join cannot attribute to a running check. The
// binding remains in the report because it claims to gate a point.
const noCheck = "<module>"

// emptyRef stands in for a `# ze point:` comment with no payload, so a bare
// marker lands in the dangling set instead of matching nothing and vanishing.
const emptyRef = "!empty"

// retiredFile is the ledger a point's removal is declared in.
const retiredFile = "RETIRED.md"

// retiredTableHead is the ledger's table header, excluded from its rows.
const retiredTableHead = "| Point | Why |"

// gitTimeout bounds one git call. Each reads an object or lists a tree, which
// is milliseconds, so a run past this is a wedged repository rather than a slow
// one.
const gitTimeout = 60 * time.Second

var (
	// `# ze point: <rule>/<section>/<slug>` on a line of its own, directly
	// above the check it binds. The payload is captured WHOLE rather than as
	// one bare token, so a typo lands in the dangling set instead of matching
	// nothing and vanishing.
	bindingLine = regexp.MustCompile(`^[ \t\n\r\f\v]*#[ \t\n\r\f\v]*ze point:(.*)$`)
	// `# ze point: none -- <why>` declares that a check enforces no written
	// point. The reason is REQUIRED: without it, "nobody bound this yet" and
	// "there is nothing to bind" would look the same.
	noPointLine  = regexp.MustCompile(`^none[ \t\n\r\f\v]+--[ \t\n\r\f\v]+(\S.*)$`)
	topLevelDef  = regexp.MustCompile(`^def[ \t\n\r\f\v]+([A-Za-z_]\w*)[ \t\n\r\f\v]*\(`)
	retirementRe = regexp.MustCompile(
		"^\\|[ \t\n\r\f\v]*`([a-z0-9-]+/[a-z0-9-]+/[a-z0-9-]+)`[ \t\n\r\f\v]*\\|[ \t\n\r\f\v]*([^|]*\\S)[ \t\n\r\f\v]*\\|[ \t\n\r\f\v]*$")
)

// structuralKinds are excluded from the ungated DENOMINATOR, so the number
// means "points stating something nothing gates" rather than "markdown blocks".
var structuralKinds = [...]string{kindHeading, kindFence}

// Binding is one `# ze point:` comment: where it is, what carries it, what it
// names.
type Binding struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Check string `json:"check"`
	// Ref is the point id, or "" when the check declares it binds none.
	Ref string `json:"ref"`
	// Reason is why the check binds no point. It is empty for a real ref.
	Reason string `json:"reason,omitempty"`
}

// GateMap is the join between the bindings in the dispatchers and the points on
// disk.
type GateMap struct {
	// Points maps every point id on disk to its kind.
	Points   map[string]string
	Bindings []Binding
	// Gated maps a point id to the bindings naming it.
	Gated map[string][]Binding
	// Ungated names the instruction points no binding names.
	Ungated []string
	// Dangling holds a binding naming a point that does not exist, or sitting
	// above no check.
	Dangling []Binding
	// Unbound holds a check declaring `none -- <why>`.
	Unbound []Binding
	// Rationales maps a point id to the repo-relative path its `rationale:`
	// names. Only the points that declare one appear.
	Rationales map[string]string
	// MissingRationale pairs a point id with a rationale path absent from disk.
	MissingRationale [][2]string
	// Excepted maps a point id to the ids its `excepted-by` names.
	Excepted map[string][]string
	// MissingException pairs a point id with an `excepted-by` target that is
	// not on disk, or that is the declaring point itself.
	MissingException [][2]string
}

// candidates answers the ungated denominator: every point that is not
// structural.
func (g GateMap) candidates() []string {
	var out []string
	for ref, kind := range g.Points {
		if !slices.Contains(structuralKinds[:], kind) {
			out = append(out, ref)
		}
	}
	sort.Strings(out)
	return out
}

// gatedBindings answers every binding that resolved to a point.
func (g GateMap) gatedBindings() []Binding {
	var out []Binding
	for _, ref := range sortedKeys(g.Gated) {
		out = append(out, g.Gated[ref]...)
	}
	return out
}

// parseBindings answers every binding comment in one dispatcher, attributed to
// the check below it.
//
// A binding binds only the NEXT top-level `def`. Only blank lines and comments
// can occur between them. Other content breaks the binding, so the report
// attributes it to noCheck instead of dropping it.
//
// A payload must contain a point id or `none -- <why>`. The report keeps an
// invalid payload as its spelled ref. No point matches it, so it fails as
// dangling. Dropping it would let a typo remove a check without a failure.
func parseBindings(text, path string) []Binding {
	type pending struct {
		line   int
		ref    string
		reason string
	}
	var waiting []pending
	var out []Binding

	flush := func(check string) {
		for _, p := range waiting {
			out = append(out, Binding{File: path, Line: p.line, Check: check, Ref: p.ref, Reason: p.reason})
		}
		waiting = nil
	}

	for i, line := range strings.Split(text, "\n") {
		if found := bindingLine.FindStringSubmatch(line); found != nil {
			payload := strings.TrimSpace(found[1])
			if declared := noPointLine.FindStringSubmatch(payload); declared != nil {
				waiting = append(waiting, pending{line: i + 1, reason: declared[1]})
				continue
			}
			if payload == "" {
				payload = emptyRef
			}
			waiting = append(waiting, pending{line: i + 1, ref: payload})
			continue
		}
		if len(waiting) == 0 {
			continue
		}
		if found := topLevelDef.FindStringSubmatch(line); found != nil {
			flush(found[1])
			continue
		}
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			flush(noCheck)
		}
	}
	flush(noCheck)
	return out
}

// pointsOnDisk answers every point's `<rule>/<section>/<slug>` id mapped to the
// point it names.
//
// It reads FILES instead of manifests because a binding names the path id. A
// malformed point is an error. Skipping it would remove it from both measured
// sets and make the problem appear smaller.
func pointsOnDisk(pointsDir string) (map[string]Point, error) {
	out := map[string]Point{}
	var tb textbuf.Buffer
	for _, ruleDir := range pointDirs(pointsDir) {
		sections, err := os.ReadDir(ruleDir)
		if err != nil {
			return nil, err
		}
		for _, section := range sections {
			if !section.IsDir() {
				continue
			}
			sectionDir := filepath.Join(ruleDir, section.Name())
			files, err := os.ReadDir(sectionDir)
			if err != nil {
				return nil, err
			}
			for _, file := range files {
				if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
					continue
				}
				raw, err := os.ReadFile(filepath.Join(sectionDir, file.Name())) // #nosec G304 -- a path derived from the checkout
				if err != nil {
					return nil, err
				}
				slug := strings.TrimSuffix(file.Name(), ".md")
				point, err := ParsePoint(string(raw), slug)
				if err != nil {
					return nil, err
				}
				tb.Reset()
				out[tb.Str(filepath.Base(ruleDir)).Byte('/').Str(section.Name()).
					Byte('/').Str(slug).String()] = point
			}
		}
	}
	return out, nil
}

// dispatchers answers every PreToolUse Python dispatcher, and the problems
// found deriving them.
//
// It fails closed in both directions. Each problem removes a check from the
// join while the report still says "no dangling bindings". A settings.json
// command without a file removes all bindings in that file. An unregistered
// `pretool-*.py` file contains checks that never run.
//
// An unreadable settings.json is a failure, not an empty roster. An empty roster
// incorrectly claims that no dispatchers exist.
func dispatchers(root string) ([]string, []string) {
	var tb textbuf.Buffer
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(settingsRel))) // #nosec G304 -- a path derived from the checkout
	if err != nil {
		return nil, []string{tb.Str(settingsRel).Str(": cannot be read (").Err(err).
			Str("); the dispatcher roster is unknown").String()}
	}
	// These fields need no JSON tags. Their names match Claude Code keys
	// case-insensitively. A tag would use Ze's kebab-case convention and stop
	// matching the external schema.
	var settings struct {
		Hooks struct {
			PreToolUse []struct {
				Hooks []struct{ Command string }
			}
		}
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, []string{tb.Str(settingsRel).Str(": cannot be read (").Err(err).
			Str("); the dispatcher roster is unknown").String()}
	}

	var problems, registered []string
	for _, entry := range settings.Hooks.PreToolUse {
		for _, hook := range entry.Hooks {
			name := hook.Command
			if at := strings.LastIndex(name, "/"); at >= 0 {
				name = name[at+1:]
			}
			tb.Reset()
			if !strings.HasSuffix(name, ".py") ||
				!strings.Contains(hook.Command, tb.Str(hookDir).Byte('/').Str(name).String()) {
				continue
			}
			if !slices.Contains(registered, name) {
				registered = append(registered, name)
			}
		}
	}
	if len(registered) == 0 {
		tb.Reset()
		problems = append(problems, tb.Str(settingsRel).Str(": no PreToolUse entry runs a ").
			Str(hookDir).Str("/*.py dispatcher; the gate map would read nothing and must not report success").String())
	}

	sorted := slices.Sorted(slices.Values(registered))
	var paths []string
	for _, name := range sorted {
		path := filepath.Join(root, filepath.FromSlash(hookDir), name)
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			tb.Reset()
			problems = append(problems, tb.Str(settingsRel).Str(": PreToolUse runs ").
				Str(hookDir).Byte('/').Str(name).
				Str(", which does not exist; every binding in it is out of the join").String())
			continue
		}
		paths = append(paths, path)
	}

	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(hookDir)))
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasPrefix(name, dispatcherGlob) || !strings.HasSuffix(name, ".py") {
				continue
			}
			if slices.Contains(registered, name) {
				continue
			}
			tb.Reset()
			problems = append(problems, tb.Str(hookDir).Byte('/').Str(name).
				Str(": no PreToolUse entry in ").Str(settingsRel).
				Str(" runs it, so its checks never fire; wire it up or delete it").String())
		}
	}
	return paths, problems
}

// gitOutput runs one git command in root and answers its stdout and whether it
// succeeded. A git that cannot run at all is answered as ok=false, which every
// caller reads apart from a command that ran and failed.
func gitOutput(root string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- a fixed verb and refs this tool built
	cmd.Dir = root
	out, err := cmd.Output()
	return string(out), err == nil
}

// headSources answers each dispatcher's text at git HEAD, and the ones HEAD
// does not carry.
//
// ok=false means that git cannot answer. It differs from an empty mapping. The
// function probes HEAD once to preserve this distinction. Otherwise, a missing
// commit, an unreadable checkout, and a renamed dispatcher all produce an empty
// map. The caller would then infer a baseline that contains nothing. A disabled
// ratchet prints the same zero as an active ratchet.
//
// The report names a file that is absent at HEAD. The change either adds a
// dispatcher with no baseline or renames one and removes its baseline bindings.
func headSources(root string, names []string) (map[string]string, []string, bool) {
	if _, ok := gitOutput(root, "rev-parse", "--verify", "HEAD"); !ok {
		return nil, nil, false
	}
	out := map[string]string{}
	var absent []string
	var tb textbuf.Buffer
	for _, name := range names {
		tb.Reset()
		text, ok := gitOutput(root, "show", tb.Str("HEAD:").Str(hookDir).Byte('/').Str(name).String())
		if ok {
			out[name] = text
			continue
		}
		absent = append(absent, name)
	}
	return out, absent, true
}

// bindingsAtHead answers each check's point ids at HEAD, for the checks that
// named one.
//
// It is keyed by CHECK rather than flattened to a ref set, because a rename
// moves a ref and leaves the check where it was. The ref set alone cannot tell
// a renamed point from a deleted one, and unboundRegressions needs to.
func bindingsAtHead(sources map[string]string) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, name := range sortedKeys(sources) {
		for _, binding := range parseBindings(sources[name], name) {
			if binding.Ref == "" || binding.Ref == emptyRef || binding.Check == noCheck {
				continue
			}
			if out[binding.Check] == nil {
				out[binding.Check] = map[string]bool{}
			}
			out[binding.Check][binding.Ref] = true
		}
	}
	return out
}

// gatedRegressions answers the points that were gated at HEAD and that no
// binding names now.
//
// The gated set is MONOTONIC. The published-table check catches deletion of a
// `# ze point:` comment. But deletion of both the comment and its backticked
// table stem leaves both sources empty. The point becomes ungated while every
// gate exits 0. This is the shortest route from red to green, and path ids exist
// to prevent it.
func gatedRegressions(gm GateMap, baseline map[string]map[string]bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, refs := range baseline {
		for ref := range refs {
			if seen[ref] {
				continue
			}
			seen[ref] = true
			if _, onDisk := gm.Points[ref]; !onDisk {
				continue
			}
			if _, gated := gm.Gated[ref]; !gated {
				out = append(out, ref)
			}
		}
	}
	sort.Strings(out)
	return out
}

// unboundRegressions answers the checks that named a point at HEAD and declare
// `none -- <why>` now.
//
// This is the laundering route that gatedRegressions cannot see. A renamed point
// makes its binding dangle and fail. Replacing that binding with
// `# ze point: none -- <why>` puts it in UNBOUND, which does not fail.
//
// retired contains ids that retiredRowsSince VALIDATED as declared since HEAD.
// Such a ref is not a regression. Retirement, declaration, and check relabeling
// are the normal route out of the corpus. A PARTIAL declaration fails closed.
func unboundRegressions(gm GateMap, baseline map[string]map[string]bool, retired map[string]bool) []string {
	now := map[string]bool{}
	for _, binding := range gm.gatedBindings() {
		now[binding.Check] = true
	}
	declared := map[string]bool{}
	for _, binding := range gm.Unbound {
		declared[binding.Check] = true
	}

	var out []string
	var tb textbuf.Buffer
	for _, check := range sortedKeys(baseline) {
		if !declared[check] || now[check] {
			continue
		}
		var live []string
		for ref := range baseline[check] {
			if !retired[ref] {
				live = append(live, ref)
			}
		}
		if len(live) == 0 {
			continue
		}
		sort.Strings(live)
		tb.Reset()
		out = append(out, tb.Str(check).Str(": named ").Join(live, ", ").
			Str(" at HEAD, declares `none` now").String())
	}
	sort.Strings(out)
	return out
}

// retirementRows answers the ledger's table rows, its header and separator
// excluded. The rest of the file is prose explaining the mechanism, so a row is
// recognized by its markdown table shape rather than by position.
func retirementRows(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || line == retiredTableHead {
			continue
		}
		if strings.Trim(line, "|-: ") == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// retiredRowsSince answers the ids declared retired since HEAD, and the rows
// that declare nothing.
//
// The function returns IDS, not a count. A count cannot identify invalid
// substitutions. An edit can delete a real point and declare a nonexistent id.
// An edit can also change a committed row's Why text to permit a second deletion.
//
// Four shapes declare nothing and are REFUSED:
//
// - A row lacks a `<rule>/<section>/<slug>` and a reason.
// - A row names an id that was absent at git HEAD.
// - A row names a point that remains on disk.
// - A row duplicates an id in this file.
//
// Two shapes declare nothing and are SKIPPED. HEAD already contains each
// declaration, so another run must not reject a committed line. These shapes
// are an unchanged row and an id that HEAD already declares. They keep the
// ledger a scope instead of an allowlist.
func retiredRowsSince(nowText, wasText string, headIDs map[string]bool, haveHeadIDs bool,
	nowIDs map[string]bool) (map[string]bool, []string) {
	var tb textbuf.Buffer
	where := tb.Str(pointsRel).Byte('/').Str(retiredFile).String()

	unchanged := map[string]bool{}
	atHead := map[string]bool{}
	for _, row := range retirementRows(wasText) {
		unchanged[row] = true
		if found := retirementRe.FindStringSubmatch(row); found != nil {
			atHead[found[1]] = true
		}
	}

	declared := map[string]bool{}
	var problems []string
	for _, row := range retirementRows(nowText) {
		if unchanged[row] {
			continue
		}
		found := retirementRe.FindStringSubmatch(row)
		if found == nil {
			tb.Reset()
			problems = append(problems, tb.Str(where).Str(": ").Str(pyRepr(row)).
				Str(" is not '<rule>/<section>/<slug> -- <why>'").String())
			continue
		}
		ref := found[1]
		switch {
		case atHead[ref]:
		case declared[ref]:
			tb.Reset()
			problems = append(problems, tb.Str(where).Str(": `").Str(ref).
				Str("` is declared twice; one row retires one point").String())
		case haveHeadIDs && !headIDs[ref]:
			tb.Reset()
			problems = append(problems, tb.Str(where).Str(": `").Str(ref).
				Str("` names no point at HEAD; a retirement declares an instruction that left the corpus, and this id was never in it").String())
		case nowIDs[ref]:
			tb.Reset()
			problems = append(problems, tb.Str(where).Str(": `").Str(ref).
				Str("` is still on disk; a retirement declares an instruction that left, and this one has not").String())
		default:
			declared[ref] = true
		}
	}
	return declared, problems
}

// retiredSinceHead runs retiredRowsSince over the ledger on disk and the ledger
// at HEAD.
func retiredSinceHead(root, pointsDir string, headIDs map[string]bool, haveHeadIDs bool,
	nowIDs map[string]bool) (map[string]bool, []string) {
	nowText := ""
	if raw, err := os.ReadFile(filepath.Join(pointsDir, retiredFile)); err == nil { // #nosec G304 -- a path derived from the checkout
		nowText = string(raw)
	}
	var tb textbuf.Buffer
	wasText, _ := gitOutput(root, "show", tb.Str("HEAD:").Str(retiredRel).String())
	return retiredRowsSince(nowText, wasText, headIDs, haveHeadIDs, nowIDs)
}

// headPointIDs answers every point id at git HEAD and whether git answered.
//
// It reads file names at a fixed depth of two. Thus, manifests, the ledger, and
// deeper files are not points. Retirement rows are checked against ids, so this
// function returns names instead of only a count.
func headPointIDs(root string) (map[string]bool, bool) {
	if _, ok := gitOutput(root, "rev-parse", "--verify", "HEAD"); !ok {
		return nil, false
	}
	listed, ok := gitOutput(root, "ls-tree", "-r", "--name-only", "HEAD", pointsRel)
	if !ok {
		return nil, false
	}
	out := map[string]bool{}
	var tb textbuf.Buffer
	for line := range strings.SplitSeq(listed, "\n") {
		parts := strings.Split(strings.TrimSpace(line), "/")
		if len(parts) != 6 || parts[0] != "ai" || parts[1] != "rules" || parts[2] != "points" {
			continue
		}
		if !strings.HasSuffix(parts[5], ".md") {
			continue
		}
		tb.Reset()
		out[tb.Str(parts[3]).Byte('/').Str(parts[4]).Byte('/').
			Str(strings.TrimSuffix(parts[5], ".md")).String()] = true
	}
	return out, true
}

// corpusShrink answers the point ids git HEAD carried that are gone from disk
// and undeclared.
//
// IDENTITY, never a count. An addition masks a count because the rule retains
// its point total. A specific instruction still leaves. On 2026-08-09, 17
// points were deleted and 6 were declared. The count reported ZERO affected
// rules while four rules each lost a point behind an addition.
//
// A RENAME therefore retires the old id. The count form cannot detect one. This
// form reports it, so the ledger records the destination for readers of the old
// id.
func corpusShrink(headIDs, nowIDs, declared map[string]bool) []string {
	var vanished []string
	for ref := range headIDs {
		if nowIDs[ref] || declared[ref] {
			continue
		}
		vanished = append(vanished, ref)
	}
	sort.Strings(vanished)

	byRule := map[string][]string{}
	for _, ref := range vanished {
		rule, _, _ := strings.Cut(ref, "/")
		byRule[rule] = append(byRule[rule], ref)
	}

	out := make([]string, 0, len(byRule))
	var tb textbuf.Buffer
	for _, rule := range sortedKeys(byRule) {
		refs := byRule[rule]
		tb.Reset()
		out = append(out, tb.Str(rule).Str(": ").Int(int64(len(refs))).
			Str(" vanished since HEAD with no ").Str(retiredFile).Str(" row: ").
			Join(refs, ", ").String())
	}
	return out
}

// rationaleProblems answers the declared rationale links, and the ones naming
// no RECORD.
//
// A rationale is a repo-relative path. Resolve it from the repository root, not
// from the point directory. Report paths outside the tree as missing. Rationale
// links can name only records in this repository, where another run can check
// them.
//
// An EMPTY file is missing too. The field claims a record explains the
// instruction, and touching a file satisfied a check that asked only whether a
// path resolved. That is a claim with a file behind it rather than a record.
func rationaleProblems(points map[string]Point, root string) (map[string]string, [][2]string) {
	declared := map[string]string{}
	var missing [][2]string
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		resolvedRoot = root
	}
	for _, ref := range sortedKeys(points) {
		point := points[ref]
		if point.Rationale == "" {
			continue
		}
		declared[ref] = point.Rationale
		target := filepath.Clean(filepath.Join(resolvedRoot, filepath.FromSlash(point.Rationale)))
		inside := target == resolvedRoot ||
			strings.HasPrefix(target, resolvedRoot+string(filepath.Separator))
		if !inside || !hasContent(target) {
			missing = append(missing, [2]string{ref, point.Rationale})
		}
	}
	return declared, missing
}

// hasContent reports whether a path is a file holding anything but whitespace.
// Unreadable counts as not.
func hasContent(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- a path a point declared and this walk confined to the checkout
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(raw)) != ""
}

// exceptionProblems answers the declared exception links, and the ones naming
// no point.
//
// Two shapes fail. First, a ref that no point on disk carries leaves the general
// point's link without a target. A missing target lets a dedup pass remove a
// guard while every gate stays green.
//
// Second, a point naming ITSELF fails because a point cannot make an exception
// to its own statement. Without this check, the self-reference would resolve
// and look like a declared relationship.
func exceptionProblems(points map[string]Point) (map[string][]string, [][2]string) {
	declared := map[string][]string{}
	var missing [][2]string
	for _, ref := range sortedKeys(points) {
		point := points[ref]
		if point.ExceptedBy == "" {
			continue
		}
		named := ExceptionRefs(point.ExceptedBy)
		if len(named) == 0 {
			missing = append(missing, [2]string{ref, point.ExceptedBy})
			continue
		}
		declared[ref] = named
		for _, target := range named {
			if _, ok := points[target]; target == ref || !ok {
				missing = append(missing, [2]string{ref, target})
			}
		}
	}
	return declared, missing
}

// buildGateMap joins the dispatchers' binding comments against the points on
// disk. root is where a point's `rationale:` path is resolved from.
func buildGateMap(gateFiles []string, pointsDir, root string) (GateMap, error) {
	onDisk, err := pointsOnDisk(pointsDir)
	if err != nil {
		return GateMap{}, err
	}
	rationales, missingRationale := rationaleProblems(onDisk, root)
	excepted, missingException := exceptionProblems(onDisk)

	points := make(map[string]string, len(onDisk))
	for ref, point := range onDisk {
		points[ref] = point.Kind
	}

	var bindings []Binding
	for _, path := range gateFiles {
		raw, err := os.ReadFile(path) // #nosec G304 -- a dispatcher path settings.json named
		if err != nil {
			return GateMap{}, err
		}
		bindings = append(bindings, parseBindings(string(raw), filepath.Base(path))...)
	}

	gated := map[string][]Binding{}
	var dangling, unbound []Binding
	for _, binding := range bindings {
		switch {
		case binding.Check == noCheck:
			dangling = append(dangling, binding)
		case binding.Ref == "":
			unbound = append(unbound, binding)
		default:
			if _, ok := points[binding.Ref]; ok {
				gated[binding.Ref] = append(gated[binding.Ref], binding)
				continue
			}
			dangling = append(dangling, binding)
		}
	}

	var ungated []string
	for ref, kind := range points {
		if _, isGated := gated[ref]; isGated || slices.Contains(structuralKinds[:], kind) {
			continue
		}
		ungated = append(ungated, ref)
	}
	sort.Strings(ungated)

	return GateMap{
		Points: points, Bindings: bindings, Gated: gated, Ungated: ungated,
		Dangling: dangling, Unbound: unbound,
		Rationales: rationales, MissingRationale: missingRationale,
		Excepted: excepted, MissingException: missingException,
	}, nil
}

// sortedKeys answers a map's keys in order, so every walk over a map is
// deterministic.
func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
