// Tests for the setup tool cover the report vocabulary, probes, installation
// routes, and machine state that is not a binary.
//
// Each case that needs an external command sets the Shell seam and does not
// start a process. The three system-state steps use their apply branches only
// here.

package setup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/leaction"
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

// --- The command surface ----------------------------------------------------

// TestBareCommandListsAndInstallsNothing pins the split the owner ordered on
// 2026-09-02.
// VALIDATES: `le setup` answers the action listing, and `install` is still the
// row that carries the install run the bare name used to start.
// PREVENTS: a developer who typed the area name to see what the area holds
// finding that it has begun writing to the machine.
func TestBareCommandListsAndInstallsNothing(t *testing.T) {
	result, code := Answer(nil)
	if code != 0 {
		t.Fatalf("the bare command answered %d, want 0", code)
	}
	// A run answers *Report, so any other type here means the machine was read
	// or written before this test could stop it.
	listing, ok := result.(leaction.List)
	if !ok {
		t.Fatalf("the bare command returned %T, want leaction.List", result)
	}
	if listing.Area != area {
		t.Errorf("the listing names area %q, want %q", listing.Area, area)
	}
	if !reflect.DeepEqual(listing, Actions()) {
		t.Error("the bare command answers something other than the action listing")
	}

	install, found := leaction.Row{}, false
	for _, row := range listing.Actions {
		if row.Verb == installVerb {
			install, found = row, true
		}
	}
	switch {
	case !found:
		t.Fatalf("the listing does not name %q, so the run has no word left", installVerb)
	case !install.Writes:
		t.Errorf("%q is listed as a read, and it installs tools and rewrites vendor/", installVerb)
	}

	// The listing and the help hint are two surfaces on one command, and the
	// reader who typed `--help` never sees the listing, so both name the run.
	if !strings.Contains(Subs(), installVerb) {
		t.Errorf("the help hint is %q, and it must name %q", Subs(), installVerb)
	}
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
			if got := check.checkVerdict(); got != one.check {
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
	report.Add(Outcome{Name: "gopls-lsp-installed", State: StatePending})

	if got := report.checkVerdict(); got != 1 {
		t.Fatalf("the verdict is %d", got)
	}
	page := report.Text()
	if !strings.Contains(page, "Missing required tools: go") {
		t.Errorf("the missing tool is not named:\n%s", page)
	}
	if !strings.Contains(page, "Needs a step only you can take: gopls-lsp-installed") {
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
	for _, tool := range requiredTools() {
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
		"the pin, with build": {pinned + " (0.8.1)\n", 0, true},
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
	rec := rootShell("qemu-system-x86_64")
	setup := &Setup{Shell: rec.shell()}
	both := Tool{Name: "qemu", Probe: []string{"qemu-system-x86_64", "qemu-system-aarch64"}}
	if setup.Probe(both) {
		t.Error("a tool needing two binaries reported present with one")
	}
	either := Tool{Name: "qemu", Probe: both.Probe, ProbeAny: true}
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
	dirs := setup.e2fsprogsDirs()
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
			if got := setup.goplsHealth().Health; got != one.want {
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
	if got := setup.goplsHealth().Health; got != HealthNA {
		t.Errorf("the health is %s, want %s", got, HealthNA)
	}
}

func TestLanguageServerProbeUsesGoServerTable(t *testing.T) {
	plugins := lspPlugins()
	if len(plugins) != 1 || plugins[0].Binary != toolGopls ||
		!slices.Equal(plugins[0].Extensions, []string{".go"}) {
		t.Fatalf("LSP plugins = %#v, want one gopls entry", plugins)
	}
	rec := rootShell(toolGopls)
	setup := &Setup{Root: "/repo", Shell: rec.shell()}
	setup.runGoplsProbe()
	want := [][]string{{toolGopls, "symbols", goplsProbeFile}}
	if !reflect.DeepEqual(rec.calls, want) {
		t.Errorf("server probes ran %v, want %v", rec.ran(), want)
	}
}

func TestAnAbsentServerIsSkippedAndABrokenOneIsMissing(t *testing.T) {
	absent := visitServer(serverAnswer{Health: HealthAbsent, Detail: "x"})
	if absent.State != StateSkipped {
		t.Errorf("an absent server is %s: one missing server would be two failures", absent.State)
	}
	broken := visitServer(serverAnswer{Health: HealthBroken, Detail: "x"})
	if broken.State != StateMissing {
		t.Errorf("a broken server is %s: installing it again does not repair it", broken.State)
	}
	na := visitServer(serverAnswer{Health: HealthNA, Detail: "no file"})
	if na.State != StatePresent || !strings.HasPrefix(na.Detail, "n/a: ") {
		t.Errorf("a question that does not apply is %s (%s)", na.State, na.Detail)
	}
}

// --- The harness plugins ----------------------------------------------------

func TestLSPPluginTablePopulation(t *testing.T) {
	want := []lspPlugin{{
		Plugin:     "gopls-lsp",
		Binary:     toolGopls,
		Extensions: []string{".go"},
		Why:        "every LSP call on a .go file is refused without it",
	}}
	got := lspPlugins()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LSP server table changed:\ngot:  %#v\nwant: %#v", got, want)
	}
	if command := got[0].installCommand(); command != "/plugin install gopls-lsp@claude-plugins-official" {
		t.Errorf("gopls install command = %q", command)
	}
}

func TestEveryLSPServerHasOneRequiredInstallRoute(t *testing.T) {
	// VALIDATES: Each server producer names one required tool installation row.
	// PREVENTS: The server and tool tables drifting apart.
	for _, plugin := range lspPlugins() {
		var matches []Tool
		for _, tool := range requiredTools() {
			if slices.Contains(tool.Probe, plugin.Binary) {
				matches = append(matches, tool)
			}
		}
		if len(matches) != 1 {
			t.Errorf("%s binary %q occurs in %d required tool rows", plugin.Plugin, plugin.Binary, len(matches))
			continue
		}
		tool := matches[0]
		if !tool.installableBy(ManagerBrew) {
			t.Errorf("%s has no cross-platform install route on macOS", plugin.Plugin)
		}
		if !tool.installableBy(ManagerApt) {
			t.Errorf("%s has no cross-platform install route on Linux", plugin.Plugin)
		}
	}
}

func TestAnAbsentPluginRecordMeansNothingIsInstalled(t *testing.T) {
	setup := &Setup{Home: t.TempDir()}
	missing := setup.missingLSPPlugins()
	if len(missing) != len(lspPlugins()) {
		t.Errorf("%d of %d plugins are missing", len(missing), len(lspPlugins()))
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
	missing := setup.missingLSPPlugins()
	if len(missing) != 0 {
		t.Errorf("the missing set is %v", missing)
	}
}

// TestAMissingPluginIsPendingRatherThanMissing pins the state that says a human
// must run one command: nothing this tool runs can install a plugin.
func TestAMissingPluginIsPendingRatherThanMissing(t *testing.T) {
	setup := &Setup{Home: t.TempDir()}
	report := &Report{}
	outcome := setup.visitLspPlugin(report, lspPlugins()[0], setup.missingLSPPlugins())
	if outcome.State != StatePending {
		t.Errorf("a missing plugin is %s, want %s", outcome.State, StatePending)
	}
	if !strings.Contains(report.Text(), "/plugin install gopls-lsp@claude-plugins-official") {
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
			if got := setup.detectPackageManager(); got != one.want {
				t.Errorf("the manager is %q, want %q", got, one.want)
			}
		})
	}
}

func TestGoInstallRouteWinsOverThePackageManager(t *testing.T) {
	rec := rootShell("go", "apt-get")
	setup := &Setup{Shell: rec.shell()}
	installer := setup.NewInstaller(ManagerApt, &Report{})
	installer.Install(Tool{Name: "gopls", GoInstall: goplsTarget, Apt: "golang-gopls"})
	want := []string{"go install -mod=vendor " + goplsTarget}
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
	if colima.installableBy(ManagerApt) {
		t.Error("a brew-only tool claims an apt route")
	}
	setup := &Setup{Shell: rootShell().shell()}
	outcome := setup.visitTool(&Report{}, colima, ManagerApt, nil)
	if outcome.State != StateSkipped {
		t.Errorf("a tool with no route on this platform is %s, want %s", outcome.State, StateSkipped)
	}
}

func TestAGoInstallThatLandsOffPathIsPending(t *testing.T) {
	rec := rootShell("go")
	report := &Report{}
	setup := &Setup{Shell: rec.shell()}
	installer := setup.NewInstaller(ManagerApt, report)
	tool := Tool{Name: "gopls", Probe: []string{"gopls"}, GoInstall: goplsTarget, Required: true}
	outcome := setup.visitTool(report, tool, ManagerApt, installer)
	if outcome.State != StatePending {
		t.Errorf("a tool installed off PATH is %s, want %s", outcome.State, StatePending)
	}
	if !strings.Contains(report.Text(), "~/go/bin") {
		t.Errorf("the PATH fix is not named:\n%s", report.Text())
	}
}

// --- Vendoring --------------------------------------------------------------

// TestAFailedVendorFailsTheRun proves a failed `go mod vendor` makes setup fail.
func TestAFailedVendorFailsTheRun(t *testing.T) {
	rec := rootShell("go")
	rec.answers["go mod vendor"] = Result{Code: 1, Err: "inconsistent vendoring\n"}
	report := &Report{}
	setup := &Setup{Root: t.TempDir(), Shell: rec.shell()}

	if setup.vendorGoDeps(report) {
		t.Fatal("a failed `go mod vendor` reported success")
	}
	if !strings.Contains(report.Text(), "inconsistent vendoring") {
		t.Errorf("the complaint is not named:\n%s", report.Text())
	}
}

func TestVendoringWithoutGoIsAFailureRatherThanASkip(t *testing.T) {
	setup := &Setup{Root: t.TempDir(), Shell: rootShell().shell()}
	if setup.vendorGoDeps(&Report{}) {
		t.Error("vendoring with no go reported success")
	}
}

func TestVendoringDownloadsEveryApplianceModule(t *testing.T) {
	root := t.TempDir()
	for _, module := range []string{"github.com/gokrazy/gokrazy", "github.com/rtr7/kernel"} {
		path := filepath.Join(root, "gokrazy", "ze", "builddir", filepath.FromSlash(module), "go.mod")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("module fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rec := rootShell("go")
	if !(&Setup{Root: root, Shell: rec.shell()}).vendorGoDeps(&Report{}) {
		t.Fatal("native dependency setup failed")
	}
	got := rec.ran()
	if !slices.Equal(got, []string{
		"go mod tidy", "go mod vendor", "go mod download all", "go mod download all",
	}) {
		t.Fatalf("commands = %v", got)
	}
}

func TestVendoringReappliesOwnedVendorPatches(t *testing.T) {
	root := t.TempDir()
	for _, relative := range vendorPatchPaths {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture patch\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rec := rootShell("go", "git")
	for _, relative := range vendorPatchPaths {
		rec.answers["git apply --reverse --check "+relative] = Result{Code: 1, Err: "not applied"}
	}
	if !(&Setup{Root: root, Shell: rec.shell()}).vendorGoDeps(&Report{}) {
		t.Fatal("vendoring with applicable owned patches failed")
	}
	want := []string{"go mod tidy", "go mod vendor"}
	for _, relative := range vendorPatchPaths {
		want = append(want,
			"git apply --reverse --check "+relative,
			"git apply --check "+relative,
			"git apply "+relative,
		)
	}
	if got := rec.ran(); !slices.Equal(got, want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
}

func TestVendoringFailsWhenAnOwnedPatchCannotApply(t *testing.T) {
	root := t.TempDir()
	relative := vendorPatchPaths[0]
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture patch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := rootShell("go", "git")
	rec.answers["git apply --reverse --check "+relative] = Result{Code: 1, Err: "not applied"}
	rec.answers["git apply --check "+relative] = Result{Code: 1, Err: "does not apply"}
	report := &Report{}
	if (&Setup{Root: root, Shell: rec.shell()}).vendorGoDeps(report) {
		t.Fatal("vendoring accepted an owned patch that could not apply")
	}
	if !strings.Contains(report.Text(), "does not apply") {
		t.Fatalf("patch failure is not reported:\n%s", report.Text())
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
	state, err := setup.usernsState()
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
	for body, want := range map[string]userns{"1\n": usernsRestricted, "0\n": usernsOK} {
		knob := filepath.Join(t.TempDir(), "restrict")
		if err := os.WriteFile(knob, []byte(body), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		setup := &Setup{UsernsProc: knob}
		got, err := setup.usernsState()
		if err != nil || got != want {
			t.Errorf("%q answered %s (%v), want %s", body, got, err, want)
		}
	}
}

func TestAKernelWithNoSuchKnobHasNothingToDo(t *testing.T) {
	setup := &Setup{UsernsProc: filepath.Join(t.TempDir(), "absent")}
	state, err := setup.usernsState()
	if err != nil || state != usernsNA {
		t.Errorf("an absent knob answered %s (%v), want %s", state, err, usernsNA)
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

	if setup.applyUserns(report) {
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
	if got := absent.kvmState(); got != kvmNA {
		t.Errorf("no device answered %s, want %s", got, kvmNA)
	}

	device := filepath.Join(t.TempDir(), "kvm")
	if err := os.WriteFile(device, nil, 0o000); err != nil {
		t.Fatalf("write: %v", err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root can open a 0000 file, so the two unusable states are unreachable here")
	}

	member := &Setup{KvmDev: device, KvmGroupMember: func() bool { return true }, Shell: rootShell().shell()}
	if got := member.kvmState(); got != kvmPendingLogin {
		t.Errorf("a member with a stale session answered %s, want %s", got, kvmPendingLogin)
	}
	outsider := &Setup{KvmDev: device, KvmGroupMember: func() bool { return false }, Shell: rootShell().shell()}
	if got := outsider.kvmState(); got != kvmNoGroup {
		t.Errorf("a non-member answered %s, want %s", got, kvmNoGroup)
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
	if !setup.applyKvm(&Report{}) {
		t.Error("the group database lists the user and it reported failure")
	}
	if !slices.Equal(rec.ran(), []string{"usermod -aG kvm thomas"}) {
		t.Errorf("it ran %v", rec.ran())
	}
}

func TestLoopbackIsIPv6OnlyOnLinuxAndBothOnDarwin(t *testing.T) {
	linux := &Setup{GOOS: "linux"}
	if !slices.Equal(linux.loopbackAddresses(), []string{LoopbackIPv6}) {
		t.Errorf("linux wants %v", linux.loopbackAddresses())
	}
	darwin := &Setup{GOOS: "darwin"}
	want := []string{"127.0.0.2", "127.0.0.3", "127.0.0.4", "127.0.0.5", LoopbackIPv6}
	if !slices.Equal(darwin.loopbackAddresses(), want) {
		t.Errorf("darwin wants %v", darwin.loopbackAddresses())
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

	missing := setup.missingLoopback()
	if !slices.Equal(missing, []string{"127.0.0.4", "127.0.0.5"}) {
		t.Fatalf("the missing set is %v", missing)
	}
	setup.applyLoopback(&Report{}, missing)
	want := []string{"ifconfig lo0 alias 127.0.0.4", "ifconfig lo0 alias 127.0.0.5"}
	if !slices.Equal(rec.ran(), want) {
		t.Errorf("it ran %v, want %v", rec.ran(), want)
	}
}

// --- The tool table ---------------------------------------------------------

// TestTheApplianceRowsCarryTheDoctorsDependencies verifies that the
// authoritative tool rows carry each appliance dependency, its probe, and the
// doctor check that consumes it.
func TestTheApplianceRowsCarryTheDoctorsDependencies(t *testing.T) {
	byName := map[string]Tool{}
	for _, tool := range allTools() {
		byName[tool.Name] = tool
	}

	for name, want := range map[string]struct {
		probe  []string
		doctor string
	}{
		"grub":      {[]string{"grub-mkstandalone", "grub2-mkstandalone"}, "appliance-grub"},
		"xorriso":   {[]string{"xorriso"}, "appliance-xorriso"},
		"e2fsprogs": {[]string{"mkfs.ext4", "debugfs"}, "appliance-e2fsprogs"},
	} {
		tool, ok := byName[name]
		if !ok {
			t.Errorf("the tool table has no %s row, so setup installs nothing the appliance needs", name)
			continue
		}
		if !slices.Equal(tool.Probe, want.probe) {
			t.Errorf("%s probes %v, want %v", name, tool.Probe, want.probe)
		}
		if tool.DoctorCheck != want.doctor {
			t.Errorf("%s names doctor check %q, want %q", name, tool.DoctorCheck, want.doctor)
		}
	}
}

func TestToolTablePopulationAndInstallRoutes(t *testing.T) {
	groups := []struct {
		name     string
		tools    []Tool
		names    []string
		required bool
	}{
		{
			name:     "required",
			tools:    requiredTools(),
			names:    strings.Fields("go git protobuf jq golangci-lint staticcheck goimports gopls qemu e2fsprogs xorriso grub"),
			required: true,
		},
		{
			name:  "optional",
			tools: optionalTools(),
			names: strings.Fields("sshpass docker colima xl2tpd ppp"),
		},
	}
	seen := make(map[string]bool)
	for _, group := range groups {
		t.Run(group.name, func(t *testing.T) {
			names := make([]string, 0, len(group.tools))
			for _, tool := range group.tools {
				names = append(names, tool.Name)
				if tool.Required != group.required {
					t.Errorf("%s has Required=%t, want %t", tool.Name, tool.Required, group.required)
				}
				if tool.Name == "" || len(tool.Probe) == 0 {
					t.Errorf("incomplete tool row: %#v", tool)
				}
				if seen[tool.Name] {
					t.Errorf("tool %q occurs more than once", tool.Name)
				}
				seen[tool.Name] = true
				if !tool.installableBy(ManagerBrew) && !tool.installableBy(ManagerApt) {
					t.Errorf("%s has no supported installation route", tool.Name)
				}
				if tool.GoInstall != "" && (tool.Brew != "" || tool.Apt != "") {
					t.Errorf("%s mixes its preferred go install route with a package-manager route", tool.Name)
				}
			}
			if !slices.Equal(names, group.names) {
				t.Errorf("tool population or order changed: got %v, want %v", names, group.names)
			}
		})
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
		if got := grubAptPackage(machine); got != want {
			t.Errorf("%s installs %s, want %s", machine, got, want)
		}
	}
}
