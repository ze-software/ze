// Design: docs/labs/l2tp-interop.md -- real command identities cannot fall back to numeric labels
package l2tp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func scaleSelectorTestRows(t *testing.T) ([]scaleSelectorSession, scalePopulation) {
	t.Helper()
	old := scaleTestSessions()
	population := scaleTestPopulation(t, old)
	rows := make([]scaleSelectorSession, 0, len(old))
	for index, row := range old {
		rows = append(rows, scaleSelectorSession{ID: fmt.Sprintf("l2tp-%032x", index+1), Listener: "172.29.0.2:1701", Tunnel: row.Tunnel, Local: row.Local, Remote: row.Remote, Username: row.Username, Address: row.Address, State: row.State})
	}
	return rows, population
}

// TestScaleSelectorCensusRequiresEveryOriginalBirth keeps the old tuple/address
// census and additionally rejects absent/reused/changed immutable references.
func TestScaleSelectorCensusRequiresEveryOriginalBirth(t *testing.T) {
	for _, broken := range []string{"", "missing reference", "zero reference", "reused reference", "missing listener", "changed address", "missing subscriber"} {
		t.Run(broken, func(t *testing.T) {
			rows, original := scaleSelectorTestRows(t)
			switch broken {
			case "missing reference":
				rows[0].ID = ""
			case "zero reference":
				rows[0].ID = "l2tp-00000000000000000000000000000000"
			case "reused reference":
				rows[1].ID = rows[0].ID
			case "missing listener":
				rows[0].Listener = ""
			case "changed address":
				rows[0].Address = rows[1].Address
			case "missing subscriber":
				rows = rows[1:]
			}
			raw, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := readScaleSelectorSessions(raw, original)
			if broken == "" {
				if err != nil || len(actual) != len(original.byUser) {
					t.Fatalf("valid census: rows=%d error=%v", len(actual), err)
				}
				return
			}
			if err == nil {
				t.Fatal("accepted", broken)
			}
		})
	}
}

type scaleSelectorReadLab struct {
	recordingLab
	session        scaleSelectorSession
	broken         string
	refusalStatus  string
	refusalError   string
	transportError error
}

func (lab *scaleSelectorReadLab) Exec(_ context.Context, peer string, args []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	lab.calls = append(lab.calls, recordedCall{operation: "exec", peer: peer, arguments: slices.Clone(args)})
	if lab.transportError != nil {
		return interoplab.CommandResult{}, lab.transportError
	}
	if peer != peerZe || len(args) == 0 || args[0] != commandWget {
		return interoplab.CommandResult{}, errors.New("selector proof left the daemon REST entry point")
	}
	var command string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--post-data=") {
			continue
		}
		var body struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(arg, "--post-data=")), &body); err != nil {
			return interoplab.CommandResult{}, err
		}
		command = body.Command
	}
	if strings.HasPrefix(command, "clear ") {
		raw, err := json.Marshal(lnsPolicyReply{Status: lab.refusalStatus, Error: lab.refusalError, Data: json.RawMessage(`{}`)})
		return interoplab.CommandResult{Stdout: string(raw)}, err
	}
	if command == "" {
		return interoplab.CommandResult{}, errors.New("no actual command was posted")
	}
	row := map[string]any{"subscriber-id": lab.session.ID, "listener": lab.session.Listener, "session-id": lab.session.Local, "tunnel-id": lab.session.Tunnel}
	var data any
	switch {
	case strings.HasPrefix(command, "show l2tp session history "):
		row["transitions"] = []map[string]string{{"from": "wait-connect", "to": "established"}}
		if lab.broken == "history identity" {
			row["subscriber-id"] = "l2tp-ffffffffffffffffffffffffffffffff"
		}
		data = row
	case strings.HasPrefix(command, "show l2tp observer "):
		event := map[string]any{"subscriber-id": lab.session.ID, "session-id": lab.session.Local, "tunnel-id": lab.session.Tunnel, "type": "session-up"}
		if lab.broken == "mixed history" {
			event["subscriber-id"] = "l2tp-ffffffffffffffffffffffffffffffff"
		}
		if lab.broken == "audit side effect" {
			event["type"] = "disconnect-requested"
		}
		row["events"] = []any{event}
		data = row
	case strings.HasPrefix(command, "show l2tp session traffic "):
		for _, key := range []string{"rx-bytes", "tx-bytes", "rx-packets", "tx-packets", "rx-errors", "tx-errors", "rx-dropped", "tx-dropped"} {
			row[key] = uint64(3)
		}
		if lab.broken == "measured-zero-diagnostics" {
			for _, key := range []string{"rx-errors", "tx-errors", "rx-dropped", "tx-dropped"} {
				row[key] = uint64(0)
			}
		}
		if lab.broken == "missing counter" {
			delete(row, "rx-errors")
		}
		if lab.broken == "unknown counter" {
			row["rx-errors"] = nil
		}
		if lab.broken == "idle counter substitute" {
			row["rx-packets"] = uint64(0)
		}
		data = row
	case strings.HasPrefix(command, "show l2tp session id "):
		data = lab.session
	default:
		return interoplab.CommandResult{}, fmt.Errorf("unexpected actual selector command %q", command)
	}
	rawData, err := json.Marshal(data)
	if err != nil {
		return interoplab.CommandResult{}, err
	}
	raw, err := json.Marshal(lnsPolicyReply{Status: "done", Data: rawData})
	return interoplab.CommandResult{Stdout: string(raw)}, err
}

