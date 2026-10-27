// Design: docs/contributing/terminal-demos.md -- current-cycle, non-vacuous failure proof.
// Related: validate_policy_evidence.go -- the actual live-check evidence predicates.

package siteterminaldemo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/peer"
)

// TestRPKICycleEvidence rejects historical outage/recovery pairs and timestamps
// indistinguishable at the daemon command's whole-second precision.
// VALIDATES: only a new failed poll and a sync after that failure prove recovery.
// PREVENTS: retained healthy VRPs or old log entries satisfying the current outage.
func TestRPKICycleEvidence(t *testing.T) {
	boundary := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	failure := boundary.Add(time.Second)
	oldPair := []policyLogEntry{
		{Timestamp: boundary.Add(-time.Second), Level: "info", Message: rpkiCompletedSync},
		{Timestamp: boundary.Add(-2 * time.Second), Level: "warn", Message: rpkiFailedPoll},
	}
	tests := []struct {
		name      string
		cycle     rpkiOutage
		entries   []policyLogEntry
		recovered bool
		want      bool
	}{
		{name: "historical failure", cycle: rpkiOutage{Boundary: boundary}, entries: oldPair},
		{name: "historical recovery without current failure", cycle: rpkiOutage{Boundary: boundary}, entries: oldPair, recovered: true},
		{name: "historical sync after a new failure", cycle: rpkiOutage{Boundary: boundary, Failure: failure}, entries: oldPair, recovered: true},
		{name: "healthy cache without sync", cycle: rpkiOutage{Boundary: boundary, Failure: failure}, recovered: true},
		{name: "missing boundary", entries: []policyLogEntry{{Timestamp: failure, Level: "warn", Message: rpkiFailedPoll}}},
		{name: "equal boundary", cycle: rpkiOutage{Boundary: boundary}, entries: []policyLogEntry{{Timestamp: boundary, Level: "warn", Message: rpkiFailedPoll}}},
		{name: "new warning", cycle: rpkiOutage{Boundary: boundary}, entries: []policyLogEntry{{Timestamp: failure, Level: "warn", Message: rpkiFailedPoll}}, want: true},
		{name: "wrong severity", cycle: rpkiOutage{Boundary: boundary}, entries: []policyLogEntry{{Timestamp: failure, Level: "info", Message: rpkiFailedPoll}}},
		{name: "equal failure", cycle: rpkiOutage{Boundary: boundary, Failure: failure}, entries: []policyLogEntry{{Timestamp: failure, Level: "info", Message: rpkiCompletedSync}}, recovered: true},
		{name: "new sync without current failure", cycle: rpkiOutage{Boundary: boundary}, entries: []policyLogEntry{{Timestamp: failure.Add(time.Second), Level: "info", Message: rpkiCompletedSync}}, recovered: true},
		{name: "new sync after current failure", cycle: rpkiOutage{Boundary: boundary, Failure: failure}, entries: []policyLogEntry{{Timestamp: failure.Add(time.Second), Level: "info", Message: rpkiCompletedSync}}, recovered: true, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := rpkiCycleEvent(test.entries, test.cycle, test.recovered)
			if (err == nil) != test.want {
				t.Fatalf("current-cycle evidence error = %v, want success %t", err, test.want)
			}
		})
	}
}

