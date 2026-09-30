// Design: docs/architecture/ospf/ospf-af-unify.md -- the OSPFv3 packet encoder.
// Related: encoder_v6.go -- v6Encoder, the OSPFv3 packet producer.
// Related: rfc5340_test.go -- TestRFC5340ReservedHeaderOctetIgnoredOnReceive, the receive half.
//
// VALIDATES: RFC 5340 Appendix A.3.1 send half: the Reserved octet of the OSPFv3 common
// header (offset 15) is zero in every packet Ze encodes.
// PREVENTS: a header writer that leaves the octet to whatever the buffer held, or lets the
// Instance ID beside it spill into it.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// rfc5340ReservedOffset is the Reserved octet of the OSPFv3 common header (RFC 5340 A.3.1).
const rfc5340ReservedOffset = 15

// RFC requirement: RFC5340-A.3.1-2 positive -- RFC 5340 Appendix A.3.1: "These fields are
// reserved. They SHOULD be set to 0 when sending protocol packets". The OSPFv3 encoder's
// Hello, Database Description and Link State Acknowledgment packets each carry 0 in the
// Reserved octet (offset 15).
// RFC requirement: RFC5340-A.3.1-2 negative -- the inputs are pushed toward a non-zero
// octet: an encoder whose Instance ID (the octet beside it) is 0xFF still writes Reserved 0,
// and a common header written into a buffer pre-filled with 0xFF leaves 0 at offset 15, so a
// writer that skipped the octet or spilled the Instance ID into it fails here.
func TestRFC5340ReservedHeaderOctetZeroOnSend(t *testing.T) {
	// Goal: the send clause of A.3.1. Method: encode each packet shape and read offset 15;
	// then write a header over a dirty buffer.
	rid := ridOf("10.0.0.1")
	for _, instance := range []uint8{0, 0xFF} {
		enc := v6Encoder{instanceID: instance}
		for name, pkt := range map[string][]byte{
			"hello":  enc.EncodeHello(rid, types.BackboneArea, packet.Hello{HelloInterval: 10, DeadInterval: 40}),
			"dbdesc": enc.EncodeDBDesc(rid, types.BackboneArea, packet.DBDesc{InterfaceMTU: 1500, DDSequence: 1}),
			"lsack":  enc.EncodeLSAck(rid, types.BackboneArea, packet.LSAck{}),
		} {
			if assert.Greater(t, len(pkt), rfc5340ReservedOffset, "%s: packet shorter than the header", name) {
				assert.Zero(t, pkt[rfc5340ReservedOffset], "%s with Instance ID %#x: Reserved octet must be 0", name, instance)
			}
		}
	}

	dirty := make([]byte, ospfv3packet.CommonHeaderLen)
	for i := range dirty {
		dirty[i] = 0xFF
	}
	ospfv3packet.Header{InstanceID: ospfv3types.InstanceID(0xFF)}.WriteTo(dirty, 0)
	assert.Zero(t, dirty[rfc5340ReservedOffset], "the header writer zeroes the Reserved octet over a dirty buffer")
}
