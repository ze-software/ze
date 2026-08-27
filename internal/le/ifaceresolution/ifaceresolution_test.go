// Related: ifaceresolution.go -- the guard these tests drive from its entry point
//
// Every test here calls the tool as a function. The gate used to be reachable
// only as a subprocess, so a case it did not already hold in this checkout
// could not be asserted at all.

package ifaceresolution

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

// VALIDATES: each of the three patterns is a finding, in a non-test .go file
// under a scanned root.
// PREVENTS: a pattern dropped from the list, which would leave the invariant
// unguarded for that spelling while the gate still passed.
func TestEachPatternIsAFinding(t *testing.T) {
	dir := tree(t, map[string]string{
		"internal/a/net.go":    "package a\n\nfunc f() { _, _ = net.InterfaceByName(\"eth0\") }\n",
		"internal/a/link.go":   "package a\n\nfunc g() { _, _ = handle.LinkByName(\"eth0\") }\n",
		"internal/a/ioctl.go":  "package a\n\nconst req = SIOCGIFINDEX\n",
		"internal/a/clean.go":  "package a\n\nfunc h() {}\n",
		"cmd/ze/main.go":       "package main\n\nfunc main() { _, _ = net.InterfaceByName(\"eth0\") }\n",
		"pkg/sdk/sdk.go":       "package sdk\n\nfunc k() { _, _ = netlink.LinkByName(\"eth0\") }\n",
		"internal/a/a_test.go": "package a\n\nfunc TestX() { _, _ = net.InterfaceByName(\"eth0\") }\n",
		"internal/a/notes.md":  "net.InterfaceByName(\n",
	})

	findings, err := Check(dir, 0)
	if err != nil {
		t.Fatalf("check the fixture: %v", err)
	}
	if len(findings) != 5 {
		t.Fatalf("the fixture draws %d findings, want 5: %v", len(findings), findings)
	}

	want := []string{"cmd/ze/main.go", "internal/a/ioctl.go", "internal/a/link.go", "internal/a/net.go", "pkg/sdk/sdk.go"}
	for i, path := range want {
		if findings[i].File != path {
			t.Errorf("finding %d names %s, want %s (the answer is sorted by file then line)", i, findings[i].File, path)
		}
		if findings[i].Line != 3 {
			t.Errorf("finding %d is at line %d, want 3", i, findings[i].Line)
		}
	}
}

// VALIDATES: a mention inside a comment is not a call, and a "://" inside a
// string literal does not truncate the line.
// PREVENTS: the two false positives stripComment exists to prevent, either of
// which would make the gate unusable and drive an exemption for a file that
// never violated anything.
func TestAMentionInProseIsNotACall(t *testing.T) {
	cases := map[string]struct {
		line  string
		found bool
	}{
		"a whole-line comment":             {"// net.InterfaceByName(x) is what this replaces", false},
		"a trailing comment":               {"\tvar x = 1 // net.InterfaceByName(x)", false},
		"a URL in a string keeps its code": {"\t_, _ = net.InterfaceByName(\"http://host\")", true},
		"a real call":                      {"\t_, _ = net.InterfaceByName(name)", true},
		"a function value, not a call":     {"\tvar f = net.InterfaceByName", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := tree(t, map[string]string{"internal/a/a.go": "package a\n\nfunc f() {\n" + tc.line + "\n}\n"})
			findings, err := Check(dir, 0)
			if err != nil {
				t.Fatalf("check the fixture: %v", err)
			}
			if got := len(findings) > 0; got != tc.found {
				t.Errorf("%q draws %d findings, want found=%v", tc.line, len(findings), tc.found)
			}
		})
	}
}

// VALIDATES: a directory entry exempts every file beneath it, a file entry
// exempts only that exact path, and nothing else is exempt.
// PREVENTS: a file prefix exempting a sibling whose name merely starts with it.
func TestAllowlistMatchesDirectoriesAndExactFiles(t *testing.T) {
	cases := map[string]bool{
		"internal/component/iface/resolve.go":       true,
		"internal/component/iface/deep/nested/x.go": true,
		"internal/plugins/ldp/register.go":          true,
		"internal/le/deployment/l2tpdiag_linux_ops.go": true,
		"internal/le/deployment/l2tpdiag_linux.go":     false,
		"internal/le/interoplab/bgp/isis_inject_linux.go": true,
		"internal/le/interoplab/bgp/bgp.go":               false,
		"internal/plugins/ldp/register.go2.go":      false,
		"internal/plugins/ldp/other.go":             false,
		"internal/component/bgp/reactor/reactor.go": false,
	}
	for path, want := range cases {
		if got := allowed(path); got != want {
			t.Errorf("allowed(%q) = %v, want %v", path, got, want)
		}
	}
}

