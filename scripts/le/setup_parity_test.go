// The migration's proof for `le setup`: the script and the command probe the
// same machine, would run the same commands, and write the same page.
//
// internal/le/devsetup replaces scripts/le/application/setup.py. Both versions
// remain until the swap. This file is deliberately HERE because it is a
// migration artifact. The commit that deletes the script also deletes this
// proof.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- for every mode that exists, the two
// halves exec the same external commands with the same argv in the same order,
// write byte-identical pages, and answer the same exit code.
// PREVENTS: a dev-setup that probes for a tool the other half does not, or
// installs through a route the other half spells differently. Neither is visible
// in a summary line: both halves would still say "Setup complete".
//
// NOTHING IS INSTALLED. Every external binary is a recording stand-in on PATH.
// The test uses a fixture checkout and HOME. It compares the ARGV that each half
// sends outside the process and the page that each half writes.
//
// ONE DELIBERATE AREA IS OUTSIDE THE STAND-INS. Three Linux system-state steps
// write to a real kernel setting, group database, and loopback interface. Both
// halves READ and agree on that state. The probe-only case compares these rows.
// This test does not start their apply branches because that would change the
// test machine. Tests in internal/le/devsetup pin their argv through a struct-field
// seam.

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/devsetup"
)

// setupStandIn records its argv and prints the configured response. It fails
// when the case includes that exact argv in the failure list. One script serves
// every executable name. It also records the invoked name so the result
// identifies the binary.
//
// SHELL BUILTINS ONLY. PATH holds nothing but the stand-ins themselves, which is
// the whole point of the fixture, so basename, cat and grep are all absent here.
// Reaching for one made every stand-in fail with "not found" and the failure
// read as a broken probe.
const setupStandIn = `#!/bin/sh
name=${0##*/}
printf '%s' "$name" >> "$LE_SETUP_CALLS"
for arg in "$@"; do printf ' %s' "$arg" >> "$LE_SETUP_CALLS"; done
printf '\n' >> "$LE_SETUP_CALLS"
if [ -f "$LE_SETUP_OUT/$name" ]; then
	while IFS= read -r line; do printf '%s\n' "$line"; done < "$LE_SETUP_OUT/$name"
fi
if [ -n "$LE_SETUP_FAIL" ] && [ -f "$LE_SETUP_FAIL" ]; then
	while IFS= read -r line; do
		if [ "$line" = "$name $*" ]; then exit 9; fi
	done < "$LE_SETUP_FAIL"
fi
exit 0
`

// pythonBin is the interpreter for the script half. Package initialization
// resolves it before each case replaces PATH with stand-ins, including python3.
// Per-call resolution would find the previous case's stand-in and run nothing.
var pythonBin = func() string {
	found, err := exec.LookPath("python3")
	if err != nil {
		return "python3"
	}
	return found
}()

// everyBinary lists each executable name that either half searches for on PATH.
// It combines the tool probes, two language servers, two package managers, and
// sudo. A case selects the available machine tools by naming a subset.
var everyBinary = []string{
	"go", "git", "protoc", "jq", "golangci-lint", "staticcheck", "goimports",
	"gopls", "python3", "qemu-system-x86_64", "qemu-system-aarch64", "xorriso",
	"grub-mkstandalone", "pipx", "uv", "ruff", "mypy", "pyright",
	"pyright-langserver", "sshpass",
	"docker", "colima", "xl2tpd", "pppd", "brew", "apt-get", "sudo",
}

// machine is one fixture host: a checkout, a HOME, a PATH holding the chosen
// stand-ins, and the two directories the stand-ins read.
type machine struct {
	root    string
	home    string
	path    string
	answers string
	failing string
	brew    string
}

