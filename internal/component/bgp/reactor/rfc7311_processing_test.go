package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
)

func aigpTestBody(metric uint64) []byte {
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1, 0x40, 5, 4, 0, 0, 0, 100}
	attrs = append(attrs, 0x80, byte(attribute.AttrAIGP), 11)
	var tlv [11]byte
	attribute.WriteAIGPMetric(tlv[:], 0, metric)
	attrs = append(attrs, tlv[:]...)
	return makeUpdateBody(nil, attrs, []byte{24, 10, 20, 0})
}

func aigpReceivedMetric(t *testing.T, body []byte) (uint64, bool) {
	t.Helper()
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	_, _, value, present := attribute.AttrFind(sections.Attrs(body), attribute.AttrAIGP)
	if !present {
		return 0, false
	}
	off, err := attribute.AIGPMetricOffset(value)
	require.NoError(t, err)
	require.GreaterOrEqual(t, off, 0)
	return binary.BigEndian.Uint64(value[off:]), true
}

// RFC requirement: RFC7311-3.3-1 positive -- explicit session enablement accepts AIGP.
// RFC requirement: RFC7311-3.3-1 negative -- explicit disablement overrides internal-session defaults.
// RFC requirement: RFC7311-3.3-2 positive -- an unconfigured external session discards AIGP.
// RFC requirement: RFC7311-3.3-2 negative -- explicit domain enablement permits an external session.
// RFC requirement: RFC7311-3.3-4 positive -- disabled receive processing discards only AIGP.
// RFC requirement: RFC7311-3.3-4 negative -- enabled receive processing retains the metric.
func TestAIGPSessionReceiveBoundary(t *testing.T) {
	enabled, disabled := true, false
	for _, tc := range []struct {
		name    string
		peerAS  uint32
		enabled *bool
		want    bool
	}{
		{"external-default", 65002, nil, false},
		{"external-enabled", 65002, &enabled, true},
		{"internal-default", 65001, nil, true},
		{"internal-disabled", 65001, &disabled, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, tc.peerAS, 0x01020304)
			settings.AIGPSession = tc.enabled
			s := NewSession(settings)
			got, _, err := s.enforceRFC7606(wireu.NewWireUpdate(aigpTestBody(123), 0))
			require.NoError(t, err)
			require.NotNil(t, got)
			metric, present := aigpReceivedMetric(t, got.Payload())
			require.Equal(t, tc.want, present)
			if tc.want {
				require.Equal(t, uint64(123), metric)
			}
			sections, err := wire.ParseUpdateSections(got.Payload())
			require.NoError(t, err)
			require.Equal(t, []byte{24, 10, 20, 0}, sections.NLRI(got.Payload()))
		})
	}
}

// RFC requirement: RFC7311-3.2-5 positive -- the receive walk drops a truncated trailing TLV without losing the route.
// RFC requirement: RFC7311-3.2-5 negative -- a complete unknown TLV and duplicate metric remain valid.
func TestAIGPReceiveValidatesEveryTLV(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020304)
		s := NewSession(settings)
		body := aigpTestBody(123)
		sections, err := wire.ParseUpdateSections(body)
		require.NoError(t, err)
		attrs := bytes.Clone(sections.Attrs(body))
		hdr, _, value, _ := attribute.AttrFind(attrs, attribute.AttrAIGP)
		extra := []byte{99, 0, 5, 0xaa, 0xbb}
		extra = append(extra, value...)
		if malformed {
			extra = append(extra, 99, 0)
		}
		attrs[hdr+2] += byte(len(extra))
		attrs = append(attrs, extra...)
		body = makeUpdateBody(nil, attrs, sections.NLRI(body))
		got, _, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
		require.NoError(t, err)
		_, present := aigpReceivedMetric(t, got.Payload())
		require.Equal(t, !malformed, present)
		parsed, err := wire.ParseUpdateSections(got.Payload())
		require.NoError(t, err)
		require.Equal(t, []byte{24, 10, 20, 0}, parsed.NLRI(got.Payload()))
	}
}

