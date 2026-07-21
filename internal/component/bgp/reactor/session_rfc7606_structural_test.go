// RFC: rfc/short/rfc7606.md — revised UPDATE error handling
// Overview: session_validation.go — enforceRFC7606, the §3 call site
//
// RFC 7606 requirements that can only be pinned where the UPDATE meets a real session:
// §3.a's NOTIFICATION obligation, §3.b's structural length conflict, §3.i's Withdrawn
// Routes check, and §2's requirement that a valid UPDATE still announces.
//
// The conforming cases here are the half that message-level tests cannot supply. A rule
// tested only on violations passes on an implementation that rejects everything, so each
// negative already in the suite needs a partner showing the valid UPDATE still gets
// through untouched.

package reactor

import (
	"encoding/binary"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/core/bgp/context"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// validPathAttrs is a well-formed eBGP attribute set: ORIGIN, AS_PATH and NEXT_HOP all
// present, correctly flagged, correctly sized. Nothing in it may trip any RFC 7606 rule.
//
// The AS_PATH is EMPTY, and deliberately so: this fixture is shared by two harnesses that
// negotiate differently. newValidateSession sets no capabilities, so Negotiated() is nil
// and validation runs with asn4=false (2-octet ASNs); setupEstablishedSessionEBGP
// negotiates capability.ASN4, so asn4=true (4-octet ASNs). A populated segment can only
// satisfy one of them — under the other, the count=1 segment leaves trailing octets that
// parse as a second segment with an unrecognized type, tripping §7.2 and turning an
// "everything valid" fixture into a treat-as-withdraw. That would silently invalidate
// every test built on it.
//
// A zero-length AS_PATH is valid under both, and legitimately so: §4 names AS_PATH as one
// of only two attributes that may validly have an attribute length of zero.
var validPathAttrs = []byte{
	0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
	0x40, 0x02, 0x00, // AS_PATH = empty (valid at any ASN width; RFC 7606 §4)
	0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
}

// =============================================================================
// RFC 7606 Section 3 (i) — the Withdrawn Routes field is checked like the NLRI field
// =============================================================================

// TestEnforceRFC7606_ValidWithdrawnNLRIAccepted pins the conforming side of §3.i.
//
// VALIDATES: a syntactically correct, NON-EMPTY Withdrawn Routes field is accepted.
// PREVENTS: the §3.i check rejecting every withdrawal — which is what a negative-only
// suite would let through unnoticed.
//
// Driven through enforceRFC7606, the §3.i call site (session_validation.go:52), rather
// than ValidateNLRISyntax alone: the requirement is that the check is APPLIED to the
// withdrawn field, so the test has to enter where that wiring lives. It pairs
// TestEnforceRFC7606_InvalidWithdrawnNLRI, which drives length 33 through the same door.
//
// The field is non-empty on purpose. ValidateNLRISyntax returns nil early for an empty
// field (rfc7606.go:833), so a withdrawal-only UPDATE with no routes would pass without
// the syntax check ever running, and would prove nothing about §3.i.
//
// RFC requirement: RFC7606-3.i-1 positive — a syntactically correct Withdrawn Routes field is accepted, exactly as a correct NLRI field would be.
func TestEnforceRFC7606_ValidWithdrawnNLRIAccepted(t *testing.T) {
	s := newValidateSession()

	// Three withdrawals spanning the syntax rules §3.i imports from §5.3: the shortest
	// legal prefix, an ordinary one, and the family maximum. The last one ends flush with
	// the field, so the overrun rule is exercised at its boundary too.
	withdrawn := []byte{
		0,             // 0.0.0.0/0
		24, 192, 0, 2, // 192.0.2.0/24
		32, 10, 0, 0, 1, // 10.0.0.1/32
	}
	body := makeUpdateBody(withdrawn, nil, nil)
	wu := wireu.NewWireUpdate(body, 0)

	newWU, action, err := s.enforceRFC7606(wu)
	require.NoError(t, err, "a correct Withdrawn Routes field must not reset the session")
	assert.Equal(t, message.RFC7606ActionNone, action,
		"nothing in this UPDATE is malformed, so no error handling applies")
	assert.NotNil(t, newWU)
	assert.Equal(t, body, newWU.Payload(), "a valid UPDATE must pass through unrewritten")
}

// =============================================================================
// RFC 7606 Section 3 (b) — Withdrawn Routes Length + Total Attribute Length + 23
// =============================================================================

// TestEnforceRFC7606_SectionLengthsExactlyFitAccepted pins the conforming side of §3.b.
//
// VALIDATES: an UPDATE whose section lengths sum to EXACTLY the message length is accepted.
// PREVENTS: an off-by-one in the §3.b arithmetic rejecting every well-formed UPDATE that
// happens to end flush.
//
// §3.b triggers when the sum EXCEEDS the message length; equality is the conforming edge
// and must not fire. The body below is built so that
//
//	2 + WithdrawnLen + 2 + TotalAttrLen + len(NLRI) == len(body)
//
// with no slack, which is the same relation as §3.b's "+ 23" once the 19-octet header and
// the two 2-octet length fields are accounted for. The arithmetic is asserted explicitly
// below rather than trusted, so the test cannot silently stop testing the boundary.
//
// RFC requirement: RFC7606-3.b-1 positive — section lengths that sum exactly to the message length are not a conflict.
func TestEnforceRFC7606_SectionLengthsExactlyFitAccepted(t *testing.T) {
	s := newValidateSession()

	withdrawn := []byte{24, 192, 0, 2} // 192.0.2.0/24
	nlri := []byte{0x08, 0x0a}         // 10.0.0.0/8
	body := makeUpdateBody(withdrawn, validPathAttrs, nlri)

	// Pin the boundary this test claims to sit on: every octet is accounted for, so the
	// sum equals the message length rather than merely fitting inside it.
	require.Equal(t, 2+len(withdrawn)+2+len(validPathAttrs)+len(nlri), len(body),
		"the sections must exactly exhaust the body for this to be the §3.b boundary")
	require.Equal(t, uint16(len(withdrawn)), binary.BigEndian.Uint16(body[0:2]))
	require.Equal(t, uint16(len(validPathAttrs)),
		binary.BigEndian.Uint16(body[2+len(withdrawn):4+len(withdrawn)]))

	wu := wireu.NewWireUpdate(body, 0)

	newWU, action, err := s.enforceRFC7606(wu)
	require.NoError(t, err, "exact fit is not a length conflict and must not reset the session")
	assert.Equal(t, message.RFC7606ActionNone, action)
	assert.Equal(t, body, newWU.Payload(), "a valid UPDATE must pass through unrewritten")
}

// TestSessionRFC7606SectionLengthConflictNotification pins the whole of §3.b.
//
// VALIDATES: Total Attribute Length overrunning the message produces a NOTIFICATION with
// Error Code 3 (UPDATE Message Error) and subcode 1 (Malformed Attribute List), and the
// UPDATE never reaches the plugins.
// PREVENTS: a structural length conflict being handled by treat-as-withdraw, which §3(j)
// forbids because the NLRI field cannot even be located, let alone parsed.
//
// §3.b names the SUBCODE, so only a test that reads the NOTIFICATION off the wire covers
// it. TestEnforceRFC7606_ShortBody reaches the same reset path but asserts the action
// alone; the subcode it never inspects is the actual content of this requirement.
//
// enforceRFC7606 runs on the raw body before any UPDATE parse (session_read.go:162), so
// the conflict is caught here rather than by a downstream decoder.
//
// RFC requirement: RFC7606-3.b-1 negative — a section-length conflict sends a NOTIFICATION with subcode Malformed Attribute List.
func TestSessionRFC7606SectionLengthConflictNotification(t *testing.T) {
	session, client, callbackCount, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	// Withdrawn Routes Length = 0, Total Attribute Length = 0xFF, but only the well-formed
	// attribute set actually follows. The declared attribute section therefore runs past
	// the end of the message: §3.b's "Withdrawn Routes Length + Total Attribute Length +
	// 23 exceeds the Message Length".
	update := make([]byte, 0, 4+len(validPathAttrs))
	// Withdrawn Routes Length = 0, then Total Attribute Length = 255 (a lie).
	update = append(update, 0x00, 0x00, 0x00, 0xff)
	update = append(update, validPathAttrs...)

	require.Less(t, len(validPathAttrs), 0xff,
		"the declared attribute length must exceed what follows, or there is no conflict")

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
	require.Error(t, err, "a section-length conflict must reset the session")
	require.Contains(t, err.Error(), "session reset")
	require.Equal(t, fsm.StateIdle, session.State(), "session must be Idle after session-reset")
	require.Equal(t, 0, *callbackCount,
		"the UPDATE must not reach plugins: its section boundaries cannot be trusted")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for NOTIFICATION")
	}

	require.GreaterOrEqual(t, len(received), message.HeaderLen+2, "NOTIFICATION too short")
	hdr, hdrErr := message.ParseHeader(received[:message.HeaderLen])
	require.NoError(t, hdrErr)
	require.Equal(t, message.TypeNOTIFICATION, hdr.Type, "must send NOTIFICATION")
	notifBody := received[message.HeaderLen:]
	require.Equal(t, byte(message.NotifyUpdateMessage), notifBody[0],
		"NOTIFICATION error code must be 3 (UPDATE Message Error)")
	require.Equal(t, message.NotifyUpdateMalformedAttr, notifBody[1],
		"§3.b: NOTIFICATION subcode must be 1 (Malformed Attribute List)")
}

