// The migration's proof for the gates in this directory: each script and its
// command agree.
//
// The tools under scripts/checks are being replaced by packages under letools,
// and the two sides live together until the swap
// (plan/spec-le-is-a-ze-binary.md, step 14). This file is what makes that safe,
// and it is deliberately HERE rather than beside the new packages: it is a
// migration artifact, so it is deleted by the same commit that deletes the
// scripts it compares against.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- over one tree, each script and its
// command report the same page, the same JSON and the same exit code.
// PREVENTS: a silent behavior change in a port whose output nobody reads. These
// are GATES: a check that stopped being taken, a floor that stopped firing or a
// population that stopped being walked passes every other test in this
// repository.
//
// The scripts are built with the tags THIS test binary carries, read from its
// own build info. A gate deriving its answer from the linked registry then sees
// the same plugin set on both sides, which is what makes the comparison mean
// anything: a reduced tag set compiles modules out and the two sides would
// disagree about the product rather than about the port.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/letools/cidispatch"
	"github.com/ze-software/ze/letools/cligrammar"
	"github.com/ze-software/ze/letools/commandownership"
	"github.com/ze-software/ze/letools/configclaims"
	"github.com/ze-software/ze/letools/configcoercion"
	"github.com/ze-software/ze/letools/dashstdio"
	"github.com/ze-software/ze/letools/fspersistence"
	"github.com/ze-software/ze/letools/ifaceresolution"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/pluginboundary"
	"github.com/ze-software/ze/letools/portdefaults"
	"github.com/ze-software/ze/letools/staticcheckmatrix"
	"github.com/ze-software/ze/letools/testsensitivity"
	"github.com/ze-software/ze/letools/trackedbuild"
	"github.com/ze-software/ze/letools/yangleafmentions"
)

// The two bounds these comparisons need. A link of the product and a walk of
// this checkout are both well inside five minutes on this hardware, so a run
// past either is a hung process rather than a slow one.
const (
	parityBuildTimeout = 300 * time.Second
	parityRunTimeout   = 300 * time.Second
)

// parityDir holds every script this file compiles, built once per test binary.
// A per-case build would relink the product for every case.
var (
	parityDir   string
	parityBuilt sync.Map
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "checks-parity")
	if err != nil {
		panic("BUG: checks parity test: cannot create a temporary directory")
	}
	parityDir = dir
	code := m.Run()
	os.RemoveAll(dir) //nolint:errcheck // temporary directory
	os.Exit(code)
}

// parityTags answers the build tags this test binary was compiled with, so a
// script can be compiled with the same ones.
func parityTags() []string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	for _, setting := range info.Settings {
		if setting.Key != "-tags" {
			continue
		}
		return strings.FieldsFunc(setting.Value, func(r rune) bool { return r == ',' })
	}
	return nil
}

// buildOnce is the compiled path of one script, and the error that produced no
// path. Both travel together so a second caller is told why rather than
// rebuilding.
type buildOnce struct {
	once sync.Once
	path string
	err  error
}

// parityScript compiles one script of this directory and answers its path. The
// compilation happens once per source per test binary, whichever case asks
// first.
func parityScript(t *testing.T, source string) string {
	t.Helper()

	entry, _ := parityBuilt.LoadOrStore(source, &buildOnce{})
	built, ok := entry.(*buildOnce)
	if !ok {
		t.Fatalf("the build cache holds a %T for %s", entry, source)
	}
	built.once.Do(func() { built.path, built.err = compileScript(source) })
	if built.err != nil {
		t.Fatalf("compile %s: %v", source, built.err)
	}
	return built.path
}

// compileScript is a function of its own so its context can be canceled by a
// defer that actually runs.
func compileScript(source string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), parityBuildTimeout)
	defer cancel()

	repo, err := filepath.Abs("../..")
	if err != nil {
		return "", fmt.Errorf("resolve the repository root: %w", err)
	}

	out := filepath.Join(parityDir, strings.TrimSuffix(source, ".go"))
	args := []string{"build", "-o", out}
	if tags := parityTags(); len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	args = append(args, filepath.Join("scripts", "checks", source))

	build := exec.CommandContext(ctx, "go", args...) //nolint:gosec // arguments are this file's own literals plus its build tags
	build.Dir = repo
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if combined, buildErr := build.CombinedOutput(); buildErr != nil {
		return "", fmt.Errorf("%w\n%s", buildErr, combined)
	}
	return out, nil
}

// runParityScript runs one compiled script from the repository root and answers
// what a caller of the gate sees: its stdout, its stderr and its exit code.
func runParityScript(t *testing.T, source string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), parityRunTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parityScript(t, source), args...)
	cmd.Dir = repoRoot(t)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		code = 0
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Fatalf("run %s %v: %v (%s)", source, args, err, errOut.String())
	}
	return out.String(), errOut.String(), code
}

// VALIDATES: AC-11 for the config-claims gate -- the script's page and the
// command's page are the same text over the live registry, and both answer the
// same exit code.
// PREVENTS: a port that reads a different inventory, or renders one the reader
// can no longer act on.
func TestConfigClaimsPagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "config_claims.go")

	payload, commandCode := configclaims.Answer(nil)
	report, ok := payload.(configclaims.Report)
	if !ok {
		t.Fatalf("the command answered %T, want a Report", payload)
	}

	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr, so this checkout does not pass the gate: %s", scriptErr)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 for the machine-readable half -- the script's --json and the
// command's payload are the same document.
// PREVENTS: a key renamed, dropped or retyped by the port, which a page
// comparison alone would not see because the page prints only three of the four
// fields.
func TestConfigClaimsJSONAgrees(t *testing.T) {
	scriptOut, scriptErr, _ := runParityScript(t, "config_claims.go", "--json")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr, so this checkout does not pass the gate: %s", scriptErr)
	}

	payload, _ := configclaims.Answer(nil)
	commandRaw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal the command's payload: %v", err)
	}

	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptOut)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}

	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// runParityScriptIn runs one compiled script from dir rather than from the
// repository root, which is how a gate whose walk roots are relative is pointed
// at a fixture tree.
func runParityScriptIn(t *testing.T, dir, source string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), parityRunTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parityScript(t, source), args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		code = 0
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Fatalf("run %s %v in %s: %v (%s)", source, args, dir, err, errOut.String())
	}
	return out.String(), errOut.String(), code
}

// ifaceFixture writes a tree holding one of every pattern the gate looks for,
// plus the three cases it must NOT report.
func ifaceFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"internal/a/net.go":               "package a\n\nfunc f() { _, _ = net.InterfaceByName(\"eth0\") }\n",
		"internal/a/link.go":              "package a\n\nfunc g() { _, _ = handle.LinkByName(\"eth0\") }\n",
		"internal/a/ioctl.go":             "package a\n\nconst req = SIOCGIFINDEX\n",
		"internal/a/prose.go":             "package a\n\n// net.InterfaceByName(x) is what this replaces\nfunc h() {}\n",
		"internal/a/a_test.go":            "package a\n\nfunc TestX() { _, _ = net.InterfaceByName(\"eth0\") }\n",
		"internal/component/iface/own.go": "package iface\n\nfunc r() { _, _ = netlink.LinkByName(\"eth0\") }\n",
		"cmd/ze/main.go":                  "package main\n\nfunc main() { _, _ = net.InterfaceByName(\"eth0\") }\n",
		"pkg/sdk/sdk.go":                  "package sdk\n\nfunc k() { _, _ = netlink.LinkByName(\"eth0\") }\n",
	}
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

// siteLines keeps the "  file:line: code" lines out of either side's report, in
// order, so the two can be compared whichever stream each wrote them to.
func siteLines(report string) []string {
	var kept []string
	for line := range strings.SplitSeq(report, "\n") {
		if strings.HasPrefix(line, "  ") && strings.Contains(line, ".go:") {
			kept = append(kept, line)
		}
	}
	return kept
}

