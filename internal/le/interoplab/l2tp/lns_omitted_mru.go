// Design: docs/labs/l2tp-interop.md -- omitted peer MRU and installed transmit cap.
// Related: lns_policy.go -- shared packet correlation and live daemon queries.

package l2tp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkLNSOmittedMRU requires a real default-MRU pppd exchange and measures
// Ze's own interface. A client-side MTU measurement cannot prove Ze's sends.
func checkLNSOmittedMRU(ctx context.Context, lab labOperations, timeout time.Duration) error {
	if err := waitPeerLog(ctx, lab, peerZe, logSessionIPLine, timeout); err != nil {
		return err
	}
	trace, err := readLNSPolicyTrace(ctx, lab)
	if err != nil {
		return err
	}
	if err := lnsOmittedMRUEvidence(trace); err != nil {
		return interoplab.WithSearchedOutput(err, trace)
	}
	link, err := waitPPPInterface(ctx, lab, peerZe, timeout)
	if err != nil {
		return err
	}
	output, err := execOutput(ctx, lab, peerZe, []string{"ip", "-j", "link", "show", "dev", link})
	if err != nil {
		return err
	}
	if err := lnsClientMTU(output); err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("Ze transmit interface: %w", err), output)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return err
	}
	if _, err := waitRadiusLine(ctx, lab, "Access-Request", "User-Name=alice", timeout); err != nil {
		return err
	}
	if err := checkLNSPolicyOutputForMRU(ctx, lab, 1500); err != nil {
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

func lnsOmittedMRUEvidence(trace string) error {
	lines, localAck, err := lnsLocalMRUAgreement(trace)
	if err != nil {
		return err
	}
	// Check every transmitted peer Configure-Request, including retries. The
	// last request must be acknowledged before the credential exchange starts.
	requestIndex := -1
	var requestID byte
	for index, line := range lines {
		_, packet, found := strings.Cut(line, "sent [LCP ConfReq ")
		if !found {
			continue
		}
		packet, _, closed := strings.Cut(packet, "]")
		if !closed {
			return errors.New("pppd Configure-Request record lacks closing bracket")
		}
		if strings.Contains(packet, "<mru ") {
			return errors.New("default-MRU peer explicitly sent an MRU option")
		}
		id, _, err := lnsTracePacket(lines, index, "sent [LCP ConfReq ")
		if err != nil {
			return err
		}
		requestID, requestIndex = id, index
	}
	if requestIndex < 0 {
		return errors.New("pppd trace lacks an actual Configure-Request without MRU")
	}
	ackID, index, err := lnsTracePacket(lines, requestIndex+1, "rcvd [LCP ConfAck ")
	if err != nil {
		return err
	}
	if ackID != requestID || strings.Contains(lines[index], "<mru ") {
		return errors.New("Ze did not acknowledge the peer's omitted-MRU request")
	}
	return lnsCHAPSuccess(lines, max(localAck, index)+1)
}
