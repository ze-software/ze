// Design: docs/architecture/core-design.md -- which point each hook check enforces
// Overview: points.go -- the point files this join reads
// Detail: coverage_report.go -- the answer this join is rendered as
// Detail: hooktable.go -- the published claim the join is compared against
//
// coverage.go implements the gate-map half of internal/le/rules/points.go. It
// joins native Go `// ze point:` comments to the points on disk.
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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// hookRuntimeRel is the package whose native action registry runs the checks
// measured by this join.
const (
	hookRuntimeRel = "internal/le/hookruntime"
	hookRegistry   = "nativeHookActions"
)

// noCheck names a binding comment not attached to a top-level Go function. It
// remains visible because it still claims to gate a point.
const noCheck = "<source>"

// emptyRef stands in for a `// ze point:` comment with no payload.
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
	// `// ze point: <rule>/<section>/<slug>` on a line of its own in a Go
	// function's doc comment. The payload is captured whole so a typo becomes
	// dangling instead of disappearing.
	bindingLine = regexp.MustCompile(`^[ \t]*//[ \t]*ze point:(.*)$`)
	// `// ze point: none -- <why>` declares that a registered native check
	// enforces no written point. The reason distinguishes an intentional
	// exception from a forgotten binding.
	noPointLine  = regexp.MustCompile(`^none[ \t\n\r\f\v]+--[ \t\n\r\f\v]+(\S.*)$`)
	retirementRe = regexp.MustCompile(
		"^\\|[ \t\n\r\f\v]*`([a-z0-9-]+/[a-z0-9-]+/[a-z0-9-]+)`[ \t\n\r\f\v]*\\|[ \t\n\r\f\v]*([^|]*\\S)[ \t\n\r\f\v]*\\|[ \t\n\r\f\v]*$")
)

// structuralKinds are excluded from the ungated DENOMINATOR, so the number
// means "points stating something nothing gates" rather than "markdown blocks".
var structuralKinds = [...]string{kindHeading, kindFence}

// Binding is one native Go `// ze point:` comment: where it is, what carries
// it, and what it names.
type Binding struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Check string `json:"check"`
	// Ref is the point id, or "" when the check declares it binds none.
	Ref string `json:"ref"`
	// Reason is why the check binds no point. It is empty for a real ref.
	Reason string `json:"reason,omitempty"`
}

