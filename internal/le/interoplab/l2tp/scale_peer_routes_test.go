// Design: docs/labs/l2tp-interop.md -- unbound kernel return-route evidence.
// Related: scale_peer_routes.go -- the LAC-only fixture and its readback boundary.
package l2tp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestScalePeerRouteReadback proves the unbound lookup selects this subscriber's
// interface, source, shared peer and dedicated table, rather than another session.
func TestScalePeerRouteReadback(t *testing.T) {
	const valid = `[{"dst":"10.100.0.1","from":"10.100.0.9","dev":"sc1s02","table":2002,"flags":[],"uid":0,"cache":[]}]`
	address := netip.MustParseAddr("10.100.0.9")
	if err := requireScalePeerRoute([]byte(valid), address, "sc1s02", 2002); err != nil {
		t.Fatalf("correct unbound subscriber route refused: %v", err)
	}
	for _, test := range []struct {
		name string
		raw  string
	}{
		{"wrong interface", `[{"dst":"10.100.0.1","from":"10.100.0.9","dev":"sc1s01","table":2002}]`},
		{"wrong source", `[{"dst":"10.100.0.1","from":"10.100.0.8","dev":"sc1s02","table":2002}]`},
		{"wrong destination", `[{"dst":"10.100.0.2","from":"10.100.0.9","dev":"sc1s02","table":2002}]`},
		{"wrong table", `[{"dst":"10.100.0.1","from":"10.100.0.9","dev":"sc1s02","table":2001}]`},
		{"unmeasured source", `[{"dst":"10.100.0.1","dev":"sc1s02","table":2002}]`},
		{"unmeasured table", `[{"dst":"10.100.0.1","from":"10.100.0.9","dev":"sc1s02"}]`},
		{"no route", `[]`},
		{"ambiguous routes", `[{"dst":"10.100.0.1","from":"10.100.0.9","dev":"sc1s02","table":2002},{"dst":"10.100.0.1","from":"10.100.0.9","dev":"sc1s01","table":2002}]`},
		{"invalid JSON", `[{`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := requireScalePeerRoute([]byte(test.raw), address, "sc1s02", 2002); err == nil {
				t.Fatal("incorrect or unavailable return-route evidence accepted")
			}
		})
	}
}

// TestScalePeerRouteTableEncoding accepts the two iproute2 decimal encodings
// and refuses fractional or missing table evidence without numeric truncation.
func TestScalePeerRouteTableEncoding(t *testing.T) {
	for _, test := range []struct {
		table string
		valid bool
	}{
		{`2002`, true}, {`"2002"`, true},
		{`2002.5`, false}, {`"2002.5"`, false},
		{`2.002e3`, false}, {`null`, false}, {`"main"`, false},
	} {
		t.Run(test.table, func(t *testing.T) {
			raw := fmt.Sprintf(`[{"dst":"10.100.0.1","from":"10.100.0.9","dev":"sc1s02","table":%s}]`, test.table)
			err := requireScalePeerRoute([]byte(raw), netip.MustParseAddr("10.100.0.9"), "sc1s02", 2002)
			if (err == nil) != test.valid {
				t.Fatalf("table %s accepted=%v, want %v: %v", test.table, err == nil, test.valid, err)
			}
		})
	}
}

type scalePeerRouteLab struct {
	recordingLab
	expected []recordedCall
	replies  []string
	failAt   int
}

func (l *scalePeerRouteLab) Exec(_ context.Context, peer string, args []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	index := len(l.calls)
	l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: append([]string(nil), args...)})
	if index >= len(l.expected) {
		return interoplab.CommandResult{}, errors.New("unexpected extra peer preparation call")
	}
	expected := l.expected[index]
	if peer != expected.peer {
		return interoplab.CommandResult{}, fmt.Errorf("peer %s, want %s", peer, expected.peer)
	}
	if !slices.Equal(args, expected.arguments) {
		return interoplab.CommandResult{}, fmt.Errorf("command %v, want %v", args, expected.arguments)
	}
	if index == l.failAt {
		return interoplab.CommandResult{}, errors.New("selected peer preparation failure")
	}
	return interoplab.CommandResult{Stdout: l.replies[index]}, nil
}

