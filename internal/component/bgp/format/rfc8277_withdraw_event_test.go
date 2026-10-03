// Design: docs/architecture/wire/nlri.md -- withdrawal framing of a labeled route
// RFC: rfc/short/rfc8277.md -- RFC8277-2.4-1, the Compatibility field on receipt

package format

import (
	"net/netip"
	"strings"
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
)

var labeledIPv4 = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}

// labeledWithdrawEvent renders the JSON event for a received UPDATE whose only
// attribute is an MP_UNREACH_NLRI withdrawing 10.0.0.0/8 in ipv4/mpls-label,
// with compat in the Compatibility field and pathID in front when addPath.
func labeledWithdrawEvent(t *testing.T, compat [3]byte, addPath bool, pathID byte) string {
	t.Helper()

	// RFC 8277 Section 2.4: the withdrawn NLRI is [Length][Compatibility][Prefix],
	// so 10.0.0.0/8 has Length 24 + 8 = 32.
	var withdrawn []byte
	if addPath {
		withdrawn = append(withdrawn, 0, 0, 0, pathID)
	}
	withdrawn = append(withdrawn, 32, compat[0], compat[1], compat[2], 10)
	value := append([]byte{0, 1, 4}, withdrawn...) // AFI 1, SAFI 4
	attrs := append([]byte{0x80, 15, byte(len(value))}, value...)
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)

	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{labeledIPv4: addPath})
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wu.Attrs()
	if err != nil {
		t.Fatalf("attributes: %v", err)
	}
	msg := bgptypes.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   body,
		AttrsWire:  attrsWire,
		WireUpdate: wu,
	}
	peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001}
	content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatParsed}
	return string(AppendMessage(nil, &peer, msg, content))
}

// TestLabeledWithdrawalEventNamesThePrefix renders the JSON event a plugin
// receives for a labeled unicast withdrawal carrying the Compatibility value
// RFC 8277 Section 2.4 says a sender SHOULD use, with and without ADD-PATH.
//
// VALIDATES: the event's del operation for ipv4/mpls-label names 10.0.0.0/8,
// and under ADD-PATH it carries the path identifier with it.
// PREVENTS: MPUnreachWire.NLRIs framing the withdrawal with the announcement
// reader, which walks 0x800000 as a label stack entry whose S bit is clear,
// runs past the NLRI, and leaves the event with no withdrawal at all.
//
// RFC requirement: RFC8277-2.4-1 positive -- the JSON event for an ipv4/mpls-label withdrawal whose Compatibility field is 0x800000 names 10.0.0.0/8 in its del operation, with and without an ADD-PATH identifier.
func TestLabeledWithdrawalEventNamesThePrefix(t *testing.T) {
	t.Parallel()

	compat := [3]byte{0x80, 0x00, 0x00}
	plain := labeledWithdrawEvent(t, compat, false, 0)
	if want := `"ipv4/mpls-label":[{"action":"del","nlri":["10.0.0.0/8"]}]`; !strings.Contains(plain, want) {
		t.Fatalf("event lacks %s:\n%s", want, plain)
	}
	withPath := labeledWithdrawEvent(t, compat, true, 7)
	if want := `"ipv4/mpls-label":[{"action":"del","nlri":[{"prefix":"10.0.0.0/8","path-id":7}]}]`; !strings.Contains(withPath, want) {
		t.Fatalf("ADD-PATH event lacks %s:\n%s", want, withPath)
	}
}

// TestLabeledWithdrawalEventIgnoresOtherCompatibilityValues renders the same
// event for Compatibility values a sender SHOULD NOT use: zero, and 0x000641,
// which is label 100 with the S bit set and so reads as a whole label stack.
//
// VALIDATES: each event names 10.0.0.0/8 alone. The value is not rejected, and
// it is not read as a label, so it changes nothing in the event.
// PREVENTS: a withdrawal the announcement reader happens to frame, because its
// Compatibility field has the S bit set, being reported with that field as a
// label, and one it cannot frame being dropped.
//
// RFC requirement: RFC8277-2.4-1 negative -- withdrawals whose Compatibility field is 0x000000, or 0x000641 (label 100 with S set), produce the same del operation naming 10.0.0.0/8 and nothing else.
func TestLabeledWithdrawalEventIgnoresOtherCompatibilityValues(t *testing.T) {
	t.Parallel()

	want := `"ipv4/mpls-label":[{"action":"del","nlri":["10.0.0.0/8"]}]`
	for _, compat := range [][3]byte{{0x00, 0x00, 0x00}, {0x00, 0x06, 0x41}} {
		event := labeledWithdrawEvent(t, compat, false, 0)
		if !strings.Contains(event, want) {
			t.Fatalf("Compatibility %x: event lacks %s:\n%s", compat, want, event)
		}
	}
}