// gateMap is the join between native registered-check bindings and points.
type gateMap struct {
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
func (g gateMap) candidates() []string {
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
func (g gateMap) gatedBindings() []Binding {
	var out []Binding
	for _, ref := range sortedKeys(g.Gated) {
		out = append(out, g.Gated[ref]...)
	}
	return out
}

type hookFile struct {
	bindings  []Binding
	functions map[string]int
	registry  map[string][]string
}

// parseHookFile reads Go syntax, top-level functions, binding comments, and the
// native action registry when this is the file that declares it.
func parseHookFile(text, path string) (hookFile, error) {
	files := token.NewFileSet()
	tree, err := parser.ParseFile(files, path, text, parser.ParseComments)
	if err != nil {
		return hookFile{}, err
	}
	out := hookFile{functions: map[string]int{}}
	attached := map[*ast.CommentGroup]bool{}
	for _, declaration := range tree.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil {
			continue
		}
		out.functions[function.Name.Name] = files.Position(function.Pos()).Line
		if function.Doc == nil {
			continue
		}
		attached[function.Doc] = true
		for _, comment := range function.Doc.List {
			if binding, ok := bindingComment(comment.Text, path,
				files.Position(comment.Slash).Line, function.Name.Name); ok {
				out.bindings = append(out.bindings, binding)
			}
		}
	}
	for _, group := range tree.Comments {
		if attached[group] {
			continue
		}
		for _, comment := range group.List {
			if binding, ok := bindingComment(comment.Text, path,
				files.Position(comment.Slash).Line, noCheck); ok {
				out.bindings = append(out.bindings, binding)
			}
		}
	}
	out.registry, err = parseHookRegistry(tree)
	return out, err
}

func bindingComment(text, path string, line int, check string) (Binding, bool) {
	found := bindingLine.FindStringSubmatch(text)
	if found == nil {
		return Binding{}, false
	}
	payload := strings.TrimSpace(found[1])
	if declared := noPointLine.FindStringSubmatch(payload); declared != nil {
		return Binding{File: path, Line: line, Check: check, Reason: declared[1]}, true
	}
	if payload == "" {
		payload = emptyRef
	}
	return Binding{File: path, Line: line, Check: check, Ref: payload}, true
}

// parseHookRegistry answers the action-to-check function names from the
// composite literal used by hookruntime.Run. A malformed registry is an error,
// never a shortened roster.
func parseHookRegistry(tree *ast.File) (map[string][]string, error) {
	var literal *ast.CompositeLit
	for _, declaration := range tree.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.VAR {
			continue
		}
		for _, raw := range generic.Specs {
			spec, ok := raw.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range spec.Names {
				if name.Name != hookRegistry {
					continue
				}
				if literal != nil || index >= len(spec.Values) {
					return nil, fmt.Errorf("%s must be declared exactly once", hookRegistry)
				}
				literal, ok = spec.Values[index].(*ast.CompositeLit)
				if !ok {
					return nil, fmt.Errorf("%s must be a composite literal", hookRegistry)
				}
			}
		}
	}
	if literal == nil {
		return nil, nil
	}

	actions := map[string][]string{}
	for _, raw := range literal.Elts {
		entry, ok := raw.(*ast.KeyValueExpr)
		if !ok {
			return nil, fmt.Errorf("%s contains a non-keyed action", hookRegistry)
		}
		key, ok := entry.Key.(*ast.BasicLit)
		if !ok || key.Kind != token.STRING {
			return nil, fmt.Errorf("%s contains a non-string action name", hookRegistry)
		}
		action, err := strconv.Unquote(key.Value)
		if err != nil || action == "" {
			return nil, fmt.Errorf("%s contains an invalid action name", hookRegistry)
		}
		body, ok := entry.Value.(*ast.CompositeLit)
		if !ok {
			return nil, fmt.Errorf("%s action %q is not a composite literal", hookRegistry, action)
		}
		if _, duplicate := actions[action]; duplicate {
			return nil, fmt.Errorf("%s declares action %q twice", hookRegistry, action)
		}
		var checks []string
		for _, rawField := range body.Elts {
			field, ok := rawField.(*ast.KeyValueExpr)
			name, named := field.Key.(*ast.Ident)
			if !ok || !named || name.Name != "checks" {
				continue
			}
			list, ok := field.Value.(*ast.CompositeLit)
			if !ok {
				return nil, fmt.Errorf("%s action %q has a non-literal checks field", hookRegistry, action)
			}
			for _, rawCheck := range list.Elts {
				check, ok := rawCheck.(*ast.Ident)
				if !ok {
					return nil, fmt.Errorf("%s action %q contains a non-function check", hookRegistry, action)
				}
				checks = append(checks, check.Name)
			}
		}
		if len(checks) == 0 {
			return nil, fmt.Errorf("%s action %q runs no checks", hookRegistry, action)
		}
		actions[action] = checks
	}
	if len(actions) == 0 {
		return nil, fmt.Errorf("%s runs no actions", hookRegistry)
	}
	return actions, nil
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
				point, err := parsePoint(string(raw), slug)
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

// nativeHookSources answers the Go files in hookruntime and refuses any
// disagreement between their top-level functions, binding comments, and the
// registry that hookruntime.Run executes.
func nativeHookSources(root string) (map[string]string, []string) {
	directory := filepath.Join(root, filepath.FromSlash(hookRuntimeRel))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: cannot be read (%v); the native hook roster is unknown",
			hookRuntimeRel, err)}
	}

	sources := map[string]string{}
	parsed := map[string]hookFile{}
	var problems []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(directory, name)) // #nosec G304 -- entry came from the fixed hookruntime directory
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s/%s: cannot be read: %v", hookRuntimeRel, name, err))
			continue
		}
		sources[name] = string(body)
		file, err := parseHookFile(string(body), name)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s/%s: cannot be parsed: %v", hookRuntimeRel, name, err))
			continue
		}
		parsed[name] = file
	}
	if len(sources) == 0 {
		problems = append(problems, hookRuntimeRel+": no Go sources; the gate map would read nothing")
		return sources, problems
	}

	functions := map[string]string{}
	var registry map[string][]string
	for _, name := range sortedKeys(parsed) {
		file := parsed[name]
		for function := range file.functions {
			if previous := functions[function]; previous != "" {
				problems = append(problems, fmt.Sprintf("%s: top-level function %s is also declared in %s",
					name, function, previous))
				continue
			}
			functions[function] = name
		}
		if file.registry == nil {
			continue
		}
		if registry != nil {
			problems = append(problems, hookRegistry+" is declared in more than one Go file")
			continue
		}
		registry = file.registry
	}
	if registry == nil {
		problems = append(problems, hookRegistry+": native hook action registry is missing")
		return sources, problems
	}

	registered := map[string]string{}
	for _, action := range sortedKeys(registry) {
		for _, check := range registry[action] {
			if previous := registered[check]; previous != "" {
				problems = append(problems, fmt.Sprintf("%s: check %s is also wired to %s", action, check, previous))
				continue
			}
			registered[check] = action
			if functions[check] == "" {
				problems = append(problems, fmt.Sprintf("%s: check %s names no top-level hookruntime function", action, check))
			}
		}
	}

	bound := map[string]bool{}
	for _, name := range sortedKeys(parsed) {
		for _, binding := range parsed[name].bindings {
			if binding.Check == noCheck {
				continue
			}
			bound[binding.Check] = true
			if registered[binding.Check] == "" {
				problems = append(problems, fmt.Sprintf("%s:%d: binding on unwired check %s",
					name, binding.Line, binding.Check))
			}
		}
	}
	for _, check := range sortedKeys(registered) {
		if !bound[check] {
			problems = append(problems, fmt.Sprintf("%s: registered check %s has no `// ze point:` binding",
				registered[check], check))
		}
	}
	return sources, problems
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

