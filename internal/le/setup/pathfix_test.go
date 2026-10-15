// Tests for the four defects a fresh Debian and Ubuntu host exposed on
// 2026-10-15: Go tools built off PATH, pppd refused to a non-root user, the
// verdict naming a command that installs nothing, and `ip` never provisioned.
//
// Each case drives the same code the command runs through the Setup seams, and
// starts no process except where the real PATH lookup is the thing under test.

package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// goEnvKey is the argv the recorder answers for the Go install directory.
const goEnvKey = "go env GOBIN GOPATH"

// offPathSetup builds a host whose GOPATH bin directory holds the named tools
// and is not on PATH. It answers the setup and that directory.
func offPathSetup(t *testing.T, gobin string, tools ...string) (*Setup, string) {
	t.Helper()
	gopath := t.TempDir()
	dir := filepath.Join(gopath, "bin")
	if gobin != "" {
		dir = gobin
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range tools {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o700); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	rec := rootShell("go")
	rec.answers[goEnvKey] = Result{Out: gobin + "\n" + gopath + "\n"}
	env := map[string]string{"PATH": "/usr/local/go/bin:/usr/bin:/bin"}
	setup := &Setup{Shell: rec.shell(), Env: func(key string) string { return env[key] }}
	return setup, dir
}

// goplsRow is the table's gopls row, so the case follows the table.
func goplsRow(t *testing.T) Tool {
	t.Helper()
	for _, tool := range requiredTools() {
		if tool.Name == toolGopls {
			return tool
		}
	}
	t.Fatal("the table has no gopls row")
	return Tool{}
}

// TestAGoInstallThatLandsOffPathNamesTheRealDirectory: an install run whose
// `go install` wrote into GOPATH/bin, which is not on PATH, ends pending with
// the exact export line for that directory and the profile to keep it in.
// PREVENTS: the hardcoded "add ~/go/bin" that was wrong for root (/root/go/bin)
// and for any GOBIN, and named no command.
func TestAGoInstallThatLandsOffPathNamesTheRealDirectory(t *testing.T) {
	setup, dir := offPathSetup(t, "", toolGopls)
	report := &Report{}
	installer := setup.NewInstaller(ManagerApt, report)

	outcome := setup.visitTool(report, goplsRow(t), ManagerApt, installer)
	if outcome.State != StatePending {
		t.Errorf("a tool installed off PATH is %s, want %s", outcome.State, StatePending)
	}
	if !strings.Contains(outcome.Detail, dir) {
		t.Errorf("the row does not say where the tool is: %q", outcome.Detail)
	}
	want := "Run: export PATH=\"$PATH\":'" + dir + "'"
	if !strings.Contains(report.Text(), want) {
		t.Errorf("the PATH fix %q is not named:\n%s", want, report.Text())
	}
	if !strings.Contains(report.Text(), "~/.profile") {
		t.Errorf("the profile to keep the line in is not named:\n%s", report.Text())
	}
}

// TestGOBINWinsOverGOPATH: `go install` writes to GOBIN when it is set, so that
// is the directory the fix names.
func TestGOBINWinsOverGOPATH(t *testing.T) {
	gobin := filepath.Join(t.TempDir(), "gobin")
	setup, _ := offPathSetup(t, gobin, toolGopls)
	report := &Report{}

	outcome := setup.visitTool(report, goplsRow(t), ManagerApt, nil)
	if !strings.Contains(outcome.Detail, gobin) {
		t.Errorf("the row names %q, want GOBIN %q", outcome.Detail, gobin)
	}
	if !strings.Contains(report.Text(), "\"$PATH\":'"+gobin+"'") {
		t.Errorf("the PATH fix does not name GOBIN:\n%s", report.Text())
	}
}

// TestThePathLineSurvivesAHostileDirectoryName: a GOBIN holding $, a
// backtick, a double quote, a single quote and a space must reach PATH
// verbatim when the printed line runs in POSIX sh. The line is executed, not
// pattern-matched, because only the shell decides what expands.
// PREVENTS: `export PATH="$PATH:<dir>"`, which expanded $ and ran backticks.
func TestThePathLineSurvivesAHostileDirectoryName(t *testing.T) {
	hostile := "a$HOME`id`" + `"b' c`
	gobin := filepath.Join(t.TempDir(), hostile)
	setup, _ := offPathSetup(t, gobin, toolGopls)
	report := &Report{}
	setup.visitTool(report, goplsRow(t), ManagerApt, nil)

	var line string
	for row := range strings.SplitSeq(report.Text(), "\n") {
		if _, after, ok := strings.Cut(row, "Run: "); ok {
			line = after
		}
	}
	if line == "" {
		t.Fatalf("no Run: line is printed:\n%s", report.Text())
	}

	shell := exec.CommandContext(t.Context(), "/bin/sh", "-c", line+`; printf %s "$PATH"`)
	shell.Env = []string{"PATH=/start"}
	out, err := shell.Output()
	if err != nil {
		t.Fatalf("sh refused %q: %v", line, err)
	}
	if got, want := string(out), "/start:"+gobin; got != want {
		t.Errorf("PATH is %q, want %q", got, want)
	}
}

// TestACheckRunNamesAToolBuiltOffPathAsAStep: check mode on the same host
// reports the tool as one step only the developer can take, with the same
// fix, rather than MISSING, which pointed at an install that already ran.
// It is never PRESENT: no shell can run the tool yet.
func TestACheckRunNamesAToolBuiltOffPathAsAStep(t *testing.T) {
	setup, dir := offPathSetup(t, "", toolGopls)
	report := &Report{}

	outcome := setup.visitTool(report, goplsRow(t), ManagerApt, nil)
	if outcome.State != StatePending {
		t.Errorf("check calls a tool built off PATH %s, want %s", outcome.State, StatePending)
	}
	if !strings.Contains(report.Text(), "\"$PATH\":'"+dir+"'") {
		t.Errorf("check does not name the PATH fix:\n%s", report.Text())
	}
}

// TestThePathLineIsPrintedOnce: five Go tools in one directory are one fix.
func TestThePathLineIsPrintedOnce(t *testing.T) {
	setup, _ := offPathSetup(t, "", toolGopls, toolGoimports)
	report := &Report{}
	for _, tool := range requiredTools() {
		if tool.Name == toolGopls || tool.Name == toolGoimports {
			setup.visitTool(report, tool, ManagerApt, nil)
		}
	}
	if got := strings.Count(report.Text(), "export PATH="); got != 1 {
		t.Errorf("the PATH line is printed %d times, want once:\n%s", got, report.Text())
	}
}

// TestAZshProfileIsZshenv: zsh reads ~/.zshenv in every shell.
func TestAZshProfileIsZshenv(t *testing.T) {
	setup := &Setup{Env: func(key string) string { return map[string]string{"SHELL": "/bin/zsh"}[key] }}
	if got := setup.shellProfile(); got != "~/.zshenv" {
		t.Errorf("zsh profile is %q, want ~/.zshenv", got)
	}
}

// TestAGoToolMissingFromAnOnPathDirectoryIsNotCalledOffPath: when the install
// directory is already on PATH, a failed probe has another cause, and the
// report must not tell the developer to add a directory PATH already holds.
func TestAGoToolMissingFromAnOnPathDirectoryIsNotCalledOffPath(t *testing.T) {
	setup, dir := offPathSetup(t, "", toolGopls)
	setup.Env = func(key string) string { return map[string]string{"PATH": dir}[key] }
	report := &Report{}

	outcome := setup.visitTool(report, goplsRow(t), ManagerApt, setup.NewInstaller(ManagerApt, report))
	if strings.Contains(report.Text(), "export PATH=") {
		t.Errorf("it asks to add a directory PATH already holds:\n%s", report.Text())
	}
	if outcome.State != StatePending {
		t.Errorf("the state is %s, want %s", outcome.State, StatePending)
	}
}

// TestPppdIsFoundWhateverItsMode: Debian ships /usr/sbin/pppd as root:dip 4754,
// so exec.LookPath refuses it to a user outside dip although it is installed.
// The probe accepts the file. 0644 is the discriminating mode: a 0754 file is
// executable by its owner, which this test is, so only a mode refusing this
// user reproduces what a non-dip user sees. PATH holds only the temp directory
// and the system directories are empty, so a host /usr/sbin/pppd (macOS ships
// one) cannot answer for the file under test.
func TestPppdIsFoundWhateverItsMode(t *testing.T) {
	for _, mode := range []os.FileMode{0o754, 0o644} {
		dir := t.TempDir()
		path := filepath.Join(dir, "pppd")
		if err := os.WriteFile(path, nil, mode); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatalf("chmod: %v", err)
		}

		setup := pppdSetup(dir)
		if !setup.Probe(optionalToolNamed(t, toolPpp)) {
			t.Errorf("pppd with mode %o is reported absent", mode)
		}
	}
}

