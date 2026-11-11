// VALIDATES: AC-4 through the commit seam's judge: a commit that adds a
// problem site with no doctor code is refused, naming the file and line and
// the cover; a commit that adds a covered site is admitted; an uncovered site
// the parent already held is not refused (report lists it); an empty change
// set is admitted without reading either side; and a thin scan is an error.
// It also validates spec-doctor-coverage-12-plugin-infra AC-43 (a Warn whose
// message is not a literal, and a call through a wrapper that forwards its
// message and attributes to Warn, are sites judged at the caller) and AC-46's
// check form (a plugin's rpc.DoctorCheckDiagnostic with no Fix is refused).
// PREVENTS: `./le commit create` landing a Warn line no doctor check answers
// for, and a commit refused for the uncovered sites the tree already holds.
package archdoctorcoverage

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

// fixtureAnchor is the fixture line a commit side adds a new line after.
const fixtureAnchor = "\tlog.Warn(\"fixture: no code\", \"peer\", peer)\n"

// fixtureSide reads the fixture tree from disk, with old replaced by
// replacement in path.
func fixtureSide(t *testing.T, path, old, replacement string) changecheck.Side {
	t.Helper()
	return func(read string) (string, bool, error) {
		content, err := os.ReadFile(filepath.Join(fixtureTree, filepath.FromSlash(read)))
		if err != nil {
			return "", false, err
		}
		text := string(content)
		if read == path {
			if !strings.Contains(text, old) {
				t.Fatalf("fixture %s holds no %q", read, old)
			}
			text = strings.Replace(text, old, replacement, 1)
		}
		return text, true, nil
	}
}

// addingLine is a commit side whose sites.go holds line after the anchor,
// and the line number line lands on.
func addingLine(t *testing.T, line string) (changecheck.Side, int) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(fixtureTree, filepath.FromSlash(fixtureSites)))
	if err != nil {
		t.Fatal(err)
	}
	before, _, found := bytes.Cut(content, []byte(fixtureAnchor))
	if !found {
		t.Fatalf("fixture %s holds no %q", fixtureSites, fixtureAnchor)
	}
	lineNumber := bytes.Count(before, []byte("\n")) + 2
	return fixtureSide(t, fixtureSites, fixtureAnchor, fixtureAnchor+"\t"+line+"\n"), lineNumber
}

// TestDoctorCoverageCommitRefusesAddedUncoveredSite holds AC-4's commit
// judge: a commit adding a Warn line with no code to an internal/plugins file
// is refused naming file:line and the cover, and the refusal names none of
// the uncovered sites the parent already held.
func TestDoctorCoverageCommitRefusesAddedUncoveredSite(t *testing.T) {
	after, line := addingLine(t, `log.Warn("fixture: added uncovered", "peer", peer)`)
	change := changecheck.Change{
		Root:   fixtureTree,
		Paths:  []string{fixtureSites},
		Before: fixtureSide(t, "", "", ""),
		After:  after,
	}
	text, err := judgeChangeAt(change, fixtureCodesFloor, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	where := fixtureSites + ":" + strconv.Itoa(line)
	for _, want := range []string{where, "fixture: added uncovered", "no code attribute", "\nFix:", `"code"`, "report.RaiseError", "./le arch doctor-coverage report"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal does not name %q:\n%s", want, text)
		}
	}
	for _, held := range []string{"fixture: no code\"", "fixture: dynamic code", "fixture: orphan code", "fixture: raise uncovered", "fixture: package logger"} {
		if strings.Contains(text, held) {
			t.Errorf("refusal names %q, a site the parent already held:\n%s", held, text)
		}
	}
}

