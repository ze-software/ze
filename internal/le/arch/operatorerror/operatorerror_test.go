// VALIDATES: the operator-error scan derives its handler set from the
// registration sites, classifies each return site of a registered handler as
// typed, plain or pass-through, refuses a plain refusal only on a changed line
// or a commit's addition, and lists every site in report.
// PREVENTS: a handler refusal that names no fix returning unnoticed, and a
// gate that refuses the plain sites the tree already holds.
package archoperatorerror

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// fixtureTree is a tree of one plugin whose handlers cover every status, plus
// a test file and a tooling package that register handlers the scan skips.
const fixtureTree = "testdata/tree"

const (
	fixtureHandlers = "internal/plugins/fixture/handlers.go"
	fixtureSDK      = "internal/plugins/fixture/sdk.go"
)

// scanFixture scans the fixture tree.
func scanFixture(t *testing.T) Inventory {
	t.Helper()
	inventory, err := Scan(fixtureTree)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return inventory
}

// siteOf answers the one site whose handler and expression contain the given
// text, failing the test when there is not exactly one.
func siteOf(t *testing.T, sites []Site, handler, expr string) Site {
	t.Helper()
	var matched []Site
	for _, site := range sites {
		if site.Handler != handler {
			continue
		}
		if !strings.Contains(site.Expr, expr) {
			continue
		}
		matched = append(matched, site)
	}
	if len(matched) != 1 {
		t.Fatalf("sites of %s returning %q = %d, want 1: %+v", handler, expr, len(matched), sites)
	}
	return matched[0]
}

// everyLine is a change set that changed every line of the fixture files.
func everyLine() repochanged.ChangedLines {
	return repochanged.ChangedLines{
		fixtureHandlers:                        {{From: 1, To: 10000}},
		fixtureSDK:                             {{From: 1, To: 10000}},
		"internal/plugins/fixture/register.go": {{From: 1, To: 10000}},
	}
}

// TestOperatorErrorRefusesPlainErrorFromHandler is AC-2: a plain errors.New
// or fmt.Errorf returned from a function registered as an RPC handler, a
// handler a factory returns, a config verifier, or an SDK config-verify
// callback is refused on a changed line, and the refusal names the handler,
// the plain error, why and the fix.
func TestOperatorErrorRefusesPlainErrorFromHandler(t *testing.T) {
	inventory := scanFixture(t)
	report := checkLines(inventory.Sites, everyLine())
	if report.exitCode() != 1 {
		t.Fatalf("exit code = %d, want 1", report.exitCode())
	}
	want := []struct{ handler, expr string }{
		{"handlePlain", `errors.New("peer not found")`},
		{"handlePlain", `fmt.Errorf("peer %s refused"`},
		{"forward (returned func literal)", `errors.New("forward refused: "`},
		{"verifyFixture", `fmt.Errorf("fixture section invalid")`},
		{"run (func literal)", `errors.New("verify refused")`},
	}
	if len(report.Findings) != len(want) {
		t.Fatalf("findings = %d, want %d: %+v", len(report.Findings), len(want), report.Findings)
	}
	for _, expected := range want {
		site := siteOf(t, report.Findings, expected.handler, expected.expr)
		if site.Status != StatusPlain {
			t.Errorf("%s %s: status %s, want plain", expected.handler, expected.expr, site.Status)
		}
	}
	text := report.Text()
	for _, part := range []string{"handlePlain", `errors.New("peer not found")`, "Why: ", "Fix: return an opererr.Error", "./le arch operator-error report"} {
		if !strings.Contains(text, part) {
			t.Errorf("refusal does not say %q:\n%s", part, text)
		}
	}
	verifier := siteOf(t, report.Findings, "verifyFixture", "fmt.Errorf")
	if verifier.Kind != KindConfigVerifier {
		t.Errorf("verifyFixture kind = %s, want config-verifier", verifier.Kind)
	}
	sdkVerify := siteOf(t, report.Findings, "run (func literal)", "verify refused")
	if sdkVerify.Kind != KindSDKConfigVerify {
		t.Errorf("OnConfigVerify literal kind = %s, want sdk-config-verify", sdkVerify.Kind)
	}
}

// TestOperatorErrorAcceptsTypedError is AC-3: a handler returning an error the
// opererr package built (a call into it, a composite literal's address) is
// typed and never a finding.
func TestOperatorErrorAcceptsTypedError(t *testing.T) {
	inventory := scanFixture(t)
	for _, expected := range []struct{ handler, expr string }{
		{"handleTyped", `opererr.New("peer not found"`},
		{"handleTyped", `opererr.Wrap(`},
		{"handleTyped", `&opererr.Error{`},
		{"rpcFixture", `opererr.New("rpc refused"`},
	} {
		site := siteOf(t, inventory.Sites, expected.handler, expected.expr)
		if site.Status != StatusTyped {
			t.Errorf("%s %s: status %s, want typed", expected.handler, expected.expr, site.Status)
		}
	}
	report := checkLines(inventory.Sites, everyLine())
	for _, finding := range report.Findings {
		if finding.Handler == "handleTyped" {
			t.Errorf("a typed return was refused: %+v", finding)
		}
		if finding.Handler == "rpcFixture" {
			t.Errorf("a typed return was refused: %+v", finding)
		}
	}
}

