// Design: docs/architecture/api/process-protocol.md -- one record walk per value answer.
package sdk

import (
	"errors"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// executeCommandRecordOutput counts the original walk while the existing
// collapse holds its document. CollapseRecords owns header/schema preflight,
// so an invalid header never enters the producer. This explicitly buffered
// API has no collection-wide message cap and never walks its producer twice.
func executeCommandRecordOutput(status string, records Records) (*rpc.ExecuteCommandOutput, error) {
	metadata := &rpc.ExecuteCommandRecordAnswer{Message: answerFailureMessage(status, nil), Warning: status == rpc.StatusWarning}
	var walkErr error
	document, err := rpc.CollapseRecords(records.Key, records.Fields, func(yield func(rpc.Record) bool) {
		for row := range rpc.HeldRecords(records.Rows) {
			// Check the combined count before incrementing either side of it.
			if metadata.Count == ^uint64(0)-metadata.Faults {
				walkErr = errors.New("execute-command record counts overflow")
				return
			}
			switch {
			case len(row.Fault) > 0:
				metadata.Faults++
			case len(row.Item) > 0:
				metadata.Count++
			}
			// CollapseRecords refuses an empty row and stops this same producer.
			if !yield(row) {
				return
			}
		}
	})
	if err != nil {
		return nil, err
	}
	if walkErr != nil {
		return nil, walkErr
	}
	tail := metadata.Tail()
	return &rpc.ExecuteCommandOutput{Status: rpc.Verdict(&tail), Data: document, RecordAnswer: metadata}, nil
}