// VALIDATES: AC-11 for the iface-resolution gate over the REAL checkout -- the
// script's page and the command's page are the same text, and both answer 0.
// PREVENTS: a port whose allowlist or pattern list drifted from the script's
// while both copies exist, which is the cost of duplicate-then-swap and the
// thing this file is here to bound.
func TestIfaceResolutionPagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "iface_resolution.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr, so this checkout does not pass the gate: %s", scriptErr)
	}

	payload, commandCode := ifaceresolution.Answer(nil)
	findings, ok := payload.(ifaceresolution.Findings)
	if !ok {
		t.Fatalf("the command answered %T, want Findings", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := findings.Text(); got != scriptOut {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 over a tree that FAILS the gate -- the two report the same
// sites, in the same order, and both exit 1.
// PREVENTS: a comparison that only ever sees a clean tree. This checkout draws
// zero findings, so every pattern, the allowlist, the test-file rule and the
// comment rule are exercised by this case and by no other.
//
// ONE difference is deliberate and is asserted rather than compared: the script
// writes the site list to stderr and the OK verdict to stdout, while the
// command's sites ARE its answer and every le answer is rendered on stdout. The
// script is already inconsistent about this -- its --json writes the same sites
// to stdout -- so there was no single stream to preserve.
func TestIfaceResolutionFindingsAgreeOverAFailingTree(t *testing.T) {
	dir := ifaceFixture(t)

	scriptOut, scriptErr, scriptCode := runParityScriptIn(t, dir, "iface_resolution.go")
	if scriptOut != "" {
		t.Errorf("the script wrote a page to stdout over a failing tree: %s", scriptOut)
	}

	findings, err := ifaceresolution.Check(dir, 0)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	commandCode := 0
	if len(findings) > 0 {
		commandCode = 1
	}

	if scriptCode != 1 || commandCode != 1 {
		t.Fatalf("the script exits %d and the command answers %d over a tree holding five violations", scriptCode, commandCode)
	}

	want := siteLines(scriptErr)
	got := siteLines(findings.Text())
	if len(want) != 5 {
		t.Fatalf("the script reported %d sites over the fixture, want 5: %v", len(want), want)
	}
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Errorf("the site lists differ.\nscript:\n%s\ncommand:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

// VALIDATES: AC-11 for the machine-readable half over the same failing tree.
// PREVENTS: a key renamed or retyped by the port, which the page comparison
// cannot see because the page reformats all three.
func TestIfaceResolutionJSONAgrees(t *testing.T) {
	dir := ifaceFixture(t)

	scriptOut, _, _ := runParityScriptIn(t, dir, "iface_resolution.go", "--json")
	findings, err := ifaceresolution.Check(dir, 0)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	commandRaw, err := json.Marshal(findings)
	if err != nil {
		t.Fatalf("marshal the findings: %v", err)
	}

	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptOut)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}

	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: the script STILL passes over a tree holding none of its three walk
// roots, and the command refuses the same tree.
// PREVENTS: this test going quiet if somebody fixes the script. It asserts the
// DEFECT, so it reddens the day the script is repaired -- and the answer then is
// to delete this case with the script it describes.
//
// The defect: iface_resolution.go skips a walk root it cannot stat
// (`if fi, statErr := os.Stat(root); statErr != nil || !fi.IsDir() { continue }`),
// and cmd, internal and pkg are its whole population. Run anywhere but a Ze
// checkout it prints "iface-resolution: OK" and exits 0 having read no file at
// all. The port carries a floor instead (ifaceresolution.scanFloor).
func TestScriptStillPassesOverATreeItNeverRead(t *testing.T) {
	empty := t.TempDir()

	scriptOut, scriptErr, scriptCode := runParityScriptIn(t, empty, "iface_resolution.go")
	if scriptCode != 0 || !strings.Contains(scriptOut, "iface-resolution: OK") {
		t.Fatalf("the script no longer fails open (exit %d, %q, %q): delete this case and the script's fail-open row with it", scriptCode, scriptOut, scriptErr)
	}

	if _, err := ifaceresolution.Check(empty, 500); err == nil {
		t.Error("the command passed over a tree it never read, so the port carries the same defect")
	}
}

// ownershipFixture writes a tree holding one violation of each of the four
// kinds the gate draws.
func ownershipFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"internal/bgp/cli/register.go":    "package cli\n\nimport _ \"github.com/ze-software/ze/cmd/ze\"\n",
		"internal/doctor/cli/register.go": "package cli\n\nfunc f() { registry.MustRegisterRootHandler(\"doctor\", nil, m) }\n",
		"cmd/ze/x.go":                     "package main\n\nfunc f() {\n\tregistry.MustRegisterRootHandler(\"bgp\", nil, m)\n\tregistry.RegisterRoot(\"mystery\", m)\n}\n",
	}
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

// VALIDATES: AC-11 for the command-ownership gate over the REAL checkout -- the
// script's page and the command's page are the same text, and both answer 0.
// PREVENTS: an allowlist that drifted between the two copies while both exist.
func TestCommandOwnershipPagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "command_ownership.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr: %s", scriptErr)
	}

	payload, commandCode := commandownership.Answer(nil)
	findings, ok := payload.(commandownership.Findings)
	if !ok {
		t.Fatalf("the command answered %T, want Findings", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := findings.Text(); got != scriptOut {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 over a tree that FAILS the gate, page and JSON, all four
// finding kinds present.
// PREVENTS: a comparison that only ever sees a clean tree. This checkout draws
// zero findings, so every kind, the allowlist, the alias spelling and the
// variant exemption are exercised by this case and by no other.
func TestCommandOwnershipAgreesOverAFailingTree(t *testing.T) {
	dir := ownershipFixture(t)

	scriptOut, scriptErr, scriptCode := runParityScriptIn(t, dir, "command_ownership.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr: %s", scriptErr)
	}

	findings, err := commandownership.Check(dir, 0)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	if len(findings) != 4 {
		t.Fatalf("the command draws %d findings over the fixture, want one of each kind: %v", len(findings), findings)
	}
	if scriptCode != 1 {
		t.Fatalf("the script exits %d over a tree holding four violations", scriptCode)
	}
	if got := findings.Text(); got != scriptOut {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}

	scriptJSON, _, _ := runParityScriptIn(t, dir, "command_ownership.go", "--json")
	// The script appends its verdict line to its own --json, on stdout, which
	// is the deliberate difference TestOwnershipScriptStillCorruptsItsOwnJSON
	// pins. Cut it off so the documents can be compared.
	if cut := strings.Index(scriptJSON, "\ncommand-ownership: "); cut >= 0 {
		scriptJSON = scriptJSON[:cut]
	}
	commandRaw, err := json.Marshal(findings)
	if err != nil {
		t.Fatalf("marshal the findings: %v", err)
	}
	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptJSON), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptJSON)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}
	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: the script STILL reports OK over a cmd/ze file it cannot parse,
// and over a tree holding neither of its two walk roots. The command refuses
// both.
// PREVENTS: this test going quiet if somebody fixes the script. It asserts the
// DEFECT, so it reddens the day the script is repaired -- and the answer then is
// to delete this case with the script it describes.
//
// The defect: command_ownership.go discards every read error it makes.
// forEachRegistryCall and fileImports return silently on a parser error, the
// three walks discard theirs, and the walk roots are relative to the working
// directory. A cmd/ze file with a syntax error therefore registers no root at
// all, and the gate reports "command-ownership: OK" over it and exits 0.
func TestOwnershipScriptStillPassesOverWhatItCannotRead(t *testing.T) {
	broken := t.TempDir()
	path := filepath.Join(broken, "cmd", "ze", "bad.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("package main\n\nfunc f() { registry.RegisterRoot(\"mystery\"\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	scriptOut, _, scriptCode := runParityScriptIn(t, broken, "command_ownership.go")
	if scriptCode != 0 || !strings.Contains(scriptOut, "command-ownership: OK") {
		t.Fatalf("the script no longer fails open on a file it cannot parse (exit %d, %q): delete this case and the fail-open row with it", scriptCode, scriptOut)
	}
	if _, err := commandownership.Check(broken, 0); err == nil {
		t.Error("the command passed over a file it could not parse, so the port carries the same defect")
	}

	empty := t.TempDir()
	scriptOut, _, scriptCode = runParityScriptIn(t, empty, "command_ownership.go")
	if scriptCode != 0 || !strings.Contains(scriptOut, "command-ownership: OK") {
		t.Fatalf("the script no longer fails open on a tree it never read (exit %d, %q)", scriptCode, scriptOut)
	}
	if _, err := commandownership.Check(empty, 500); err == nil {
		t.Error("the command passed over a tree it never read, so the port carries the same defect")
	}
}

// VALIDATES: the script STILL writes its verdict line into its own --json
// stream, and the command does not.
// PREVENTS: this test going quiet if somebody fixes the script. It asserts the
// DEFECT, so it reddens the day the script is repaired.
//
// The defect: command_ownership.go writes the findings as JSON to stdout and
// then writes "command-ownership: FAILED, N problem(s)" to the SAME stream. The
// gate declares json_flag='--json' (scripts/le/application/check_cli.py), so a
// caller that parses it fails exactly when the gate has something to report --
// a clean run's JSON parses and a failing run's does not. The port answers a
// payload and leroot renders it, so nothing but the document reaches stdout.
func TestOwnershipScriptStillCorruptsItsOwnJSON(t *testing.T) {
	dir := ownershipFixture(t)

	scriptJSON, _, scriptCode := runParityScriptIn(t, dir, "command_ownership.go", "--json")
	if scriptCode != 1 {
		t.Fatalf("the script exits %d over the failing fixture", scriptCode)
	}
	if json.Valid([]byte(scriptJSON)) {
		t.Fatal("the script's --json now parses over a failing tree: delete this case and the corrupted-JSON row with it")
	}
	if !strings.Contains(scriptJSON, "command-ownership: FAILED") {
		t.Errorf("the script's --json is invalid for some other reason:\n%s", scriptJSON)
	}

	findings, err := commandownership.Check(dir, 0)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	raw, err := json.Marshal(findings)
	if err != nil {
		t.Fatalf("marshal the findings: %v", err)
	}
	if !json.Valid(raw) {
		t.Errorf("the command's payload does not parse either: %s", raw)
	}
}

// coercionFixture writes a tree holding one of each finding kind plus the two
// correct shapes the guard must leave alone.
func coercionFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"internal/a/config.go": "package p\n\nfunc toInt(v any) (int, bool) {\n\tswitch n := v.(type) {\n\tcase int:\n\t\treturn n, true\n\t}\n\treturn 0, false\n}\n",
		"internal/b/config.go": "package p\n\nfunc parse(m map[string]any) bool {\n\tb, _ := m[\"enabled\"].(bool)\n\treturn b\n}\n",
		"internal/c/config.go": "package p\n\nimport \"strconv\"\n\nfunc toInt(v any) (int, bool) {\n\tswitch n := v.(type) {\n\tcase int:\n\t\treturn n, true\n\tcase string:\n\t\ti, _ := strconv.Atoi(n)\n\t\treturn i, true\n\t}\n\treturn 0, false\n}\n",
		"internal/d/parser.go": "package p\n\nfunc parse(m map[string]any) bool {\n\tb, _ := m[\"enabled\"].(bool)\n\treturn b\n}\n",
	}
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

// VALIDATES: AC-11 for both config-coercion gates over the REAL checkout -- the
// script and the command answer the same page and the same exit code, for the
// check and for the selftest.
// PREVENTS: a port whose type list or allowlist drifted from the script's while
// both copies exist.
func TestConfigCoercionPagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "config_string_coercion.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr, so this checkout does not pass the gate: %s", scriptErr)
	}

	payload, commandCode := configcoercion.Answer([]string{"check"})
	findings, ok := payload.(configcoercion.Findings)
	if !ok {
		t.Fatalf("the check answered %T, want Findings", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := findings.Text(); got != scriptOut {
		t.Errorf("the check pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}

	scriptOut, scriptErr, scriptCode = runParityScript(t, "config_string_coercion.go", "--selftest")
	if scriptErr != "" {
		t.Fatalf("the script's selftest wrote to stderr: %s", scriptErr)
	}
	payload, commandCode = configcoercion.Answer([]string{"selftest"})
	report, ok := payload.(leroot.SelftestReport)
	if !ok {
		t.Fatalf("the selftest answered %T, want a SelftestReport", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the selftest script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the selftest pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 over a tree that FAILS the gate -- the two report the same
// sites and the same JSON, and both exit 1.
// PREVENTS: a comparison that only ever sees a clean tree. This checkout draws
// zero findings, so both detection shapes, the config.go-only rule and the
// sorting are exercised by this case and by no other.
//
// ONE difference is deliberate and is asserted rather than compared: the script
// writes the site list to stderr and the OK verdict to stdout, while the
// command's sites ARE its answer and every le answer is rendered on stdout. The
// script is already inconsistent about this -- its --json writes the same sites
// to stdout -- so there was no single stream to preserve.
func TestConfigCoercionAgreesOverAFailingTree(t *testing.T) {
	dir := coercionFixture(t)

	_, scriptErr, scriptCode := runParityScriptIn(t, dir, "config_string_coercion.go")
	findings, err := configcoercion.Check(dir)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("the command draws %d findings over the fixture, want one of each kind: %v", len(findings), findings)
	}
	if scriptCode != 1 {
		t.Fatalf("the script exits %d over a tree holding two violations", scriptCode)
	}

	want := siteLines(scriptErr)
	got := siteLines(findings.Text())
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Errorf("the site lists differ.\nscript:\n%s\ncommand:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}

	scriptJSON, _, _ := runParityScriptIn(t, dir, "config_string_coercion.go", "--json")
	commandRaw, err := json.Marshal(findings)
	if err != nil {
		t.Fatalf("marshal the findings: %v", err)
	}
	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptJSON), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptJSON)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}
	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: AC-11 for both port-defaults gates over the REAL checkout -- the
// script and the command answer the same page and the same exit code, for the
// check and for the selftest.
// PREVENTS: a port whose service map, regexes or reason codes drifted from the
// script's while both copies exist.
func TestPortDefaultsPagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "port_defaults.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr, so this checkout does not pass the gate: %s", scriptErr)
	}

	payload, commandCode := portdefaults.Answer([]string{"check"})
	result, ok := payload.(portdefaults.Result)
	if !ok {
		t.Fatalf("the check answered %T, want a Result", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := result.Text(); got != scriptOut {
		t.Errorf("the check pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}

	scriptOut, scriptErr, scriptCode = runParityScript(t, "port_defaults.go", "--selftest")
	if scriptErr != "" {
		t.Fatalf("the script's selftest wrote to stderr: %s", scriptErr)
	}
	payload, commandCode = portdefaults.Answer([]string{"selftest"})
	report, ok := payload.(leroot.SelftestReport)
	if !ok {
		t.Fatalf("the selftest answered %T, want a SelftestReport", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the selftest script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the selftest pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 for the machine-readable half over the REAL checkout.
// PREVENTS: a key renamed or retyped by the port. The page prints three of the
// five drift fields and neither of the two result fields, so a page comparison
// alone cannot see the JSON.
func TestPortDefaultsJSONAgrees(t *testing.T) {
	scriptOut, scriptErr, _ := runParityScript(t, "port_defaults.go", "--json")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr: %s", scriptErr)
	}

	payload, _ := portdefaults.Answer([]string{"check"})
	commandRaw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal the command's payload: %v", err)
	}

	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptOut)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}
	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: AC-11 for the yang-leaf-mentions report over the REAL checkout --
// the script's page and the command's page are the same text, and both answer
// 0.
// PREVENTS: a port that resolves the owning package differently, which would
// change every row's third column while the counts stayed identical.
func TestYangLeafMentionsPagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "yang_leaf_mentions.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr: %s", scriptErr)
	}

	payload, commandCode := yangleafmentions.Answer([]string{"report"})
	report, ok := payload.(yangleafmentions.Report)
	if !ok {
		t.Fatalf("the command answered %T, want a Report", payload)
	}
	if scriptCode != 0 || commandCode != 0 {
		t.Errorf("the script exits %d and the command answers %d; the report is advisory and owes 0", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 for the machine-readable half over the REAL checkout.
// PREVENTS: a key renamed or retyped by the port. The page prints three of the
// four finding fields and folds both counts into one sentence, so a page
// comparison alone cannot see the JSON.
func TestYangLeafMentionsJSONAgrees(t *testing.T) {
	scriptOut, scriptErr, _ := runParityScript(t, "yang_leaf_mentions.go", "--json")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr: %s", scriptErr)
	}

	payload, _ := yangleafmentions.Answer([]string{"report"})
	commandRaw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal the command's payload: %v", err)
	}

	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptOut)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}
	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: AC-11 over a fixture tree with a known answer. Both halves
// discover the module, parse three leaves, and report only the unnamed leaf.
// PREVENTS: a comparison that sees only this checkout. Both halves can make
// the same error on the real tree. The fixture tests both polarities of the
// heuristic against a tree that carries one of each.
func TestYangLeafMentionsAgreeOverAFixtureTree(t *testing.T) {
	dir := t.TempDir()
	if err := yangleafmentions.WriteFixture(dir); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}

	scriptOut, scriptErr, scriptCode := runParityScriptIn(t, dir, "yang_leaf_mentions.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr over the fixture: %s", scriptErr)
	}
	if scriptCode != 0 {
		t.Fatalf("the script exits %d over the fixture, want 0", scriptCode)
	}

	report, err := yangleafmentions.ScanTree(dir)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("the command reported %d findings over the fixture, want the one unread leaf", len(report.Findings))
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the fixture pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 for the selftest half -- both sides pass over the fixture and
// both answer 0.
// PREVENTS: a port whose selftest asserts less than the script's did, which
// would leave the heuristic unguarded while the gate still went green.
func TestYangLeafMentionsSelftestsAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "yang_leaf_mentions.go", "--selftest")
	if scriptErr != "" {
		t.Fatalf("the selftest script wrote to stderr: %s", scriptErr)
	}

	payload, commandCode := yangleafmentions.Answer([]string{"selftest"})
	report, ok := payload.(leroot.SelftestReport)
	if !ok {
		t.Fatalf("the selftest answered %T, want a SelftestReport", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the selftest script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the selftest pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
	// The script asserted seven properties of the fixture scan and reported
	// only the first that broke. The port answers a row per property, so a
	// count below seven is coverage the port dropped.
	if len(report.Results) != 7 {
		t.Errorf("the selftest answered %d case rows, want the script's seven properties", len(report.Results))
	}
}

// VALIDATES: AC-11 for the cli-grammar gate over the REAL checkout -- the
// script's page and the command's page are the same text, and both answer 0.
// PREVENTS: a feeder that stopped firing in the port. The page carries the
// count of each population, so a feeder reading nothing changes the text even
// when it draws no row.
func TestCLIGrammarPagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "cli_grammar.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr: %s", scriptErr)
	}

	payload, commandCode := cligrammar.Answer(nil)
	result, ok := payload.(cligrammar.Result)
	if !ok {
		t.Fatalf("the command answered %T, want a Result", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := result.Text(); got != scriptOut {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 for the machine-readable half over the REAL checkout.
// PREVENTS: a key renamed or retyped by the port. The page prints six of the
// eleven fields, so a page comparison alone cannot see the JSON.
func TestCLIGrammarJSONAgrees(t *testing.T) {
	scriptOut, scriptErr, _ := runParityScript(t, "cli_grammar.go", "--json")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr: %s", scriptErr)
	}

	payload, _ := cligrammar.Answer(nil)
	commandRaw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal the command's payload: %v", err)
	}

	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptOut)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}
	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: the script STILL passes over a tree holding none of the three
