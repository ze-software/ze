package webtesting

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestParseHarnessDirectives verifies the new .wb option/action directives
// (viewport, locale, auth, login) parse into the WBTestCase fields.
//
// VALIDATES: option=viewport/locale/auth populate Viewport/Locale/Auth and
// RequiresAuth; action=login parses as an action step.
// PREVENTS: a silent regression where a new directive is ignored (the parser
// fails closed on unknown directives, but the field plumbing needs its own
// guard).
func TestParseHarnessDirectives(t *testing.T) {
	content := strings.Join([]string{
		"option=viewport:width=390:height=844",
		"option=locale:lang=fr",
		"option=auth:user=noc:password=secret:role=read-only",
		"option=auth:user=root:password=admin-pw:role=admin",
		"action=login:user=noc:password=secret",
		"action=open:path=/admin/",
		"expect=element:text=Forbidden",
	}, "\n")

	tc, err := ParseWBFile(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tc.Viewport.Width != 390 || tc.Viewport.Height != 844 {
		t.Errorf("viewport = %dx%d, want 390x844", tc.Viewport.Width, tc.Viewport.Height)
	}
	if tc.Locale != "fr" {
		t.Errorf("locale = %q, want fr", tc.Locale)
	}
	if !tc.RequiresAuth() || len(tc.Auth) != 2 {
		t.Fatalf("auth users = %+v, want 2 and RequiresAuth", tc.Auth)
	}
	if tc.Auth[0] != (WBAuthUser{Name: "noc", Password: "secret", Role: "read-only"}) {
		t.Errorf("auth[0] = %+v", tc.Auth[0])
	}
	if tc.Auth[1].Role != "admin" {
		t.Errorf("auth[1].Role = %q, want admin", tc.Auth[1].Role)
	}
	// The login action is the first action step.
	if len(tc.Actions) == 0 || tc.Actions[0].Kind != "login" || tc.Actions[0].Values["user"] != "noc" {
		t.Errorf("first action = %+v, want login user=noc", tc.Actions)
	}
}

// TestParseAuthRequiresUser verifies auth without a user= is a parse error.
func TestParseAuthRequiresUser(t *testing.T) {
	if _, err := ParseWBFile("option=auth:password=x"); err == nil {
		t.Fatal("expected error for auth without user=")
	}
}

// TestParseViewportRejectsNonNumeric verifies a bad viewport width errors.
func TestParseViewportRejectsNonNumeric(t *testing.T) {
	if _, err := ParseWBFile("option=viewport:width=wide"); err == nil {
		t.Fatal("expected error for non-numeric viewport width")
	}
}

// TestSetViewportEmitsCommand verifies the viewport directive drives the real
// agent-browser "set viewport <w> <h>" command. As the first command on a fresh
// browser it also starts the daemon, so it must carry --ignore-https-errors.
func TestSetViewportEmitsCommand(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	b := NewBrowser("https://127.0.0.1:1234")
	if err := b.SetViewport(390, 844); err != nil {
		t.Fatalf("SetViewport: %v", err)
	}
	cmds := readAgentLog(t, logPath)
	if !slices.Contains(cmds, "--ignore-https-errors set viewport 390 844") {
		t.Errorf("commands = %v, want '--ignore-https-errors set viewport 390 844'", cmds)
	}
}

// TestSetLocaleEmitsAcceptLanguageHeader verifies the locale directive drives
// the agent-browser "set headers" command with an Accept-Language override. As
// the first command on a fresh browser it also starts the daemon, so it must
// carry --ignore-https-errors.
func TestSetLocaleEmitsAcceptLanguageHeader(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	b := NewBrowser("https://127.0.0.1:1234")
	if err := b.SetLocale("fr"); err != nil {
		t.Fatalf("SetLocale: %v", err)
	}
	cmds := readAgentLog(t, logPath)
	if !slices.Contains(cmds, `--ignore-https-errors set headers {"Accept-Language":"fr"}`) {
		t.Errorf("commands = %v, want the Accept-Language header set with cert-ignore", cmds)
	}
}

// TestLoginActionDrivesLoginForm verifies the login action fills the username
// and password fields and submits.
func TestLoginActionDrivesLoginForm(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	b := NewBrowser("https://127.0.0.1:1234")
	if err := b.Login("noc", "secret"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	cmds := readAgentLog(t, logPath)
	joined := strings.Join(cmds, "\n")
	for _, want := range []string{"fill #username noc", "fill #password secret", "press Enter"} {
		if !strings.Contains(joined, want) {
			t.Errorf("login commands missing %q; got:\n%s", want, joined)
		}
	}
}

// TestParseWaitUntilDirective verifies the state-based wait parses into an action
// carrying both of its keys.
//
// VALIDATES: action=wait-until:path=..:contains=.. becomes a wait-until action
// step with path and contains.
// PREVENTS: the directive parsing as a bare kind with its keys dropped, which
// would make WaitUntil return its missing-parameter error at run time instead of
// waiting on anything.
func TestParseWaitUntilDirective(t *testing.T) {
	tc, err := ParseWBFile("action=wait-until:path=/config/diff:contains=Review changes (0)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(tc.Actions) != 1 || tc.Actions[0].Kind != "wait-until" {
		t.Fatalf("actions = %+v, want one wait-until action", tc.Actions)
	}
	if got := tc.Actions[0].Values["path"]; got != "/config/diff" {
		t.Errorf("path = %q, want /config/diff", got)
	}
	if got := tc.Actions[0].Values["contains"]; got != "Review changes (0)" {
		t.Errorf("contains = %q, want %q", got, "Review changes (0)")
	}
}

// TestWaitUntilRefetchesUntilTheServerReportsTheState is the reason the directive
// exists: the state it waits for lives on the SERVER, so each round must fetch the
// page again. A wait that sampled the DOM once could only ever report the state
// that was true before the action it follows.
//
// VALIDATES: WaitUntil re-opens the path until the served HTML contains the wanted
// text, then returns nil.
// PREVENTS: a single-sample wait passing on a stale readback, and a wait that
// re-reads the same loaded page forever.
func TestWaitUntilRefetchesUntilTheServerReportsTheState(t *testing.T) {
	logPath := installFlippingAgentBrowser(t, 3)
	b := NewBrowser("https://127.0.0.1:1234")

	if err := b.WaitUntil("/config/diff", "ready"); err != nil {
		t.Fatalf("WaitUntil: %v", err)
	}

	opens := 0
	for _, c := range readAgentLog(t, logPath) {
		if strings.HasPrefix(c, "open ") {
			opens++
		}
	}
	if opens != 3 {
		t.Errorf("opened %d times, want 3 (one per poll until the server flipped)", opens)
	}
}

// TestWaitUntilRejectsMissingParameters keeps a mistyped directive loud. Both keys
// are required, and the run-time error names them; the deadline for a condition
// that never holds is retryCommand's, covered by TestRetryPositiveReturnsLastError.
//
// VALIDATES: an empty path or an empty contains fails immediately.
// PREVENTS: a wait-until with a dropped key silently waiting on nothing and
// passing.
func TestWaitUntilRejectsMissingParameters(t *testing.T) {
	installFlippingAgentBrowser(t, 1)
	b := NewBrowser("https://127.0.0.1:1234")

	if err := b.WaitUntil("", "ready"); err == nil {
		t.Error("WaitUntil with no path returned nil, want an error")
	}
	if err := b.WaitUntil("/config/diff", ""); err == nil {
		t.Error("WaitUntil with no contains returned nil, want an error")
	}
}

// installFlippingAgentBrowser installs a fake agent-browser that serves
// "<html>pending</html>" until the flip-th `open`, then "<html>ready</html>". It
// models the only thing WaitUntil is for: a server state that changes between two
// fetches of the same path.
func installFlippingAgentBrowser(t *testing.T, flip int) string {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "agent-browser.log")
	opensPath := filepath.Join(dir, "opens")
	scriptPath := filepath.Join(dir, "agent-browser")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$AGENT_BROWSER_TEST_LOG\"\n" +
		"case \"$1\" in\n" +
		"  eval) echo true ;;\n" +
		"  open) echo x >> \"" + opensPath + "\" ;;\n" +
		"  get)\n" +
		"    n=$(wc -l < \"" + opensPath + "\")\n" +
		"    if [ \"$n\" -ge " + strconv.Itoa(flip) + " ]; then echo '<html>ready</html>'; else echo '<html>pending</html>'; fi ;;\n" +
		"esac\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agent-browser: %v", err)
	}
	if err := os.WriteFile(opensPath, nil, 0o600); err != nil {
		t.Fatalf("write opens counter: %v", err)
	}

	t.Setenv("AGENT_BROWSER_TEST_LOG", logPath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

// TestRunWBTestCaseAppliesViewportAndLocaleFirst verifies the runner applies the
// viewport and locale session options before the first navigation.
func TestRunWBTestCaseAppliesViewportAndLocaleFirst(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	tc := &WBTestCase{
		Viewport: WBViewport{Width: 390, Height: 844},
		Locale:   "fr",
		Actions:  []WBAction{{Kind: "open", Values: map[string]string{"path": "/login"}, Line: 1}},
		Steps:    []WBStep{{Type: WBStepAction, ActionIndex: 0}},
	}
	res := runWBTestCase(tc, "https://127.0.0.1:1234", "sess-x")
	if !res.Passed {
		t.Fatalf("run failed: %s", res.Error)
	}
	cmds := readAgentLog(t, logPath)
	// Contains (not HasPrefix): the daemon-starting command carries a leading
	// --ignore-https-errors global flag, so the "set viewport"/"set headers"
	// text is not necessarily at the start of the logged line.
	viewportIdx := slices.IndexFunc(cmds, func(c string) bool { return strings.Contains(c, "set viewport") })
	localeIdx := slices.IndexFunc(cmds, func(c string) bool { return strings.Contains(c, "set headers") })
	openIdx := slices.IndexFunc(cmds, func(c string) bool { return strings.Contains(c, "open https://") })
	if viewportIdx < 0 || localeIdx < 0 || openIdx < 0 {
		t.Fatalf("missing commands: viewport=%d locale=%d open=%d in %v", viewportIdx, localeIdx, openIdx, cmds)
	}
	if viewportIdx > openIdx || localeIdx > openIdx {
		t.Errorf("viewport(%d)/locale(%d) must precede open(%d)", viewportIdx, localeIdx, openIdx)
	}
}