// newMachine builds a fixture host carrying exactly these binaries.
func newMachine(t *testing.T, binaries []string) *machine {
	t.Helper()

	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module x\n\ngo 1.26\n")
	write(t, filepath.Join(root, "feature-gates.txt"), "ze_bgp\tBGP\n")

	path := t.TempDir()
	for _, name := range binaries {
		file := filepath.Join(path, name)
		if err := os.WriteFile(file, []byte(setupStandIn), 0o700); err != nil { //nolint:gosec // an executable stand-in must be executable
			t.Fatalf("write %s: %v", file, err)
		}
	}

	return &machine{root: root, home: t.TempDir(), path: path, answers: t.TempDir()}
}

// withE2fsprogs gives the host a Homebrew prefix carrying both e2fsprogs tools.
//
// Neither half resolves this probe through PATH. It searches directories that
// end with /usr/sbin and /sbin. Thus, a case cannot make the probe ABSENT on a
// host that has it. A clean-run case makes it present in every search directory
// (probes.go, E2fsprogsDirs).
func (m *machine) withE2fsprogs(t *testing.T) *machine {
	t.Helper()
	m.brew = t.TempDir()
	sbin := filepath.Join(m.brew, "opt", "e2fsprogs", "sbin")
	if err := os.MkdirAll(sbin, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", sbin, err)
	}
	for _, name := range []string{"mkfs.ext4", "debugfs"} {
		write(t, filepath.Join(sbin, name), "")
	}
	return m
}

// answering makes the named stand-in print body when it runs.
func (m *machine) answering(t *testing.T, name, body string) {
	t.Helper()
	write(t, filepath.Join(m.answers, name), body)
}

// failingOn makes the stand-ins fail for exactly these argv lines, each spelled
// the way the stand-in records it.
func (m *machine) failingOn(t *testing.T, lines ...string) *machine {
	t.Helper()
	m.failing = filepath.Join(t.TempDir(), "failing")
	write(t, m.failing, strings.Join(lines, "\n")+"\n")
	return m
}

// plugins writes a harness plugin record listing these qualified plugin names.
func (m *machine) plugins(t *testing.T, names ...string) *machine {
	t.Helper()
	record := map[string]map[string]any{"plugins": {}}
	for _, name := range names {
		record["plugins"][name] = map[string]any{}
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal the plugin record: %v", err)
	}
	dir := filepath.Join(m.home, ".claude", "plugins")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	write(t, filepath.Join(dir, "installed_plugins.json"), string(raw))
	return m
}

// run drives one mode through both halves and answers what each wrote, what each
// would have run, and what each exited with.
type setupRun struct {
	scriptPage, portedPage   string
	scriptCalls, portedCalls []string
	scriptCode, portedCode   int
}

// drive runs one mode on both sides of the migration.
//
// The script half is a subprocess because a Python tool uses the process
// boundary. The test calls the ported half in process because it is a function.
// This is the purpose of the port. A second binary would compare the dispatcher
// instead of the tool.
func driveSetup(t *testing.T, m *machine, check, vendor bool) setupRun {
	t.Helper()
	var got setupRun

	var flags []string
	if check {
		flags = append(flags, "--check")
	}
	if !vendor {
		flags = append(flags, "--no-vendor")
	}

	scriptCalls := filepath.Join(t.TempDir(), "calls")
	got.scriptPage, got.scriptCode = runSetupScript(t, m, flags, scriptCalls)
	got.scriptCalls = readCalls(t, scriptCalls)

	portedCalls := filepath.Join(t.TempDir(), "calls")
	t.Setenv("PATH", m.path)
	t.Setenv("HOME", m.home)
	t.Setenv("LE_SETUP_CALLS", portedCalls)
	t.Setenv("LE_SETUP_OUT", m.answers)
	t.Setenv("LE_SETUP_FAIL", m.failing)
	t.Setenv("HOMEBREW_PREFIX", m.brew)

	report, code := (&devsetup.Setup{Root: m.root, Check: check, Vendor: vendor}).Run()
	got.portedPage, got.portedCode = report.Text(), code
	got.portedCalls = readCalls(t, portedCalls)
	return got
}

