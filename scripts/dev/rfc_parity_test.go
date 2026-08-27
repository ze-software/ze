// The migration's proof for the RFC conformance gate: the script and the
// command answer the same thing.
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11 -- over this checkout and
// over fixture trees, rfc_requirements.py --extraction-status answers what
// `le rfc extraction-status` answers, in exit code and in bytes, and the two
// halves agree on every INTERNAL answer the envelope is derived from.
// PREVENTS: a port that agrees about a number while disagreeing about how it
// got there. The envelope publishes six integers, and a sign-off evaluation
// that stopped firing would leave every one of them unchanged for a corpus
// where no artifact is broken -- which is this corpus today.
//
// So the comparison has two halves. The GATE comparison runs both programs and
// compares stdout and the exit code. The INTERNALS comparison runs a driver
// that imports the module and prints what the gate never does: every tag the
// tree scan found, every requirement row parsed, the carrier table, the
// sign-off violations and the valid set. A fixture that breaks a sign-off then
// proves the two halves refuse it for the SAME REASON rather than merely both
// refusing it.
//
// A FIXTURE case copies the script into the fixture tree. The script derives
// its checkout from its own __file__, so this checkout's copy would judge this
// checkout while the command judged the fixture. It copies four more files for
// the same reason: the module reads the functional run list and the workflow
// directory at IMPORT time, and a fixture without them cannot be imported at
// all.
//
// Helpers carry an rfcPy prefix. Other steps are porting into this same
// package, and two helpers of one name cannot both exist.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcPyScript is the module this file compares against.
const rfcPyScript = "rfc_requirements.py"

// rfcPyCopied is every file the module needs to be IMPORTED at all: itself, the
// tagged-unit definition it loads by path, the functional run list it reads the
// verify tier from, and the workflow directory it reads the nightly tier from.
var rfcPyCopied = []string{
	"scripts/dev/rfc_requirements.py",
	"scripts/dev/rfc_tagged_scope.py",
	"scripts/le/application/functional.py",
}

// ansiRE strips the color a script writes and a command does not. Color is a
// rendering rather than an answer, and the port leaves it to the terminal.
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// rfcPyTree writes a fixture checkout the module can be imported inside, and
// points the command at it.
//
// The files map is repo-relative and is written on top of the copied files, so
// a case can replace one of them.
func rfcPyTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := devPyTree(t, files)
	source := devPyRoot(t)
	for _, rel := range rfcPyCopied {
		body, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(rel))) // #nosec G304 -- a tracked path
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		rfcPyWrite(t, root, rel, string(body))
	}
	// One scheduled workflow, copied whole rather than invented: the interop
	// carriers' tier is derived from what it names, so a fixture that made one
	// up would compare two halves against a pipeline neither repository has.
	names, err := os.ReadDir(filepath.Join(source, ".github", "workflows"))
	if err != nil {
		t.Fatalf("reading the workflow directory: %v", err)
	}
	for _, entry := range names {
		if strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml") {
			body, err := os.ReadFile(filepath.Join(source, ".github", "workflows", entry.Name())) // #nosec G304 -- a tracked path
			if err != nil {
				t.Fatalf("reading %s: %v", entry.Name(), err)
			}
			rfcPyWrite(t, root, ".github/workflows/"+entry.Name(), string(body))
		}
	}
	devPyPointAt(t, root)
	return root
}

// rfcPyWrite puts one file into a fixture tree, creating its directory.
func rfcPyWrite(t *testing.T, tree, rel, body string) {
	t.Helper()

	path := filepath.Join(tree, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("fixture directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("fixture file %s: %v", rel, err)
	}
}

// rfcPyRunScript runs the copy of the module that sits INSIDE tree.
func rfcPyRunScript(t *testing.T, tree string, args ...string) devPyResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	argv := append([]string{filepath.Join(tree, "scripts", "dev", rfcPyScript)}, args...)
	cmd := exec.CommandContext(ctx, "python3", argv...) // #nosec G204 -- a fixture path and a test's own arguments
	cmd.Dir = tree
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()

	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running %s: %v: %s", rfcPyScript, err, errOut.String())
	}
	return devPyResult{Stdout: out.String(), Stderr: errOut.String(), Code: cmd.ProcessState.ExitCode()}
}

// rfcPyStatusCommand runs `le rfc extraction-status` over the tree the
// environment names, through the same path the binary runs it through.
//
// It captures the PROCESS stderr as well as the writer leroot.Run is handed.
// A refusal that stopped the gate is written by leaction.ReportError, which
// names os.Stderr directly -- that is the spelling every ported le tool uses,
// and a comparison that read only the injected writer would compare an empty
// string against the script's whole failure page.
func rfcPyStatusCommand(t *testing.T) devPyResult {
	t.Helper()
	return rfcPyActionCommand(t, "extraction-status")
}

// rfcPyActionCommand runs one `le rfc <action>` over the tree the environment
// names, capturing the PROCESS stderr as well as the writer leroot.Run is
// handed. See rfcPyStatusCommand for why both are needed.
func rfcPyActionCommand(t *testing.T, action ...string) devPyResult {
	t.Helper()

	saved := os.Stderr
	captured, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatalf("capturing stderr: %v", err)
	}
	os.Stderr = captured
	result := devPyRunCommand(t, "rfc", rfc.Answer, action)
	os.Stderr = saved
	if err := captured.Close(); err != nil {
		t.Fatalf("closing the stderr capture: %v", err)
	}
	body, err := os.ReadFile(captured.Name()) // #nosec G304 -- a path this test made
	if err != nil {
		t.Fatalf("reading the stderr capture: %v", err)
	}
	result.Stderr += string(body)
	return result
}

// rfcPyGateText normalises the one deliberate difference in a failure page: the
// script prints `rfc-requirements: cannot run: X` in color on STDOUT, and the
// command prints `error: X` on stderr, which is the spelling every ported le
// tool uses and the stream a genuine failure belongs on.
func rfcPyGateText(result devPyResult) string {
	text := ansiRE.ReplaceAllString(result.Stdout+result.Stderr, "")
	text = strings.ReplaceAll(text, "rfc-requirements: cannot run: ", "error: ")
	return text
}

// rfcPyAgree fails unless both halves answered the same code and the same page.
func rfcPyAgree(t *testing.T, what string, script, command devPyResult) {
	t.Helper()
	devPyAgree(t, what, script, command, rfcPyGateText(script), rfcPyGateText(command))
}

// ─── The internals comparison ───────────────────────────────────────────────

// rfcPyInternals is what both halves are asked for beyond the envelope.
type rfcPyInternals struct {
	Tags        [][]any  `json:"tags"`
	Carriers    [][]any  `json:"carriers"`
	Rows        [][]any  `json:"req_rows"`
	ParseErrors []string `json:"parse_errs"`
	SignoffErrs []string `json:"signoff_errs"`
	Signed      []string `json:"signed"`
	Registers   []any    `json:"registers"`
}

// rfcPyDriver is the program the interpreter runs. It imports the module by
// path and prints the answers the gate never publishes, so a check that stopped
// firing is visible here even when the published count is unchanged.
const rfcPyDriver = `
import importlib.util, json, sys
spec = importlib.util.spec_from_file_location("rr", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
enrolled, reqs, parse_errs, tags, by_stem = m._collect_for_check()
signed, errs = m.evaluate_extractions(reqs)
print(json.dumps({
  "tags": [[t.rid, t.polarity, t.file, t.line] for t in tags],
  "carriers": [[c.name, c.kind, c.tier, c.prefix, c.suffix, c.reader, c.runner,
                c.pipeline, c.derived] for c in m.CARRIERS],
  "req_rows": [[r.rfc, r.rid, r.level, r.text, r.section, r.line, r.ticked,
                list(r.annotation) if r.annotation else None,
                list(r.superseded) if r.superseded else None] for r in reqs],
  "parse_errs": parse_errs,
  "signoff_errs": errs,
  "signed": sorted(signed),
  "registers": sorted(m.derived_registers(signed, reqs).items()),
}))
`

// rfcPyScriptInternals asks the module inside tree for its internal answers.
func rfcPyScriptInternals(t *testing.T, tree string) rfcPyInternals {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", rfcPyDriver, // #nosec G204 -- a fixture path
		filepath.Join(tree, "scripts", "dev", rfcPyScript))
	cmd.Dir = tree
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("the driver could not read the module: %v: %s", err, errOut.String())
	}
	var found rfcPyInternals
	if err := json.Unmarshal(out.Bytes(), &found); err != nil {
		t.Fatalf("the driver answered no JSON: %v: %s", err, out.String())
	}
	return found
}

