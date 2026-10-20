// Design: docs/architecture/exabgp-bridge.md -- what the bridge refuses
// Related: bridge.go -- muxRunner.Dispatch and the reader that releases it
// Related: bridge_lines.go -- the batch that answers a failed dispatch with the error marker

package bridge

import (
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// terminatorResult is the outcome the terminator of a record answer states:
// its message is the command's failure (rpc.Verdict), and no message is
// success.
//
// A route ze answers `warning` for is success here, as ExaBGP answers it: a
// selected peer is configured for its family and its session is down or did
// not negotiate the family. A route for a family no selected peer is
// configured for is refused by ze itself (route.ErrNoPeerConfiguredFamily),
// which is ExaBGP's `carrying` check (X-23, reactor/api/command/announce.py).
func terminatorResult(payload string) pendingResult {
	tail, err := rpc.ParseAnswerTail(rpc.AnswerKindTerminator, []byte(payload))
	if err != nil {
		return pendingResult{ok: false, errText: "ze's answer could not be read: " + err.Error()}
	}
	if tail.Message != "" {
		return pendingResult{ok: false, errText: tail.Message}
	}
	return pendingResult{ok: true}
}
