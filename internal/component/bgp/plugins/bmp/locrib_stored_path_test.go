// Design: docs/guide/bmp.md -- Loc-RIB messages convey the selected route.
// Related: bmp_locrib.go -- buildLocRIBUpdateBody and handleBestChange.
package bmp

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// TestLocRIBBodyPreservesStoredASPath checks the authoritative stored segment
// bytes, including an empty path, rather than the flattened display ASPath.
func TestLocRIBBodyPreservesStoredASPath(t *testing.T) {
	for _, current := range []struct {
		name string
		path []byte
	}{
		{name: "locally originated empty path", path: []byte{}},
		{name: "sequence with four octet ASN", path: []byte{2, 1, 0xfa, 0x56, 0xea, 0x01}},
		{name: "set between sequences", path: []byte{
			2, 1, 0, 0, 0xfd, 0xe9,
			1, 2, 0xfa, 0x56, 0xea, 0x01, 0, 0, 0xfd, 0xea,
			2, 1, 0, 0, 0xfd, 0xeb,
		}},
	} {
		t.Run(current.name, func(t *testing.T) {
			entry := storedPathEntry(current.path)
			// The summary is intentionally stale. It cannot replace the stored
			// path that the RIB selected, even for a locally originated route.
			entry.ASPath = []uint32{64512}
			body := mustLocRIBBody(t, family.IPv4Unicast, &entry)
			sections, err := wire.ParseUpdateSections(body)
			if err != nil {
				t.Fatal(err)
			}
			attrs := sections.Attrs(body)
			if count := countAttr(attrs, attrASPATH); count != 1 {
				t.Fatalf("AS_PATH count=%d, want one: %x", count, attrs)
			}
			if got := findAttr(attrs, attrASPATH); !bytes.Equal(got, current.path) {
				t.Fatalf("AS_PATH=%x, want stored segments %x", got, current.path)
			}
		})
	}
}

// TestLocRIBCollectorPreservesStoredEmptyASPath drives the normal best-change
// fan-out and sender queue, then reads the actual collector's BGP UPDATE.
func TestLocRIBCollectorPreservesStoredEmptyASPath(t *testing.T) {
	conn := newRecordingConn()
	sender := newTestSession(t, "stored-empty-path", conn)
	sender.locRIBUpSent.Store(true)
	bp := &BMPPlugin{
		senders:  []*senderSession{sender},
		identity: &localIdentity{asn: 65000, routerID: 0x01020305},
	}
	entry := storedPathEntry(nil)
	entry.ASPath = nil
	bp.handleBestChange(&ribevents.BestChangeBatch{
		Protocol: "bgp", Family: family.IPv4Unicast,
		Changes: []ribevents.BestChangeEntry{entry},
	})
	waitQueueDrained(t, sender)
	messages := decodeBMPStream(t, conn.written())
	if len(messages) != 1 {
		t.Fatalf("collector received %d messages, want one Route Monitoring", len(messages))
	}
	route, ok := messages[0].(*RouteMonitoring)
	if !ok {
		t.Fatalf("collector received %T, want Route Monitoring", messages[0])
	}
	if len(route.BGPUpdate) < message.HeaderLen {
		t.Fatalf("collector UPDATE is truncated: %x", route.BGPUpdate)
	}
	body := route.BGPUpdate[message.HeaderLen:]
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatal(err)
	}
	attrs := sections.Attrs(body)
	if count := countAttr(attrs, attrASPATH); count != 1 {
		t.Fatalf("collector AS_PATH count=%d, want one empty AS_PATH: %x", count, attrs)
	}
	if got := findAttr(attrs, attrASPATH); len(got) != 0 {
		t.Fatalf("collector AS_PATH=%x, want empty", got)
	}
	if got := sections.NLRI(body); !bytes.Equal(got, []byte{24, 10, 20, 30}) {
		t.Fatalf("collector NLRI=%x, want 10.20.30.0/24", got)
	}
}

// TestLocRIBMissingASPathIsRefused distinguishes absent stored identity from
// the valid, explicitly present empty AS_PATH checked above.
func TestLocRIBMissingASPathIsRefused(t *testing.T) {
	entry := storedPathEntry(nil)
	entry.Attributes = []byte{0x40, attrORIGIN, 1, 0}
	entry.ASPath = []uint32{65001}
	if body, err := buildLocRIBUpdateBody(family.IPv4Unicast, &entry); err == nil {
		t.Fatalf("missing stored AS_PATH encoded as %x, want refusal", body)
	}
}

// TestLocRIBMissingNextHopIsRefused prevents an announce from looking like a
// successful, attribute-only UPDATE that conveys none of the selected route.
func TestLocRIBMissingNextHopIsRefused(t *testing.T) {
	entry := storedPathEntry([]byte{2, 1, 0, 0, 0xfd, 0xe9})
	entry.NextHop = netip.Addr{}
	if body, err := buildLocRIBUpdateBody(family.IPv4Unicast, &entry); err == nil {
		t.Fatalf("missing next hop encoded as %x, want refusal", body)
	}
	entry.NextHop = netip.MustParseAddr("192.0.2.1")
	body := mustLocRIBBody(t, family.IPv4Unicast, &entry)
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatal(err)
	}
	if got := sections.NLRI(body); !bytes.Equal(got, []byte{24, 10, 20, 30}) {
		t.Fatalf("valid next-hop control NLRI=%x, want 10.20.30.0/24", got)
	}
}

// storedPathEntry carries a complete, four-octet stored path attribute set.
// Attribute bytes are flags(1), type(1), length(1), then the segment value.
func storedPathEntry(path []byte) ribevents.BestChangeEntry {
	attrs := []byte{0x40, attrORIGIN, 1, 0, 0x40, attrASPATH, byte(len(path))}
	attrs = append(attrs, path...)
	return ribevents.BestChangeEntry{
		Action:     ribevents.BestChangeAdd,
		Attributes: attrs,
		Prefix:     netip.MustParsePrefix("10.20.30.0/24"),
		NextHop:    netip.MustParseAddr("192.0.2.1"),
	}
}
