// Design: docs/architecture/provisioning/pxe-staging.md -- RFC 4578 client system architecture (option 93)
// Related: handler_test.go -- newTestPXEServer and the PXE request builders

package dhcpserver

import (
	"encoding/binary"
	"net"
	"testing"
)

// buildPXEDiscoverArchOption encodes a PXE DISCOVER whose option 93 carries the
// given length octet and data, followed by `trailer` as raw option bytes.
func buildPXEDiscoverArchOption(mac net.HardwareAddr, xid uint32, length byte, data, trailer []byte) []byte {
	pkt := make([]byte, 400)
	pkt[0] = opRequest
	pkt[1] = htypeEthernet
	pkt[2] = hlenEthernet
	binary.BigEndian.PutUint32(pkt[4:8], xid)
	copy(pkt[28:34], mac)
	binary.BigEndian.PutUint32(pkt[236:240], magicCookie)
	off := 240
	off += copy(pkt[off:], []byte{optMessageType, 1, msgDiscover})
	vendorClass := []byte("PXEClient:Arch:00007:UNDI:003016")
	off += copy(pkt[off:], []byte{optVendorClassID, byte(len(vendorClass))})
	off += copy(pkt[off:], vendorClass)
	off += copy(pkt[off:], []byte{optClientArch, length})
	off += copy(pkt[off:], data)
	off += copy(pkt[off:], trailer)
	pkt[off] = optEnd
	return pkt
}

// TestRFC4578ClientArchLengthEvenAndPositive proves the server honors an option
// 93 only when its length is an even number greater than zero.
// VALIDATES: RFC 4578 Section 2.1 -- octet "n" gives the number of octets
// containing architecture types, and "It MUST be an even number greater than
// zero."
// PREVENTS: a zero-length option 93 read as an architecture type, which takes
// the two octets that follow it (here a Pad and the code 7 of the next option,
// together the value 7, UEFI x64) and serves the wrong bootfile. Method: the
// DISCOVER goes through the handler, and the bootfile the reply names shows the
// architecture the server accepted.
func TestRFC4578ClientArchLengthEvenAndPositive(t *testing.T) {
	t.Parallel()

	h := newTestPXEServer(t)
	defer h.leases.stop()

	bootfile := func(pkt []byte) string {
		t.Helper()
		reply := h.handle(pkt)
		if reply == nil {
			t.Fatal("expected a DHCPOFFER, got nil")
		}
		return string(getResponseOption(reply, optBootfileName))
	}

	two := buildPXEDiscoverArchOption(net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x57, 0x01}, 0x45781, 2, []byte{0, pxeArchUEFIx64}, nil)
	four := buildPXEDiscoverArchOption(net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x57, 0x02}, 0x45782, 4, []byte{0, pxeArchUEFIx64, 0, 0}, nil)
	// RFC requirement: RFC4578-2.1-2 positive -- an option 93 whose length is an even number greater than zero, 2 or 4, is accepted: its first architecture type (7, UEFI x64) is parsed and selects the UEFI bootfile.
	for _, tc := range []struct {
		name string
		pkt  []byte
	}{{"length 2", two}, {"length 4", four}} {
		if got := parsePXEArch(tc.pkt); got != pxeArchUEFIx64 {
			t.Errorf("%s: parsePXEArch = %d, want %d", tc.name, got, pxeArchUEFIx64)
		}
		if got := bootfile(tc.pkt); got != h.pxe.BootfileUEFI {
			t.Errorf("%s: bootfile = %q, want the UEFI %q", tc.name, got, h.pxe.BootfileUEFI)
		}
	}

	// Option 93 with length 0, then a Pad and an option 7 of length 0.
	zero := buildPXEDiscoverArchOption(net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x57, 0x03}, 0x45783, 0, nil, []byte{optPad, 7, 0})
	// RFC requirement: RFC4578-2.1-2 negative -- an option 93 whose length is zero is not "greater than zero" and is refused: the two octets after it, which read as type 7, are not taken as an architecture type, and the BIOS bootfile is served.
	if got := parsePXEArch(zero); got != 0 {
		t.Errorf("zero-length option 93: parsePXEArch = %d, want 0", got)
	}
	if got := bootfile(zero); got != h.pxe.BootfileBIOS {
		t.Errorf("zero-length option 93: bootfile = %q, want the BIOS %q", got, h.pxe.BootfileBIOS)
	}
}
