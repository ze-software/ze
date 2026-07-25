// Design: docs/architecture/core-design.md -- in-process test producer for the AS112 redistribute source
//
// Package fakeas112 is a test-only internal plugin that drives the AS112
// redistribute producer namespace on demand, so `.ci` tests can exercise the
// bgp-redistribute origin-AS + community path without starting the real AS112
// DNS engine (no watchdog, no name server, no lo host addresses).
//
// It emits on the SAME (as112, route-change) typed handle the real producer
// uses (as112events.RouteChange / as112events.ProtocolID), and announces the
// SAME four fixed covering prefixes the real producer announces -- read from the
// shared as112events.CoveringPrefixesV4/V6 source so the announced set cannot
// drift from the real producer:
//
//	v4: 192.175.48.0/24 (Direct Delegation), 192.31.196.0/24 (DNAME Redirection)
//	v6: 2620:4f:8000::/48 (Direct Delegation), 2001:4:112::/48 (DNAME Redirection)
//
// It NEVER emits the /32,/128 host addresses bound on lo -- conflating the
// covering prefixes with the host addresses is the documented AS112 mistake.
//
// Command:
//
//	fakeas112 emit <add|del> [family <ipv4|ipv6|both>] [asn <N>] [community <c1,c2,...>]
//
// `family` (default both) selects which address family(ies) to emit, so a
// single-stack test can drive one family the way the real producer does under an
// ipv4-only/ipv6-only config. One `emit add` builds one batch per selected
// family (all covering prefixes of that family) tagged with OriginASN (default
// 112) and the parsed community list, so the consumer emits `update text origin
// igp origin-as <asn> [community ...]`. A `del` withdraws only the families that
// are currently announced (never a family that was not added).
package fakeas112

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	as112events "github.com/ze-software/ze/internal/plugins/as112/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	errFakeas112NoEventBus         = errors.New("fakeas112: no event bus")
	errUsageFakeas112Emit          = errors.New("usage: fakeas112 emit <add|del> [family <ipv4|ipv6|both>] [asn <N>] [community <c1,c2,...>]")
	errFakeas112MissingASNValue    = errors.New("fakeas112: `asn` requires a value")
	errFakeas112MissingCommsValue  = errors.New("fakeas112: `community` requires a value")
	errFakeas112MissingFamilyValue = errors.New("fakeas112: `family` requires a value (ipv4|ipv6|both)")
)

// Name is the canonical plugin name.
const Name = "fakeas112"

// defaultOriginASN is the AS112 project ASN (RFC 7534). Used when the driver
// omits an explicit `asn <N>`.
const defaultOriginASN uint32 = 112

// emitFamilies is the default family order the fake announces (both families),
// so a bare `emit` produces all four covering prefixes. The covering prefixes
// themselves come from the shared as112events.CoveringPrefixesV4/V6, the single
// source shared with the real producer so the announced set cannot drift.
var emitFamilies = []family.Family{family.IPv4Unicast, family.IPv6Unicast}

// loggerPtr / eventBusPtr follow the fakeredist/fakel2tp pattern: package-level
// atomic pointers set by the registration's ConfigureEngineLogger /
// ConfigureEventBus callbacks before RunEngine.
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

// coveringPrefixesFor returns the covering prefixes for a family (from the
// shared as112events source).
func coveringPrefixesFor(fam family.Family) []netip.Prefix {
	if fam.AFI == family.IPv4Unicast.AFI {
		return as112events.CoveringPrefixesV4
	}
	return as112events.CoveringPrefixesV6
}