// rfcPyCommandInternals asks the package for the same answers.
func rfcPyCommandInternals(t *testing.T, tree string) rfcPyInternals {
	t.Helper()

	collected, err := rfc.Collect(tree)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	deriver := rfc.NewDeriver(tree)
	signed, errs, err := rfc.EvaluateExtractions(deriver, collected.Requirements)
	if err != nil {
		t.Fatalf("EvaluateExtractions: %v", err)
	}
	gated := rfc.GatedCounts(collected.Requirements)
	registers := map[string]string{}
	for _, stem := range rfcPySortedKeys(signed) {
		inv, err := deriver.Inventory(stem, gated[stem])
		if err != nil {
			t.Fatalf("Inventory(%s): %v", stem, err)
		}
		if inv != nil {
			registers[stem] = inv.Register
		}
	}
	carriers, err := rfc.Carriers(tree)
	if err != nil {
		t.Fatalf("Carriers: %v", err)
	}

	found := rfcPyInternals{
		Tags: [][]any{}, Carriers: [][]any{}, Rows: [][]any{},
		ParseErrors: []string{}, SignoffErrs: []string{}, Signed: []string{},
		Registers: []any{},
	}
	for _, tag := range collected.Tags {
		found.Tags = append(found.Tags, []any{tag.RID, tag.Polarity, tag.File, float64(tag.Line)})
	}
	for _, one := range carriers {
		found.Carriers = append(found.Carriers, []any{one.Name, one.Kind, one.Tier, one.Prefix,
			one.Suffix, one.Reader, one.Runner, one.Pipeline, one.Derived})
	}
	for _, req := range collected.Requirements {
		found.Rows = append(found.Rows, []any{req.RFC, req.RID, req.Level, req.Text, req.Section,
			float64(req.Line), req.Ticked, rfcPyAnnotation(req), rfcPySuccessor(req)})
	}
	found.ParseErrors = append(found.ParseErrors, collected.ParseErrors...)
	found.SignoffErrs = append(found.SignoffErrs, errs...)
	found.Signed = append(found.Signed, rfcPySortedKeys(signed)...)
	for _, stem := range rfcPySortedKeys(registers) {
		found.Registers = append(found.Registers, []any{stem, registers[stem]})
	}
	return found
}

// rfcPyAnnotation renders a coverage annotation the way the driver renders the
// Python tuple, so the two are compared field by field rather than by shape.
func rfcPyAnnotation(req rfc.Requirement) any {
	if req.Annotation == nil {
		return nil
	}
	var polarity any
	if req.Annotation.Polarity != "" {
		polarity = req.Annotation.Polarity
	}
	return []any{req.Annotation.Kind, polarity, req.Annotation.Reason}
}

func rfcPySuccessor(req rfc.Requirement) any {
	if req.Superseded == nil {
		return nil
	}
	var target any
	if req.Superseded.Target != "" {
		target = req.Superseded.Target
	}
	return []any{req.Superseded.Disposition, target, req.Superseded.Reason}
}

func rfcPySortedKeys[V any](in map[string]V) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// rfcPyInternalsAgree compares every internal answer, naming the first row that
// differs rather than dumping two documents.
func rfcPyInternalsAgree(t *testing.T, what string, script, command rfcPyInternals) {
	t.Helper()

	rfcPyRowsAgree(t, what, "tag", script.Tags, command.Tags)
	rfcPyRowsAgree(t, what, "carrier", script.Carriers, command.Carriers)
	rfcPyRowsAgree(t, what, "requirement row", script.Rows, command.Rows)
	rfcPyListsAgree(t, what, "parse error", script.ParseErrors, command.ParseErrors)
	rfcPyListsAgree(t, what, "sign-off violation", script.SignoffErrs, command.SignoffErrs)
	rfcPyListsAgree(t, what, "signed stem", script.Signed, command.Signed)
	rfcPyRowsAgree(t, what, "derived register", rfcPyRows(script.Registers), rfcPyRows(command.Registers))
}

// rfcPyRows re-shapes the register pairs, which the driver prints as a list of
// two-element lists.
func rfcPyRows(in []any) [][]any {
	out := make([][]any, 0, len(in))
	for _, one := range in {
		row, ok := one.([]any)
		if !ok {
			row = []any{one}
		}
		out = append(out, row)
	}
	return out
}

func rfcPyRowsAgree(t *testing.T, what, kind string, script, command [][]any) {
	t.Helper()

	if len(script) != len(command) {
		t.Errorf("%s: the script found %d %s(s) and the command found %d",
			what, len(script), kind, len(command))
	}
	for i := 0; i < len(script) && i < len(command); i++ {
		left, right := rfcPyRender(script[i]), rfcPyRender(command[i])
		if left != right {
			t.Fatalf("%s: %s %d differs\nscript:  %s\ncommand: %s", what, kind, i, left, right)
		}
	}
}

func rfcPyListsAgree(t *testing.T, what, kind string, script, command []string) {
	t.Helper()

	if len(script) != len(command) {
		t.Errorf("%s: the script found %d %s(s) and the command found %d\nscript:\n%s\ncommand:\n%s",
			what, len(script), kind, len(command),
			strings.Join(script, "\n"), strings.Join(command, "\n"))
	}
	for i := 0; i < len(script) && i < len(command); i++ {
		if script[i] != command[i] {
			t.Fatalf("%s: %s %d differs\nscript:  %s\ncommand: %s", what, kind, i, script[i], command[i])
		}
	}
}

func rfcPyRender(row []any) string {
	raw, err := json.Marshal(row)
	if err != nil {
		return "unrenderable"
	}
	return string(raw)
}

// ─── The fixture corpus ─────────────────────────────────────────────────────

// rfcPySource is the RFC's own text every fixture case derives its inventory
// from. Two sentences carry a capitalised keyword and one does not, so the
// derived register is rfc2119 and section 2 holds two sites.
const rfcPySource = `Test RFC 9999

1.  Introduction

    This document describes widgets.

2.  Widgets

    A speaker MUST send the widget. A speaker SHOULD log it. A receiver
    MUST NOT drop the widget.
`

// rfcPySummary is the checklist those two sites map to.
const rfcPySummary = `# RFC 9999

| Field | Value |
|-------|-------|
| Obsoleted-by | None |

## Compliance Checklist

- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2)
- [ ] [RFC9999-2-2] [MUST NOT] A receiver MUST NOT drop the widget (§2)
`

// rfcPyBase answers the fixture files every case starts from.
func rfcPyBase() map[string]string {
	return map[string]string{
		"rfc/enrolled.txt":     "rfc9999\n",
		"rfc/short/rfc9999.md": rfcPySummary,
		"rfc/full/rfc9999.txt": rfcPySource,
	}
}

// rfcPyWith answers the base fixture with the named files added or replaced.
func rfcPyWith(extra map[string]string) map[string]string {
	files := rfcPyBase()
	maps.Copy(files, extra)
	return files
}

// rfcPyArtifact renders a sign-off over rfcPySource. The disposition of each
// site is the caller's, which is what lets one helper serve the valid case and
// every refusal.
func rfcPyArtifact(sites, sections string) string {
	return `{
  "schema-version": 1,
  "stem": "rfc9999",
  "register": "rfc2119",
  "source-path": "rfc/full/rfc9999.txt",
  "source-sha": "` + rfcPySourceSHA + `",
  "signed-off": "2026-08-26",
  "reviewer": "the parity test",
  "sections": [` + sections + `],
  "sites": [` + sites + `]
}
`
}

// rfcPySourceSHA is the fingerprint rfcPySource derives. It is a literal rather
// than a call, so a change to the fingerprint function reddens every fixture
// here instead of silently agreeing with itself.
const rfcPySourceSHA = "cdebb2f8411148db"

// rfcPyProseSourceSHA is the same source with every keyword lowercased, which
// derives the `prose` register instead.
const rfcPyProseSourceSHA = "2d1b156296454db9"

// The three sections rfcPySource derives, walked.
const rfcPySections = `
    {"id": "front", "sites": 0, "disposition": "walked"},
    {"id": "1", "sites": 0, "disposition": "walked"},
    {"id": "2", "sites": 2, "disposition": "walked"}`

// The two sites rfcPySource derives, each mapped to its row.
const rfcPyMappedSites = `
    {"id": "2:1", "quote": "A speaker MUST send the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-1"},
    {"id": "2:2", "quote": "A receiver MUST NOT drop the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-2"}`

// ─── The cases ──────────────────────────────────────────────────────────────

func TestRFCExtractionStatusBothHalvesAgreeOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)

	script := rfcPyRunScript(t, root, "--extraction-status")
	command := rfcPyStatusCommand(t)

	rfcPyAgree(t, "rfc extraction-status over the checkout", script, command)
	if !strings.Contains(command.Stdout, `"signed-by-register"`) {
		t.Errorf("the command answered no envelope over the real checkout: %q", command.Stdout)
	}
	if command.Stderr != "" {
		t.Errorf("the command wrote to stderr over a clean checkout: %q", command.Stderr)
	}
}

func TestRFCInternalsBothHalvesAgreeOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)

	script := rfcPyScriptInternals(t, root)
	command := rfcPyCommandInternals(t, root)

	rfcPyInternalsAgree(t, "the internals over the checkout", script, command)
	if len(command.Tags) == 0 || len(command.Rows) == 0 {
		t.Fatalf("the command read nothing over the real checkout: %d tag(s), %d row(s)",
			len(command.Tags), len(command.Rows))
	}
}

// rfcPyCompare runs both halves over one fixture and compares the envelope and
// the internals together, which is what tells "both refused" from "both refused
// for the same reason".
func rfcPyCompare(t *testing.T, what string, files map[string]string) devPyResult {
	t.Helper()

	tree := rfcPyTree(t, files)
	script := rfcPyRunScript(t, tree, "--extraction-status")
	command := rfcPyStatusCommand(t)
	rfcPyAgree(t, what, script, command)
	rfcPyInternalsAgree(t, what, rfcPyScriptInternals(t, tree), rfcPyCommandInternals(t, tree))
	return command
}

func TestRFCBothHalvesAgreeOverAFixtureWithNoSignOff(t *testing.T) {
	answer := rfcPyCompare(t, "an enrolled RFC nobody has walked", rfcPyBase())

	if !strings.Contains(answer.Stdout, `"backlog": 1`) {
		t.Errorf("the unsigned RFC is not in the backlog: %s", answer.Stdout)
	}
}

func TestRFCBothHalvesAgreeOverAValidSignOff(t *testing.T) {
	answer := rfcPyCompare(t, "a complete sign-off", rfcPyWith(map[string]string{
		"rfc/extraction/rfc9999.json": rfcPyArtifact(rfcPyMappedSites, rfcPySections),
	}))

	if !strings.Contains(answer.Stdout, `"signed": 1`) {
		t.Errorf("the valid sign-off earned no credit: %s", answer.Stdout)
	}
	if !strings.Contains(answer.Stdout, `"rfc2119": 1`) {
		t.Errorf("the derived register is not published: %s", answer.Stdout)
	}
}

func TestRFCBothHalvesRefuseTheSameSignOffDefects(t *testing.T) {
	cases := []struct {
		name     string
		sites    string
		sections string
		summary  string
		want     string
	}{
		{
			name: "an unclassified site",
			sites: `
    {"id": "2:1", "quote": "A speaker MUST send the widget.", "disposition": null},
    {"id": "2:2", "quote": "A receiver MUST NOT drop the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-2"}`,
			sections: rfcPySections,
			want:     "site 2:1 is UNCLASSIFIED",
		},
		{
			name: "a quote edited away from the source",
			sites: `
    {"id": "2:1", "quote": "A speaker MAY send the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-1"},
    {"id": "2:2", "quote": "A receiver MUST NOT drop the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-2"}`,
			sections: rfcPySections,
			want:     "quote does not match the source",
		},
		{
			name: "a site the derivation does not hold",
			sites: rfcPyMappedSites + `,
    {"id": "9:1", "quote": "Invented.", "disposition": "mapped",
     "mapped-to": "RFC9999-2-1"}`,
			sections: rfcPySections,
			want:     "site 9:1 is not in the derived inventory",
		},
		{
			name: "a derived site absent from the walk",
			sites: `
    {"id": "2:1", "quote": "A speaker MUST send the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-1"}`,
			sections: rfcPySections,
			want:     "derived site 2:2 is absent from the sign-off",
		},
		{
			name:  "a section left unclassified",
			sites: rfcPyMappedSites,
			sections: `
    {"id": "front", "sites": 0, "disposition": "walked"},
    {"id": "1", "sites": 0, "disposition": null},
    {"id": "2", "sites": 2, "disposition": "walked"}`,
			want: "section 1 is UNCLASSIFIED",
		},
		{
			name:  "a section whose site count was typed rather than derived",
			sites: rfcPyMappedSites,
			sections: `
    {"id": "front", "sites": 0, "disposition": "walked"},
    {"id": "1", "sites": 0, "disposition": "walked"},
    {"id": "2", "sites": 7, "disposition": "walked"}`,
			want: "section 2 records 7 site(s); the source derives 2",
		},
		{
			name: "a site naming a requirement the summary does not declare",
			sites: `
    {"id": "2:1", "quote": "A speaker MUST send the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-9"},
    {"id": "2:2", "quote": "A receiver MUST NOT drop the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-2"}`,
			sections: rfcPySections,
			want:     "names RFC9999-2-9, which does not exist",
		},
		{
			name: "a duplicate-of that no other site maps",
			sites: `
    {"id": "2:1", "quote": "A speaker MUST send the widget.",
     "disposition": "excluded", "excluded-kind": "duplicate-of",
     "mapped-to": "RFC9999-2-1", "reason": "said twice"},
    {"id": "2:2", "quote": "A receiver MUST NOT drop the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-2"}`,
			sections: rfcPySections,
			want:     "but no other site MAPS that id",
		},
		{
			name: "a MUST-level sentence mapped to an advisory row",
			sites: `
    {"id": "2:1", "quote": "A speaker MUST send the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-1"},
    {"id": "2:2", "quote": "A receiver MUST NOT drop the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-2"}`,
			sections: rfcPySections,
			summary: strings.Replace(rfcPySummary, "[RFC9999-2-2] [MUST NOT]",
				"[RFC9999-2-2] [SHOULD]", 1),
			want: "which is advisory and never gates",
		},
		{
			name: "a gated requirement no site backs",
			sites: `
    {"id": "2:1", "quote": "A speaker MUST send the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-1"},
    {"id": "2:2", "quote": "A receiver MUST NOT drop the widget.",
     "disposition": "excluded", "excluded-kind": "not-a-requirement",
     "reason": "a diagram caption"}`,
			sections: rfcPySections,
			want:     "no source site maps to it and no section lists it",
		},
		{
			name:  "an unsourced id the summary does not declare",
			sites: rfcPyMappedSites,
			sections: `
    {"id": "front", "sites": 0, "disposition": "walked"},
    {"id": "1", "sites": 0, "disposition": "walked"},
    {"id": "2", "sites": 2, "disposition": "walked",
     "unsourced-ids": ["RFC9999-4-1"]}`,
			want: "unsourced-ids names RFC9999-4-1",
		},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			files := map[string]string{
				"rfc/extraction/rfc9999.json": rfcPyArtifact(one.sites, one.sections),
			}
			if one.summary != "" {
				files["rfc/short/rfc9999.md"] = one.summary
			}
			tree := rfcPyTree(t, rfcPyWith(files))

			script := rfcPyScriptInternals(t, tree)
			command := rfcPyCommandInternals(t, tree)
			rfcPyInternalsAgree(t, one.name, script, command)

			if len(command.SignoffErrs) == 0 {
				t.Fatalf("%s was accepted by the command", one.name)
			}
			if !strings.Contains(strings.Join(command.SignoffErrs, "\n"), one.want) {
				t.Errorf("%s: no violation named %q\n%s", one.name, one.want,
					strings.Join(command.SignoffErrs, "\n"))
			}
			if len(command.Signed) != 0 {
				t.Errorf("%s still earned credit: %v", one.name, command.Signed)
			}
		})
	}
}

func TestRFCBothHalvesRefuseAStaleSourceFingerprintWithOneMessage(t *testing.T) {
	artifact := strings.Replace(rfcPyArtifact(rfcPyMappedSites, rfcPySections),
		rfcPySourceSHA, "0000000000000000", 1)
	tree := rfcPyTree(t, rfcPyWith(map[string]string{
		"rfc/extraction/rfc9999.json": artifact,
	}))

	script := rfcPyScriptInternals(t, tree)
	command := rfcPyCommandInternals(t, tree)
	rfcPyInternalsAgree(t, "a stale source fingerprint", script, command)

	// One accurate error, not a wall: with the source moved every site and
	// every section would mismatch too, and the only useful message is this one.
	if len(command.SignoffErrs) != 1 {
		t.Fatalf("a stale fingerprint produced %d violations, want 1:\n%s",
			len(command.SignoffErrs), strings.Join(command.SignoffErrs, "\n"))
	}
	if !strings.Contains(command.SignoffErrs[0], "source-sha no longer matches") {
		t.Errorf("the one violation is not the stale one: %s", command.SignoffErrs[0])
	}
}

