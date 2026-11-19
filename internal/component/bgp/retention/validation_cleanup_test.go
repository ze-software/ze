// Design: docs/guide/graceful-restart.md -- exact passive validation cleanup.
package retention

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"math"
	"testing"
)

// The real cleanup codec keeps exact-width original IDs and the family and
// NO_LLGR predicates. Invalid scopes cannot become broad cleanup variants.
func TestRetainedValidationCleanupCodec(t *testing.T) {
	payload, err := EncodeCleanup(math.MaxUint64, math.MaxUint64-1, "192.0.2.50", []family.Family{family.IPv4Unicast}, true)
	require.NoError(t, err)
	var wire map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &wire))
	require.Equal(t, "18446744073709551615", wire["peer-owner"])
	require.Equal(t, "18446744073709551614", wire["connection-id"])
	envelope := rpc.EventEnvelope{Namespace: Namespace, Event: CleanupEventType, Payload: json.RawMessage(payload)}
	encoded, err := json.Marshal(envelope)
	require.NoError(t, err)
	parsed, matched := AcceptsCleanup(string(encoded))
	require.True(t, matched)
	cleanup, err := DecodeCleanup(parsed)
	require.NoError(t, err)
	require.Equal(t, uint64(math.MaxUint64), cleanup.Owner())
	require.Equal(t, uint64(math.MaxUint64-1), cleanup.Connection())
	require.True(t, cleanup.NoLLGROnly())
	require.True(t, cleanup.MatchesFamily(family.IPv4Unicast))
	require.False(t, cleanup.MatchesFamily(family.IPv6Unicast))
	for _, bad := range []string{
		`{"peer-owner":"0","connection-id":"41","peer-address":"192.0.2.50","families":[]}`,
		`{"peer-owner":"1701","connection-id":"0","peer-address":"192.0.2.50","families":[]}`,
		`{"peer-owner":"1701","connection-id":"41","peer-address":"bad","families":[]}`,
		`{"peer-owner":"1701","connection-id":"41","peer-address":"192.0.2.50","families":["unknown/family"]}`,
		`{"peer-owner":"1701","connection-id":"41","peer-address":"192.0.2.50","families":[],"no-llgr-only":true}`,
	} {
		_, err := DecodeCleanup(rpc.EventEnvelope{Payload: json.RawMessage(bad)})
		require.Error(t, err)
	}
	_, err = EncodeCleanup(1701, 41, "192.0.2.50", nil, true)
	require.Error(t, err)
	require.False(t, (Cleanup{}).MatchesFamily(family.IPv4Unicast))
}
