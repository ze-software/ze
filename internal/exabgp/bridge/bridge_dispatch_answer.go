// Design: docs/architecture/exabgp-bridge.md -- what the bridge refuses
// Related: bridge.go -- muxRunner.Dispatch and the reader that fills answerDocuments
// Related: bridge_lines.go -- the batch that answers a failed dispatch with the error marker

package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ErrRouteSentNothing is a route command ze accepted and sent to no peer,
// because no selected peer has the route's family negotiated.
var ErrRouteSentNothing = errors.New("the route reached no peer")

// updateTextStatusWarning is the status ze's update-text handler answers when
// it announced and withdrew nothing (handleUpdateText,
// internal/component/bgp/plugins/cmd/update/update_text.go). The in-process
// dispatch carries it; the record wire does not, and answers `done` with the
// same document.
const updateTextStatusWarning = "warning"

// RefuseUnsentRoute answers a route command ze sent to no peer with a refusal
// carrying ze's reason, and every other answer with nil.
//
// ExaBGP answers `error` when no selected peer carries a route's family
// (X-23, `carrying` in reactor/api/command/announce.py), while ze's
// update-text handler answers `warning` with the document
// {"message": "announce ipv6/unicast: no peers have family negotiated"}. A
// sent route is answered with RouteResult instead, which always carries the
// `announced` and `withdrawn` counts, so the document alone tells the two
// apart on a transport that drops the status. ze tests the NEGOTIATED family
// of each selected peer, ExaBGP the CONFIGURED one: a peer whose session is
// down is skipped by ze's reactor before the family is read.
//
// command is the ze command dispatched; only `update text` commands are read,
// because their success document is RouteResult.
func RefuseUnsentRoute(command, status string, document json.RawMessage) error {
	if !strings.Contains(command, " update text ") {
		return nil
	}
	var answer struct {
		Announced *uint32 `json:"announced"`
		Withdrawn *uint32 `json:"withdrawn"`
		Message   string  `json:"message"`
	}
	if len(document) > 0 {
		if err := json.Unmarshal(document, &answer); err != nil {
			return fmt.Errorf("ze's answer to %q could not be read, so whether the route was sent is unknown: %w",
				command, err)
		}
	}
	if status != updateTextStatusWarning {
		if answer.Announced != nil {
			return nil
		}
		if answer.Withdrawn != nil {
			return nil
		}
		if answer.Message == "" {
			return nil
		}
	}
	return fmt.Errorf("%w: %s; give a selected peer the family in its neighbor's family block, "+
		"or select a peer whose session has it negotiated", ErrRouteSentNothing, answer.Message)
}

// answerDocumentsMax bounds the answers held between their first record and
// their terminator. The standalone runner awaits one dispatch at a time
// (Batch), so one entry is the steady state; the bound only stops an answer
// whose terminator never arrives from growing the map.
const answerDocumentsMax = 64

// answerDocuments holds the first record of each record answer in flight, by
// request id, until its terminator releases the waiter. Owned by the one
// reader goroutine (zebgpToPluginWithScanner); not safe for concurrent use.
type answerDocuments map[uint64]json.RawMessage

// note keeps the first record of answer id.
func (d answerDocuments) note(id uint64, payload string) {
	if _, held := d[id]; held {
		return
	}
	if len(d) >= answerDocumentsMax {
		slog.Warn("zebgp->plugin: too many answers without a terminator, record not kept", "id", id)
		return
	}
	tail, err := rpc.ParseAnswerTail(rpc.AnswerKindRecord, []byte(payload))
	if err != nil {
		slog.Warn("zebgp->plugin: unreadable answer record", "id", id, "error", err)
		return
	}
	d[id] = tail.Item
}

// result is the outcome the terminator of answer id states: its message is
// the command's failure (rpc.Verdict), and no message is success carrying the
// first record.
func (d answerDocuments) result(id uint64, payload string) pendingResult {
	document := d[id]
	delete(d, id)
	tail, err := rpc.ParseAnswerTail(rpc.AnswerKindTerminator, []byte(payload))
	if err != nil {
		return pendingResult{ok: false, errText: "ze's answer could not be read: " + err.Error()}
	}
	if tail.Message != "" {
		return pendingResult{ok: false, errText: tail.Message}
	}
	return pendingResult{ok: true, document: document}
}
