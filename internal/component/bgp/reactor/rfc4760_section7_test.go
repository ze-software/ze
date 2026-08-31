// RFC: rfc/short/rfc4760.md — multiprotocol extensions, Section 7 error handling
// RFC: rfc/short/rfc7606.md — revised UPDATE error handling, Section 3 clause (j)
// Overview: session_validation.go — enforceRFC7606 and rfc7606SessionReset
// Related: rfc7606_session_structural_test.go — the same harness for the §3 rules
//
// RFC 4760 Section 7 states the obligation this file pins, quoted here as the RFC has it:
// "If a BGP speaker receives from a neighbor an UPDATE message that
// contains the MP_REACH_NLRI or MP_UNREACH_NLRI attribute, and if the speaker determines
// that the attribute is incorrect, the speaker MUST delete all the BGP routes received
// from that neighbor whose AFI/SAFI is the same as the one carried in the incorrect
// MP_REACH_NLRI or MP_UNREACH_NLRI attribute."
//
// RFC 7606 does not retire that obligation. Its Section 3 clause (j) covers an MP
// attribute that cannot be parsed: "the procedures of [RFC4271] and/or [RFC4760] continue
// to apply, meaning that the 'session reset' approach (or the 'AFI/SAFI disable' approach)
// MUST be followed". Ze follows session reset. A reset drops every route from the peer,
// which is a superset of the incorrect attribute's AFI/SAFI.
//
// The requirement is pinned where the reset happens, on a real session. The NOTIFICATION
// goes out, the connection closes, and the UPDATE reaches no plugin.

package reactor

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// mpAttr wraps an MP_REACH_NLRI (code 14) or MP_UNREACH_NLRI (code 15) value in its
// attribute header. RFC 4760 defines both as optional non-transitive. The flags octet is
// therefore 0x80, which does not trip RFC 7606 Section 5.3's flags rule. The only defect
// in each fixture below is the one its test names.
func mpAttr(code byte, value []byte) []byte {
	attr := make([]byte, 0, 3+len(value))
	attr = append(attr, 0x80, code, byte(len(value)))
	return append(attr, value...)
}

// requireSessionResetOnWire drives one UPDATE body into an Established eBGP session. It
// requires RFC 4760 Section 7's outcome. The session is torn down, so every route from
// the peer goes with it, and the peer is told why.
func requireSessionResetOnWire(t *testing.T, update []byte) {
	t.Helper()

	session, client, callbackCount, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	var received []byte
	done := make(chan struct{})
	go func() {
		client.Write(buildUpdateMsg(update)) //nolint:errcheck // test goroutine
		buf := make([]byte, 4096)
		n, _ := client.Read(buf) //nolint:errcheck // read NOTIFICATION
		received = buf[:n]
		close(done)
	}()

	err := session.ReadAndProcess()
	require.Error(t, err, "an incorrect MP attribute must reset the session")
	require.Contains(t, err.Error(), "session reset")
	require.Equal(t, fsm.StateIdle, session.State(),
		"Idle is what deletes the peer's routes. The session, and every route it carried, is gone")
	require.Equal(t, 0, *callbackCount,
		"the UPDATE must not reach plugins: its NLRI boundaries cannot be trusted")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for NOTIFICATION")
	}

	require.GreaterOrEqual(t, len(received), message.HeaderLen+2, "NOTIFICATION too short")
	hdr, hdrErr := message.ParseHeader(received[:message.HeaderLen])
	require.NoError(t, hdrErr)
	require.Equal(t, msgtype.TypeNOTIFICATION, hdr.Type,
		"RFC 7606 Section 3(a): a session reset is indicated by a NOTIFICATION")
	require.Equal(t, byte(message.NotifyUpdateMessage), received[message.HeaderLen],
		"NOTIFICATION error code must be 3 (UPDATE Message Error)")
}

// TestRFC4760IncorrectMPReachDeletesTheNeighborsRoutes drives an MP_REACH_NLRI whose last
// NLRI overruns the attribute.
//
// VALIDATES: an MP_REACH_NLRI that cannot be parsed takes the session down. That removes
// every route the peer contributed, IPv4 unicast among them.
// PREVENTS: the UPDATE being treated as withdraw, or accepted, when the attribute's NLRI
// boundaries are unknown. Either outcome leaves the peer's routes for that AFI/SAFI
// installed, which is what Section 7 forbids.
//
// The prefix length is 24, inside the family maximum of 32. The "greater than 32" rule
// cannot be what fires, so the only defect is the overrun. An overrun is what makes the
// attribute unparseable, and clause (j) routes that to session reset rather than to
// treat-as-withdraw.
//
// RFC requirement: RFC4760-7-1 positive -- an incorrect MP_REACH_NLRI deletes all routes
// received from that peer for the attribute's AFI/SAFI. ValidateNLRISyntaxAddPath
// (message/rfc7606.go) returns RFC7606ActionSessionReset for the overrun. It is reached
// for the MP attribute's own NLRI through validateMPNLRIField. rfc7606SessionReset
// (session_validation.go) then performs the reset, so the session reaches Idle and carries
// the peer's IPv4 unicast routes out with it.
func TestRFC4760IncorrectMPReachDeletesTheNeighborsRoutes(t *testing.T) {
	mpReach := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01,                   // SAFI = 1 (Unicast)
		0x04,                   // Next-hop length = 4
		0xc0, 0x00, 0x02, 0x01, // Next hop = 192.0.2.1
		0x00,             // Reserved
		0x18, 0x0a, 0x00, // NLRI: /24 needs three octets, two follow — it overruns
	}
	attrs := append(append([]byte{}, validPathAttrs...), mpAttr(14, mpReach)...)

	update := make([]byte, 0, 4+len(attrs))
	update = append(update, 0x00, 0x00, byte(len(attrs)>>8), byte(len(attrs)))
	update = append(update, attrs...)

	requireSessionResetOnWire(t, update)
}

