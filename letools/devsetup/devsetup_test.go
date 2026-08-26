// Tests for the setup tool cover the report vocabulary, probes, installation
// routes, and machine state that is not a binary.
//
// Each case that needs an external command sets the Shell seam and does not
// start a process. This is the Go equivalent of the mock.patch.object that the
// Python tests used. The three system-state steps use their apply branches
// only HERE. scripts/le/setup_parity_test.go compares the two halves on a real
// machine, so it must not change either half.

package devsetup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// recorder is a Shell that answers from a table and records every argv.
type recorder struct {
	present map[string]bool
	answers map[string]Result
	calls   [][]string
	euid    int
}

// shell builds the seam a case drives the tool through.
func (r *recorder) shell() *Shell {
	return &Shell{
		Look: func(name string) (string, bool) {
			if !r.present[name] {
				return "", false
			}
			return "/test/bin/" + name, true
		},
		Exec: func(_ context.Context, cmd Cmd) Result {
			r.calls = append(r.calls, cmd.Argv)
			if answer, ok := r.answers[strings.Join(cmd.Argv, " ")]; ok {
				return answer
			}
			return Result{Argv: cmd.Argv}
		},
		Euid: func() int { return r.euid },
		Tty:  func() bool { return false },
	}
}

// ran answers the recorded argv lines, joined for comparison.
func (r *recorder) ran() []string {
	lines := make([]string, 0, len(r.calls))
	for _, argv := range r.calls {
		lines = append(lines, strings.Join(argv, " "))
	}
	return lines
}

// rootShell is a recorder carrying exactly these binaries.
func rootShell(present ...string) *recorder {
	rec := &recorder{present: map[string]bool{}, answers: map[string]Result{}}
	for _, name := range present {
		rec.present[name] = true
	}
	return rec
}

// --- The report vocabulary --------------------------------------------------

func TestOnlyPendingAndMissingBlock(t *testing.T) {
	for state, want := range map[State]bool{
		StatePresent:   false,
		StateInstalled: false,
		StateSkipped:   false,
		StatePending:   true,
		StateMissing:   true,
	} {
		if got := state.Blocking(); got != want {
			t.Errorf("%s.Blocking() = %v, want %v", state, got, want)
		}
	}
}

func TestAnOutcomeLineNamesTheStateAndTheDetail(t *testing.T) {
	line := Outcome{Name: "gopls", State: StateMissing, Detail: "REQUIRED"}.Line()
	if line != "  [MISSING  ] gopls (REQUIRED)" {
		t.Errorf("the line is %q", line)
	}
	bare := Outcome{Name: "jq", State: StatePresent}.Line()
	if bare != "  [present  ] jq" {
		t.Errorf("the line is %q", bare)
	}
}

func TestTheVerdictIsDerivedFromTheOutcomes(t *testing.T) {
	cases := map[string]struct {
		outcomes []Outcome
		install  int
		check    int
		says     string
	}{
		"clean": {
			[]Outcome{{Name: "go", State: StatePresent}}, 0, 0, "Setup complete. All tools already present.",
		},
		"missing wins over pending": {
			[]Outcome{{Name: "go", State: StateMissing}, {Name: "jq", State: StatePending}},
			1, 1, "Missing required tools: go",
		},
		"pending alone still fails": {
			[]Outcome{{Name: "jq", State: StatePending}}, 1, 1, "Finish the steps above for: jq",
		},
		"installed and skipped are reported": {
			[]Outcome{{Name: "go", State: StateInstalled}, {Name: "colima", State: StateSkipped}},
			0, 0, "Setup complete. installed: go; skipped (optional): colima",
		},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			install := &Report{}
			for _, outcome := range one.outcomes {
				install.Add(outcome)
			}
			if got := install.Summarize(); got != one.install {
				t.Errorf("an install run answered %d, want %d", got, one.install)
			}
			if !strings.Contains(install.Text(), one.says) {
				t.Errorf("no %q in:\n%s", one.says, install.Text())
			}

			check := &Report{}
			for _, outcome := range one.outcomes {
				check.Add(outcome)
			}
			if got := check.CheckVerdict(); got != one.check {
				t.Errorf("a check run answered %d, want %d", got, one.check)
			}
		})
	}
}

