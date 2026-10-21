// Design: docs/architecture/api/json-format.md -- the del operation of an UPDATE event
// RFC: rfc/short/rfc7911.md -- RFC7911-3-1, the Path Identifier in withdrawn routes

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

// withdrawOnlyAddPathEvent renders the event for a received UPDATE that holds
// only the Withdrawn Routes field, withdrawing 10.0.0.0/8 with Path Identifier
// 7 from a peer that negotiated ADD-PATH receive for ipv4/unicast. The message
// is built the way the reactor builds it: AttrsWire is nil because an UPDATE
// with a zero Total Path Attribute Length has no attribute section.
func withdrawOnlyAddPathEvent(t *testing.T, content bgptypes.ContentConfig) string {
	t.Helper()

	// RFC 7911 Section 3: Path Identifier (4 octets), Length (1 octet), Prefix.
	withdrawn := []byte{0, 0, 0, 7, 8, 10}
	body := append([]byte{0, byte(len(withdrawn))}, withdrawn...)
	body = append(body, 0, 0) // Total Path Attribute Length 0, no NLRI.

	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wu.Attrs()
	if err != nil {
		t.Fatalf("attributes: %v", err)
	}
	if attrsWire != nil {
		t.Fatalf("a withdraw-only UPDATE has no attribute section, got AttrsWire %v", attrsWire)
	}
	msg := bgptypes.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   body,
		WireUpdate: wu,
	}
	peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001}
	return string(AppendMessage(nil, &peer, msg, content))
}

// TestWithdrawOnlyUpdateKeepsPathIdentifier renders a withdraw-only UPDATE from
// an ADD-PATH peer in parsed JSON and in text.
//
// VALIDATES: the del operation names 10.0.0.0/8 with path-id 7 in JSON, and the
// text form withdraws 10.0.0.0/8 and nothing else.
// PREVENTS: the formatter reading the ADD-PATH context from AttrsWire, which a
// withdraw-only UPDATE does not have, so the withdrawn routes were walked
// without their Path Identifier: 0.0.0.7 read as a prefix length of zero, and
// the event carried no withdrawal of 10.0.0.0/8 at all.
//
// RFC requirement: RFC7911-3-1 positive -- the event for a withdraw-only UPDATE from an ADD-PATH ipv4/unicast peer names 10.0.0.0/8 with its Path Identifier 7 in the del operation in JSON, and the text event withdraws 10.0.0.0/8 alone.
func TestWithdrawOnlyUpdateKeepsPathIdentifier(t *testing.T) {
	t.Parallel()

	jsonEvent := withdrawOnlyAddPathEvent(t, bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatParsed})
	if want := `"ipv4/unicast":[{"action":"del","nlri":[{"prefix":"10.0.0.0/8","path-id":7}]}]`; !strings.Contains(jsonEvent, want) {
		t.Fatalf("JSON event lacks %s:\n%s", want, jsonEvent)
	}

	textEvent := withdrawOnlyAddPathEvent(t, bgptypes.ContentConfig{Encoding: plugin.EncodingText, Format: plugin.FormatParsed})
	// The text form prints an ipv4/unicast prefix alone, with or without ADD-PATH
	// (docs/architecture/api/text-format.md), so it proves the withdrawal was
	// framed with the Path Identifier by naming 10.0.0.0/8 and nothing else.
	if want := "nlri ipv4/unicast del 10.0.0.0/8"; !strings.HasSuffix(strings.TrimSpace(textEvent), want) {
		t.Fatalf("text event does not end with %q:\n%s", want, textEvent)
	}
}