func TestRFCBothHalvesRefuseARegisterStrongerThanTheSource(t *testing.T) {
	// A source with no capitalised keyword derives `prose`, so an rfc2119
	// claim over it is stronger than the text supports.
	source := strings.ReplaceAll(rfcPySource, "MUST NOT", "must not")
	source = strings.ReplaceAll(source, "MUST", "must")
	// The lowercase source has its own fingerprint, and a sign-off carrying the
	// capitalised one would be refused for staleness BEFORE the register is
	// judged -- which would leave this case asserting the wrong refusal.
	artifact := strings.Replace(rfcPyArtifact(`
    {"id": "2:1", "quote": "A speaker must send the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-1"},
    {"id": "2:2", "quote": "A receiver must not drop the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-2"}`, rfcPySections),
		rfcPySourceSHA, rfcPyProseSourceSHA, 1)
	tree := rfcPyTree(t, rfcPyWith(map[string]string{
		"rfc/full/rfc9999.txt":        source,
		"rfc/extraction/rfc9999.json": artifact,
	}))

	script := rfcPyScriptInternals(t, tree)
	command := rfcPyCommandInternals(t, tree)
	rfcPyInternalsAgree(t, "a register stronger than the source", script, command)

	if !strings.Contains(strings.Join(command.SignoffErrs, "\n"), "is STRONGER than the source supports") {
		t.Errorf("the strong claim was accepted:\n%s", strings.Join(command.SignoffErrs, "\n"))
	}
}

// ─── The refusals that stop the run ─────────────────────────────────────────

// A malformed artifact, a malformed checklist line and an unreadable scenario
// check each stop the collection rather than producing a violation, so the
// internals driver cannot be asked about them: it would die with the module.
// These cases compare the GATE, which is what an operator and a Make target
// see.

// rfcPyTokenizeRE normalises the interpreter's own reason for refusing a Python
// file. That reason is the reader's, not the gate's: Python names a line and a
// construct, and the Go reader names the state it could not resolve. Everything
// around it -- the path, the frame, the refusal -- is compared byte for byte.
var rfcPyTokenizeRE = regexp.MustCompile(`cannot tokenize as Python \(.*\); a file whose`)

// rfcPyGateOnly runs both halves over one fixture and compares what the gate
// printed and exited with.
func rfcPyGateOnly(t *testing.T, what string, files map[string]string) devPyResult {
	t.Helper()

	tree := rfcPyTree(t, files)
	script := rfcPyRunScript(t, tree, "--extraction-status")
	command := rfcPyStatusCommand(t)

	left := rfcPyTokenizeRE.ReplaceAllString(rfcPyGateText(script), "cannot tokenize as Python (why); a file whose")
	right := rfcPyTokenizeRE.ReplaceAllString(rfcPyGateText(command), "cannot tokenize as Python (why); a file whose")
	devPyAgree(t, what, script, command, left, right)
	return command
}

func TestRFCBothHalvesRefuseTheSameMalformedArtifacts(t *testing.T) {
	cases := []struct {
		name     string
		artifact string
		want     string
	}{
		{
			name:     "a document that is not an object",
			artifact: "[]\n",
			want:     "expected a JSON object, got list",
		},
		{
			name: "a key nobody reads",
			artifact: `{"schema-version": 1, "stem": "rfc9999", "register": "rfc2119",
 "source-path": "rfc/full/rfc9999.txt", "source-sha": "x", "sections": [],
 "sites": [], "reviewed-by": "typo"}`,
			want: "unknown key(s) ['reviewed-by']",
		},
		{
			name: "a schema version this reader does not know",
			artifact: `{"schema-version": 2, "stem": "rfc9999", "register": "rfc2119",
 "source-path": "a", "source-sha": "b", "sections": [], "sites": []}`,
			want: "schema-version must be 1, got 2",
		},
		{
			name: "a stem that disagrees with the filename",
			artifact: `{"schema-version": 1, "stem": "rfc1234", "register": "rfc2119",
 "source-path": "a", "source-sha": "b", "sections": [], "sites": []}`,
			want: "does not match the filename",
		},
		{
			name: "a register outside the closed set",
			artifact: `{"schema-version": 1, "stem": "rfc9999", "register": "strong",
 "source-path": "a", "source-sha": "b", "sections": [], "sites": []}`,
			want: "is missing, empty or unknown",
		},
		{
			name: "a site count written as a float",
			artifact: `{"schema-version": 1, "stem": "rfc9999", "register": "rfc2119",
 "source-path": "a", "source-sha": "b", "sites": [],
 "sections": [{"id": "1", "sites": 2.0, "disposition": "walked"}]}`,
			want: "'sites' must be a non-negative integer",
		},
		{
			name: "a skipped section with no kind",
			artifact: `{"schema-version": 1, "stem": "rfc9999", "register": "rfc2119",
 "source-path": "a", "source-sha": "b", "sites": [],
 "sections": [{"id": "1", "sites": 0, "disposition": "skipped", "reason": "why"}]}`,
			want: "skipped needs a 'skip-kind' from",
		},
		{
			name: "an excluded site with no reason",
			artifact: `{"schema-version": 1, "stem": "rfc9999", "register": "rfc2119",
 "source-path": "a", "source-sha": "b", "sections": [],
 "sites": [{"id": "2:1", "quote": "q", "disposition": "excluded",
            "excluded-kind": "not-a-requirement"}]}`,
			want: "excluded needs a non-empty 'reason'",
		},
		{
			name: "a relocation naming a deferral shard",
			artifact: `{"schema-version": 1, "stem": "rfc9999", "register": "rfc2119",
 "source-path": "a", "source-sha": "b", "sections": [],
 "sites": [{"id": "2:1", "quote": "q", "disposition": "excluded",
            "excluded-kind": "relocated-to-spec", "reason": "moved",
            "relocated-to": "plan/deferrals/x.md", "reserved-id": "RFC9999-2-1"}]}`,
			want: "needs a 'relocated-to' naming the spec",
		},
		{
			name: "a relocation field on a site that is not one",
			artifact: `{"schema-version": 1, "stem": "rfc9999", "register": "rfc2119",
 "source-path": "a", "source-sha": "b", "sections": [],
 "sites": [{"id": "2:1", "quote": "q", "disposition": "mapped",
            "mapped-to": "RFC9999-2-1", "relocated-to": "plan/spec-x.md"}]}`,
			want: "mean something only on a relocated-to-spec exclusion",
		},
		{
			name: "two sites sharing one locator",
			artifact: `{"schema-version": 1, "stem": "rfc9999", "register": "rfc2119",
 "source-path": "a", "source-sha": "b", "sections": [],
 "sites": [{"id": "2:1", "quote": "q", "disposition": null},
           {"id": "2:1", "quote": "r", "disposition": null}]}`,
			want: "duplicate site locator '2:1'",
		},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			answer := rfcPyGateOnly(t, one.name, rfcPyWith(map[string]string{
				"rfc/extraction/rfc9999.json": one.artifact,
			}))
			if answer.Code != 2 {
				t.Fatalf("%s exited %d, want 2", one.name, answer.Code)
			}
			if !strings.Contains(answer.Stderr, one.want) {
				t.Errorf("%s: the refusal does not say %q\n%s", one.name, one.want, answer.Stderr)
			}
		})
	}
}

func TestRFCBothHalvesRefuseTheSameMalformedChecklistLines(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "a compliance line with no id",
			line: "- [ ] [MUST] A speaker MUST send the widget (§2)",
			want: "checklist line has no requirement id",
		},
		{
			name: "the retired counter form",
			line: "- [ ] [RFC9999-R012] [MUST] A speaker MUST send the widget (§2)",
			want: "carries an RFC 2119 keyword but does not parse",
		},
		{
			name: "an id whose section disagrees with the citation",
			line: "- [ ] [RFC9999-5.3-1] [MUST] A speaker MUST send the widget (§2)",
			want: "disagrees with its section",
		},
		{
			name: "an annotation with no reason",
			line: "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) {gap}",
			want: "has no reason",
		},
		{
			name: "an annotation kind nobody knows",
			line: "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) {waived: later}",
			want: "unknown annotation kind 'waived'",
		},
		{
			name: "a single-polarity annotation with no polarity",
			line: "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) {single-polarity: why}",
			want: "single-polarity needs a polarity from",
		},
		{
			name: "a superseded marker with no reason",
			line: "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) {superseded: dropped}",
			want: "has no reason",
		},
		{
			name: "a superseded restatement naming nothing",
			line: "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) {superseded: restated; why}",
			want: "needs exactly one successor requirement id",
		},
		{
			name: "two coverage annotations on one line",
			line: "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) {gap: a} {gap: b}",
			want: "two coverage annotations on one line",
		},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			var summary strings.Builder
			summary.WriteString("# RFC 9999\n\n## Compliance Checklist\n\n")
			summary.WriteString(one.line)
			summary.WriteString("\n")
			tree := rfcPyTree(t, rfcPyWith(map[string]string{
				"rfc/short/rfc9999.md": summary.String(),
			}))

			// A parse failure is REPORTED rather than fatal, so the gate still
			// answers an envelope and the internals driver still runs.
			script := rfcPyScriptInternals(t, tree)
			command := rfcPyCommandInternals(t, tree)
			rfcPyInternalsAgree(t, one.name, script, command)

			if len(command.ParseErrors) != 1 {
				t.Fatalf("%s produced %d parse error(s), want 1: %v",
					one.name, len(command.ParseErrors), command.ParseErrors)
			}
			if !strings.Contains(command.ParseErrors[0], one.want) {
				t.Errorf("%s: the parse error does not say %q: %s",
					one.name, one.want, command.ParseErrors[0])
			}
		})
	}
}

