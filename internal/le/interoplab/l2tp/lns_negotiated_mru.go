// Design: docs/labs/l2tp-interop.md -- unequal configured and negotiated MRU evidence.
// Related: lns_policy.go -- shared wire correlation and daemon command path.

package l2tp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const lnsDistinctPeerMRU uint16 = 1400

// checkLNSNegotiatedMRU correlates the peer's smaller receive request with
// both daemon session views and Ze's installed transmit MTU. Config stays 1492.
func checkLNSNegotiatedMRU(ctx context.Context, lab labOperations, timeout time.Duration) error {
	if err := waitPeerLog(ctx, lab, peerZe, logSessionIPLine, timeout); err != nil {
		return err
	}
	trace, err := readLNSPolicyTrace(ctx, lab)
	if err != nil {
		return err
	}
	if err := lnsDistinctMRUEvidence(trace); err != nil {
		return interoplab.WithSearchedOutput(err, trace)
	}
	link, err := waitPPPInterface(ctx, lab, peerZe, timeout)
	if err != nil {
		return err
	}
	output, err := execOutput(ctx, lab, peerZe, []string{"ip", "-j", commandLink, commandShow, ipOptionDevice, link})
	if err != nil {
		return err
	}
	if err := lnsMeasuredLinkMTU(output, lnsDistinctPeerMRU); err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("transmit interface on Ze: %w", err), output)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return err
	}
	if _, err := waitRadiusLine(ctx, lab, "Access-Request", "User-Name=alice", timeout); err != nil {
		return err
	}
	// The aggregate view must also carry measured negotiation, rather than
	// only the specific detail view being correct.
	sessions, err := lnsPolicySessions(ctx, lab)
	if err != nil {
		return err
	}
	if len(sessions) != 1 || sessions[0].ID == 0 {
		return errors.New("expected one identified session in the negotiated-MRU aggregate view")
	}
	if err := lnsNegotiatedSessionMRU(sessions[0], lnsDistinctPeerMRU); err != nil {
		return fmt.Errorf("aggregate session negotiation: %w", err)
	}
	if err := checkLNSPolicyOutputForMRU(ctx, lab, lnsDistinctPeerMRU); err != nil {
		return err
	}
	if err := retainLNSPolicyEvidence(ctx, lab, trace, output); err != nil {
		return err
	}
	if err := stopLACDaemon(ctx, lab); err != nil {
		return err
	}
	return waitL2TPClean(ctx, lab, timeout)
}

func lnsDistinctMRUEvidence(trace string) error {
	lines, localAck, err := lnsLocalMRUAgreement(trace)
	if err != nil {
		return err
	}
	id, index, err := lnsTracePacket(lines, 0, "sent [LCP ConfReq ", "<mru 1400>")
	if err != nil {
		return err
	}
	ackID, index, err := lnsTracePacket(lines, index+1, "rcvd [LCP ConfAck ", "<mru 1400>")
	if err != nil {
		return err
	}
	if ackID != id {
		return errors.New("the identifier of Ze's MRU1400 Ack does not match the peer request")
	}
	return lnsCHAPSuccess(lines, max(localAck, index)+1)
}