// TestRFC4760IncorrectMPUnreachDeletesTheNeighborsRoutes is the MP_UNREACH_NLRI half.
//
// VALIDATES: Section 7 names both attributes, and the withdrawal attribute gets the same
// treatment as the reachability one.
// PREVENTS: the check being wired only on MP_REACH_NLRI, which the MP_REACH test alone
// would never reveal.
//
// The UPDATE carries reachable IPv4 NLRI on purpose. RFC 7606 Section 5.2 escalates a
// treat-as-withdraw to session reset when an UPDATE has attributes and NO reachable NLRI.
// An MP_UNREACH-only body meets that description. Without the NLRI this test stays green
// on an implementation that downgraded the clause (j) verdict. It then proves nothing
// about the MP attribute. With the NLRI, only the overrun can reset.
//
// RFC requirement: RFC4760-7-1 positive -- an incorrect MP_UNREACH_NLRI reaches the same
// outcome through the same producers. ValidateNLRISyntaxAddPath (message/rfc7606.go)
// returns RFC7606ActionSessionReset for the overrun, and the session is reset rather than
// the UPDATE processed.
func TestRFC4760IncorrectMPUnreachDeletesTheNeighborsRoutes(t *testing.T) {
	mpUnreach := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01,             // SAFI = 1 (Unicast)
		0x18, 0x0a, 0x00, // Withdrawal: /24 needs three octets, two follow — it overruns
	}
	attrs := append(append([]byte{}, validPathAttrs...), mpAttr(15, mpUnreach)...)

	update := make([]byte, 0, 4+len(attrs)+2)
	update = append(update, 0x00, 0x00, byte(len(attrs)>>8), byte(len(attrs)))
	update = append(update, attrs...)
	update = append(update, 0x08, 0x0a) // NLRI: 10.0.0.0/8, so Section 5.2 cannot escalate

	requireSessionResetOnWire(t, update)
}

// TestRFC4760CorrectMPReachKeepsTheNeighborsRoutes pins the conforming side.
//
// VALIDATES: the deletion is conditional on the attribute being INCORRECT. A well-formed
// MP_REACH_NLRI for the same AFI/SAFI leaves the session Established and reaches the
// plugins, so the peer's routes stay.
// PREVENTS: satisfying Section 7 by resetting on every MP attribute, which both positives
// above would pass. Section 7 is a rule about incorrect attributes. An implementation that
// deletes on correct ones has replaced multiprotocol BGP with a hangup.
//
// The absence of a NOTIFICATION is asserted, not assumed. net.Pipe is unbuffered, so a
// deadline that expires is proof nothing was written.
//
// RFC requirement: RFC4760-7-1 negative -- a correct MP_REACH_NLRI is not an incorrect
// one. enforceRFC7606 (session_validation.go) returns RFC7606ActionNone, no session reset
// runs, and the peer's routes for that AFI/SAFI survive.
func TestRFC4760CorrectMPReachKeepsTheNeighborsRoutes(t *testing.T) {
	session, client, callbackCount, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	mpReach := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01,                   // SAFI = 1 (Unicast)
		0x04,                   // Next-hop length = 4
		0xc0, 0x00, 0x02, 0x01, // Next hop = 192.0.2.1
		0x00,                   // Reserved
		0x18, 0x0a, 0x00, 0x00, // NLRI: 10.0.0.0/24, three octets present
	}
	attrs := append(append([]byte{}, validPathAttrs...), mpAttr(14, mpReach)...)

	update := make([]byte, 0, 4+len(attrs))
	update = append(update, 0x00, 0x00, byte(len(attrs)>>8), byte(len(attrs)))
	update = append(update, attrs...)

	written := make(chan struct{})
	go func() {
		client.Write(buildUpdateMsg(update)) //nolint:errcheck // test goroutine
		close(written)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "a correct MP_REACH_NLRI must not reset the session")
	require.Equal(t, fsm.StateEstablished, session.State(),
		"the neighbor's routes stay because the session stays")
	require.Equal(t, 1, *callbackCount, "the valid UPDATE must reach the plugins")

	select {
	case <-written:
	case <-time.After(time.Second):
		t.Fatal("timeout writing UPDATE")
	}

	require.NoError(t, client.SetReadDeadline(time.Now().Add(200*time.Millisecond)))
	buf := make([]byte, 4096)
	n, readErr := client.Read(buf)
	require.ErrorIs(t, readErr, os.ErrDeadlineExceeded,
		"nothing is wrong with this UPDATE, so the read must expire with nothing to report. "+
			"Got %d byte(s): %x", n, buf[:max(n, 0)])
}
