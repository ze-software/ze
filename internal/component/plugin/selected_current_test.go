// Design: docs/architecture/api/process-protocol.md -- captured selecting reader.
package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedOwnerCapturedReader keeps the immutable connection callback and
// comparable owner identity without a process-name lookup or mutable rebinding.
func TestSelectedOwnerCapturedReader(t *testing.T) {
	done := make(chan struct{})
	owner, err := NewSelectedOwner(ProcessSender("reader-owner"), done, nil)
	require.NoError(t, err)
	input := rpc.CurrentSelectionInput{Destination: "192.0.2.7", DestinationSession: 11,
		Keys: []rpc.CurrentSelectionKey{{Family: "ipv4/unicast", KeyNLRI: []byte{0}, Revision: 13}}}
	_, err = owner.CurrentSelection(t.Context(), input)
	require.ErrorContains(t, err, "unbound")
	calls := 0
	owner, err = owner.WithCurrentSelection(func(_ context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		calls++
		return &rpc.CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession,
			Cut: 13, Results: []rpc.CurrentSelectionResult{{Family: input.Keys[0].Family, KeyNLRI: input.Keys[0].KeyNLRI,
				Revision: input.Keys[0].Revision, Presence: rpc.CurrentSelectionPresent, CurrentRevision: 13}}}, nil
	})
	require.NoError(t, err)
	copy := owner
	require.True(t, owner.SameIncarnation(copy))
	_, err = owner.WithCurrentSelection(func(context.Context, rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) { return nil, nil })
	require.ErrorContains(t, err, "already bound")
	_, err = owner.CurrentSelection(t.Context(), input)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	close(done)
	_, err = copy.CurrentSelection(t.Context(), input)
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

// TestSelectedOwnerCapturedReaderRejectsLateReply ends its original lifetime
// inside a valid callback; a complete late answer cannot restore authority.
func TestSelectedOwnerCapturedReaderRejectsLateReply(t *testing.T) {
	done := make(chan struct{})
	owner, err := NewSelectedOwner(ProcessSender("late-reader"), done, nil)
	require.NoError(t, err)
	owner, err = owner.WithCurrentSelection(func(_ context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		close(done)
		return &rpc.CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession,
			Cut: 1, Results: []rpc.CurrentSelectionResult{{Family: "ipv4/unicast", KeyNLRI: []byte{0}, Revision: 1,
				Presence: rpc.CurrentSelectionPresent, CurrentRevision: 1}}}, nil
	})
	require.NoError(t, err)
	_, err = owner.CurrentSelection(t.Context(), rpc.CurrentSelectionInput{Destination: "192.0.2.7", DestinationSession: 11,
		Keys: []rpc.CurrentSelectionKey{{Family: "ipv4/unicast", KeyNLRI: []byte{0}, Revision: 1}}})
	require.ErrorContains(t, err, "stopped")
}
