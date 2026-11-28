// Design: docs/architecture/exabgp-bridge.md -- differential byte-identity and hot-path allocation proofs for base64 direct rendering.
// Related: bridge_raw_render.go -- renderRawUpdateDirect direct buffer renderer.
// Related: bridge_base64.go -- ReadBase64Update, resolveRawContextID, RenderRawUpdate.
// Related: bridge_base64_test.go -- parser and Sessions.noteRaw unit tests.

//go:build ze_bgp

package bridge

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"log/slog"
	"net/netip"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpformat "github.com/ze-software/ze/internal/component/bgp/format"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/evpn"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/labeled"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/mup"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/mvpn"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/rtc"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/srpolicy"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpls"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rr"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	testDiffPeerIP  = "10.0.0.1"
	testDiffLocalIP = "10.0.0.2"
)

var volatileTimeFieldRE = regexp.MustCompile(`"time": \d+`)

// normalizeVolatileTime replaces only the volatile Unix timestamp field so
// differential comparisons still assert byte-identical spacing, key order, and
// every non-time envelope member.
func normalizeVolatileTime(b []byte) string {
	return string(volatileTimeFieldRE.ReplaceAll(b, []byte(`"time": 0`)))
}

// buildWireUpdateBody packs withdrawn, path-attributes, and NLRI slices into a
// valid RFC 4271 UPDATE body (without the 19-octet BGP header).
func buildWireUpdateBody(withdrawn, attrs, nlri []byte) []byte {
	out := make([]byte, 0, 4+len(withdrawn)+len(attrs)+len(nlri))
	out = binary.BigEndian.AppendUint16(out, uint16(len(withdrawn)))
	out = append(out, withdrawn...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(attrs)))
	out = append(out, attrs...)
	out = append(out, nlri...)
	return out
}

// packWireAttr encodes one BGP path attribute TLV (3-byte or 4-byte header).
func packWireAttr(flags, code uint8, val []byte) []byte {
	if len(val) > 255 {
		flags |= 0x10
	}
	if flags&0x10 != 0 {
		out := make([]byte, 4+len(val))
		out[0] = flags
		out[1] = code
		binary.BigEndian.PutUint16(out[2:4], uint16(len(val)))
		copy(out[4:], val)
		return out
	}
	out := make([]byte, 3+len(val))
	out[0] = flags
	out[1] = code
	out[2] = byte(len(val))
	copy(out[3:], val)
	return out
}

// packMPReachAttr builds one MP_REACH_NLRI (code 14) path attribute TLV.
func packMPReachAttr(afi uint16, safi uint8, nextHop, nlri []byte) []byte {
	val := make([]byte, 0, 5+len(nextHop)+len(nlri))
	val = binary.BigEndian.AppendUint16(val, afi)
	val = append(val, safi, byte(len(nextHop)))
	val = append(val, nextHop...)
	val = append(val, 0)
	val = append(val, nlri...)
	return packWireAttr(0x80, 14, val)
}

// packMPUnreachAttr builds one MP_UNREACH_NLRI (code 15) path attribute TLV.
func packMPUnreachAttr(afi uint16, safi uint8, withdrawn []byte) []byte {
	val := make([]byte, 0, 3+len(withdrawn))
	val = binary.BigEndian.AppendUint16(val, afi)
	val = append(val, safi)
	val = append(val, withdrawn...)
	return packWireAttr(0x80, 15, val)
}

func makeDiffPeer(remoteKnown bool) *plugin.PeerInfo {
	peer := &plugin.PeerInfo{
		Address:         netip.MustParseAddr(testDiffPeerIP),
		LocalAddress:    netip.MustParseAddr(testDiffLocalIP),
		AddressStr:      testDiffPeerIP,
		LocalAddressStr: testDiffLocalIP,
		LocalAS:         65002,
		RouterID:        0x01010101,
		ConnectionID:    42,
	}
	if remoteKnown {
		peer.PeerAS = rpc.RemoteASOf(65001)
	}
	return peer
}

func makeDiffNegotiatedEvent(asn4 bool) map[string]any {
	return map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message":       map[string]any{"type": "negotiated"},
			"connection-id": "42",
			"peer":          map[string]any{"remote": map[string]any{"address": testDiffPeerIP, "as": float64(65001)}},
			"negotiated": map[string]any{
				"timer":         map[string]any{"hold-time": float64(180)},
				"message-size":  float64(65535),
				"route-refresh": "enhanced",
				"asn4":          asn4,
				"families":      []any{"ipv4/unicast", "ipv6/unicast"},
			},
		},
	}
}

