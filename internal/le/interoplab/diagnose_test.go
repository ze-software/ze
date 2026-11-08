package interoplab

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// fakeDiagnosticSource answers canned states, logs and probe output per
// container, and records every Exec, so a test sees which reads ran.
type fakeDiagnosticSource struct {
	states map[string]string
	logs   map[string]string
	execs  []string
}

func (f *fakeDiagnosticSource) ContainerState(_ context.Context, name string) (string, error) {
	state, ok := f.states[name]
	if !ok {
		return "", errors.New("no such container " + name)
	}
	return state, nil
}

func (f *fakeDiagnosticSource) Logs(_ context.Context, name string, _ int) (LogResult, error) {
	return LogResult{Text: f.logs[name], Available: true}, nil
}

func (f *fakeDiagnosticSource) Exec(_ context.Context, name string, argv []string, _ []EnvironmentVariable) (CommandResult, error) {
	f.execs = append(f.execs, name+" "+strings.Join(argv, " "))
	return CommandResult{Stdout: "answer of " + strings.Join(argv, " ") + "\n"}, nil
}

// TestDiagnosePeersNamesTheCause proves a failed scenario's report carries,
// per container it started and in start order, the state, the log tail and,
// only for a running container, the link and address reads; a peer whose
// container never started gets no block, and an unreadable state is
// reported, not skipped. Method: a fake source with one exited, one running,
// one unstarted and one vanished container.
func TestDiagnosePeersNamesTheCause(t *testing.T) {
	source := &fakeDiagnosticSource{
		states: map[string]string{"c-cpe": "exited exit=1 oom=false", "c-lac": "running exit=0 oom=false"},
		logs:   map[string]string{"c-cpe": "error: firewall: apply failed\n", "c-lac": ""},
	}
	peers := []PeerConfig{
		{Name: "ze-lac", Container: "c-lac"},
		{Name: "never", Container: "c-never"},
		{Name: "ze-cpe", Container: "c-cpe"},
		{Name: "gone", Container: "c-gone"},
	}
	blocks := diagnosePeers(context.Background(), source, peers, []string{"c-cpe", "c-lac", "c-gone"})
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks, want 3 (one per started container):\n%s", len(blocks), strings.Join(blocks, ""))
	}
	if !strings.HasPrefix(blocks[0], "peer ze-lac (container c-lac): state running") {
		t.Errorf("first block is not ze-lac's, in start order:\n%s", blocks[0])
	}
	if !strings.Contains(blocks[0], "log: empty") {
		t.Errorf("ze-lac's empty log is not reported as empty:\n%s", blocks[0])
	}
	if !strings.Contains(blocks[0], "answer of ip -d -o link show") || !strings.Contains(blocks[0], "answer of ip -br address show") {
		t.Errorf("running ze-lac's links and addresses are missing:\n%s", blocks[0])
	}
	if !strings.Contains(blocks[1], "state exited exit=1") || !strings.Contains(blocks[1], "    error: firewall: apply failed") {
		t.Errorf("exited ze-cpe's state or log tail is missing:\n%s", blocks[1])
	}
	if !strings.Contains(blocks[1], "not read, the container is not running") {
		t.Errorf("exited ze-cpe does not say why its namespace was not read:\n%s", blocks[1])
	}
	if !strings.Contains(blocks[2], "state unreadable: no such container c-gone") {
		t.Errorf("vanished container's unreadable state is not reported:\n%s", blocks[2])
	}
	want := []string{"c-lac ip -d -o link show", "c-lac ip -br address show"}
	if !slices.Equal(source.execs, want) {
		t.Errorf("execs = %q, want only the running container's probes %q", source.execs, want)
	}
}

// TestDiagnosePeerReadsDeclaredLogFiles proves a peer that logs to files
// gets the tail of each declared file in its block, after the namespace
// reads and in declaration order, so an LNS whose stdout is empty still
// shows why its call failed; a container that is not running is not
// probed. Method: a fake source records each Exec; one running peer
// declares two files, one exited peer declares one.
func TestDiagnosePeerReadsDeclaredLogFiles(t *testing.T) {
	source := &fakeDiagnosticSource{
		states: map[string]string{"c-lns": "running exit=0 oom=false", "c-dead": "exited exit=1 oom=false"},
		logs:   map[string]string{},
	}
	peers := []PeerConfig{
		{Name: "lns", Container: "c-lns", LogFiles: []string{"/var/log/xl2tpd.log", "/var/log/ppp/pppd-lns.log"}},
		{Name: "dead", Container: "c-dead", LogFiles: []string{"/var/log/accel-ppp.log"}},
	}
	blocks := diagnosePeers(context.Background(), source, peers, []string{"c-lns", "c-dead"})
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2:\n%s", len(blocks), strings.Join(blocks, ""))
	}
	want := []string{
		"c-lns ip -d -o link show",
		"c-lns ip -br address show",
		"c-lns tail -n 40 /var/log/xl2tpd.log",
		"c-lns tail -n 40 /var/log/ppp/pppd-lns.log",
	}
	if !slices.Equal(source.execs, want) {
		t.Errorf("execs = %q, want %q", source.execs, want)
	}
	if !strings.Contains(blocks[0], "  tail -n 40 /var/log/ppp/pppd-lns.log:\n    answer of tail -n 40 /var/log/ppp/pppd-lns.log") {
		t.Errorf("the pppd log tail is not in the LNS block:\n%s", blocks[0])
	}
	if !strings.Contains(blocks[1], "log files: not read, the container is not running") {
		t.Errorf("the exited peer does not say why its log file was not read:\n%s", blocks[1])
	}
}

// TestSuiteReportTextPrintsDiagnostics proves the terminal report prints a
// failed scenario's diagnostic blocks under its name, after its verdict.
func TestSuiteReportTextPrintsDiagnostics(t *testing.T) {
	report := SuiteReport{Failed: 1, FailedNames: []string{"access-up"}, Scenarios: []ScenarioResult{{
		Name: "access-up", Error: "ze-cpe's pppoe0 never held 203.0.113.1",
		Diagnostics: []string{"peer ze-cpe (container c): state exited exit=1 oom=false\n"},
	}}}
	text := report.Text()
	verdict := strings.Index(text, "interop: access-up: FAIL: ze-cpe's pppoe0 never held 203.0.113.1\n")
	block := strings.Index(text, "interop: access-up: peer ze-cpe (container c): state exited exit=1 oom=false\n")
	if verdict < 0 || block < verdict {
		t.Errorf("report does not print the diagnostic block after the verdict:\n%s", text)
	}
}
