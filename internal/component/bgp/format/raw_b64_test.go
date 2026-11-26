// Design: docs/architecture/api/json-format.md -- base64 positional counted UPDATE line.
// Related: raw_b64.go -- encoder under test.

package format

import (
	"encoding/base64"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type parsedRawB64Line struct {
	flags        uint8
	localAS      uint32
	remoteAS     uint32
	connID       uint64
	peer         string
	local        string
	routerID     string
	extraAddPath string
	b64Body      string
	decodedBody  []byte
}

func parseTestRawB64Line(t *testing.T, line string) parsedRawB64Line {
	t.Helper()

	require.True(t, strings.HasSuffix(line, "\n"), "line must end with single newline: %q", line)
	body := strings.TrimSuffix(line, "\n")
	assert.NotContains(t, body, "\n", "line must not contain embedded newline")
	assert.NotContains(t, body, "\r", "line must not contain carriage return")
	assert.NotContains(t, body, `"`, "line must contain zero double quotes")
	assert.NotContains(t, body, `\`, "line must contain zero backslashes")

	require.True(t, strings.HasPrefix(body, rawB64KindPrefix), "line must start with %q, got %q", rawB64KindPrefix, body)
	rest := body[len(rawB64KindPrefix):]

	flagsVal, rest := cutTestNumber(t, rest, "flags")
	require.LessOrEqual(t, flagsVal, uint64(63), "flags must fit in 6 bits")
	localASVal, rest := cutTestNumber(t, rest, "local-as")
	remoteASVal, rest := cutTestNumber(t, rest, "remote-as")
	connIDVal, rest := cutTestNumber(t, rest, "conn-id")

	peer, rest := cutTestCountedText(t, rest, "peer", true)
	local, rest := cutTestCountedText(t, rest, "local", true)
	routerID, rest := cutTestCountedText(t, rest, "router-id", true)
	extraAddPath, rest := cutTestCountedText(t, rest, "extra-addpath", true)
	b64Body, rest := cutTestCountedText(t, rest, "b64-body", false)
	require.Empty(t, rest, "unexpected trailing bytes after b64-body: %q", rest)

	decoded, err := base64.StdEncoding.DecodeString(b64Body)
	require.NoError(t, err, "b64-body must be valid standard base64")

	return parsedRawB64Line{
		flags:        uint8(flagsVal),
		localAS:      uint32(localASVal),
		remoteAS:     uint32(remoteASVal),
		connID:       connIDVal,
		peer:         peer,
		local:        local,
		routerID:     routerID,
		extraAddPath: extraAddPath,
		b64Body:      b64Body,
		decodedBody:  decoded,
	}
}

func cutTestNumber(t *testing.T, s, field string) (uint64, string) {
	t.Helper()
	numStr, rest, ok := strings.Cut(s, " ")
	require.True(t, ok, "missing space after %s in %q", field, s)
	val, err := strconv.ParseUint(numStr, 10, 64)
	require.NoError(t, err, "invalid decimal %s %q", field, numStr)
	return val, rest
}

func cutTestCountedText(t *testing.T, s, field string, expectSpace bool) (string, string) {
	t.Helper()
	lenStr, afterColon, ok := strings.Cut(s, ":")
	require.True(t, ok, "missing ':' in %s: %q", field, s)
	n, err := strconv.ParseUint(lenStr, 10, 64)
	require.NoError(t, err, "invalid length in %s: %q", field, lenStr)
	require.LessOrEqual(t, int(n), len(afterColon), "%s length %d exceeds remaining %d bytes", field, n, len(afterColon))
	val := afterColon[:n]
	rest := afterColon[n:]
	if expectSpace {
		require.NotEmpty(t, rest, "missing trailing space after %s", field)
		require.Equal(t, byte(' '), rest[0], "expected space after %s, got %q", field, rest)
		rest = rest[1:]
	}
	return val, rest
}

func registerTestRawB64Ctx(t *testing.T, asn4 bool, dir bgpctx.Direction, addPath map[family.Family]capability.AddPathMode) bgpctx.ContextID {
	t.Helper()
	ec := bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{
		ASN4:        asn4,
		AddPathMode: addPath,
	}, dir)
	id, err := bgpctx.Registry.Register(ec)
	require.NoError(t, err)
	return id
}

// TestAppendRawB64Update_FieldsAndFlags verifies positional counted `base64`
// UPDATE formatting across received/sent directions, ASN4/unknown contexts,
// dynamic peers, absent peer fields, unicast and non-unicast ADD-PATH flags,
// EOR markers, and malformed UPDATE bodies.
//
// VALIDATES: AC-1 -- every positional field, all 6 flag bits, deterministic
// extra-addpath sorting, exact base64 body encoding, single newline, zero
// quotes/backslashes, and bypass of bgpfilter.ApplyToUpdate on malformed wire.
// PREVENTS: missing flag bits, nil-context panics, dropped malformed UPDATEs,
// or non-deterministic extra-addpath wire output.
func TestAppendRawB64Update_FieldsAndFlags(t *testing.T) {
	content := bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatBase64,
	}

	const (
		testPeerIP   = "10.0.0.1"
		testLocalIP  = "10.0.0.2"
		testRouterIP = "1.1.1.1"
	)

	basePeer := &plugin.PeerInfo{
		Address:      netip.MustParseAddr(testPeerIP),
		LocalAddress: netip.MustParseAddr(testLocalIP),
		LocalAS:      65000,
		PeerAS:       rpc.RemoteASOf(65001),
		RouterID:     0x01010101,
		ConnectionID: 42,
	}

	sampleUpdate := []byte{
		0x00, 0x00,
		0x00, 0x14,
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9,
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01,
		0x18, 0xc0, 0x00, 0x02,
	}

	ctxASN4True := registerTestRawB64Ctx(t, true, bgpctx.DirectionRecv, nil)
	ctxASN4False := registerTestRawB64Ctx(t, false, bgpctx.DirectionRecv, nil)
	ctxUnicastAP := registerTestRawB64Ctx(t, true, bgpctx.DirectionRecv, map[family.Family]capability.AddPathMode{
		family.IPv4Unicast: capability.AddPathReceive,
		family.IPv6Unicast: capability.AddPathBoth,
	})
	ctxExtraAP := registerTestRawB64Ctx(t, true, bgpctx.DirectionSend, map[family.Family]capability.AddPathMode{
		family.IPv4Unicast:   capability.AddPathSend,
		family.IPv6Multicast: capability.AddPathSend,
		family.IPv4Multicast: capability.AddPathBoth,
		// Receive-only must NOT appear in a DirectionSend context.
		family.IPv6Unicast: capability.AddPathReceive,
	})

	t.Run("received_asn4_true", func(t *testing.T) {
		var wu wireu.WireUpdate
		wireu.InitWireUpdate(&wu, sampleUpdate, ctxASN4True)
		msg := bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   sampleUpdate,
			Direction:  rpc.DirectionReceived,
			WireUpdate: &wu,
		}
		got := parseTestRawB64Line(t, string(AppendMessage(nil, basePeer, msg, content)))
		assert.Equal(t, rawB64FlagHasASN4|rawB64FlagASN4|rawB64FlagHasRemoteAS, got.flags)
		assert.Equal(t, uint32(65000), got.localAS)
		assert.Equal(t, uint32(65001), got.remoteAS)
		assert.Equal(t, uint64(42), got.connID)
		assert.Equal(t, testPeerIP, got.peer)
		assert.Equal(t, testLocalIP, got.local)
		assert.Equal(t, testRouterIP, got.routerID)
		assert.Empty(t, got.extraAddPath)
		assert.Equal(t, sampleUpdate, got.decodedBody)
	})

	t.Run("sent_via_AppendSentMessage_overrides_direction", func(t *testing.T) {
		var wu wireu.WireUpdate
		wireu.InitWireUpdate(&wu, sampleUpdate, ctxASN4True)
		msg := bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   sampleUpdate,
			Direction:  rpc.DirectionUnspecified,
			WireUpdate: &wu,
		}
		got := parseTestRawB64Line(t, string(AppendSentMessage(nil, basePeer, msg, content)))
		assert.Equal(t, rawB64FlagSent|rawB64FlagHasASN4|rawB64FlagASN4|rawB64FlagHasRemoteAS, got.flags)
		assert.Equal(t, sampleUpdate, got.decodedBody)
	})

	t.Run("asn4_false", func(t *testing.T) {
		var wu wireu.WireUpdate
		wireu.InitWireUpdate(&wu, sampleUpdate, ctxASN4False)
		msg := bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   sampleUpdate,
			Direction:  rpc.DirectionReceived,
			WireUpdate: &wu,
		}
		got := parseTestRawB64Line(t, string(AppendMessage(nil, basePeer, msg, content)))
		assert.Equal(t, rawB64FlagHasASN4|rawB64FlagHasRemoteAS, got.flags)
	})

	t.Run("unknown_encoding_context", func(t *testing.T) {
		var wu wireu.WireUpdate
		wireu.InitWireUpdate(&wu, sampleUpdate, bgpctx.ContextID(65535))
		msg := bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   sampleUpdate,
			Direction:  rpc.DirectionReceived,
			WireUpdate: &wu,
		}
		got := parseTestRawB64Line(t, string(AppendMessage(nil, basePeer, msg, content)))
		assert.Equal(t, rawB64FlagHasRemoteAS, got.flags)
		assert.Empty(t, got.extraAddPath)

		msgNoWire := bgptypes.RawMessage{
			Type:      msgtype.TypeUPDATE,
			RawBytes:  sampleUpdate,
			Direction: rpc.DirectionReceived,
		}
		gotNoWire := parseTestRawB64Line(t, string(AppendMessage(nil, basePeer, msgNoWire, content)))
		assert.Equal(t, rawB64FlagHasRemoteAS, gotNoWire.flags)
		assert.Empty(t, gotNoWire.extraAddPath)
	})

	t.Run("dynamic_peer_and_absent_fields", func(t *testing.T) {
		dynPeer := &plugin.PeerInfo{
			Address: netip.MustParseAddr("2001:db8::1"),
			LocalAS: 65100,
			// PeerAS zero = dynamic peer before OPEN negotiation.
			// LocalAddress invalid, RouterID 0, ConnectionID 0.
		}
		msg := bgptypes.RawMessage{
			Type:      msgtype.TypeUPDATE,
			RawBytes:  sampleUpdate,
			Direction: rpc.DirectionReceived,
		}
		got := parseTestRawB64Line(t, string(AppendMessage(nil, dynPeer, msg, content)))
		assert.Equal(t, uint8(0), got.flags)
		assert.Equal(t, uint32(65100), got.localAS)
		assert.Equal(t, uint32(0), got.remoteAS)
		assert.Equal(t, uint64(0), got.connID)
		assert.Equal(t, "2001:db8::1", got.peer)
		assert.Empty(t, got.local)
		assert.Empty(t, got.routerID)
		assert.Empty(t, got.extraAddPath)

		gotNilPeer := parseTestRawB64Line(t, string(AppendMessage(nil, nil, msg, content)))
		assert.Equal(t, uint8(0), gotNilPeer.flags)
		assert.Equal(t, uint32(0), gotNilPeer.localAS)
		assert.Equal(t, uint32(0), gotNilPeer.remoteAS)
		assert.Equal(t, uint64(0), gotNilPeer.connID)
		assert.Empty(t, gotNilPeer.peer)
		assert.Empty(t, gotNilPeer.local)
		assert.Empty(t, gotNilPeer.routerID)
	})

	t.Run("unicast_addpath_flag_bits", func(t *testing.T) {
		var wu wireu.WireUpdate
		wireu.InitWireUpdate(&wu, sampleUpdate, ctxUnicastAP)
		msg := bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   sampleUpdate,
			Direction:  rpc.DirectionReceived,
			WireUpdate: &wu,
		}
		got := parseTestRawB64Line(t, string(AppendMessage(nil, basePeer, msg, content)))
		wantFlags := rawB64FlagHasASN4 | rawB64FlagASN4 | rawB64FlagHasRemoteAS |
			rawB64FlagAddPathIPv4Unicast | rawB64FlagAddPathIPv6Unicast
		assert.Equal(t, wantFlags, got.flags)
		assert.Empty(t, got.extraAddPath)
	})

	t.Run("non_unicast_extra_addpath_sorted", func(t *testing.T) {
		var wu wireu.WireUpdate
		wireu.InitWireUpdate(&wu, sampleUpdate, ctxExtraAP)
		msg := bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   sampleUpdate,
			Direction:  rpc.DirectionSent,
			WireUpdate: &wu,
		}
		got := parseTestRawB64Line(t, string(AppendSentMessage(nil, basePeer, msg, content)))
		wantFlags := rawB64FlagSent | rawB64FlagHasASN4 | rawB64FlagASN4 |
			rawB64FlagHasRemoteAS | rawB64FlagAddPathIPv4Unicast
		assert.Equal(t, wantFlags, got.flags)
		assert.Equal(t, "ipv4/multicast,ipv6/multicast", got.extraAddPath)
	})

	t.Run("eor_ipv4_mp_and_empty_body", func(t *testing.T) {
		ipv4EOR := []byte{0x00, 0x00, 0x00, 0x00}
		var wuIPv4 wireu.WireUpdate
		wireu.InitWireUpdate(&wuIPv4, ipv4EOR, ctxASN4True)
		gotIPv4 := parseTestRawB64Line(t, string(AppendMessage(nil, basePeer, bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   ipv4EOR,
			Direction:  rpc.DirectionReceived,
			WireUpdate: &wuIPv4,
		}, content)))
		assert.Equal(t, ipv4EOR, gotIPv4.decodedBody)

		// IPv6 unicast MP_UNREACH_NLRI EOR: withdrawn=0, attrLen=6, attr 15 len 3 (AFI=2, SAFI=1).
		mpEOR := []byte{0x00, 0x00, 0x00, 0x06, 0x80, 0x0f, 0x03, 0x00, 0x02, 0x01}
		var wuMP wireu.WireUpdate
		wireu.InitWireUpdate(&wuMP, mpEOR, ctxASN4True)
		gotMP := parseTestRawB64Line(t, string(AppendMessage(nil, basePeer, bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   mpEOR,
			Direction:  rpc.DirectionReceived,
			WireUpdate: &wuMP,
		}, content)))
		assert.Equal(t, mpEOR, gotMP.decodedBody)

		gotEmpty := parseTestRawB64Line(t, string(AppendMessage(nil, basePeer, bgptypes.RawMessage{
			Type:      msgtype.TypeUPDATE,
			RawBytes:  nil,
			Direction: rpc.DirectionReceived,
		}, content)))
		assert.Empty(t, gotEmpty.b64Body)
		assert.Empty(t, gotEmpty.decodedBody)
	})

	t.Run("malformed_update_body_emitted_verbatim", func(t *testing.T) {
		// Truncated path attribute length (claims 0x00ff bytes, only 1 follows);
		// ApplyToUpdate would reject it, but base64 must emit exact raw bytes.
		malformed := []byte{0x00, 0x00, 0x00, 0xff, 0x40}
		msg := bgptypes.RawMessage{
			Type:      msgtype.TypeUPDATE,
			RawBytes:  malformed,
			Direction: rpc.DirectionReceived,
		}
		var scratch [4096]byte
		got := parseTestRawB64Line(t, string(AppendMessage(scratch[:0], basePeer, msg, content)))
		assert.Equal(t, malformed, got.decodedBody)
	})
}
