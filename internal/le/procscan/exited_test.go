package procscan

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// statLine builds a /proc/<pid>/stat line in the kernel's layout: pid, the
// comm in parentheses, then the state and the remaining fields, with the start
// time as the 22nd field (the 20th after the closing parenthesis).
func statLine(comm, state, start string) []byte {
	var line bytes.Buffer
	line.WriteString("4242 (")
	line.WriteString(comm)
	line.WriteString(") ")
	line.WriteString(state)
	for field := 1; field < 19; field++ {
		line.WriteString(" 0")
	}
	line.WriteString(" ")
	line.WriteString(start)
	line.WriteString(" 1000 0\n")
	return line.Bytes()
}

// VALIDATES: a /proc stat line whose state is zombie or dead reads as exited,
// every other state as running, and the start token is the 22nd field.
// PREVENTS: an exited-but-unreaped `go test` child keeping its lab owner alive
// after a docker-peers Ctrl-C, so the orphan sweep left its containers for the
// next sweep.
func TestParseProcStatReadsTheExitedStates(t *testing.T) {
	for _, row := range []struct {
		state  string
		exited bool
	}{
		{"R", false}, {"S", false}, {"D", false}, {"T", false}, {"t", false}, {"I", false},
		{"Z", true}, {"X", true}, {"x", true},
	} {
		start, exited, ok := parseProcStat(statLine("go (test) )", row.state, "987654"))
		if !ok {
			t.Errorf("state %s: parseProcStat refused a well-formed line", row.state)
			continue
		}
		if start != "987654" {
			t.Errorf("state %s: start = %q; want 987654", row.state, start)
		}
		if exited != row.exited {
			t.Errorf("state %s: exited = %v; want %v", row.state, exited, row.exited)
		}
	}
}

func TestParseProcStatRefusesAShortLine(t *testing.T) {
	for _, line := range []string{"", "4242 (go) Z", "4242 go Z 0 0 0", "4242 (go) Z 0 0 0 0"} {
		if _, _, ok := parseProcStat([]byte(line)); ok {
			t.Errorf("parseProcStat(%q) answered; want a refusal", line)
		}
	}
}

// VALIDATES: a ps line whose state is zombie or dead is left out of the scan,
// and a running line keeps its pid, argv and CLI mark.
// PREVENTS: the BSD and macOS path (no /proc) reporting a zombie as present.
func TestParsePSLeavesOutExitedProcesses(t *testing.T) {
	output := "  101 Ss   Mon Oct 19 10:00:00 2026     01:00 claude claude --resume\n" +
		"  102 Z    Mon Oct 19 10:00:00 2026     00:30 go <defunct>\n" +
		"  103 X    Mon Oct 19 10:00:00 2026     00:30 go <defunct>\n" +
		"  104 R+   Mon Oct 19 10:00:00 2026  01:02:03 go go test ./...\n"
	processes := parsePS(output)
	if len(processes) != 2 {
		t.Fatalf("parsePS kept %d processes; want 2 (101 and 104): %+v", len(processes), processes)
	}
	if processes[0].PID != 101 || !processes[0].CLI || processes[0].Start != "Mon_Oct_19_10_00_00_2026" {
		t.Errorf("first process = %+v; want pid 101, a CLI, start Mon_Oct_19_10_00_00_2026", processes[0])
	}
	if processes[1].PID != 104 || processes[1].CLI || len(processes[1].Argv) != 3 {
		t.Errorf("second process = %+v; want pid 104, not a CLI, argv go test ./...", processes[1])
	}
}

// VALIDATES: through the real entry points, a child that has exited and that
// nobody has reaped yet is absent from Lookup and from Scan.
// METHOD: start `true`, wait for /proc to show it as a zombie, ask, then reap.
func TestAnUnreapedChildIsAbsent(t *testing.T) {
	if _, err := os.Stat(procRoot); err != nil {
		t.Skip("no /proc on this host; TestParsePSLeavesOutExitedProcesses covers the ps path")
	}
	child := exec.CommandContext(t.Context(), "true")
	if err := child.Start(); err != nil {
		t.Fatalf("start true: %v", err)
	}
	pid := child.Process.Pid
	defer child.Wait() //nolint:errcheck // reaping the zombie is the point; its status is irrelevant
	statPath := filepath.Join(procRoot, strconv.Itoa(pid), "stat")
	deadline := time.Now().Add(5 * time.Second)
	for {
		stat, err := os.ReadFile(statPath)
		if err != nil {
			t.Fatalf("read %s while the child is unreaped: %v", statPath, err)
		}
		// Read the state byte here rather than through parseProcStat, so the
		// wait does not depend on the code under test.
		if _, after, found := bytes.Cut(stat, []byte(") Z")); found && len(after) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pid %d never became a zombie within 5s", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, found, err := Lookup(pid)
	if err != nil {
		t.Fatalf("Lookup(%d) = error %v; want an answer", pid, err)
	}
	if found {
		t.Errorf("Lookup(%d) found an unreaped zombie; want absent", pid)
	}
	processes, err := Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, process := range processes {
		if process.PID == pid {
			t.Errorf("Scan reported the unreaped zombie %d as running", pid)
		}
	}
}