// directories its source feeders walk, and the command refuses the same tree.
// PREVENTS: this test going quiet if somebody fixes the script. It asserts the
// DEFECT, so it reddens the day the script is repaired -- and the answer then is
// to delete this case with the script it describes.
//
// The defect: cli_grammar.go walks `internal`, `cmd/ze`, and `demos/terminal`
// relative to the working directory. It discards every walk, open, scan, and
// parse error. Outside a checkout, it reads no .yang file, resolves no root,
// and opens no demo script. It then prints "cli-grammar: OK" and exits 0 with
// three of its five feeders silent. The port uses cligrammar.DefaultFloor
// instead.
func TestGrammarScriptStillPassesOverATreeItNeverRead(t *testing.T) {
	empty := t.TempDir()

	scriptOut, scriptErr, scriptCode := runParityScriptIn(t, empty, "cli_grammar.go")
	if scriptCode != 0 || !strings.Contains(scriptOut, "cli-grammar: OK") {
		t.Fatalf("the script no longer fails open (exit %d, %q, %q): delete this case and the script's fail-open row with it", scriptCode, scriptOut, scriptErr)
	}
	if !strings.Contains(scriptOut, "Roots checked: 0") {
		t.Errorf("the script no longer reports zero roots over an empty tree: %s", scriptOut)
	}

	if _, err := cligrammar.Check(empty, cligrammar.DefaultFloor); err == nil {
		t.Error("the command passed over a tree it never read, so the port carries the same defect")
	}
}

