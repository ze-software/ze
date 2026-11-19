// Design: docs/architecture/api/process-protocol.md -- scalar stored replay scope.
package rpc

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// A scalar relay keeps maximum-width identities and rejects an attempted bound
// shape before it can become an intentional unbound operator request.
func TestRelayStoredRouteScopeKeepsLosslessIdentity(t *testing.T) {
	input := RelayStoredRouteInput{Destination: "192.0.2.7", DestinationOwner: ^uint64(0), DestinationSession: ^uint64(0) - 1, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"destination-owner":"18446744073709551615"`)
	require.Contains(t, string(raw), `"destination-session":"18446744073709551614"`)
	var decoded RelayStoredRouteInput
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.NoError(t, decoded.Validate())
	require.Equal(t, input.DestinationOwner, decoded.DestinationOwner)
	require.Equal(t, input.DestinationSession, decoded.DestinationSession)
	for _, raw := range []string{
		`{"destination":"192.0.2.7","destination-owner":null}`,
		`{"destination":"192.0.2.7","destination-owner":"0","destination-session":"0","deadline-ms":0}`,
		`{"destination":"192.0.2.7","destination-owner":18446744073709551615,"destination-session":"17","deadline-ms":1}`,
		`{"destination":"192.0.2.7","destination-owner":"17","destination-session":null,"deadline-ms":1}`,
		`{"destination":"192.0.2.7","destination-owner":"17","destination-session":"19","deadline-ms":null}`,
		`{"destination":"192.0.2.7","destination-session":"19","deadline-ms":1}`,
		`{"destination":"192.0.2.7","destination-owners":["17"]}`,
	} {
		var candidate RelayStoredRouteInput
		err := json.Unmarshal([]byte(raw), &candidate)
		if err == nil {
			err = candidate.Validate()
		}
		require.Error(t, err, raw)
	}
	var unbound RelayStoredRouteInput
	require.NoError(t, json.Unmarshal([]byte(`{"destination":"192.0.2.7","routes":[]}`), &unbound))
	require.False(t, unbound.Bound())
	require.NoError(t, unbound.Validate())
	expired := input
	expired.DeadlineMS = time.Now().Add(-time.Second).UnixMilli()
	require.Error(t, expired.Validate())
	excessive := input
	excessive.DeadlineMS = time.Now().Add(2 * CancellableRequestTimeout).UnixMilli()
	require.Error(t, excessive.Validate())
}
