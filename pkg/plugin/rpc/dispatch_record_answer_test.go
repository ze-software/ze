// Design: docs/architecture/api/process-protocol.md -- command outcomes.
package rpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Goal: a dispatch value cannot bypass the existing record metadata decoder.
// Method: decode malformed metadata, including repeated outer fields, while
// checking that a failed decode leaves the caller's original value unchanged.
func TestDispatchRecordValueRefusesMalformedMetadata(t *testing.T) {
	for _, metadata := range []string{
		`null`, `{}`, `{"count":0}`, `{"count":null,"faults":0}`,
		`{"count":-1,"faults":0}`, `{"count":0,"faults":0,"extra":true}`,
		`{"count":0,"count":1,"faults":0}`,
		`{"count":0,"faults":0},"record-answer":null`,
		`null,"record-answer":{"count":0,"faults":0}`,
	} {
		t.Run(metadata, func(t *testing.T) {
			original := DispatchCommandOutput{Status: StatusDone, Data: json.RawMessage(`{"original":true}`)}
			output := original
			err := json.Unmarshal([]byte(`{"status":"warning","data":{"changed":true},"record-answer":`+metadata+`}`), &output)
			require.Error(t, err)
			require.Equal(t, original, output)
		})
	}
}
