package types

import "testing"

// TestRouteActionVerb pins the producer-owned action -> forwarding-op mapping
// that the FIB backends share.
//
// VALIDATES: RouteAction.Verb maps each action to the forwarding-plane op a FIB
// backend performs (Add->Install, Update->Replace, Withdraw/Del->Remove,
// Unspecified/unknown->Skip).
// PREVENTS: the kernel/vpp/p4 FIB backends each re-encoding that Withdraw and
// Del both mean remove and that Unspecified is a no-op -- they now derive it
// from this one method on the type they already depend on.
func TestRouteActionVerb(t *testing.T) {
	tests := []struct {
		action RouteAction
		want   RouteVerb
	}{
		{RouteActionAdd, RouteVerbInstall},
		{RouteActionUpdate, RouteVerbReplace},
		{RouteActionWithdraw, RouteVerbRemove},
		{RouteActionDel, RouteVerbRemove},
		{RouteActionUnspecified, RouteVerbSkip},
		{RouteAction(99), RouteVerbSkip},
	}
	for _, tt := range tests {
		if got := tt.action.Verb(); got != tt.want {
			t.Errorf("RouteAction(%d).Verb() = %d, want %d", tt.action, got, tt.want)
		}
	}
}

// TestRouteActionVerbNoAlloc guards the hot FIB install path: Verb is a value
// enum and must not allocate.
func TestRouteActionVerbNoAlloc(t *testing.T) {
	if n := testing.AllocsPerRun(100, func() { _ = RouteActionWithdraw.Verb() }); n != 0 {
		t.Errorf("RouteAction.Verb allocated %v times, want 0", n)
	}
}
