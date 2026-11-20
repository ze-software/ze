// Design: docs/architecture/testing/interop.md -- the FreeRADIUS subscriber suite, radius-subscriber-framed-mtu
// Related: check_framed_mtu.go -- checkFramedMTU, judgeKernelMTU, judgeSession
//
// The tests run the registered checker against a lab that answers each probe
// with fixed text, so the verdict for the Framed-MTU applied and for it
// ignored is fixed. No container starts.

package radiussubscriber

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// framedMTULab answers the four probes checkFramedMTU makes: the server's
// detail file, ze's PPP links, ze's sessions and the LAC's ping, and the
// peer logs a failure carries. Any other call reaches the nil embedded
// interface and panics the test.
type framedMTULab struct {
	interoplab.CheckerLab
	links    string
	sessions string
	measured bool
}

func (l *framedMTULab) Query(_ context.Context, peer string, args []string, _ []interoplab.EnvironmentVariable) (string, error) {
	command := strings.Join(args, " ")
	switch {
	case peer == peerServer && command == catCommand+" "+detailPath:
		return "Fri Nov 20 04:00:00 2026\n\tAcct-Status-Type = Start\n\tAcct-Session-Id = \"1-2-0123456789abcdef01234567-1\"\n", nil
	case peer == peerZe && command == "ip -j link show type ppp":
		l.measured = true
		return l.links, nil
	case peer == peerZe && strings.Contains(command, "show l2tp sessions"):
		return l.sessions, nil
	}
	return "", fmt.Errorf("unexpected query to %s: %s", peer, command)
}

func (l *framedMTULab) Exec(_ context.Context, peer string, args []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	if peer == peerLAC && len(args) > 0 && args[0] == "ping" {
		return interoplab.CommandResult{}, nil
	}
	return interoplab.CommandResult{}, fmt.Errorf("unexpected exec on %s: %v", peer, args)
}

// Logs answers the diagnostics checker appends to a failure.
func (l *framedMTULab) Logs(context.Context, string, int) (interoplab.LogResult, error) {
	return interoplab.LogResult{Available: true}, nil
}

func framedMTULinks(mtu int) string {
	return fmt.Sprintf(`[{"ifindex":3,"ifname":"ppp0","flags":["POINTOPOINT","UP"],"mtu":%d}]`, mtu)
}

func framedMTUSessions(mru, mtu string) string {
	return `{"status":"done","data":[{"local-sid":7,"username":"alice","mru":` + mru + `,"mtu":` + mtu + `}]}`
}

// VALIDATES: the registered checker passes only when ze's kernel pppN and
// `show l2tp sessions` both carry the Framed-MTU 1400 beside the LCP mru 1500.
// PREVENTS: a checker that passes with the Framed-MTU ignored (1500 on both),
// with only one of the two observations right, or with the 1400 explained by
// LCP rather than RADIUS (mru 1400).
func TestFramedMTUCheckerDiscriminates(t *testing.T) {
	for _, test := range []struct {
		name     string
		links    string
		sessions string
		valid    bool
	}{
		{"applied", framedMTULinks(1400), framedMTUSessions("1500", "1400"), true},
		{"ignored", framedMTULinks(1500), framedMTUSessions("1500", "1500"), false},
		{"kernel ignored", framedMTULinks(1500), framedMTUSessions("1500", "1400"), false},
		{"report ignored", framedMTULinks(1400), framedMTUSessions("1500", "1500"), false},
		{"lcp explains it", framedMTULinks(1400), framedMTUSessions("1400", "1400"), false},
		{"lcp not open", framedMTULinks(1400), framedMTUSessions("null", "1400"), false},
		{"two interfaces", `[{"ifname":"ppp0","mtu":1400},{"ifname":"ppp1","mtu":1400}]`, framedMTUSessions("1500", "1400"), false},
		{"refused command", framedMTULinks(1400), `{"status":"error","error":"unknown command"}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			check := scenarioCheckerMap(time.Second)[scenarioFramedMTU]
			if check == nil {
				t.Fatal("framed-MTU checker is not registered")
			}
			lab := &framedMTULab{links: test.links, sessions: test.sessions}
			err := check(t.Context(), &interoplab.CheckContext{Lab: lab})
			if (err == nil) != test.valid {
				t.Fatalf("checker error=%v, want valid=%v", err, test.valid)
			}
			if !lab.measured {
				t.Fatal("checker never read ze's kernel interface")
			}
		})
	}
}

// VALIDATES: judgeKernelMTU refuses output that is not iproute2's JSON array,
// an empty array and a nameless entry, rather than reading any as an MTU.
// PREVENTS: an unreadable kernel answer judged as a measurement.
func TestJudgeKernelMTURefusesUnmeasured(t *testing.T) {
	if err := judgeKernelMTU(framedMTULinks(1400), 1400); err != nil {
		t.Fatalf("refused the measured MTU: %v", err)
	}
	for _, output := range []string{"", "broken", `[]`, `[{"mtu":1400}]`, `{"ifname":"ppp0","mtu":1400}`} {
		if err := judgeKernelMTU(output, 1400); err == nil {
			t.Errorf("accepted unmeasured kernel answer %q", output)
		}
	}
}

// VALIDATES: parseSessions refuses a refused command and an answer it cannot
// decode, and judgeSession refuses a null mtu and another user.
// PREVENTS: an error answer read as no sessions, or a session before its
// network lifetime read as the Framed-MTU missing.
func TestParseAndJudgeSessions(t *testing.T) {
	sessions, err := parseSessions(framedMTUSessions("1500", "1400"))
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions %+v, error %v", sessions, err)
	}
	if err := judgeSession(sessions[0], 1400, 1500); err != nil {
		t.Fatalf("refused the applied session: %v", err)
	}
	for _, answer := range []string{"", "broken", `{"status":"error","error":"x"}`, `{"status":"done","data":{}}`} {
		if _, err := parseSessions(answer); err == nil {
			t.Errorf("accepted answer %q", answer)
		}
	}
	for _, answer := range []string{
		framedMTUSessions("1500", "null"),
		strings.Replace(framedMTUSessions("1500", "1400"), "alice", "bob", 1),
	} {
		sessions, err := parseSessions(answer)
		if err != nil || len(sessions) != 1 {
			t.Fatalf("sessions %+v, error %v", sessions, err)
		}
		if err := judgeSession(sessions[0], 1400, 1500); err == nil {
			t.Errorf("accepted session %q", answer)
		}
	}
}
