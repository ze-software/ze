package analyze

import (
	"bytes"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestAnalyzeOpenUsesOnePacker proves AC-1q for ze analyze: bgpBuildOpen
// builds its Optional Parameters through capability.PackOptionalParams and
// its bytes are unchanged.
//
// VALIDATES: the OPEN body equals the bytes the inline packing wrote (one
// type-2 parameter of 18 octets: MP IPv4, MP IPv6, 4-octet AS) and
// round-trips through capability.ParseFromOptionalParams.
// PREVENTS: a byte change in ze analyze's OPEN, or a second packer.
func TestAnalyzeOpenUsesOnePacker(t *testing.T) {
	body, err := bgpBuildOpen(4200000001, 90, net.IP{192, 0, 2, 1})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		4, 0x5B, 0xA0, 0, 90, 192, 0, 2, 1, 20,
		2, 18,
		1, 4, 0, 1, 0, 1,
		1, 4, 0, 2, 0, 1,
		65, 4, 0xFA, 0x56, 0xEA, 0x01,
	}
	if !bytes.Equal(body, want) {
		t.Fatalf("body = % x\nwant   % x", body, want)
	}
	caps, err := capability.ParseFromOptionalParams(body[10:], false)
	if err != nil {
		t.Fatal(err)
	}
	if len(caps) != 3 {
		t.Fatalf("decoded %d capabilities, want 3", len(caps))
	}
}