// ─── The tag scan ───────────────────────────────────────────────────────────

// rfcPyTaggedTree lays a tag in each carrier shape the table recognizes, so one
// fixture proves the reader, the pre-filter and the skip rules together.
func rfcPyTagFiles() map[string]string {
	return map[string]string{
		// A unit test: read line by line, tags anywhere.
		"internal/widget/widget_test.go": `package widget

// RFC requirement: RFC9999-2-1 positive -- the widget is sent
func TestSend(t *testing.T) {}
`,
		// Development-tool tests exercise the scanner. Moving the tool under
		// internal/ must not turn its fixture strings into protocol proof.
		"internal/le/rfc/audit_test.go": `package rfc

// RFC requirement: RFC9999-9-9 positive -- scanner fixture, not protocol evidence
func TestFixture(t *testing.T) {}
`,
		// A .ci in a suite the run list names: verify tier.
		"test/plugin/widget.ci": `# RFC requirement: RFC9999-2-2 negative -- a dropped widget is refused
name=widget
`,
		// A terminator block's body is RAW file content, so its own comment is
		// not a tag. This is the case a line scanner invents a phantom from.
		"test/plugin/embedded.ci": `name=embedded
tmpfs=run.sh terminator=EOF
#!/bin/sh
# RFC requirement: RFC9999-9-9 positive -- not a tag, this is a shell comment
EOF
`,
		// The gitignored incubator: SKIPPED, never refused.
		"test/draft/wip.ci": `# RFC requirement: RFC9999-9-9 positive -- a draft claims nothing
name=wip
`,
		// A scenario check: comments only, and a hash inside a string is not one.
		"test/interop/scenarios/widget/check.py": `"""A check.

# RFC requirement: RFC9999-9-9 positive -- inside a docstring, not a comment
"""

PROMPT = "# RFC requirement: RFC9999-9-9 negative -- inside a string"

# RFC requirement: RFC9999-2-1 negative -- the peer refuses a bad widget
def check():
    pass  # RFC requirement: RFC9999-9-9 positive -- trailing, not a tag
`,
	}
}

func TestRFCBothHalvesReadTheSameTagsFromEveryCarrier(t *testing.T) {
	tree := rfcPyTree(t, rfcPyWith(rfcPyTagFiles()))

	script := rfcPyScriptInternals(t, tree)
	command := rfcPyCommandInternals(t, tree)
	rfcPyInternalsAgree(t, "the tag scan over every carrier", script, command)

	if len(command.Tags) != 3 {
		t.Fatalf("the command found %d tag(s), want 3 (one Go, one .ci, one check.py): %v",
			len(command.Tags), command.Tags)
	}
	for _, tag := range command.Tags {
		if rid, _ := tag[0].(string); rid == "RFC9999-9-9" {
			t.Errorf("a phantom tag was read from a place that is not a comment: %v", tag)
		}
	}
}

func TestRFCBothHalvesRefuseATagInACarrierNothingRuns(t *testing.T) {
	answer := rfcPyGateOnly(t, "a tag under a suite nothing runs", rfcPyWith(map[string]string{
		"test/nosuite/widget.ci": `# RFC requirement: RFC9999-2-1 positive -- nothing runs this
name=widget
`,
	}))

	if answer.Code != 2 {
		t.Fatalf("a tag in an unrun carrier exited %d, want 2", answer.Code)
	}
	if !strings.Contains(answer.Stderr, "which nothing executes automatically") {
		t.Errorf("the refusal does not name the carrier: %s", answer.Stderr)
	}
}

func TestRFCBothHalvesRefuseAScenarioCheckThatCannotBeRead(t *testing.T) {
	answer := rfcPyGateOnly(t, "a check.py no reader can trust", rfcPyWith(map[string]string{
		"test/interop/scenarios/broken/check.py": `# RFC requirement: RFC9999-2-1 positive -- unreachable
BROKEN = "an unterminated literal
`,
	}))

	if answer.Code != 2 {
		t.Fatalf("an unreadable check.py exited %d, want 2", answer.Code)
	}
	if !strings.Contains(answer.Stderr, "cannot be reported as carrying no RFC requirement tags") {
		t.Errorf("the refusal does not say why: %s", answer.Stderr)
	}
}

// ─── The relocation tripwire ────────────────────────────────────────────────

// A relocated site says a named spec owes the obligation under a reserved id.
// Nothing about that claim is checkable a year later unless the gate re-reads
// it, so these cases move the row around inside the destination spec and check
// that both halves agree about which of those moves still RESERVES it.

// rfcPyOneRowSummary declares only the first requirement, so the second is the
// one a relocation can claim to have moved.
const rfcPyOneRowSummary = `# RFC 9999

## Compliance Checklist

- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2)
`

// rfcPyRelocatedSites maps the first site and relocates the second.
const rfcPyRelocatedSites = `
    {"id": "2:1", "quote": "A speaker MUST send the widget.",
     "disposition": "mapped", "mapped-to": "RFC9999-2-1"},
    {"id": "2:2", "quote": "A receiver MUST NOT drop the widget.",
     "disposition": "excluded", "excluded-kind": "relocated-to-spec",
     "reason": "an owner ruling moved it", "relocated-to": "plan/spec-widget.md",
     "reserved-id": "RFC9999-2-2"}`

func TestRFCBothHalvesAgreeOnWhatReservesARelocatedObligation(t *testing.T) {
	cases := []struct {
		name     string
		spec     string
		reserved bool
	}{
		{
			name:     "a live row naming the id",
			spec:     "# Spec\n\nThis spec owes RFC9999-2-2 and will implement it.\n",
			reserved: true,
		},
		{
			name:     "the id inside an HTML comment",
			spec:     "# Spec\n\n<!-- RFC9999-2-2 was here -->\n",
			reserved: false,
		},
		{
			name:     "the id struck through",
			spec:     "# Spec\n\n~~This spec owes RFC9999-2-2.~~\n",
			reserved: false,
		},
		{
			name:     "the id inside a backtick fence",
			spec:     "# Spec\n\n```\nRFC9999-2-2\n```\n",
			reserved: false,
		},
		{
			name:     "the id inside a tilde fence",
			spec:     "# Spec\n\n~~~\nRFC9999-2-2\n~~~\n",
			reserved: false,
		},
		{
			name:     "the id inside an indented code block",
			spec:     "# Spec\n\n    grep RFC9999-2-2 rfc/short/rfc9999.md\n",
			reserved: false,
		},
		{
			name:     "a neighboring ordinal and not the id itself",
			spec:     "# Spec\n\nThis spec owes RFC9999-2-25.\n",
			reserved: false,
		},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			tree := rfcPyTree(t, rfcPyWith(map[string]string{
				"rfc/short/rfc9999.md":        rfcPyOneRowSummary,
				"rfc/extraction/rfc9999.json": rfcPyArtifact(rfcPyRelocatedSites, rfcPySections),
				"plan/spec-widget.md":         one.spec,
			}))

			script := rfcPyScriptInternals(t, tree)
			command := rfcPyCommandInternals(t, tree)
			rfcPyInternalsAgree(t, one.name, script, command)

			held := len(command.SignoffErrs) == 0
			if held != one.reserved {
				t.Errorf("%s: reserved=%v, want %v\n%s", one.name, held, one.reserved,
					strings.Join(command.SignoffErrs, "\n"))
			}
			if !one.reserved && !strings.Contains(strings.Join(command.SignoffErrs, "\n"),
				"no longer names RFC9999-2-2 in live prose") {
				t.Errorf("%s: the refusal is not the tripwire's:\n%s", one.name,
					strings.Join(command.SignoffErrs, "\n"))
			}
		})
	}
}

func TestRFCBothHalvesRefuseARelocationWhoseSpecIsGone(t *testing.T) {
	tree := rfcPyTree(t, rfcPyWith(map[string]string{
		"rfc/short/rfc9999.md":        rfcPyOneRowSummary,
		"rfc/extraction/rfc9999.json": rfcPyArtifact(rfcPyRelocatedSites, rfcPySections),
	}))

	script := rfcPyScriptInternals(t, tree)
	command := rfcPyCommandInternals(t, tree)
	rfcPyInternalsAgree(t, "a relocation whose spec is gone", script, command)

	if !strings.Contains(strings.Join(command.SignoffErrs, "\n"),
		"which does not exist or cannot be read") {
		t.Errorf("a missing destination spec was accepted:\n%s",
			strings.Join(command.SignoffErrs, "\n"))
	}
}

