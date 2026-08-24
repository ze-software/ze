package webtesting

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBrowserOpenUsesHTTPSIgnoreOnlyForDaemonStart(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	browser := newBrowser("https://127.0.0.1:1234")

	if err := browser.Open("/first"); err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := browser.Open("/second"); err != nil {
		t.Fatalf("second open: %v", err)
	}

	assertAgentCommands(t, logPath, []string{
		"--ignore-https-errors open https://127.0.0.1:1234/first",
		"eval " + inflightIdleExpr,
		"open https://127.0.0.1:1234/second",
		"eval " + inflightIdleExpr,
	})
}

// TestBrowserViewportBeforeOpenStartsDaemonWithHTTPSIgnore is the regression
// guard for the ERR_CERT_AUTHORITY_INVALID failure of option=viewport /
// option=locale tests. --ignore-https-errors is honored only at daemon launch,
// so it must ride whichever command starts the daemon. When SetViewport (or
// SetLocale) precedes the first Open, that pre-navigation command is the one
// that spawns the daemon and therefore must carry the flag; the later Open must
// NOT repeat it. Before the fix SetViewport used a plain runAgent, starting the
// daemon without cert-ignore, and the following open failed against the
// self-signed web cert.
func TestBrowserViewportBeforeOpenStartsDaemonWithHTTPSIgnore(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	browser := newBrowser("https://127.0.0.1:1234")

	if err := browser.setViewport(390, 844); err != nil {
		t.Fatalf("set viewport: %v", err)
	}
	if err := browser.setLocale("fr"); err != nil {
		t.Fatalf("set locale: %v", err)
	}
	if err := browser.Open("/"); err != nil {
		t.Fatalf("open: %v", err)
	}

	assertAgentCommands(t, logPath, []string{
		"--ignore-https-errors set viewport 390 844",
		`set headers {"Accept-Language":"fr"}`,
		"open https://127.0.0.1:1234/",
		"eval " + inflightIdleExpr,
	})
}

func TestBrowserCloseResetsDaemonStart(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	browser := newBrowser("https://127.0.0.1:1234")

	if err := browser.Open("/first"); err != nil {
		t.Fatalf("first open: %v", err)
	}
	browser.Close()
	if err := browser.Open("/second"); err != nil {
		t.Fatalf("second open: %v", err)
	}

	assertAgentCommands(t, logPath, []string{
		"--ignore-https-errors open https://127.0.0.1:1234/first",
		"eval " + inflightIdleExpr,
		"--ignore-https-errors close --all",
		"--ignore-https-errors open https://127.0.0.1:1234/second",
		"eval " + inflightIdleExpr,
	})
}

func TestBrowserSessionScopedEnv(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	browser := NewBrowserWithSession("https://127.0.0.1:5678", "test-web-01")

	if err := browser.Open("/page"); err != nil {
		t.Fatalf("open: %v", err)
	}

	cmds := readAgentLog(t, logPath)
	if len(cmds) == 0 {
		t.Fatal("no commands logged")
	}

	envPath := filepath.Join(filepath.Dir(logPath), "agent-browser-env.log")
	envData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env log: %v", err)
	}
	envLines := strings.Split(strings.TrimSuffix(string(envData), "\n"), "\n")
	if !slices.Contains(envLines, "AGENT_BROWSER_SESSION=test-web-01") {
		t.Errorf("AGENT_BROWSER_SESSION not set in env; got:\n%s", string(envData))
	}
}

func TestBrowserCloseOwnSession(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	browser := NewBrowserWithSession("https://127.0.0.1:5678", "sess-a")

	if err := browser.Open("/page"); err != nil {
		t.Fatalf("open: %v", err)
	}
	browser.Close()

	cmds := readAgentLog(t, logPath)
	lastClose := ""
	for _, c := range cmds {
		if strings.Contains(c, "close") {
			lastClose = c
		}
	}
	if lastClose == "" {
		t.Fatal("no close command logged")
	}
	if strings.Contains(lastClose, "--all") {
		t.Errorf("session-scoped browser used close --all; want close without --all: %q", lastClose)
	}
}

func TestBrowserNoSessionClosesAll(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	browser := newBrowser("https://127.0.0.1:5678")

	if err := browser.Open("/page"); err != nil {
		t.Fatalf("open: %v", err)
	}
	browser.Close()

	cmds := readAgentLog(t, logPath)
	lastClose := ""
	for _, c := range cmds {
		if strings.Contains(c, "close") {
			lastClose = c
		}
	}
	if lastClose == "" {
		t.Fatal("no close command logged")
	}
	if !strings.Contains(lastClose, "--all") {
		t.Errorf("sessionless browser should use close --all; got: %q", lastClose)
	}
}

