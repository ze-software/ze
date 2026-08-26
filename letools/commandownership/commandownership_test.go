// Related: commandownership.go -- the gate these tests drive from its entry point
//
// Every test here calls the tool as a function. The gate used to be reachable
// only as a subprocess, so a violation this checkout does not hold could not be
// asserted at all: the whole suite was one case, "the tree passes".

package commandownership

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree writes files under a temporary directory and answers it.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return dir
}

// kinds answers the Kind of each finding, in order, which is what a case
// asserts when the message text is not the point.
func kinds(findings Findings) []string {
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		out = append(out, finding.Kind)
	}
	return out
}

// VALIDATES: an owner command package importing cmd/ze is a finding, and one
// importing anything else is not.
// PREVENTS: an owner that cannot be reached without the process entry point,
// which is invariant 1 and the reason the gate exists.
func TestAnOwnerImportingCmdZeIsAFinding(t *testing.T) {
	dir := tree(t, map[string]string{
		"internal/bgp/cli/register.go":    "package cli\n\nimport _ \"github.com/ze-software/ze/cmd/ze/internal/thing\"\n",
		"internal/ntp/cli/register.go":    "package cli\n\nimport _ \"github.com/ze-software/ze/internal/core/env\"\n",
		"internal/ldp/client/register.go": "package client\n\nimport _ \"github.com/ze-software/ze/cmd/ze\"\n",
		"internal/ldp/other/register.go":  "package other\n\nimport _ \"github.com/ze-software/ze/cmd/ze\"\n",
	})

	findings, err := Check(dir, 0)
	if err != nil {
		t.Fatalf("check the fixture: %v", err)
	}
	if got := kinds(findings); len(got) != 2 {
		t.Fatalf("the fixture draws %v, want two owner-imports-cmd-ze findings: %v", got, findings)
	}
	for _, finding := range findings {
		if finding.Kind != "owner-imports-cmd-ze" {
			t.Errorf("finding kind is %q, want owner-imports-cmd-ze", finding.Kind)
		}
	}
	if findings[0].File != "internal/bgp/cli/register.go" || findings[1].File != "internal/ldp/client/register.go" {
		t.Errorf("the findings name %s and %s, want the cli and client owners in sorted order", findings[0].File, findings[1].File)
	}
}

// VALIDATES: a root handler registered from cmd/ze is a finding unless the root
// is allowlisted, and a variant build tag exempts the file.
// PREVENTS: an owner-backed command registered centrally, which is invariant 2.
func TestARootHandlerInCmdZeIsAFindingUnlessExempt(t *testing.T) {
	cases := map[string]struct {
		body string
		want int
	}{
		"an unallowlisted root": {"package main\n\nfunc f() { registry.MustRegisterRootHandler(\"bgp\", nil, m) }\n", 1},
		"an allowlisted root":   {"package main\n\nfunc f() { registry.MustRegisterRootHandler(\"version\", nil, m) }\n", 0},
		"the alias spelling":    {"package main\n\nfunc f() { cmdregistry.RegisterRootHandler(\"bgp\", nil, m) }\n", 1},
		"a ze_test variant":     {"//go:build ze_test\n\npackage main\n\nfunc f() { registry.MustRegisterRootHandler(\"bgp\", nil, m) }\n", 0},
		"a ze_chaos variant":    {"//go:build ze_chaos\n\npackage main\n\nfunc f() { registry.MustRegisterRootHandler(\"bgp\", nil, m) }\n", 0},
		"another package":       {"package main\n\nfunc f() { other.MustRegisterRootHandler(\"bgp\", nil, m) }\n", 0},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := tree(t, map[string]string{"cmd/ze/x.go": tc.body})
			findings, err := Check(dir, 0)
			if err != nil {
				t.Fatalf("check the fixture: %v", err)
			}
			if len(findings) != tc.want {
				t.Fatalf("the fixture draws %d findings, want %d: %v", len(findings), tc.want, findings)
			}
			if tc.want > 0 && findings[0].Kind != "root-handler-in-cmd-ze" {
				t.Errorf("finding kind is %q, want root-handler-in-cmd-ze", findings[0].Kind)
			}
		})
	}
}

// VALIDATES: a metadata-only root registered centrally must be in the
// allowlist, and an allowlisted root that an owner already registers must not.
// PREVENTS: invariants 3 and its converse -- a central root nobody declared,
// and an exemption left behind after the migration it covered.
func TestTheAllowlistIsCheckedBothWays(t *testing.T) {
	dir := tree(t, map[string]string{
		"cmd/ze/roots.go": "package main\n\nfunc f() {\n\tregistry.RegisterRoot(\"help\", m)\n\tregistry.RegisterRoot(\"mystery\", m)\n}\n",
	})
	findings, err := Check(dir, 0)
	if err != nil {
		t.Fatalf("check the fixture: %v", err)
	}
	if len(findings) != 1 || findings[0].Kind != "root-not-allowlisted" {
		t.Fatalf("the fixture draws %v, want one root-not-allowlisted for the undeclared root", findings)
	}
	if !strings.Contains(findings[0].Msg, `"mystery"`) {
		t.Errorf("the message is %q, want it to name the undeclared root", findings[0].Msg)
	}

	owned := tree(t, map[string]string{
		"internal/doctor/cli/register.go": "package cli\n\nfunc f() { registry.MustRegisterRootHandler(\"doctor\", nil, m) }\n",
	})
	findings, err = Check(owned, 0)
	if err != nil {
		t.Fatalf("check the fixture: %v", err)
	}
	if len(findings) != 1 || findings[0].Kind != "allowlisted-owned-root" {
		t.Fatalf("the fixture draws %v, want one allowlisted-owned-root for a root that now has an owner", findings)
	}
	if findings[0].File != "internal/doctor/cli/register.go" {
		t.Errorf("the finding names %s, want the owner that registers it", findings[0].File)
	}
}