func TestRFCBothHalvesRefuseARelocationTheSummaryStillDeclares(t *testing.T) {
	tree := rfcPyTree(t, rfcPyWith(map[string]string{
		"rfc/extraction/rfc9999.json": rfcPyArtifact(rfcPyRelocatedSites, rfcPySections),
		"plan/spec-widget.md":         "# Spec\n\nThis spec owes RFC9999-2-2.\n",
	}))

	script := rfcPyScriptInternals(t, tree)
	command := rfcPyCommandInternals(t, tree)
	rfcPyInternalsAgree(t, "a relocation the summary still declares", script, command)

	if !strings.Contains(strings.Join(command.SignoffErrs, "\n"), "still declares RFC9999-2-2") {
		t.Errorf("an obligation claimed in two places was accepted:\n%s",
			strings.Join(command.SignoffErrs, "\n"))
	}
}

func TestRFCBothHalvesCountARelocationApartFromADismissal(t *testing.T) {
	answer := rfcPyCompare(t, "a relocated obligation", rfcPyWith(map[string]string{
		"rfc/short/rfc9999.md":        rfcPyOneRowSummary,
		"rfc/extraction/rfc9999.json": rfcPyArtifact(rfcPyRelocatedSites, rfcPySections),
		"plan/spec-widget.md":         "# Spec\n\nThis spec owes RFC9999-2-2.\n",
	}))

	if !strings.Contains(answer.Stdout, `"relocated": 1`) {
		t.Errorf("the relocation is not counted apart from the totals: %s", answer.Stdout)
	}
	if !strings.Contains(answer.Stdout, `"signed": 1`) {
		t.Errorf("the sign-off earned no credit: %s", answer.Stdout)
	}
}

// ─── The one place the two halves are meant to differ ───────────────────────

func TestTheScriptCrashesWhereTheCommandRefuses(t *testing.T) {
	// CARRIERS is built at IMPORT time in the script, and two of its inputs
	// raise when they cannot be read. That import runs before main(), so the
	// raise lands outside every try the five drivers wrap themselves in: the
	// script aborts with a traceback and exit 1 where its own docstring
	// promises exit 2 for a gate that could not run.
	//
	// The port answers the documented code. This case asserts BOTH halves, so
	// it reddens the day somebody repairs the script -- and the answer then is
	// to delete it with the row in
	// plan/journal/analyzer-crashes-instead-of-reporting.md.
	tree := rfcPyTree(t, rfcPyBase())
	if err := os.RemoveAll(filepath.Join(tree, ".github")); err != nil {
		t.Fatalf("removing the workflow directory: %v", err)
	}

	script := rfcPyRunScript(t, tree, "--extraction-status")
	if script.Code != 1 {
		t.Fatalf("the script exited %d; this case exists because it exits 1 with a "+
			"traceback, and a repair means deleting it\n%s%s",
			script.Code, script.Stdout, script.Stderr)
	}
	if !strings.Contains(script.Stderr, "Traceback (most recent call last)") {
		t.Errorf("the script no longer crashes; delete this case and its journal row:\n%s",
			script.Stderr)
	}

	command := rfcPyStatusCommand(t)
	if command.Code != 2 {
		t.Errorf("the command exited %d, want the documented 2", command.Code)
	}
	if !strings.Contains(command.Stderr, "cannot read the workflow directory") {
		t.Errorf("the command's refusal does not name what it could not read: %q",
			command.Stderr)
	}
	if strings.Contains(command.Stderr, "goroutine") {
		t.Errorf("the command answered a stack rather than a sentence: %q", command.Stderr)
	}
}

// ─── The audit half: the re-seal, which WRITES ──────────────────────────────

// The fingerprints the tag fixture derives. Literals rather than calls, so a
// change to the fingerprint function or to the tagged-unit definition reddens
// every case here instead of silently agreeing with itself.
const (
	// rfcPyGoFileSHA is the whole of internal/widget/widget_test.go, which is
	// what a `tests` map records.
	rfcPyGoFileSHA = "3436dd64a28e7501"
	// rfcPyGoUnitSHA is TestSend's own text, doc comment through closing
	// brace, which is what a `units` map records.
	rfcPyGoUnitSHA = "afd7ad04ced0d451"
	// rfcPyCheckSHA is the scenario check. It is file-scoped, so one value
	// serves both maps: only Go has a unit boundary this gate narrows to.
	rfcPyCheckSHA = "656e279465958948"
	// rfcPyCISHA is the .ci carrying the negative tag, file-scoped too.
	rfcPyCISHA = "ca610f4eae4c09f0"
	// The two checklist rows' own text.
	rfcPyReqSHA1 = "e411980086b83356"
	rfcPyReqSHA2 = "9e8a3dd9da1b7474"
	// rfcPyOtherSHA is a well-formed fingerprint of nothing in the tree, which
	// is how a case says "this one moved" without having to edit the file.
	rfcPyOtherSHA = "0123456789abcdef"
)

// rfcPyGoKey and rfcPyCheckKey are the two keys RFC9999-2-1's tags mint, in tag
// order: the Go file names its enclosing function, the scenario check is the
// whole file.
const (
	rfcPyGoKey    = "internal/widget/widget_test.go::TestSend"
	rfcPyCheckKey = "test/interop/scenarios/widget/check.py"
	rfcPyCIKey    = "test/plugin/widget.ci"
)

// rfcPyAudit renders one audit record over the tag fixture. The maps are the
// caller's, which is what lets one helper serve every freshness state.
func rfcPyAudit(requirements string) string {
	return `{
  "rfc": "rfc9999",
  "audited": "2026-08-26",
  "reaudit_note": "the note an earlier re-stamp left",
  "requirements": {` + requirements + `
  }
}
`
}

// rfcPyVerdict renders one verdict, with the tests and units maps spelled by
// the caller. An empty units map is written as an ABSENT field, which is the
// pre-units spelling the transitional rule reads.
func rfcPyVerdict(rid, reqSHA, tests, units string) string {
	body := `
    "` + rid + `": {
      "verdict": "enforced",
      "note": "the widget is sent, and a bad one refused",
      "requirement_sha": "` + reqSHA + `",
      "tests": {` + tests + `}`
	if units != "" {
		body += `,
      "units": {` + units + `}`
	}
	return body + `
    }`
}

// rfcPyPair renders the two-entry map RFC9999-2-1's tags produce.
//
// Only the Go half varies: the scenario check is file-scoped, so its recorded
// value is the same in both maps and a case moves the Go one to say what moved.
func rfcPyPair(goSHA string) string {
	return `"` + rfcPyGoKey + `": "` + goSHA + `", "` + rfcPyCheckKey + `": "` + rfcPyCheckSHA + `"`
}

// rfcPyAuditTree answers the tag fixture plus one audit record.
func rfcPyAuditTree(record string) map[string]string {
	files := rfcPyWith(rfcPyTagFiles())
	files["rfc/audit/rfc9999.json"] = record
	return files
}

// rfcPyAuditBytes reads every audit file of a tree, so two trees can be
// compared byte for byte after both halves have written.
func rfcPyAuditBytes(t *testing.T, tree string) map[string]string {
	t.Helper()

	dir := filepath.Join(tree, "rfc", "audit")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	out := map[string]string{}
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(dir, entry.Name())) // #nosec G304 -- a path this test made
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		out[entry.Name()] = string(body)
	}
	return out
}

// rfcPyReseal runs both halves of the re-seal over TWO identical fixture trees
// and compares the page each printed and the bytes each wrote.
//
// Two trees rather than one, because this action WRITES: running both halves
// over one tree would let the second read what the first produced, and a writer
// compared against its own output proves nothing. The pages are compared for
// what an operator sees, and the resulting rfc/audit/ directories are compared
// byte for byte, which is the only thing that says the two records are the
// same record.
func rfcPyReseal(t *testing.T, what string, files map[string]string) devPyResult {
	t.Helper()

	scriptTree := rfcPyTree(t, files)
	commandTree := rfcPyTree(t, files)
	before := rfcPyAuditBytes(t, scriptTree)

	script := rfcPyRunScript(t, scriptTree, "--reseal")
	devPyPointAt(t, commandTree)
	command := rfcPyActionCommand(t, "reseal")

	devPyAgree(t, what, script, command, rfcPyGateText(script), rfcPyGateText(command))

	wrote := rfcPyAuditBytes(t, scriptTree)
	got := rfcPyAuditBytes(t, commandTree)
	if len(wrote) != len(got) {
		t.Fatalf("%s: the script left %d audit file(s) and the command %d", what, len(wrote), len(got))
	}
	for name, body := range wrote {
		if got[name] != body {
			t.Errorf("%s: the two halves wrote different bytes into %s\nscript:\n%s\ncommand:\n%s",
				what, name, body, got[name])
		}
		if _, held := before[name]; !held {
			t.Errorf("%s: the script invented %s", what, name)
		}
	}
	return command
}