// runSetupScript runs the Python half in its own process, over the fixture host.
func runSetupScript(t *testing.T, m *machine, flags []string, calls string) (string, int) {
	t.Helper()
	argv := append([]string{"-m", "le.application.setup"}, flags...)
	cmd := exec.CommandContext(t.Context(), pythonBin, argv...)
	cmd.Dir = m.root
	cmd.Env = []string{
		"PYTHONPATH=" + filepath.Join(repoRoot(t), "scripts"),
		"ZE_REPO_ROOT=" + m.root,
		"PATH=" + m.path,
		"HOME=" + m.home,
		"LE_SETUP_CALLS=" + calls,
		"LE_SETUP_OUT=" + m.answers,
		"LE_SETUP_FAIL=" + m.failing,
		"HOMEBREW_PREFIX=" + m.brew,
	}

	stdout, err := cmd.Output()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !asExitError(err, &exit) {
			t.Fatalf("run the script: %v", err)
		}
		code = exit.ExitCode()
	}
	return string(stdout), code
}

// agree asserts that both halves did and said the same thing.
func agree(t *testing.T, got setupRun) {
	t.Helper()
	if got.scriptPage != got.portedPage {
		t.Errorf("the two halves wrote different pages\nscript:\n%s\ncommand:\n%s", got.scriptPage, got.portedPage)
	}
	if !slices.Equal(got.scriptCalls, got.portedCalls) {
		t.Errorf("the two halves ran different commands\nscript:\n%s\ncommand:\n%s",
			strings.Join(got.scriptCalls, "\n"), strings.Join(got.portedCalls, "\n"))
	}
	if got.scriptCode != got.portedCode {
		t.Errorf("the two halves exited %d and %d", got.scriptCode, got.portedCode)
	}
}

// TestAnUnsupportedPlatformAgrees pins the one branch that runs no step at all:
// no package manager, so both halves name every tool by hand and fail.
func TestAnUnsupportedPlatformAgrees(t *testing.T) {
	got := driveSetup(t, newMachine(t, nil), true, true)
	agree(t, got)
	if !strings.Contains(got.portedPage, "Unsupported platform") {
		t.Errorf("no unsupported-platform verdict in:\n%s", got.portedPage)
	}
	if !strings.Contains(got.portedPage, "Manual installation required") {
		t.Errorf("the manual list is missing from:\n%s", got.portedPage)
	}
	if got.portedCode == 0 {
		t.Error("a host that can install nothing reported success")
	}
}

// TestAProbeOnlyRunAgrees is the mode every doc names and the one a developer
// types. Nothing is on PATH but apt-get, so every row is a failure and the page
// is the whole tool table with its verdicts.
func TestAProbeOnlyRunAgrees(t *testing.T) {
	got := driveSetup(t, newMachine(t, []string{"apt-get"}), true, true)
	agree(t, got)
	if got.portedCode == 0 {
		t.Error("a host missing every tool reported success")
	}
	if len(got.portedCalls) != 0 {
		t.Errorf("check mode ran a command: %s", strings.Join(got.portedCalls, "\n"))
	}
}

// TestAProbeOnlyRunAgreesOnAFullMachine is the other end of the same mode: every
// probe answers, both servers answer, both plugins are installed. The rows a
// broken machine never reaches are the ones this compares.
func TestAProbeOnlyRunAgreesOnAFullMachine(t *testing.T) {
	m := fullMachine(t)
	got := driveSetup(t, m, true, true)
	agree(t, got)
	if !strings.Contains(got.portedPage, "gopls-answers (3 symbols") {
		t.Errorf("gopls did not answer in:\n%s", got.portedPage)
	}
	// The wording below is the SCRIPT's, letter for letter, and the two pages
	// are compared byte for byte until step 14 deletes it.
	if !strings.Contains(got.portedPage, "pyright-answers (7 file analysed") { //nolint:misspell // the script's wording
		t.Errorf("pyright did not answer in:\n%s", got.portedPage)
	}
}

