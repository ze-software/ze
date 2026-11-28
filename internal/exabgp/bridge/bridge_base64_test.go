// Design: docs/architecture/exabgp-bridge.md -- unit tests for base64 UPDATE parsing, session cache, and fallback rendering.
// Related: bridge_base64.go -- ReadBase64Update, resolveRawContextID, RenderRawUpdate.
// Related: bridge_negotiated.go -- Sessions.noteRaw and pre-serialized negotiated JSON bytes.

//go:build ze_bgp

package bridge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

func buildTestBase64Line(flags uint8, localAS, remoteAS uint32, connID uint64, peer, local, routerID, extraAddPath string, body []byte) string {
	b64Body := base64.StdEncoding.EncodeToString(body)
	var b strings.Builder
	b.WriteString(base64KindPrefix)
	b.WriteString(strconv.FormatUint(uint64(flags), 10))
	b.WriteByte(' ')
	b.WriteString(strconv.FormatUint(uint64(localAS), 10))
	b.WriteByte(' ')
	b.WriteString(strconv.FormatUint(uint64(remoteAS), 10))
	b.WriteByte(' ')
	b.WriteString(strconv.FormatUint(connID, 10))
	for _, field := range []string{peer, local, routerID, extraAddPath} {
		b.WriteByte(' ')
		if field == "" {
			b.WriteString(base64AbsentToken)
		} else {
			b.WriteString(field)
		}
	}
	b.WriteByte(' ')
	b.WriteString(strconv.Itoa(len(b64Body)))
	b.WriteByte(':')
	b.WriteString(b64Body)
	return b.String()
}

