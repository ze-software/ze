// Design: docs/architecture/core-design.md -- in-process test producer for redistevents
//
// Package fakeredist is a test-only internal plugin that registers itself as
// a redistribute source ("fakeredist") and a redistevents producer, then
// exposes a CommandDecl surface for `.ci` tests to drive synthetic route-
// change batches.
//
// Commands:
//
//	fakeredist emit add <family> <prefix> [<nexthop>]
//	fakeredist emit remove <family> <prefix>
//	fakeredist emit-burst <N> add <family> <base-prefix>
//	fakeredist emit-burst <N> remove <family> <base-prefix>
//
// Each `emit` invocation builds one single-entry RouteChangeBatch and emits
// it via the local typed handle. `emit-burst` emits N single-entry batches
// sequentially with the host portion auto-incremented from base; the test
// fixture sized N=500 covers the burst .ci scenario (AC-13).
//
// fakeredist must register at init() so bgp-redistribute can enumerate it
// via `redistevents.Producers()` during its own startup. The plugin only
// runs when the operator (or .ci test) invokes it as `ze.fakeredist`, so
// shipping it in production all.go is harmless: zero runtime cost, no
// background goroutine, no implicit emission.

package fakeredist

import (
	"fmt"
	"log/slog"
	"net/netip"
	"strconv"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Name is the canonical plugin name.
const Name = "fakeredist"

// ProtocolName is the redistevents protocol name AND the redistribute source
// name. They must agree because bgp-redistribute uses the protocol name as
// both Origin and Source when consulting the evaluator.
const ProtocolName = "fakeredist"

// loggerPtr / eventBusPtr follow the sysrib pattern: package-level atomic
// pointers set by the registration's ConfigureEngineLogger / ConfigureEventBus
// callbacks before RunEngine.
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

// Handle is the local typed event handle for (fakeredist, route-change).
// Both this package and bgp-redistribute call events.Register with the same
// (namespace, eventType, T = *RouteChangeBatch) tuple; the events registry
// is idempotent on that contract.
var Handle = events.Register[*redistevents.RouteChangeBatch](ProtocolName, redistevents.EventType)

// ProtocolID is the numeric identity allocated for fakeredist by the
// redistevents registry. Captured at init for cheap comparisons elsewhere.
var ProtocolID redistevents.ProtocolID

// emitOnce builds and emits a one-entry batch. Returns the count of
// in-process subscribers reached and any emit error.
func emitOnce(action redistevents.RouteAction, fam family.Family, prefix netip.Prefix, nh netip.Addr) (int, error) {
	bus := getEventBus()
	if bus == nil {
		return 0, fmt.Errorf("fakeredist: no event bus")
	}
	if ProtocolID == redistevents.ProtocolUnspecified {
		// Defensive: should never happen because init() registers.
		return 0, fmt.Errorf("fakeredist: protocol id not registered")
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = ProtocolID
	b.AFI = uint16(fam.AFI)
	b.SAFI = uint8(fam.SAFI)
	b.Entries = append(b.Entries, redistevents.RouteChangeEntry{
		Action:  action,
		Prefix:  prefix,
		NextHop: nh,
	})
	return Handle.Emit(bus, b)
}

// parseAction maps the CLI token "add" / "remove" to a RouteAction. Returns
// ActionUnspecified + error for any other input so the command handler can
// surface a CLI-level error string.
func parseAction(token string) (redistevents.RouteAction, error) {
	if token == "add" {
		return redistevents.ActionAdd, nil
	}
	if token == "remove" {
		return redistevents.ActionRemove, nil
	}
	return redistevents.ActionUnspecified, fmt.Errorf("invalid action %q (want add|remove)", token)
}

// parseFamily looks up the canonical family by string ("ipv4/unicast",
// "ipv6/unicast", ...). Returns the zero family on error.
func parseFamily(token string) (family.Family, error) {
	if f, ok := family.LookupFamily(token); ok {
		return f, nil
	}
	return family.Family{}, fmt.Errorf("unknown family %q", token)
}

// parseEmitArgs splits "add|remove <family> <prefix> [<nexthop>]" tokens.
func parseEmitArgs(args []string) (redistevents.RouteAction, family.Family, netip.Prefix, netip.Addr, error) {
	if len(args) < 3 || len(args) > 4 {
		return 0, family.Family{}, netip.Prefix{}, netip.Addr{},
			fmt.Errorf("usage: fakeredist emit <add|remove> <family> <prefix> [<nexthop>]")
	}
	action, err := parseAction(args[0])
	if err != nil {
		return 0, family.Family{}, netip.Prefix{}, netip.Addr{}, err
	}
	fam, err := parseFamily(args[1])
	if err != nil {
		return 0, family.Family{}, netip.Prefix{}, netip.Addr{}, err
	}
	prefix, err := netip.ParsePrefix(args[2])
	if err != nil {
		return 0, family.Family{}, netip.Prefix{}, netip.Addr{}, fmt.Errorf("invalid prefix %q: %w", args[2], err)
	}
	var nh netip.Addr
	if len(args) == 4 {
		nh, err = netip.ParseAddr(args[3])
		if err != nil {
			return 0, family.Family{}, netip.Prefix{}, netip.Addr{}, fmt.Errorf("invalid nexthop %q: %w", args[3], err)
		}
	}
	return action, fam, prefix, nh, nil
}

// runEmit handles `fakeredist emit ...`.
func runEmit(args []string) (string, error) {
	action, fam, prefix, nh, err := parseEmitArgs(args)
	if err != nil {
		return "", err
	}
	delivered, err := emitOnce(action, fam, prefix, nh)
	if err != nil {
		return "", err
	}
	logger().Debug("fakeredist: emitted",
		"action", action, "family", fam, "prefix", prefix, "delivered", delivered)
	return fmt.Sprintf(`{"delivered":%d}`, delivered), nil
}

// burstMaxN is the upper bound on a single emit-burst invocation. Sized to
// cover any realistic .ci scenario (AC-13 uses N=500) while preventing a
// pathological CLI invocation from wedging the daemon. fakeredist ships in
// production all.go, so this guard runs in production too.
const burstMaxN = 1_000_000

// runEmitBurst handles `fakeredist emit-burst <N> <add|remove> <family>
// <base-prefix>`. Emits N single-entry batches with the host portion of the
// base prefix auto-incremented (so peers see N distinct prefixes).
func runEmitBurst(args []string) (string, error) {
	if len(args) != 4 {
		return "", fmt.Errorf("usage: fakeredist emit-burst <N> <add|remove> <family> <base-prefix>")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n <= 0 {
		return "", fmt.Errorf("invalid burst count %q (want positive integer)", args[0])
	}
	if n > burstMaxN {
		return "", fmt.Errorf("burst count %d exceeds maximum %d", n, burstMaxN)
	}
	action, err := parseAction(args[1])
	if err != nil {
		return "", err
	}
	fam, err := parseFamily(args[2])
	if err != nil {
		return "", err
	}
	base, err := netip.ParsePrefix(args[3])
	if err != nil {
		return "", fmt.Errorf("invalid base-prefix %q: %w", args[3], err)
	}

	// Iterate addresses starting at base.Addr() and increment each step. We
	// keep the same prefix length for every emit; the address moves.
	addr := base.Addr()
	bits := base.Bits()
	delivered := 0
	emitted := 0
	for range n {
		entry, err := netip.ParsePrefix(addr.String() + "/" + strconv.Itoa(bits))
		if err != nil {
			return "", fmt.Errorf("internal: build entry prefix: %w", err)
		}
		d, err := emitOnce(action, fam, entry, netip.Addr{})
		if err != nil {
			// Report the partial-success counts in the error so callers can
			// tell exactly how many emissions landed before the failure.
			return "", fmt.Errorf("emit %d/%d (delivered %d) failed: %w", emitted, n, delivered, err)
		}
		emitted++
		delivered += d
		addr = addr.Next()
		if !addr.IsValid() {
			break
		}
	}
	return fmt.Sprintf(`{"delivered":%d,"emitted":%d}`, delivered, emitted), nil
}

// dispatchCommand is the OnExecuteCommand entry point. The engine routes
// commands by prefix; we receive the matched prefix as `command` and the
// remaining tokens as `args`.
func dispatchCommand(_, command string, args []string, _ string) (string, string, error) {
	if command == "fakeredist emit" {
		data, err := runEmit(args)
		if err != nil {
			return rpc.StatusError, "", err
		}
		return rpc.StatusDone, data, nil
	}
	if command == "fakeredist emit-burst" {
		data, err := runEmitBurst(args)
		if err != nil {
			return rpc.StatusError, "", err
		}
		return rpc.StatusDone, data, nil
	}
	if command == "fakeredist help" {
		return rpc.StatusDone, helpStub(), nil
	}
	return rpc.StatusError, "", fmt.Errorf("unknown command: %s", command)
}

// helpStub used by `fakeredist help`.
func helpStub() string {
	return "fakeredist emit add|remove <family> <prefix> [<nexthop>]\n" +
		"fakeredist emit-burst <N> add|remove <family> <base-prefix>"
}
