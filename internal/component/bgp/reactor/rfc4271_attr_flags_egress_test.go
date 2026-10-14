// Design: docs/architecture/wire/attributes.md -- the reserved flag bits on send
// Overview: session_egress_normalize.go -- normalizeTransmittedUpdate

package reactor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// egressAttr is one path attribute as a test reads it off the wire.
type egressAttr struct {
	flags, code byte
	value       []byte
}

// egressAttrs splits an attribute section, failing the test on bad framing.
func egressAttrs(t *testing.T, attrs []byte) []egressAttr {
	t.Helper()
	var out []egressAttr
	for len(attrs) != 0 {
		require.GreaterOrEqual(t, len(attrs), 3, "truncated attribute header")
		header, length := 3, int(attrs[2])
		if attrs[0]&0x10 != 0 {
			require.GreaterOrEqual(t, len(attrs), 4, "truncated extended header")
			header, length = 4, int(binary.BigEndian.Uint16(attrs[2:]))
		}
		require.GreaterOrEqual(t, len(attrs), header+length, "attribute overruns the section")
		out = append(out, egressAttr{flags: attrs[0], code: attrs[1], value: attrs[header : header+length]})
		attrs = attrs[header+length:]
	}
	return out
}

// egressBody frames attrs and nlri as an UPDATE body with no withdrawn routes.
func egressBody(attrs, nlri []byte) []byte {
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	body = append(body, attrs...)
	return append(body, nlri...)
}

// egressNLRI is 198.51.100.0/24.
var egressNLRI = []byte{24, 198, 51, 100}

// lowNibbleAttrs is ORIGIN, AS_PATH and NEXT_HOP flagged 0x4F and an
// unrecognized optional transitive attribute 241 flagged 0xCF, then an
// unrecognized optional transitive attribute 242 in Extended Length form
// flagged 0xD5: every attribute carries reserved bits.
func lowNibbleAttrs() []byte {
	return []byte{
		0x4F, 1, 1, 0, // ORIGIN igp
		0x4F, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH AS_SEQUENCE [65001]
		0x4F, 3, 4, 192, 0, 2, 99, // NEXT_HOP 192.0.2.99
		0xCF, 241, 3, 0xAA, 0xBB, 0xCC, // unrecognized, 0xCF
		0xD5, 242, 0, 2, 0xDD, 0xEE, // unrecognized, Extended Length, 0xD5
	}
}

// egressWrite sends body through one session writer and returns the UPDATE body
// that reached the wire. The source bytes are checked unedited.
func egressWrite(t *testing.T, rail string, body []byte) []byte {
	t.Helper()
	source := bytes.Clone(body)
	peer, conn := newAnnouncePeer(t, "192.0.2.2")
	s := peer.session
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	switch rail {
	case "raw":
		err = s.SendRawMessage(uint8(msgtype.TypeUPDATE), body)
	case "parsed":
		err = s.SendUpdate(&message.Update{PathAttributes: sections.Attrs(body), NLRI: sections.NLRI(body)})
	case "override":
		s.egressRouteFilter = func([]byte) (bool, []byte) { return false, body }
		err = s.SendUpdate(&message.Update{PathAttributes: sections.Attrs(body), NLRI: sections.NLRI(body)})
	case "relay":
		err = s.sendRawUpdateBody(body)
	default:
		t.Fatalf("unknown rail %q", rail)
	}
	require.NoError(t, err)
	require.Equal(t, source, body, "the writer edited the borrowed source bytes")
	packet := conn.written()
	require.GreaterOrEqual(t, len(packet), message.HeaderLen, "writer emitted no UPDATE")
	require.Equal(t, len(packet), int(binary.BigEndian.Uint16(packet[16:18])), "message length field")
	return packet[message.HeaderLen:]
}

// egressRails are the session writers every UPDATE funnels through: the raw
// diagnostic write (Q-3 (a)), the parsed send, a filter override and the relay.
var egressRails = []string{"raw", "parsed", "override", "relay"}