// TestACheckRunNamesAPendingStepApartFromAMissingTool verifies the only
// difference between the two verdicts. A plugin that this tool must not install
// is not a missing tool. Directing its reader to the tool table does not help.
func TestACheckRunNamesAPendingStepApartFromAMissingTool(t *testing.T) {
	report := &Report{}
	report.Add(Outcome{Name: "go", State: StateMissing})
	report.Add(Outcome{Name: "pyright-lsp-installed", State: StatePending})

	if got := report.CheckVerdict(); got != 1 {
		t.Fatalf("the verdict is %d", got)
	}
	page := report.Text()
	if !strings.Contains(page, "Missing required tools: go") {
		t.Errorf("the missing tool is not named:\n%s", page)
	}
	if !strings.Contains(page, "Needs a step only you can take: pyright-lsp-installed") {
		t.Errorf("the pending step is not named apart:\n%s", page)
	}
}

func TestTheReportEncodesItsOutcomes(t *testing.T) {
	report := &Report{}
	report.Note("noise a reader sees and a machine does not")
	report.Add(Outcome{Name: "go", State: StatePresent})

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var rows []Outcome
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "go" {
		t.Errorf("the payload is %s", raw)
	}
}

// --- The probes -------------------------------------------------------------

func TestTheStaticcheckPinIsCarriedIntoTheInstall(t *testing.T) {
	var found []Tool
	for _, tool := range RequiredTools() {
		if tool.Name == "staticcheck" {
			found = append(found, tool)
		}
	}
	if len(found) != 1 {
		t.Fatalf("staticcheck occurs %d times in the required table", len(found))
	}
	if !strings.HasSuffix(found[0].GoInstall, StaticcheckVersion) {
		t.Errorf("the install target %q does not carry the pin %q", found[0].GoInstall, StaticcheckVersion)
	}
}

func TestOnlyThePinnedStaticcheckCounts(t *testing.T) {
	pinned := "staticcheck " + StaticcheckVersion
	cases := map[string]struct {
		out  string
		code int
		want bool
	}{
		"the pin, bare":       {pinned + "\n", 0, true},
		"the pin, with build": {pinned + " (v0.7.0)\n", 0, true},
		"a stale version":     {"staticcheck 2025.1.1 (v0.6.1)\n", 0, false},
		"a failing tool":      {pinned + "\n", 1, false},
		"a timed-out probe":   {"", exitTimedOut, false},
		"an empty build id":   {pinned + " ()\n", 0, false},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			rec := rootShell("staticcheck")
			rec.answers["/test/bin/staticcheck -version"] = Result{Code: one.code, Out: one.out}
			setup := &Setup{Shell: rec.shell()}

			tool := Tool{Name: "staticcheck", Probe: []string{"staticcheck"}}
			if got := setup.Probe(tool); got != one.want {
				t.Errorf("the probe answered %v, want %v", got, one.want)
			}
		})
	}
}

func TestAnAbsentStaticcheckIsNeverRun(t *testing.T) {
	rec := rootShell()
	setup := &Setup{Shell: rec.shell()}
	if setup.Probe(Tool{Name: "staticcheck", Probe: []string{"staticcheck"}}) {
		t.Error("an absent staticcheck reported present")
	}
	if len(rec.calls) != 0 {
		t.Errorf("it was run anyway: %v", rec.ran())
	}
}

func TestProbeAnyNeedsOneNameAndTheDefaultNeedsEvery(t *testing.T) {
	rec := rootShell("pyright")
	setup := &Setup{Shell: rec.shell()}

	both := Tool{Name: "pyright", Probe: []string{"pyright", "pyright-langserver"}}
	if setup.Probe(both) {
		t.Error("a tool needing two binaries reported present with one")
	}
	either := Tool{Name: "pyright", Probe: []string{"pyright", "pyright-langserver"}, ProbeAny: true}
	if !setup.Probe(either) {
		t.Error("a tool needing either binary reported absent with one")
	}
}

