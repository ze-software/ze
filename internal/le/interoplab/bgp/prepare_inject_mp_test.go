package bgp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// The selected lab network changes address fields, never VPN RDs, NLRI or
// deliberately malformed attributes. Mixed UPDATEs owe both address changes.
func TestRenderInjectedMPNextHopBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		attrs string
		want  string
	}{
		{name: "labeled IPv4", attrs: "800e0f00010404ac1e000900280006410a00", want: "800e0f00010404ac1f470900280006410a00"},
		{name: "VPN IPv4", attrs: "800e1e0001800c0000000000000000ac1e00090060000c810000fdec0000000a0a", want: "800e1e0001800c0000000000000000ac1f47090060000c810000fdec0000000a0a"},
		{name: "legacy and MP", attrs: "400304ac1e0009800e0f00010404ac1e000900280006410a00", want: "400304ac1f4709800e0f00010404ac1f470900280006410a00"},
		{name: "IPv6 address is not IPv4", attrs: "800e150001011020010db8ac1e0009000000000000000900"},
		{name: "nonlab IPv4", attrs: "800e0900010104c000020900"},
		{name: "truncated MP next hop preserves legacy", attrs: "400304ac1e0009800e0600010104ac1e"},
		{name: "missing reserved byte preserves legacy", attrs: "400304ac1e0009800e0800010104ac1e0009"},
		{name: "duplicate MP preserves legacy", attrs: "400304ac1e0009800e0900010104ac1e000900800e0900010104ac1e000900"},
		{name: "wrong MP flags preserves legacy", attrs: "400304ac1e0009c00e0900010104ac1e000900"},
		{name: "duplicate ORIGIN preserves legacy", attrs: "40010100400304ac1e0009"},
		{name: "duplicate unknown attribute preserves MP", attrs: "c0fa03010203c0fa03040506800e0900010104ac1e000900"},
		{name: "unique unknown attribute remains intact", attrs: "c0fa03ac1e00800e0900010104ac1e000900", want: "c0fa03ac1e00800e0900010104ac1f470900"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			frame := injectedMPFrame(t, test.attrs)
			wanted := test.want
			if wanted == "" {
				wanted = test.attrs
			}
			want := injectedMPFrame(t, wanted)
			renderInjectedNextHop(frame, [4]byte{172, 31, 71, 0})
			if !bytes.Equal(frame, want) {
				t.Fatalf("selected-network UPDATE differs\ngot  %X\nwant %X", frame, want)
			}
		})
	}
}

func injectedMPFrame(t *testing.T, encoded string) []byte {
	t.Helper()
	attrs, err := hex.DecodeString("4001010040020602010000fdec" + encoded)
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 4+len(attrs))
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	copy(body[4:], attrs)
	return speakerMessage(bgpUpdate, body)
}
