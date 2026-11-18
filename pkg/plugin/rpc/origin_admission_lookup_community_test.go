// Design: docs/architecture/api/process-protocol.md -- public snapshot cardinality.
package rpc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Last-valid and first-invalid community counts must agree with the public
// schema independently of the much wider encoded-frame bound.
func TestOriginAdmissionLookupCommunityCardinality(t *testing.T) {
	const lastValid = 65535
	const firstInvalid = 65536
	require.Equal(t, lastValid, OriginAdmissionCommunityMax)
	input, output := originAdmissionLookupFixture()
	storage := make([]uint32, firstInvalid)
	output.Snapshot.Community = storage[:lastValid]
	require.NoError(t, output.Validate(input))
	output.Snapshot.Community = storage[:firstInvalid]
	require.ErrorContains(t, output.Validate(input), "65535")
}
