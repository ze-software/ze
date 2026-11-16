// Design: docs/labs/l2tp-interop.md -- independent many-session evidence.
// Related: scale_evidence.go -- typed daemon, FRR and accounting boundaries.
package l2tp

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"testing"
)

type scaleTestSession struct {
	Tunnel   uint16 `json:"tunnel-local-tid"`
	Local    uint16 `json:"local-sid"`
	Remote   uint16 `json:"remote-sid"`
	Username string `json:"username"`
	Address  string `json:"assigned-addr"`
	State    string `json:"state"`
}

func scaleTestSessions() []scaleTestSession {
	return []scaleTestSession{
		{101, 1, 1, "scale-1-01", "10.100.0.2", "established"},
		{101, 2, 2, "scale-1-02", "10.100.0.3", "established"},
		{202, 1, 1, "scale-2-01", "10.100.0.4", "established"},
		{202, 2, 2, "scale-2-02", "10.100.0.5", "established"},
		{303, 1, 1, "scale-3-01", "10.100.0.6", "established"},
		{303, 2, 2, "scale-3-02", "10.100.0.7", "established"},
	}
}

func scaleTestPopulation(t *testing.T, rows []scaleTestSession) scalePopulation {
	t.Helper()
	users := make([]string, 0, len(rows))
	for _, row := range rows {
		users = append(users, row.Username)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	population, err := readScaleSessions(raw, users)
	if err != nil {
		t.Fatal(err)
	}
	return population
}

func TestScaleSessionEvidence(t *testing.T) {
	rows := scaleTestSessions()
	population := scaleTestPopulation(t, rows)
	if len(population.byUser) != len(rows) {
		t.Fatalf("got %d sessions, want %d", len(population.byUser), len(rows))
	}
	if population.byUser["scale-1-01"].local != population.byUser["scale-2-01"].local {
		t.Fatal("same local SID on distinct tunnels was not preserved")
	}
	for _, name := range []string{"duplicate address", "duplicate tuple", "missing username", "zero SID", "not established", "missing address", "unknown username"} {
		t.Run(name, func(t *testing.T) {
			changed := append([]scaleTestSession(nil), rows...)
			switch name {
			case "duplicate address":
				changed[1].Address = changed[0].Address
			case "duplicate tuple":
				changed[1].Local = changed[0].Local
			case "missing username":
				changed[1].Username = ""
			case "zero SID":
				changed[1].Local = 0
			case "not established":
				changed[1].State = "connecting"
			case "missing address":
				changed[1].Address = ""
			case "unknown username":
				changed[1].Username = "foreign"
			}
			raw, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			users := make([]string, 0, len(rows))
			for _, row := range rows {
				users = append(users, row.Username)
			}
			if _, err := readScaleSessions(raw, users); err == nil {
				t.Fatal("invalid session evidence accepted")
			}
		})
	}
}

func TestScaleFRRRouteEvidence(t *testing.T) {
	prefix := netip.MustParsePrefix("10.100.0.2/32")
	for _, test := range []struct {
		name, raw    string
		present, bad bool
	}{
		{"real Ze path", `{"prefix":"10.100.0.2/32","pathCount":1,"paths":[{"valid":true,"peer":{"peerId":"172.29.0.2"}}]}`, true, false},
		{"withdrawn", `{}`, false, false},
		{"key only", `{"prefix":"10.100.0.2/32"}`, false, true},
		{"empty paths", `{"prefix":"10.100.0.2/32","pathCount":1,"paths":[]}`, false, true},
		{"zero count", `{"prefix":"10.100.0.2/32","pathCount":0,"paths":[{"valid":true,"peer":{"peerId":"172.29.0.2"}}]}`, false, true},
		{"wrong prefix", `{"prefix":"10.100.0.3/32","pathCount":1,"paths":[{"valid":true,"peer":{"peerId":"172.29.0.2"}}]}`, false, true},
		{"foreign path", `{"prefix":"10.100.0.2/32","pathCount":1,"paths":[{"valid":true,"peer":{"peerId":"172.29.0.9"}}]}`, false, true},
		{"malformed", `{`, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			present, err := readScaleRoute([]byte(test.raw), prefix)
			if (err != nil) != test.bad {
				t.Fatalf("error = %v, want bad %v", err, test.bad)
			}
			if present != test.present {
				t.Fatalf("present = %v, want %v", present, test.present)
			}
		})
	}
}

func scaleTestDetail(rows []scaleTestSession, status string) string {
	var result strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&result, "Thu Nov 15 00:00:00 2026\n\tAcct-Status-Type = %s\n\tAcct-Session-Id = \"full-session-%d-%d\"\n\tUser-Name = \"%s\"\n\tFramed-IP-Address = %s\n\n", status, row.Tunnel, row.Local, row.Username, row.Address)
	}
	return result.String()
}

func TestScaleAccountingEvidence(t *testing.T) {
	rows := scaleTestSessions()
	population := scaleTestPopulation(t, rows)
	detail := scaleTestDetail(rows, "Start")
	accounting, err := readScaleAccounting(detail, population, "Start")
	if err != nil {
		t.Fatal(err)
	}
	if accounting.byUser[rows[0].Username] != "full-session-101-1" {
		t.Fatal("full accounting ID lost")
	}
	for _, test := range []struct{ name, detail string }{
		{"missing", scaleTestDetail(rows[1:], "Start")},
		{"wrong user", strings.Replace(detail, rows[0].Username, "foreign", 1)},
		{"wrong address", strings.Replace(detail, "Framed-IP-Address = 10.100.0.2", "Framed-IP-Address = 10.100.0.99", 1)},
		{"duplicate ID", strings.Replace(detail, "full-session-202-1", "full-session-101-1", 1)},
		{"conflicting user", detail + strings.Replace(scaleTestDetail(rows[:1], "Start"), "full-session-101-1", "different-full-id", 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := readScaleAccounting(test.detail, population, "Start"); err == nil {
				t.Fatal("invalid accounting evidence accepted")
			}
		})
	}
}

func TestScaleDisconnectTarget(t *testing.T) {
	rows := scaleTestSessions()
	before := scaleTestPopulation(t, rows)
	after := scaleTestPopulation(t, rows[1:])
	if err := requireScaleRemoval(before, after, rows[0].Username); err != nil {
		t.Fatal(err)
	}
	if err := requireScaleRemoval(before, before, rows[0].Username); err == nil {
		t.Fatal("target still present accepted")
	}
	wrong := scaleTestPopulation(t, append(append([]scaleTestSession(nil), rows[:2]...), rows[3:]...))
	if err := requireScaleRemoval(before, wrong, rows[0].Username); err == nil {
		t.Fatal("same-SID sibling removal accepted")
	}
	changed := append([]scaleTestSession(nil), rows[1:]...)
	changed[0].Address = "10.100.0.99"
	if err := requireScaleRemoval(before, scaleTestPopulation(t, changed), rows[0].Username); err == nil {
		t.Fatal("survivor readdress accepted")
	}
}
