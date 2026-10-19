// VALIDATES: RFC 9774 Section 3 on the receive rail. An UPDATE whose AS_PATH or AS4_PATH
// carries an AS_SET or AS_CONFED_SET is treat-as-withdraw at Session.enforceRFC7606, the
// entry point processMessage calls before any plugin sees the UPDATE, so no such route is
// installed and the prefix it names is withdrawn from consumers.
// PREVENTS: installing or relaying an aggregate whose origin cannot be validated (RFC 9774
// Section 4), and the opposite failure: a path of AS_SEQUENCE and AS_CONFED_SEQUENCE
// segments must pass untouched, which the negative cases here prove.

package reactor

import (
	"bytes"
	"io"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// AS_PATH segment types (RFC 4271 Section 4.3, RFC 5065 Section 3).
const (
	rfc9774SegSet       byte = 1
	rfc9774SegSequence  byte = 2
	rfc9774SegConfedSeq byte = 3
	rfc9774SegConfedSet byte = 4
)

// rfc9774Segment builds one AS_PATH segment with the AS numbers at the given width.
func rfc9774Segment(segType byte, octets int, asns ...uint32) []byte {
	out := []byte{segType, byte(len(asns))}
	for _, asn := range asns {
		if octets == 4 {
			out = append(out, byte(asn>>24), byte(asn>>16))
		}
		out = append(out, byte(asn>>8), byte(asn))
	}
	return out
}

// rfc9774Attrs builds ORIGIN, the AS_PATH value given, NEXT_HOP, then any extra
// attribute bytes (LOCAL_PREF for iBGP, AS4_PATH, MP_REACH_NLRI).
func rfc9774Attrs(asPath []byte, extra ...byte) []byte {
	attrs := collapseAttr(0x40, 1, []byte{0})
	attrs = append(attrs, collapseAttr(0x40, 2, asPath)...)
	attrs = append(attrs, collapseAttr(0x40, 3, []byte{192, 0, 2, 254})...)
	return append(attrs, extra...)
}

// TestRFC9774ReceiveASSetTreatAsWithdraw drives AC-1, AC-2 and AC-7 through
// enforceRFC7606 on a two-octet eBGP session, a four-octet eBGP session and an iBGP
// session. Each set case has a twin with the same shape and a sequence segment in the
// set's place, so the verdict is bound to the segment type and to nothing else.
//
// RFC 9774 Section 3: "MUST use the "treat-as-withdraw" error handling behavior per
// [RFC7606] upon reception of BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs
// in the AS_PATH or AS4_PATH [RFC6793]." This test covers the AS_PATH.
//
// RFC requirement: RFC9774-3-2 positive -- an UPDATE whose AS_PATH holds an AS_SET or
// AS_CONFED_SET draws treat-as-withdraw from Session.enforceRFC7606 without a session
// error, on two-octet eBGP, four-octet eBGP and iBGP sessions.
// RFC requirement: RFC9774-3-2 negative -- the same UPDATE with a sequence segment in the
// set's place draws no RFC 7606 action.
func TestRFC9774ReceiveASSetTreatAsWithdraw(t *testing.T) {
	ebgp4 := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	s4, _ := firstASSession(t, ebgp4, []capability.Capability{&capability.ASN4{ASN: 65002}}, nil)
	ibgp := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
	sIBGP, _ := firstASSession(t, ibgp, []capability.Capability{&capability.ASN4{ASN: 65001}}, nil)
	localPref := collapseAttr(0x40, 5, []byte{0, 0, 0, 100})

	sessions := []struct {
		name   string
		s      *Session
		octets int
		lead   []uint32
		extra  []byte
	}{
		{"ebgp-2-octet", rfc7607Session(), 2, []uint32{65002}, nil},
		{"ebgp-4-octet", s4, 4, []uint32{65002}, nil},
		{"ibgp", sIBGP, 4, nil, localPref},
	}
	nlri := []byte{24, 192, 0, 2}
	for _, sess := range sessions {
		for _, setType := range []byte{rfc9774SegSet, rfc9774SegConfedSet} {
			twinType := rfc9774SegSequence
			if setType == rfc9774SegConfedSet {
				twinType = rfc9774SegConfedSeq
			}
			var lead []byte
			if len(sess.lead) > 0 {
				lead = rfc9774Segment(rfc9774SegSequence, sess.octets, sess.lead...)
			}
			if setType == rfc9774SegConfedSet {
				// A confederation segment precedes the AS_SEQUENCE (RFC 5065 Section 3).
				lead = nil
			}
			tail := rfc9774Segment(rfc9774SegSequence, sess.octets, 65010)
			withSet := append(append(append([]byte(nil), lead...), rfc9774Segment(setType, sess.octets, 65020, 65021)...), tail...)
			withTwin := append(append(append([]byte(nil), lead...), rfc9774Segment(twinType, sess.octets, 65020, 65021)...), tail...)
			if setType == rfc9774SegConfedSet && sess.name != "ibgp" {
				// The eBGP first-AS check (RFC 4271 Section 6.3) would refuse a path that does
				// not open with the neighbor's AS, so the confed segment follows it here.
				lead = rfc9774Segment(rfc9774SegSequence, sess.octets, sess.lead...)
				withSet = append(append(append([]byte(nil), lead...), rfc9774Segment(setType, sess.octets, 65020)...), tail...)
				withTwin = append(append(append([]byte(nil), lead...), rfc9774Segment(twinType, sess.octets, 65020)...), tail...)
			}

			_, action, err := sess.s.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, rfc9774Attrs(withSet, sess.extra...), nlri), 0))
			require.NoError(t, err, "%s type %d: a set must not reset the session", sess.name, setType)
			assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action,
				"%s type %d: RFC 9774 Section 3 makes a set in AS_PATH treat-as-withdraw", sess.name, setType)

			_, action, err = sess.s.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, rfc9774Attrs(withTwin, sess.extra...), nlri), 0))
			require.NoError(t, err)
			assert.Equal(t, message.RFC7606ActionNone, action,
				"%s type %d: the same path with a sequence segment must be accepted", sess.name, twinType)
		}
	}
}