// emitFamily publishes one per-family batch of the covering prefixes on the
// EventBus, tagged with OriginASN + community (and replayID for a replay).
// Mirrors the real as112Producer.emit: one batch per (protocol, family) tuple,
// released immediately after Emit (subscribers dispatch synchronously).
func emitFamily(action redistevents.RouteAction, fam family.Family, asn uint32, community []uint32, replayID uint64) (int, error) {
	bus := getEventBus()
	if bus == nil {
		return 0, errFakeas112NoEventBus
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = as112events.ProtocolID
	b.AFI = uint16(fam.AFI)
	b.SAFI = uint8(fam.SAFI)
	b.OriginASN = asn
	b.Community = community
	b.ReplayID = replayID
	for _, pfx := range coveringPrefixesFor(fam) {
		b.Entries = append(b.Entries, redistevents.RouteChangeEntry{Action: action, Prefix: pfx})
	}
	return as112events.RouteChange.Emit(bus, b)
}

// emitFamiliesList emits the covering prefixes for each family in fams. It does
// NOT mutate the store (runEmit and reemitAll own the store transitions), so it
// is the single low-level emit path for adds, withdraws, and replays alike.
// Returns the total in-process subscribers reached across the batches.
func emitFamiliesList(action redistevents.RouteAction, fams []family.Family, asn uint32, community []uint32, replayID uint64) (int, error) {
	total := 0
	for _, fam := range fams {
		delivered, err := emitFamily(action, fam, asn, community, replayID)
		if err != nil {
			return total, err
		}
		total += delivered
	}
	return total, nil
}

// parseAction maps the CLI token to a RouteAction. Accepts "add", "del" (the
// documented surface) and "remove" (redistevents' own token) as an alias.
func parseAction(token string) (redistevents.RouteAction, error) {
	switch token {
	case "add":
		return redistevents.ActionAdd, nil
	case "del", "remove":
		return redistevents.ActionRemove, nil
	}
	return redistevents.ActionUnspecified, fmt.Errorf("invalid action %q (want add|del)", token)
}

// parseCommunities parses a comma-separated community list ("nopeer",
// "no-export", "65535:65284", "0xFFFFFF04", ...) via attribute.ParseCommunity,
// so each token round-trips to the same uint32 the consumer renders as
// <hi>:<lo>.
func parseCommunities(spec string) ([]uint32, error) {
	var out []uint32
	for tok := range strings.SplitSeq(spec, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		c, err := attribute.ParseCommunity(tok)
		if err != nil {
			return nil, fmt.Errorf("invalid community %q: %w", tok, err)
		}
		out = append(out, c)
	}
	return out, nil
}

// parseFamily maps the `family` token to the emitted families. "both" (or an
// omitted family) selects both; "ipv4"/"ipv6" select one, mirroring the real
// producer's ipv4-only/ipv6-only address-family config.
func parseFamily(token string) ([]family.Family, error) {
	switch token {
	case "both", "":
		return emitFamilies, nil
	case "ipv4", "ipv4/unicast", "v4":
		return []family.Family{family.IPv4Unicast}, nil
	case "ipv6", "ipv6/unicast", "v6":
		return []family.Family{family.IPv6Unicast}, nil
	}
	return nil, fmt.Errorf("invalid family %q (want ipv4|ipv6|both)", token)
}

// emitArgs is the parsed form of `emit <add|del> [family <af>] [asn <N>]
// [community <list>]`.
type emitArgs struct {
	action    redistevents.RouteAction
	families  []family.Family
	asn       uint32
	community []uint32
}

// parseEmitArgs parses the emit token stream: a mandatory action followed by
// optional `family <af>`, `asn <N>`, and `community <c1,c2,...>` keyword
// arguments in any order.
func parseEmitArgs(args []string) (emitArgs, error) {
	if len(args) < 1 {
		return emitArgs{}, errUsageFakeas112Emit
	}
	action, err := parseAction(args[0])
	if err != nil {
		return emitArgs{}, err
	}
	out := emitArgs{action: action, families: emitFamilies, asn: defaultOriginASN}
	i := 1
	for i < len(args) {
		switch args[i] {
		case "family":
			if i+1 >= len(args) {
				return emitArgs{}, errFakeas112MissingFamilyValue
			}
			fams, err := parseFamily(args[i+1])
			if err != nil {
				return emitArgs{}, err
			}
			out.families = fams
			i += 2
		case "asn":
			if i+1 >= len(args) {
				return emitArgs{}, errFakeas112MissingASNValue
			}
			n, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return emitArgs{}, fmt.Errorf("invalid asn %q: %w", args[i+1], err)
			}
			out.asn = uint32(n)
			i += 2
		case "community":
			if i+1 >= len(args) {
				return emitArgs{}, errFakeas112MissingCommsValue
			}
			comms, err := parseCommunities(args[i+1])
			if err != nil {
				return emitArgs{}, err
			}
			out.community = comms
			i += 2
		default:
			return emitArgs{}, fmt.Errorf("%w: unexpected token %q", errUsageFakeas112Emit, args[i])
		}
	}
	return out, nil
}

// runEmit handles `fakeas112 emit ...`. An add emits the requested families and
// records them in the store; a del withdraws only the families currently
// announced (intersected with the request), so it never emits a withdraw for a
// family that was never added.
func runEmit(args []string) (string, error) {
	ea, err := parseEmitArgs(args)
	if err != nil {
		return "", err
	}

	var delivered int
	if ea.action == redistevents.ActionAdd {
		store.applyAdd(ea.families, ea.asn, ea.community)
		delivered, err = emitFamiliesList(redistevents.ActionAdd, ea.families, ea.asn, ea.community, 0)
	} else {
		removed, asn, community := store.applyDel(ea.families)
		delivered, err = emitFamiliesList(redistevents.ActionRemove, removed, asn, community, 0)
	}
	if err != nil {
		return "", err
	}
	logger().Debug("fakeas112: emitted",
		"action", ea.action, "families", len(ea.families), "asn", ea.asn, "communities", len(ea.community), "delivered", delivered)
	var b textbuf.Buffer
	return b.Reset().Str(`{"delivered":`).Int(int64(delivered)).Byte('}').String(), nil
}

// dispatchCommand is the OnExecuteCommand entry point.
func dispatchCommand(_, command string, args []string, _ string) (string, any, error) {
	if command == "request fakeas112 emit" {
		data, err := runEmit(args)
		if err != nil {
			return rpc.StatusError, "", err
		}
		return rpc.StatusDone, data, nil
	}
	if command == "show fakeas112 help" {
		return rpc.StatusDone, helpStub(), nil
	}
	return rpc.StatusError, "", fmt.Errorf("unknown command: %s", command)
}

// helpStub used by `fakeas112 help`.
func helpStub() string {
	return "fakeas112 emit add|del [family <ipv4|ipv6|both>] [asn <N>] [community <c1,c2,...>]"
}
