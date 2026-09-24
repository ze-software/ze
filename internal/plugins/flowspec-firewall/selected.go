// Design: docs/guide/flowspec-protected-router.md -- selected FlowSpec installation
package flowspecfirewall

import (
	"encoding/hex"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// handleSelected installs only the RIB's authorized winning path. Raw received
// UPDATEs never call this handler. The event owns its NLRI and attributes.
func (b *bridge) handleSelected(change *ribevents.FlowSpecChange) {
	if change.Family.SAFI != family.SAFIFlowSpec {
		return
	}
	b.selectedMu.Lock()
	defer b.selectedMu.Unlock()
	if b.stopped {
		return
	}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	identity, err := nlrisplit.GetPrefixKey(change.Family)(change.NLRI, scratch[:], change.Withdraw)
	if err != nil {
		countRuleRefused(refusedReasonParse)
		b.log.Warn("flowspec: selected NLRI framing is malformed", "error", err)
		return
	}
	var keybuf textbuf.Buffer
	key := keybuf.Str(change.Family.String()).Byte('|').Str(hex.EncodeToString(identity)).String()
	// Replacement first removes the previous action, including when the new
	// action cannot be installed. An old discard must not survive a replace.
	b.rules.remove(key)
	if change.Withdraw {
		b.applyRules()
		return
	}
	if len(change.ExtendedCommunities)%8 != 0 || len(change.IPv6ExtendedCommunities)%20 != 0 {
		countRuleRefused(refusedReasonParse)
		b.log.Warn("flowspec: selected extended communities are truncated", "nlri", key)
		b.applyRules()
		return
	}
	fs, err := flowspec.ParseFlowSpec(change.Family, change.NLRI)
	if err != nil {
		countRuleRefused(refusedReasonParse)
		b.log.Warn("flowspec: selected NLRI is malformed", "error", err)
		b.applyRules()
		return
	}
	communities := make([]string, 0, len(change.ExtendedCommunities)/8+len(change.IPv6ExtendedCommunities)/20)
	for raw := change.ExtendedCommunities; len(raw) >= 8; raw = raw[8:] {
		var ec attribute.ExtendedCommunity
		copy(ec[:], raw[:8])
		communities = append(communities, string(ec.AppendDecoded(nil)))
	}
	for raw := change.IPv6ExtendedCommunities; len(raw) >= 20; raw = raw[20:] {
		var ec attribute.IPv6ExtendedCommunity
		copy(ec[:], raw[:20])
		communities = append(communities, string(ec.AppendDecoded(nil)))
	}
	act := parseExtendedCommunities(communities)
	terms, err := translateFlowSpec(fs, act, key)
	if err != nil {
		countRuleRefused(refusalReason(err))
		b.log.Warn("flowspec: selected rule cannot be installed", "nlri", fs.String(), "error", err)
	} else if !b.rules.add(key, ruleEntry{flow: fs, terms: terms, action: act}) {
		countRuleRefused(refusedReasonMaxRules)
		b.log.Warn("flowspec: selected rule limit reached", "limit", maxRulesDefault)
	}
	b.applyRules()
}
