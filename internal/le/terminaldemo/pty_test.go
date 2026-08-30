package terminaldemo

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTapeParserPreservesTheRecordingContract(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "common.tape"), strings.Join([]string{
		`Set Shell "bash"`,
		"Set FontSize 20",
		"Set Width 1680",
		"Set Height 1008",
		"Set Padding 17",
		"Set TypingSpeed 125ms",
		"Set WaitTimeout 30s",
	}, "\n"))
	path := filepath.Join(root, "demo.tape")
	mustWrite(t, path, strings.Join([]string{
		"Source common.tape",
		"Output artifacts/example.gif",
		"Hide",
		`Type "show bgp"`,
		"Enter",
		`Wait+Screen /Established/`,
		"Show",
	}, "\n"))

	tape, err := parseTape(path, root)
	if err != nil {
		t.Fatalf("parse tape: %v", err)
	}
	if tape.output != "artifacts/example.gif" || tape.settings.shell != "bash" || tape.settings.typing != 125*time.Millisecond {
		t.Fatalf("parsed tape = %#v", tape)
	}
	columns, rows, err := terminalGrid(tape.settings)
	if err != nil {
		t.Fatalf("terminal grid: %v", err)
	}
	if columns != 137 || rows != 36 {
		t.Fatalf("terminal grid = %dx%d, want 137x36", columns, rows)
	}
	if len(tape.actions) != 5 || tape.actions[1].text != "show bgp" || tape.actions[3].pattern.String() != "Established" {
		t.Fatalf("actions = %#v", tape.actions)
	}
}

func TestTapeParserRefusesDefinitionsItCannotDrive(t *testing.T) {
	for _, testCase := range []struct {
		name string
		text string
		want string
	}{
		{name: "unknown directive", text: "Launch now\n", want: "unknown directive"},
		{name: "bare wait", text: "Wait /ready/\n", want: "use Wait+Screen"},
		{name: "late setting", text: "Enter\nSet Width 80\n", want: "after the session starts"},
		{name: "bad duration", text: "Sleep tomorrow\n", want: "needs a duration"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "demo.tape")
			mustWrite(t, path, testCase.text)
			_, err := parseTape(path, "")
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("parse error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func TestNativeRecorderDrivesARealPTYAndWritesAsciicastV2(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "session.gif")
	path := filepath.Join(root, "demo.tape")
	mustWrite(t, path, strings.Join([]string{
		`Set Shell "bash"`,
		"Set TypingSpeed 1ms",
		"Set WaitTimeout 2s",
		"Output " + output,
		`Type "printf 'READY\\n'; exit"`,
		"Enter",
		"Wait+Screen /READY/",
	}, "\n"))

	var stdout bytes.Buffer
	if err := recordTape(path, &stdout); err != nil {
		t.Fatalf("record tape: %v", err)
	}
	castPath := strings.TrimSuffix(output, ".gif") + ".cast"
	content, err := os.ReadFile(castPath)
	if err != nil {
		t.Fatalf("read cast: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 2 {
		t.Fatalf("cast has %d lines: %s", len(lines), content)
	}
	var header struct {
		Version int `json:"version"`
		Width   int `json:"width"`
		Height  int `json:"height"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		t.Fatalf("decode cast header: %v", err)
	}
	if header.Version != 2 || header.Width < 1 || header.Height < 1 || !bytes.Contains(content, []byte("READY")) {
		t.Fatalf("cast header/content = %#v / %s", header, content)
	}
	if !strings.Contains(stdout.String(), "recorded "+castPath) {
		t.Fatalf("recorder output = %q", stdout.String())
	}
}

// VALIDATES: a wait pattern matches a phrase that carries a style change inside
// it, which is what a highlighted menu row paints.
// PREVENTS: matching the raw stream again, where an escape between two words of
// the pattern makes a phrase that is plainly on screen never match, and the
// recorder times out naming text its own error message then prints.
func TestPTYWaitMatchesAPhraseWithAnEscapeInsideIt(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunPTY([]string{
		"--ready", "READY",
		"--timeout", "5",
		"--command", "open",
		"--command", "@wait > show",
		"--command", "done",
		"--",
		"sh", "-c", `printf 'READY\n'; read first; printf '> \033[32mshow\033[0m\n'; read second; printf 'got:%s\n' "$second"`,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunPTY code = %d, stderr = %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "> show") || !strings.Contains(got, "got:done") {
		t.Fatalf("output = %q, want the styled phrase and the command that followed the wait", got)
	}
}

func TestLegacyPTYCommandsPreserveWaitWindowsAndStripANSI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunPTY([]string{
		"--ready", "READY",
		"--timeout", "2",
		"--command", "hello",
		"--",
		"sh", "-c", `printf '\033[31mREADY\033[0m\n'; read line; printf 'got:%s\n' "$line"`,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunPTY code = %d, stderr = %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "READY") || !strings.Contains(got, "got:hello") || strings.Contains(got, "\x1b[") {
		t.Fatalf("legacy PTY output = %q", got)
	}
}
