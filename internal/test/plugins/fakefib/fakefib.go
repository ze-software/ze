// Design: docs/architecture/core-design.md -- test-only sysrib event emitter for FIB functional tests

package fakefib

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

const Name = "fakefib"

var (
	errNoEventBus = errors.New("fakefib: no event bus")
	errUsageEmit  = errors.New("usage: fakefib emit <add|withdraw> <family> <prefix> [nexthop <ip>] [routetype <blackhole|unreachable|prohibit>] [metric <n>] [tableid <n>] [labels <l1,l2,...>] [srv6-sid <ipv6>]")
)

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

func emitSysribChange(fam family.Family, change sysribevents.BestChangeEntry) (int, error) {
	bus := getEventBus()
	if bus == nil {
		return 0, errNoEventBus
	}
	batch := &sysribevents.BestChangeBatch{
		Family:  fam,
		Changes: []sysribevents.BestChangeEntry{change},
	}
	return sysribevents.BestChange.Emit(bus, batch)
}

func parseRouteType(s string) (sysribevents.RouteType, error) {
	switch s {
	case "blackhole":
		return sysribevents.RouteTypeBlackhole, nil
	case "unreachable":
		return sysribevents.RouteTypeUnreachable, nil
	case "prohibit":
		return sysribevents.RouteTypeProhibit, nil
	default:
		return 0, fmt.Errorf("unknown route type %q (want blackhole, unreachable, prohibit)", s)
	}
}

func parseLabels(s string) ([]uint32, error) {
	parts, count := stringsx.SplitCount(s, ",")
	labels := make([]uint32, 0, count)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid label %q: %w", p, err)
		}
		labels = append(labels, uint32(n))
	}
	return labels, nil
}

func runEmit(args []string) (json.RawMessage, error) {
	if len(args) < 3 {
		return nil, errUsageEmit
	}

	action := args[0]
	familyName := args[1]
	raw := args[2]

	fam, ok := family.LookupFamily(familyName)
	if !ok {
		return nil, fmt.Errorf("unknown family %q", familyName)
	}
	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid prefix %q: %w", raw, err)
	}

	if action == "withdraw" {
		change := sysribevents.BestChangeEntry{
			Action: routeaction.Withdraw,
			Prefix: prefix,
		}
		delivered, err := emitSysribChange(fam, change)
		if err != nil {
			return nil, err
		}
		var b textbuf.Buffer
		return json.RawMessage(b.Reset().Str(`{"delivered":`).Int(int64(delivered)).Byte('}').String()), nil
	}

	if action != "add" {
		return nil, fmt.Errorf("invalid action %q (want add or withdraw)", action)
	}

	change := sysribevents.BestChangeEntry{
		Action:   routeaction.Add,
		Prefix:   prefix,
		Protocol: "fakefib",
	}

	kvArgs := args[3:]
	if len(kvArgs)%2 != 0 {
		return nil, fmt.Errorf("attribute %q has no value", kvArgs[len(kvArgs)-1])
	}
	for i := 0; i < len(kvArgs); i += 2 {
		key := kvArgs[i]
		val := kvArgs[i+1]
		switch key {
		case "nexthop":
			addr, err := netip.ParseAddr(val)
			if err != nil {
				return nil, fmt.Errorf("invalid nexthop %q: %w", val, err)
			}
			change.NextHop = addr
		case "routetype":
			rt, err := parseRouteType(val)
			if err != nil {
				return nil, err
			}
			change.RouteType = rt
		case "metric":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid metric %q: %w", val, err)
			}
			change.Metric = uint32(n)
		case "tableid":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid tableid %q: %w", val, err)
			}
			change.TableID = uint32(n)
		case "labels":
			labels, err := parseLabels(val)
			if err != nil {
				return nil, err
			}
			change.Labels = labels
		case "srv6-sid":
			sid, err := netip.ParseAddr(val)
			if err != nil {
				return nil, fmt.Errorf("invalid srv6-sid %q: %w", val, err)
			}
			change.SRv6SID = sid
		default:
			return nil, fmt.Errorf("unknown attribute %q", key)
		}
	}

	delivered, err := emitSysribChange(fam, change)
	if err != nil {
		return nil, err
	}

	logger().Debug("fakefib: emitted", "action", action, "family", fam, "prefix", prefix, "delivered", delivered)
	var b textbuf.Buffer
	return json.RawMessage(b.Reset().Str(`{"delivered":`).Int(int64(delivered)).Byte('}').String()), nil
}

func dispatchCommand(_, command string, args []string, _ string) (string, any, error) {
	if command == "request fakefib emit" {
		data, err := runEmit(args)
		if err != nil {
			return rpc.StatusError, "", err
		}
		return rpc.StatusDone, data, nil
	}
	if command == "show fakefib help" {
		return rpc.StatusDone, map[string]any{"help": "fakefib emit add|withdraw <family> <prefix> [nexthop <ip>] [routetype <blackhole|unreachable|prohibit>] [metric <n>] [tableid <n>] [labels <l1,l2,...>] [srv6-sid <ipv6>]"}, nil
	}
	return rpc.StatusError, "", fmt.Errorf("unknown command: %s", command)
}
