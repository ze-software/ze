// Design: docs/guide/flowspec-protected-router.md -- ordered packet actions
package flowspecfirewall

import "github.com/ze-software/ze/internal/component/firewall"

// ruleChains shares a single action and limiter between a rule's alternatives.
// A goto from the match chain returns to the base chain's jump, so a packet
// matching both source and destination port is charged and sampled only once.
// The jump depth is two regardless of the number of FlowSpec rules.
func ruleChains(entry ruleEntry, deferMark bool) ([]firewall.Term, []firewall.Chain) {
	if len(entry.terms) == 0 {
		return nil, nil
	}
	if !entry.action.continueRules && entry.action.rateLimit == 0 {
		if !deferMark {
			return entry.terms, nil
		}
		terms := append([]firewall.Term(nil), entry.terms...)
		for i := range terms {
			terms[i].Actions = beforeMarkActions(terms[i].Actions)
		}
		return terms, nil
	}
	name := entry.terms[0].Name
	matchName, actionName := name+"-match", name+"-action"
	matches := make([]firewall.Term, 0, len(entry.terms))
	for _, term := range entry.terms {
		matches = append(matches, firewall.Term{
			Name:    term.Name,
			Matches: term.Matches,
			Actions: []firewall.Action{firewall.Goto{Target: actionName}},
		})
	}
	var actions []firewall.Term
	then := make([]firewall.Action, 0, len(entry.terms[0].Actions)+1)
	for _, action := range entry.terms[0].Actions {
		if _, sample := action.(firewall.Log); sample {
			// Sample every match, including packets a subsequent limiter
			// discards. Alternatives share this chain, so this runs once.
			actions = append(actions, firewall.Term{Name: name + "-sample", Actions: []firewall.Action{action}})
		} else {
			then = append(then, action)
		}
	}
	if entry.action.rateLimit > 0 && !entry.action.discard {
		dimension := firewall.RateDimensionBytes
		if entry.action.rateInPackets {
			dimension = firewall.RateDimensionPackets
		}
		// RFC 8955 Sections 7.1-2 specify a maximum rate. Matching the excess
		// and dropping it avoids letting excess packets fall through to accept.
		actions = append(actions, firewall.Term{
			Name: name + "-excess",
			Actions: []firewall.Action{firewall.Limit{
				Rate: uint64(entry.action.rateLimit), Unit: "second",
				Dimension: dimension, Over: true,
			}, firewall.Drop{}},
		})
	}
	if deferMark {
		then = beforeMarkActions(then)
	}
	if entry.action.continueRules && !entry.action.discard {
		then = append(then, firewall.Return{})
	}
	actions = append(actions, firewall.Term{Name: name, Actions: then})
	return []firewall.Term{{Name: name, Actions: []firewall.Action{firewall.Jump{Target: matchName}}}}, []firewall.Chain{
		{Name: actionName, Terms: actions},
		{Name: matchName, Terms: matches},
	}
}

const flowMarkChain = "flowspec-mark"

func beforeMarkActions(actions []firewall.Action) []firewall.Action {
	out := make([]firewall.Action, 0, len(actions))
	for _, action := range actions {
		switch action.(type) {
		case firewall.SetDSCP:
			// Header changes occur only after all applicable matches.
		case firewall.Accept:
			out = append(out, firewall.Goto{Target: flowMarkChain})
		default:
			out = append(out, action)
		}
	}
	return out
}

func markingTerms(entry ruleEntry) []firewall.Term {
	if entry.action.discard || (!entry.action.hasMark && entry.action.continueRules) {
		return nil
	}
	terms := append([]firewall.Term(nil), entry.terms...)
	for i := range terms {
		terms[i].Actions = nil
		if entry.action.hasMark {
			terms[i].Actions = append(terms[i].Actions, firewall.SetDSCP{Value: entry.action.markDSCP})
		}
		terms[i].Actions = append(terms[i].Actions, firewall.Accept{})
	}
	return terms
}