// VALIDATES: a scan that read less than the caller's floor is an ERROR, not a
// clean tree.
// PREVENTS: the fail-open this gate exists to prevent, applied to itself. The
// script reports OK and exits 0 over a tree holding none of its three roots
// (scripts/checks/parity_test.go, TestScriptStillPassesOverATreeItNeverRead).
func TestATreeTooSmallToBeTheOneAskedAboutIsAnError(t *testing.T) {
	dir := tree(t, map[string]string{"internal/a/a.go": "package a\n"})

	if _, err := Check(dir, 2); err == nil {
		t.Error("a one-file tree passed a floor of 2, so the gate would pass having read almost nothing")
	}

	empty := t.TempDir()
	_, err := Check(empty, 1)
	if err == nil {
		t.Fatal("a tree holding none of the three roots passed, so the gate passed having read nothing")
	}
	if !strings.Contains(err.Error(), "only 0 Go files scanned") {
		t.Errorf("the error is %q, want it to say how little was read", err)
	}
}

// VALIDATES: a file the walk listed and cannot read stops the run.
// PREVENTS: a count silently short by whatever the walk skipped, which is the
// failure the sibling gates in this directory were each carrying.
func TestAnUnreadableFileStopsTheRun(t *testing.T) {
	dir := tree(t, map[string]string{"internal/a/a.go": "package a\n"})
	dangling := filepath.Join(dir, "internal", "a", "gone.go")
	if err := os.Symlink(filepath.Join(dir, "internal", "a", "never-written.go"), dangling); err != nil {
		t.Skipf("this filesystem does not take a symbolic link: %v", err)
	}

	if _, err := Check(dir, 0); err == nil {
		t.Error("a file the walk listed and could not open did not stop the run")
	}
}

// VALIDATES: the answer IS the rows, under the script's own keys, so `| json`
// renders the array the script's --json rendered.
// PREVENTS: a payload wrapping the rows in a struct, which would change what
// every caller of the JSON reads.
func TestFindingsAreStructuredRows(t *testing.T) {
	raw, err := json.Marshal(Findings{{File: "a.go", Line: 7, Code: "x"}})
	if err != nil {
		t.Fatalf("marshal the findings: %v", err)
	}
	if string(raw) != `[{"file":"a.go","line":7,"code":"x"}]` {
		t.Errorf("the payload is %s, want the script's array of three-key objects", raw)
	}
}

// VALIDATES: the page a person reads names every site and carries the remedy,
// and a clean run renders the verdict the script printed.
// PREVENTS: a violation list that says what is wrong and not what to do, which
// is what sends an author to an exemption instead of the resolver.
func TestTextNamesEverySiteAndTheRemedy(t *testing.T) {
	if got := (Findings{}).Text(); got != "iface-resolution: OK\n" {
		t.Errorf("a clean run renders %q, want the script's verdict", got)
	}

	text := Findings{{File: "internal/a/a.go", Line: 12, Code: "netlink.LinkByName(n)"}}.Text()
	for _, want := range []string{
		"iface-resolution: 1 direct kernel resolution site(s) outside the allowlist:",
		"  internal/a/a.go:12: netlink.LinkByName(n)",
		"iface.Resolve / iface.Addresses / iface.Subscribe",
		"scripts/checks/iface_resolution.go",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the page does not carry %q:\n%s", want, text)
		}
	}
}

// VALIDATES: the command takes no argument and says so.
// PREVENTS: a path positional creeping in, which would break
// keyword-before-value.
func TestAnswerRefusesAnArgument(t *testing.T) {
	payload, code := Answer([]string{"internal"})
	if code != 1 || payload != nil {
		t.Errorf("a stray argument answers (%v, %d), want (nil, 1)", payload, code)
	}
}

// VALIDATES: this checkout passes the gate, from the entry point a developer
// runs.
// PREVENTS: a consumer resolving a configured interface name straight against
// the kernel. This is where TestNoDirectInterfaceResolution
// (scripts/checks/iface_resolution_test.go) now lives: it forked the script and
// asserted the tree passes and the verdict reads OK, and both facts are here.
func TestThisCheckoutPassesTheGate(t *testing.T) {
	payload, code := Answer(nil)
	findings, ok := payload.(Findings)
	if !ok {
		t.Fatalf("the command answered %T, want Findings", payload)
	}
	if code != 0 {
		t.Fatalf("the gate answers %d over this checkout: %s", code, findings.Text())
	}
	if got := findings.Text(); got != "iface-resolution: OK\n" {
		t.Errorf("a passing run renders %q", got)
	}
}