// TestPppdIsAbsentWhenNoDirectoryHoldsIt: the negative space of the probe. A
// PATH directory with no pppd and no system directories must answer absent, so
// a probe that always answers present fails here.
func TestPppdIsAbsentWhenNoDirectoryHoldsIt(t *testing.T) {
	setup := pppdSetup(t.TempDir())
	if setup.Probe(optionalToolNamed(t, toolPpp)) {
		t.Error("pppd is reported present, but no searched directory holds it")
	}
}

// pppdSetup builds a host whose PATH is dir alone and whose system directories
// are empty, so the probe sees only what the test put in dir.
func pppdSetup(dir string) *Setup {
	return &Setup{
		Shell:         &Shell{},
		Env:           func(key string) string { return map[string]string{"PATH": dir}[key] },
		SystemBinDirs: func() []string { return nil },
	}
}

// TestPppdIsSearchedOnPathThenInSbin: the PATH entries come first, and the
// system directories a non-root PATH omits are still searched.
func TestPppdIsSearchedOnPathThenInSbin(t *testing.T) {
	setup := &Setup{Env: func(key string) string { return map[string]string{"PATH": "/a::/b"}[key] }}
	want := []string{"/a", "/b", "/usr/sbin", "/sbin"}
	if got := setup.rootOnlyFileDirs(); !slices.Equal(got, want) {
		t.Errorf("searched %v, want %v", got, want)
	}
}

