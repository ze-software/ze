//go:build linux

package l2tp

import (
	"encoding/binary"
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// decodedNLA holds one parsed netlink attribute.
type decodedNLA struct {
	typ  uint16
	data []byte
}

// parseNLAs walks a serialized NLA stream and returns one decodedNLA per
// attribute. Used by tests to verify attribute encoding without relying on
// position.
func parseNLAs(t *testing.T, buf []byte) []decodedNLA {
	t.Helper()
	var out []decodedNLA
	off := 0
	for off < len(buf) {
		if off+4 > len(buf) {
			t.Fatalf("truncated NLA header at offset %d", off)
		}
		length := binary.LittleEndian.Uint16(buf[off : off+2])
		typ := binary.LittleEndian.Uint16(buf[off+2 : off+4])
		if length < 4 || int(length) > len(buf)-off {
			t.Fatalf("bad NLA length %d at offset %d", length, off)
		}
		data := buf[off+4 : off+int(length)]
		out = append(out, decodedNLA{typ: typ, data: append([]byte(nil), data...)})
		pad := (4 - int(length)%4) % 4
		off += int(length) + pad
	}
	return out
}

func findNLA(attrs []decodedNLA, typ uint16) *decodedNLA {
	for i := range attrs {
		if attrs[i].typ == typ {
			return &attrs[i]
		}
	}
	return nil
}

func TestGenlTunnelCreateMsg(t *testing.T) {
	// VALIDATES: AC-3 -- tunnel create carries the right attributes.
	// PREVENTS: kernel rejects tunnel create due to missing / wrong attrs.
	buf := marshalTunnelCreateAttrs(100, 200, 42)
	attrs := parseNLAs(t, buf)

	checks := []struct {
		name string
		typ  uint16
		want uint32
	}{
		{"CONN_ID", l2tpAttrConnID, 100},
		{"PEER_CONN_ID", l2tpAttrPeerConnID, 200},
		{"FD", l2tpAttrFD, 42},
	}
	for _, c := range checks {
		a := findNLA(attrs, c.typ)
		if a == nil {
			t.Fatalf("missing attr %s (%d)", c.name, c.typ)
		}
		if len(a.data) != 4 {
			t.Fatalf("attr %s: expected 4 bytes, got %d", c.name, len(a.data))
		}
		if got := binary.LittleEndian.Uint32(a.data); got != c.want {
			t.Fatalf("attr %s: want %d got %d", c.name, c.want, got)
		}
	}

	proto := findNLA(attrs, l2tpAttrProtoVersion)
	if proto == nil || len(proto.data) != 1 || proto.data[0] != 2 {
		t.Fatalf("PROTO_VERSION: want 2, got %v", proto)
	}
	encap := findNLA(attrs, l2tpAttrEncapType)
	if encap == nil || len(encap.data) != 2 {
		t.Fatalf("ENCAP_TYPE missing or wrong length: %v", encap)
	}
	if v := binary.LittleEndian.Uint16(encap.data); v != l2tpEncapUDP {
		t.Fatalf("ENCAP_TYPE: want %d (UDP), got %d", l2tpEncapUDP, v)
	}
}

func TestGenlSessionCreateMsg(t *testing.T) {
	// VALIDATES: AC-5 -- session create carries the core attributes.
	// PREVENTS: kernel rejects session create due to missing / wrong attrs.
	buf := marshalSessionCreateAttrs(sessionCreateParams{
		tunnelID:  100,
		localSID:  1001,
		remoteSID: 2001,
	})
	attrs := parseNLAs(t, buf)

	checks := []struct {
		name string
		typ  uint16
		want uint32
	}{
		{"CONN_ID", l2tpAttrConnID, 100},
		{"SESSION_ID", l2tpAttrSessionID, 1001},
		{"PEER_SESSION_ID", l2tpAttrPeerSessionID, 2001},
	}
	for _, c := range checks {
		a := findNLA(attrs, c.typ)
		if a == nil || len(a.data) != 4 {
			t.Fatalf("attr %s missing or wrong length: %v", c.name, a)
		}
		if v := binary.LittleEndian.Uint32(a.data); v != c.want {
			t.Fatalf("attr %s: want %d got %d", c.name, c.want, v)
		}
	}

	pw := findNLA(attrs, l2tpAttrPwType)
	if pw == nil || len(pw.data) != 2 {
		t.Fatalf("PW_TYPE missing or wrong length: %v", pw)
	}
	if v := binary.LittleEndian.Uint16(pw.data); v != l2tpPWTypePPP {
		t.Fatalf("PW_TYPE: want %d (PPP=7), got %d", l2tpPWTypePPP, v)
	}

	if findNLA(attrs, l2tpAttrLNSMode) != nil {
		t.Fatal("LNS_MODE should be absent when lnsMode=false")
	}
	if findNLA(attrs, l2tpAttrSendSeq) != nil {
		t.Fatal("SEND_SEQ should be absent when sendSeq=false")
	}
	if findNLA(attrs, l2tpAttrRecvSeq) != nil {
		t.Fatal("RECV_SEQ should be absent when recvSeq=false")
	}
}

func TestGenlSessionCreateLNS(t *testing.T) {
	// VALIDATES: AC-6 -- LNS_MODE=1 when lnsMode=true.
	// PREVENTS: LAC-mode kernel session for an LNS-side setup.
	buf := marshalSessionCreateAttrs(sessionCreateParams{
		tunnelID: 1, localSID: 2, remoteSID: 3, lnsMode: true,
	})
	attrs := parseNLAs(t, buf)
	a := findNLA(attrs, l2tpAttrLNSMode)
	if a == nil || len(a.data) != 1 || a.data[0] != 1 {
		t.Fatalf("LNS_MODE: want 1, got %v", a)
	}
}

func TestGenlSessionCreateSequencing(t *testing.T) {
	// VALIDATES: AC-7 -- SEND_SEQ and RECV_SEQ both =1 when sequencing.
	// PREVENTS: data plane silently drops sequence numbers.
	buf := marshalSessionCreateAttrs(sessionCreateParams{
		tunnelID: 1, localSID: 2, remoteSID: 3,
		sendSeq: true, recvSeq: true,
	})
	attrs := parseNLAs(t, buf)
	for _, typ := range []uint16{l2tpAttrSendSeq, l2tpAttrRecvSeq} {
		a := findNLA(attrs, typ)
		if a == nil || len(a.data) != 1 || a.data[0] != 1 {
			t.Fatalf("attr %d: want 1, got %v", typ, a)
		}
	}
}

func TestAppendNLAPadding(t *testing.T) {
	// VALIDATES: NLA padding rounds length up to 4-byte boundary.
	// PREVENTS: unaligned follow-on attributes which the kernel rejects.
	buf := appendNLA(nil, 99, nl.Uint8Attr(1))
	if len(buf) != 8 {
		t.Fatalf("1-byte NLA: expected padded length 8, got %d", len(buf))
	}
	if l := binary.LittleEndian.Uint16(buf[0:2]); l != 5 {
		t.Fatalf("NLA length field: want 5, got %d", l)
	}
	buf = appendNLA(nil, 99, nl.Uint16Attr(0xBEEF))
	if len(buf) != 8 {
		t.Fatalf("2-byte NLA: expected padded length 8, got %d", len(buf))
	}
	buf = appendNLA(nil, 99, nl.Uint32Attr(0xDEADBEEF))
	if len(buf) != 8 {
		t.Fatalf("4-byte NLA: expected length 8, got %d", len(buf))
	}
}

func TestMarshalTunnelCreateBoundaryIDs(t *testing.T) {
	// VALIDATES: boundary tunnel IDs 1 and 65535 encode correctly.
	// PREVENTS: silent truncation of uint16 into uint32 NLA.
	cases := []struct {
		name             string
		localTID, remote uint16
	}{
		{"min", 1, 1},
		{"max", 65535, 65535},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			attrs := parseNLAs(t, marshalTunnelCreateAttrs(c.localTID, c.remote, 7))
			conn := findNLA(attrs, l2tpAttrConnID)
			if v := binary.LittleEndian.Uint32(conn.data); v != uint32(c.localTID) {
				t.Fatalf("CONN_ID: want %d got %d", c.localTID, v)
			}
			peer := findNLA(attrs, l2tpAttrPeerConnID)
			if v := binary.LittleEndian.Uint32(peer.data); v != uint32(c.remote) {
				t.Fatalf("PEER_CONN_ID: want %d got %d", c.remote, v)
			}
		})
	}
}
