//go:build !linux

// Design: docs/architecture/chaos-web-dashboard.md — netns-scoped interface fault actions (non-Linux stubs)
//
// Interface manipulation via netlink is Linux-only; on other platforms these
// actions are a no-op so the chaos engine still compiles and runs. The real
// kernel effect is exercised by the Linux integration test.

package peer

import (
	"fmt"

	"github.com/ze-software/ze/internal/chaos/engine"
)

func executeIfaceLinkFlap(action engine.ChaosAction, emit func(Event)) chaosResult {
	params := engine.ParseIfaceFaultParams(action.Params)
	if params.Iface != "" {
		emit(Event{Type: EventError, Err: fmt.Errorf("iface-link-flap %s: interface faults require linux", params.Iface)})
	}
	return chaosResult{Disconnected: false}
}

func executeIfaceAddrRemove(action engine.ChaosAction, emit func(Event)) chaosResult {
	params := engine.ParseIfaceFaultParams(action.Params)
	if params.Iface != "" {
		emit(Event{Type: EventError, Err: fmt.Errorf("iface-addr-remove %s: interface faults require linux", params.Iface)})
	}
	return chaosResult{Disconnected: false}
}