// TestScaleSelectedViewsUseQualifiedDaemonCommands makes the real REST writer
// carry every qualifier, including quoting, through all four read handlers.
func TestScaleSelectedViewsUseQualifiedDaemonCommands(t *testing.T) {
	rows, _ := scaleSelectorTestRows(t)
	lab := &scaleSelectorReadLab{session: rows[0]}
	listeners := map[string]string{rows[0].Listener: "main listener"}
	if err := scaleSelectedViews(t.Context(), lab, rows[0], listeners); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"show l2tp session ", "show l2tp session history ", "show l2tp observer ", "show l2tp session traffic "} {
		wanted := prefix + `id 1 tunnel-id 101 listener "main listener"`
		found := false
		for _, call := range lab.calls {
			for _, argument := range call.arguments {
				if !strings.HasPrefix(argument, "--post-data=") {
					continue
				}
				var body struct {
					Command string `json:"command"`
				}
				if err := json.Unmarshal([]byte(strings.TrimPrefix(argument, "--post-data=")), &body); err != nil {
					t.Fatal(err)
				}
				if body.Command == wanted {
					found = true
				}
			}
		}
		if !found {
			t.Fatal("read did not reach actual qualified command", wanted)
		}
	}
}

func TestScaleSelectedViewsRefuseForeignOrUnavailableEvidence(t *testing.T) {
	rows, _ := scaleSelectorTestRows(t)
	for _, broken := range []string{"history identity", "mixed history", "audit side effect", "missing counter", "unknown counter", "idle counter substitute"} {
		t.Run(broken, func(t *testing.T) {
			lab := &scaleSelectorReadLab{session: rows[0], broken: broken}
			if err := scaleSelectedViews(t.Context(), lab, rows[0], map[string]string{rows[0].Listener: "main"}); err == nil {
				t.Fatal("accepted", broken)
			}
		})
	}
}

func TestScaleSelectorRefusalRequiresItsActualCause(t *testing.T) {
	for _, test := range []struct {
		status, cause string
		transport     error
		valid         bool
	}{
		{"error", "L2TP session selection is ambiguous", nil, true},
		{"done", "ambiguous", nil, false},
		{"error", "unknown command", nil, false},
		{"error", "ambiguous", errors.New("REST connection failed"), false},
	} {
		lab := &scaleSelectorReadLab{refusalStatus: test.status, refusalError: test.cause, transportError: test.transport}
		err := scaleSelectorRefusal(t.Context(), lab, "clear l2tp session id 1", "ambiguous")
		if (err == nil) != test.valid {
			t.Fatalf("status=%s cause=%s result=%v", test.status, test.cause, err)
		}
	}
}

// TestScaleSelectedViewsAcceptMeasuredZeroDiagnostics accepts measured zeros
// through the real REST writer while requiring all four qualified reads.
func TestScaleSelectedViewsAcceptMeasuredZeroDiagnostics(t *testing.T) {
	rows, _ := scaleSelectorTestRows(t)
	lab := &scaleSelectorReadLab{session: rows[0], broken: "measured-zero-diagnostics"}
	listeners := map[string]string{rows[0].Listener: "main listener"}
	if err := scaleSelectedViews(t.Context(), lab, rows[0], listeners); err != nil {
		t.Fatal(err)
	}
	wanted := []string{
		`show l2tp session id 1 tunnel-id 101 listener "main listener"`,
		`show l2tp session history id 1 tunnel-id 101 listener "main listener"`,
		`show l2tp observer id 1 tunnel-id 101 listener "main listener"`,
		`show l2tp session traffic id 1 tunnel-id 101 listener "main listener"`,
	}
	if len(lab.calls) != len(wanted) {
		t.Fatalf("qualified reads: got %d calls, want %d", len(lab.calls), len(wanted))
	}
	for index, call := range lab.calls {
		var actual string
		for _, argument := range call.arguments {
			if !strings.HasPrefix(argument, "--post-data=") {
				continue
			}
			var body struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(argument, "--post-data=")), &body); err != nil {
				t.Fatal(err)
			}
			actual = body.Command
		}
		if actual != wanted[index] {
			t.Fatalf("qualified read %d: got %q, want %q", index, actual, wanted[index])
		}
	}
}
