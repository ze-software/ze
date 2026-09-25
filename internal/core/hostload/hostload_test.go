package hostload

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// VALIDATES: Load.Contended requires load > CPUs AND a concurrent test process.
// PREVENTS: the verify status tool and the runner drifting on the "contended"
// verdict -- both now read this single definition.
func TestLoadContended(t *testing.T) {
	tests := []struct {
		name     string
		load     Load
		expected bool
	}{
		{"quiet machine", Load{LoadAvg1: 1.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 0}, false},
		{"loaded but no concurrent processes", Load{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 0}, false},
		{"loaded with concurrent le test", Load{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 2, GoTestProcs: 0}, true},
		{"loaded with concurrent go test", Load{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 3}, true},
		{"concurrency but quiet (no load gate)", Load{LoadAvg1: 2.0, CPUs: 8, ZeProcs: 2, GoTestProcs: 4}, false},
		{"at CPU boundary with concurrency", Load{LoadAvg1: 8.0, CPUs: 8, ZeProcs: 2, GoTestProcs: 0}, false},
		{"just over CPU boundary with concurrency", Load{LoadAvg1: 8.1, CPUs: 8, ZeProcs: 2, GoTestProcs: 0}, true},
		{"zero CPUs defaults to not contended", Load{LoadAvg1: 0, CPUs: 0, ZeProcs: 0, GoTestProcs: 0}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.load.Contended(); got != tc.expected {
				t.Errorf("Contended() = %v, want %v for %+v", got, tc.expected, tc.load)
			}
		})
	}
}

func TestLoadString(t *testing.T) {
	l := Load{LoadAvg1: 3.5, CPUs: 8, ZeProcs: 2, GoTestProcs: 1}
	s := l.String()
	if s == "" {
		t.Fatal("String() returned empty")
	}
	for _, want := range []string{"3.5", "8", "2", "1"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() = %q, missing %q", s, want)
		}
	}
}

func TestParseDigitsStopsAtNonDigit(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"42\n", 42},
		{"0\n", 0},
		{"", 0},
		{"error 42", 0},
		{"  7  ", 7},
		{"123abc", 123},
	}
	for _, tc := range tests {
		if got := parseDigits(tc.input); got != tc.want {
			t.Errorf("parseDigits(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestSnapshot(t *testing.T) {
	l := Snapshot()
	if l.CPUs <= 0 {
		t.Errorf("CPUs = %d, want > 0", l.CPUs)
	}
	if l.LoadAvg1 < 0 {
		t.Errorf("LoadAvg1 = %f, want >= 0", l.LoadAvg1)
	}
}

// VALIDATES: an argument list counts as the harness exactly when the program
// is le and its first argument is test.
// PREVENTS: the count reading a process name the harness no longer has, which
// made every run look uncontended once the harness became `le test <name>`.
func TestIsHarnessCommand(t *testing.T) {
	tests := []struct {
		args string
		want bool
	}{
		{"/home/u/ze/bin/le test ospf -p 4\n", true},
		{"le test fixture plugin/x 2165", true},
		{"./bin/le test", true},
		{"/home/u/ze/bin/le verify worktree", false},
		{"le", false},
		{"/usr/bin/le-test ospf", false},
		{"/usr/bin/ze test", false},
		{"go test ./internal/...", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := isHarnessCommand(tc.args); got != tc.want {
			t.Errorf("isHarnessCommand(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

// VALIDATES: the process sample harnessProcessCount reads carries a live
// `le test` process, and isHarnessCommand counts that entry.
// Method: copy sh to a file named le and run it as `le test`, where `test` is a
// script in its working directory that blocks on stdin. The assertion looks for
// this child's own entry, so other harness runs on the host cannot move it.
func TestHarnessSampleSeesALiveLeTest(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on PATH")
	}
	dir := t.TempDir()
	body, err := os.ReadFile(shell)
	if err != nil {
		t.Fatal(err)
	}
	le := filepath.Join(dir, "le")
	if err := os.WriteFile(le, body, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test"), []byte("read line\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	child := exec.Command(le, "test")
	child.Dir = dir
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Wait() //nolint:errcheck // the exit status is not the subject

	defer stdin.Close() //nolint:errcheck // closing ends the child's read

	want := le + " test"
	deadline := time.Now().Add(5 * time.Second)
	for {
		lines, err := processArguments()
		if err != nil {
			t.Fatalf("processArguments: %v", err)
		}
		for _, line := range lines {
			if strings.TrimSpace(line) != want {
				continue
			}
			if !isHarnessCommand(line) {
				t.Fatalf("isHarnessCommand(%q) = false for a live `le test`", line)
			}
			_, _ = io.WriteString(stdin, "done\n")
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no process sample entry %q", want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