// fsPersistenceFixture writes a tree holding one violation of each shape the
// gate draws, plus the four shapes it must NOT report.
func fsPersistenceFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"internal/plugins/a/save.go":             "package a\n\nimport \"os\"\n\nfunc save(p string, d []byte) error { return os.WriteFile(p, d, 0o600) }\n",
		"internal/plugins/a/alias.go":            "package a\n\nimport fsys \"os\"\n\nfunc keep(p string, d []byte) error { return fsys.WriteFile(p, d, 0o600) }\n",
		"internal/component/b/open.go":           "package b\n\nimport \"os\"\n\nfunc w(p string) (*os.File, error) { return os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o644) }\n\nfunc r(p string) (*os.File, error) { return os.OpenFile(p, os.O_RDONLY, 0) }\n",
		"cmd/ze/read.go":                         "package main\n\nimport \"os\"\n\nfunc load(p string) ([]byte, error) { return os.ReadFile(p) }\n",
		"cmd/ze/skip_test.go":                    "package main\n\nimport \"os\"\n\nfunc TestX() { _ = os.WriteFile(\"x\", nil, 0o600) }\n",
		"internal/component/config/storage/s.go": "package storage\n\nimport \"os\"\n\nfunc put(p string, d []byte) error { return os.WriteFile(p, d, 0o600) }\n",
	}
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

// VALIDATES: AC-11 for the fs-persistence gate over the REAL checkout -- the
// script's page and the command's page are the same text, and both answer 0.
// PREVENTS: an allowlist that drifted from the script's while both copies
// exist, which is the cost of duplicate-then-swap and the thing this file is
// here to bound.
func TestFSPersistencePagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "direct_fs_persistence.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr, so this checkout does not pass the gate: %s", scriptErr)
	}

	payload, commandCode := fspersistence.Answer([]string{"check"})
	findings, ok := payload.(fspersistence.Findings)
	if !ok {
		t.Fatalf("the command answered %T, want Findings", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := findings.Text(); got != scriptOut {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 over a tree that FAILS the gate -- the two report the same
// sites, in the same order, and both answer 1.
// PREVENTS: a comparison that only ever sees a clean tree. This checkout draws
// zero findings, so every write primitive, the read-only rule, the test-file
// rule and both allowlists are exercised by this case and by no other.
//
// ONE difference is deliberate and is asserted rather than compared. The script
// writes the site list to stderr and the OK verdict to stdout. The command's
// sites ARE its answer, and every le answer is rendered on stdout.
func TestFSPersistenceFindingsAgreeOverAFailingTree(t *testing.T) {
	dir := fsPersistenceFixture(t)

	scriptOut, scriptErr, scriptCode := runParityScriptIn(t, dir, "direct_fs_persistence.go")
	if scriptOut != "" {
		t.Errorf("the script wrote a page to stdout over a failing tree: %s", scriptOut)
	}

	findings, err := fspersistence.Check(dir, 0)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	commandCode := 0
	if len(findings) > 0 {
		commandCode = 1
	}
	if scriptCode != 1 || commandCode != 1 {
		t.Fatalf("the script exits %d and the command answers %d over a tree holding three raw writes", scriptCode, commandCode)
	}

	want := siteLines(scriptErr)
	got := siteLines(findings.Text())
	if len(want) != 3 {
		t.Fatalf("the script reported %d sites over the fixture, want 3: %v", len(want), want)
	}
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Errorf("the site lists differ.\nscript:\n%s\ncommand:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

// VALIDATES: AC-11 for the machine-readable half over the same failing tree.
// PREVENTS: a key renamed or retyped by the port, which the page comparison
// cannot see because the page reformats all five.
func TestFSPersistenceJSONAgrees(t *testing.T) {
	dir := fsPersistenceFixture(t)

	scriptOut, _, _ := runParityScriptIn(t, dir, "direct_fs_persistence.go", "--json")
	findings, err := fspersistence.Check(dir, 0)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	commandRaw, err := json.Marshal(findings)
	if err != nil {
		t.Fatalf("marshal the findings: %v", err)
	}

	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptOut)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}
	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: AC-11 for the selftest half -- both sides pass over their fixtures
// and both answer 0.
// PREVENTS: a port whose selftest asserts less than the script's did, which
// would leave the guard unguarded while the gate still went green.
func TestFSPersistenceSelftestsAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "direct_fs_persistence.go", "--selftest")
	if scriptErr != "" {
		t.Fatalf("the selftest script wrote to stderr: %s", scriptErr)
	}

	payload, commandCode := fspersistence.Answer([]string{"selftest"})
	report, ok := payload.(leroot.SelftestReport)
	if !ok {
		t.Fatalf("the selftest answered %T, want a SelftestReport", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the selftest script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the selftest pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
	// The script ran eight fixtures and reported every one that broke. The port
	// answers a row per fixture, so a count below eight is coverage it dropped.
	if len(report.Results) != 8 {
		t.Errorf("the selftest answered %d case rows, want the script's eight fixtures", len(report.Results))
	}
}

// boundaryFixture writes a tree holding two unguarded plugin packages, one
// guarded package whose guard sits in a sibling file, and a blank import.
//
// It also writes a copy of the composition-root generator because the SCRIPT
// gets its scan roots from that file's source text. The command calls
// letools/pluginimports to get the roots. The copy lets both sides receive the
// same question.
func boundaryFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	const ifacePkg = "github.com/ze-software/ze/internal/component/iface"
	files := map[string]string{
		"internal/plugins/plain/register.go":          "package plain\n\nimport \"" + ifacePkg + "\"\n\nfunc run() { iface.GetBackend() }\n",
		"internal/plugins/aliased/register.go":        "package aliased\n\nimport ifcomp \"" + ifacePkg + "\"\n\nfunc run() { ifcomp.GetBackend() }\n",
		"internal/plugins/guardedaliased/register.go": "package guardedaliased\n\nimport ifcomp \"" + ifacePkg + "\"\n\nfunc run() { ifcomp.GetBackend() }\n",
		"internal/plugins/guardedaliased/guard.go":    "package guardedaliased\n\nfunc checkInternal() { p.IsInternal() }\n",
		"internal/plugins/blankimport/register.go":    "package blankimport\n\nimport _ \"" + ifacePkg + "\"\n",
	}
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	generator := filepath.Join(dir, "scripts", "codegen", "plugin_imports.go")
	if err := os.MkdirAll(filepath.Dir(generator), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(generator), err)
	}
	source, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "codegen", "plugin_imports.go"))
	if err != nil {
		t.Fatalf("read the generator: %v", err)
	}
	if err := os.WriteFile(generator, source, 0o600); err != nil {
		t.Fatalf("write the generator copy: %v", err)
	}
	return dir
}