// TestARowThatNamesNoExecutableIsNotPresent closes the vacuous branch: an empty
// probe list makes an "every name found" test true of nothing.
func TestARowThatNamesNoExecutableIsNotPresent(t *testing.T) {
	setup := &Setup{Shell: rootShell().shell()}
	if setup.Probe(Tool{Name: "nothing"}) {
		t.Error("a row naming no executable reported present")
	}
}

func TestE2fsprogsIsSearchedByDirectory(t *testing.T) {
	prefix := t.TempDir()
	half := filepath.Join(prefix, "opt", "e2fsprogs", "sbin")
	if err := os.MkdirAll(half, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(half, "mkfs.ext4"), nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	setup := &Setup{
		Shell: rootShell().shell(),
		Env:   func(key string) string { return map[string]string{homebrewPrefixKey: prefix}[key] },
	}
	dirs := setup.E2fsprogsDirs()
	if !slices.Contains(dirs, half) {
		t.Errorf("the keg-only directory is not searched: %v", dirs)
	}
	if !slices.Contains(dirs, "/usr/sbin") {
		t.Errorf("the system directory is not searched: %v", dirs)
	}
}

func TestCellarVersionsOrderByNumber(t *testing.T) {
	// Plain string order places 1.47.10 lower than 1.47.4. Thus, the first
	// formula with a two-digit patch returns last month's e2fsprogs.
	newer := "/p/Cellar/e2fsprogs/1.47.10/sbin"
	older := "/p/Cellar/e2fsprogs/1.47.4/sbin"
	if !cellarNewerFirst(newer, older) {
		t.Error("1.47.10 did not outrank 1.47.4")
	}
	if !cellarNewerFirst("/p/Cellar/e2fsprogs/1.47.4_1/sbin", older) {
		t.Error("a Homebrew revision did not outrank the release it revises")
	}
	if !cellarNewerFirst(older, "/p/Cellar/e2fsprogs/1.47.rc1/sbin") {
		t.Error("a release candidate outranked the release")
	}
}

// --- The language servers ---------------------------------------------------

func TestGoplsMustAnswerWithSymbols(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "core", "clock"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, goplsProbeFile), []byte("package clock\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cases := map[string]struct {
		present bool
		answer  Result
		want    Health
	}{
		"absent":            {false, Result{}, HealthAbsent},
		"answers":           {true, Result{Out: "Now Function 20:6-20:9\n"}, HealthOK},
		"runs and says not": {true, Result{Out: "nothing useful\n"}, HealthBroken},
		"fails":             {true, Result{Code: 1, Err: "no module cache\n"}, HealthBroken},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			rec := rootShell()
			rec.present["gopls"] = one.present
			setup := &Setup{Root: root, Shell: rec.shell(), Gopls: func() Result { return one.answer }}
			if got := setup.GoplsHealth().Health; got != one.want {
				t.Errorf("the health is %s, want %s", got, one.want)
			}
		})
	}
}

// TestGoplsHasNothingToProbeWithoutItsFile pins the NA branch, which exists for
// the day the probe file is renamed out from under it.
func TestGoplsHasNothingToProbeWithoutItsFile(t *testing.T) {
	rec := rootShell("gopls")
	setup := &Setup{Root: t.TempDir(), Shell: rec.shell()}
	if got := setup.GoplsHealth().Health; got != HealthNA {
		t.Errorf("the health is %s, want %s", got, HealthNA)
	}
}

// TestPyrightIsJudgedOnItsSummaryNotItsExitCode verifies the difference from
// gopls. pyright exits 1 when it finds a type error. Thus, the exit code shows
// whether the CODE is clean, not whether the SERVER worked.
func TestPyrightIsJudgedOnItsSummaryNotItsExitCode(t *testing.T) {
	rec := rootShell("pyright")
	setup := &Setup{
		Root:  t.TempDir(),
		Shell: rec.shell(),
		Pyright: func() Result {
			return Result{Code: 1, Out: `{"summary": {"filesAnalyzed": 1}}`}
		},
	}
	if got := setup.PyrightHealth().Health; got != HealthOK {
		t.Errorf("a server that found a type error is %s, want %s", got, HealthOK)
	}
}

