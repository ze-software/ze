// Design: docs/architecture/testing/interop.md -- foreign collector selection evidence.
// Related: check_med_pmacct_epoch.go -- correlate received routes with Peer Up identity.
package bgp

import (
	"context"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	medWholeSetScenario   = "bgp-addpath-best-path-pmacct"
	medWholeSetInjector   = "172.30.0.9"
	medWholeSetPrefix     = "10.99.77.0/24"
	medWholeSetCheckpoint = "/tmp/med-cycle-checkpoint"
	pmacctLogUpdate       = "update"
)

// checkMEDWholeSet keeps the original prefix separate from the three-route
// cycle. RFC 4271 Section 9.1.2.2 says: "The criteria MUST be applied in the order
// specified." A removes C at MED before B defeats A at Router ID. Losing A is
// therefore observable even though A was not the selected route.
func checkMEDWholeSet(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.Addr().Is4() {
		return fmt.Errorf("MED collector requires a selected IPv4 network, got %s", check.Network.IPv4)
	}
	initial := append(medWholeSetBaselineOperations(),
		operation{kind: opFRRSession, argument: zeLabAddress, timeout: 90 * time.Second},
		operation{kind: opGoBGPSession, argument: zeLabAddress, timeout: 90 * time.Second},
	)
	if err := medWholeSetRunOperations(ctx, check, 1, initial); err != nil {
		return err
	}
	network := check.Network.IPv4.Addr().As4()
	inputs := []medWholeSetCandidate{
		newMEDWholeSetCandidate(network, 9, 65004, 1, 100),
		newMEDWholeSetCandidate(network, 3, 65004, 3, 0),
	}
	for index := range inputs {
		if err := waitMEDWholeSetInput(ctx, check.Lab, &inputs[index]); err != nil {
			return medWholeSetFailure(check, 7+index, err)
		}
	}
	if err := medWholeSetRunOperations(ctx, check, 9, []operation{
		medWholeSetSaveCheckpoint(),
		{kind: opExec, peer: peerGoBGP, command: []string{
			cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, medWholeSetPrefix,
			"-a", gobgpFamilyIPv4, gobgpOrigin, "igp", "med", "50", gobgpNextHop, gobgpLabAddress,
		}},
	}); err != nil {
		return err
	}
	candidate := newMEDWholeSetCandidate(network, 5, 65005, 2, 50)
	if err := waitMEDWholeSetInput(ctx, check.Lab, &candidate); err != nil {
		return medWholeSetFailure(check, 11, err)
	}
	if err := waitMEDWholeSetSelected(ctx, check.Lab, &candidate); err != nil {
		return medWholeSetFailure(check, 12, err)
	}
	if err := medWholeSetRunOperations(ctx, check, 13, []operation{
		medWholeSetSaveCheckpoint(),
		medWholeSetFRRNetwork("no network " + medWholeSetPrefix),
	}); err != nil {
		return err
	}
	if err := waitMEDWholeSetSelected(ctx, check.Lab, &inputs[0]); err != nil {
		return medWholeSetFailure(check, 15, err)
	}
	if err := medWholeSetRunOperations(ctx, check, 16, []operation{
		medWholeSetSaveCheckpoint(),
		medWholeSetFRRNetwork("network " + medWholeSetPrefix + " route-map MED_CYCLE"),
	}); err != nil {
		return err
	}
	if err := waitMEDWholeSetSelected(ctx, check.Lab, &candidate); err != nil {
		return medWholeSetFailure(check, 18, err)
	}
	return medWholeSetRunOperations(ctx, check, 19, medWholeSetBaselineOperations())
}

// medWholeSetBaselineOperations preserves the original RFC 7911 proof: one
// prefix's two Path Identifiers never partition its election. The lower-MED
// path arrives first, both paths must reach Adj-RIB-In, and pmacct's Loc-RIB
// history must contain the better path and never the worse one.
func medWholeSetBaselineOperations() []operation {
	return []operation{
		{kind: opWaitContains, peer: peerPMACCT, command: []string{"sh", "-c", pmacctAddPathAdjInRows},
			contains: []string{pmacctAddPathBetterPath, pmacctAddPathWorsePath}, timeout: 120 * time.Second},
		{kind: opWaitContains, peer: peerPMACCT, command: []string{"sh", "-c", pmacctAddPathLocRIBRows},
			contains: []string{pmacctAddPathBetterPath}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: peerPMACCT, command: []string{"sh", "-c", pmacctAddPathLocRIBRows},
			contains: []string{pmacctAddPathBetterPath}, delay: 10 * time.Second},
		{kind: opRequireAbsent, peer: peerPMACCT, command: []string{"sh", "-c", pmacctAddPathLocRIBRows},
			absent: []string{pmacctAddPathWorsePath}, proof: []string{pmacctAddPathBetterPath}},
	}
}

func medWholeSetRunOperations(ctx context.Context, check *interoplab.CheckContext, first int, operations []operation) error {
	for index := range operations {
		if err := runOperation(ctx, check.Network, check.Lab, &operations[index]); err != nil {
			return medWholeSetFailure(check, first+index, err)
		}
	}
	return nil
}

func medWholeSetFailure(check *interoplab.CheckContext, assertion int, cause error) error {
	return fmt.Errorf("MED collector assertion %d (selected IPv4=%s): %w", assertion, check.Network.IPv4, cause)
}

// medWholeSetSaveCheckpoint MUST precede each route-changing command whose
// result waitMEDWholeSetSelected reads. The file is private to this lab container.
func medWholeSetSaveCheckpoint() operation {
	return operation{kind: opExec, peer: peerPMACCT, command: []string{
		"sh", "-c", "wc -l < " + pmacctMsgLogPath + " > " + medWholeSetCheckpoint,
	}}
}

func medWholeSetFRRNetwork(change string) operation {
	return operation{kind: opExec, peer: peerFRR, command: []string{
		cmdVtysh, "-c", frrConfigureTerminal, "-c", "router bgp 65004",
		"-c", frrAddressFamilyIPv4Unicast, "-c", change,
	}}
}
