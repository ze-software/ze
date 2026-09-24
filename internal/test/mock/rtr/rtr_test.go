// Design: docs/architecture/testing/interop.md -- mock RTR data for ASPA carriers.
package rtr

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

// TestRTRMockASPAPDUWireFormat pins the draft-ietf-sidrops-8210bis-27 Section
// 5.12 layout independently of Ze's decoder: flags in the header, customer at
// offset 8, and providers at offset 12. The former 16-byte base is invalid.
func TestRTRMockASPAPDUWireFormat(t *testing.T) {
	t.Parallel()
	writer, reader := net.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	if err := reader.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := writer.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- rtrMockSendASPAPDU(writer, aspaFlag{customerAS: 64502, providers: []uint32{64500, 64501}})
	}()
	want := []byte{2, 11, 1, 0, 0, 0, 0, 20, 0, 0, 0xfb, 0xf6, 0, 0, 0xfb, 0xf4, 0, 0, 0xfb, 0xf5}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(reader, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("ASPA PDU = %x, want %x", got, want)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
