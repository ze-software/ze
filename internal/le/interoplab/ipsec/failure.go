// Design: docs/architecture/testing/interop.md -- Debugging Failures, the IPsec log tails.
// Related: ipsec.go -- runAt wraps every scenario checker with checkerFailure.
package ipsec

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// failureLogLines is how many lines of each peer's log a failed checker carries.
const failureLogLines = 60

// failureLogReadMax bounds all the log reads of one failure together, so an aborted
// run waits at most this long for its evidence.
const failureLogReadMax = 20 * time.Second

// failurePeers is every peer an IPsec scenario can start, in the order a reader
// wants them: Ze first, then the peer it talks to, then the boxes on the path.
var failurePeers = []string{zePeer, swanPeer, natPeer, frrPeer}

// peerLogTail is one peer's log tail, or the reason it could not be read.
type peerLogTail struct {
	peer string
	text string
	err  error
}

// checkerFailure appends each running peer's log tail to a checker failure, so a red
// run carries its evidence after the containers are gone. A peer the scenario did
// not start is skipped. The checker's own deadline is often what failed, so the
// tails are read under a context that drops the cancellation and carries its own
// bound, failureLogReadMax, over all the reads together.
func checkerFailure(ctx context.Context, lab interoplab.CheckerLab, cause error) error {
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureLogReadMax)
	defer cancel()
	tails := make([]peerLogTail, 0, len(failurePeers))
	for _, peer := range failurePeers {
		result, err := lab.Logs(readCtx, peer, failureLogLines)
		if errors.Is(err, interoplab.ErrUnknownPeer) {
			continue
		}
		if err == nil && !result.Available {
			err = errors.New("docker logs answered with no readable log")
		}
		tails = append(tails, peerLogTail{peer: peer, text: result.Text, err: err})
	}
	return checkerFailureReport(cause, tails)
}

// checkerFailureReport renders cause followed by one section for each tail. It
// wraps cause, so errors.Is still finds what the checker returned.
func checkerFailureReport(cause error, tails []peerLogTail) error {
	var report textbuf.Buffer
	for _, tail := range tails {
		report.Str("\n--- ").Str(tail.peer).Str(" log")
		if tail.err != nil {
			report.Str(": ").Err(tail.err).Str(" ---")
			continue
		}
		report.Str(" (last ").Str(strconv.Itoa(failureLogLines)).Str(" lines) ---\n").Str(tail.text)
	}
	return fmt.Errorf("%w%s", cause, report.String())
}
