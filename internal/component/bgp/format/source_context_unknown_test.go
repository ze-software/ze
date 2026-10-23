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

// addPathUpdateEvent renders, as parsed JSON, an UPDATE announcing 10.0.0.0/24
// with Path Identifier 1 in its legacy NLRI field, under context id ctxID.
func addPathUpdateEvent(t *testing.T, ctxID bgpctx.ContextID) string {
	t.Helper()

	attrs := []byte{
		0x40, 1, 1, 0, // ORIGIN IGP
		0x40, 2, 0, // AS_PATH empty
		0x40, 3, 4, 192, 0, 2, 1, // NEXT_HOP 192.0.2.1
	}
	nlri := []byte{0, 0, 0, 1, 24, 10, 0, 0} // RFC 7911 Section 3: Path Identifier, then the prefix
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
	body = append(body, nlri...)

	wu := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wu.Attrs()
	if err != nil {
		t.Fatalf("attributes: %v", err)
	}
	msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: body, AttrsWire: attrsWire, WireUpdate: wu}
	peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001}
	content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatParsed}
	return string(AppendMessage(nil, &peer, msg, content))
}

// TestUnknownSourceContextReadsNoRoute renders one ADD-PATH UPDATE under its
// registered context and under a context id the registry does not hold.
//
// VALIDATES: under the registered context the event names 10.0.0.0/24; under
// the unknown id it carries no route at all.
// PREVENTS: the unknown id reading as "no ADD-PATH", which walks the Path
// Identifier as a prefix and announces a route the peer never sent.
func TestUnknownSourceContextReadsNoRoute(t *testing.T) {
	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	id, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	if known := addPathUpdateEvent(t, id); !strings.Contains(known, "10.0.0.0/24") {
		t.Fatalf("registered context: event names no 10.0.0.0/24: %s", known)
	}

	unknownID := ^bgpctx.ContextID(0)
	for bgpctx.Registry.Get(unknownID) != nil {
		unknownID--
	}
	unknown := addPathUpdateEvent(t, unknownID)
	if !strings.Contains(unknown, `"nlri":{}`) {
		t.Fatalf("unknown context: event reads routes: %s", unknown)
	}
}
