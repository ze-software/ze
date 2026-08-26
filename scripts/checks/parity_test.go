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
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/letools/commandownership"
	"github.com/ze-software/ze/letools/configclaims"
	"github.com/ze-software/ze/letools/configcoercion"
	"github.com/ze-software/ze/letools/ifaceresolution"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/portdefaults"
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
