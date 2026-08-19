// Design: docs/architecture/core-design.md -- FlowSpec-to-firewall bridge engine
// Related: register.go -- plugin registration that invokes runEngine
//
// Package flowspecfirewall translates BGP FlowSpec routes into nftables
// firewall rules. It subscribes to BGP UPDATE events for FlowSpec families,
// parses NLRI components and extended community actions, and registers
// the resulting firewall terms via the multi-owner firewall registry.
package flowspecfirewall

import (
	"encoding/json"
	"log/slog"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const maxRulesDefault = 1000

var loggerPtr atomic.Pointer[slog.Logger]

func init() { //nolint:gochecknoinits // logger bootstrap only
	loggerPtr.Store(slogutil.DiscardLogger())
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// bridge holds the runtime state of the flowspec-firewall plugin.
type bridge struct {
	rules *ruleMap
	addrs *localAddrs
	log   *slog.Logger
}

func newBridge(log *slog.Logger) *bridge {
	return &bridge{
		rules: newRuleMap(maxRulesDefault),
		addrs: newLocalAddrs(),
		log:   log,
	}
}

// handleEvent dispatches one ze-bgp JSON event to the handler for its type.
//
// bgp.ParseEvent owns the envelope: the daemon writes the event type in
// `bgp.message.type`, the peer at `bgp.peer`, and the UPDATE body under
// `bgp.update` (internal/component/bgp/format, appendFilterResultJSON and
// appendParsedUpdateJSONDirect). Reading that shape here with a second parser
// is how this bridge came to read a flat envelope no writer produces.
func (b *bridge) handleEvent(jsonStr string) error {
	event, err := bgp.ParseEvent([]byte(jsonStr))
	if err != nil {
		b.log.Debug("flowspec: malformed event", "error", err)
		return nil //nolint:nilerr // a malformed event is dropped, and the line above says so
	}

	peer := event.GetPeerAddress()
	if peer == "" {
		b.log.Debug("flowspec: event carries no peer address, dropped", "type", event.Type)
		return nil
	}

	switch event.TypeKind {
	case rpc.EventKindState:
		var state rpc.SessionState
		if err := state.UnmarshalText([]byte(event.State)); err != nil {
			b.log.Debug("flowspec: unreadable session state, the peer is treated as down",
				"state", event.State, "peer", peer, "error", err)
		}
		// Anything that is not "up" drops the peer's rules, an unreadable
		// state included: a peer whose session ze cannot read is a peer whose
		// FlowSpec routes ze must stop enforcing.
		if state != rpc.SessionStateUp {
			b.handlePeerDown(peer)
		}
	case rpc.EventKindUpdate:
		b.handleUpdate(event, peer)
	default:
		// Debug, not warn: this plugin subscribes to updates and state
		// changes, and the engine delivers End-of-RIB and route-refresh
		// markers on the same subscription. Every one of them is a legitimate
		// event this bridge has nothing to do with, so the line records the
		// drop without turning a normal session into a stream of warnings.
		b.log.Debug("flowspec: event type not handled, dropped", "type", event.Type, "peer", peer)
	}
	return nil
}

// handlePeerDown removes all rules learned from the departing peer.
func (b *bridge) handlePeerDown(peer string) {
	n := b.rules.removePeer(peer)
	if n > 0 {
		b.applyRules()
		b.log.Info("flowspec peer down, rules removed", "peer", peer, "count", n)
	}
}

// handleUpdate processes a BGP UPDATE event for FlowSpec families.
//
// The traffic action comes from the extended communities the daemon writes
// under `update.attr`, and the routes from the per-family arrays under
// `update.nlri`. bgp.ParseEvent has already resolved both: the attributes into
// Event.ExtendedCommunities and each family key into a typed family.Family.
func (b *bridge) handleUpdate(event *bgp.Event, peer string) {
	act := parseExtendedCommunities(event.ExtendedCommunities)
	changed := false

	for fam, ops := range event.FamilyOps {
		if fam.SAFI != family.SAFIFlowSpec {
			continue
		}
		for _, op := range ops {
			for _, item := range op.NLRIs {
				changed = b.handleFlowSpecOp(peer, fam, op.Action, item, act) || changed
			}
		}
	}

	if changed {
		b.applyRules()
	}
}

// handleFlowSpecOp applies one operation to one FlowSpec NLRI.
// It returns true when the rule map changed.
//
// ParseEvent hands every NLRI over decoded, so the JSON the FlowSpec writer
// produced is rebuilt here for the component parser and for the rule key. That
// re-encode is the whole cost of sharing one event parser with the rest of the
// engine, and it is paid at operator pace: a FlowSpec route arrives when a
// peer announces a filter, not on every UPDATE.
func (b *bridge) handleFlowSpecOp(peer string, fam family.Family, action routeaction.Action, item any, act flowAction) bool {
	nlriJSON, err := json.Marshal(item)
	if err != nil {
		countRuleRefused(refusedReasonParse)
		b.log.Warn("flowspec: NLRI could not be re-encoded, the route will not be enforced",
			"peer", peer, "family", fam, "error", err)
		return false
	}

	var tb textbuf.Buffer
	nlriKey := tb.Str(fam.String()).Byte('|').Str(string(nlriJSON)).String()

	switch action.Verb() {
	case routeaction.VerbInstall, routeaction.VerbReplace:
		return b.handleFlowSpecAdd(peer, fam, nlriKey, nlriJSON, act)
	case routeaction.VerbRemove:
		b.rules.remove(peer, nlriKey)
		return true
	case routeaction.VerbSkip:
		// An action that maps to neither install nor remove. It is dropped,
		// and this line is what stops the drop being silent.
		b.log.Debug("flowspec: route action not handled, the route is dropped",
			"peer", peer, "action", action.String(), "nlri", nlriKey)
	}
	return false
}

// handleFlowSpecAdd processes a single FlowSpec NLRI addition.
// Returns true if the rule map changed.
//
// Every route it does not enforce moves the refusal counter and writes a log
// line, the route carrying no traffic action included: translateFlowSpec
// returns errNoAction for that one, so it takes the same path as every other
// refusal. Returning early on it instead would leave the peer believing ze
// filters traffic that ze does not, with nothing recorded anywhere.
func (b *bridge) handleFlowSpecAdd(peer string, fam family.Family, nlriKey string, nlriJSON json.RawMessage, act flowAction) bool {
	fs, err := parseNLRIJSON(fam, nlriJSON)
	if err != nil {
		countRuleRefused(refusalReason(err))
		b.log.Warn("flowspec: NLRI parse failed, the route will not be enforced",
			"peer", peer, "nlri", nlriKey, "error", err)
		return false
	}

	dest := destPrefixFromJSON(fam, nlriJSON)
	local := dest.IsValid() && b.addrs.containsWithin(dest)

	terms, err := translateFlowSpec(fs, act, nlriKey)
	if err != nil {
		// The route is dropped here rather than registered, because a term no
		// backend can lower fails inside Backend.Apply, and Apply returns
		// before its single Flush -- one such route would leave the tables of
		// every other firewall owner unapplied in the kernel.
		countRuleRefused(refusalReason(err))
		b.log.Warn("flowspec: rule refused, it will not be enforced",
			"peer", peer, "nlri", nlriKey, "error", err)
		return false
	}

	entry := ruleEntry{terms: terms, local: local}
	if !b.rules.add(peer, nlriKey, entry) {
		countRuleRefused(refusedReasonMaxRules)
		b.log.Warn("flowspec: max rules reached, the route will not be enforced",
			"peer", peer, "nlri", nlriKey, "limit", maxRulesDefault)
		return false
	}
	return true
}

func (b *bridge) applyRules() {
	tables := b.rules.buildTable()
	firewall.RegisterTables("flowspec", tables)
	if err := firewall.ApplyAll(); err != nil {
		b.log.Warn("flowspec: firewall apply failed", "error", err)
	}
}

// runEngine is the engine-mode entry point for the flowspec-firewall plugin.
func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("flowspec-firewall plugin starting")

	p := sdk.NewWithConn("flowspec-firewall", conn)
	defer func() { _ = p.Close() }()

	b := newBridge(log)

	p.SetStartupSubscriptions(
		[]string{"update direction received", "state"},
		nil,
		"parsed",
	)

	p.OnEvent(func(jsonStr string) error {
		return b.handleEvent(jsonStr)
	})

	eb := getEventBusRef()
	if eb != nil {
		unsub1 := eb.Subscribe("interface", "addr-added", b.addrs.handleAddrAdded)
		unsub2 := eb.Subscribe("interface", "addr-removed", b.addrs.handleAddrRemoved)
		defer unsub1()
		defer unsub2()
	}

	ctx, cancel := sdk.SignalContext()
	defer cancel()

	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		log.Error("flowspec-firewall plugin failed", "error", err)
		return 1
	}

	firewall.RegisterTables("flowspec", nil)
	log.Info("flowspec-firewall plugin stopped")
	return 0
}
