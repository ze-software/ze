// The guard that stops the retired `bgp-redistribute` ingress filter's shape
// from returning. That filter asked this package to judge a route a peer had
// just announced, a question that names no importing protocol, and passed "".
// Every rule the config loader builds carries the enclosing `destination` key,
// so every one of them rejected, and one `redistribute` block anywhere in the
// config discarded every route from every peer with no log line.
//
// These tests drive the three entry points directly, which is the only way to
// reach the shape: no production caller passes an empty name today, so a test
// that went through one would prove nothing about the next caller written.

package redistribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// unnamedProtocolRoute is the route the retired filter judged: a route a peer
// announced, which has an origin and a source and no importing protocol at all.
var unnamedProtocolRoute = RedistRoute{Origin: "bgp", Family: family.IPv4Unicast, Source: "ebgp"}

// loaderShapedRules are the rules the config loader builds for
// `redistribute { destination ospf { import bgp } }`. Destination is populated
// on every rule a config produces, which is what made the empty importing
// protocol reject rather than accept.
var loaderShapedRules = []ImportRule{{Source: "bgp", Destination: "ospf"}}

// TestImportRuleAcceptRefusesAnUnnamedImportingProtocol drives ImportRule.Accept
// with the value the retired filter passed.
//
// VALIDATES: an empty importing protocol is refused rather than answered.
// PREVENTS: a caller that holds no importing protocol reading a rejection as a
// routing decision.
func TestImportRuleAcceptRefusesAnUnnamedImportingProtocol(t *testing.T) {
	rule := loaderShapedRules[0]

	// The verdict for a real protocol name is unchanged, either way.
	assert.True(t, rule.Accept(unnamedProtocolRoute, "ospf"))
	assert.False(t, rule.Accept(unnamedProtocolRoute, "isis"))

	assert.PanicsWithValue(t,
		"BUG: redistribute: the importing protocol has no name, and an empty name is not a verdict",
		func() { rule.Accept(unnamedProtocolRoute, "") })
}

// TestEvaluatorAcceptRefusesAnUnnamedImportingProtocol drives the exported
// wrapper the retired filter actually called.
//
// VALIDATES: Evaluator.Accept refuses an empty importing protocol whether or
// not it holds rules. The empty-rule case is why the guard is not left to
// ImportRule.Accept alone: evaluate never enters its loop, so the callee's
// guard is unreachable and the caller would read a silent false.
// PREVENTS: the guard being deleted as a duplicate of the one below it.
func TestEvaluatorAcceptRefusesAnUnnamedImportingProtocol(t *testing.T) {
	withRules := NewEvaluator(loaderShapedRules)
	assert.True(t, withRules.Accept(unnamedProtocolRoute, "ospf"))
	assert.PanicsWithValue(t,
		"BUG: redistribute: the importing protocol has no name, and an empty name is not a verdict",
		func() { withRules.Accept(unnamedProtocolRoute, "") })

	noRules := NewEvaluator(nil)
	assert.False(t, noRules.Accept(unnamedProtocolRoute, "ospf"))
	assert.Panics(t, func() { noRules.Accept(unnamedProtocolRoute, "") })
}

// TestHasDestinationRefusesAnUnnamedDestination holds the sibling path to the
// same rule. A destination IS the importing protocol, and a false here cancels
// a replay rather than dropping a route, which is the same silence one stage
// further on.
//
// VALIDATES: HasDestination refuses an empty name.
// PREVENTS: a consumer registered under an empty name canceling its own replay.
func TestHasDestinationRefusesAnUnnamedDestination(t *testing.T) {
	ev := NewEvaluator(loaderShapedRules)

	assert.True(t, ev.HasDestination("ospf"))
	assert.False(t, ev.HasDestination("isis"))

	assert.Panics(t, func() { ev.HasDestination("") })
}

// TestUnnamedImportingProtocolIsRefusedForEveryRuleShape walks the two rule
// shapes an empty name reads differently, and requires the same refusal from
// both.
//
// VALIDATES: the refusal is a property of the QUESTION, not of the rules. A
// destination-scoped rule rejects an empty name and a destination-agnostic rule
// accepts one, so an unguarded caller reads a verdict whose value depends on
// config it never looked at.
// PREVENTS: the guard being narrowed to the scoped case, which would leave the
// agnostic case answering true and silently redistributing into a protocol
// nobody named.
func TestUnnamedImportingProtocolIsRefusedForEveryRuleShape(t *testing.T) {
	shapes := map[string]ImportRule{
		"destination-scoped":   {Source: "bgp", Destination: "ospf"},
		"destination-agnostic": {Source: "bgp"},
	}
	for name, rule := range shapes {
		t.Run(name, func(t *testing.T) {
			require.Panics(t, func() { rule.Accept(unnamedProtocolRoute, "") })
			require.Panics(t, func() { NewEvaluator([]ImportRule{rule}).Accept(unnamedProtocolRoute, "") })
		})
	}
}