func TestRFCBothHalvesResealAShiftedVerdictIntoTheSameBytes(t *testing.T) {
	// The units are byte-identical and the file-level fingerprint is not, which
	// is a line shift and nothing else: exactly the state the re-seal exists
	// for.
	record := rfcPyAudit(rfcPyVerdict("RFC9999-2-1", rfcPyReqSHA1,
		rfcPyPair(rfcPyOtherSHA),
		rfcPyPair(rfcPyGoUnitSHA)))

	command := rfcPyReseal(t, "a shifted verdict", rfcPyAuditTree(record))

	if command.Code != 0 {
		t.Fatalf("the re-seal exited %d over a shifted verdict, want 0", command.Code)
	}
	if !strings.Contains(command.Stdout, "re-stamped rfc9999 RFC9999-2-1") {
		t.Errorf("the page does not name the verdict it re-stamped: %q", command.Stdout)
	}
	// The re-stamp REPLACED the stale value, rather than merely rewriting the
	// file. Without this the case would pass over a writer that wrote the
	// record back unchanged.
	if !strings.Contains(command.Stdout, "re-sealed 1 shifted verdict(s); 0 refused") {
		t.Errorf("the page does not count the re-stamp: %q", command.Stdout)
	}
}

func TestRFCBothHalvesRefuseTheSameUnresealableStates(t *testing.T) {
	cases := []struct {
		name   string
		record string
		want   string
	}{
		{
			// The unit itself moved: a real change to what was judged.
			name: "a stale unit",
			record: rfcPyAudit(rfcPyVerdict("RFC9999-2-1", rfcPyReqSHA1,
				rfcPyPair(rfcPyGoFileSHA),
				rfcPyPair(rfcPyOtherSHA))),
			want: "refused rfc9999 RFC9999-2-1: stale-unit, a human must re-read it",
		},
		{
			// The obligation's own text moved, which invalidates every
			// judgement under it and is checked before anything else.
			name: "a stale requirement",
			record: rfcPyAudit(rfcPyVerdict("RFC9999-2-1", rfcPyOtherSHA,
				rfcPyPair(rfcPyGoFileSHA),
				rfcPyPair(rfcPyGoUnitSHA))),
			want: "refused rfc9999 RFC9999-2-1: stale-requirement, a human must re-read it",
		},
		{
			// No units recorded: the pre-units file-level rule, which has no
			// shifted state at all, so a moved file is stale and stays stale.
			name: "a pre-units record whose file moved",
			record: rfcPyAudit(rfcPyVerdict("RFC9999-2-1", rfcPyReqSHA1,
				rfcPyPair(rfcPyOtherSHA), "")),
			want: "refused rfc9999 RFC9999-2-1: stale-unit, a human must re-read it",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			command := rfcPyReseal(t, one.name, rfcPyAuditTree(one.record))
			if !strings.Contains(command.Stdout, one.want) {
				t.Errorf("the page does not carry %q:\n%s", one.want, command.Stdout)
			}
			if !strings.Contains(command.Stdout, "nothing to re-seal") {
				t.Errorf("a refused verdict was re-stamped anyway:\n%s", command.Stdout)
			}
		})
	}
}

func TestRFCBothHalvesLeaveAFreshVerdictAlone(t *testing.T) {
	cases := []struct {
		name   string
		record string
	}{
		{
			name: "every fingerprint current",
			record: rfcPyAudit(rfcPyVerdict("RFC9999-2-1", rfcPyReqSHA1,
				rfcPyPair(rfcPyGoFileSHA),
				rfcPyPair(rfcPyGoUnitSHA))),
		},
		{
			// The transitional rule: no units, and the file-level map agrees.
			name: "a pre-units record whose files are current",
			record: rfcPyAudit(rfcPyVerdict("RFC9999-2-1", rfcPyReqSHA1,
				rfcPyPair(rfcPyGoFileSHA), "")),
		},
		{
			// A second RFC's row, tagged in a .ci: the file is the unit, so
			// both maps hold the same value.
			name: "a file-scoped verdict",
			record: rfcPyAudit(rfcPyVerdict("RFC9999-2-2", rfcPyReqSHA2,
				`"`+rfcPyCIKey+`": "`+rfcPyCISHA+`"`,
				`"`+rfcPyCIKey+`": "`+rfcPyCISHA+`"`)),
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			command := rfcPyReseal(t, one.name, rfcPyAuditTree(one.record))
			if !strings.Contains(command.Stdout, "nothing to re-seal: no verdict is in the 'shifted' state (0 refused)") {
				t.Errorf("a fresh verdict was not left alone:\n%s", command.Stdout)
			}
		})
	}
}

func TestRFCBothHalvesRefuseTheSameMalformedAuditRecords(t *testing.T) {
	valid := rfcPyPair(rfcPyGoFileSHA)
	cases := []struct {
		name   string
		record string
		want   string
	}{
		{
			name:   "a document that is not an object",
			record: "[]\n",
			want:   "rfc/audit/rfc9999.json: expected a JSON object, got list",
		},
		{
			name:   "a key nobody reads",
			record: `{"rfc": "rfc9999", "audited": "x", "requirements": {}, "reviewer": "typo"}`,
			want:   "unknown key(s) ['reviewer']",
		},
		{
			name:   "a record that names another RFC",
			record: `{"rfc": "rfc7606", "audited": "x", "requirements": {}}`,
			want:   "'rfc' is 'rfc7606' but the filename says 'rfc9999'",
		},
		{
			name:   "requirements that are not an object",
			record: `{"rfc": "rfc9999", "audited": "x", "requirements": []}`,
			want:   "'requirements' must be an object",
		},
		{
			name:   "a history that is not a list of strings",
			record: `{"rfc": "rfc9999", "audited": "x", "requirements": {}, "reaudit_history": [1]}`,
			want:   "'reaudit_history' must be a list of strings",
		},
		{
			name: "a verdict outside the closed vocabulary",
			record: `{"rfc": "rfc9999", "audited": "x", "requirements": {"RFC9999-2-1": {
 "verdict": "implemented", "note": "n", "requirement_sha": "` + rfcPyReqSHA1 + `"}}}`,
			want: "has verdict 'implemented', which is not one of",
		},
		{
			name: "a fingerprint that is not a fingerprint",
			record: `{"rfc": "rfc9999", "audited": "x", "requirements": {"RFC9999-2-1": {
 "verdict": "enforced", "note": "n", "requirement_sha": "nope"}}}`,
			want: "'nope' is not a fingerprint",
		},
		{
			name: "a fingerprint map that is not a map",
			record: `{"rfc": "rfc9999", "audited": "x", "requirements": {"RFC9999-2-1": {
 "verdict": "enforced", "note": "n", "requirement_sha": "` + rfcPyReqSHA1 + `",
 "tests": "` + rfcPyGoFileSHA + `"}}}`,
			want: "'tests' must be an object, got str",
		},
		{
			name: "a key in the retired path:line form",
			record: `{"rfc": "rfc9999", "audited": "x", "requirements": {"RFC9999-2-1": {
 "verdict": "enforced", "note": "n", "requirement_sha": "` + rfcPyReqSHA1 + `",
 "tests": {"internal/widget/widget_test.go:3": "` + rfcPyGoFileSHA + `"}}}}`,
			want: "is the retired '<path>:<line>' form",
		},
		{
			name: "a key naming a path outside the repository",
			record: `{"rfc": "rfc9999", "audited": "x", "requirements": {"RFC9999-2-1": {
 "verdict": "enforced", "note": "n", "requirement_sha": "` + rfcPyReqSHA1 + `",
 "tests": {"../../etc/passwd": "` + rfcPyGoFileSHA + `"}}}}`,
			want: "names a path outside the repository",
		},
		{
			name: "a no_code_path on a verdict that does not mean it",
			record: `{"rfc": "rfc9999", "audited": "x", "requirements": {"RFC9999-2-1": {
 "verdict": "enforced", "note": "n", "requirement_sha": "` + rfcPyReqSHA1 + `",
 "tests": {` + valid + `}, "no_code_path": "none"}}}`,
			want: "carries 'no_code_path' with verdict 'enforced'",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			tree := rfcPyTree(t, rfcPyAuditTree(one.record))
			script := rfcPyRunScript(t, tree, "--reseal")
			devPyPointAt(t, tree)
			command := rfcPyActionCommand(t, "reseal")

			devPyAgree(t, one.name, script, command,
				rfcPyJSONReasonRE.ReplaceAllString(rfcPyGateText(script), "cannot read: why"),
				rfcPyJSONReasonRE.ReplaceAllString(rfcPyGateText(command), "cannot read: why"))
			if command.Code != 2 {
				t.Fatalf("a malformed record exited %d, want 2", command.Code)
			}
			if !strings.Contains(command.Stderr, one.want) {
				t.Errorf("the refusal does not name the defect %q:\n%s", one.want, command.Stderr)
			}
		})
	}
}

