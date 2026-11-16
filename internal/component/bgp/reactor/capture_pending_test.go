// Design: docs/architecture/bgp/protocol-event-capture.md -- pending input activation and retirement.
package reactor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/capture"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestPendingCaptureSelectionBeforeCloseDrainsInput delivers both lifecycle
// signals before the worker starts. A selected Session must retain its input
// even if its close is observed before its activation signal is scheduled.
func TestPendingCaptureSelectionBeforeCloseDrainsInput(t *testing.T) {
	for range 20 {
		settings := testCapturePeer(t.TempDir())
		fc := sim.NewFakeClock(time.Unix(1700000000, 0))
		pending := newPendingSessionCapture(settings, false, fc, nil, nil)
		wire := message.PackTo(&message.Keepalive{}, nil)
		pending.recordConfig(fc.Now(), capture.OpCommit, "pending-selection", []byte(`{"hold-time":90}`))
		fc.Add(time.Nanosecond)
		pending.recordMessage(fc.Now(), 4, wire, 7, 9)
		pending.selectForPublication()
		pending.Close()
		_, events := readEvents(t, pending.Path())
		incomingCaptureConfig(t, events, "pending-selection")
		messages := 0
		for _, event := range events {
			if event.Type != capture.EventMessage {
				continue
			}
			messages++
			if !bytes.Equal(event.Data, wire) {
				t.Fatalf("selected input changed its original bytes: %x", event.Data)
			}
			if event.SourceID != 7 {
				t.Fatalf("selected source identity=%d, want 7", event.SourceID)
			}
			if event.CtxID != 9 {
				t.Fatalf("selected context identity=%d, want 9", event.CtxID)
			}
		}
		if messages != 1 {
			t.Fatalf("selected input wrote %d messages, want 1", messages)
		}
		if events[0].Event != capture.SessionCaptureStart {
			t.Fatal("selected-before-close input lost its start marker")
		}
		if events[len(events)-1].Event != capture.SessionCaptureStop {
			t.Fatal("selected-before-close input was not drained and terminated")
		}
	}
}

// TestPendingCaptureCloseBeforeSelectionCreatesNoFile retires an unpublished
// candidate first. A stale selection attempt must not rotate another Session's
// file or create a late capture after the retiring owner has joined its worker.
func TestPendingCaptureCloseBeforeSelectionCreatesNoFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "unpublished")
	settings := testCapturePeer(dir)
	fc := sim.NewFakeClock(time.Unix(1700000000, 0))
	pending := newPendingSessionCapture(settings, false, fc, nil, nil)
	pending.recordMessage(fc.Now(), 4, message.PackTo(&message.Keepalive{}, nil), 0, 0)
	pending.Close()
	pending.selectForPublication()
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unpublished input touched its capture directory: %v", err)
	}
}

// TestPendingCaptureFullQueueReportsItsGap fills the existing bounded queue
// before activation, then selects it. Every retained byte and the precise
// overflow tally must survive; a deferred file must not claim a full capture.
func TestPendingCaptureFullQueueReportsItsGap(t *testing.T) {
	settings := testCapturePeer(t.TempDir())
	fc := sim.NewFakeClock(time.Unix(1700000000, 0))
	pending := newPendingSessionCapture(settings, false, fc, nil, nil)
	pending.Start()
	wire := message.PackTo(&message.Keepalive{}, nil)
	for range captureQueueDepth + 64 {
		pending.recordMessage(fc.Now(), 4, wire, 0, 0)
		fc.Add(time.Nanosecond)
	}
	if got := pending.Drops(); got != 64 {
		t.Fatalf("pending queue overflow=%d, want 64", got)
	}
	pending.selectForPublication()
	pending.Close()
	_, events := readEvents(t, pending.Path())
	if events[0].Event != capture.SessionCaptureStart {
		t.Fatal("overflow put a gap marker before the original capture-start")
	}
	messages, gaps := 0, 0
	for _, event := range events {
		if event.Type == capture.EventMessage {
			messages++
			if !bytes.Equal(event.Data, wire) {
				t.Fatalf("retained pending frame changed: %x", event.Data)
			}
		}
		if event.Type == capture.EventSession && event.Event == capture.SessionDrops {
			gaps++
			if event.Drops != 64 {
				t.Fatalf("pending overflow record=%d, want 64", event.Drops)
			}
		}
	}
	if messages != captureQueueDepth {
		t.Fatalf("pending capture retained %d messages, want %d", messages, captureQueueDepth)
	}
	if gaps != 1 {
		t.Fatalf("pending capture wrote %d gap markers, want 1", gaps)
	}
}

// TestCapturePreparedInputDoesNotReserveFile selects another input while an
// earlier candidate is still unselected. Both must later write their own exact
// queue, with one rotation and no second writer owning the path concurrently.
func TestCapturePreparedInputDoesNotReserveFile(t *testing.T) {
	settings := testCapturePeer(t.TempDir())
	fc := sim.NewFakeClock(time.Unix(1700000000, 0))
	r := &Reactor{clock: fc}
	first := newPendingSessionCapture(settings, false, fc, nil, r)
	r.registerCapture(first)
	first.Start()
	firstWire := message.PackTo(delayOpenRemote(0x02020202), nil)
	first.recordMessage(fc.Now(), 1, firstWire, 0, 0)
	if r.captureHeldForPath(first.Path()) {
		t.Fatal("an unselected input reserved the physical file")
	}
	second := newPendingSessionCapture(settings, false, fc, nil, r)
	r.registerCapture(second)
	secondWire := message.PackTo(&message.Keepalive{}, nil)
	second.recordMessage(fc.Now(), 4, secondWire, 0, 0)
	second.selectForPublication()
	second.Close()
	r.unregisterCapture(second)
	if r.captureHeldForPath(first.Path()) {
		t.Fatal("a joined writer retained its file reservation")
	}
	first.selectForPublication()
	first.Close()
	r.unregisterCapture(first)
	_, current := readEvents(t, first.Path())
	_, previous := readEvents(t, first.Path()+rotatedSuffix)
	for _, observation := range []struct {
		events []*capture.Event
		want   []byte
	}{{current, firstWire}, {previous, secondWire}} {
		messages := 0
		for _, event := range observation.events {
			if event.Type != capture.EventMessage {
				continue
			}
			messages++
			if !bytes.Equal(event.Data, observation.want) {
				t.Fatalf("a different file owner changed captured bytes: %x", event.Data)
			}
		}
		if messages != 1 {
			t.Fatalf("file owner wrote %d original frames, want 1", messages)
		}
	}
	incomingCaptureFiles(t, settings.Capture.Directory, 2)
}
