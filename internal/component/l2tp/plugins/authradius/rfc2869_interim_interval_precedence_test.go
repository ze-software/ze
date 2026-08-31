// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- RADIUS attribute metadata store
// Related: acct.go -- acctInterval, onSessionIPAssigned, interimLoop
//
// VALIDATES: that the acct-interval config leaf beats the Acct-Interim-Interval
// attribute of an Access-Accept. An unset leaf leaves the attribute in charge.
// PREVENTS: a RADIUS server that retimes a deployment in silence. Ze took the
// Access-Accept value whenever it was present. A server shortened an operator's
// interim cadence, and the operator had no way to overrule it. RFC 2869 Section
// 2.1 puts the NAS in charge.
//
// The two cases read the wire, not a field. They fail if onSessionIPAssigned
// stops passing what acctInterval answered down to the ticker. The cadences are
// milliseconds because a test cannot wait an hour. The config layer refuses
// anything under 60 seconds (config.go). TestAcctIntervalPrecedence covers the
// arithmetic case by case.

package l2tpauthradius

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
)

// interimsSeen answers how many Interim-Update records the capture holds.
func interimsSeen(capture *acctCapture) int {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	seen := 0
	for _, pkt := range capture.packets {
		if pkt.statusType == radius.AcctStatusInterimUpdate {
			seen++
		}
	}
	return seen
}

// waitForInterims answers how many Interim-Update records the capture holds. It
// returns once the capture holds want of them, or once within has passed.
func waitForInterims(capture *acctCapture, want int, within time.Duration) int {
	deadline := time.Now().Add(within)
	for {
		seen := interimsSeen(capture)
		if seen >= want {
			return seen
		}
		if time.Now().After(deadline) {
			return seen
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// startPrecedenceSession wires an accounting client to a capturing server. It
// records an Access-Accept that asked for fromAccept seconds, then starts the
// session. configured is the acct-interval leaf, and zero stands for an unset
// one.
func startPrecedenceSession(t *testing.T, tunnelID, sessionID uint16, configured time.Duration, fromAccept uint32) *acctCapture {
	t.Helper()
	sharedKey := []byte("precedence")
	capture := newAcctCapture()
	conn, addr := startAcctServer(t, sharedKey, capture)
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
		Timeout: 2 * time.Second,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() }) //nolint:errcheck // test cleanup

	l2tp.StoreSessionMetadata(tunnelID, sessionID, &l2tp.AuthMetadata{AcctInterimInterval: fromAccept})
	t.Cleanup(func() { l2tp.ClearSessionMetadata(tunnelID, sessionID) })

	acct := newRADIUSAcct()
	acct.setClient(client, "test-nas", configured, addr, nil, "")
	t.Cleanup(acct.Stop)

	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		TunnelID:  tunnelID,
		SessionID: sessionID,
		Username:  "alice",
		PeerAddr:  "10.0.0.1",
	})
	return capture
}

// TestRFC2869LocalAcctIntervalOverridesAccessAccept configures a local interval.
// It answers the session's Access-Accept with the longest interval the clamp
// admits. Interim records that keep arriving are the local value in force.
//
// RFC 2869 Section 2.1: "It is also possible to statically configure an interim
// value on the NAS itself. Note that a locally configured value on the NAS MUST
// override the value found in an Access-Accept."
//
// RFC requirement: RFC2869-2.1-2 positive -- with acct-interval set, the session
// accounts at the configured cadence. The Acct-Interim-Interval attribute of its
// Access-Accept changes nothing (acctInterval, acct.go).
func TestRFC2869LocalAcctIntervalOverridesAccessAccept(t *testing.T) {
	// 3600 is the clamp ceiling. A server that wins here buys an hour of
	// silence, so the count below is zero.
	capture := startPrecedenceSession(t, 71, 9, 40*time.Millisecond, 3600)

	const wantInterims = 3
	seen := waitForInterims(capture, wantInterims, 2*time.Second)
	if seen < wantInterims {
		t.Fatalf("the server saw %d interim record(s) in 2s; the session was configured for a "+
			"40ms interval and its Access-Accept asked for 3600s, so the Access-Accept won and "+
			"RFC 2869 Section 2.1 is broken", seen)
	}
}

// TestRFC2869AbsentAcctIntervalLeavesTheAccessAcceptInCharge unsets the leaf. It
// answers the same Access-Accept. Silence is the server's hour honored.
//
// This pole keeps a NAS that ignores the Access-Accept outright from satisfying
// the case above. The override is what a locally configured value does, and
// there is nothing to override with here.
//
// RFC requirement: RFC2869-2.1-2 negative -- with acct-interval unset, no local
// value overrides anything. The session runs at the Acct-Interim-Interval its
// Access-Accept carried, and sends no interim record inside it (acctInterval,
// acct.go).
func TestRFC2869AbsentAcctIntervalLeavesTheAccessAcceptInCharge(t *testing.T) {
	capture := startPrecedenceSession(t, 72, 9, 0, 3600)

	// The Accounting-Start is owed at once. The wait below is therefore not a
	// wait on a dead session.
	capture.waitN(t, 1)

	seen := waitForInterims(capture, 1, 500*time.Millisecond)
	if seen != 0 {
		t.Fatalf("the server saw %d interim record(s) in 500ms; the Access-Accept asked for "+
			"3600s and no acct-interval was configured, so the session invented a cadence of "+
			"its own", seen)
	}
}

// TestAcctIntervalPrecedence states the arithmetic the two wire cases cannot
// separate. An hour and five minutes both read as silence inside a test.
func TestAcctIntervalPrecedence(t *testing.T) {
	cases := []struct {
		name       string
		configured time.Duration
		fromAccept uint32
		want       time.Duration
	}{
		{"configured alone", 120 * time.Second, 0, 120 * time.Second},
		{"configured beats the Access-Accept", 120 * time.Second, 900, 120 * time.Second},
		{"the Access-Accept decides when nothing is configured", 0, 900, 900 * time.Second},
		{"the Access-Accept is clamped up to the floor", 0, 30, 60 * time.Second},
		{"the Access-Accept is clamped down to the ceiling", 0, 7200, 3600 * time.Second},
		{"neither side states one", 0, 0, acctIntervalDefault},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := acctInterval(tc.configured, tc.fromAccept); got != tc.want {
				t.Fatalf("acctInterval(%v, %d) = %v, want %v", tc.configured, tc.fromAccept, got, tc.want)
			}
		})
	}
}

// TestAcctIntervalNeverAnswersZero pins the property the dropped YANG default
// carried. An absent leaf must not read as an interval of zero. Zero panics
// time.NewTicker, and it stops interim accounting outright.
func TestAcctIntervalNeverAnswersZero(t *testing.T) {
	if got := acctInterval(0, 0); got <= 0 {
		t.Fatalf("acctInterval(0, 0) = %v, want a positive interval", got)
	}
}
