package webtesting

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