func scalePeerRouteFixture(t *testing.T) (scalePopulation, *scalePeerRouteLab) {
	t.Helper()
	rows := make([]scaleTestSession, 0, 48)
	lab := &scalePeerRouteLab{
		expected: make([]recordedCall, 0, 48*5),
		replies:  make([]string, 0, 48*5),
		failAt:   -1,
	}
	for ordinal := 1; ordinal <= 48; ordinal++ {
		lac, number := 1+(ordinal-1)/16, 1+(ordinal-1)%16
		peer := "kl2tpd-" + strconv.Itoa(lac)
		user := fmt.Sprintf("scale-%d-%02d", lac, number)
		name := fmt.Sprintf("sc%ds%02d", lac, number)
		address := "10.100.0." + strconv.Itoa(ordinal+1)
		table := strconv.Itoa(2000 + ordinal)
		rows = append(rows, scaleTestSession{uint16(lac), uint16(number), uint16(number), user, address, "established"})
		lookup := []string{"ip", "-j", "-4", "route", "get", "10.100.0.1", "from", address}
		for _, command := range [][]string{
			{"sh", "-c", scalePeerPPPIdentity, "peer-session", "/var/run/pppd/ppp-" + user + ".pid", name},
			lookup,
			{"ip", "-4", "route", "add", "10.100.0.1/32", "dev", name, "src", address, "table", table},
			{"ip", "-4", "rule", "add", "priority", table, "from", address + "/32", "lookup", table},
			lookup,
		} {
			lab.expected = append(lab.expected, recordedCall{operation: "exec", peer: peer, arguments: command})
		}
		lab.replies = append(lab.replies, "", `[{"dst":"10.100.0.1","dev":"sc1s01"}]`, "", "",
			fmt.Sprintf(`[{"dst":"10.100.0.1","from":%q,"dev":%q,"table":%s}]`, address, name, table))
	}
	return scaleTestPopulation(t, rows), lab
}

// TestScalePeerBaselinePreparation proves each of the 48 validated sources
// gets a unique table/rule, an unbound before/after lookup and read-only PID
// preflight. Silent successful route writes must not be mistaken for errors.
func TestScalePeerBaselinePreparation(t *testing.T) {
	population, lab := scalePeerRouteFixture(t)
	if err := prepareScalePeers(t.Context(), lab, population); err != nil {
		t.Fatal(err)
	}
	if len(lab.calls) != 48*5 {
		t.Fatalf("peer preparation made %d calls, want %d", len(lab.calls), 48*5)
	}
	if strings.Contains(scalePeerPPPIdentity, "kill") {
		t.Fatal("read-only baseline identity check can signal a process")
	}
	for _, check := range []string{
		`read -r pid < "$1" || exit 2`, `case "$pid" in ''|*[!0-9]*) exit 3;; esac`,
		`test "$(sed -n '2p' "$1")" = "$2" || exit 4`,
		`test "$(cat "/proc/$pid/comm")" = pppd || exit 5`,
	} {
		if !strings.Contains(scalePeerPPPIdentity, check) {
			t.Fatalf("exact PID validation lost %q", check)
		}
	}
}

// TestScalePeerPreparationRefusesFailure proves PID/read/write/readback
// failures stop baseline preparation instead of disappearing behind traffic.
func TestScalePeerPreparationRefusesFailure(t *testing.T) {
	for index := range 5 {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			population, lab := scalePeerRouteFixture(t)
			lab.failAt = index
			if err := prepareScalePeers(t.Context(), lab, population); err == nil {
				t.Fatal("peer preparation error accepted")
			}
			if len(lab.calls) != index+1 {
				t.Fatalf("preparation continued after failure: %d calls", len(lab.calls))
			}
		})
	}
	population, lab := scalePeerRouteFixture(t)
	lab.replies[4] = `[{"dst":"10.100.0.1","from":"10.100.0.2","dev":"sc1s02","table":2001}]`
	if err := prepareScalePeers(t.Context(), lab, population); err == nil {
		t.Fatal("wrong actual unbound egress accepted after setup")
	}
	if len(lab.calls) != 5 {
		t.Fatal("preparation continued after incorrect readback")
	}
}
