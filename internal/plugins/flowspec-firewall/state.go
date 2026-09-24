// Design: docs/guide/flowspec-protected-router.md -- selected rule ordering
package flowspecfirewall

import (
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
)

const tableName = "ze_flowspec"

type ruleEntry struct {
	flow   *flowspec.FlowSpec
	action flowAction
	terms  []firewall.Term
}

type ruleMap struct {
	mu       sync.Mutex
	rules    map[string]ruleEntry
	maxRules int
}

func newRuleMap(maxRules int) *ruleMap {
	return &ruleMap{rules: make(map[string]ruleEntry), maxRules: maxRules}
}

func (rm *ruleMap) add(key string, entry ruleEntry) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if _, exists := rm.rules[key]; !exists && len(rm.rules) >= rm.maxRules {
		return false
	}
	rm.rules[key] = entry
	return true
}

func (rm *ruleMap) remove(key string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.rules, key)
}

func (rm *ruleMap) buildTable() []firewall.Table {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if len(rm.rules) == 0 {
		return nil
	}
	type orderedRule struct {
		key   string
		entry ruleEntry
	}
	ordered := make([]orderedRule, 0, len(rm.rules))
	for key, entry := range rm.rules {
		ordered = append(ordered, orderedRule{key, entry})
	}
	slices.SortFunc(ordered, func(a, b orderedRule) int {
		if a.entry.flow != nil && b.entry.flow != nil {
			if order := flowspec.Compare(a.entry.flow, b.entry.flow); order != 0 {
				return order
			}
		}
		return strings.Compare(a.key, b.key)
	})
	deferMark := false
	for _, rule := range ordered {
		deferMark = deferMark || rule.entry.action.hasMark
	}
	var chains []firewall.Chain
	if deferMark {
		var marks []firewall.Term
		for _, rule := range ordered {
			marks = append(marks, markingTerms(rule.entry)...)
		}
		marks = append(marks, firewall.Term{Name: "mark-default", Actions: []firewall.Action{firewall.Accept{}}})
		chains = append(chains, firewall.Chain{Name: flowMarkChain, Terms: marks})
	}
	var terms []firewall.Term
	for _, rule := range ordered {
		matched, actions := ruleChains(rule.entry, deferMark)
		chains = append(chains, actions...)
		terms = append(terms, matched...)
	}
	if deferMark {
		terms = append(terms, firewall.Term{Name: "finish-matching", Actions: []firewall.Action{firewall.Goto{Target: flowMarkChain}}})
	}
	// The route describes packet destinations, not interface ownership. A
	// prefix containing one local address can also contain transit addresses.
	chains = append(chains,
		firewall.Chain{Name: "flowspec-fwd", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookForward, Priority: -1, Policy: firewall.PolicyAccept, Terms: terms},
		firewall.Chain{Name: "flowspec-in", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookInput, Priority: -1, Policy: firewall.PolicyAccept, Terms: terms},
	)
	return []firewall.Table{{Name: tableName, Family: firewall.FamilyInet, Chains: chains}}
}