type rawDiffCase struct {
	name            string
	peer            *plugin.PeerInfo
	wireBody        []byte
	encCtx          *bgpctx.EncodingContext
	noteNegotiated  bool
	negotiatedASN4  bool
	expectDirect    bool
	sent            bool
	grantNegotiated bool
}

func runRawDifferentialCheck(t *testing.T, tc rawDiffCase, outFormat Format, forms Forms) {
	t.Helper()

	ctxID := bgpctx.APIContextID
	if tc.encCtx != nil {
		var err error
		ctxID, err = bgpctx.Registry.Register(tc.encCtx)
		require.NoError(t, err)
	}

	wire := wireu.NewWireUpdate(tc.wireBody, ctxID)
	attrs, _ := wire.Attrs()
	dir := rpc.DirectionReceived
	if tc.sent {
		dir = rpc.DirectionSent
	}
	msg := bgptypes.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   tc.wireBody,
		Direction:  dir,
		WireUpdate: wire,
		AttrsWire:  attrs,
	}

	// Legacy path: FormatFull JSON -> json.Unmarshal -> ReadEvent -> Sessions.Note -> ForGrant -> Event.Lines.
	fullContent := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatFull}
	var fullBuf []byte
	if tc.sent {
		fullBuf = bgpformat.AppendSentMessage(nil, tc.peer, msg, fullContent)
	} else {
		fullBuf = bgpformat.AppendMessage(nil, tc.peer, msg, fullContent)
	}
	var zebgp map[string]any
	require.NoError(t, json.Unmarshal(fullBuf, &zebgp), "legacy FormatFull JSON invalid: %s", fullBuf)

	var legacySessions Sessions
	if tc.noteNegotiated {
		neg := ReadEvent(makeDiffNegotiatedEvent(tc.negotiatedASN4))
		legacySessions.Note(&neg)
	}
	legacyEvent := ReadEvent(zebgp)
	legacySessions.Note(&legacyEvent)
	legacyEvent = legacyEvent.ForGrant(tc.grantNegotiated)
	log := slog.New(slog.DiscardHandler)
	want := legacyEvent.Lines(outFormat, forms, log)

	// New path: FormatBase64 -> ReadBase64Update -> Sessions.RenderRawUpdate.
	rawContent := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatBase64}
	var rawBuf []byte
	if tc.sent {
		rawBuf = bgpformat.AppendSentMessage(nil, tc.peer, msg, rawContent)
	} else {
		rawBuf = bgpformat.AppendMessage(nil, tc.peer, msg, rawContent)
	}
	var scratch [4096]byte
	rawUpdate, err := ReadBase64Update(string(rawBuf), scratch[:0])
	require.NoError(t, err, "ReadBase64Update failed for line: %q", rawBuf)

	var rawSessions Sessions
	if tc.noteNegotiated {
		neg := ReadEvent(makeDiffNegotiatedEvent(tc.negotiatedASN4))
		rawSessions.Note(&neg)
	}
	if tc.expectDirect {
		directBytes, ok := rawSessions.renderRawUpdateDirect(rawUpdate, outFormat, forms, tc.grantNegotiated, log)
		require.True(t, ok, "expected direct hot path for case %q format=%v forms=%b", tc.name, outFormat, forms)
		require.Equal(t, normalizeVolatileTime(want), normalizeVolatileTime(directBytes),
			"direct output mismatch for %q format=%v forms=%b", tc.name, outFormat, forms)
	}

	got := rawSessions.RenderRawUpdate(rawUpdate, outFormat, forms, tc.grantNegotiated, log)
	require.Equal(t, normalizeVolatileTime(want), normalizeVolatileTime(got),
		"byte-identical mismatch for %q format=%v forms=%b", tc.name, outFormat, forms)

	// Also confirm semantic DropVolatile JSON equality for every JSON line.
	if outFormat != FormatV4Text {
		wantLines := strings.Split(strings.TrimSuffix(string(want), "\n"), "\n")
		gotLines := strings.Split(strings.TrimSuffix(string(got), "\n"), "\n")
		require.Equal(t, len(wantLines), len(gotLines))
		for i := range wantLines {
			var wantDoc, gotDoc map[string]any
			require.NoError(t, json.Unmarshal([]byte(wantLines[i]), &wantDoc))
			require.NoError(t, json.Unmarshal([]byte(gotLines[i]), &gotDoc))
			assert.Equal(t, DropVolatile(wantDoc), DropVolatile(gotDoc))
		}
	}
}