// =============================================================================
// RFC 7606 Section 3 (a) — a session reset MUST be indicated by a NOTIFICATION
// =============================================================================

// TestSessionRFC7606ValidUpdateSendsNoNotification pins the conforming side of §3.a.
//
// VALIDATES: a valid UPDATE produces NO NOTIFICATION and leaves the session Established.
// PREVENTS: an implementation that satisfies §3.a by notifying on everything — which
// every §3.a negative in the suite would pass.
//
// The absence is asserted, not assumed: the client half of the pipe is given a read
// deadline and the read must time out. net.Pipe is unbuffered and synchronous, so had the
// session written a NOTIFICATION, the read would return it instead of expiring.
//
// RFC requirement: RFC7606-3.a-1 positive — an UPDATE with no error sends no NOTIFICATION and the session survives.
func TestSessionRFC7606ValidUpdateSendsNoNotification(t *testing.T) {
	session, client, callbackCount, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	update := make([]byte, 0, 4+len(validPathAttrs)+2)
	update = append(update, 0x00, 0x00, byte(len(validPathAttrs)>>8), byte(len(validPathAttrs)))
	update = append(update, validPathAttrs...)
	update = append(update, 0x08, 0x0a) // NLRI: 10.0.0.0/8

	written := make(chan struct{})
	go func() {
		client.Write(buildUpdateMsg(update)) //nolint:errcheck // test goroutine
		close(written)
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "a valid UPDATE must not produce an error")
	require.Equal(t, fsm.StateEstablished, session.State(), "session must stay Established")
	require.Equal(t, 1, *callbackCount, "the valid UPDATE must reach the plugins")

	select {
	case <-written:
	case <-time.After(time.Second):
		t.Fatal("timeout writing UPDATE")
	}

	// Nothing may come back. Reading with a deadline is what turns "no NOTIFICATION" from
	// an assumption into an assertion.
	require.NoError(t, client.SetReadDeadline(time.Now().Add(200*time.Millisecond)))
	buf := make([]byte, 4096)
	n, readErr := client.Read(buf)
	require.ErrorIs(t, readErr, os.ErrDeadlineExceeded,
		"§3.a scopes NOTIFICATION to errors for which a session reset is specified; "+
			"this UPDATE has no error, so the read must expire with nothing to report. "+
			"Got %d byte(s): %x", n, buf[:max(n, 0)])
}

