// Design: docs/architecture/api/process-protocol.md -- command outcomes.
package plugin

import (
	"errors"
	"slices"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// CompleteRecordAnswer publishes the outcome after the response's single
// record walk ends. The caller MUST supply the actual completed terminator,
// never a successful outcome inferred from a head or an empty document.
// The response and its walk have one consumer; concurrent use is unsupported.
// Failure leaves the response unchanged. Nil metadata is an unfinished answer.
func (response *Response) CompleteRecordAnswer(metadata *rpc.ExecuteCommandRecordAnswer) error {
	if response == nil {
		return errors.New("command response is absent")
	}
	if metadata == nil {
		return errors.New("command answer has no completed record outcome")
	}
	tail := metadata.Tail()
	status := rpc.Verdict(&tail)
	if err := metadata.Validate(status); err != nil {
		return err
	}
	if metadata.DocumentAbsent {
		if response.Data != nil {
			return errors.New("command answer declares absent data beside a document")
		}
	} else if response.Data == nil {
		return errors.New("command answer declares a document but carries no data")
	}
	owned := *metadata
	owned.Warnings = slices.Clone(metadata.Warnings)
	response.recordAnswer = &owned
	response.Status = status
	response.Error = owned.Message
	response.Warnings = slices.Clone(owned.Warnings)
	return nil
}

// RecordAnswerMetadata returns an owned snapshot of a forwarded command's
// completed outcome. The caller MUST read it after consuming the response's
// record walk. False means no completed original outcome is available, as for
// an ordinary built response or a streamed walk stopped before its terminator.
// A nil response also returns false. Concurrent use with the walk is unsupported.
func (response *Response) RecordAnswerMetadata() (*rpc.ExecuteCommandRecordAnswer, bool) {
	if response == nil {
		return nil, false
	}
	if response.recordAnswer == nil {
		return nil, false
	}
	owned := *response.recordAnswer
	owned.Warnings = slices.Clone(response.recordAnswer.Warnings)
	return &owned, true
}