// TestReadBase64Update_RoundTripAndMalformed verifies positional space-delimited
// `base64` UPDATE parsing across valid inputs, RFC 8654 jumbo bodies up to
// 65,516 bytes, scratch buffer reuse/spilling, and strict rejection of every
// malformed boundary condition.
//
// VALIDATES: AC-1, AC-9 -- exact round-trip of all fields and flag bits, RFC
// 8654 65,516-byte jumbo UPDATE support, and zero-panic rejection of bad kind,
// bad numbers, unknown flags, AS overflow, empty tokens, embedded control
// bytes, missing spaces, trailing bytes, invalid base64, and > 65,516-byte
// decoded bodies.
// PREVENTS: slice panics on hostile inputs, silent truncation of jumbo RFC
// 8654 UPDATEs, or accepting unknown flag bits / trailing garbage.
func TestReadBase64Update_RoundTripAndMalformed(t *testing.T) {
	t.Run("round_trip_all_fields_and_flags", func(t *testing.T) {
		body := []byte{
			0x00, 0x00, 0x00, 0x14,
			0x40, 0x01, 0x01, 0x00,
			0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9,
			0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01,
			0x18, 0xc0, 0x00, 0x02,
		}
		line := buildTestBase64Line(
			base64FlagsMax,
			math.MaxUint32,
			math.MaxUint32,
			math.MaxUint64,
			"2001:db8::1",
			"2001:db8::2",
			"192.0.2.1",
			"ipv4/multicast,ipv6/multicast",
			body,
		) + "\n"

		var scratch [4096]byte
		got, err := ReadBase64Update(line, scratch[:0])
		require.NoError(t, err)
		assert.Equal(t, base64FlagsMax, got.Flags)
		assert.True(t, got.isSent())
		assert.True(t, got.hasASN4())
		assert.True(t, got.asn4())
		assert.True(t, got.hasRemoteAS())
		assert.True(t, got.addPathIPv4Unicast())
		assert.True(t, got.addPathIPv6Unicast())
		assert.Equal(t, modeSend, got.Direction())
		assert.Equal(t, uint32(math.MaxUint32), got.LocalAS)
		assert.Equal(t, uint32(math.MaxUint32), got.RemoteAS)
		assert.Equal(t, uint64(math.MaxUint64), got.ConnID)
		assert.Equal(t, "2001:db8::1", got.Peer)
		assert.Equal(t, "2001:db8::2", got.Local)
		assert.Equal(t, "192.0.2.1", got.RouterID)
		assert.Equal(t, "ipv4/multicast,ipv6/multicast", got.ExtraAddPath)
		assert.Equal(t, body, got.Body)
		// Decoded body must reuse the caller's scratch backing array.
		require.NotEmpty(t, got.Body)
		assert.Equal(t, &scratch[0], &got.Body[0])
	})

	t.Run("zero_flags_and_empty_body_without_newline", func(t *testing.T) {
		line := "upd 0 0 0 0 - - - - 0:"
		got, err := ReadBase64Update(line, nil)
		require.NoError(t, err)
		assert.Equal(t, uint8(0), got.Flags)
		assert.False(t, got.isSent())
		assert.False(t, got.hasASN4())
		assert.False(t, got.asn4())
		assert.False(t, got.hasRemoteAS())
		assert.False(t, got.addPathIPv4Unicast())
		assert.False(t, got.addPathIPv6Unicast())
		assert.Equal(t, modeReceive, got.Direction())
		assert.Empty(t, got.Peer)
		assert.Empty(t, got.Local)
		assert.Empty(t, got.RouterID)
		assert.Empty(t, got.ExtraAddPath)
		assert.Nil(t, got.Body)
	})

	t.Run("rfc8654_jumbo_max_65516_spills_past_scratch", func(t *testing.T) {
		jumbo := bytes.Repeat([]byte{0xab}, rawUpdateBodyMax)
		line := buildTestBase64Line(14, 65000, 65001, 7, "10.0.0.1", "10.0.0.2", "1.1.1.1", "", jumbo)

		var scratch [4096]byte
		got, err := ReadBase64Update(line, scratch[:0])
		require.NoError(t, err)
		assert.Equal(t, rawUpdateBodyMax, len(got.Body))
		assert.Equal(t, jumbo, got.Body)
	})

	t.Run("malformed_lines_rejected", func(t *testing.T) {
		overOne := bytes.Repeat([]byte{0xcd}, rawUpdateBodyMax+1)
		overOneLine := buildTestBase64Line(14, 65000, 65001, 1, "10.0.0.1", "10.0.0.2", "1.1.1.1", "", overOne)

		overEncoded := bytes.Repeat([]byte{0xef}, rawUpdateBodyMax+4)
		overEncodedLine := buildTestBase64Line(14, 65000, 65001, 1, "10.0.0.1", "10.0.0.2", "1.1.1.1", "", overEncoded)

		badCases := []struct {
			name string
			line string
		}{
			{"empty", ""},
			{"wrong_kind", "xxx 0 0 0 0 - - - - 0:"},
			{"short_prefix", "upd"},
			{"tab_prefix", "upd\t0 0 0 0 - - - - 0:"},
			{"crlf_ending", "upd 0 0 0 0 - - - - 0:\r\n"},
			{"bare_cr", "upd 0 0 0 0 - - - - 0:\r"},
			{"double_newline", "upd 0 0 0 0 - - - - 0:\n\n"},
			{"empty_number", "upd  0 0 0 - - - - 0:"},
			{"negative_number", "upd -1 0 0 0 - - - - 0:"},
			{"non_digit_number", "upd 1a 0 0 0 - - - - 0:"},
			{"colon_in_number", "upd 1: 0 0 0 - - - - 0:"},
			{"flags_64", "upd 64 0 0 0 - - - - 0:"},
			{"flags_255", "upd 255 0 0 0 - - - - 0:"},
			{"local_as_overflow", "upd 0 4294967296 0 0 - - - - 0:"},
			{"remote_as_overflow", "upd 0 0 4294967296 0 - - - - 0:"},
			{"conn_id_overflow", "upd 0 0 0 18446744073709551616 - - - - 0:"},
			{"missing_fields", "upd 0 0 0 0 - - -"},
			{"empty_peer_token", "upd 0 0 0 0  - - - 0:"},
			{"tab_in_peer", "upd 0 0 0 0 10.0.0.1\t - - - 0:"},
			{"missing_colon_body", "upd 0 0 0 0 - - - - 0"},
			{"empty_count_body", "upd 0 0 0 0 - - - - :"},
			{"negative_count_body", "upd 0 0 0 0 - - - - -1:"},
			{"out_of_bounds_body", "upd 0 0 0 0 - - - - 999999999999999999:ab"},
			{"trailing_space", "upd 0 0 0 0 - - - - 0: "},
			{"trailing_bytes", "upd 0 0 0 0 - - - - 0:extra"},
			{"invalid_base64_chars", "upd 0 0 0 0 - - - - 4:!!!!"},
			{"invalid_base64_len", "upd 0 0 0 0 - - - - 3:AAA"},
			{"base64_embedded_cr", "upd 0 0 0 0 - - - - 5:AA\rAA"},
			{"base64_embedded_lf", "upd 0 0 0 0 - - - - 5:AA\nAA"},
			{"decoded_body_65517", overOneLine},
			{"encoded_body_too_long", overEncodedLine},
		}

		for _, tc := range badCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := ReadBase64Update(tc.line, nil)
				assert.Error(t, err, "expected error for malformed line %q", tc.name)
			})
		}
	})
}