// VALIDATES: AC-11 for the plugin-boundary gate over the REAL checkout -- the
// script's page and the command's page are the same text, and both answer 0.
// PREVENTS: a watch list or an allowlist that drifted from the script's while
// both copies exist.
func TestPluginBoundaryPagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "plugin_process_boundary.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr, so this checkout does not pass the gate: %s", scriptErr)
	}

	payload, commandCode := pluginboundary.Answer([]string{"check"})
	findings, ok := payload.(pluginboundary.Findings)
	if !ok {
		t.Fatalf("the command answered %T, want Findings", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := findings.Text(); got != scriptOut {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 for the scan roots. The script's --print-roots and the
// command's roots action name the same directories in the same order.
// PREVENTS: the two sides judging different populations. Every other comparison
// here would miss that difference. The script parses the generator's source
// text, and the command calls the generator's function. This case confirms that
// the two derivations agree.
func TestPluginBoundaryRootsAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "plugin_process_boundary.go", "--print-roots")
	if scriptErr != "" || scriptCode != 0 {
		t.Fatalf("the script failed to print its roots (exit %d): %s", scriptCode, scriptErr)
	}

	payload, commandCode := pluginboundary.Answer([]string{"roots"})
	roots, ok := payload.(pluginboundary.RootList)
	if !ok {
		t.Fatalf("the command answered %T, want a RootList", payload)
	}
	if commandCode != 0 {
		t.Errorf("the roots action answers %d, want 0", commandCode)
	}
	if got := roots.Text(); got != scriptOut {
		t.Errorf("the derived roots differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 over a tree that FAILS the gate -- the two report the same
// sites, in the same order, and both answer 1.
// PREVENTS: a comparison that only ever sees a clean tree. This checkout draws
// zero findings, so alias resolution, the sibling-file guard rule and the blank
// import are exercised by this case and by no other.
//
// ONE difference is deliberate and is asserted rather than compared. The script
// writes the site list to stderr and the OK verdict to stdout. The command's
// sites ARE its answer, and every le answer is rendered on stdout.
func TestPluginBoundaryFindingsAgreeOverAFailingTree(t *testing.T) {
	dir := boundaryFixture(t)

	scriptOut, scriptErr, scriptCode := runParityScriptIn(t, dir, "plugin_process_boundary.go")
	if scriptOut != "" {
		t.Errorf("the script wrote a page to stdout over a failing tree: %s", scriptOut)
	}

	findings, err := pluginboundary.Check(dir, 0)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	commandCode := 0
	if len(findings) > 0 {
		commandCode = 1
	}
	if scriptCode != 1 || commandCode != 1 {
		t.Fatalf("the script exits %d and the command answers %d over a tree holding two unguarded calls", scriptCode, commandCode)
	}

	want := siteLines(scriptErr)
	got := siteLines(findings.Text())
	if len(want) != 2 {
		t.Fatalf("the script reported %d sites over the fixture, want 2: %v", len(want), want)
	}
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Errorf("the site lists differ.\nscript:\n%s\ncommand:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

// VALIDATES: AC-11 for the machine-readable half over the same failing tree.
// PREVENTS: a key renamed or retyped by the port, which the page comparison
// cannot see because the page reformats all three.
func TestPluginBoundaryJSONAgrees(t *testing.T) {
	dir := boundaryFixture(t)

	scriptOut, _, _ := runParityScriptIn(t, dir, "plugin_process_boundary.go", "--json")
	findings, err := pluginboundary.Check(dir, 0)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	commandRaw, err := json.Marshal(findings)
	if err != nil {
		t.Fatalf("marshal the findings: %v", err)
	}

	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptOut)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}
	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: AC-11 for the selftest half -- both sides pass over their fixtures
// and both answer 0.
// PREVENTS: a port whose selftest asserts less than the script's did, which
// would leave alias resolution unguarded while the gate still went green.
func TestPluginBoundarySelftestsAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "plugin_process_boundary.go", "--selftest")
	if scriptErr != "" {
		t.Fatalf("the selftest script wrote to stderr: %s", scriptErr)
	}

	payload, commandCode := pluginboundary.Answer([]string{"selftest"})
	report, ok := payload.(leroot.SelftestReport)
	if !ok {
		t.Fatalf("the selftest answered %T, want a SelftestReport", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the selftest script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the selftest pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
	// The script asserted seven properties -- four package fixtures and three
	// of the scan-root derivation. The port answers a row per property, so a
	// count below seven is coverage it dropped.
	if len(report.Results) != 7 {
		t.Errorf("the selftest answered %d case rows, want the script's seven properties", len(report.Results))
	}
}

// VALIDATES: the script STILL passes over a tree that carries its generator and
// none of the plugin roots it names, and the command refuses the same tree.
// PREVENTS: this test going quiet if somebody fixes the script. It asserts the
// DEFECT, so it reddens the day the script is repaired -- and the answer then is
// to delete this case with the script it describes.
//
// The defect: plugin_process_boundary.go skips any scan root it cannot stat
// (`if fi, statErr := os.Stat(root); statErr != nil || !fi.IsDir() {
// continue }`). These roots are its whole population. Over a tree with only
// scripts/codegen/plugin_imports.go, it reads no files. It then prints
// "plugin-process-boundary: OK" and exits 0. The port skips only a declared
// root that the tree does not carry. It answers all other stat failures and
// enforces the file-count floor pluginboundary.scanFloor.
func TestBoundaryScriptStillPassesOverATreeItNeverRead(t *testing.T) {
	bare := t.TempDir()
	generator := filepath.Join(bare, "scripts", "codegen", "plugin_imports.go")
	if err := os.MkdirAll(filepath.Dir(generator), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(generator), err)
	}
	source, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "codegen", "plugin_imports.go"))
	if err != nil {
		t.Fatalf("read the generator: %v", err)
	}
	if err := os.WriteFile(generator, source, 0o600); err != nil {
		t.Fatalf("write the generator copy: %v", err)
	}

	scriptOut, scriptErr, scriptCode := runParityScriptIn(t, bare, "plugin_process_boundary.go")
	if scriptCode != 0 || !strings.Contains(scriptOut, "plugin-process-boundary: OK") {
		t.Fatalf("the script no longer fails open (exit %d, %q, %q): delete this case and the script's fail-open row with it", scriptCode, scriptOut, scriptErr)
	}

	if _, err := pluginboundary.Check(bare, 400); err == nil {
		t.Error("the command passed over a tree it never read, so the port carries the same defect")
	}
}

// runParityScriptEnv runs one compiled script from the repository root with
// extra environment. This lets a gate use a fixture answer when an environment
// variable defines its scope.
func runParityScriptEnv(t *testing.T, environ []string, source string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), parityRunTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parityScript(t, source), args...)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), environ...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		code = 0
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Fatalf("run %s %v: %v (%s)", source, args, err, errOut.String())
	}
	return out.String(), errOut.String(), code
}

// scopeAnswer writes a feature-tag answer file and answers its path.
func scopeAnswer(t *testing.T, tags string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scope-tags")
	if err := os.WriteFile(path, []byte(tags), 0o600); err != nil {
		t.Fatalf("write the scope answer: %v", err)
	}
	return path
}

