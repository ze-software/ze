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

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// handleEvent dispatches a JSON BGP event to the appropriate handler.
func (b *bridge) handleEvent(jsonStr string) error {
	var envelope struct {
		Type  string `json:"type"`
		State string `json:"state,omitempty"`
		Peer  struct {
			Remote struct {
				Address string `json:"address"`
			} `json:"remote"`
		} `json:"peer"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &envelope); err != nil {
		b.log.Debug("flowspec: malformed event", "error", err)
		return nil //nolint:nilerr // malformed events are silently dropped
	}

	switch envelope.Type {
	case "state":
		if envelope.State != "up" && envelope.Peer.Remote.Address != "" {
			b.handlePeerDown(envelope.Peer.Remote.Address)
		}
	case "update":
		if envelope.Peer.Remote.Address != "" {
			b.handleUpdate(jsonStr, envelope.Peer.Remote.Address)
		}
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

// flowSpecOp is a single FlowSpec family operation from the event JSON.
type flowSpecOp struct {
	Action string          `json:"action"`
	NLRI   json.RawMessage `json:"nlri"`
}

// handleUpdate processes a BGP UPDATE event for FlowSpec families.
func (b *bridge) handleUpdate(jsonStr, peer string) {
	var raw map[string]json.RawMessage
	if json.Unmarshal([]byte(jsonStr), &raw) != nil {
		return
	}

	var extComms []string
	if ec, ok := raw["extended-communities"]; ok {
		if err := json.Unmarshal(ec, &extComms); err != nil {
			b.log.Debug("flowspec: malformed extended-communities", "error", err)
		}
	}

	act := parseExtendedCommunities(extComms)
	changed := false

	for famStr, data := range raw {
		fam, ok := family.LookupFamily(famStr)
		if !ok {
			continue
		}
		if fam.SAFI != family.SAFIFlowSpec {
			continue
		}

		var ops []flowSpecOp
		if json.Unmarshal(data, &ops) != nil {
			continue
		}

		for _, op := range ops {
			var tb textbuf.Buffer
			nlriKey := tb.Str(famStr).Byte('|').Str(string(op.NLRI)).String()
			switch op.Action {
			case "add":
				changed = b.handleFlowSpecAdd(peer, fam, nlriKey, op.NLRI, act) || changed
			case "del", "withdraw":
				b.rules.remove(peer, nlriKey)
				changed = true
			}
		}
	}

	if changed {
		b.applyRules()
	}
}

// handleFlowSpecAdd processes a single FlowSpec NLRI addition.
// Returns true if the rule map changed.
func (b *bridge) handleFlowSpecAdd(peer string, fam family.Family, nlriKey string, nlriJSON json.RawMessage, act flowAction) bool {
	if !act.discard && act.rateLimit == 0 && !act.hasMark {
		return false
	}

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
