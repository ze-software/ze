// Design: docs/architecture/api/process-protocol.md -- command outcomes.
package server

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: the actual legacy value slot carries original counts after collapse.
// Method: dispatch through the registered recipient, then inspect the existing
// value's JSON without deriving any count from the collapsed document's keys.
func TestDispatchRecordValueKeepsOriginalMetadata(t *testing.T) {
	for _, count := range []int{0, 2, rpc.AnswerBufferThreshold + 3} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			s := cleanupAnswerDispatcher(t, rpc.AnswerTail{Warning: true, Warnings: []string{"original warning"}}, cleanupAnswerRows(count, 1, nil))
			client := cleanupAnswerSDKValueClient(t, s, true)
			result, err := cleanupAnswerSDKValueCall(t, client, false)
			require.Error(t, err)
			wire, marshalErr := json.Marshal(result)
			require.NoError(t, marshalErr)
			var document struct {
				RecordAnswer *rpc.ExecuteCommandRecordAnswer `json:"record-answer"`
			}
			require.NoError(t, json.Unmarshal(wire, &document))
			require.NotNil(t, document.RecordAnswer)
			require.Equal(t, uint64(count), document.RecordAnswer.Count)
			require.Equal(t, uint64(1), document.RecordAnswer.Faults)
			require.Equal(t, []string{"original warning"}, document.RecordAnswer.Warnings)
		})
	}
}