// TestSessionsNoteRaw_ReconnectAndNoNegotiated verifies Q-E4 reconnect
// invalidation on conn-id mismatch without `bgp:state`, pre-serialized
// ExaBGP-spaced negotiated JSON bytes, backward-compatible `conn-id == 0`
// preservation, and AS_PATH/ADD-PATH decoding when `bgp:negotiated` is absent.
//
// VALIDATES: AC-8 -- Sessions.noteRaw reconnect invalidation, cached negotiated
// JSON equality, conn-id == 0 preservation, and RenderRawUpdate decoding for
// 2-byte ASN, ADD-PATH, dynamic peer-as auto, EOR, sent direction, and forms.
// PREVENTS: stale negotiated blocks surviving reconnect without bgp:state, or
// corrupted AS_PATH/ADD-PATH decoding when bgp:negotiated is excluded.
func TestSessionsNoteRaw_ReconnectAndNoNegotiated(t *testing.T) {
	const testPeerIP = "10.0.0.1"
	const testLocalIP = "10.0.0.2"

	var sessions Sessions
	negotiatedEvent := ReadEvent(map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message":       map[string]any{"type": "negotiated"},
			"connection-id": "42",
			"peer":          map[string]any{"remote": map[string]any{"address": testPeerIP, "as": float64(65001)}},
			"negotiated": map[string]any{
				"timer":         map[string]any{"hold-time": float64(180)},
				"message-size":  float64(65535),
				"route-refresh": "enhanced",
				"asn4":          false,
				"families":      []any{"ipv4/unicast"},
			},
		},
	})
	sessions.Note(&negotiatedEvent)

	// Matching connID returns the cached block, exact pre-serialized JSON, and cached asn4=false.
	gotNeg, gotJSON, gotASN4 := sessions.noteRaw(testPeerIP, 42, false, false)
	assert.Equal(t, float64(180), gotNeg["hold_time"])
	assert.False(t, gotASN4)
	wantJSON, err := marshalExabgpJSON(gotNeg)
	require.NoError(t, err)
	assert.Equal(t, wantJSON, gotJSON)

	// Synthetic connID == 0 preserves the cached session entry (AC-8).
	zeroNeg, zeroJSON, zeroASN4 := sessions.noteRaw(testPeerIP, 0, false, false)
	assert.Equal(t, float64(180), zeroNeg["hold_time"])
	assert.Equal(t, wantJSON, zeroJSON)
	assert.False(t, zeroASN4)

	// Changed non-zero connID invalidates the stale session immediately without bgp:state (Q-E4).
	reconnNeg, reconnJSON, reconnASN4 := sessions.noteRaw(testPeerIP, 43, true, true)
	assert.Equal(t, float64(0), reconnNeg["hold_time"])
	assert.True(t, reconnASN4)
	wantUnexchanged, err := marshalExabgpJSON(unexchangedNegotiated())
	require.NoError(t, err)
	assert.Equal(t, wantUnexchanged, reconnJSON)

	// Confirm the stale entry stays deleted on subsequent lookups.
	afterNeg, afterJSON, afterASN4 := sessions.noteRaw(testPeerIP, 43, false, false)
	assert.Equal(t, float64(0), afterNeg["hold_time"])
	assert.Equal(t, wantUnexchanged, afterJSON)
	assert.True(t, afterASN4)

	// Q-E4: bgp:negotiated excluded -- decode 2-byte AS_PATH + IPv4 unicast ADD-PATH
	// solely from base64 flags on a fresh Sessions with no negotiated event.
	t.Run("no_negotiated_2byte_aspath_and_addpath_and_dynamic_peer", func(t *testing.T) {
		var fresh Sessions
		// 2-byte AS_PATH [65001] (0xfde9), ORIGIN IGP, NEXT_HOP 10.0.0.1,
		// ADD-PATH NLRI: path-id 0x01020304 (1.2.3.4) + 192.0.2.0/24.
		wireBody := []byte{
			0x00, 0x00, // Withdrawn Length = 0
			0x00, 0x12, // Path Attributes Length = 18
			0x40, 0x01, 0x01, 0x00, // ORIGIN: IGP (4B)
			0x40, 0x02, 0x04, 0x02, 0x01, 0xfd, 0xe9, // AS_PATH 2-byte: AS_SEQUENCE [65001] (7B)
			0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP: 10.0.0.1 (7B)
			0x01, 0x02, 0x03, 0x04, 0x18, 0xc0, 0x00, 0x02, // ADD-PATH id=1.2.3.4, 192.0.2.0/24
		}
		// Flags: sent(1) | hasASN4(2) | asn4=false(0) | hasRemoteAS=false(0) | addPathIPv4Unicast(16).
		flags := base64FlagSent | base64FlagHasASN4 | base64FlagAddPathIPv4Unicast
		line := buildTestBase64Line(flags, 65002, 0, 99, testPeerIP, testLocalIP, "1.1.1.1", "", wireBody)
		raw, parseErr := ReadBase64Update(line, nil)
		require.NoError(t, parseErr)

		log := slog.New(slog.DiscardHandler)
		rendered := fresh.RenderRawUpdate(raw, FormatV6JSON, FormConsolidated, false, log)
		var doc map[string]any
		require.NoError(t, json.Unmarshal(rendered, &doc), "invalid JSON: %s", rendered)

		assert.NotEmpty(t, doc["header"], "consolidate form must attach reconstructed header")
		assert.NotEmpty(t, doc["body"], "consolidate form must attach hex body")

		neighbor, ok := doc["neighbor"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, modeSend, neighbor["direction"])
		assert.Equal(t, "1.1.1.1", neighbor["router-id"])
		_, hasNeg := neighbor["negotiated"]
		assert.False(t, hasNeg, "grantNegotiated=false must omit neighbor.negotiated")

		asnObj, ok := neighbor["asn"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(0), asnObj["peer"], "dynamic peer (!hasRemoteAS) must render asn.peer == 0")

		msgObj, ok := neighbor["message"].(map[string]any)
		require.True(t, ok)
		updObj, ok := msgObj["update"].(map[string]any)
		require.True(t, ok)
		attrsObj, ok := updObj["attribute"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, map[string]any{
			"0": map[string]any{"element": "as-sequence", "value": []any{float64(65001)}},
		}, attrsObj["as-path"])

		announce, ok := updObj["announce"].(map[string]any)
		require.True(t, ok)
		ipv4Map, ok := announce["ipv4 unicast"].(map[string]any)
		require.True(t, ok)
		nhList, ok := ipv4Map[testPeerIP].([]any)
		require.True(t, ok)
		require.Len(t, nhList, 1)
		entry, ok := nhList[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "192.0.2.0/24", entry["nlri"])
		assert.Equal(t, "1.2.3.4", entry["path-information"])

		// With grantNegotiated=true on a peer without a negotiated event, unexchangedNegotiated attaches.
		renderedGranted := fresh.RenderRawUpdate(raw, FormatV6JSON, FormParsed, true, log)
		var grantedDoc map[string]any
		require.NoError(t, json.Unmarshal(renderedGranted, &grantedDoc))
		grantedNeighbor, _ := grantedDoc["neighbor"].(map[string]any)
		grantedNeg, hasGrantedNeg := grantedNeighbor["negotiated"].(map[string]any)
		require.True(t, hasGrantedNeg, "grantNegotiated=true must attach unexchanged negotiated block")
		assert.Equal(t, float64(0), grantedNeg["hold_time"])
	})

	t.Run("eor_and_context_cache_reuse", func(t *testing.T) {
		var fresh Sessions
		log := slog.New(slog.DiscardHandler)

		// IPv4 unicast EOR (4 zero bytes).
		ipv4EOR := []byte{0x00, 0x00, 0x00, 0x00}
		lineV4 := buildTestBase64Line(base64FlagHasASN4|base64FlagASN4|base64FlagHasRemoteAS, 65002, 65001, 1, testPeerIP, testLocalIP, "", "", ipv4EOR)
		rawV4, parseErr := ReadBase64Update(lineV4, nil)
		require.NoError(t, parseErr)
		renderedV4 := fresh.RenderRawUpdate(rawV4, FormatV6JSON, FormParsed, false, log)
		var docV4 map[string]any
		require.NoError(t, json.Unmarshal(renderedV4, &docV4))
		neighborV4, ok := docV4["neighbor"].(map[string]any)
		require.True(t, ok)
		msgV4, ok := neighborV4["message"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, map[string]any{"afi": "ipv4", "safi": "unicast"}, msgV4["eor"])

		// IPv6 unicast MP_UNREACH EOR.
		mpEOR := []byte{0x00, 0x00, 0x00, 0x06, 0x80, 0x0f, 0x03, 0x00, 0x02, 0x01}
		lineV6 := buildTestBase64Line(base64FlagHasASN4|base64FlagASN4|base64FlagHasRemoteAS, 65002, 65001, 1, testPeerIP, testLocalIP, "", "", mpEOR)
		rawV6, parseErr := ReadBase64Update(lineV6, nil)
		require.NoError(t, parseErr)
		renderedV6 := fresh.RenderRawUpdate(rawV6, FormatV6JSON, FormParsed, false, log)
		var docV6 map[string]any
		require.NoError(t, json.Unmarshal(renderedV6, &docV6))
		neighborV6, ok := docV6["neighbor"].(map[string]any)
		require.True(t, ok)
		msgV6, ok := neighborV6["message"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, map[string]any{"afi": "ipv6", "safi": "unicast"}, msgV6["eor"])

		// Context cache reuse: repeated resolves must not register new ContextIDs.
		firstID, ctxErr := resolveRawContextID(true, true, false, "ipv4/multicast")
		require.NoError(t, ctxErr)
		countAfterFirst := bgpctx.Registry.Count()
		for range 100 {
			againID, againErr := resolveRawContextID(true, true, false, "ipv4/multicast")
			require.NoError(t, againErr)
			assert.Equal(t, firstID, againID)
		}
		assert.Equal(t, countAfterFirst, bgpctx.Registry.Count(), "cached ContextID must be reused without new Registry entries")
	})
}