func TestAPyrightReplyBehindABootstrapPreambleIsStillARepl(t *testing.T) {
	// nodeenv prints a Python dict repr, single quotes, in front of a reply
	// that is otherwise valid JSON. Decoding the whole stream fails, and setup
	// then reds on exactly the fresh box it exists to prepare.
	out := "{'python': '/x/bin/python'}\ninstalling node\n{\"summary\": {\"filesAnalyzed\": 3}}\n"
	summary, ok := PyrightSummary(out)
	if !ok {
		t.Fatalf("no summary found in %q", out)
	}
	if summary["filesAnalyzed"] != float64(3) {
		t.Errorf("the summary is %v", summary)
	}
}

func TestPyrightThatAnalysedNothingIsBroken(t *testing.T) {
	rec := rootShell("pyright")
	setup := &Setup{
		Root:    t.TempDir(),
		Shell:   rec.shell(),
		Pyright: func() Result { return Result{Out: `{"summary": {"filesAnalyzed": 0}}`} },
	}
	if got := setup.PyrightHealth().Health; got != HealthBroken {
		t.Errorf("a server that analyzed nothing is %s, want %s", got, HealthBroken)
	}
}

func TestAnAbsentServerIsSkippedAndABrokenOneIsMissing(t *testing.T) {
	absent := visitServer("gopls-answers", ServerAnswer{Health: HealthAbsent, Detail: "x"})
	if absent.State != StateSkipped {
		t.Errorf("an absent server is %s: one missing server would be two failures", absent.State)
	}
	broken := visitServer("gopls-answers", ServerAnswer{Health: HealthBroken, Detail: "x"})
	if broken.State != StateMissing {
		t.Errorf("a broken server is %s: installing it again does not repair it", broken.State)
	}
	na := visitServer("gopls-answers", ServerAnswer{Health: HealthNA, Detail: "no file"})
	if na.State != StatePresent || !strings.HasPrefix(na.Detail, "n/a: ") {
		t.Errorf("a question that does not apply is %s (%s)", na.State, na.Detail)
	}
}

// --- The harness plugins ----------------------------------------------------

func TestAnAbsentPluginRecordMeansNothingIsInstalled(t *testing.T) {
	setup := &Setup{Home: t.TempDir()}
	missing := setup.MissingLSPPlugins()
	if len(missing) != len(LSPPlugins()) {
		t.Errorf("%d of %d plugins are missing", len(missing), len(LSPPlugins()))
	}
}

func TestAnInstalledPluginIsNotReported(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "plugins")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `{"plugins": {"gopls-lsp@claude-plugins-official": {}}}`
	if err := os.WriteFile(filepath.Join(dir, "installed_plugins.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	setup := &Setup{Home: home}
	missing := setup.MissingLSPPlugins()
	if len(missing) != 1 || missing[0].Plugin != "pyright-lsp" {
		t.Errorf("the missing set is %v", missing)
	}
}

// TestAMissingPluginIsPendingRatherThanMissing pins the state that says a human
// must run one command: nothing this tool runs can install a plugin.
func TestAMissingPluginIsPendingRatherThanMissing(t *testing.T) {
	setup := &Setup{Home: t.TempDir()}
	report := &Report{}
	outcome := setup.visitLspPlugin(report, LSPPlugins()[1], setup.MissingLSPPlugins())

	if outcome.State != StatePending {
		t.Errorf("a missing plugin is %s, want %s", outcome.State, StatePending)
	}
	if !strings.Contains(report.Text(), "/plugin install pyright-lsp@claude-plugins-official") {
		t.Errorf("the install command is not named:\n%s", report.Text())
	}
}

// --- The install routes -----------------------------------------------------

// TestThePackageManagerIsGatedOnThePlatform prevents Linuxbrew from managing a
// Linux box. A check of PATH alone selects brew first on any platform. This
// silently changes which package manager owns the machine.
func TestThePackageManagerIsGatedOnThePlatform(t *testing.T) {
	cases := map[string]struct {
		goos    string
		present []string
		want    PackageManager
	}{
		"linux with apt":            {"linux", []string{"apt-get"}, ManagerApt},
		"linux carrying linuxbrew":  {"linux", []string{"brew"}, ManagerNone},
		"linux with both":           {"linux", []string{"brew", "apt-get"}, ManagerApt},
		"macos with brew":           {"darwin", []string{"brew"}, ManagerBrew},
		"macos carrying an apt-get": {"darwin", []string{"apt-get"}, ManagerNone},
		"neither":                   {"linux", nil, ManagerNone},
		"another platform":          {"freebsd", []string{"brew", "apt-get"}, ManagerNone},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			setup := &Setup{GOOS: one.goos, Shell: rootShell(one.present...).shell()}
			if got := setup.DetectPackageManager(); got != one.want {
				t.Errorf("the manager is %q, want %q", got, one.want)
			}
		})
	}
}

