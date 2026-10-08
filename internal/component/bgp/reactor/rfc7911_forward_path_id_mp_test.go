// Design: docs/architecture/edge-cases/addpath.md -- locally significant Path Identifiers.
// Related: forward_path_id.go -- fwdRegenerateRawPathIDs.
// Related: forward_body.go -- fwdReencodeMPAttributes.
// RFC 7911 Sections 2 and 5 -- see rfc/short/rfc7911.md.
package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"
)

// TestForwardPathIDMPGeneration sends two IPv6 prefixes from colliding sources
// through the raw MP rewrite, the same-framing MP conversion, and the conversion
// from an unframed source. It reads exact native prefix/identifier pairs, then
// replaces and withdraws those paths in the opposite prefix order.
// RFC requirement: RFC7911-2-2 positive -- "A BGP speaker that re-advertises a route MUST generate its own Path Identifier to be associated with the re-advertised route." (Section 2). Each MP path keeps its local identity through replacement and withdrawal.
// RFC requirement: RFC7911-2-2 negative -- identical received identifiers for the same native prefix from different sources must not collapse into one outgoing path at the recipient. Neither numeric inequality from the received value nor inequality across different prefixes is required.
// MUTATION: Skip the MP_REACH or MP_UNREACH rewrite in fwdRegenerateRawPathIDs,
// or pass either MP attribute through unchanged in fwdReencodeMPAttributes.
// The collision or exact inverse-withdrawal assertion must fail, not a producer halt.
func TestForwardPathIDMPGeneration(t *testing.T) {
	for index, mode := range []string{"same-context", "cross-context", "unframed-source"} {
		t.Run(mode, func(t *testing.T) {
			fam := family.IPv6Unicast
			destCtx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: true})
			destCtxID, err := bgpctx.Registry.Register(destCtx)
			if err != nil {
				t.Fatal(err)
			}
			srcCtxID := destCtxID
			framed := mode != "unframed-source"
			if mode != "same-context" {
				srcCtx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{
					fam: framed, family.IPv4Unicast: true,
				})
				srcCtxID, err = bgpctx.Registry.Register(srcCtx)
				if err != nil {
					t.Fatal(err)
				}
				if srcCtxID == destCtxID {
					t.Fatal("fixture did not select MP attribute conversion")
				}
			}
			peer := forwardBodyTestPeer(destCtx, destCtxID)
			firstSource := source.SourceID(4401 + index*2)
			secondSource := firstSource + 1
			t.Cleanup(func() {
				fwdPathIDs.releaseSource(firstSource)
				fwdPathIDs.releaseSource(secondSource)
			})
			emit := func(src source.SourceID, asn uint32, prefixes [2]netip.Prefix, withdraw bool) [2]uint32 {
				t.Helper()
				var section []byte
				for _, prefix := range prefixes {
					if framed {
						section = binary.BigEndian.AppendUint32(section, 0)
					}
					section = append(section, nlri.NewINET(fam, prefix, 0).Bytes()...)
				}
				nextHop := netip.MustParseAddr("2001:db8::1").AsSlice()
				mp := buildMPReachSource(uint16(fam.AFI), byte(fam.SAFI), nextHop, section)
				attrs := append(forwardBodyBaseAttrs(t, asn), mp...)
				if withdraw {
					attrs = makeAttr(0x80, byte(attribute.AttrMPUnreachNLRI), append([]byte{0, 2, 1}, section...))
				}
				body := fwdShapeBody(nil, attrs, nil)
				original := bytes.Clone(body)
				wire := fwdPathIDWire(body, srcCtxID, src)
				result, ok := buildFwdBody(wire, message.MaxMsgLen, destCtxID, peer,
					netip.MustParseAddr("192.0.2.60"), &fwdParseCache{})
				if !ok {
					t.Fatal("valid MP UPDATE did not forward")
				}
				defer returnReadBuffer(result.transcodeBuf)
				var update *message.Update
				if mode == "same-context" {
					if len(result.rawBodies) != 1 {
						t.Fatalf("raw MP rewrite produced %d bodies", len(result.rawBodies))
					}
					update, err = message.UnpackUpdate(result.rawBodies[0])
					if err != nil {
						t.Fatal(err)
					}
				} else {
					if len(result.rawBodies) != 0 {
						t.Fatal("MP conversion unexpectedly selected the raw rail")
					}
					if len(result.updates) != 1 {
						t.Fatalf("MP conversion produced %d updates", len(result.updates))
					}
					update = result.updates[0]
				}
				if len(update.NLRI) != 0 {
					t.Fatal("MP announcement escaped into legacy NLRI")
				}
				if len(update.WithdrawnRoutes) != 0 {
					t.Fatal("MP withdrawal escaped into legacy NLRI")
				}
				code, opposite := attribute.AttrMPReachNLRI, attribute.AttrMPUnreachNLRI
				if withdraw {
					code, opposite = opposite, code
				}
				if _, _, _, found := attribute.AttrFind(update.PathAttributes, opposite); found {
					t.Fatal("MP UPDATE contains the opposite operation")
				}
				_, _, value, found := attribute.AttrFind(update.PathAttributes, code)
				if !found {
					t.Fatal("outgoing MP attribute is missing")
				}
				header := []byte{0, 2, 1}
				if !withdraw {
					header = append(header, 16)
					header = append(header, nextHop...)
					header = append(header, 0)
				}
				if !bytes.HasPrefix(value, header) {
					t.Fatalf("MP family/next hop changed: %x, want header %x", value, header)
				}
				iter := nlri.NewNLRIIterator(value[len(header):], true)
				var identifiers [2]uint32
				for i, prefix := range prefixes {
					native, id, present := iter.Next()
					if !present {
						t.Fatalf("missing native prefix %s", prefix)
					}
					if !bytes.Equal(native, nlri.NewINET(fam, prefix, 0).Bytes()) {
						t.Fatalf("identifier %d accompanies %x, want %s", id, native, prefix)
					}
					identifiers[i] = id
				}
				if iter.Remaining() != 0 {
					t.Fatalf("unexpected trailing MP NLRI: %d octets", iter.Remaining())
				}
				if !bytes.Equal(body, original) {
					t.Fatal("forwarding modified the received MP UPDATE")
				}
				return identifiers
			}

			prefixes := [2]netip.Prefix{netip.MustParsePrefix("2001:db8:1::/48"), netip.MustParsePrefix("2001:db8:2::/48")}
			first := emit(firstSource, 65001, prefixes, false)
			second := emit(secondSource, 65002, prefixes, false)
			for i, prefix := range prefixes {
				if first[i] == second[i] {
					t.Fatalf("two sources collapsed into (%s, %d)", prefix, first[i])
				}
			}
			reversed := [2]netip.Prefix{prefixes[1], prefixes[0]}
			wantFirst := [2]uint32{first[1], first[0]}
			if got := emit(firstSource, 65010, reversed, false); got != wantFirst {
				t.Fatalf("attribute replacement changed native path pairing: %v, want %v", got, wantFirst)
			}
			if got := emit(firstSource, 65010, reversed, true); got != wantFirst {
				t.Fatalf("MP_UNREACH does not invert the advertised native paths: %v, want %v", got, wantFirst)
			}
			if got := emit(secondSource, 65002, prefixes, false); got != second {
				t.Fatalf("withdrawing another source changed the surviving paths: %v, want %v", got, second)
			}
			if got := emit(secondSource, 65002, reversed, true); got != [2]uint32{second[1], second[0]} {
				t.Fatalf("second source's inverse withdrawal changed its native path pairing: %v", got)
			}
		})
	}
}