// headSources answers each current hookruntime Go source at git HEAD, and the
// source names HEAD does not carry.
//
// ok=false means git could not answer at all. Missing individual files stay
// visible in absent rather than silently shortening the baseline.
func headSources(root string, names []string) (map[string]string, []string, bool) {
	if _, ok := gitOutput(root, "rev-parse", "--verify", "HEAD"); !ok {
		return nil, nil, false
	}
	out := map[string]string{}
	var absent []string
	var tb textbuf.Buffer
	for _, name := range names {
		tb.Reset()
		text, ok := gitOutput(root, "show", tb.Str("HEAD:").Str(hookRuntimeRel).Byte('/').Str(name).String())
		if ok {
			out[name] = text
			continue
		}
		absent = append(absent, name)
	}
	return out, absent, true
}

// bindingsAtHead answers each Go check's point ids at HEAD.
func bindingsAtHead(sources map[string]string) (map[string]map[string]bool, error) {
	out := map[string]map[string]bool{}
	for _, name := range sortedKeys(sources) {
		file, err := parseHookFile(sources[name], name)
		if err != nil {
			return nil, err
		}
		for _, binding := range file.bindings {
			if binding.Ref == "" || binding.Ref == emptyRef || binding.Check == noCheck {
				continue
			}
			if out[binding.Check] == nil {
				out[binding.Check] = map[string]bool{}
			}
			out[binding.Check][binding.Ref] = true
		}
	}
	return out, nil
}

// gatedRegressions answers the points that were gated at HEAD and that no
// binding names now.
//
// The gated set is MONOTONIC. The published-table check catches deletion of a
// `// ze point:` comment. But deletion of both the comment and its backticked
// table stem leaves both sources empty. The point becomes ungated while every
// gate exits 0. This is the shortest route from red to green, and path ids exist
// to prevent it.
func gatedRegressions(gm gateMap, baseline map[string]map[string]bool) []string {
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
// `// ze point: none -- <why>` puts it in UNBOUND, which does not fail.
//
// retired contains ids that retiredRowsSince VALIDATED as declared since HEAD.
// Such a ref is not a regression. Retirement, declaration, and check relabeling
// are the normal route out of the corpus. A PARTIAL declaration fails closed.
func unboundRegressions(gm gateMap, baseline map[string]map[string]bool, retired map[string]bool) []string {
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
		named := exceptionRefs(point.ExceptedBy)
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

// buildGateMap joins native Go binding comments against the points on disk.
// root is where a point's `rationale:` path is resolved from.
func buildGateMap(sources map[string]string, pointsDir, root string) (gateMap, error) {
	onDisk, err := pointsOnDisk(pointsDir)
	if err != nil {
		return gateMap{}, err
	}
	rationales, missingRationale := rationaleProblems(onDisk, root)
	excepted, missingException := exceptionProblems(onDisk)

	points := make(map[string]string, len(onDisk))
	for ref, point := range onDisk {
		points[ref] = point.Kind
	}

	var bindings []Binding
	for _, name := range sortedKeys(sources) {
		file, err := parseHookFile(sources[name], name)
		if err != nil {
			return gateMap{}, err
		}
		bindings = append(bindings, file.bindings...)
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

	return gateMap{
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