// RFC requirement: RFC7311-3.3-3 positive -- even raw UPDATE injection cannot send AIGP on a disabled session.
// RFC requirement: RFC7311-3.3-3 negative -- an enabled session emits the supplied metric.
func TestAIGPFinalWriterBoundary(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		peer, conn := newAnnouncePeer(t, "192.0.2.2")
		peer.session.settings.AIGPSession = &enabled
		peer.session.settings.AIGPOriginate = true
		peer.session.settings.LocalAddress = netip.MustParseAddr("192.0.2.1")
		body := aigpTestBody(456)
		require.NoError(t, peer.session.SendRawMessage(uint8(msgtype.TypeUPDATE), body))
		frame := conn.written()
		require.GreaterOrEqual(t, len(frame), message.HeaderLen)
		metric, present := aigpReceivedMetric(t, frame[message.HeaderLen:])
		require.Equal(t, enabled, present)
		if enabled {
			require.Equal(t, uint64(456), metric)
		}
		sections, err := wire.ParseUpdateSections(frame[message.HeaderLen:])
		require.NoError(t, err)
		require.Equal(t, []byte{24, 10, 20, 0}, sections.NLRI(frame[message.HeaderLen:]))
	}
}

// RFC requirement: RFC7311-3.4.1-1 positive -- enabling origination allows a metric with next-hop-self.
// RFC requirement: RFC7311-3.4.1-1 negative -- origination remains blocked when its control is false.
// RFC requirement: RFC7311-3.4.1-2 positive -- an absent originate setting removes an explicitly supplied metric.
// RFC requirement: RFC7311-3.4.1-2 negative -- an explicit originate setting admits the same local route.
// RFC requirement: RFC7311-3.4.1-3 positive -- a local next hop permits authorized origination.
// RFC requirement: RFC7311-3.4.1-3 negative -- a third-party next hop cannot originate AIGP.
func TestAIGPOriginationControlsReachWire(t *testing.T) {
	for _, tc := range []struct{ enabled, self bool }{{false, true}, {true, true}, {true, false}} {
		peer, conn := newAnnouncePeer(t, "192.0.2.2")
		enabled := true
		peer.session.settings.AIGPSession = &enabled
		peer.session.settings.AIGPOriginate = tc.enabled
		peer.session.settings.LocalAddress = netip.MustParseAddr("192.0.2.9")
		if tc.self {
			peer.session.settings.LocalAddress = netip.MustParseAddr("192.0.2.1")
		}
		body := aigpTestBody(99)
		sections, err := wire.ParseUpdateSections(body)
		require.NoError(t, err)
		require.NoError(t, peer.session.SendUpdate(&message.Update{PathAttributes: sections.Attrs(body), NLRI: sections.NLRI(body)}))
		frame := conn.written()
		require.GreaterOrEqual(t, len(frame), message.HeaderLen)
		metric, present := aigpReceivedMetric(t, frame[message.HeaderLen:])
		require.Equal(t, tc.enabled && tc.self, present)
		if present {
			require.Equal(t, uint64(99), metric)
		}
	}
}

