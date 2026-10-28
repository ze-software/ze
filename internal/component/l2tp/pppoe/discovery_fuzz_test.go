// Design: docs/architecture/wire/l2tp.md -- PPPoE discovery wire format
// Related: discovery.go -- ParseDiscovery and BuildPADO under fuzz

package pppoe

import (
	"encoding/binary"
	"testing"
)

// FuzzParseDiscovery drives arbitrary frames through ParseDiscovery and, when
// one parses, through BuildPADO and BuildPADS as the AC would answer it. The
// seeds include the malformed inputs behind accel-ppp-ng's PPPoE fixes.
//
// Invariants: no panic; every tag value lies inside the frame's PPPoE payload;
// no more than maxTags tags; a built reply is nil or a frame of at most
// EthMaxLen octets that ParseDiscovery accepts.
func FuzzParseDiscovery(f *testing.F) {
	frame := func(verType byte, payload []byte) []byte {
		return ngDiscFrame(ngHost, verType, CodePADI, payload)
	}
	service := ngTag(TagServiceName, nil)
	seeds := [][]byte{
		nil,
		frame(PPPoEVerType, service),
		// accel-ppp-ng c93b367 / 8305387 (T8825): Host-Uniq filling the frame.
		frame(PPPoEVerType, append(append([]byte(nil), service...),
			ngTag(TagHostUniq, make([]byte, EthMaxLen-MinDiscFrame-2*TagHdrLen))...)),
		// accel-ppp-ng 93163c8 (T9238): truncated tag header after Service-Name.
		frame(PPPoEVerType, append(append([]byte(nil), service...), 0x01, 0x03)),
		// accel-ppp-ng e042eab (T9238): unsupported TYPE and VER.
		frame(0x12, service),
		frame(0x21, service),
		// accel-ppp-ng ef7323f (T8464): PADR without Service-Name.
		ngDiscFrame(ngHost, PPPoEVerType, CodePADR, ngTag(TagACCookie, make([]byte, 24))),
		// accel-ppp-ng 2fa6475 (T8550): zero-length tags only.
		frame(PPPoEVerType, make([]byte, 64)),
		// PPPoE LENGTH larger than the frame.
		func() []byte {
			b := frame(PPPoEVerType, service)
			binary.BigEndian.PutUint16(b[EthHdrLen+4:], 0xFFFF)
			return b
		}(),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		pkt, err := ParseDiscovery(data)
		if err != nil {
			return
		}
		if len(pkt.Tags) > maxTags {
			t.Fatalf("%d tags, over maxTags %d", len(pkt.Tags), maxTags)
		}
		payloadLen := int(binary.BigEndian.Uint16(data[EthHdrLen+4:]))
		total := 0
		for _, tag := range pkt.Tags {
			total += TagHdrLen + len(tag.Value)
		}
		if total > payloadLen {
			t.Fatalf("tags cover %d octets, payload is %d", total, payloadLen)
		}
		var pado, pads [EthMaxLen]byte
		checkNGReply(t, "PADO", BuildPADO(pado[:], ngHost, &pkt, "ze-ac", []string{"internet"}, make([]byte, 24)))
		checkNGReply(t, "PADS", BuildPADS(pads[:], ngHost, &pkt, "ze-ac", 1))
	})
}

// checkNGReply asserts a built reply is absent or a parseable frame that fits
// one Ethernet frame.
func checkNGReply(t *testing.T, name string, reply []byte) {
	t.Helper()
	if reply == nil {
		return
	}
	if len(reply) > EthMaxLen {
		t.Fatalf("%s is %d octets, over %d", name, len(reply), EthMaxLen)
	}
	if _, err := ParseDiscovery(reply); err != nil {
		t.Fatalf("%s does not parse: %v", name, err)
	}
}