// optionalToolNamed answers the table row with this name.
func optionalToolNamed(t *testing.T, name string) Tool {
	t.Helper()
	for _, tool := range allTools() {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("the table has no %s row", name)
	return Tool{}
}

// TestTheInstallVerdictNamesTheInstallAction: the bare `./le setup` prints the
// action listing and installs nothing, so the re-run line names `install`.
func TestTheInstallVerdictNamesTheInstallAction(t *testing.T) {
	report := &Report{}
	report.Add(Outcome{Name: "gopls", State: StatePending})
	if got := report.Summarize(); got != 1 {
		t.Fatalf("the verdict is %d", got)
	}
	if !strings.Contains(report.Text(), "Then re-run: ./le setup install\n") {
		t.Errorf("the re-run line does not name the install action:\n%s", report.Text())
	}
}

// TestACheckVerdictNamesTheCommandForEachState: a missing tool points at the
// install action, and a pending step points back at the check.
func TestACheckVerdictNamesTheCommandForEachState(t *testing.T) {
	report := &Report{}
	report.Add(Outcome{Name: "jq", State: StateMissing})
	report.Add(Outcome{Name: "gopls", State: StatePending})
	report.checkVerdict()
	page := report.Text()
	if !strings.Contains(page, "Install them with: ./le setup install") {
		t.Errorf("the missing verdict names no command:\n%s", page)
	}
	if !strings.Contains(page, "then re-run: ./le setup check") {
		t.Errorf("the pending verdict names no command:\n%s", page)
	}
}

// TestTheLoopbackProgramHasARequiredInstallRow: the Linux loopback step runs
// a program that slim Debian and Ubuntu images lack, so a required row
// installs it with apt, and macOS, whose step runs ifconfig, skips it.
func TestTheLoopbackProgramHasARequiredInstallRow(t *testing.T) {
	linux := &Setup{GOOS: osLinux}
	program := linux.loopbackAddArgv(loopbackPeerIPv6)[0]

	var row Tool
	for _, tool := range allTools() {
		if slices.Contains(tool.Probe, program) {
			row = tool
		}
	}
	if row.Name == "" {
		t.Fatalf("no row installs %q, which the Linux loopback step runs", program)
	}
	if row.Apt != "iproute2" {
		t.Errorf("the %s row installs apt package %q, want iproute2", row.Name, row.Apt)
	}
	if !row.Required {
		t.Errorf("the %s row is optional, but the loopback step needs it", row.Name)
	}

	mac := &Setup{GOOS: osDarwin, Shell: rootShell().shell()}
	if outcome := mac.visitTool(&Report{}, row, ManagerBrew, nil); outcome.State != StateSkipped {
		t.Errorf("macOS reports the %s row %s, want %s", row.Name, outcome.State, StateSkipped)
	}
}

// TestThePluginRowIsSkippedWithoutClaudeCode: with no `claude` on PATH and no
// ~/.claude, there is no LSP tool and no session for the slash command, so the
// row is skipped and says why.
func TestThePluginRowIsSkippedWithoutClaudeCode(t *testing.T) {
	setup := &Setup{Home: t.TempDir(), Shell: rootShell().shell()}
	report := &Report{}
	outcome := setup.visitLspPlugin(report, lspPlugins()[0], setup.missingLSPPlugins())
	if outcome.State != StateSkipped {
		t.Errorf("without Claude Code the plugin row is %s, want %s", outcome.State, StateSkipped)
	}
	if !strings.Contains(outcome.Detail, "Claude Code is not installed") {
		t.Errorf("the row does not say why: %q", outcome.Detail)
	}
	if strings.Contains(report.Text(), "/plugin install") {
		t.Errorf("it asks for a slash command with no session to run it:\n%s", report.Text())
	}
}

// TestThePluginRowIsPendingWhenClaudeIsOnPath: `claude` on PATH alone is
// Claude Code present, so a missing plugin stays a step with its command.
func TestThePluginRowIsPendingWhenClaudeIsOnPath(t *testing.T) {
	setup := &Setup{Home: t.TempDir(), Shell: rootShell(toolClaude).shell()}
	report := &Report{}
	outcome := setup.visitLspPlugin(report, lspPlugins()[0], setup.missingLSPPlugins())
	if outcome.State != StatePending {
		t.Errorf("with claude on PATH the plugin row is %s, want %s", outcome.State, StatePending)
	}
	if !strings.Contains(report.Text(), "/plugin install gopls-lsp@claude-plugins-official") {
		t.Errorf("the install command is not named:\n%s", report.Text())
	}
}
