package webtesting

import (
	"slices"
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