// =============================================================================
// RFC 7606 Section 2 — treat-as-withdraw applies to malformed UPDATEs, and only those
// =============================================================================

// TestSessionRFC7606ValidUpdateDispatchesAnnouncement pins the conforming side of §2's
// treat-as-withdraw rule.
//
// VALIDATES: a VALID UPDATE is dispatched as an ANNOUNCEMENT — its NLRI stays in the NLRI
// field, its Withdrawn Routes field stays empty, and its path attributes survive intact.
// PREVENTS: SynthesizeWithdraw firing on UPDATEs that have nothing wrong with them, which
// would withdraw every route the peer ever announced while every negative test still passed.
//
// This is the exact mirror of TestSessionRFC7606TreatAsWithdrawDispatchesWithdrawal, which
// sends the same UPDATE with a malformed ORIGIN and requires 10.0.0.0/8 to come out
// WITHDRAWN. Together the two pin the rewrite to the malformed case and nothing else:
// alone, either one passes on an implementation that treats every UPDATE the same way.
//
// The dispatched bytes are captured rather than a bare callback count, because the
// requirement is about WHICH message reaches the plugins, not that one did.
//
// RFC requirement: RFC7606-2-1 positive — an UPDATE with no error is handled as the announcement it is, not as a withdrawal.
//
// RFC7606-2-5 (routes left in / removed from the Adj-RIB-In) is proven where the Adj-RIB-In
// can actually be observed, not at this dispatch boundary: see
// TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute in the adj_rib_in plugin.
func TestSessionRFC7606ValidUpdateDispatchesAnnouncement(t *testing.T) {
	session, client, _, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	var dispatched []byte
	var dispatchCount int
	session.onMessageReceived = func(_ netip.Addr, _ message.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string,
	) bool {
		if direction == rpc.DirectionReceived && wu != nil {
			dispatchCount++
			dispatched = append([]byte(nil), wu.Payload()...)
		}
		return false
	}

	// The same UPDATE as the treat-as-withdraw test, with a WELL-FORMED ORIGIN.
	update := make([]byte, 0, 4+len(validPathAttrs)+2)
	update = append(update, 0x00, 0x00, byte(len(validPathAttrs)>>8), byte(len(validPathAttrs)))
	update = append(update, validPathAttrs...)
	update = append(update, 0x08, 0x0a) // NLRI: 10.0.0.0/8

	go func() {
		sendUpdateAndDrain(client, buildUpdateMsg(update))
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "a valid UPDATE must not return an error")
	require.Equal(t, fsm.StateEstablished, session.State(), "session must survive")

	require.Equal(t, 1, dispatchCount, "the announcement must reach the plugins")
	require.GreaterOrEqual(t, len(dispatched), 4)

	withdrawnLen := int(binary.BigEndian.Uint16(dispatched[0:2]))
	withdrawn := dispatched[2 : 2+withdrawnLen]
	attrLen := int(binary.BigEndian.Uint16(dispatched[2+withdrawnLen : 2+withdrawnLen+2]))
	attrs := dispatched[2+withdrawnLen+2 : 2+withdrawnLen+2+attrLen]
	nlri := dispatched[2+withdrawnLen+2+attrLen:]

	assert.Empty(t, withdrawn,
		"nothing was withdrawn: §2's rewrite is scoped to UPDATEs handled by treat-as-withdraw")
	assert.Equal(t, []byte{0x08, 0x0a}, nlri,
		"10.0.0.0/8 must stay ANNOUNCED, so it is installed in the Adj-RIB-In rather than removed")
	assert.Equal(t, validPathAttrs, attrs,
		"a valid UPDATE's path attributes must survive: nothing here failed validation")
	assert.Equal(t, update, dispatched, "the UPDATE must reach the plugins byte-for-byte unchanged")
}
