package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// TestRFC8669LabelIndexTransmissionClearsReservedAndFlags sends both clean and
// dirty Label-Index TLVs, on an IPv4 labeled-unicast route (the family RFC 8669
// Section 3.1 defines the TLV for), through the real session writers, including
// both relay planners followed by their raw-body writer. The source and Originator SRGB
// remain byte-identical; only the transmitted Label-Index fields are cleared.
// MUTATION: omit the final writer normalization and dirty relay octets escape.
// RFC requirement: RFC8669-3.1-3 positive -- clean Label-Index Reserved is zero on every tested session writer and both forwarding rails.
// RFC requirement: RFC8669-3.1-3 negative -- nonzero received Label-Index Reserved cannot escape either relay rail or raw, parsed and filter-overridden session writers.
// RFC requirement: RFC8669-3.1-5 positive -- clean Label-Index Flags remain zero on every tested session writer and both forwarding rails.
// RFC requirement: RFC8669-3.1-5 negative -- nonzero received Label-Index Flags cannot escape either relay rail or raw, parsed and filter-overridden session writers.
func TestRFC8669LabelIndexTransmissionClearsReservedAndFlags(t *testing.T) {
	for _, dirty := range []bool{false, true} {
		for _, rail := range []string{"raw", "packet", "parsed", "override", "general", "route-server"} {
			t.Run(rail+map[bool]string{false: "/clean", true: "/dirty"}[dirty], func(t *testing.T) {
				label := bytes.Clone(rfc8669LabelIndexTLV)
				if dirty {
					label[3], label[4], label[5] = 0xa5, 0x5a, 0xff
				}
				srgb := bytes.Clone(rfc8669SRGBTLV)
				srgb[3], srgb[4] = 0xab, 0xcd
				sid := append(append(label, rfc8669UnknownTLV...), srgb...)
				body := rfc8669LabeledUnicastBody(sid)
				if rail == "general" || rail == "route-server" {
					body = rfc8669RelayUpdate(t, rail == "route-server", NextHopUnchanged, body)
				}
				source := bytes.Clone(body)
				peer, conn := newAnnouncePeer(t, "192.0.2.2")
				s := peer.session
				sections, err := wire.ParseUpdateSections(body)
				if err != nil {
					t.Fatal(err)
				}
				switch rail {
				case "parsed":
					err = s.SendUpdate(&message.Update{PathAttributes: sections.Attrs(body), NLRI: sections.NLRI(body)})
				case "override":
					s.egressRouteFilter = func([]byte) (bool, []byte) { return false, body }
					err = s.SendUpdate(&message.Update{PathAttributes: sections.Attrs(body), NLRI: sections.NLRI(body)})
				case "packet":
					packet := message.PackTo(&message.Update{PathAttributes: sections.Attrs(body), NLRI: sections.NLRI(body)}, nil)
					err = s.SendRawMessage(0, packet)
				case "general", "route-server":
					err = s.sendRawUpdateBody(body)
				default:
					err = s.SendRawMessage(uint8(msgtype.TypeUPDATE), body)
				}
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(body, source) {
					t.Fatal("transmission changed the borrowed source UPDATE")
				}
				packet := conn.written()
				if len(packet) < message.HeaderLen {
					t.Fatal("writer emitted no UPDATE")
				}
				out := packet[message.HeaderLen:]
				written, err := wire.ParseUpdateSections(out)
				if err != nil {
					t.Fatal(err)
				}
				_, _, value, found := attribute.AttrFind(written.Attrs(out), attribute.AttrPrefixSID)
				want := append(append(bytes.Clone(rfc8669LabelIndexTLV), rfc8669UnknownTLV...), srgb...)
				if !found || !bytes.Equal(value, want) {
					t.Fatalf("Prefix-SID = %x, want %x", value, want)
				}
				if !bytes.Equal(written.NLRI(out), sections.NLRI(body)) {
					t.Fatal("normalization changed the NLRI")
				}
			})
		}
	}
}

// rfc8669LabeledUnicastBody builds an UPDATE body for one IPv4 labeled-unicast
// route (RFC 8277: MP_REACH_NLRI AFI 1, SAFI 4, label 100 with the bottom of
// stack bit, prefix 198.51.100.0/24) carrying sid as attribute 40.
func rfc8669LabeledUnicastBody(sid []byte) []byte {
	reach := []byte{
		0, 1, 4, // AFI IPv4, SAFI labeled unicast
		4, 192, 0, 2, 99, // next hop 192.0.2.99
		0,                                  // reserved
		48, 0x00, 0x06, 0x41, 198, 51, 100, // 24 label bits + 24 prefix bits
	}
	attrs := []byte{
		0x40, 1, 1, 0, // ORIGIN igp
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH AS_SEQUENCE [65001]
		0x80, 14, byte(len(reach)),
	}
	attrs = append(attrs, reach...)
	attrs = append(attrs, 0xC0, prefixSIDCodeByte, byte(len(sid)))
	attrs = append(attrs, sid...)
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	return append(body, attrs...)
}
