// Design: docs/architecture/core-design.md -- per-peer FlowSpec rule state

package flowspecfirewall

import (
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/firewall"
)

// tableName carries the "ze_" ownership prefix, as every other firewall owner
// does. The nft backend sweeps a table before it re-adds it only when the name
// carries that prefix, so without it a withdrawn route kept enforcing and every
// reconcile appended a second copy of each rule. An operator still reads and
// types "flowspec": firewall.StripZeTablePrefix removes the prefix for the CLI
// and the web pages.
const tableName = "ze_flowspec"

// ruleEntry holds translated firewall terms and their hook assignment.
type ruleEntry struct {
	terms []firewall.Term
	local bool // true = input hook, false = forward hook
}

// ruleMap tracks active FlowSpec rules keyed by (peer, NLRI wire bytes).
type ruleMap struct {
	mu       sync.Mutex
	peers    map[string]map[string]ruleEntry // peer -> nlriKey -> entry
	maxRules int
	count    int
}

func newRuleMap(maxRules int) *ruleMap {
	return &ruleMap{
		peers:    make(map[string]map[string]ruleEntry),
		maxRules: maxRules,
	}
}

func (rm *ruleMap) add(peer, nlriKey string, entry ruleEntry) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	peerRules, ok := rm.peers[peer]
	if !ok {
		peerRules = make(map[string]ruleEntry)
		rm.peers[peer] = peerRules
	}

	if _, exists := peerRules[nlriKey]; !exists {
		if rm.count >= rm.maxRules {
			return false
		}
		rm.count++
	}
	peerRules[nlriKey] = entry
	return true
}

func (rm *ruleMap) remove(peer, nlriKey string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	peerRules, ok := rm.peers[peer]
	if !ok {
		return
	}
	if _, exists := peerRules[nlriKey]; exists {
		delete(peerRules, nlriKey)
		rm.count--
		if len(peerRules) == 0 {
			delete(rm.peers, peer)
		}
	}
}

func (rm *ruleMap) removePeer(peer string) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	peerRules, ok := rm.peers[peer]
	if !ok {
		return 0
	}
	n := len(peerRules)
	rm.count -= n
	delete(rm.peers, peer)
	return n
}

// buildTable returns the ze_flowspec table with two chains, or nil if empty.
func (rm *ruleMap) buildTable() []firewall.Table {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.count == 0 {
		return nil
	}

	var fwdTerms, inTerms []firewall.Term
	for _, peerRules := range rm.peers {
		for _, entry := range peerRules {
			if entry.local {
				inTerms = append(inTerms, entry.terms...)
			} else {
				fwdTerms = append(fwdTerms, entry.terms...)
			}
		}
	}

	slices.SortFunc(fwdTerms, func(a, b firewall.Term) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(inTerms, func(a, b firewall.Term) int { return strings.Compare(a.Name, b.Name) })

	var chains []firewall.Chain
	if len(fwdTerms) > 0 {
		chains = append(chains, firewall.Chain{
			Name:     "flowspec-fwd",
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookForward,
			Priority: -1,
			Policy:   firewall.PolicyAccept,
			Terms:    fwdTerms,
		})
	}
	if len(inTerms) > 0 {
		chains = append(chains, firewall.Chain{
			Name:     "flowspec-in",
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookInput,
			Priority: -1,
			Policy:   firewall.PolicyAccept,
			Terms:    inTerms,
		})
	}

	return []firewall.Table{{
		Name:   tableName,
		Family: firewall.FamilyInet,
		Chains: chains,
	}}
}