// TestRFC9774ReceiveAS4PathSetTreatAsWithdraw drives AC-3 and AC-4: a clean AS_PATH with
// a set in AS4_PATH is treat-as-withdraw on a two-octet session, where the AS4_PATH is
// consumed, and on a four-octet session too, where RFC 6793 would have it ignored (owner
// decision Q-2 (a): Section 3 names AS4_PATH without a condition).
func TestRFC9774ReceiveAS4PathSetTreatAsWithdraw(t *testing.T) {
	ebgp4 := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	s4, _ := firstASSession(t, ebgp4, []capability.Capability{&capability.ASN4{ASN: 65002}}, nil)
	nlri := []byte{24, 192, 0, 2}
	as4Path := func(segType byte) []byte {
		value := append(rfc9774Segment(rfc9774SegSequence, 4, 65002), rfc9774Segment(segType, 4, 4200000001)...)
		return collapseAttr(0xc0, 17, value)
	}
	for _, sess := range []struct {
		name   string
		s      *Session
		octets int
	}{
		{"2-octet", rfc7607Session(), 2},
		{"4-octet", s4, 4},
	} {
		asPath := rfc9774Segment(rfc9774SegSequence, sess.octets, 65002, 23456)
		for _, setType := range []byte{rfc9774SegSet, rfc9774SegConfedSet} {
			body := makeUpdateBody(nil, rfc9774Attrs(asPath, as4Path(setType)...), nlri)
			_, action, err := sess.s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
			require.NoError(t, err)
			assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action,
				"%s type %d: a set in AS4_PATH is treat-as-withdraw, not attribute discard", sess.name, setType)
		}
		body := makeUpdateBody(nil, rfc9774Attrs(asPath, as4Path(rfc9774SegSequence)...), nlri)
		_, action, err := sess.s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
		require.NoError(t, err)
		assert.Equal(t, message.RFC7606ActionNone, action, "%s: a sequence-only AS4_PATH is accepted", sess.name)
	}
}

