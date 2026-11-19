// Design: docs/research/vpp-deployment-reference.md -- runtime startup readiness

package vpp

import (
	"errors"
	"testing"
)

// A validation failure answers the startup owner before a manager can enter
// its retry loop; callers receive that exact failure and no readiness event.
func TestVPPStartupSetupFailureAnswersReceipt(t *testing.T) {
	runtimeStatsTestState(t)
	manager := &VPPManager{settings: &VPPSettings{Stats: StatsSettings{SocketPath: "/test/stats.sock", PollInterval: 30}}}
	manager.settings.Enabled = true
	receipt := make(chan error, 1)
	manager.startup = receipt
	setVPPEventBus(runtimeStatsTestBus{emit: func(string, string, any) { t.Error("failed setup published readiness") }})
	failure := manager.Run(t.Context())
	if failure == nil {
		t.Fatal("invalid settings started VPP")
	}
	select {
	case answered := <-receipt:
		if !errors.Is(answered, failure) {
			t.Fatalf("startup error=%v, want %v", answered, failure)
		}
	default:
		t.Fatal("setup failure did not answer startup owner")
	}
}