func installFakeAgentBrowser(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "agent-browser.log")
	envLogPath := filepath.Join(dir, "agent-browser-env.log")
	scriptPath := filepath.Join(dir, "agent-browser")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$AGENT_BROWSER_TEST_LOG\"\n" +
		"env | grep ^AGENT_BROWSER_ | sort >> \"" + envLogPath + "\"\n" +
		"case \"$1\" in eval) echo true ;; esac\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agent-browser: %v", err)
	}

	t.Setenv("AGENT_BROWSER_TEST_LOG", logPath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func readAgentLog(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func assertAgentCommands(t *testing.T, logPath string, want []string) {
	t.Helper()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	gotText := strings.TrimSuffix(string(data), "\n")
	wantText := strings.Join(want, "\n")
	if gotText != wantText {
		t.Fatalf("agent-browser commands mismatch\nwant:\n%s\ngot:\n%s", wantText, gotText)
	}
}

// VALIDATES: every entry agentEnv adds is one NAME=VALUE pair.
// PREVENTS: two settings in one entry. agentEnv builds three entries from ONE
// textbuf.Buffer and resets it before the third alone, so the first two rely on
// String() detaching the buffer it returns. It does (measured here, 2026-08-16:
// each entry carries one setting), and nothing at the call site says so. A
// Buffer that kept its bytes would hand the browser
// AGENT_BROWSER_IDLE_TIMEOUT_MS=<ms>AGENT_BROWSER_INIT_SCRIPTS=<path>, one
// entry setting neither: the init script would never load, WaitLoad would fall
// back to networkidle, and waitMs would work around an idle window the browser
// never received. Nothing else reads these entries back.
func TestAgentEnvEntriesCarryOneSettingEach(t *testing.T) {
	for _, name := range []string{
		"AGENT_BROWSER_IDLE_TIMEOUT_MS",
		"AGENT_BROWSER_INIT_SCRIPTS",
		"AGENT_BROWSER_SESSION",
	} {
		if _, set := os.LookupEnv(name); set {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatalf("unset %s: %v", name, err)
			}
		}
	}

	browser := NewBrowserWithSession("https://127.0.0.1:1234", "probe-session")

	var added []string
	for _, entry := range browser.agentEnv() {
		if strings.HasPrefix(entry, "AGENT_BROWSER_") {
			added = append(added, entry)
		}
	}

	// The init-script entry is the one a concatenation corrupts, so its absence
	// would make the loop below vacuous.
	for _, name := range []string{
		"AGENT_BROWSER_IDLE_TIMEOUT_MS=",
		"AGENT_BROWSER_INIT_SCRIPTS=",
		"AGENT_BROWSER_SESSION=",
	} {
		if !slices.ContainsFunc(added, func(e string) bool { return strings.HasPrefix(e, name) }) {
			t.Fatalf("agentEnv set no %s entry; it added %q", name, added)
		}
	}

	for _, entry := range added {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			t.Errorf("entry %q carries no value", entry)

			continue
		}
		if strings.Contains(value, "AGENT_BROWSER_") {
			t.Errorf("entry %q carries a second setting inside the value of %s", entry, name)
		}
	}

	if !slices.Contains(added, "AGENT_BROWSER_IDLE_TIMEOUT_MS=60000") {
		t.Errorf("agentEnv must state the idle window waitMs works around; it added %q", added)
	}
}

