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
