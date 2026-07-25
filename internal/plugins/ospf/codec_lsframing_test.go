// VALIDATES: the neighbor-FSM framing codecs the engine reaches through the Codec seam --
// the OSPFv2 v4Codec.DecodeLSReq / v4Codec.DecodeLSAck body decoders (codec.go) and the
// OSPFv3 v6Encoder.EncodeLSAck send path (encoder_v6.go), each round-tripped against its
// decoder and checked for a clean error (never a panic) on a truncated or type-mismatched
// datagram.
// PREVENTS: an LS Request / LS Ack body silently mis-decoding through the Codec interface;
// the v6 LS Ack encoder producing bytes the v6 codec cannot parse; a malformed datagram
// panicking the decode path instead of returning an error.
package ospf

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func lsFramingHeader() types.LSAHeader {
	return types.LSAHeader{
		Age:               42,
		Type:              types.LSType(1), // Router-LSA
		LinkStateID:       types.LinkStateID{5, 6, 7, 8},
		AdvertisingRouter: types.RouterID{2, 2, 2, 2},
		Sequence:          types.LSSequenceNumber(0x80000005),
		Checksum:          0xbeef,
		Length:            20,
	}
}

func TestV4CodecDecodeLSReq(t *testing.T) {
	req := packet.LSReq{Requests: []types.LSRequestEntry{
		{Type: types.LSType(1), LinkStateID: types.LinkStateID{0, 0, 0, 1}, AdvertisingRouter: types.RouterID{3, 3, 3, 3}},
		{Type: types.LSType(5), LinkStateID: types.LinkStateID{9, 9, 9, 9}, AdvertisingRouter: types.RouterID{4, 4, 4, 4}},
	}}
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeLSReq, RouterID: types.RouterID{1, 1, 1, 1}}, LSReq: &req}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)

	got, err := v4Codec{}.DecodeLSReq(buf)
	if err != nil {
		t.Fatalf("DecodeLSReq: %v", err)
	}
	if len(got.Requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(got.Requests))
	}
	if got.Requests[0].Type != types.LSType(1) ||
		got.Requests[0].LinkStateID != (types.LinkStateID{0, 0, 0, 1}) ||
		got.Requests[0].AdvertisingRouter != (types.RouterID{3, 3, 3, 3}) {
		t.Fatalf("request[0] = %+v", got.Requests[0])
	}
	if got.Requests[1].AdvertisingRouter != (types.RouterID{4, 4, 4, 4}) || got.Requests[1].Type != types.LSType(5) {
		t.Fatalf("request[1] = %+v", got.Requests[1])
	}

	// A Hello datagram decoded as an LS Request must return the type-mismatch sentinel.
	hello := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, RouterID: types.RouterID{1, 1, 1, 1}}, Hello: &packet.Hello{HelloInterval: 10, DeadInterval: 40}}
	hbuf := make([]byte, hello.EncodedLen())
	hello.WriteTo(hbuf, 0)
	if _, err := (v4Codec{}).DecodeLSReq(hbuf); !errors.Is(err, ErrNotLSReq) {
		t.Fatalf("DecodeLSReq(hello) error = %v, want ErrNotLSReq", err)
	}

	// A truncated datagram (shorter than the common header) errors without panicking.
	if _, err := (v4Codec{}).DecodeLSReq([]byte{0x02, 0x03, 0x00}); err == nil {
		t.Fatalf("DecodeLSReq(truncated) must return an error")
	}
}

func TestV4CodecDecodeLSAck(t *testing.T) {
	ack := packet.LSAck{Headers: []types.LSAHeader{lsFramingHeader()}}
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeLSAck, RouterID: types.RouterID{1, 1, 1, 1}}, LSAck: &ack}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)

	got, err := v4Codec{}.DecodeLSAck(buf)
	if err != nil {
		t.Fatalf("DecodeLSAck: %v", err)
	}
	if len(got.Headers) != 1 {
		t.Fatalf("headers = %d, want 1", len(got.Headers))
	}
	h := got.Headers[0]
	if h.Type != types.LSType(1) || h.LinkStateID != (types.LinkStateID{5, 6, 7, 8}) ||
		h.AdvertisingRouter != (types.RouterID{2, 2, 2, 2}) || h.Sequence != types.LSSequenceNumber(0x80000005) {
		t.Fatalf("acked header = %+v, does not round-trip", h)
	}

	// Type mismatch: a DBDesc datagram decoded as an LS Ack.
	dd := packet.Packet{Header: packet.Header{Type: packet.PacketTypeDBDesc, RouterID: types.RouterID{1, 1, 1, 1}}, DBDesc: &packet.DBDesc{InterfaceMTU: 1500}}
	dbuf := make([]byte, dd.EncodedLen())
	dd.WriteTo(dbuf, 0)
	if _, err := (v4Codec{}).DecodeLSAck(dbuf); !errors.Is(err, ErrNotLSAck) {
		t.Fatalf("DecodeLSAck(dbdesc) error = %v, want ErrNotLSAck", err)
	}

	// Truncated datagram: error, no panic.
	if _, err := (v4Codec{}).DecodeLSAck([]byte{0x02, 0x05}); err == nil {
		t.Fatalf("DecodeLSAck(truncated) must return an error")
	}
}

func TestV6EncoderEncodeLSAckRoundTrip(t *testing.T) {
	ack := packet.LSAck{Headers: []types.LSAHeader{neutralHdr()}}
	buf := v6Encoder{instanceID: 9}.EncodeLSAck(types.RouterID{1, 1, 1, 1}, types.AreaID{}, ack)

	// The common header carries the OSPFv3 type and Instance ID.
	hdr, err := v6Codec{}.DecodeHeader(buf)
	if err != nil || hdr.Type != PacketTypeLSAck || hdr.InstanceID != 9 {
		t.Fatalf("header = %+v err=%v, want LSAck/instance 9", hdr, err)
	}
	// The acked LSA header round-trips through the v6 codec.
	got, err := v6Codec{}.DecodeLSAck(buf)
	if err != nil {
		t.Fatalf("DecodeLSAck: %v", err)
	}
	if len(got.Headers) != 1 || got.Headers[0] != neutralHdr() {
		t.Fatalf("LS Ack header round-trip = %+v, want %+v", got.Headers, neutralHdr())
	}
}
