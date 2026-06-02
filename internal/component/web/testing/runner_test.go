package webtesting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserOpenUsesHTTPSIgnoreOnlyForDaemonStart(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	browser := NewBrowser("https://127.0.0.1:1234")

	if err := browser.Open("/first"); err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := browser.Open("/second"); err != nil {
		t.Fatalf("second open: %v", err)
	}

	assertAgentCommands(t, logPath, []string{
		"--ignore-https-errors open https://127.0.0.1:1234/first",
		"wait --load networkidle",
		"open https://127.0.0.1:1234/second",
		"wait --load networkidle",
	})
}

func TestBrowserCloseResetsDaemonStart(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	browser := NewBrowser("https://127.0.0.1:1234")

	if err := browser.Open("/first"); err != nil {
		t.Fatalf("first open: %v", err)
	}
	browser.Close()
	if err := browser.Open("/second"); err != nil {
		t.Fatalf("second open: %v", err)
	}

	assertAgentCommands(t, logPath, []string{
		"--ignore-https-errors open https://127.0.0.1:1234/first",
		"wait --load networkidle",
		"--ignore-https-errors close --all",
		"--ignore-https-errors open https://127.0.0.1:1234/second",
		"wait --load networkidle",
	})
}

func installFakeAgentBrowser(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "agent-browser.log")
	scriptPath := filepath.Join(dir, "agent-browser")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$AGENT_BROWSER_TEST_LOG\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agent-browser: %v", err)
	}

	t.Setenv("AGENT_BROWSER_TEST_LOG", logPath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
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
