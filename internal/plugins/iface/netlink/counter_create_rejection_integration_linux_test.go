// Design: docs/features/interfaces.md -- native rejection preserves live readers

//go:build integration && linux

package ifacenetlink

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// A real exclusive-name collision supplies the native rejection marker, while
// the previously created original reader remains useful for new real traffic.
func TestCounterCreateNativeCollisionRetainsOriginalReader(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, _ := counterCreateNative(t)
	original, read := counterCreateOwned(t, backend, creator, "cc-rejected")
	counterCreateUpTraffic(t, creator, original, 1)
	before := counterCreateRead(t, read)
	if before.TxBytes == 0 {
		t.Fatal("original positive control has no transmitted bytes")
	}
	created, rejectedRead, attempted, err := counterCreateCall(t, backend, counterCreateDummyRequest("cc-rejected"))
	if !errors.Is(err, unix.EEXIST) {
		t.Fatalf("actual exclusive collision=%v, want EEXIST", err)
	}
	if !counterCreateHasRejection(err) {
		t.Fatalf("actual exclusive collision has no definitive native receipt: %v", err)
	}
	if !attempted {
		t.Fatal("native rejection erased the actual create attempt")
	}
	if created != nil {
		t.Fatal("rejected create invented a second original link")
	}
	if rejectedRead != nil {
		t.Fatal("rejected create supplied a counter reader")
	}
	counterSendFrames(t, original.Attrs().Index, 2)
	after := counterCreateRead(t, read)
	if after.TxBytes <= before.TxBytes {
		t.Fatalf("native rejection retired original productive reader: before=%d after=%d", before.TxBytes, after.TxBytes)
	}
	if after.TxPackets <= before.TxPackets {
		t.Fatalf("native rejection lost original packets: before=%d after=%d", before.TxPackets, after.TxPackets)
	}
}
