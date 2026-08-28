package fixture

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestPluginFixture09RemoveMED(t *testing.T) {
	body, err := hex.DecodeString("0000001b4001010040020602010000fde94003040101010180040400000064180a3e00")
	if err != nil {
		t.Fatal(err)
	}
	got, changed, err := removeMED09(body)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("MED attribute was not found")
	}
	want, err := hex.DecodeString("000000144001010040020602010000fde940030401010101180a3e00")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("stripped UPDATE body = %x, want %x", got, want)
	}
}

func TestPluginFixture09L2TPControlRoundTrip(t *testing.T) {
	packet := l2tpControl09(0x1234, 0x5678, 3, 2,
		l2tpAVP09(true, 0, l2tpUint1609(12)),
		l2tpAVP09(true, 24, l2tpUint3209(10_000_000)),
	)
	if got := len(packet); got != int(packet[3]) {
		t.Fatalf("packet length = %d, header length = %d", got, packet[3])
	}
	avps := l2tpParse09(packet)
	if got := avps[0]; !bytes.Equal(got, []byte{0, 12}) {
		t.Fatalf("message-type AVP = %x, want 000c", got)
	}
	if got := avps[24]; !bytes.Equal(got, []byte{0, 0x98, 0x96, 0x80}) {
		t.Fatalf("connect-speed AVP = %x, want 00989680", got)
	}
}
