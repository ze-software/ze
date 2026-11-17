// Design: docs/architecture/api/process-protocol.md -- actionable lease answers.
package rpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSourceLeaseMissingOwnerAnswerNamesFix reaches JSON and nil-answer checks;
// each refusal must tell the caller to return the missing peer-owner member.
func TestSourceLeaseMissingOwnerAnswerNamesFix(t *testing.T) {
	for _, raw := range []string{`{"action":3,"source-receipt":7}`, `{"action":3,"source-receipt":7,"peer-owner":null}`} {
		var output SourceLeaseOutput
		err := json.Unmarshal([]byte(raw), &output)
		require.ErrorContains(t, err, "peer-owner", "the refusal must name the required answer member")
	}
	var absent *SourceLeaseOutput
	err := absent.Validate(SourceLeaseInput{Action: SourceLeaseRelease, SourceReceipt: 7})
	require.ErrorContains(t, err, "peer-owner")
}