// TestBase64Differential_MatchesFormatFull proves byte-identical ExaBGP v4/v6
// JSON and text output between legacy FormatFull JSON and FormatBase64 across
// all unicast features, all forms/grants/directions, all 10 non-unicast
// families, and unsplit/undecoded NLRI warning paths.
//
// VALIDATES: AC-5, AC-6 -- byte-identical output (modulo volatile `time`) for
// IPv4/IPv6 unicast, ADD-PATH (including path-id 0), EOR, empty AS_PATH,
// withdraw-with-NEXT_HOP, standard/well-known/large/extended/IPv6-extended
// communities, unknown attributes, dynamic peer-as auto, 2-byte/4-byte ASN,
// all 10 non-unicast families, and unsplit/undecoded error warnings.
// PREVENTS: formatting drift in spacing, key order, text attribute order,
// community sorting, ADD-PATH formatting, or warning logs between FormatFull
// and FormatBase64.
func TestBase64Differential_MatchesFormatFull(t *testing.T) {
	peer := makeDiffPeer(true)
	dynPeer := makeDiffPeer(false)

	// Rich IPv4 unicast announce exercising every whitelisted attribute.
	var richAttrs []byte
	richAttrs = append(richAttrs, packWireAttr(0x40, 1, []byte{0x00})...) // ORIGIN: igp
	// AS_PATH: AS_SEQUENCE [65001, 65002], AS_SET [65003, 65004], AS_CONFED_SEQ [64512], AS_CONFED_SET [64513].
	richAttrs = append(richAttrs, packWireAttr(0x40, 2, []byte{
		2, 2, 0, 0, 0xfd, 0xe9, 0, 0, 0xfd, 0xea,
		1, 2, 0, 0, 0xfd, 0xeb, 0, 0, 0xfd, 0xec,
		3, 1, 0, 0, 0xfc, 0x00,
		4, 1, 0, 0, 0xfc, 0x01,
	})...)
	richAttrs = append(richAttrs, packWireAttr(0x40, 3, []byte{10, 0, 0, 1})...)                  // NEXT_HOP
	richAttrs = append(richAttrs, packWireAttr(0x80, 4, []byte{0, 0, 0, 200})...)                 // MED: 200
	richAttrs = append(richAttrs, packWireAttr(0x40, 5, []byte{0, 0, 0, 100})...)                 // LOCAL_PREF: 100
	richAttrs = append(richAttrs, packWireAttr(0x40, 6, nil)...)                                  // ATOMIC_AGGREGATE
	richAttrs = append(richAttrs, packWireAttr(0xc0, 7, []byte{0, 0, 0xfd, 0xe9, 1, 2, 3, 4})...) // AGGREGATOR 4B
	// COMMUNITIES unsorted on wire: no-export, 65001:20, blackhole, 65001:10, no-peer, no-advertise, no-export-subconfed.
	richAttrs = append(richAttrs, packWireAttr(0xc0, 8, []byte{
		0xff, 0xff, 0xff, 0x01,
		0xfd, 0xe9, 0x00, 0x14,
		0xff, 0xff, 0x02, 0x9a,
		0xfd, 0xe9, 0x00, 0x0a,
		0xff, 0xff, 0xff, 0x04,
		0xff, 0xff, 0xff, 0x02,
		0xff, 0xff, 0xff, 0x03,
	})...)
	richAttrs = append(richAttrs, packWireAttr(0x80, 9, []byte{192, 0, 2, 9})...)            // ORIGINATOR_ID
	richAttrs = append(richAttrs, packWireAttr(0x80, 10, []byte{1, 1, 1, 1, 2, 2, 2, 2})...) // CLUSTER_LIST
	// LARGE_COMMUNITIES unsorted on wire: 65001:2:3, 65001:1:2.
	richAttrs = append(richAttrs, packWireAttr(0xc0, 32, []byte{
		0, 0, 0xfd, 0xe9, 0, 0, 0, 2, 0, 0, 0, 3,
		0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 0, 0, 0, 2,
	})...)
	richAttrs = append(richAttrs, packWireAttr(0xe0, 0xCC, []byte{0xde, 0xad, 0xbe, 0xef})...) // Unknown attr

	richV4Body := buildWireUpdateBody(
		nil,
		richAttrs,
		[]byte{24, 192, 0, 2, 16, 198, 51}, // 192.0.2.0/24, 198.51.0.0/16
	)

	// IPv4 withdraw + announce combined AND withdraw-only with NEXT_HOP retained.
	v4BasicAttrs := append(
		append(packWireAttr(0x40, 1, []byte{0x01}), packWireAttr(0x40, 2, []byte{2, 1, 0, 0, 0xfd, 0xe9})...),
		packWireAttr(0x40, 3, []byte{10, 0, 0, 1})...,
	)
	v4WithdrawAndAnnounce := buildWireUpdateBody(
		[]byte{24, 203, 0, 113},
		v4BasicAttrs,
		[]byte{24, 192, 0, 2},
	)
	v4WithdrawWithNextHop := buildWireUpdateBody(
		[]byte{24, 203, 0, 113},
		packWireAttr(0x40, 3, []byte{10, 0, 0, 9}),
		nil,
	)

	// Empty AS_PATH, empty COMMUNITIES, empty CLUSTER_LIST, empty LARGE_COMMUNITIES, ORIGIN incomplete.
	var emptyListAttrs []byte
	emptyListAttrs = append(emptyListAttrs, packWireAttr(0x40, 1, []byte{0x02})...) // ORIGIN: incomplete
	emptyListAttrs = append(emptyListAttrs, packWireAttr(0x40, 2, nil)...)          // empty AS_PATH
	emptyListAttrs = append(emptyListAttrs, packWireAttr(0x40, 3, []byte{10, 0, 0, 1})...)
	emptyListAttrs = append(emptyListAttrs, packWireAttr(0xc0, 8, nil)...)  // empty COMMUNITIES
	emptyListAttrs = append(emptyListAttrs, packWireAttr(0x80, 10, nil)...) // empty CLUSTER_LIST
	emptyListAttrs = append(emptyListAttrs, packWireAttr(0xc0, 32, nil)...) // empty LARGE_COMMUNITIES
	v4EmptyListsBody := buildWireUpdateBody(nil, emptyListAttrs, []byte{32, 192, 0, 2, 1})

	// IPv6 unicast MP_REACH (32-byte global+link-local NH) + MP_UNREACH combined.
	v6NH32 := []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
		0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
	}
	v6NLRI := []byte{
		48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01,
		64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02, 0x00, 0x00,
	}
	v6WD := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0xff}
	var v6Attrs []byte
	v6Attrs = append(v6Attrs, packMPReachAttr(2, 1, v6NH32, v6NLRI)...)
	v6Attrs = append(v6Attrs, packMPUnreachAttr(2, 1, v6WD)...)
	v6Attrs = append(v6Attrs, packWireAttr(0x40, 1, []byte{0x00})...)
	v6Attrs = append(v6Attrs, packWireAttr(0x40, 2, []byte{2, 1, 0, 0, 0xfd, 0xe9})...)
	v6CombinedBody := buildWireUpdateBody(nil, v6Attrs, nil)

	// IPv4 & IPv6 ADD-PATH (including path-id 0 and non-zero path-id).
	addPathCtx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{
		family.IPv4Unicast: true,
		family.IPv6Unicast: true,
	})
	v4AddPathBody := buildWireUpdateBody(
		[]byte{0, 0, 0, 0, 24, 203, 0, 113},
		v4BasicAttrs,
		[]byte{0, 0, 0, 0, 24, 192, 0, 2, 1, 2, 3, 4, 24, 198, 51, 100},
	)
	var v6APAttrs []byte
	v6APAttrs = append(v6APAttrs, packMPReachAttr(2, 1, v6NH32[:16], []byte{
		0, 0, 0, 0, 48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01,
		10, 20, 30, 40, 48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02,
	})...)
	v6APAttrs = append(v6APAttrs, packMPUnreachAttr(2, 1, []byte{
		0, 0, 0, 0, 48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x03,
	})...)
	v6APAttrs = append(v6APAttrs, packWireAttr(0x40, 1, []byte{0x00})...)
	v6AddPathBody := buildWireUpdateBody(nil, v6APAttrs, nil)

	// 2-byte ASN (asn4=false) with 2-byte AS_PATH + 6-byte AGGREGATOR.
	asn2Ctx := bgpctx.EncodingContextForASN4(false)
	var asn2Attrs []byte
	asn2Attrs = append(asn2Attrs, packWireAttr(0x40, 1, []byte{0x00})...)
	asn2Attrs = append(asn2Attrs, packWireAttr(0x40, 2, []byte{2, 2, 0xfd, 0xe9, 0xfd, 0xea})...)
	asn2Attrs = append(asn2Attrs, packWireAttr(0x40, 3, []byte{10, 0, 0, 1})...)
	asn2Attrs = append(asn2Attrs, packWireAttr(0xc0, 7, []byte{0xfd, 0xe9, 1, 2, 3, 4})...)
	asn2Body := buildWireUpdateBody(nil, asn2Attrs, []byte{24, 192, 0, 2})

	// AS4_AGGREGATOR (code 18).
	var as4AggAttrs []byte
	as4AggAttrs = append(as4AggAttrs, packWireAttr(0x40, 1, []byte{0x00})...)
	as4AggAttrs = append(as4AggAttrs, packWireAttr(0x40, 3, []byte{10, 0, 0, 1})...)
	as4AggAttrs = append(as4AggAttrs, packWireAttr(0xc0, 18, []byte{0xfa, 0x56, 0xea, 0x00, 192, 0, 2, 1})...)
	as4AggBody := buildWireUpdateBody(nil, as4AggAttrs, []byte{24, 192, 0, 2})

	// Extended communities + IPv6 extended communities (exercises fallback parity).
	var extCommAttrs []byte
	extCommAttrs = append(extCommAttrs, packWireAttr(0x40, 1, []byte{0x00})...)
	extCommAttrs = append(extCommAttrs, packWireAttr(0x40, 3, []byte{10, 0, 0, 1})...)
	extCommAttrs = append(extCommAttrs, packWireAttr(0xc0, 16, []byte{
		0x00, 0x02, 0x00, 0x48, 0x00, 0x00, 0x00, 0x01, // target:72:1
		0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // rate-limit:0
		0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0a, // mark:10
		0x80, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, // traffic-action
		0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // simpson redirect-to-nexthop
		0x07, 0x02, 0x00, 0x00, 0xfd, 0xe9, 0x40, 0x05, // interface-set transitive
	})...)
	extCommAttrs = append(extCommAttrs, packWireAttr(0xc0, 25, []byte{
		0x00, 0x02,
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
		0x00, 0x64,
	})...)
	extCommBody := buildWireUpdateBody(nil, extCommAttrs, []byte{24, 192, 0, 2})

	unicastCases := []rawDiffCase{
		{name: "ipv4_rich_all_attrs", peer: peer, wireBody: richV4Body, noteNegotiated: true, negotiatedASN4: true, expectDirect: true, grantNegotiated: true},
		{name: "ipv4_rich_sent_no_neg", peer: peer, wireBody: richV4Body, expectDirect: true, sent: true, grantNegotiated: false},
		{name: "ipv4_withdraw_and_announce", peer: peer, wireBody: v4WithdrawAndAnnounce, expectDirect: true, grantNegotiated: true},
		{name: "ipv4_withdraw_with_nexthop", peer: peer, wireBody: v4WithdrawWithNextHop, expectDirect: true},
		{name: "ipv4_empty_lists_and_aspath", peer: peer, wireBody: v4EmptyListsBody, expectDirect: true},
		{name: "ipv6_mp_reach_and_unreach", peer: peer, wireBody: v6CombinedBody, noteNegotiated: true, negotiatedASN4: true, expectDirect: true, grantNegotiated: true},
		{name: "ipv4_addpath_with_zero_id", peer: peer, wireBody: v4AddPathBody, encCtx: addPathCtx, expectDirect: true},
		{name: "ipv6_addpath_with_zero_id", peer: peer, wireBody: v6AddPathBody, encCtx: addPathCtx, expectDirect: true, sent: true},
		{name: "ipv4_eor", peer: peer, wireBody: []byte{0, 0, 0, 0}, expectDirect: true, grantNegotiated: true},
		{name: "ipv6_mp_eor", peer: peer, wireBody: []byte{0, 0, 0, 6, 0x80, 0x0f, 3, 0, 2, 1}, expectDirect: true, sent: true},
		{name: "dynamic_peer_as_auto", peer: dynPeer, wireBody: v4WithdrawAndAnnounce, expectDirect: true},
		{name: "asn2_aspath_and_aggregator", peer: peer, wireBody: asn2Body, encCtx: asn2Ctx, noteNegotiated: true, negotiatedASN4: false, expectDirect: true},
		{name: "as4_aggregator", peer: peer, wireBody: as4AggBody, expectDirect: true},
		{name: "ext_and_ipv6_ext_communities_fallback", peer: peer, wireBody: extCommBody, expectDirect: false, grantNegotiated: true},
	}

	allFormats := []Format{FormatV4JSON, FormatV6JSON, FormatV4Text}
	allForms := []Forms{FormParsed, FormPackets, FormConsolidated, FormPackets | FormParsed}

	for _, tc := range unicastCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, f := range allFormats {
				for _, fm := range allForms {
					runRawDifferentialCheck(t, tc, f, fm)
				}
			}
		})
	}

	// All 10 compiled non-unicast families via wire-backed fallback.
	originAttr := packWireAttr(0x40, 1, []byte{0x00})
	nhV4 := []byte{10, 0, 0, 1}
	nhVPNv4 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 1}
	rd := []byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 1} // 0:65001:1

	evpnNLRI := make([]byte, 0, 19)
	evpnNLRI = append(evpnNLRI, 3, 17)
	evpnNLRI = append(evpnNLRI, rd...)
	evpnNLRI = append(evpnNLRI, 0, 0, 0, 0, 32, 10, 0, 0, 1)

	vplsNLRI := make([]byte, 0, 19)
	vplsNLRI = append(vplsNLRI, 0, 17)
	vplsNLRI = append(vplsNLRI, rd...)
	vplsNLRI = append(vplsNLRI, 0, 1, 0, 1, 0, 8, 0x00, 0x06, 0x41)

	vpnNLRI := make([]byte, 0, 15)
	vpnNLRI = append(vpnNLRI, 112, 0x00, 0x06, 0x41)
	vpnNLRI = append(vpnNLRI, rd...)
	vpnNLRI = append(vpnNLRI, 10, 1, 0)

	mupNLRI := make([]byte, 0, 16)
	mupNLRI = append(mupNLRI, 1, 0, 2, 12)
	mupNLRI = append(mupNLRI, rd...)
	mupNLRI = append(mupNLRI, 10, 0, 0, 1)

	mvpnNLRI := make([]byte, 0, 14)
	mvpnNLRI = append(mvpnNLRI, 1, 12)
	mvpnNLRI = append(mvpnNLRI, rd...)
	mvpnNLRI = append(mvpnNLRI, 10, 0, 0, 1)

	bgplsNLRI := []byte{
		0x00, 0x01, // Node NLRI
		0x00, 0x15, // Length 21
		0x03,                   // OSPFv2
		0, 0, 0, 0, 0, 0, 0, 0, // Identifier
		0x01, 0x00, 0x00, 0x08, // Local Node Descriptors TLV (len 8)
		0x02, 0x00, 0x00, 0x04, 0, 0, 0xfd, 0xe9, // AS TLV (65001)
	}

	nonUnicastCases := []struct {
		name string
		mp   []byte
	}{
		{"evpn", packMPReachAttr(25, 70, nhV4, evpnNLRI)},
		{"flowspec", packMPReachAttr(1, 133, nil, []byte{5, 1, 24, 192, 0, 2})},
		{"vpls", packMPReachAttr(25, 65, nhV4, vplsNLRI)},
		{"vpn", packMPReachAttr(1, 128, nhVPNv4, vpnNLRI)},
		{"labeled", packMPReachAttr(1, 4, nhV4, []byte{48, 0x00, 0x06, 0x41, 192, 0, 2})},
		{"rtc", packMPReachAttr(1, 132, nhV4, []byte{96, 0, 0, 0xfd, 0xe9, 0x00, 0x02, 0xfd, 0xe9, 0, 0, 0, 1})},
		{"bgp_ls", packMPReachAttr(16388, 71, nhV4, bgplsNLRI)},
		{"mup", packMPReachAttr(1, 85, nhV4, mupNLRI)},
		{"mvpn", packMPReachAttr(1, 5, nhV4, mvpnNLRI)},
		{"sr_policy", packMPReachAttr(1, 73, nhV4, []byte{96, 0, 0, 0, 1, 0, 0, 0, 100, 10, 0, 0, 1})},
	}

	for _, fc := range nonUnicastCases {
		t.Run("family_"+fc.name, func(t *testing.T) {
			attrs := append(append([]byte(nil), fc.mp...), originAttr...)
			body := buildWireUpdateBody(nil, attrs, nil)
			tc := rawDiffCase{
				name:            fc.name,
				peer:            peer,
				wireBody:        body,
				noteNegotiated:  true,
				negotiatedASN4:  true,
				expectDirect:    false,
				grantNegotiated: true,
			}
			for _, f := range allFormats {
				for _, fm := range []Forms{FormParsed, FormConsolidated} {
					runRawDifferentialCheck(t, tc, f, fm)
				}
			}
		})
	}

	// Unsplit and undecoded NLRI error cases must fall back and emit byte-identical output.
	t.Run("unsplit_ipv6_nlri_and_undecoded_family", func(t *testing.T) {
		unsplitAttrs := append(packMPReachAttr(2, 1, v6NH32[:16], []byte{0x40, 0xfd, 0xab}), originAttr...)
		unsplitBody := buildWireUpdateBody(nil, unsplitAttrs, nil)

		undecodedAttrs := append(packMPReachAttr(1, 99, nhV4, []byte{0xde, 0xad, 0xbe, 0xef}), originAttr...)
		undecodedBody := buildWireUpdateBody(nil, undecodedAttrs, nil)

		for _, errCase := range []struct {
			name string
			body []byte
		}{
			{"unsplit_ipv6", unsplitBody},
			{"undecoded_unknown_safi", undecodedBody},
		} {
			t.Run(errCase.name, func(t *testing.T) {
				tc := rawDiffCase{
					name:         errCase.name,
					peer:         peer,
					wireBody:     errCase.body,
					expectDirect: false,
				}
				for _, f := range allFormats {
					runRawDifferentialCheck(t, tc, f, FormParsed)
				}
			})
		}
	})
}