// VALIDATES: AC-11 for the feature matrix over the REAL manifest -- the
// script's --print-matrix and the command's rows action render the same
// document, which is what Staticcheck reads on stdin.
// PREVENTS: a row lost, renamed, or given different tags by the port. The
// document IS the population Staticcheck judges, so a difference here is a
// combination that stops being type-checked.
func TestStaticcheckMatrixRowsAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "staticcheck_feature_matrix.go", "--print-matrix")
	if scriptErr != "" || scriptCode != 0 {
		t.Fatalf("the script failed to print its matrix (exit %d): %s", scriptCode, scriptErr)
	}

	payload, commandCode := staticcheckmatrix.Answer([]string{"rows"})
	matrix, ok := payload.(staticcheckmatrix.Matrix)
	if !ok {
		t.Fatalf("the command answered %T, want a Matrix", payload)
	}
	if commandCode != 0 {
		t.Errorf("the rows action answers %d, want 0", commandCode)
	}
	if got := matrix.Text(); got != scriptOut {
		t.Errorf("the matrices differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
	if len(matrix) < 3 {
		t.Errorf("the matrix holds %d rows; a manifest declaring features owes more", len(matrix))
	}
}

// VALIDATES: AC-11 for the SCOPED half -- pointed at the same feature-tag
// answer, both sides judge the same rows and say the same thing about why.
// PREVENTS: a port that narrows differently, which would leave rows unjudged
// with the gate still green. The unscoped comparison above cannot see this:
// every row survives when nothing is subtracted.
func TestStaticcheckMatrixScopeAgrees(t *testing.T) {
	for _, testCase := range []struct {
		name string
		tags string
	}{
		{name: "a subset of the manifest", tags: "ze_web\nze_ssh\n"},
		{name: "a tag the manifest does not declare", tags: "ze_not_a_feature\n"},
		{name: "an empty answer", tags: ""},
	} {
		answer := scopeAnswer(t, testCase.tags)
		environ := []string{"ZE_VERIFY_SCOPE_TAGS=" + answer}

		scriptOut, scriptErr, scriptCode := runParityScriptEnv(t, environ, "staticcheck_feature_matrix.go", "--print-matrix")
		if scriptCode != 0 {
			t.Fatalf("%s: the script exits %d: %s", testCase.name, scriptCode, scriptErr)
		}

		matrix, notice, err := staticcheckmatrix.DeriveScoped(repoRoot(t), answer)
		if err != nil {
			t.Fatalf("%s: the command failed: %v", testCase.name, err)
		}
		if got := matrix.Text(); got != scriptOut {
			t.Errorf("%s: the scoped matrices differ.\nscript:\n%s\ncommand:\n%s", testCase.name, scriptOut, got)
		}
		if got := notice.Text(); got != scriptErr {
			t.Errorf("%s: the scope notices differ.\nscript:  %q\ncommand: %q", testCase.name, scriptErr, got)
		}
	}
}

// VALIDATES: AC-8 -- a matrix that could not be judged answers 2, apart from a
// tree that does not type-check, which answers 1.
// PREVENTS: a wrapper failure flattening into the gate's own verdict, which
// would read as "the tree is broken" when the truth is "nothing was checked".
func TestStaticcheckMatrixUnjudgeableAnswersTwo(t *testing.T) {
	_, scriptErr, scriptCode := runParityScript(t, "staticcheck_feature_matrix.go", "--deadline=nope")
	if scriptCode != 2 {
		t.Errorf("the script exits %d for an unusable deadline, want 2", scriptCode)
	}
	if !strings.Contains(scriptErr, "matrix could not be judged") {
		t.Errorf("the script's refusal does not say the matrix could not be judged: %s", scriptErr)
	}

	if _, err := staticcheckmatrix.DeadlineFrom("nope"); err == nil {
		t.Error("the command accepted an unusable deadline")
	} else if !strings.Contains(err.Error(), "matrix could not be judged") {
		t.Errorf("the command's refusal is %v, want one saying the matrix could not be judged", err)
	}
}

// dashStdioFixture writes a tree holding one CLI-tainted raw os call of each
// shape the taint analysis follows, plus the shapes it must NOT follow.
func dashStdioFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	const argset = "\ntype argset struct{}\nfunc (argset) Arg(int) string { return \"\" }\nfunc (argset) Args() []string { return nil }\n"
	files := map[string]string{
		"internal/component/direct/x.go": "package direct\n\nimport \"os\"\n\nfunc cmd(fs argset) { _, _ = os.ReadFile(fs.Arg(0)) }\n" + argset,
		"internal/plugins/funnel/x.go":   "package funnel\n\nimport \"os\"\n\nfunc load(p string) { _, _ = os.Open(p) }\n\nfunc cmd(fs argset) { load(fs.Arg(0)) }\n" + argset,
		"cmd/ze/derived/x.go":            "package derived\n\nimport \"os\"\n\nfunc cmd(fs argset) {\n\tp := fs.Arg(0)\n\t_, _ = os.ReadFile(p[1:])\n}\n" + argset,
		"internal/chaos/marked/x.go":     "package marked\n\nimport \"os\"\n\nfunc cmd(fs argset) { _, _ = os.ReadFile(fs.Arg(0)) } //cliio:allow never \"-\"\n" + argset,
	}
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	// The script walks every root it names and stops on one that is absent, so
	// each has to exist even when it holds nothing.
	for _, root := range []string{
		"internal/component", "internal/plugins", "internal/analyze", "internal/mrt",
		"internal/perf", "internal/appliance", "internal/test", "internal/chaos", "cmd/ze",
	} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(root)), 0o750); err != nil {
			t.Fatalf("create %s: %v", root, err)
		}
	}
	return dir
}

// VALIDATES: AC-11 for the dash-stdio gate over the REAL checkout -- the
// script's page and the command's page are the same text, and both answer 0.
// PREVENTS: a taint rule that drifted from the script's while both copies
// exist.
func TestDashStdioPagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "cli_dash_stdio.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr, so this checkout does not pass the gate: %s", scriptErr)
	}

	payload, commandCode := dashstdio.Answer([]string{"check"})
	findings, ok := payload.(dashstdio.Findings)
	if !ok {
		t.Fatalf("the command answered %T, want Findings", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := findings.Text(); got != scriptOut {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 over a tree that FAILS the gate -- the two report the same
// sites, in the same order, and both answer 1.
// PREVENTS: a comparison that only ever sees a clean tree. This checkout draws
// zero findings, so the alias chain, the funnel hop, the derived-path exclusion
// and the allow marker are exercised by this case and by no other.
//
// ONE difference is deliberate and is asserted rather than compared. The script
// writes the site list to stderr and the OK verdict to stdout. The command's
// sites ARE its answer, and every le answer is rendered on stdout.
func TestDashStdioFindingsAgreeOverAFailingTree(t *testing.T) {
	dir := dashStdioFixture(t)

	scriptOut, scriptErr, scriptCode := runParityScriptIn(t, dir, "cli_dash_stdio.go")
	if scriptOut != "" {
		t.Errorf("the script wrote a page to stdout over a failing tree: %s", scriptOut)
	}

	findings, err := dashstdio.Check(dir, 0)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	commandCode := 0
	if len(findings) > 0 {
		commandCode = 1
	}
	if scriptCode != 1 || commandCode != 1 {
		t.Fatalf("the script exits %d and the command answers %d over a tree holding two tainted calls", scriptCode, commandCode)
	}

	want := siteLines(scriptErr)
	got := siteLines(findings.Text())
	if len(want) != 2 {
		t.Fatalf("the script reported %d sites over the fixture, want 2: %v", len(want), want)
	}
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Errorf("the site lists differ.\nscript:\n%s\ncommand:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

// VALIDATES: AC-11 for the machine-readable half over the same failing tree.
// PREVENTS: a key renamed or retyped by the port, which the page comparison
// cannot see because the page reformats all four.
func TestDashStdioJSONAgrees(t *testing.T) {
	dir := dashStdioFixture(t)

	scriptOut, _, _ := runParityScriptIn(t, dir, "cli_dash_stdio.go", "--json")
	findings, err := dashstdio.Check(dir, 0)
	if err != nil {
		t.Fatalf("the command failed over the fixture: %v", err)
	}
	commandRaw, err := json.Marshal(findings)
	if err != nil {
		t.Fatalf("marshal the findings: %v", err)
	}

	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptOut)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}
	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: AC-11 for the selftest half -- both sides pass over their fixtures
// and both answer 0.
// PREVENTS: a port whose selftest asserts less than the script's did, which
// would leave the taint analysis unguarded while the gate still went green.
func TestDashStdioSelftestsAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "cli_dash_stdio.go", "--selftest")
	if scriptErr != "" {
		t.Fatalf("the selftest script wrote to stderr: %s", scriptErr)
	}

	payload, commandCode := dashstdio.Answer([]string{"selftest"})
	report, ok := payload.(leroot.SelftestReport)
	if !ok {
		t.Fatalf("the selftest answered %T, want a SelftestReport", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the selftest script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the selftest pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
	// The script ran fourteen fixtures. The port answers a row per fixture, so
	// a count below fourteen is coverage it dropped.
	if len(report.Results) != 14 {
		t.Errorf("the selftest answered %d case rows, want the script's fourteen fixtures", len(report.Results))
	}
}

// VALIDATES: AC-11 for the ci-dispatch gate over the REAL checkout -- the
// script's page and the command's page are the same text, and both answer 0.
// PREVENTS: an emitter shape, a rewrite rule or a file-selection rule that
// drifted from the script's while both copies exist. The page carries the count
// of commands and of emitters, so a rule that stopped firing changes the text
// even when it draws no row.
func TestCIDispatchPagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "ci_dispatch_commands.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr, so this checkout does not pass the gate: %s", scriptErr)
	}

	payload, commandCode := cidispatch.Answer([]string{"check"})
	report, ok := payload.(cidispatch.Report)
	if !ok {
		t.Fatalf("the command answered %T, want a Report", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
	if report.EmittersChecked == 0 {
		t.Error("the command checked no emitter, so the comparison is vacuous")
	}
}

// VALIDATES: AC-11 for the machine-readable half over the REAL checkout.
// PREVENTS: a key renamed or retyped by the port. The page prints three of the
// four counts and reformats every finding field, so a page comparison alone
// cannot see the JSON.
func TestCIDispatchJSONAgrees(t *testing.T) {
	scriptOut, scriptErr, _ := runParityScript(t, "ci_dispatch_commands.go", "--json")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr: %s", scriptErr)
	}

	payload, _ := cidispatch.Answer([]string{"check"})
	commandRaw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal the command's payload: %v", err)
	}

	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptOut)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}
	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: AC-11 for the selftest half -- both sides pass over their fixtures