func TestTheCrossPlatformRoutesWinOverThePackageManager(t *testing.T) {
	rec := rootShell("go", "pipx", "apt-get")
	setup := &Setup{Shell: rec.shell()}
	installer := setup.NewInstaller(ManagerApt, &Report{})

	installer.Install(Tool{Name: "gopls", GoInstall: "x@latest", Apt: "golang-gopls"})
	installer.Install(Tool{Name: "ruff", PipxInstall: "ruff", Apt: "python3-ruff"})

	want := []string{"go install x@latest", "pipx install --force ruff"}
	if !slices.Equal(rec.ran(), want) {
		t.Errorf("it ran %v, want %v", rec.ran(), want)
	}
}

func TestAptUpdatesOncePerRun(t *testing.T) {
	rec := rootShell("apt-get")
	rec.euid = 0
	setup := &Setup{Shell: rec.shell()}
	installer := setup.NewInstaller(ManagerApt, &Report{})

	installer.Install(Tool{Name: "jq", Apt: "jq"})
	installer.Install(Tool{Name: "git", Apt: "git"})

	updates := 0
	for _, line := range rec.ran() {
		if line == "apt-get update" {
			updates++
		}
	}
	if updates != 1 {
		t.Errorf("apt-get update ran %d times: it is one index per run", updates)
	}
}

// TestARootRunNamesNoSudo verifies the line that a reader copies. Some root
// containers do not contain sudo. If the line names sudo, the reader receives
// a command that cannot run in those containers.
func TestARootRunNamesNoSudo(t *testing.T) {
	rec := rootShell("apt-get")
	rec.euid = 0
	report := &Report{}
	setup := &Setup{Shell: rec.shell()}
	setup.NewInstaller(ManagerApt, report).Install(Tool{Name: "jq", Apt: "jq"})

	if strings.Contains(report.Text(), "sudo") {
		t.Errorf("a root run named sudo:\n%s", report.Text())
	}
	want := "env DEBIAN_FRONTEND=noninteractive apt-get install -y jq"
	if !slices.Contains(rec.ran(), want) {
		t.Errorf("it ran %v, want %q among them", rec.ran(), want)
	}
}

// TestNoRouteToRootNamesTheCommandAndRunsNothing pins the recoverable failure: a
// setup program that says what to run can be finished by hand.
func TestNoRouteToRootNamesTheCommandAndRunsNothing(t *testing.T) {
	rec := rootShell("apt-get")
	rec.euid = 1000
	report := &Report{}
	setup := &Setup{Shell: rec.shell()}

	if setup.NewInstaller(ManagerApt, report).Install(Tool{Name: "jq", Apt: "jq"}) {
		t.Error("it claimed to install without a route to root")
	}
	if !strings.Contains(report.Text(), "Run: sudo apt-get install -y jq") {
		t.Errorf("the manual command is not named:\n%s", report.Text())
	}
	if len(rec.calls) != 0 {
		t.Errorf("it ran %v with no route to root", rec.ran())
	}
}

func TestAToolWithNoRouteIsSkippedRatherThanMissing(t *testing.T) {
	colima := Tool{Name: "colima", Probe: []string{"colima"}, Brew: "colima"}
	if colima.InstallableBy(ManagerApt) {
		t.Error("a brew-only tool claims an apt route")
	}
	setup := &Setup{Shell: rootShell().shell()}
	outcome := setup.visitTool(&Report{}, colima, ManagerApt, nil)
	if outcome.State != StateSkipped {
		t.Errorf("a tool with no route on this platform is %s, want %s", outcome.State, StateSkipped)
	}
}