// fullMachine represents a host where every probe succeeds. Each binary is a
// stand-in. staticcheck returns the pinned version, gopls returns symbols, and
// pyright returns a summary. Both harness plugins are also recorded.
func fullMachine(t *testing.T) *machine {
	t.Helper()
	m := newMachine(t, everyBinary).withE2fsprogs(t)
	probeDir := filepath.Join(m.root, "internal", "core", "clock")
	if err := os.MkdirAll(probeDir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", probeDir, err)
	}
	write(t, filepath.Join(probeDir, "clock.go"), "package clock\n")

	var version strings.Builder
	version.WriteString("staticcheck ")
	version.WriteString(devsetup.StaticcheckVersion)
	version.WriteString(" (v0.7.0)\n")

	m.answering(t, "staticcheck", version.String())
	m.answering(t, "gopls", "Now Function 20:6-20:9\nAdd Method 30:1-30:4\nSub Method 40:1-40:4\n")
	// nodeenv writes this preamble on its first run. It is a Python dictionary
	// representation with single quotes before an otherwise valid JSON response.
	m.answering(t, "pyright", "{'python': '/x/bin/python'}\n{\"summary\": {\"filesAnalyzed\": 7}}\n")
	m.plugins(t, "gopls-lsp@claude-plugins-official", "pyright-lsp@claude-plugins-official")
	return m
}

// TestAnAptInstallRunsTheSameCommands proves the argv for the system route. Only
// apt-get and sudo exist. Thus, each tool with a Debian package takes this route,
// and every other route reports why it is unavailable.
func TestAnAptInstallRunsTheSameCommands(t *testing.T) {
	got := driveSetup(t, newMachine(t, []string{"apt-get", "sudo"}), false, false)
	agree(t, got)

	if !slices.Contains(got.portedCalls, "sudo -n apt-get update") {
		t.Errorf("no apt-get update in:\n%s", strings.Join(got.portedCalls, "\n"))
	}
	want := "sudo -n env DEBIAN_FRONTEND=noninteractive apt-get install -y golang-go"
	if !slices.Contains(got.portedCalls, want) {
		t.Errorf("no %q in:\n%s", want, strings.Join(got.portedCalls, "\n"))
	}
	updates := 0
	for _, call := range got.portedCalls {
		if call == "sudo -n apt-get update" {
			updates++
		}
	}
	if updates != 1 {
		t.Errorf("apt-get update ran %d times; one per run is the contract", updates)
	}
}

// TestTheGoAndPipxRoutesRunTheSameCommands is the argv proof for the two routes
// that work on both platforms and so win over the package manager.
func TestTheGoAndPipxRoutesRunTheSameCommands(t *testing.T) {
	got := driveSetup(t, newMachine(t, []string{"apt-get", "sudo", "go", "pipx"}), false, false)
	agree(t, got)

	for _, want := range []string{
		"go install golang.org/x/tools/gopls@latest",
		"pipx install --force uv",
		"pipx install --force pyright",
	} {
		if !slices.Contains(got.portedCalls, want) {
			t.Errorf("no %q in:\n%s", want, strings.Join(got.portedCalls, "\n"))
		}
	}
	// A tool that declares one of these versions must NOT use the system route.
	// This keeps the pinned version equal on both platforms.
	for _, call := range got.portedCalls {
		if strings.Contains(call, "apt-get install -y golangci-lint") {
			t.Errorf("golangci-lint took the apt route: %s", call)
		}
	}
}

// TestVendoringRunsTheSameCommands pins the last step of an install run.
func TestVendoringRunsTheSameCommands(t *testing.T) {
	got := driveSetup(t, fullMachine(t), false, true)
	agree(t, got)

	for _, want := range []string{"go mod tidy", "go mod vendor"} {
		if !slices.Contains(got.portedCalls, want) {
			t.Errorf("no %q in:\n%s", want, strings.Join(got.portedCalls, "\n"))
		}
	}
	if got.portedCode != 0 {
		t.Errorf("a full machine reported %d:\n%s", got.portedCode, got.portedPage)
	}
}