// and both answer 0.
// PREVENTS: a port whose selftest asserts less than the script's did, which
// would leave the resolver and the recogniser unguarded while the gate still
// went green.
func TestCIDispatchSelftestsAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "ci_dispatch_commands.go", "--selftest")
	if scriptErr != "" {
		t.Fatalf("the selftest script wrote to stderr: %s", scriptErr)
	}

	payload, commandCode := cidispatch.Answer([]string{"selftest"})
	report, ok := payload.(leroot.SelftestReport)
	if !ok {
		t.Fatalf("the selftest answered %T, want a SelftestReport", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the selftest script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the selftest pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
	// The script asserted ten properties and stopped at the first that broke.
	// The port answers a row per property, so a count below ten is coverage it
	// dropped.
	if len(report.Results) != 10 {
		t.Errorf("the selftest answered %d case rows, want the script's ten properties", len(report.Results))
	}
}

// VALIDATES: the script STILL passes over a tree holding none of the six roots
// it walks, and the command refuses the same tree.
// PREVENTS: this test going quiet if somebody fixes the script. It asserts the
// DEFECT, so it reddens the day the script is repaired -- and the answer then is
// to delete this case with the script it describes.
//
// The defect: ci_dispatch_commands.go walks `test`, `internal`, `cmd`, `pkg`,
// `scripts`, and `demos` relative to the working directory. It answers each
// walk error with `return nil` and also skips files that it cannot read. Its
// resolver comes from the LINKED registry, so the resolver is never empty.
// Thus, the "no commands registered" guard never fires. In an empty directory,
// the script prints `Emitters checked: 0` and `ci-dispatch-check: OK`, then
// exits 0. The port enforces cidispatch.emitterFloor and answers every walk and
// read error.
func TestCIDispatchScriptStillPassesOverATreeItNeverRead(t *testing.T) {
	empty := t.TempDir()

	scriptOut, scriptErr, scriptCode := runParityScriptIn(t, empty, "ci_dispatch_commands.go")
	if scriptCode != 0 || !strings.Contains(scriptOut, "ci-dispatch-check: OK") {
		t.Fatalf("the script no longer fails open (exit %d, %q, %q): delete this case and the script's fail-open row with it", scriptCode, scriptOut, scriptErr)
	}
	if !strings.Contains(scriptOut, "Emitters checked:    0") {
		t.Errorf("the script no longer reports zero emitters over an empty tree: %s", scriptOut)
	}

	if _, err := cidispatch.Check(empty, 300); err == nil {
		t.Error("the command passed over a tree it never read, so the port carries the same defect")
	}
}

// normaliseTiming replaces the elapsed-time column of a tracked-build page so
// two runs of the same tree compare. The timings are what the two sides CANNOT
// agree on: one run warms the build cache for the other.
var normaliseTiming = regexp.MustCompile(`\s+\d+\.\ds`)

// VALIDATES: AC-11 for the tracked-build gate over the REAL checkout -- the
// script's page and the command's page carry the same commit line, the same
// flavor rows and the same verdict, and both answer 0.
// PREVENTS: a flavor lost, a tag set changed, or an anchor rule dropped by the
// port. The row carries the package count and the tag spec, so a flavor that
// stopped compiling what it exists to compile changes the text.
func TestTrackedBuildPagesAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "tracked_build.go")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr, so this commit does not compile: %s", scriptErr)
	}

	payload, commandCode := trackedbuild.Answer([]string{"check"})
	report, ok := payload.(trackedbuild.Report)
	if !ok {
		t.Fatalf("the command answered %T, want a Report", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}

	want := normaliseTiming.ReplaceAllString(scriptOut, " TIME")
	got := normaliseTiming.ReplaceAllString(report.Text(), " TIME")
	if want != got {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", want, got)
	}
	if len(report.Results) < 6 {
		t.Errorf("the command judged %d flavors, want the six shipped ones", len(report.Results))
	}
	for _, result := range report.Results {
		if result.Packages < trackedbuild.DefaultPackageFloor {
			t.Errorf("flavor %s selected %d packages, below the floor: the comparison is vacuous", result.Name, result.Packages)
		}
	}
}

// VALIDATES: AC-11 for the machine-readable half over the REAL checkout.
// PREVENTS: a key renamed or retyped by the port. The page prints four of the
// eight report fields and three of the eight result fields.
//
// Two fields are EXCLUDED, and the test asserts the exclusion. `tree` is a
// per-process scratch path, and `seconds` is a measurement. These fields cannot
// be equal across two runs. Every other key must be equal.
func TestTrackedBuildJSONAgrees(t *testing.T) {
	scriptOut, scriptErr, _ := runParityScript(t, "tracked_build.go", "--json")
	if scriptErr != "" {
		t.Fatalf("the script wrote to stderr: %s", scriptErr)
	}

	payload, _ := trackedbuild.Answer([]string{"check"})
	commandRaw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal the command's payload: %v", err)
	}

	fromScript := decodeReport(t, "the script's --json", []byte(scriptOut))
	fromCommand := decodeReport(t, "the command's payload", commandRaw)
	if _, ok := fromScript["tree"]; !ok {
		t.Error("the script's report has no tree key, so the exclusion below is stale")
	}
	delete(fromScript, "tree")
	delete(fromCommand, "tree")
	dropSeconds(t, fromScript)
	dropSeconds(t, fromCommand)

	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// decodeReport reads one side's report into a map.
func decodeReport(t *testing.T, what string, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s is not a JSON object: %v\n%s", what, err, raw)
	}
	return out
}

// dropSeconds removes the measured elapsed time from every flavor row, which is
// the one field two runs of the same tree cannot agree on.
func dropSeconds(t *testing.T, report map[string]any) {
	t.Helper()
	rows, ok := report["results"].([]any)
	if !ok {
		t.Fatalf("the report has no results array: %v", report["results"])
	}
	for _, row := range rows {
		result, ok := row.(map[string]any)
		if !ok {
			t.Fatalf("a result row is %T, want an object", row)
		}
		if _, ok := result["seconds"]; !ok {
			t.Error("a result row has no seconds key, so the exclusion is stale")
		}
		delete(result, "seconds")
	}
}

// VALIDATES: AC-11 for the build matrix -- the script's --matrix and the
// command's matrix action name the same flavors, with the same tags and the
// same anchor files.
// PREVENTS: the two sides compiling different flavors, which every other
// comparison here would then be blind to.
//
// The documents are compared FIELD BY FIELD instead of byte for byte. This is
// a deliberate correction. The script's tagSet has no JSON tags, so --matrix
// uses Go field names such as `AnchorFiles`. Every other repository answer uses
// kebab-case. The port uses the repository form. Thus, the KEYS differ on
// purpose, but the VALUES must not.
func TestTrackedBuildMatrixAgrees(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "tracked_build.go", "--matrix")
	if scriptErr != "" || scriptCode != 0 {
		t.Fatalf("the script failed to print its matrix (exit %d): %s", scriptCode, scriptErr)
	}

	var fromScript []struct {
		Name        string   `json:"Name"`
		Tags        []string `json:"Tags"`
		Features    bool     `json:"Features"`
		GOOS        string   `json:"GOOS"`
		Anchor      string   `json:"Anchor"`
		AnchorFiles []string `json:"AnchorFiles"`
		Why         string   `json:"Why"`
	}
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script's --matrix is not JSON: %v\n%s", err, scriptOut)
	}

	payload, code := trackedbuild.Answer([]string{"matrix"})
	matrix, ok := payload.(trackedbuild.Matrix)
	if !ok || code != 0 {
		t.Fatalf("the matrix action answered %T with code %d", payload, code)
	}
	if len(matrix) != len(fromScript) {
		t.Fatalf("the script names %d flavors and the command %d", len(fromScript), len(matrix))
	}
	for i, want := range fromScript {
		got := matrix[i]
		if got.Name != want.Name || got.Anchor != want.Anchor || got.GOOS != want.GOOS ||
			got.Features != want.Features || got.Why != want.Why {
			t.Errorf("flavor %d differs.\nscript:  %+v\ncommand: %+v", i, want, got)
		}
		if strings.Join(got.Tags, ",") != strings.Join(want.Tags, ",") {
			t.Errorf("flavor %s carries tags %v, the script %v", got.Name, got.Tags, want.Tags)
		}
		if strings.Join(got.AnchorFiles, ",") != strings.Join(want.AnchorFiles, ",") {
			t.Errorf("flavor %s anchors on %v, the script on %v", got.Name, got.AnchorFiles, want.AnchorFiles)
		}
	}
}