// TestOperatorErrorClassifiesPassThrough holds the pass-through rule: a
// returned error variable or a callee's call is pass-through, listed and never
// refused, and a nil error is no site at all.
func TestOperatorErrorClassifiesPassThrough(t *testing.T) {
	inventory := scanFixture(t)
	passed := siteOf(t, inventory.Sites, "handlePassThrough", "err")
	if passed.Status != StatusPassThrough {
		t.Errorf("return nil, err: status %s, want pass-through", passed.Status)
	}
	command := siteOf(t, inventory.Sites, "run (func literal)", "work()")
	if command.Status != StatusPassThrough {
		t.Errorf("return \"\", nil, work(): status %s, want pass-through", command.Status)
	}
	if command.Kind != KindSDKCommand {
		t.Errorf("OnExecuteCommand literal kind = %s, want sdk-command-handler", command.Kind)
	}
	for _, site := range inventory.Sites {
		if site.Handler != "handlePassThrough" {
			continue
		}
		if site.Expr == "nil" {
			t.Errorf("a nil error return was listed as a site: %+v", site)
		}
	}
}

// TestOperatorErrorJudgesOnlyChangedLines holds the change-set scope: with
// only one plain return's line changed, the other plain return of the same
// handler, already in the tree, is not refused.
func TestOperatorErrorJudgesOnlyChangedLines(t *testing.T) {
	inventory := scanFixture(t)
	changedSite := siteOf(t, inventory.Sites, "handlePlain", "fmt.Errorf")
	oldSite := siteOf(t, inventory.Sites, "handlePlain", "errors.New")
	report := checkLines(inventory.Sites, repochanged.ChangedLines{
		fixtureHandlers: {{From: changedSite.Line, To: changedSite.Line}},
	})
	if len(report.Findings) != 1 {
		t.Fatalf("findings = %+v, want only line %d", report.Findings, changedSite.Line)
	}
	if report.Findings[0].Line != changedSite.Line {
		t.Errorf("finding at line %d, want %d", report.Findings[0].Line, changedSite.Line)
	}
	if report.Findings[0].Line == oldSite.Line {
		t.Errorf("the plain site already in the tree was refused: %+v", report.Findings[0])
	}
	if empty := checkLines(inventory.Sites, repochanged.ChangedLines{}); len(empty.Findings) != 0 {
		t.Errorf("an empty change set refused %+v", empty.Findings)
	}
}

// TestOperatorErrorReportListsEverySite holds report: every site with its
// status, file and line, handler and kind; the handler the scan cannot follow
// listed as unresolved; and no site from an unregistered function, a test
// file or development tooling.
func TestOperatorErrorReportListsEverySite(t *testing.T) {
	inventory := scanFixture(t)
	if len(inventory.Registrations) != 10 {
		t.Errorf("registrations = %d, want 10: %+v", len(inventory.Registrations), inventory.Registrations)
	}
	report := TreeReport{Registrations: len(inventory.Registrations), Sites: inventory.Sites, Unresolved: inventory.Unresolved}
	if got := report.count(StatusPlain); got != 5 {
		t.Errorf("plain = %d, want 5", got)
	}
	if got := report.count(StatusTyped); got != 4 {
		t.Errorf("typed = %d, want 4", got)
	}
	if got := report.count(StatusPassThrough); got != 3 {
		t.Errorf("pass-through = %d, want 3", got)
	}
	text := report.Text()
	plain := siteOf(t, inventory.Sites, "handlePlain", "errors.New")
	line := fixtureHandlers + ":" + strconv.Itoa(plain.Line) + " handlePlain (rpc-handler)"
	for _, part := range []string{"plain " + line, "typed ", "pass-through ", "handlers[0]", "unresolved"} {
		if !strings.Contains(text, part) {
			t.Errorf("report does not say %q:\n%s", part, text)
		}
	}
	for _, absent := range []string{"notRegistered", "handleTest", "handleTool", "inside a nested literal"} {
		if strings.Contains(text, absent) {
			t.Errorf("report lists %q, which no shipped registration reaches:\n%s", absent, text)
		}
	}
	if len(inventory.Unresolved) != 1 {
		t.Fatalf("unresolved = %+v, want the one handlers[0] registration", inventory.Unresolved)
	}
	sites, err := Sites(fixtureTree)
	if err != nil {
		t.Fatalf("Sites: %v", err)
	}
	if len(sites) != len(inventory.Sites) {
		t.Errorf("Sites = %d, Scan = %d", len(sites), len(inventory.Sites))
	}
}

// TestOperatorErrorReadsWireMethodConstants holds that a registration's wire
// method is read from a literal and from a constant, which is what the
// registry comparison test matches on.
func TestOperatorErrorReadsWireMethodConstants(t *testing.T) {
	inventory := scanFixture(t)
	wire := make(map[string]bool)
	for _, registration := range inventory.Registrations {
		wire[registration.WireMethod] = true
	}
	for _, want := range []string{"ze-fixture:plain", "ze-fixture:typed"} {
		if !wire[want] {
			t.Errorf("no registration read wire method %q: %+v", want, inventory.Registrations)
		}
	}
}