// TestReadBase64Update_ZeroAlloc verifies that parsing a positional
// space-delimited `base64` UPDATE line with a caller-owned 4096-byte stack
// scratch buffer performs zero heap allocations per call.
//
// VALIDATES: textbuf.Buffer-backed ReadBase64Update allocates 0.0 times per
// call when the decoded UPDATE body fits in the caller's scratch buffer.
// PREVENTS: regressions where textbuf.Buffer or base64 decoding escapes to the
// heap on the ExaBGP bridge UPDATE hot path.
func TestReadBase64Update_ZeroAlloc(t *testing.T) {
	body := []byte{
		0x00, 0x00, 0x00, 0x14,
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9,
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01,
		0x18, 0xc0, 0x00, 0x02,
	}
	line := buildTestBase64Line(
		base64FlagHasASN4|base64FlagASN4|base64FlagHasRemoteAS,
		65000,
		65001,
		42,
		"192.0.2.10",
		"192.0.2.20",
		"192.0.2.30",
		"",
		body,
	) + "\n"

	var scratch [4096]byte
	var sink uint64
	allocs := testing.AllocsPerRun(100, func() {
		got, err := ReadBase64Update(line, scratch[:0])
		if err != nil {
			t.FailNow()
		}
		sink = got.ConnID + uint64(len(got.Body)+len(got.Peer)+len(got.Local)+len(got.RouterID))
	})
	assert.Equal(t, 0.0, allocs, "ReadBase64Update with stack scratch must allocate 0 times per call")
	assert.Positive(t, sink)
}