// TestWBTimeoutBoundsTheRun
//
// VALIDATES: option=timeout is a wall-clock budget the runner enforces, and a
// test that exceeds it FAILS naming the directive.
// PREVENTS: the declaration meaning nothing. `parseWBOption` wrote
// WBTestCase.Timeout and nothing read it, while `pr.Run`
// (internal/test/runner/parallel.go) gives each web test the suite's context and
// no deadline of its own. A .wb that stopped making progress therefore hung the
// whole suite instead of failing one test. Every file that declared 30s or 90s
// declared nothing.
//
// The steps are `action=wait`, which sleeps and issues no browser command below
// browserIdleWindow/2, so this measures the runner's own bound and not the
// browser's.
func TestWBTimeoutBoundsTheRun(t *testing.T) {
	installFakeAgentBrowser(t)
	wait := WBAction{Kind: "wait", Values: map[string]string{"ms": "40"}, Line: 1}
	tc := &WBTestCase{
		Timeout: 60 * time.Millisecond,
		Actions: []WBAction{wait, wait, wait, wait, wait},
		Steps: []WBStep{
			{Type: WBStepAction, ActionIndex: 0},
			{Type: WBStepAction, ActionIndex: 1},
			{Type: WBStepAction, ActionIndex: 2},
			{Type: WBStepAction, ActionIndex: 3},
			{Type: WBStepAction, ActionIndex: 4},
		},
	}

	start := time.Now()
	res := runWBTestCase(tc, "https://127.0.0.1:1234", "sess-timeout")
	elapsed := time.Since(start)

	if res.Passed {
		t.Fatalf("a test over its declared budget must fail; it passed after %s", elapsed)
	}
	if !strings.Contains(res.Error, "option=timeout") {
		t.Errorf("the failure must name the directive that bounded it, got %q", res.Error)
	}
	// Five 40ms waits is 200ms of work against a 60ms budget. Stopping means
	// fewer steps ran than the file declared. Without the bound all five run.
	if len(res.Steps) >= len(tc.Steps) {
		t.Errorf("every step ran (%d of %d), so the budget bounded nothing",
			len(res.Steps), len(tc.Steps))
	}
	// ...and that at least one step DID run. This half separates a deadline
	// checked before each step from one evaluated once, on entry, against a zero
	// budget. The second stops with no step at all, and it satisfies every other
	// assertion here. Step one cannot exceed a budget that starts
	// when the loop does, so this is one for one with the check being per-round.
	if len(res.Steps) == 0 {
		t.Errorf("no step ran at all in %s: the budget is being evaluated before "+
			"the run rather than between its steps", elapsed)
	}
}

// TestWBTimeoutLeavesAnUnderBudgetRunAlone
//
// VALIDATES: the bound fails only a test that actually exceeds it.
// PREVENTS: the guard becoming the thing that reddens the suite. A deadline
// applied at the wrong moment, or seeded from a zero Timeout, would fail every
// test at step one and look exactly like a real regression.
func TestWBTimeoutLeavesAnUnderBudgetRunAlone(t *testing.T) {
	installFakeAgentBrowser(t)
	tc := &WBTestCase{
		Timeout: 10 * time.Second,
		Actions: []WBAction{{Kind: "wait", Values: map[string]string{"ms": "1"}, Line: 1}},
		Steps:   []WBStep{{Type: WBStepAction, ActionIndex: 0}},
	}
	res := runWBTestCase(tc, "https://127.0.0.1:1234", "sess-under")
	if !res.Passed {
		t.Fatalf("a run inside its budget must pass, got %q", res.Error)
	}
}

// TestParseWBTimeout
//
// VALIDATES: option=timeout parses into a duration, defaults to
// defaultWBTimeout, and refuses a value the runner cannot enforce.
// PREVENTS: a malformed timeout silently falling back to the default. The
// declaration exists to bound the run, so a value nothing can read must not
// leave the file looking bounded.
func TestParseWBTimeout(t *testing.T) {
	tc, err := ParseWBFile("option=timeout:value=45s\naction=open:path=/")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tc.Timeout != 45*time.Second {
		t.Errorf("timeout = %s, want 45s", tc.Timeout)
	}

	tc, err = ParseWBFile("action=open:path=/")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tc.Timeout != defaultWBTimeout {
		t.Errorf("default timeout = %s, want %s", tc.Timeout, defaultWBTimeout)
	}

	for _, bad := range []string{"abc", "30", "0s", "-5s"} {
		if _, err := ParseWBFile("option=timeout:value=" + bad); err == nil {
			t.Errorf("timeout %q parsed without error; a budget the runner cannot "+
				"enforce must not leave the file looking bounded", bad)
		}
	}
}

// TestWBZeroTimeoutMeansUnspecified
//
// VALIDATES: a WBTestCase built in code, with no Timeout, runs under the default
// budget rather than expiring before its first step.
// PREVENTS: the zero value reading as "no time at all". ParseWBFile always seeds
// the field, so the only way to reach the runner with zero is a struct literal,
// and there it means the caller declared nothing. Reading it literally failed
// TestRunWBTestCaseAppliesViewportAndLocaleFirst the moment the bound was added.
func TestWBZeroTimeoutMeansUnspecified(t *testing.T) {
	installFakeAgentBrowser(t)
	tc := &WBTestCase{
		Actions: []WBAction{{Kind: "wait", Values: map[string]string{"ms": "1"}, Line: 1}},
		Steps:   []WBStep{{Type: WBStepAction, ActionIndex: 0}},
	}
	if tc.Timeout != 0 {
		t.Fatalf("the fixture must carry the zero value, got %s", tc.Timeout)
	}
	res := runWBTestCase(tc, "https://127.0.0.1:1234", "sess-zero")
	if !res.Passed {
		t.Fatalf("an undeclared timeout must fall back to the default, got %q", res.Error)
	}
}
