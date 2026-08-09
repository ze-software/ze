// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- detector traffic policy

package detect

import (
	"fmt"
	"net/netip"
)

// The DDoS traffic policy is an allow/deny rule set (indexed by prefix) plus a
// default action, modeled on authz (default-action) + anomaly/shape (prefix rules).
// It replaces the per-responder allowlists: the detector is the single enforcement
// point and encodes the outcome on the emitted event, because plugins receive only
// their own config subtree (an unordered map) and cannot read this policy.
//
// Evaluation is LONGEST-PREFIX-MATCH (most-specific rule wins), NOT config order:
// the plugin config delivery is an unordered map keyed by prefix (see
// policyroute/config.go), so insertion order does not survive. A /24 deny inside a
// /16 allow is therefore decided by the /24 automatically. Ties on prefix length
// resolve to deny (fail-safe: defend).

// Enum string values -- these MUST match the YANG enum leaf values verbatim.
const (
	actionAllow = "allow"
	actionDeny  = "deny"

	matchSource      = "source"
	matchDestination = "destination"
	matchAny         = "any"

	scopeDetection  = "detection"
	scopeMitigation = "mitigation"
)

// PolicyRule is one rule, keyed by prefix. Match selects which side of the attack
// the prefix is tested against; Scope selects the stage an allow/deny governs.
type PolicyRule struct {
	Prefix netip.Prefix `json:"prefix"`
	Action string       `json:"action"`
	Match  string       `json:"match"`
	Scope  string       `json:"scope"`
}

// Policy is the default action plus the prefix-indexed rules (unordered set;
// precedence is longest-prefix-match, computed in evaluate).
type Policy struct {
	DefaultAction string       `json:"default-action"`
	Rules         []PolicyRule `json:"rules"`
}

// policyOutcome is the result of evaluating the policy against an attack.
//   - Suppress: the attack is exempt from detection itself; the detector emits no
//     event and opens no incident (an allow rule at detection scope, or default allow).
//   - SuppressMitigation: the incident is recorded (observable) but responders install
//     no mitigation (an allow rule at mitigation scope, or a deny rule at detection scope).
//   - neither set: full handling (deny at mitigation scope, or default deny).
type policyOutcome struct {
	Suppress           bool
	SuppressMitigation bool
}

// evaluate selects the most-specific (longest-prefix) matching rule and returns its
// outcome, falling back to DefaultAction when nothing matches. sources is empty at
// the fast-emit stage (only the victim is known then); a source-matching rule cannot
// match without sources, so at emit it is skipped and the characterization
// re-evaluation (with sources) is authoritative. A nil/empty policy defaults to
// deny = full handling.
func (p *Policy) evaluate(victim netip.Prefix, sources []netip.Addr) policyOutcome {
	best := -1
	bestBits := -1
	if p != nil {
		for i := range p.Rules {
			if !p.Rules[i].matches(victim, sources) {
				continue
			}
			bits := p.Rules[i].Prefix.Bits()
			switch {
			case bits > bestBits:
				best, bestBits = i, bits
			case bits == bestBits && p.Rules[i].Action == actionDeny && p.Rules[best].Action != actionDeny:
				best = i // tie on specificity: deny wins (fail-safe)
			}
		}
	}
	if best >= 0 {
		return outcomeFor(p.Rules[best].Action, p.Rules[best].Scope)
	}
	return outcomeForDefault(p.defaultAction())
}

func (p *Policy) defaultAction() string {
	if p == nil || p.DefaultAction == "" {
		return actionDeny // absent policy defends by default (fail-safe)
	}
	return p.DefaultAction
}

// matches reports whether the rule applies to this attack. A destination rule tests
// the victim address; a source rule requires that EVERY known source falls within the
// prefix (one hostile out-of-prefix source keeps the attack live); `any` is either.
func (r PolicyRule) matches(victim netip.Prefix, sources []netip.Addr) bool {
	switch r.Match {
	case matchSource:
		return allSourcesIn(r.Prefix, sources)
	case matchDestination:
		return victimIn(r.Prefix, victim)
	default: // matchAny or unset
		return victimIn(r.Prefix, victim) || allSourcesIn(r.Prefix, sources)
	}
}

func victimIn(rule, victim netip.Prefix) bool {
	return victim.IsValid() && rule.Contains(victim.Addr())
}

func allSourcesIn(rule netip.Prefix, sources []netip.Addr) bool {
	if len(sources) == 0 {
		return false
	}
	for _, s := range sources {
		if !rule.Contains(s) {
			return false
		}
	}
	return true
}

func outcomeFor(action, scope string) policyOutcome {
	switch {
	case action == actionAllow && scope == scopeDetection:
		return policyOutcome{Suppress: true}
	case action == actionAllow: // mitigation scope (default)
		return policyOutcome{SuppressMitigation: true}
	case action == actionDeny && scope == scopeDetection:
		return policyOutcome{SuppressMitigation: true} // detect + record, do not block
	default: // deny at mitigation scope: full handling
		return policyOutcome{}
	}
}

func outcomeForDefault(action string) policyOutcome {
	if action == actionAllow {
		return policyOutcome{Suppress: true} // exempt everything unmatched
	}
	return policyOutcome{} // deny: full handling
}

func validAction(a string) bool { return a == actionAllow || a == actionDeny }

// validate rejects out-of-range enum values. Prefix validity is guaranteed by the
// YANG zt:ip-prefix type and re-checked at parse; this guards the string enums the
// config framework delivers.
func (p *Policy) validate() error {
	if p == nil {
		return nil
	}
	if !validAction(p.DefaultAction) {
		return fmt.Errorf("policy default-action %q must be allow or deny", p.DefaultAction)
	}
	for i := range p.Rules {
		r := p.Rules[i]
		if !validAction(r.Action) {
			return fmt.Errorf("policy rule %s: action %q must be allow or deny", r.Prefix, r.Action)
		}
		switch r.Match {
		case matchSource, matchDestination, matchAny:
		default:
			return fmt.Errorf("policy rule %s: match %q must be source, destination, or any", r.Prefix, r.Match)
		}
		switch r.Scope {
		case scopeDetection, scopeMitigation:
		default:
			return fmt.Errorf("policy rule %s: scope %q must be detection or mitigation", r.Prefix, r.Scope)
		}
	}
	return nil
}