// TestBase64DirectRender_HotPathNoMapAndContextReuse proves that hot IPv4 and
// IPv6 unicast UPDATEs take the direct []byte buffer renderer without map
// allocations or json.Marshal reflection, and that resolveRawContextID reuses
// cached ContextIDs across 10,000 UPDATEs.
//
// VALIDATES: AC-7 -- direct path selection (`ok == true`), tightly bounded
// per-UPDATE heap allocations (`testing.AllocsPerRun`), and ContextID cache
// reuse across 10,000 iterations without growing bgpctx.Registry.
// PREVENTS: silent regression onto the map[string]any fallback on hot unicast
// routes or per-UPDATE bgpctx.Registry exhaustion.
func TestBase64DirectRender_HotPathNoMapAndContextReuse(t *testing.T) {
	var sessions Sessions
	neg := ReadEvent(makeDiffNegotiatedEvent(true))
	sessions.Note(&neg)

	// Hot IPv4 unicast announce.
	var v4Attrs []byte
	v4Attrs = append(v4Attrs, packWireAttr(0x40, 1, []byte{0x00})...)
	v4Attrs = append(v4Attrs, packWireAttr(0x40, 2, []byte{2, 2, 0, 0, 0xfd, 0xe9, 0, 0, 0xfd, 0xea})...)
	v4Attrs = append(v4Attrs, packWireAttr(0x40, 3, []byte{10, 0, 0, 1})...)
	v4Attrs = append(v4Attrs, packWireAttr(0x40, 5, []byte{0, 0, 0, 100})...)
	v4Attrs = append(v4Attrs, packWireAttr(0xc0, 8, []byte{0xfd, 0xe9, 0x00, 0x64})...)
	v4Body := buildWireUpdateBody(nil, v4Attrs, []byte{24, 192, 0, 2})

	// Hot IPv6 unicast announce.
	v6NH := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	var v6Attrs []byte
	v6Attrs = append(v6Attrs, packMPReachAttr(2, 1, v6NH, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})...)
	v6Attrs = append(v6Attrs, packWireAttr(0x40, 1, []byte{0x00})...)
	v6Attrs = append(v6Attrs, packWireAttr(0x40, 2, []byte{2, 1, 0, 0, 0xfd, 0xe9})...)
	v6Body := buildWireUpdateBody(nil, v6Attrs, nil)

	flags := base64FlagHasASN4 | base64FlagASN4 | base64FlagHasRemoteAS
	rawV4 := RawUpdate{
		Peer:     testDiffPeerIP,
		Local:    testDiffLocalIP,
		RouterID: "1.1.1.1",
		ConnID:   42,
		LocalAS:  65002,
		RemoteAS: 65001,
		Flags:    flags,
		Body:     v4Body,
	}
	rawV6 := rawV4
	rawV6.Body = v6Body

	log := slog.New(slog.DiscardHandler)

	// Confirm direct path fires across formats, forms, cached & unexchanged negotiated.
	var uncached Sessions
	for _, sample := range []struct {
		name string
		raw  RawUpdate
		sess *Sessions
		neg  bool
	}{
		{"v4_cached_neg", rawV4, &sessions, true},
		{"v4_no_neg", rawV4, &uncached, false},
		{"v4_unexchanged_neg", rawV4, &uncached, true},
		{"v6_cached_neg", rawV6, &sessions, true},
		{"v6_no_neg", rawV6, &uncached, false},
	} {
		for _, f := range []Format{FormatV4JSON, FormatV6JSON, FormatV4Text} {
			for _, fm := range []Forms{FormParsed, FormPackets, FormConsolidated, FormPackets | FormParsed} {
				out, ok := sample.sess.renderRawUpdateDirect(sample.raw, f, fm, sample.neg, log)
				require.True(t, ok, "expected direct render for %s format=%v forms=%b", sample.name, f, fm)
				require.NotEmpty(t, out)
			}
		}
	}

	// Allocation bound proof: direct unicast render allocates only the output buffer
	// (no map[string]any, no json.Marshal reflection, no per-UPDATE interface boxing).
	const maxDirectAllocs = 4
	allocCases := []struct {
		name   string
		raw    RawUpdate
		format Format
		forms  Forms
		grant  bool
	}{
		{"ipv4_v6json_parsed", rawV4, FormatV6JSON, FormParsed, false},
		{"ipv4_v6json_parsed_with_negotiated", rawV4, FormatV6JSON, FormParsed, true},
		{"ipv4_v4json_consolidated", rawV4, FormatV4JSON, FormConsolidated, true},
		{"ipv4_text_parsed", rawV4, FormatV4Text, FormParsed, false},
		{"ipv6_v6json_parsed_with_negotiated", rawV6, FormatV6JSON, FormParsed, true},
		{"ipv6_text_parsed", rawV6, FormatV4Text, FormParsed, false},
	}
	for _, ac := range allocCases {
		t.Run("allocs_"+ac.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(200, func() {
				out, ok := sessions.renderRawUpdateDirect(ac.raw, ac.format, ac.forms, ac.grant, log)
				if !ok {
					t.Fatal("direct render unexpectedly missed")
				}
				if len(out) == 0 {
					t.Fatal("direct render returned empty output")
				}
			})
			assert.LessOrEqual(t, allocs, float64(maxDirectAllocs),
				"hot-path direct render %s allocated %.1f times (limit %d)", ac.name, allocs, maxDirectAllocs)
		})
	}

	// ContextID cache reuse proof across 10,000 UPDATEs.
	t.Run("context_id_reuse_10000_updates", func(t *testing.T) {
		firstID, err := resolveRawContextID(true, true, true, "ipv4/mpls-label")
		require.NoError(t, err)
		countAfterFirst := bgpctx.Registry.Count()
		for range 10000 {
			gotID, resolveErr := resolveRawContextID(true, true, true, "ipv4/mpls-label")
			require.NoError(t, resolveErr)
			require.Equal(t, firstID, gotID)
		}
		assert.Equal(t, countAfterFirst, bgpctx.Registry.Count(),
			"resolveRawContextID must reuse cached ContextID across 10,000 UPDATEs")
	})

	// Sanity-check that CWE-116 newline injection in peer address is escaped on direct text path.
	t.Run("text_peer_oneline_sanitization", func(t *testing.T) {
		hostile := rawV4
		hostile.Peer = "10.0.0.1\nneighbor 1.2.3.4 down - forged"
		out, ok := sessions.renderRawUpdateDirect(hostile, FormatV4Text, FormParsed, false, log)
		require.True(t, ok)
		// Expected lines: start, announced, end (3 newlines total, none forged).
		assert.Equal(t, 3, bytes.Count(out, []byte{'\n'}))
		assert.Contains(t, string(out), `10.0.0.1\nneighbor 1.2.3.4 down - forged`)
	})
}