// TestTheScriptStillReportsSuccessAfterAFailedVendor is the defect the port
// closes, held so that it reddens the day the script is repaired.
//
// le.application.setup.action discards the answer from vendor_go_deps().
// Therefore, a failed `go mod vendor` changes neither the page verdict nor the
// exit code. The run ends "Setup complete" while vendor/ disagrees with go.mod,
// and the tree does not build. This assertion compares the same host with and
// without the failure. It does not claim that the test machine is clean.
func TestTheScriptStillReportsSuccessAfterAFailedVendor(t *testing.T) {
	sound := driveSetup(t, fullMachine(t), false, true)
	broken := driveSetup(t, fullMachine(t).failingOn(t, "go mod vendor"), false, true)

	if broken.scriptCode != sound.scriptCode {
		t.Errorf("the script now answers %d for a failed vendor and %d for a sound one:"+
			" it has been repaired, so delete this case rather than weakening it",
			broken.scriptCode, sound.scriptCode)
	}
	if strings.Contains(broken.scriptPage, "Vendoring failed") {
		t.Error("the script now names the failed vendor: delete this case rather than weakening it")
	}
	if !strings.Contains(broken.scriptPage, "Setup complete") {
		t.Errorf("the script no longer reports success after a failed vendor:\n%s", broken.scriptPage)
	}

	if broken.portedCode == sound.portedCode {
		t.Errorf("the port answers %d either way: a failed vendor is not being reported", broken.portedCode)
	}
	if !strings.Contains(broken.portedPage, "Vendoring failed") {
		t.Errorf("the port did not name the failed vendor:\n%s", broken.portedPage)
	}
}

// --- The tables, compared by VALUE ------------------------------------------
//
// These cases cannot expose a row missing from one half. An unused probe reports
// nothing, and an apt spelling difference appears only on a host that uses that
// route. Thus, the test reads both tables and compares every field.

// pythonTools reads the script's whole tool table as JSON.
func pythonTools(t *testing.T) []map[string]any {
	t.Helper()
	raw := python(t, repoRoot(t), `
import json
from le.devtools.tools import ALL_TOOLS
print(json.dumps([
    {
        'name': t.name,
        'probe': list(t.probe),
        'probe_any': t.probe_any,
        'brew': t.brew or '',
        'apt': t.apt or '',
        'go_install': t.go_install or '',
        'pipx_install': t.pipx_install or '',
        'required': t.required,
        'note': t.note,
    }
    for t in ALL_TOOLS
]))
`)
	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode the script's tool table: %v", err)
	}
	return rows
}

// TestTheToolTableIsTheSameOnBothSides compares every field of every row.
func TestTheToolTableIsTheSameOnBothSides(t *testing.T) {
	scripted := pythonTools(t)
	ported := devsetup.AllTools()

	if len(scripted) != len(ported) {
		t.Fatalf("the script has %d tools and the command has %d", len(scripted), len(ported))
	}
	if len(ported) == 0 {
		t.Fatal("both tables are empty: the comparison is vacuous")
	}

	for i, want := range scripted {
		got := ported[i]
		if want["name"] != got.Name {
			t.Errorf("row %d: the script calls it %v and the command calls it %s", i, want["name"], got.Name)
			continue
		}
		checkField(t, got.Name, "probe", want["probe"], stringsAsAny(got.Probe))
		checkField(t, got.Name, "probe_any", want["probe_any"], got.ProbeAny)
		checkField(t, got.Name, "brew", want["brew"], got.Brew)
		checkField(t, got.Name, "apt", want["apt"], got.Apt)
		checkField(t, got.Name, "go_install", want["go_install"], got.GoInstall)
		checkField(t, got.Name, "pipx_install", want["pipx_install"], got.PipxInstall)
		checkField(t, got.Name, "required", want["required"], got.Required)
		checkField(t, got.Name, "note", want["note"], got.Note)
	}
}

