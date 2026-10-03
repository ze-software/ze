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
// dirty Label-Index TLVs through the real session writers, including both relay
// planners followed by their raw-body writer. The source and Originator SRGB
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
				body := rfc8669RelayBody(sid)
				if rail == "general" || rail == "route-server" {
					body = rfc8669Relay(t, rail == "route-server", NextHopUnchanged, sid)
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