// FuzzReadBase64Update verifies ReadBase64Update never panics on arbitrary
// attacker-supplied or corrupted wire lines, and enforces the RFC 8654 body
// size bound and flag mask on every accepted line.
func FuzzReadBase64Update(f *testing.F) {
	f.Add("upd 15 65001 65002 1 10.0.0.1 10.0.0.2 1.1.1.1 - 8:AAAAAA==\n")
	f.Add("upd 63 4294967295 4294967295 999 2001:db8::1 2001:db8::2 - ipv4/multicast 0:\n")
	f.Add("upd 64 1 2 3 a - - - 0:\n")
	f.Add("upd 0 1 2 3 10.0.0.1 - - - 999999999:short\n")
	f.Add("bad line\r\n")
	f.Add("")

	f.Fuzz(func(t *testing.T, line string) {
		var scratch [4096]byte
		got, err := ReadBase64Update(line, scratch[:0])
		if err != nil {
			return
		}
		if got.Flags > base64FlagsMax {
			t.Fatalf("accepted unknown flags: %d", got.Flags)
		}
		if len(got.Body) > rawUpdateBodyMax {
			t.Fatalf("accepted body over RFC 8654 maximum: %d", len(got.Body))
		}
		if got.Direction() != modeReceive && got.Direction() != modeSend {
			t.Fatalf("unexpected direction: %q", got.Direction())
		}
	})
}