// stringsAsAny is what a JSON list of strings decodes to, so the two sides
// compare as the same type.
func stringsAsAny(list []string) []any {
	out := make([]any, 0, len(list))
	for _, item := range list {
		out = append(out, item)
	}
	return out
}

// checkField reports one field of one row that the two halves disagree about.
func checkField(t *testing.T, row, field string, want, got any) {
	t.Helper()
	if wanted, ok := want.([]any); ok {
		gotten, ok := got.([]any)
		if !ok || !slices.Equal(wanted, gotten) {
			t.Errorf("%s.%s: the script has %v and the command has %v", row, field, want, got)
		}
		return
	}
	if want != got {
		t.Errorf("%s.%s: the script has %v and the command has %v", row, field, want, got)
	}
}

// TestTheGrubTableIsTheSameOnBothSides reads the script's whole architecture map
// rather than the one answer this host gets. The host's own answer is one row of
// four, and it is the row least likely to be wrong.
func TestTheGrubTableIsTheSameOnBothSides(t *testing.T) {
	raw := python(t, repoRoot(t), `
import json
from le.devtools import tools
machines = sorted(tools._GRUB_BY_MACHINE) + ['x86_64', 'riscv64', 'ppc64le']
print(json.dumps({m: tools.grub_apt_package(m) for m in machines}))
`)
	var scripted map[string]string
	if err := json.Unmarshal([]byte(raw), &scripted); err != nil {
		t.Fatalf("decode the script's grub table: %v", err)
	}
	if len(scripted) == 0 {
		t.Fatal("the script named no architecture: the comparison is vacuous")
	}

	for machine, want := range scripted {
		if got := devsetup.GrubAptPackage(machine); got != want {
			t.Errorf("%s: the script installs %s and the command installs %s", machine, want, got)
		}
	}
}

// TestTheApplianceRowsCarryTheDoctorsDependencies preserves the useful part of
// the script APPLIANCE_CHECKS. That list duplicates three tool rows so a Go test
// can read the appliance-doctor dependency names from Python. The port keeps
// this data only in the tool rows. This test keeps both forms equal while they
// coexist.
func TestTheApplianceRowsCarryTheDoctorsDependencies(t *testing.T) {
	raw := python(t, repoRoot(t), `
import json
from le.devtools.tools import APPLIANCE_CHECKS
print(json.dumps([
    {'name': c.name, 'probe': list(c.probe), 'brew': c.brew or '', 'apt': c.apt or ''}
    for c in APPLIANCE_CHECKS
]))
`)
	var checks []struct {
		Name  string   `json:"name"`
		Probe []string `json:"probe"`
		Brew  string   `json:"brew"`
		Apt   string   `json:"apt"`
	}
	if err := json.Unmarshal([]byte(raw), &checks); err != nil {
		t.Fatalf("decode the script's appliance checks: %v", err)
	}
	if len(checks) == 0 {
		t.Fatal("the script declares no appliance check: the comparison is vacuous")
	}

	byName := map[string]devsetup.Tool{}
	for _, tool := range devsetup.AllTools() {
		byName[tool.Name] = tool
	}

	for _, check := range checks {
		name, _ := strings.CutPrefix(check.Name, "appliance-")
		tool, ok := byName[name]
		if !ok {
			t.Errorf("%s has no tool row called %s in the port", check.Name, name)
			continue
		}
		if !slices.Equal(check.Probe, tool.Probe) {
			t.Errorf("%s probes %v and the %s row probes %v", check.Name, check.Probe, name, tool.Probe)
		}
		if check.Brew != tool.Brew {
			t.Errorf("%s installs %q from brew and the %s row installs %q", check.Name, check.Brew, name, tool.Brew)
		}
		if check.Apt != tool.Apt {
			t.Errorf("%s installs %q from apt and the %s row installs %q", check.Name, check.Apt, name, tool.Apt)
		}
	}
}