// TestAnInstallThatLandsOffPathIsPending verifies the discrepancy between the
// two modes that this module removes. The install run previously reported
// [installed] and exited 0, while check mode on the same box exited 1.
func TestAnInstallThatLandsOffPathIsPending(t *testing.T) {
	rec := rootShell("pipx")
	report := &Report{}
	setup := &Setup{Shell: rec.shell()}
	installer := setup.NewInstaller(ManagerApt, report)

	tool := Tool{Name: "uv", Probe: []string{"uv"}, PipxInstall: "uv", Required: true}
	outcome := setup.visitTool(report, tool, ManagerApt, installer)

	if outcome.State != StatePending {
		t.Errorf("a tool installed off PATH is %s, want %s", outcome.State, StatePending)
	}
	if !strings.Contains(report.Text(), "~/.local/bin, which pipx uses") {
		t.Errorf("the PATH fix is not named:\n%s", report.Text())
	}
}

// --- Vendoring --------------------------------------------------------------

// TestAFailedVendorFailsTheRun is the fail-open the port closes. The script
// discards vendor_go_deps()'s answer, so a failed `go mod vendor` still ends
// "Setup complete" with exit 0 while the tree will not build.
func TestAFailedVendorFailsTheRun(t *testing.T) {
	rec := rootShell("go")
	rec.answers["go mod vendor"] = Result{Code: 1, Err: "inconsistent vendoring\n"}
	report := &Report{}
	setup := &Setup{Root: t.TempDir(), Shell: rec.shell()}

	if setup.VendorGoDeps(report) {
		t.Fatal("a failed `go mod vendor` reported success")
	}
	if !strings.Contains(report.Text(), "inconsistent vendoring") {
		t.Errorf("the complaint is not named:\n%s", report.Text())
	}
}

func TestVendoringWithoutGoIsAFailureRatherThanASkip(t *testing.T) {
	setup := &Setup{Root: t.TempDir(), Shell: rootShell().shell()}
	if setup.VendorGoDeps(&Report{}) {
		t.Error("vendoring with no go reported success")
	}
}

// --- Machine state that is not a binary -------------------------------------

// TestAnUnreadableUsernsKnobIsNotAnAnswer prevents the second fail-open. For a
// read error, the script returns NA, which means "no such knob, nothing to do".
// A read error does not show that the knob is absent. It can be present and set
// to 1.
func TestAnUnreadableUsernsKnobIsNotAnAnswer(t *testing.T) {
	knob := filepath.Join(t.TempDir(), "restrict")
	if err := os.Mkdir(knob, 0o750); err != nil { // a directory reads as EISDIR
		t.Fatalf("mkdir: %v", err)
	}

	setup := &Setup{UsernsProc: knob, Shell: rootShell().shell()}
	state, err := setup.UsernsState()
	if err == nil {
		t.Fatalf("an unreadable knob answered %s with no error", state)
	}

	outcome := setup.visitUserns(&Report{})
	if outcome.State != StateMissing {
		t.Errorf("an unreadable knob is %s, want %s: nothing here knows whether Chrome can start",
			outcome.State, StateMissing)
	}
}

