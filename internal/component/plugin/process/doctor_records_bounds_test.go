package process

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestDeliveryWindowSaysWhenTheRingOverwrote fails more unwaited batches
// than the ring keeps.
//
// VALIDATES: the summary counts the kept problems and marks itself Truncated
// while an overwritten problem is still inside the window; a ring that never
// overwrote, and a window the overwritten problems have aged out of, are not
// truncated.
// PREVENTS: the delivery row reporting the ring's 64 as the plugin's failure
// count when more failed, with nothing to say the number is a floor.
func TestDeliveryWindowSaysWhenTheRingOverwrote(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "busy"})
	failure := errors.New("connection closed")
	for range deliveryProblemMax {
		proc.noteBatch(time.Millisecond, 1, failure, true)
	}
	if got := proc.DoctorRecord(time.Now()).Delivery; got.Truncated || got.Failures != deliveryProblemMax {
		t.Fatalf("a full ring that overwrote nothing: %+v", got)
	}

	for range 5 {
		proc.noteBatch(time.Millisecond, 1, failure, true)
	}
	got := proc.DoctorRecord(time.Now()).Delivery
	if got.Failures != deliveryProblemMax {
		t.Fatalf("failures %d, want the %d kept", got.Failures, deliveryProblemMax)
	}
	if !got.Truncated {
		t.Fatalf("the ring overwrote problems inside the window and the summary does not say so: %+v", got)
	}

	if later := proc.DoctorRecord(time.Now().Add(DeliveryWindow + time.Minute)).Delivery; later.Truncated {
		t.Fatalf("overwritten problems older than the window still truncate it: %+v", later)
	}
}