// VALIDATES: a Go file the gate cannot parse stops the run.
// PREVENTS: the fail-open this gate exists to prevent, applied to itself. The
// script's parser errors are discarded, so a cmd/ze file with a syntax error
// registers no root at all and the gate reports OK over it
// (scripts/checks/parity_test.go, TestOwnershipScriptStillPassesOverWhatItCannotParse).
func TestAFileThatWillNotParseStopsTheRun(t *testing.T) {
	dir := tree(t, map[string]string{
		"cmd/ze/broken.go": "package main\n\nfunc f() { registry.RegisterRoot(\"mystery\"\n",
	})
	if _, err := Check(dir, 0); err == nil {
		t.Error("a cmd/ze file that will not parse did not stop the run, so an unreadable root is a clean tree")
	}

	owner := tree(t, map[string]string{
		"internal/x/cli/register.go": "package cli\n\nimport _ \"github.com/ze-software/ze/cmd/ze\n",
	})
	if _, err := Check(owner, 0); err == nil {
		t.Error("an owner package that will not parse did not stop the run, so it imported nothing and passed")
	}

	// The import check is driven directly, because it is not the only reader of
	// an owner file: internalRootHandlerNames walks the same tree and would
	// refuse the file a second time. Asking the producer is what pins the
	// import reader's own error handling rather than its neighbour's.
	if _, err := checkOwnersAreCmdZeFree(&scan{tree: owner}); err == nil {
		t.Error("the import reader answered an empty import list for a file it could not parse")
	}
}

// VALIDATES: a population smaller than the caller's floor is an error.
// PREVENTS: the gate answering OK over a tree holding neither cmd/ze nor
// internal, which the script does from any directory that is not a checkout.
func TestATreeTooSmallToBeTheOneAskedAboutIsAnError(t *testing.T) {
	empty := t.TempDir()
	findings, err := Check(empty, 0)
	if err != nil || len(findings) != 0 {
		t.Fatalf("an empty tree with no floor answers (%v, %v), want no findings and no error", findings, err)
	}

	_, err = Check(empty, 1)
	if err == nil {
		t.Fatal("an empty tree passed a floor of 1, so the gate passed having parsed nothing")
	}
	if !strings.Contains(err.Error(), "only 0 Go files parsed") {
		t.Errorf("the error is %q, want it to say how little was read", err)
	}
}

// VALIDATES: the answer IS the rows, under the script's own keys.
// PREVENTS: a payload wrapping the rows, which would change what every caller
// of the JSON reads.
func TestFindingsAreStructuredRows(t *testing.T) {
	raw, err := json.Marshal(Findings{{Kind: "k", File: "f.go", Msg: "m"}})
	if err != nil {
		t.Fatalf("marshal the findings: %v", err)
	}
	if string(raw) != `[{"kind":"k","file":"f.go","message":"m"}]` {
		t.Errorf("the payload is %s, want the script's array of three-key objects", raw)
	}
}

// VALIDATES: the page carries every finding and the verdict, and a clean run
// renders the sentence the script printed.
// PREVENTS: a page that reports a count the finding list does not support.
func TestTextCarriesEveryFindingAndTheVerdict(t *testing.T) {
	if got := (Findings{}).Text(); !strings.HasPrefix(got, "command-ownership: OK (") {
		t.Errorf("a clean run renders %q, want the script's verdict", got)
	}

	text := Findings{{Kind: "k", File: "cmd/ze/x.go", Msg: "m"}}.Text()
	if !strings.Contains(text, "  [k] cmd/ze/x.go: m\n") {
		t.Errorf("the page does not carry its finding:\n%s", text)
	}
	if !strings.Contains(text, "command-ownership: FAILED, 1 problem(s)") {
		t.Errorf("the page does not carry the verdict:\n%s", text)
	}
}

// VALIDATES: the command takes no argument and says so.
// PREVENTS: a path positional creeping in.
func TestAnswerRefusesAnArgument(t *testing.T) {
	payload, code := Answer([]string{"cmd/ze"})
	if code != 1 || payload != nil {
		t.Errorf("a stray argument answers (%v, %d), want (nil, 1)", payload, code)
	}
}

// VALIDATES: this checkout passes the gate, from the entry point a developer
// runs.
// PREVENTS: a command surface nobody owns. This is where
// TestNoOwnerAllowlistIsEnforced (scripts/checks/checks_test.go) now lives: it
// forked the script and asserted the tree passes and the verdict reads OK, and
// both facts are here.
func TestThisCheckoutPassesTheGate(t *testing.T) {
	payload, code := Answer(nil)
	findings, ok := payload.(Findings)
	if !ok {
		t.Fatalf("the command answered %T, want Findings", payload)
	}
	if code != 0 {
		t.Fatalf("the gate answers %d over this checkout:\n%s", code, findings.Text())
	}
	if !strings.HasPrefix(findings.Text(), "command-ownership: OK (") {
		t.Errorf("a passing run renders %q", findings.Text())
	}
}