// TestOperatorErrorReadsALoopBuiltRegistration holds the loop shape
// (internal/plugins/host-cmd/cmd show_host.go): a registration literal
// appended in a range loop, its wire method computed from a literal head and
// the loop variable, is read, its inline handler judged, and its head kept so
// the registry comparison can match the runtime methods it produces.
func TestOperatorErrorReadsALoopBuiltRegistration(t *testing.T) {
	inventory := scanFixture(t)
	site := siteOf(t, inventory.Sites, "registerLoop (func literal)", "dispatchSection(section)")
	if site.Status != StatusPassThrough {
		t.Errorf("loop handler status %s, want pass-through", site.Status)
	}
	prefixes := 0
	for _, registration := range inventory.Registrations {
		if registration.WirePrefix == "ze-fixture:loop-" {
			prefixes++
		}
	}
	if prefixes != 1 {
		t.Errorf("registrations with wire prefix ze-fixture:loop- = %d, want 1: %+v", prefixes, inventory.Registrations)
	}
}

// TestReadByScanStillRefusesAMissedRegistration holds that the registry
// comparison does not weaken with the prefix match: a runtime method is
// covered by a whole wire method, or by a head the SAME package registers,
// and a method no registration covers, or one whose head another package
// registers, is reported missed.
func TestReadByScanStillRefusesAMissedRegistration(t *testing.T) {
	inventory := scanFixture(t)
	fixturePackage := modulePath + "/internal/plugins/fixture"
	cases := []struct {
		wire, registrar string
		read            bool
	}{
		{"ze-fixture:plain", fixturePackage, true},
		{"ze-fixture:loop-cpu", fixturePackage, true},
		{"ze-fixture:loop-cpu", modulePath + "/internal/plugins/other", false},
		{"ze-fixture:missing", fixturePackage, false},
		{"ze-other:show-cpu", fixturePackage, false},
	}
	for _, tc := range cases {
		if got := readByScan(inventory, tc.wire, tc.registrar); got != tc.read {
			t.Errorf("readByScan(%s, %s) = %v, want %v", tc.wire, tc.registrar, got, tc.read)
		}
	}
}

// TestOperatorErrorRefusesAThinScan holds the floor: a scan that found fewer
// registrations than the floor is an error naming the fix, never a clean run.
func TestOperatorErrorRefusesAThinScan(t *testing.T) {
	inventory := scanFixture(t)
	err := checkFloor(inventory, 11)
	if err == nil {
		t.Fatal("a scan of 10 registrations passed a floor of 11")
	}
	if !strings.Contains(err.Error(), "found 10, at least 11 expected") {
		t.Errorf("floor refusal does not count: %v", err)
	}
	if err := checkFloor(inventory, 10); err != nil {
		t.Errorf("a scan at the floor was refused: %v", err)
	}
}

// fixtureSide reads the fixture file from disk, with replace applied to path.
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

// TestOperatorErrorCommitJudgesOnlyAdditions holds the commit seam's judge: a
// commit that turns a pass-through into a plain refusal is refused, naming
// the new site and not the plain sites its parent held; a commit that adds
// none is admitted.
func TestOperatorErrorCommitJudgesOnlyAdditions(t *testing.T) {
	unchanged := fixtureSide(t, "", "", "")
	adding := changecheck.Change{
		Root:   fixtureTree,
		Paths:  []string{fixtureHandlers},
		Before: unchanged,
		After:  fixtureSide(t, fixtureHandlers, "return nil, err\n", "return nil, errors.New(\"work failed\")\n"),
	}
	text, err := judgeChangeAt(adding, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	if !strings.Contains(text, `handlePassThrough (rpc-handler registered at`) {
		t.Errorf("refusal does not name the added site:\n%s", text)
	}
	if !strings.Contains(text, `errors.New("work failed")`) {
		t.Errorf("refusal does not name the plain error:\n%s", text)
	}
	if strings.Contains(text, `peer not found`) {
		t.Errorf("refusal names a plain site the parent already held:\n%s", text)
	}

	clean := changecheck.Change{Root: fixtureTree, Paths: []string{fixtureHandlers}, Before: unchanged, After: unchanged}
	text, err = judgeChangeAt(clean, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	if text != "" {
		t.Errorf("a commit adding no plain refusal was refused:\n%s", text)
	}

	if _, err := judgeChangeAt(clean, 100); err == nil {
		t.Error("a commit judged over a thin scan was admitted")
	}
}

// TestOperatorErrorRegistersIntoTheCommitSeam holds AC-4's commit route: the
// package registers its check into the seam `./le commit create` runs.
func TestOperatorErrorRegistersIntoTheCommitSeam(t *testing.T) {
	for _, check := range changecheck.Registered() {
		if check.Name == area+" check" {
			return
		}
	}
	t.Fatalf("%s check is not registered in the commit seam", area)
}