func TestTheUsernsStatesReadTheKnob(t *testing.T) {
	for body, want := range map[string]Userns{"1\n": UsernsRestricted, "0\n": UsernsOK} {
		knob := filepath.Join(t.TempDir(), "restrict")
		if err := os.WriteFile(knob, []byte(body), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		setup := &Setup{UsernsProc: knob}
		got, err := setup.UsernsState()
		if err != nil || got != want {
			t.Errorf("%q answered %s (%v), want %s", body, got, err, want)
		}
	}
}

func TestAKernelWithNoSuchKnobHasNothingToDo(t *testing.T) {
	setup := &Setup{UsernsProc: filepath.Join(t.TempDir(), "absent")}
	state, err := setup.UsernsState()
	if err != nil || state != UsernsNA {
		t.Errorf("an absent knob answered %s (%v), want %s", state, err, UsernsNA)
	}
}

// TestLiftingTheRestrictionWritesTheDropInAndTheRunningValue pins both commands:
// one without the other leaves the machine right now or right after the next
// boot, not both.
func TestLiftingTheRestrictionWritesTheDropInAndTheRunningValue(t *testing.T) {
	knob := filepath.Join(t.TempDir(), "restrict")
	if err := os.WriteFile(knob, []byte("1\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	rec := rootShell()
	rec.euid = 0
	report := &Report{}
	setup := &Setup{UsernsProc: knob, Shell: rec.shell()}

	if setup.ApplyUserns(report) {
		t.Error("it claimed success while the knob still reads 1")
	}
	want := []string{
		"tee /etc/sysctl.d/60-ze-userns.conf",
		"sysctl -w kernel.apparmor_restrict_unprivileged_userns=0",
	}
	if !slices.Equal(rec.ran(), want) {
		t.Errorf("it ran %v, want %v", rec.ran(), want)
	}
	if !strings.Contains(report.Text(), `echo "kernel.apparmor_restrict_unprivileged_userns = 0" | tee`) {
		t.Errorf("the drop-in is recorded as a bare tee:\n%s", report.Text())
	}
}

func TestAProbeOnlyRunNeverAppliesSystemState(t *testing.T) {
	knob := filepath.Join(t.TempDir(), "restrict")
	if err := os.WriteFile(knob, []byte("1\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	rec := rootShell()
	rec.euid = 0
	setup := &Setup{Check: true, UsernsProc: knob, Shell: rec.shell()}

	outcome := setup.visitUserns(&Report{})
	if outcome.State != StateMissing {
		t.Errorf("a restricted knob in check mode is %s, want %s", outcome.State, StateMissing)
	}
	if len(rec.calls) != 0 {
		t.Errorf("check mode ran %v", rec.ran())
	}
}

func TestKvmTellsApartTheThreeWaysItIsNotUsable(t *testing.T) {
	absent := &Setup{KvmDev: filepath.Join(t.TempDir(), "kvm"), Shell: rootShell().shell()}
	if got := absent.KvmState(); got != KvmNA {
		t.Errorf("no device answered %s, want %s", got, KvmNA)
	}

	device := filepath.Join(t.TempDir(), "kvm")
	if err := os.WriteFile(device, nil, 0o000); err != nil {
		t.Fatalf("write: %v", err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root can open a 0000 file, so the two unusable states are unreachable here")
	}

	member := &Setup{KvmDev: device, KvmGroupMember: func() bool { return true }, Shell: rootShell().shell()}
	if got := member.KvmState(); got != KvmPendingLogin {
		t.Errorf("a member with a stale session answered %s, want %s", got, KvmPendingLogin)
	}
	outsider := &Setup{KvmDev: device, KvmGroupMember: func() bool { return false }, Shell: rootShell().shell()}
	if got := outsider.KvmState(); got != KvmNoGroup {
		t.Errorf("a non-member answered %s, want %s", got, KvmNoGroup)
	}
}

func TestJoiningTheKvmGroupNamesTheUser(t *testing.T) {
	rec := rootShell()
	rec.euid = 0
	setup := &Setup{
		Shell:          rec.shell(),
		User:           func() string { return "thomas" },
		KvmGroupMember: func() bool { return true },
	}
	if !setup.ApplyKvm(&Report{}) {
		t.Error("the group database lists the user and it reported failure")
	}
	if !slices.Equal(rec.ran(), []string{"usermod -aG kvm thomas"}) {
		t.Errorf("it ran %v", rec.ran())
	}
}

func TestLoopbackIsIPv6OnlyOnLinuxAndBothOnDarwin(t *testing.T) {
	linux := &Setup{GOOS: "linux"}
	if !slices.Equal(linux.LoopbackAddresses(), []string{LoopbackIPv6}) {
		t.Errorf("linux wants %v", linux.LoopbackAddresses())
	}
	darwin := &Setup{GOOS: "darwin"}
	want := []string{"127.0.0.2", "127.0.0.3", "127.0.0.4", "127.0.0.5", LoopbackIPv6}
	if !slices.Equal(darwin.LoopbackAddresses(), want) {
		t.Errorf("darwin wants %v", darwin.LoopbackAddresses())
	}
}

func TestTheLoopbackCommandIsPerPlatform(t *testing.T) {
	linux := &Setup{GOOS: "linux"}
	wantLinux := []string{"ip", "-6", "addr", "add", "fd00::2/128", "dev", "lo"}
	if got := linux.loopbackAddArgv(LoopbackIPv6); !slices.Equal(got, wantLinux) {
		t.Errorf("linux runs %v", got)
	}
	darwin := &Setup{GOOS: "darwin"}
	wantDarwin := []string{"ifconfig", "lo0", "inet6", "fd00::2/128", "alias"}
	if got := darwin.loopbackAddArgv(LoopbackIPv6); !slices.Equal(got, wantDarwin) {
		t.Errorf("darwin runs %v for IPv6", got)
	}
	if got := darwin.loopbackAddArgv("127.0.0.2"); !slices.Equal(got, []string{"ifconfig", "lo0", "alias", "127.0.0.2"}) {
		t.Errorf("darwin runs %v for IPv4", got)
	}
}

func TestOnlyTheMissingAddressesAreAdded(t *testing.T) {
	rec := rootShell()
	rec.euid = 0
	carried := map[string]bool{"127.0.0.2": true, "127.0.0.3": true, LoopbackIPv6: true}
	setup := &Setup{
		GOOS:     "darwin",
		Shell:    rec.shell(),
		Bindable: func(addr string) bool { return carried[addr] },
	}

	missing := setup.MissingLoopback()
	if !slices.Equal(missing, []string{"127.0.0.4", "127.0.0.5"}) {
		t.Fatalf("the missing set is %v", missing)
	}
	setup.ApplyLoopback(&Report{}, missing)
	want := []string{"ifconfig lo0 alias 127.0.0.4", "ifconfig lo0 alias 127.0.0.5"}
	if !slices.Equal(rec.ran(), want) {
		t.Errorf("it ran %v, want %v", rec.ran(), want)
	}
}

// --- The tool table ---------------------------------------------------------

// TestTheApplianceRowsCarryTheDoctorsDependencies verifies what the script's
// APPLIANCE_CHECKS asserted about the tool table. Its rows contain the three
// appliance dependencies and the probes that the doctor reports.
func TestTheApplianceRowsCarryTheDoctorsDependencies(t *testing.T) {
	byName := map[string]Tool{}
	for _, tool := range AllTools() {
		byName[tool.Name] = tool
	}

	for name, probe := range map[string][]string{
		"grub":      {"grub-mkstandalone", "grub2-mkstandalone"},
		"xorriso":   {"xorriso"},
		"e2fsprogs": {"mkfs.ext4", "debugfs"},
	} {
		tool, ok := byName[name]
		if !ok {
			t.Errorf("the tool table has no %s row, so setup installs nothing the appliance needs", name)
			continue
		}
		if !slices.Equal(tool.Probe, probe) {
			t.Errorf("%s probes %v, want %v", name, tool.Probe, probe)
		}
	}
}

func TestTheGrubPackageFollowsTheHostArchitecture(t *testing.T) {
	for machine, want := range map[string]string{
		"aarch64": "grub-efi-arm64-bin",
		"arm64":   "grub-efi-arm64-bin",
		"i386":    "grub-efi-ia32-bin",
		"i686":    "grub-efi-ia32-bin",
		"x86_64":  "grub-efi-amd64-bin",
		"riscv64": "grub-efi-amd64-bin",
	} {
		if got := GrubAptPackage(machine); got != want {
			t.Errorf("%s installs %s, want %s", machine, got, want)
		}
	}
}

// TestPipxComesBeforeEveryToolThatInstallsThroughIt pins an ordering the table
// cannot state: a pipx install is skipped while pipx is not there yet.
func TestPipxComesBeforeEveryToolThatInstallsThroughIt(t *testing.T) {
	pipx := -1
	for i, tool := range RequiredTools() {
		if tool.Name == "pipx" {
			pipx = i
			continue
		}
		if tool.PipxInstall != "" && pipx == -1 {
			t.Errorf("%s installs through pipx and comes before the pipx row", tool.Name)
		}
	}
	if pipx == -1 {
		t.Fatal("the required table has no pipx row")
	}
}
