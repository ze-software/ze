// Design: docs/architecture/plugin/rib-storage-design.md -- original FlowSpec replay provenance.
package rr

import (
	"context"
	"errors"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// currentStoredFlowSpec asks the existing selecting-RIB provider for this
// exact original row after the caller's own processed UPDATE frontier. It
// preserves a lagging projection as pending, never as route absence.
func currentStoredFlowSpec(ctx context.Context, validation ribevents.ValidationRoute, route rpc.StoredRoute, afterMsgID uint64) (storedEligibility, error) {
	answer, err := ribevents.LookupFlowSpecRequest(ctx, ribevents.FlowSpecRequest{Queries: []ribevents.FlowSpecQuery{{
		Operation: ribevents.FlowSpecPathOf, Route: validation, MsgID: route.MsgID, AfterMsgID: afterMsgID}}})
	if err != nil {
		return storedEligibilityUnspecified, err
	}
	if answer.Pending {
		if len(answer.Results) != 0 {
			return storedEligibilityUnspecified, errors.New("bgp-rr: pending FlowSpec answer also claims a path verdict")
		}
		if len(answer.Routes) != 0 {
			return storedEligibilityUnspecified, errors.New("bgp-rr: pending FlowSpec answer also claims a route snapshot")
		}
		return storedEligibilityPending, nil
	}
	if len(answer.Results) != 1 {
		return storedEligibilityUnspecified, errors.New("bgp-rr: current FlowSpec answer does not match its one original path")
	}
	if len(answer.Routes) != 0 {
		return storedEligibilityUnspecified, errors.New("bgp-rr: current FlowSpec path query returned an unrequested snapshot")
	}
	current := answer.Results[0]
	if !current.Verdict {
		return storedEligibilityRejected, nil
	}
	if current.Path.MsgID != route.MsgID {
		return storedEligibilityRejected, nil
	}
	if current.Path.SourceReceipt != route.SourceReceipt {
		return storedEligibilityRejected, nil
	}
	if current.Path.SourcePolicyEpoch != route.SourcePolicyEpoch {
		return storedEligibilityRejected, nil
	}
	return storedEligibilityAccepted, nil
}
