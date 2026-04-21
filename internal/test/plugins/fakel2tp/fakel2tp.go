// Design: docs/architecture/core-design.md -- in-process test producer for L2TP route-change events
//
// Package fakel2tp is a test-only internal plugin that emits synthetic
// route-change batches on the L2TP event namespace, allowing .ci tests
// to drive the bgp-redistribute consumer without a real L2TP subsystem
// (no kernel modules, no UDP listener, no PPP).
//
// Commands:
//
//	fakel2tp emit add <family> <prefix>
//	fakel2tp emit remove <family> <prefix>
//
// Each invocation builds one single-entry RouteChangeBatch and emits it
// via l2tpevents.RouteChange. The L2TP events package registers "l2tp"
// as a redistevents producer at package init, so bgp-redistribute
// discovers it automatically.
package fakel2tp

import (
	"fmt"
	"log/slog"
	"net/netip"
	"sync/atomic"

	l2tpevents "codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Name is the canonical plugin name.
const Name = "fakel2tp"

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

var eventBusPtr atomic.Pointer[ze.EventBus]

func setEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

func emitOnce(action redistevents.RouteAction, fam family.Family, prefix netip.Prefix) (int, error) {
	bus := getEventBus()
	if bus == nil {
		return 0, fmt.Errorf("fakel2tp: no event bus")
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = l2tpevents.ProtocolID
	b.AFI = uint16(fam.AFI)
	b.SAFI = uint8(fam.SAFI)
	b.Entries = append(b.Entries, redistevents.RouteChangeEntry{
		Action: action,
		Prefix: prefix,
	})
	return l2tpevents.RouteChange.Emit(bus, b)
}

func parseAction(token string) (redistevents.RouteAction, error) {
	if token == "add" {
		return redistevents.ActionAdd, nil
	}
	if token == "remove" {
		return redistevents.ActionRemove, nil
	}
	return redistevents.ActionUnspecified, fmt.Errorf("invalid action %q (want add|remove)", token)
}

func parseFamily(token string) (family.Family, error) {
	if f, ok := family.LookupFamily(token); ok {
		return f, nil
	}
	return family.Family{}, fmt.Errorf("unknown family %q", token)
}

func runEmit(args []string) (string, error) {
	if len(args) != 3 {
		return "", fmt.Errorf("usage: fakel2tp emit <add|remove> <family> <prefix>")
	}
	action, err := parseAction(args[0])
	if err != nil {
		return "", err
	}
	fam, err := parseFamily(args[1])
	if err != nil {
		return "", err
	}
	prefix, err := netip.ParsePrefix(args[2])
	if err != nil {
		return "", fmt.Errorf("invalid prefix %q: %w", args[2], err)
	}
	delivered, err := emitOnce(action, fam, prefix)
	if err != nil {
		return "", err
	}
	logger().Debug("fakel2tp: emitted",
		"action", action, "family", fam, "prefix", prefix, "delivered", delivered)
	return fmt.Sprintf(`{"delivered":%d}`, delivered), nil
}

// dispatchCommand is the OnExecuteCommand entry point.
func dispatchCommand(_, command string, args []string, _ string) (string, string, error) {
	if command == "fakel2tp emit" {
		data, err := runEmit(args)
		if err != nil {
			return rpc.StatusError, "", err
		}
		return rpc.StatusDone, data, nil
	}
	if command == "fakel2tp help" {
		return rpc.StatusDone, "fakel2tp emit add|remove <family> <prefix>", nil
	}
	return rpc.StatusError, "", fmt.Errorf("unknown command: %s", command)
}