// rfcPyJSONReasonRE normalises the decoder's own reason for refusing a
// document. That reason is the reader's, not the gate's: Python names a line
// and a column, and Go names the token it did not expect. Everything around
// it -- the path, the frame, the refusal -- is compared byte for byte.
var rfcPyJSONReasonRE = regexp.MustCompile(`cannot read: .*`)

// rfcPyShift is the line appended to a tagged file to move it without moving
// what any of its functions say. It sits after the last closing brace, so every
// unit's text is unchanged and only the whole-file fingerprint moves.
const rfcPyShift = "\n// a line the parity test appended, below every function\n"

func TestRFCBothHalvesResealTheCheckoutIntoTheSameBytes(t *testing.T) {
	root := devPyRoot(t)

	// Two exports of HEAD, so neither half reads a tree the other wrote and
	// neither reads this working tree, which several sessions are editing.
	scriptTree := rfcPyExportHead(t, root)
	commandTree := rfcPyExportHead(t, root)

	// HEAD's one audit record is entirely fresh, so a re-seal over it writes
	// nothing and would prove nothing. Shifting every tagged file that record
	// cites puts all 50 of its unit-bearing verdicts into the shifted state at
	// once, which is the case the writer exists for and the largest one this
	// repository can produce.
	shifted := rfcPyShiftTaggedFiles(t, scriptTree)
	if len(shifted) < 10 {
		t.Fatalf("only %d tagged file(s) were shifted; HEAD's audit record cites more", len(shifted))
	}
	for _, rel := range shifted {
		rfcPyAppend(t, commandTree, rel)
	}

	script := rfcPyRunScript(t, scriptTree, "--reseal")
	if script.Code != 0 {
		t.Fatalf("the script exited %d over the export: %s%s", script.Code, script.Stdout, script.Stderr)
	}
	if !strings.Contains(rfcPyGateText(script), "re-sealed ") {
		t.Fatalf("the shift did not move any verdict into the shifted state:\n%s", script.Stdout)
	}
	report, err := rfc.Reseal(commandTree)
	if err != nil {
		t.Fatalf("Reseal over the export: %v", err)
	}
	if report.Text() != rfcPyGateText(script) {
		t.Errorf("the two halves print different pages\nscript:\n%s\ncommand:\n%s",
			rfcPyGateText(script), report.Text())
	}

	wrote := rfcPyAuditBytes(t, scriptTree)
	got := rfcPyAuditBytes(t, commandTree)
	if len(wrote) == 0 {
		t.Fatal("the export carries no audit record, so nothing was compared")
	}
	for name, body := range wrote {
		if got[name] != body {
			t.Errorf("the two halves wrote different bytes into %s\nscript:\n%s\ncommand:\n%s",
				name, body, got[name])
		}
	}
}

// rfcPyExportHead writes a clean export of HEAD into a temporary directory.
//
// HEAD rather than the working tree: several sessions share this checkout, so
// the working tree moves under a run, and a writer compared across two trees
// needs both to be the same tree.
func rfcPyExportHead(t *testing.T, root string) string {
	t.Helper()

	tree := t.TempDir()
	archive := exec.CommandContext(t.Context(), "git", "-C", root, "archive", "HEAD") // #nosec G204 -- a tracked checkout
	packed, err := archive.Output()
	if err != nil {
		t.Fatalf("git archive HEAD: %v", err)
	}
	unpack := exec.CommandContext(t.Context(), "tar", "-x", "-C", tree) // #nosec G204 -- a path this test made
	unpack.Stdin = bytes.NewReader(packed)
	if out, err := unpack.CombinedOutput(); err != nil {
		t.Fatalf("unpacking the export: %v: %s", err, out)
	}
	return tree
}

// rfcPyShiftTaggedFiles appends a line below every function of every file the
// export's audit records cite, and answers the paths it touched.
func rfcPyShiftTaggedFiles(t *testing.T, tree string) []string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(tree, "rfc", "audit"))
	if err != nil {
		t.Fatalf("reading the export's audit directory: %v", err)
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(tree, "rfc", "audit", entry.Name())) // #nosec G304 -- a path this test made
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		var record struct {
			Requirements map[string]struct {
				Tests map[string]string `json:"tests"`
			} `json:"requirements"`
		}
		if err := json.Unmarshal(body, &record); err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		for _, verdict := range record.Requirements {
			for key := range verdict.Tests {
				rel, _, _ := strings.Cut(key, "::")
				rel, _, _ = strings.Cut(rel, "#")
				seen[rel] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for rel := range seen {
		out = append(out, rel)
	}
	sort.Strings(out)
	for _, rel := range out {
		rfcPyAppend(t, tree, rel)
	}
	return out
}

// rfcPyAppend adds the shifting line to one file of a tree.
func rfcPyAppend(t *testing.T, tree, rel string) {
	t.Helper()

	path := filepath.Join(tree, filepath.FromSlash(rel))
	body, err := os.ReadFile(path) // #nosec G304 -- a path this test made
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	if err := os.WriteFile(path, append(body, rfcPyShift...), 0o600); err != nil {
		t.Fatalf("shifting %s: %v", rel, err)
	}
}

func TestTheScriptCrashesOnANullReauditHistory(t *testing.T) {
	// `reaudit_history: null` LOADS: the schema check reads the field as
	// "absent or a list of strings", and None satisfies the first half. The
	// writer then reaches for the key with a default, gets the null back rather
	// than the default, and appends to it -- which raises outside every try the
	// driver wraps itself in. The script aborts with a traceback and exit 1
	// where its own docstring promises exit 2, and it does so AFTER deciding to
	// re-stamp, so the record is left holding the stale fingerprints the run
	// was started to replace.
	//
	// The port reads a null history as the absent history it is and writes the
	// note into a fresh list. This case asserts BOTH halves, so it reddens the
	// day somebody repairs the script -- and the answer then is to delete it
	// with the row in plan/journal/analyzer-crashes-instead-of-reporting.md.
	record := `{
  "rfc": "rfc9999",
  "audited": "2026-08-26",
  "reaudit_note": "the note an earlier re-stamp left",
  "reaudit_history": null,
  "requirements": {` + rfcPyVerdict("RFC9999-2-1", rfcPyReqSHA1,
		rfcPyPair(rfcPyOtherSHA),
		rfcPyPair(rfcPyGoUnitSHA)) + `
  }
}
`
	tree := rfcPyTree(t, rfcPyAuditTree(record))
	script := rfcPyRunScript(t, tree, "--reseal")
	if script.Code != 1 || !strings.Contains(script.Stderr, "Traceback (most recent call last)") {
		t.Fatalf("the script no longer crashes (exit %d); delete this case and its journal row:\n%s%s",
			script.Code, script.Stdout, script.Stderr)
	}

	command := rfcPyActionCommand(t, "reseal")
	if command.Code != 0 {
		t.Fatalf("the command exited %d over a null history, want 0: %s%s",
			command.Code, command.Stdout, command.Stderr)
	}
	if !strings.Contains(command.Stdout, "re-stamped rfc9999 RFC9999-2-1") {
		t.Errorf("the command did not re-stamp the shifted verdict: %q", command.Stdout)
	}
	written, err := os.ReadFile(filepath.Join(tree, "rfc", "audit", "rfc9999.json")) // #nosec G304 -- a path this test made
	if err != nil {
		t.Fatalf("reading what the command wrote: %v", err)
	}
	var reread struct {
		History []string `json:"reaudit_history"`
		Note    string   `json:"reaudit_note"`
	}
	if err := json.Unmarshal(written, &reread); err != nil {
		t.Fatalf("the command wrote a record nothing can read: %v: %s", err, written)
	}
	if len(reread.History) != 1 || reread.History[0] != "the note an earlier re-stamp left" {
		t.Errorf("the earlier note was not preserved into history: %v", reread.History)
	}
	if !strings.Contains(reread.Note, "Mechanical re-stamp") {
		t.Errorf("the new note was not recorded: %q", reread.Note)
	}
	// The script crashed before writing, so its own tree still holds the stale
	// value. Naming it here is what makes the defect's cost visible rather than
	// merely its symptom.
	stale, err := os.ReadFile(filepath.Join(tree, "rfc", "audit", "rfc9999.json")) // #nosec G304 -- a path this test made
	if err != nil {
		t.Fatalf("re-reading the record: %v", err)
	}
	if strings.Contains(string(stale), rfcPyOtherSHA) {
		t.Errorf("the command left the stale fingerprint behind: %s", stale)
	}
}
