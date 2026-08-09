// Design: docs/architecture/isis/isis-3-l2-transport.md -- MTU expose + neighbor inference

package transport

import "testing"

func TestExposeInterfaceMTU(t *testing.T) {
	// VALIDATES: AC-3 the transport exposes the ioctl interface MTU so the
	// engine can size the Padding TLV. The transport itself never pads.
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)
	_ = tr.HandleLinkUp("eth0")

	mtu, ok := tr.InterfaceMTU("eth0")
	if !ok {
		t.Fatal("MTU not available for open circuit")
	}
	if mtu != 1500 {
		t.Errorf("MTU = %d, want 1500", mtu)
	}
	if _, ok := tr.InterfaceMTU("nope"); ok {
		t.Error("MTU reported for a circuit that is not open")
	}
}

func TestInferNeighborMTU(t *testing.T) {
	// VALIDATES: AC-4 a received padded-Hello frame size maps to an inferred
	// neighbor MTU. ISO/IEC 10589 sec 8.2.3: a padded Hello lets a receiver
	// infer the sender's MTU from the frame size.
	cases := []struct {
		name      string
		frameSize int
		want      int
	}{
		// A frame padded to fill a 1500-MTU link: 802.3 length field reflects
		// LLC+PDU; the inferred neighbor MTU is the LLC+PDU payload size.
		{"full-1500", FrameHeaderLen + (1500 - LLCHeaderLen), 1500},
		{"smaller-1492", FrameHeaderLen + (1492 - LLCHeaderLen), 1492},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := InferNeighborMTU(tc.frameSize)
			if got != tc.want {
				t.Errorf("InferNeighborMTU(%d) = %d, want %d", tc.frameSize, got, tc.want)
			}
		})
	}
}

func TestInferNeighborMTUTooSmall(t *testing.T) {
	// VALIDATES: a frame smaller than the header yields a non-positive inferred
	// MTU (no underflow; caller treats <=0 as "unknown").
	if got := InferNeighborMTU(FrameHeaderLen - 1); got > 0 {
		t.Errorf("InferNeighborMTU(short) = %d, want <= 0", got)
	}
}

func TestMTUMismatch(t *testing.T) {
	// VALIDATES: AC-4 a neighbor padded Hello smaller than the local MTU is
	// surfaced as a mismatch the engine can act on (per ISO/IEC 10589 sec 8.2.3).
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)
	_ = tr.HandleLinkUp("eth0")

	var gotLocal, gotNeighbor int
	var fired bool
	tr.OnMTUMismatch(func(name string, localMTU, neighborMTU int) {
		fired = true
		gotLocal, gotNeighbor = localMTU, neighborMTU
	})

	// Peer padded its Hello to a 1492-byte link; local MTU is 1500.
	neighborFrameSize := FrameHeaderLen + (1492 - LLCHeaderLen)
	tr.ObserveNeighborFrame("eth0", neighborFrameSize)

	if !fired {
		t.Fatal("MTU mismatch callback did not fire")
	}
	if gotLocal != 1500 || gotNeighbor != 1492 {
		t.Errorf("mismatch local=%d neighbor=%d, want 1500/1492", gotLocal, gotNeighbor)
	}
}

func TestMTUNoMismatchWhenEqual(t *testing.T) {
	// VALIDATES: AC-4 equal MTUs do not raise a spurious mismatch (R-5).
	be := newFakeBackend()
	tr := New(be)
	tr.EnableInterface("eth0", Level2)
	_ = tr.HandleLinkUp("eth0")

	fired := false
	tr.OnMTUMismatch(func(string, int, int) { fired = true })
	tr.ObserveNeighborFrame("eth0", FrameHeaderLen+(1500-LLCHeaderLen))
	if fired {
		t.Error("MTU mismatch fired for equal MTUs")
	}
}
