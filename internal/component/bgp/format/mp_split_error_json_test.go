// Design: docs/architecture/api/json-format.md -- the error convention of a family operation
// RFC: rfc/short/rfc4760.md -- MP_REACH_NLRI and MP_UNREACH_NLRI

package format

import (
	"encoding/json"
	"strings"
	"testing"

	bgpfilter "github.com/ze-software/ze/internal/component/bgp/filter"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Each MP attribute value below names a prefix longer than the octets it
// carries, so its NLRI field does not split into routes. The prefix octets hold
// hex letters, so the expected "raw" pins the uppercase spelling.
var (
	// AFI 2, SAFI 1, next hop length 16 (2001:db8::1), reserved, NLRI [64][FD AB].
	splitErrReachV6 = []byte{0, 2, 1, 16, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 64, 0xfd, 0xab}
	// AFI 2, SAFI 1, withdrawn [64][FD AB].
	splitErrUnreachV6 = []byte{0, 2, 1, 64, 0xfd, 0xab}
	// AFI 1, SAFI 1, next hop length 4 (192.0.2.1), reserved, NLRI [24][0A 0B].
	splitErrReachV4 = []byte{0, 1, 1, 4, 192, 0, 2, 1, 0, 24, 0x0a, 0x0b}
)

// mpSplitErrorJSON renders an UPDATE whose only attributes are ORIGIN and mp,
// one MP_REACH_NLRI or MP_UNREACH_NLRI, through the JSON fast path.
func mpSplitErrorJSON(t *testing.T, mp []byte) string {
	t.Helper()

	attrs := append([]byte{0x40, 1, 1, 0}, mp...)
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
	wireUpdate := wireu.NewWireUpdate(body, testEncodingContext())
	attrsWire, err := wireUpdate.Attrs()
	if err != nil {
		t.Fatalf("attributes: %v", err)
	}
	msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: body, AttrsWire: attrsWire, WireUpdate: wireUpdate}
	return string(appendParsedUpdateJSONDirect(nil, testPeer(), msg, nil, rpc.DirectionReceived, "update"))
}

// splitErrorCase is one family operation whose NLRI field does not split.
type splitErrorCase struct {
	name string
	// mp is the whole attribute (flags, code, length, value) for the fast path.
	mp []byte
	// result holds the same attribute value for the `format full` path.
	result bgpfilter.FilterResult
	// operation opens the expected family operation.
	operation string
	// raw is the expected "raw" member.
	raw string
}

func splitErrorCases() []splitErrorCase {
	return []splitErrorCase{
		{
			"ipv6_mp_reach", append([]byte{0x80, 14, byte(len(splitErrReachV6))}, splitErrReachV6...),
			bgpfilter.FilterResult{MPReach: []wireu.MPReachWire{splitErrReachV6}},
			`"ipv6/unicast":[{"next-hop":"2001:db8::1","action":"add",`, `"raw":"40FDAB"`,
		},
		{
			"ipv6_mp_unreach", append([]byte{0x80, 15, byte(len(splitErrUnreachV6))}, splitErrUnreachV6...),
			bgpfilter.FilterResult{MPUnreach: []wireu.MPUnreachWire{splitErrUnreachV6}},
			`"ipv6/unicast":[{"action":"del",`, `"raw":"40FDAB"`,
		},
		{
			"ipv4_mp_reach", append([]byte{0x80, 14, byte(len(splitErrReachV4))}, splitErrReachV4...),
			bgpfilter.FilterResult{MPReach: []wireu.MPReachWire{splitErrReachV4}},
			`"ipv4/unicast":[{"next-hop":"192.0.2.1","action":"add",`, `"raw":"180A0B"`,
		},
	}
}

// assertSplitErrorRendered fails unless out is JSON carrying the case's
// operation with the error convention and no empty nlri list.
func assertSplitErrorRendered(t *testing.T, out string, tt splitErrorCase) {
	t.Helper()

	var document map[string]any
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		t.Fatalf("the event is not JSON: %v\n%s", err, out)
	}
	for _, want := range []string{tt.operation, `"error":"`, `"parsed":false`, tt.raw} {
		if !strings.Contains(out, want) {
			t.Fatalf("the event lacks %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, `"nlri":[]`) {
		t.Fatalf("the event renders an empty nlri list for a field that did not split:\n%s", out)
	}
}

// TestMPNLRISplitErrorIsRendered renders, through the event fast path, an
// ipv6/unicast MP_REACH_NLRI and MP_UNREACH_NLRI and an ipv4/unicast
// MP_REACH_NLRI (the arm that merges with body NLRI) whose NLRI field does not
// split into routes.
//
// VALIDATES: each family operation carries "error", "parsed":false and the
// field's bytes as uppercase "raw", and no "nlri" list.
// PREVENTS: the split error dropped by appendParsedUpdateJSONDirect: the
// MP_REACH arms rendered "nlri":[], an announce of no route, and the MP_UNREACH
// arm left the family out, so the withdrawal vanished from the event.
// RFC 4760 Section 3 defines the field the operation stands for: "A variable
// length field that lists NLRI for the feasible routes that are being
// advertised in this attribute".
func TestMPNLRISplitErrorIsRendered(t *testing.T) {
	t.Parallel()

	for _, tt := range splitErrorCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertSplitErrorRendered(t, mpSplitErrorJSON(t, tt.mp), tt)
		})
	}
}

// TestFilterResultSplitErrorIsRendered renders the same three fields through
// the `format full` path, appendFilterResultJSON over a FilterResult.
//
// VALIDATES: each family operation carries the same error convention as the
// fast path, so json-format.md holds for both producers.
// PREVENTS: AnnouncedByFamily and WithdrawnByFamily skipping a family whose
// NLRI did not split, with only a Debug log, so the family vanished from the
// event.
func TestFilterResultSplitErrorIsRendered(t *testing.T) {
	t.Parallel()

	for _, tt := range splitErrorCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := appendFilterResultJSON(nil, testPeer(), tt.result, 1, rpc.DirectionReceived, nil, "update", true)
			assertSplitErrorRendered(t, string(out), tt)
		})
	}
}
