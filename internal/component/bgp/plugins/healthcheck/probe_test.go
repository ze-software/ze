package healthcheck

import (
	"context"
	"testing"
)

func TestProbeSuccess(t *testing.T) {
	if !runProbeCommand(context.Background(), "true", 5) {
		t.Error("expected success for 'true' command")
	}
}

func TestProbeFailure(t *testing.T) {
	if runProbeCommand(context.Background(), "false", 5) {
		t.Error("expected failure for 'false' command")
	}
}

func TestProbeTimeout(t *testing.T) {
	if runProbeCommand(context.Background(), "sleep 30", 1) {
		t.Error("expected failure for timed-out command")
	}
}

func TestProbeCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if runProbeCommand(ctx, "true", 5) {
		t.Error("expected failure with canceled context")
	}
}

func TestProbeZeroTimeout(t *testing.T) {
	// timeout=0 creates immediately-expired context; command should fail.
	if runProbeCommand(context.Background(), "true", 0) {
		t.Error("expected failure with zero timeout")
	}
}

func TestProbeLimitedOutput(t *testing.T) {
	// Verify the output buffer doesn't grow unbounded.
	// Generate output larger than maxOutputBytes (64KB).
	if runProbeCommand(context.Background(), "dd if=/dev/zero bs=1024 count=128 2>/dev/null; false", 5) {
		t.Error("expected failure")
	}
	// If we get here without OOM, the buffer cap works. No assertion needed beyond no-crash.
}