// TestRFC9774ReceiveReannounceWithdraws drives AC-5 and AC-6 over the real socket: a
// prefix announced with a clean path, then re-announced with an AS_SET, reaches the
// consumers as a withdrawal of that prefix; an IPv6 unicast MP_REACH_NLRI carrying an
// AS_SET reaches them as an MP_UNREACH_NLRI for the same NLRI. firstASReceive captures
// the semantic UPDATE, so a dropped UPDATE cannot pass for a withdrawal.
func TestRFC9774ReceiveReannounceWithdraws(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}
	s, client := firstASSession(t, settings, []capability.Capability{
		&capability.ASN4{ASN: 65002},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}, nil)

	prefix := []byte{24, 203, 0, 113}
	clean := rfc9774Segment(rfc9774SegSequence, 4, 65002, 65010)
	withSet := append(rfc9774Segment(rfc9774SegSequence, 4, 65002), rfc9774Segment(rfc9774SegSet, 4, 65010, 65011)...)

	wu := firstASReceive(t, s, client, makeUpdateBody(nil, rfc9774Attrs(clean), prefix))
	nlri, err := wu.NLRI()
	require.NoError(t, err)
	require.Equal(t, prefix, nlri, "the clean announcement is delivered")

	wu = firstASReceive(t, s, client, makeUpdateBody(nil, rfc9774Attrs(withSet), prefix))
	assert.Equal(t, makeUpdateBody(prefix, nil, nil), wu.Payload(),
		"the AS_SET re-announcement is delivered as a withdrawal of the prefix")

	v6 := []byte{32, 0x20, 0x01, 0x0d, 0xb8}
	mpReach := []byte{0x00, 0x02, 0x01, 16}
	mpReach = append(mpReach, netip.MustParseAddr("2001:db8::1").AsSlice()...)
	mpReach = append(mpReach, 0)
	mpReach = append(mpReach, v6...)
	attrs := collapseAttr(0x40, 1, []byte{0})
	attrs = append(attrs, collapseAttr(0x40, 2, withSet)...)
	attrs = append(attrs, collapseAttr(0x80, 14, mpReach)...)

	wu = firstASReceive(t, s, client, makeUpdateBody(nil, attrs, nil))
	withdrawn, gotAttrs, gotNLRI := payloadSections(t, wu.Payload())
	assert.Empty(t, withdrawn)
	assert.Empty(t, gotNLRI)
	require.GreaterOrEqual(t, len(gotAttrs), 3)
	assert.Equal(t, byte(15), gotAttrs[1], "the only attribute is MP_UNREACH_NLRI")
	assert.True(t, bytes.HasSuffix(gotAttrs, append([]byte{0x00, 0x02, 0x01}, v6...)),
		"MP_UNREACH_NLRI withdraws the same IPv6 NLRI: %x", gotAttrs)
	assert.Equal(t, fsm.StateEstablished, s.State())
}

// TestRFC9774ReceiveSetNotCoalesced covers R-7 on the coalescing reader: a clean UPDATE
// and then an AS_SET UPDATE, each read through readAndProcessCoalesced, reach the
// consumers as an announcement and a withdrawal. The coalescer batches only a verdict
// batchable() accepts, and the set verdict is reached before batching.
func TestRFC9774ReceiveSetNotCoalesced(t *testing.T) {
	session, client, capture, cleanup := setupCapturingSession(t, 65002, false, false)
	defer cleanup()
	answers := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(client)
		answers <- data
	}()

	clean := rfc9774Segment(rfc9774SegSequence, 4, 65002)
	withSet := append(rfc9774Segment(rfc9774SegSequence, 4, 65002), rfc9774Segment(rfc9774SegSet, 4, 65010)...)
	first := []byte{24, 198, 51, 100}
	second := []byte{24, 203, 0, 113}
	for _, update := range [][]byte{
		makeUpdateBody(nil, rfc9774Attrs(clean), first),
		makeUpdateBody(nil, rfc9774Attrs(withSet), second),
	} {
		written := make(chan error, 1)
		go func() {
			_, err := client.Write(buildUpdateMsg(update))
			written <- err
		}()
		require.NoError(t, session.readAndProcessCoalesced(session.Conn(), session.bufReader))
		require.NoError(t, <-written)
	}
	require.NoError(t, session.flushCoalesce())

	got := capture.all()
	var announced, withdrawnSeen bool
	for _, payload := range got {
		withdrawn, _, nlri := payloadSections(t, payload)
		if bytes.Contains(nlri, second) {
			t.Fatalf("the AS_SET prefix was announced to consumers: %x", payload)
		}
		if bytes.Equal(nlri, first) {
			announced = true
		}
		if bytes.Equal(withdrawn, second) {
			withdrawnSeen = true
		}
	}
	assert.True(t, announced, "the clean prefix is announced: %x", got)
	assert.True(t, withdrawnSeen, "the AS_SET prefix is withdrawn: %x", got)
	assert.Equal(t, fsm.StateEstablished, session.State())
	cleanup()
	assert.Empty(t, <-answers, "treat-as-withdraw sends no NOTIFICATION")
}