// VALIDATES: AC-11 for the selftest half -- both sides pass over their fixtures
// and both answer 0.
// PREVENTS: a port whose selftest asserts less than the script's did, which
// would leave the two vacuity guards unproven while the gate still went green.
func TestTrackedBuildSelftestsAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "tracked_build.go", "--selftest")
	if scriptErr != "" {
		t.Fatalf("the selftest script wrote to stderr: %s", scriptErr)
	}

	payload, commandCode := trackedbuild.Answer([]string{"selftest"})
	report, ok := payload.(leroot.SelftestReport)
	if !ok {
		t.Fatalf("the selftest answered %T, want a SelftestReport", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the selftest script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the selftest pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
	// The script asserted seven properties and stopped at the first that broke.
	// The port answers a row per property, so a count below seven is coverage it
	// dropped.
	if len(report.Results) != 7 {
		t.Errorf("the selftest answered %d case rows, want the script's seven properties", len(report.Results))
	}
}

// VALIDATES: both sides refuse a revision that names no commit, with the same
// code.
// PREVENTS: a bad revision reading as a broken commit, which sends a reader
// hunting an uncommitted producer that does not exist.
func TestTrackedBuildRefusesABadRevisionTheSameWay(t *testing.T) {
	_, scriptErr, scriptCode := runParityScript(t, "tracked_build.go", "--rev=refs/heads/no-such-branch")
	if scriptCode != 2 {
		t.Errorf("the script exits %d for an unknown revision, want 2", scriptCode)
	}
	if !strings.Contains(scriptErr, "does not name a commit") {
		t.Errorf("the script's refusal does not say why: %s", scriptErr)
	}

	options, err := trackedbuild.OptionsFrom("refs/heads/no-such-branch", "", "", "1m")
	if err != nil {
		t.Fatalf("the options were refused: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), parityRunTimeout)
	defer cancel()
	_, code, err := trackedbuild.Run(ctx, repoRoot(t), options)
	if code != 2 {
		t.Errorf("the command answers %d for an unknown revision, want 2", code)
	}
	if err == nil || !strings.Contains(err.Error(), "does not name a commit") {
		t.Errorf("the command's refusal is %v, want one saying the revision names no commit", err)
	}
}

// VALIDATES: AC-11 for the test-sensitivity ratchet over the REAL checkout --
// the script's verdict line and the command's are the same text, both write the
// same slack notice, and both answer 0.
// PREVENTS: a detector that credits differently in the port. The verdict line
// carries both counts and the file total, so a detector that stopped firing
// changes the text.
func TestTestSensitivityChecksAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "inert_tests.go", "--check")

	payload, commandCode := testsensitivity.Answer([]string{"check"})
	verdict, ok := payload.(testsensitivity.Verdict)
	if !ok {
		t.Fatalf("the command answered %T, want a Verdict", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := verdict.Text(); got != scriptOut {
		t.Errorf("the verdict lines differ.\nscript:  %q\ncommand: %q", scriptOut, got)
	}
	if got := verdict.Breach(); got != scriptErr {
		t.Errorf("the breach notices differ.\nscript:  %q\ncommand: %q", scriptErr, got)
	}
	if verdict.Result.FilesScanned == 0 || verdict.Result.TestsScanned == 0 {
		t.Error("the command scanned nothing, so the comparison is vacuous")
	}
}

// VALIDATES: AC-11 for the REPORT page over the working tree -- both sides list
// the same assert-nothing tests and the same tag orphans, in the same order.
// PREVENTS: a finding lost or a line renumbered by the port. The verdict line
// above carries only the counts, so two different lists of the same length
// would pass it.
func TestTestSensitivityReportsAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "inert_tests.go")
	if scriptErr != "" || scriptCode != 0 {
		t.Fatalf("the script failed to report (exit %d): %s", scriptCode, scriptErr)
	}

	payload, commandCode := testsensitivity.Answer([]string{"report"})
	result, ok := payload.(testsensitivity.Result)
	if !ok {
		t.Fatalf("the command answered %T, want a Result", payload)
	}
	if commandCode != 0 {
		t.Errorf("the report action answers %d, want 0", commandCode)
	}
	if got := result.Text(); got != scriptOut {
		t.Errorf("the pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
}

// VALIDATES: AC-11 for the TRACKED population, which is the one the generated
// test-health page reads.
// PREVENTS: the published page moving because the port read a different
// population. The working-tree comparison above cannot see this: the two
// populations differ only when something is uncommitted.
func TestTestSensitivityTrackedJSONAgrees(t *testing.T) {
	scriptOut, _, _ := runParityScript(t, "inert_tests.go", "--json", "--tracked-only")

	payload, commandCode := testsensitivity.Answer([]string{"tracked"})
	if commandCode != 0 {
		t.Errorf("the tracked action answers %d, want 0", commandCode)
	}
	commandRaw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal the command's payload: %v", err)
	}

	var fromScript, fromCommand any
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script's --json is not JSON: %v\n%s", err, scriptOut)
	}
	if err := json.Unmarshal(commandRaw, &fromCommand); err != nil {
		t.Fatalf("the command's payload is not JSON: %v\n%s", err, commandRaw)
	}
	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: AC-11 for the machine-readable half of the CHECK -- both sides
// answer the same document, and the verdict travels in it.
// PREVENTS: the guard whose only enforcement path could never deny. `valid` used
// to be set unconditionally, so `--json --check` reported findings and exited 0.
func TestTestSensitivityCheckJSONAgrees(t *testing.T) {
	scriptOut, _, _ := runParityScript(t, "inert_tests.go", "--check", "--json")

	payload, _ := testsensitivity.Answer([]string{"check"})
	commandRaw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal the command's payload: %v", err)
	}

	fromScript := decodeReport(t, "the script's --check --json", []byte(scriptOut))
	fromCommand := decodeReport(t, "the command's payload", commandRaw)
	if valid, ok := fromScript["valid"].(bool); !ok || !valid {
		t.Fatalf("the script's document does not carry a true valid flag: %v", fromScript["valid"])
	}
	scriptCanonical, _ := json.Marshal(fromScript)   //nolint:errcheck // it just unmarshaled
	commandCanonical, _ := json.Marshal(fromCommand) //nolint:errcheck // it just unmarshaled
	if !bytes.Equal(scriptCanonical, commandCanonical) {
		t.Errorf("the JSON documents differ.\nscript:  %s\ncommand: %s", scriptCanonical, commandCanonical)
	}
}

// VALIDATES: AC-11 for the selftest half -- both sides pass over their fixtures
// and both answer 0.
// PREVENTS: a port whose selftest asserts less than the script's did, which
// would leave both detectors unguarded while the gate still went green.
func TestTestSensitivitySelftestsAgree(t *testing.T) {
	scriptOut, scriptErr, scriptCode := runParityScript(t, "inert_tests.go", "--selftest")
	if scriptErr != "" {
		t.Fatalf("the selftest script wrote to stderr: %s", scriptErr)
	}

	payload, commandCode := testsensitivity.Answer([]string{"selftest"})
	report, ok := payload.(leroot.SelftestReport)
	if !ok {
		t.Fatalf("the selftest answered %T, want a SelftestReport", payload)
	}
	if scriptCode != commandCode {
		t.Errorf("the selftest script exits %d and the command answers %d", scriptCode, commandCode)
	}
	if got := report.Text(); got != scriptOut {
		t.Errorf("the selftest pages differ.\nscript:\n%s\ncommand:\n%s", scriptOut, got)
	}
	// The script ran 27 assert-nothing fixtures, 3 cross-package fixtures, 15 tag
	// fixtures, and one make-variable expansion. It stopped at the first failure.
	// The port answers one row for each property. A count less than 46 means that
	// the port dropped coverage.
	if len(report.Results) != 46 {
		t.Errorf("the selftest answered %d case rows, want the script's 46 properties", len(report.Results))
	}
}