// TestIRRRequiresCompletedAnnouncements uses the live RIB predicate itself.
// VALIDATES: each policy has a final accepted marker and excludes the other marker.
// PREVENTS: an empty or partially received RIB being mistaken for a proven refusal,
// or a retained previous-cycle marker certifying the new customer session.
func TestIRRRequiresCompletedAnnouncements(t *testing.T) {
	const wanted = "ipv4/unicast:10.0.0.0/24"
	const outside = "ipv4/unicast:192.168.0.0/24"
	tests := []struct {
		name     string
		keys     []string
		accepted bool
		want     bool
	}{
		{name: "healthy complete", keys: []string{wanted, irrHealthyControl}, accepted: true, want: true},
		{name: "wrong complete", keys: []string{irrWrongControl}, want: true},
		{name: "empty wrong set"},
		{name: "healthy first update only", keys: []string{wanted}, accepted: true},
		{name: "wrong old control only", keys: []string{irrHealthyControl}},
		{name: "wrong both controls", keys: []string{irrWrongControl, irrHealthyControl}},
		{name: "restored retains wrong control", keys: []string{wanted, irrHealthyControl, irrWrongControl}, accepted: true},
		{name: "healthy missing accepted route", keys: []string{irrHealthyControl}, accepted: true},
		{name: "wrong admits original route", keys: []string{wanted, irrWrongControl}},
		{name: "healthy admits outside", keys: []string{wanted, outside, irrHealthyControl}, accepted: true},
		{name: "wrong admits outside", keys: []string{outside, irrWrongControl}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			routes := make([]rpkiDemoRoute, 0, len(test.keys))
			for _, key := range test.keys {
				routes = append(routes, rpkiDemoRoute{Key: key})
			}
			document, err := json.Marshal(map[string]any{"adj-rib-in": map[string][]rpkiDemoRoute{"127.0.0.4": routes}})
			if err != nil {
				t.Fatal(err)
			}
			err = requireIRRRoutes(string(document), test.accepted)
			if (err == nil) != test.want {
				t.Fatalf("announcement completion error = %v, want success %t", err, test.want)
			}
		})
	}
}

// TestTransitConnectionEvidence observes fixture accepts, not replaceable Peer counters.
// VALIDATES: one real connection over the chapter's lifetime.
// PREVENTS: a recreated Peer with a fresh counter of one hiding a reconnect.
func TestTransitConnectionEvidence(t *testing.T) {
	const first = "listening on 127.0.0.2:1179\n\nnew connection from 127.0.0.1:30001\n"
	for _, test := range []struct {
		name, log string
		want      bool
	}{
		{name: "none", log: "listening on 127.0.0.2:1179\n"},
		{name: "one", log: first, want: true},
		{name: "replacement", log: first + "new connection from 127.0.0.1:30002\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := checkTransitConnectionLog(test.log); (err == nil) != test.want {
				t.Fatalf("fixture connection error = %v, want success %t", err, test.want)
			}
		})
	}
}

// TestIRRControlIsLastOnTheFixtureSendPath parses the actual lab fixture with
// the same parser Peer.runMessageLoop consumes in SendRoutes order.
// VALIDATES: both excluded routes and the opposite control precede the accepted control.
// PREVENTS: one fixed control order passing a policy before its last exclusion was processed.
func TestIRRControlIsLastOnTheFixtureSendPath(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoDemoRoot, showcaseLabID, "customer.msg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ asSet, final string }{
		{asSet: "AS-TEST", final: "10.0.1.0/24"},
		{asSet: "AS-V4ONLY", final: "192.0.2.0/24"},
	} {
		t.Run(test.asSet, func(t *testing.T) {
			ordered, err := orderIRRAnnouncements(string(data), test.asSet)
			if err != nil {
				t.Fatal(err)
			}
			_, parsed, err := peer.ParseExpect(strings.NewReader(ordered))
			if err != nil {
				t.Fatal(err)
			}
			if len(parsed.SendRoutes) == 0 {
				t.Fatal("customer fixture sends no routes")
			}
			if got := parsed.SendRoutes[len(parsed.SendRoutes)-1].Prefix; got != test.final {
				t.Fatalf("last wire announcement = %s, want accepted control %s", got, test.final)
			}
			for _, prefix := range []string{"10.0.0.0/24", "192.168.0.0/24", "10.0.1.0/24", "192.0.2.0/24"} {
				count := 0
				for _, route := range parsed.SendRoutes {
					if route.Prefix == prefix {
						count++
					}
				}
				if count != 1 {
					t.Fatalf("fixture sends %s %d times, want exactly once", prefix, count)
				}
			}
			for _, missing := range []string{"10.0.0.0/24", "192.168.0.0/24", test.final} {
				var incomplete strings.Builder
				for line := range strings.SplitSeq(string(data), "\n") {
					if !strings.Contains(line, ":prefix="+missing+":") {
						incomplete.WriteString(line)
						incomplete.WriteByte('\n')
					}
				}
				if _, err := orderIRRAnnouncements(incomplete.String(), test.asSet); err == nil {
					t.Fatalf("missing %s still allowed a completion proof", missing)
				}
			}
		})
	}
}
