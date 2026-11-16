// Design: docs/architecture/core-design.md -- in-process test producer for L2TP route-change events
//
// Package fakel2tp is a test-only internal plugin that emits synthetic
// route-change batches on the L2TP event namespace, allowing .ci tests
// to drive the bgp-redistribute consumer without a real L2TP subsystem
// (no kernel modules, no UDP listener, no PPP).
// Its synthetic subscriber commands also invoke the real pool handlers, for
// portable tests of daemon reload and allocation. They require internal mode.
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
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"sync/atomic"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	errFakel2tpNoEventBus                     = errors.New("fakel2tp: no event bus")
	errUsageFakel2tpEmitAddremoveFamilyPrefix = errors.New("usage: fakel2tp emit <add|remove> <family> <prefix>")
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
		return 0, errFakel2tpNoEventBus
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

func runEmit(args []string) (map[string]any, error) {
	if len(args) != 3 {
		return nil, errUsageFakel2tpEmitAddremoveFamilyPrefix
	}
	action, err := parseAction(args[0])
	if err != nil {
		return nil, err
	}
	fam, err := parseFamily(args[1])
	if err != nil {
		return nil, err
	}
	prefix, err := netip.ParsePrefix(args[2])
	if err != nil {
		return nil, fmt.Errorf("invalid prefix %q: %w", args[2], err)
	}
	delivered, err := emitOnce(action, fam, prefix)
	if err != nil {
		return nil, err
	}
	logger().Debug("fakel2tp: emitted",
		"action", action, "family", fam, "prefix", prefix, "delivered", delivered)
	// A map, never pre-encoded text: a command answer is structured data, and the
	// engine refuses a payload that arrives as a JSON string (plugin.RawJSON).
	return map[string]any{"delivered": delivered}, nil
}

// commandHandlers owns the test-only command registrations and their declarations.
// It is immutable after package initialization and safe for concurrent reads.
var commandHandlers map[string]func([]string) (any, error)

func init() {
	commandHandlers = map[string]func([]string) (any, error){
		"request fakel2tp emit":               func(args []string) (any, error) { return runEmit(args) },
		"request fakel2tp pool address":       poolAddress,
		"request fakel2tp pool pppoe-address": poolPPPoEAddress,
		"request fakel2tp pool release":       poolRelease,
		"request fakel2tp pool prefix":        poolPrefix,
		"show fakel2tp help": func([]string) (any, error) {
			names := make([]string, 0, len(commandHandlers))
			for name := range commandHandlers {
				names = append(names, name)
			}
			slices.Sort(names)
			return map[string]any{"commands": names, "usage": "fakel2tp emit add|remove <family> <prefix>"}, nil
		},
	}
}

// dispatchCommand is the OnExecuteCommand entry point.
func dispatchCommand(_, command string, args []string, _ string) (string, any, error) {
	handler, ok := commandHandlers[command]
	if !ok {
		return rpc.StatusError, "", opererr.New("unknown test command "+command,
			"The synthetic subscriber did not run.", "ze show fakel2tp help")
	}
	data, err := handler(args)
	if err != nil {
		return rpc.StatusError, "", err
	}
	return rpc.StatusDone, data, nil
}
