// Design: docs/architecture/testing/interop.md -- change encapsulation without rebuilding an SA.
// Related: migration.go -- fixed-header contract; the vendored netlink library owns socket framing.
package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestMigrationPayloadPreservesTheFixedIdentity pins Linux's 132-byte migration UAPI.
func TestMigrationPayloadPreservesTheFixedIdentity(t *testing.T) {
	selected := fixtureState()
	for index := range selected.selector {
		selected.selector[index] = byte(index + 1)
	}
	got, err := migrationPayload(selected, encapsulation{form: formUDP, sourcePort: 4500, targetPort: 4500})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 160 {
		t.Fatalf("payload = %d bytes, want fixed header 132 plus ENCAP 28", len(got))
	}
	source, target := selected.key.source.As4(), selected.key.target.As4()
	if !bytes.Equal(got[:4], target[:]) {
		t.Fatal("old destination changed")
	}
	if binary.BigEndian.Uint32(got[16:20]) != selected.key.spi {
		t.Fatal("SPI is not the selected SPI in network order")
	}
	if binary.NativeEndian.Uint16(got[20:22]) != 2 {
		t.Fatal("old address family is not AF_INET")
	}
	if got[22] != 50 {
		t.Fatal("old state protocol is not ESP")
	}
	if !bytes.Equal(got[24:28], target[:]) {
		t.Fatal("new destination differs from installed destination")
	}
	if !bytes.Equal(got[40:44], source[:]) {
		t.Fatal("new source differs from installed source")
	}
	if !bytes.Equal(got[64:120], selected.selector[:]) {
		t.Fatal("opaque vendor selector was reconstructed or changed")
	}
	if binary.NativeEndian.Uint32(got[120:124]) != selected.reqid {
		t.Fatal("reqid changed")
	}
	if binary.NativeEndian.Uint16(got[128:130]) != 2 {
		t.Fatal("new address family is not AF_INET")
	}
	for _, padding := range [][]byte{got[4:16], got[23:24], got[28:40], got[44:64], got[124:128], got[130:132]} {
		if !bytes.Equal(padding, make([]byte, len(padding))) {
			t.Fatal("a mark, flag or reserved byte changed")
		}
	}
	if binary.NativeEndian.Uint16(got[132:134]) != 28 {
		t.Fatal("ENCAP attribute length is not 28")
	}
	if binary.NativeEndian.Uint16(got[134:136]) != 4 {
		t.Fatal("the only attribute is not XFRMA_ENCAP")
	}
	if binary.NativeEndian.Uint16(got[136:138]) != 2 {
		t.Fatal("encapsulation type is not UDP_ENCAP_ESPINUDP")
	}
	if binary.BigEndian.Uint16(got[138:140]) != 4500 {
		t.Fatal("source port is not network-order 4500")
	}
	if binary.BigEndian.Uint16(got[140:142]) != 4500 {
		t.Fatal("target port is not network-order 4500")
	}
}

// TestMigrationPayloadExplicitlyRemovesEncapsulation prevents omission from inheriting UDP.
func TestMigrationPayloadExplicitlyRemovesEncapsulation(t *testing.T) {
	got, err := migrationPayload(fixtureState(), encapsulation{form: formBare})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 160 {
		t.Fatalf("bare change has %d bytes, want an explicit ENCAP attribute", len(got))
	}
	if !bytes.Equal(got[136:], make([]byte, 24)) {
		t.Fatal("bare change did not carry the all-zero ENCAP removal sentinel")
	}
}