// RFC requirement: RFC7311-3.4.3-1 positive -- an unchanged next hop preserves the complete received attribute despite a policy replacement.
// RFC requirement: RFC7311-3.4.3-1 negative -- next-hop-self does accumulate distance.
// RFC requirement: RFC7311-3.4.3-2 positive -- overflowing addition saturates at the unsigned maximum.
// RFC requirement: RFC7311-3.4.3-2 negative -- an ordinary sum is not clamped prematurely.
// RFC requirement: RFC7311-3.4.3-3 positive -- a near-maximum metric never wraps to a preferred low metric.
// RFC requirement: RFC7311-3.4.3-3 negative -- zero/ordinary metrics retain their arithmetic value.
// RFC requirement: RFC7311-3.4.3-4 positive -- next-hop-self adds the resolved interior distance.
// RFC requirement: RFC7311-3.4.3-4 negative -- transparent forwarding never adds that distance.
// RFC requirement: RFC7311-3.4.3-5 positive -- a non-zero increment is required for next-hop-self.
// RFC requirement: RFC7311-3.4.3-5 negative -- zero with no configured link cost removes AIGP.
// RFC requirement: RFC7311-3.4.3-6 positive -- a direct link's non-zero configured cost is accumulated without an IGP.
// RFC requirement: RFC7311-3.4.3-6 negative -- no configured link cost cannot silently advertise an unchanged metric after next-hop-self.
func TestAIGPForwardedMetricsReachFinalWire(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		metric, cost, link               uint64
		self, resolved, missing, present bool
		want                             uint64
	}{
		{"unchanged", 100, 30, 0, false, true, false, true, 100},
		{"interior", 100, 30, 0, true, true, false, true, 130},
		{"saturate", ^uint64(0) - 5, 30, 0, true, true, false, true, ^uint64(0)},
		{"link-policy", 100, 0, 7, true, false, false, true, 107},
		{"zero-refused", 100, 0, 0, true, true, false, false, 0},
		{"missing-recursive", 100, 30, 7, true, true, true, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			igpcost.Set(func(netip.Addr) igpcost.Distance {
				return igpcost.Distance{Cost: tc.cost, Resolved: tc.resolved, MissingAIGP: tc.missing}
			})
			t.Cleanup(func() { igpcost.Set(nil) })
			body := aigpTestBody(tc.metric)
			original := bytes.Clone(body)
			facts := &peerForwardFacts{aigpEnabled: true, localAddr: netip.MustParseAddr("192.0.2.9")}
			var mods filterapi.ModAccumulator
			var replacement [11]byte
			attribute.WriteAIGPMetric(replacement[:], 0, 1)
			mods.Op(uint8(attribute.AttrAIGP), filterapi.AttrModSet, replacement[:])
			if tc.self {
				mods.Op(uint8(attribute.AttrNextHop), filterapi.AttrModSet, []byte{192, 0, 2, 9})
			}
			applyFactsAIGP(facts, payloadAIGP(body), payloadNextHop(body), body, netip.MustParseAddr("192.0.2.1"), tc.link, &mods)
			rebuilt, _, failure := buildModifiedPayload(body, &mods, attrModHandlersWithDefaults(), nil, nil)
			require.Equal(t, modifyFailureNone, failure)
			require.NotNil(t, rebuilt)
			peer, conn := newAnnouncePeer(t, "192.0.2.2")
			enabled := true
			peer.session.settings.AIGPSession = &enabled
			// This is a received path on the forwarding rail, not a request to
			// originate an attribute under AIGP_ORIGINATE.
			fwdBatchHandler(fwdKey{}, []fwdItem{{peer: peer, rawBodies: [][]byte{rebuilt}, sourceMessageID: 1}})
			metric, present := aigpReceivedMetric(t, conn.written()[message.HeaderLen:])
			require.Equal(t, tc.present, present)
			if present {
				require.Equal(t, tc.want, metric)
			}
			require.Equal(t, original, body, "one destination must not modify another destination's source")
		})
	}
}

// RFC requirement: RFC7311-3.2-1 positive -- configured origination keeps AIGP for an explicitly permitted in-domain AS_PATH (the legacy ID maps to Section 3.4.1).
// RFC requirement: RFC7311-3.2-1 negative -- a route whose path leaves that domain reaches the socket without AIGP.
func TestAIGPConfiguredOriginationRejectsOutsideDomain(t *testing.T) {
	for _, asn := range []uint32{65001, 65002} {
		peer, conn := newAnnouncePeer(t, "192.0.2.2")
		s := peer.session
		s.settings.AIGPSession = new(true)
		s.settings.AIGPOriginate = true
		s.settings.AIGPDomainAS = []uint32{65001}
		s.settings.LocalAddress = netip.MustParseAddr("192.0.2.1")
		s.negotiated = &capability.Negotiated{ASN4: true}
		path := binary.BigEndian.AppendUint32([]byte{2, 1}, asn)
		var mods filterapi.ModAccumulator
		mods.Op(uint8(attribute.AttrASPath), filterapi.AttrModSet, path)
		body, _, failure := buildModifiedPayload(aigpTestBody(99), &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, failure)
		require.NoError(t, s.SendRawMessage(uint8(msgtype.TypeUPDATE), body))
		frame := conn.written()
		require.GreaterOrEqual(t, len(frame), message.HeaderLen)
		metric, present := aigpReceivedMetric(t, frame[message.HeaderLen:])
		require.Equal(t, asn == 65001, present)
		if present {
			require.Equal(t, uint64(99), metric)
		}
		sections, err := wire.ParseUpdateSections(frame[message.HeaderLen:])
		require.NoError(t, err)
		require.Equal(t, []byte{24, 10, 20, 0}, sections.NLRI(frame[message.HeaderLen:]))
	}
}