// TestRFC4271WriteClearsLowNibbleEveryAttribute sends an UPDATE whose every
// attribute carries reserved flag bits through each session writer.
//
// VALIDATES: AC-1a on writeUpdateBody: every flags octet leaves with bits 0-3
// clear and bits 4-7 as given (Extended Length kept); values and NLRI unchanged.
// PREVENTS: a relayed or raw-sent attribute repeating a peer's reserved bits.
// RFC requirement: RFC4271-4.3-3 positive -- on the raw, parsed, filter-override and relay session writers every attribute's flags octet is sent with its four lower-order bits zero and its four upper bits unchanged.
func TestRFC4271WriteClearsLowNibbleEveryAttribute(t *testing.T) {
	in := egressBody(lowNibbleAttrs(), egressNLRI)
	want := egressAttrs(t, lowNibbleAttrs())
	for _, rail := range egressRails {
		t.Run(rail, func(t *testing.T) {
			out := egressWrite(t, rail, in)
			sections, err := wire.ParseUpdateSections(out)
			require.NoError(t, err)
			got := egressAttrs(t, sections.Attrs(out))
			require.Len(t, got, len(want))
			for i := range want {
				require.Equal(t, want[i].code, got[i].code)
				require.Equal(t, want[i].flags&0xF0, got[i].flags, "attribute %d flags", got[i].code)
				require.Equal(t, want[i].value, got[i].value, "attribute %d value", got[i].code)
			}
			require.Equal(t, egressNLRI, sections.NLRI(out))
		})
	}
}

// TestRFC4271WriteLeavesCleanFlagsUnchanged sends an UPDATE whose flags octets
// carry no reserved bit, one attribute in Extended Length form.
//
// VALIDATES: AC-6: the bytes sent equal the bytes given, on every writer.
// PREVENTS: the clear touching Optional, Transitive, Partial or Extended Length.
// RFC requirement: RFC4271-4.3-3 negative -- an UPDATE whose flags octets already have the lower-order bits zero is sent byte-for-byte as given on the raw, parsed, filter-override and relay writers, Extended Length kept.
func TestRFC4271WriteLeavesCleanFlagsUnchanged(t *testing.T) {
	clean := lowNibbleAttrs()
	for i := 0; i < len(clean); {
		header, length := 3, int(clean[i+2])
		if clean[i]&0x10 != 0 {
			header, length = 4, int(binary.BigEndian.Uint16(clean[i+2:]))
		}
		clean[i] &^= 0x0F
		i += header + length
	}
	in := egressBody(clean, egressNLRI)
	for _, rail := range egressRails {
		t.Run(rail, func(t *testing.T) {
			require.Equal(t, in, egressWrite(t, rail, in))
		})
	}
}

// discardConn is a recordingConn whose Write keeps nothing, for a benchmark.
type discardConn struct{ recordingConn }

func (c *discardConn) Write(b []byte) (int, error) { return len(b), nil }

// BenchmarkWriteUpdateBodyEgressWalk measures Session.writeUpdateBody, the one
// writer every UPDATE funnels through, over bodies with no LARGE_COMMUNITY, 4
// and 20 distinct values, and 20 values with one repeat. It drives the writer,
// not the walk, so the same file runs at the parent commit for the baseline.
// Agents prepare it; the owner runs it on the Mac.
func BenchmarkWriteUpdateBodyEgressWalk(b *testing.B) {
	settings := &PeerSettings{
		Connection:   ConnectionBoth,
		Address:      netip.MustParseAddr("192.0.2.2"),
		LocalAddress: netip.MustParseAddr("192.0.2.1"),
		LocalAS:      65000,
		PeerAS:       65001,
		RouterID:     0x01020301,
	}
	cases := []struct {
		name   string
		values int
		repeat bool
	}{{"no-large-community", 0, false}, {"4-distinct", 4, false}, {"20-distinct", 20, false}, {"20-one-repeat", 20, true}}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			s := NewSession(settings)
			for _, ev := range []fsm.Event{fsm.EventManualStart, fsm.EventTCPConnectionConfirmed, fsm.EventBGPOpen, fsm.EventKeepaliveMsg} {
				if err := s.fsm.Event(ev); err != nil {
					b.Fatal(err)
				}
			}
			conn := &discardConn{}
			s.conn = conn
			s.bufWriter = bufio.NewWriterSize(conn, 4096)
			attrs := bytes.Clone(lowNibbleAttrs())
			if tc.values != 0 {
				attrs = append(attrs, largeCommunityAttr(largeCommunityValues(tc.values, tc.repeat))...)
			}
			body := egressBody(attrs, egressNLRI)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := s.writeUpdateBody(body, true); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
