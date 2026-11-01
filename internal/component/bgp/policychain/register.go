package policychain

import (
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func init() {
	// The match filter type runs at a BGP chain over the conditions
	// MatchConditions holds (match.go). A check that cannot answer fails the
	// route closed, as it does at a Loc-RIB chain.
	MustRegisterEvaluator(policy.MatchFilterType, rpc.OnErrorReject, compileMatchEvaluator)
}
