// Design: docs/architecture/mrt.md — the daemon's PEER_INDEX_TABLE producer.
package mrt

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

// TestRFC6396DumpProducerEmitsNoViewName reads the actual RIB dumper's file,
// comparing its entire PEER_INDEX_TABLE body to external octets. Unlike checking
// utf8.ValidString on an empty test input, this detects an invalid name injected
// at the production call site, or bytes written after a zero name length.
// RFC requirement: RFC6396-4.3.1-2 positive -- writeTableDumpV2 and writePeerIndexTable produce an exact PEER_INDEX_TABLE with no View Name octets and length zero; the next two bytes are PeerCount 1 and the remaining bytes are exactly the peer entry.
func TestRFC6396DumpProducerEmitsNoViewName(t *testing.T) {
	c := New(Config{}, nil)
	path := filepath.Join(t.TempDir(), "rib.mrt")
	c.routes = mrtfmt.NewWriter(path)
	c.ribDumper = fakeRIBDumper{fn: func(v registry.RIBDumpVisitor) {
		peer := v.OnPeer("192.0.2.1", 0x12345678, [4]byte{192, 0, 2, 3}, false)
		v.OnRoute(peer, 1, 1, 24, []byte{10, 0, 0}, nil)
	}}
	c.writeTableDumpV2()
	if err := c.routes.Close(); err != nil {
		t.Fatal(err)
	}
	wire, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0, 0, 0, 0, // Collector BGP ID.
		0, 0, // View Name Length, with no name octets.
		0, 1, // Peer Count.
		2, 192, 0, 2, 3, 192, 0, 2, 1, 0x12, 0x34, 0x56, 0x78,
	}
	hdr, err := mrtfmt.DecodeHeader(wire)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.Type != mrtfmt.TypeTableDumpV2 || hdr.Subtype != mrtfmt.TDV2PeerIndexTable || int(hdr.Length) != len(want) {
		t.Fatalf("producer PEER_INDEX_TABLE header=%+v", hdr)
	}
	if !bytes.Equal(wire[12:12+len(want)], want) {
		t.Fatalf("producer PEER_INDEX_TABLE body=%x want=%x", wire[12:12+len(want)], want)
	}
}