// TestDoctorCoverageCommitAdmitsAddedCoveredSite holds that a commit adding a
// Warn line whose code a doctor check in the tree declares is admitted.
func TestDoctorCoverageCommitAdmitsAddedCoveredSite(t *testing.T) {
	after, _ := addingLine(t, `log.Warn("fixture: added covered", "code", diagnostic.CodeDoctorFixtureCovered)`)
	change := changecheck.Change{
		Root:   fixtureTree,
		Paths:  []string{fixtureSites},
		Before: fixtureSide(t, "", "", ""),
		After:  after,
	}
	text, err := judgeChangeAt(change, fixtureCodesFloor, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	if text != "" {
		t.Errorf("a commit adding a covered site was refused:\n%s", text)
	}
}

// addedCheckFile is a file the commit fixture adds with a doctor check in it,
// in a package no file of the tree imports. Only the After side holds it.
const addedCheckFile = "internal/component/added/doctor.go"

// addedCheck is addedCheckFile's text: one check declaring one code.
const addedCheck = "package added\n\n" +
	"import \"github.com/ze-software/ze/internal/core/diagnostic\"\n\n" +
	"var addedDoctorCheck = diagnostic.DoctorCheck{\n" +
	"\tName:  \"fixture-added-check\",\n" +
	"\tCodes: []string{\"doctor-fixture-added\"},\n" +
	"}\n"

// TestDoctorCoverageCommitAdmitsCheckAddedWithItsSite holds that a commit
// adding a doctor check and the first site naming its code, together, is
// admitted: the After side's covered set is read from the tree the commit
// leaves, which declares the check. The same site committed without the check
// is refused, because no check in that tree declares its code.
func TestDoctorCoverageCommitAdmitsCheckAddedWithItsSite(t *testing.T) {
	withSite, line := addingLine(t, `log.Warn("fixture: added with its check", "code", "doctor-fixture-added")`)
	unchanged := fixtureSide(t, "", "", "")
	after := func(path string) (string, bool, error) {
		if path == addedCheckFile {
			return addedCheck, true, nil
		}
		return withSite(path)
	}
	before := func(path string) (string, bool, error) {
		if path == addedCheckFile {
			return "", false, nil
		}
		return unchanged(path)
	}
	together := changecheck.Change{Root: fixtureTree, Paths: []string{fixtureSites, addedCheckFile}, Before: before, After: after}
	text, err := judgeChangeAt(together, fixtureCodesFloor, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	if text != "" {
		t.Errorf("a commit adding a doctor check and its first site was refused:\n%s", text)
	}

	alone := changecheck.Change{Root: fixtureTree, Paths: []string{fixtureSites}, Before: unchanged, After: withSite}
	text, err = judgeChangeAt(alone, fixtureCodesFloor, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	where := fixtureSites + ":" + strconv.Itoa(line)
	for _, want := range []string{where, "fixture: added with its check", "no doctor check in the tree declares its code doctor-fixture-added"} {
		if !strings.Contains(text, want) {
			t.Errorf("a site whose code no check declares: refusal does not name %q:\n%s", want, text)
		}
	}
}

// TestDoctorCoverageCommitLeavesHeldSitesToReport holds that a commit
// carrying a file whose uncovered sites the parent already held is admitted,
// while report still lists those sites: the tree's sites are the audit's.
// A commit that strips the code from a covered site adds an uncovered one.
func TestDoctorCoverageCommitLeavesHeldSitesToReport(t *testing.T) {
	unchanged := fixtureSide(t, "", "", "")
	change := changecheck.Change{Root: fixtureTree, Paths: []string{fixtureSites}, Before: unchanged, After: unchanged}
	text, err := judgeChangeAt(change, fixtureCodesFloor, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	if text != "" {
		t.Errorf("a commit adding no site was refused for the sites its parent held:\n%s", text)
	}
	listed := treeReport(scanFixture(t)).Text()
	if !strings.Contains(listed, `"fixture: no code"`) {
		t.Errorf("report does not list the held uncovered site:\n%s", listed)
	}

	stripped := fixtureSide(t, fixtureSites,
		`log.Warn("fixture: covered literal", "code", "doctor-fixture-covered")`,
		`log.Warn("fixture: covered literal")`)
	stripping := changecheck.Change{Root: fixtureTree, Paths: []string{fixtureSites}, Before: unchanged, After: stripped}
	text, err = judgeChangeAt(stripping, fixtureCodesFloor, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	if !strings.Contains(text, "fixture: covered literal") {
		t.Errorf("a commit removing a site's code was admitted:\n%s", text)
	}
}

// TestDoctorCoverageCommitAdmitsEmptyChange holds that a change set carrying
// no shipped Go file is admitted without reading a side,
// and that a carried file below a thin scan is an error, never an admit.
func TestDoctorCoverageCommitAdmitsEmptyChange(t *testing.T) {
	refuseRead := func(path string) (string, bool, error) {
		t.Errorf("an empty change set read %s", path)
		return "", false, nil
	}
	for _, paths := range [][]string{nil, {"docs/guide/x.md", "internal/plugins/fixture/sites_test.go"}} {
		change := changecheck.Change{Root: fixtureTree, Paths: paths, Before: refuseRead, After: refuseRead}
		text, err := judgeChangeAt(change, fixtureCodesFloor, 1)
		if err != nil {
			t.Fatalf("judgeChangeAt %v: %v", paths, err)
		}
		if text != "" {
			t.Errorf("change set %v was refused:\n%s", paths, text)
		}
	}

	unchanged := fixtureSide(t, "", "", "")
	clean := changecheck.Change{Root: fixtureTree, Paths: []string{fixtureSites}, Before: unchanged, After: unchanged}
	if _, err := judgeChangeAt(clean, fixtureCodesFloor, 1000); err == nil {
		t.Error("a commit judged over a thin scan was admitted")
	}
}

// lineOf answers the 1-based line of text holding needle, failing the test
// unless exactly one line holds it.
func lineOf(t *testing.T, text, needle string) int {
	t.Helper()
	if strings.Count(text, needle) != 1 {
		t.Fatalf("fixture holds %q %d times, want once", needle, strings.Count(text, needle))
	}
	before, _, _ := strings.Cut(text, needle)
	return strings.Count(before, "\n") + 1
}

// wrapperFile is a file the AC-43 commit fixture adds beside the fixture's
// sites, in the same package. Only the After side holds it.
const wrapperFile = "internal/plugins/fixture/wrapper.go"

// wrapperSource is wrapperFile's text: two wrappers that forward their message
// and attributes to Warn (one through the other, the dhcpserver warnLimited
// chain), one that forwards to Error, one that prefixes the message (the dhcp
// v6 warnOutage shape), and calls of every shape AC-43 judges, beside the
// shapes that stay no site.
const wrapperSource = `package fixture

import "log/slog"

// limiter writes rate-limited warnings.
type limiter struct{ log *slog.Logger }

// reply is a value whose Error method is no logger.
type reply struct{}

// Error records err.
func (reply) Error(err error) {}

// warnLimited forwards its message and attributes to Warn.
func warnLimited(log *slog.Logger, msg string, attrs ...any) {
	log.Warn(msg, attrs...)
}

// warnUnserved forwards to warnLimited, a wrapper of a wrapper.
func (l *limiter) warnUnserved(msg string, attrs ...any) {
	warnLimited(l.log, msg, attrs...)
}

// failLimited forwards its message and attributes to Error.
func failLimited(log *slog.Logger, msg string, attrs ...any) {
	log.Error(msg, attrs...)
}

// warnPrefixed forwards its message, prefixed, and attributes to Warn.
func warnPrefixed(log *slog.Logger, msg string, args ...any) {
	log.Warn("fixture prefix: "+msg, args...)
}

// run calls every shape.
func (l *limiter) run(warning string, level slog.Level, err error, r reply) {
	l.log.Warn("fixture: concatenated " + warning)
	message := "fixture: variable message"
	l.log.Warn(message)
	warnLimited(l.log, "fixture: wrapped uncovered", "peer", warning)
	l.warnUnserved("fixture: wrapped twice uncovered")
	failLimited(l.log, "fixture: wrapped error uncovered")
	warnPrefixed(l.log, "fixture: prefixed uncovered")
	warnLimited(l.log, "fixture: wrapped covered", "code", "doctor-fixture-covered")
	l.warnUnserved("fixture: wrapped twice covered", slog.String("code", "doctor-fixture-covered"))
	l.log.Log(nil, level, "fixture: level unspelled")
	r.Error(err)
}
`

// TestGateSeesWrapperAndNonLiteralWarn is AC-43. Goal: a Warn whose message
// is a concatenation or a variable with no attributes is a site, and a call
// through a function that forwards its message and attributes to Warn or
// Error (found from its body, at any depth of wrapping) is a site judged at
// the caller for the caller's code attribute, while the wrapper's own
// forwarding call is none. Method: a commit adds wrapperFile; the judge must
// refuse exactly the uncovered callers and the two non-literal Warns, and the
// scan of that side must read each caller's kind, call and code. A Log whose
// level the source does not spell, and a value's Error(err), stay no site.
func TestGateSeesWrapperAndNonLiteralWarn(t *testing.T) {
	unchanged := fixtureSide(t, "", "", "")
	before := func(path string) (string, bool, error) {
		if path == wrapperFile {
			return "", false, nil
		}
		return unchanged(path)
	}
	after := func(path string) (string, bool, error) {
		if path == wrapperFile {
			return wrapperSource, true, nil
		}
		return unchanged(path)
	}
	change := changecheck.Change{Root: fixtureTree, Paths: []string{wrapperFile}, Before: before, After: after}
	text, err := judgeChangeAt(change, fixtureCodesFloor, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	refused := []string{
		`l.log.Warn("fixture: concatenated " + warning)`,
		`l.log.Warn(message)`,
		`warnLimited(l.log, "fixture: wrapped uncovered"`,
		`l.warnUnserved("fixture: wrapped twice uncovered")`,
		`failLimited(l.log, "fixture: wrapped error uncovered")`,
		`warnPrefixed(l.log, "fixture: prefixed uncovered")`,
	}
	for _, needle := range refused {
		where := wrapperFile + ":" + strconv.Itoa(lineOf(t, wrapperSource, needle)) + ":"
		if !strings.Contains(text, where) {
			t.Errorf("refusal does not name %s (%s):\n%s", where, needle, text)
		}
	}
	admitted := []string{
		"\tlog.Warn(msg, attrs...)",
		"\twarnLimited(l.log, msg, attrs...)",
		"\tlog.Error(msg, attrs...)",
		`"fixture prefix: "+msg`,
		`"fixture: wrapped covered"`,
		`"fixture: wrapped twice covered"`,
		`"fixture: level unspelled"`,
		"\tr.Error(err)",
	}
	for _, needle := range admitted {
		where := wrapperFile + ":" + strconv.Itoa(lineOf(t, wrapperSource, needle)) + ":"
		if strings.Contains(text, where) {
			t.Errorf("refusal names %s (%s), which is no uncovered site:\n%s", where, needle, text)
		}
	}
	if got := strings.Count(text, wrapperFile+":"); got != len(refused) {
		t.Errorf("refusal names %d sites of %s, want %d:\n%s", got, wrapperFile, len(refused), text)
	}

	inventory, err := scan(fixtureTree, map[string]overlayFile{wrapperFile: {text: wrapperSource, present: true}}, fixtureCodesFloor, fixtureDoors)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	cases := []struct {
		message string
		kind    Kind
		call    string
		status  Status
	}{
		{`"fixture: concatenated " + warning`, KindLogWarn, "l.log.Warn", StatusNoCode},
		{"message", KindLogWarn, "l.log.Warn", StatusNoCode},
		{"fixture: wrapped uncovered", KindLogWarn, "warnLimited", StatusNoCode},
		{"fixture: wrapped twice uncovered", KindLogWarn, "l.warnUnserved", StatusNoCode},
		{"fixture: wrapped error uncovered", KindLogError, "failLimited", StatusNoCode},
		{"fixture: prefixed uncovered", KindLogWarn, "warnPrefixed", StatusNoCode},
		{"fixture: wrapped covered", KindLogWarn, "warnLimited", StatusCovered},
		{"fixture: wrapped twice covered", KindLogWarn, "l.warnUnserved", StatusCovered},
	}
	var inWrapper int
	for i := range inventory.Sites {
		if inventory.Sites[i].File == wrapperFile {
			inWrapper++
		}
	}
	if inWrapper != len(cases) {
		t.Errorf("scan found %d sites in %s, want %d: %+v", inWrapper, wrapperFile, len(cases), inventory.Sites)
	}
	for _, tc := range cases {
		site := siteWith(t, inventory.Sites, tc.message)
		if site.File != wrapperFile {
			t.Errorf("%q: file %s, want %s", tc.message, site.File, wrapperFile)
		}
		if site.Kind != tc.kind {
			t.Errorf("%q: kind %s, want %s", tc.message, site.Kind, tc.kind)
		}
		if site.Call != tc.call {
			t.Errorf("%q: call %q, want %q", tc.message, site.Call, tc.call)
		}
		if site.Status != tc.status {
			t.Errorf("%q: status %s, want %s", tc.message, site.Status, tc.status)
		}
		if tc.status == StatusCovered && site.Code != "doctor-fixture-covered" {
			t.Errorf("%q: code %q, want doctor-fixture-covered", tc.message, site.Code)
		}
	}
}

// pluginFindingCopp is a plugin answering its findings as rpc literals.
const pluginFindingCopp = "internal/plugins/copp/doctor.go"

// pluginFindingAnchor is the line of pluginFindingCopp a commit side adds a
// function before.
const pluginFindingAnchor = "// Anchor: a commit adds a function before this line.\n"

// pluginFindingTree writes a tree holding the plugin finding type
// rpc.DoctorCheckDiagnostic (and an unqualified literal of it in its own
// package), its SDK alias, a plugin answering findings through rpc and one
// through the alias, and answers its root.
func pluginFindingTree(t *testing.T) string {
	t.Helper()
	tree := t.TempDir()
	writeTreeFile(t, tree, "pkg/plugin/rpc/types.go", "package rpc\n\n"+
		"// DoctorCheckDiagnostic is a plugin's doctor finding.\n"+
		"type DoctorCheckDiagnostic struct {\n\tCode, Severity, Message, Fix string\n}\n\n"+
		"// Empty answers no finding.\n"+
		"func Empty() []DoctorCheckDiagnostic { return []DoctorCheckDiagnostic{} }\n\n"+
		"// Own answers a finding built in the declaring package.\n"+
		"func Own() DoctorCheckDiagnostic {\n"+
		"\treturn DoctorCheckDiagnostic{Message: \"rpc: own finding\", Fix: \"ze show doctor\"}\n}\n")
	writeTreeFile(t, tree, "pkg/plugin/sdk/sdk_types.go", "package sdk\n\n"+
		"import \"github.com/ze-software/ze/pkg/plugin/rpc\"\n\n"+
		"// DoctorCheckDiagnostic is the SDK alias of the plugin finding.\n"+
		"type DoctorCheckDiagnostic = rpc.DoctorCheckDiagnostic\n")
	writeTreeFile(t, tree, pluginFindingCopp, "package copp\n\n"+
		"import \"github.com/ze-software/ze/pkg/plugin/rpc\"\n\n"+
		"// Check answers the plugin's findings.\n"+
		"func Check() []rpc.DoctorCheckDiagnostic {\n"+
		"\treturn []rpc.DoctorCheckDiagnostic{\n"+
		"\t\t{Code: \"doctor-copp-missing\", Severity: \"error\", Message: \"copp: no fix\"},\n"+
		"\t\t{Code: \"doctor-copp-fixed\", Severity: \"error\", Message: \"copp: fix set\", Fix: \"set system copp enable\"},\n"+
		"\t}\n}\n\n"+pluginFindingAnchor)
	writeTreeFile(t, tree, "internal/plugins/static/doctor.go", "package static\n\n"+
		"import \"github.com/ze-software/ze/pkg/plugin/sdk\"\n\n"+
		"// Check answers the plugin's finding through the SDK alias.\n"+
		"func Check() *sdk.DoctorCheckDiagnostic {\n"+
		"\treturn &sdk.DoctorCheckDiagnostic{Code: \"doctor-static-missing\", Message: \"static: no fix\"}\n}\n\n"+
		"// Empty answers a finding whose Fix is the empty string.\n"+
		"func Empty() sdk.DoctorCheckDiagnostic {\n"+
		"\treturn sdk.DoctorCheckDiagnostic{Message: \"static: fix empty\", Fix: \"\"}\n}\n")
	return tree
}

// TestDoctorCoverageCheckRefusesPluginFindingWithoutFix is AC-46's check
// form. Goal: a commit adding an rpc.DoctorCheckDiagnostic literal with no
// Fix is refused as a diagnostic.Diagnostic with no Fix is, naming its file
// and line and the fix; one adding it with its Fix is admitted; the finding
// with no Fix the parent already held is never refused. Method: a commit
// adds a function to a plugin of pluginFindingTree.
func TestDoctorCoverageCheckRefusesPluginFindingWithoutFix(t *testing.T) {
	tree := pluginFindingTree(t)
	side := func(added string) changecheck.Side {
		return func(read string) (string, bool, error) {
			content, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(read)))
			if err != nil {
				return "", false, err
			}
			text := string(content)
			if read == pluginFindingCopp && added != "" {
				text = strings.Replace(text, pluginFindingAnchor, added+pluginFindingAnchor, 1)
			}
			return text, true, nil
		}
	}
	without := "func Added() rpc.DoctorCheckDiagnostic {\n" +
		"\treturn rpc.DoctorCheckDiagnostic{Code: \"doctor-copp-added\", Message: \"copp: added without fix\"}\n" +
		"}\n\n"
	content, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(pluginFindingCopp)))
	if err != nil {
		t.Fatal(err)
	}
	// The added function's literal opens on its second line.
	line := lineOf(t, string(content), pluginFindingAnchor) + 1
	change := changecheck.Change{Root: tree, Paths: []string{pluginFindingCopp}, Before: side(""), After: side(without)}
	text, err := judgeChangeAt(change, 0, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	where := pluginFindingCopp + ":" + strconv.Itoa(line)
	for _, want := range []string{where, "rpc.DoctorCheckDiagnostic", "copp: added without fix", fixFinding, "\nFix:", "./le arch doctor-coverage report"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal does not name %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "copp: no fix") {
		t.Errorf("refusal names a finding the parent already held:\n%s", text)
	}

	with := "func Added() rpc.DoctorCheckDiagnostic {\n" +
		"\treturn rpc.DoctorCheckDiagnostic{Message: \"copp: added with fix\", Fix: \"set system copp enable\"}\n" +
		"}\n\n"
	change.After = side(with)
	text, err = judgeChangeAt(change, 0, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	if text != "" {
		t.Errorf("a commit adding a plugin finding with its Fix was refused:\n%s", text)
	}
}
