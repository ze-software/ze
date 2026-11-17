// Design: docs/architecture/api/process-protocol.md -- local origin receipt domain.
package reactor

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func originOwnerFixture(t *testing.T, name string) (plugin.SelectedOwner, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	owner, err := plugin.NewSelectedOwner(plugin.ProcessSender(name), ctx.Done(), nil)
	require.NoError(t, err)
	owner, err = owner.WithConnection(nilOriginCurrentReader, ctx)
	require.NoError(t, err)
	return owner, cancel
}

func originReceiptFixture(t *testing.T) (*selectedWriterFixture, *originReceiptStore, originProducer, *originSource, *originBuildValue) {
	t.Helper()
	writer := newSelectedWriterFixture(t, false)
	owner, _ := originOwnerFixture(t, selectedWriterProcess)
	producer, err := newProcessOriginProducer(owner)
	require.NoError(t, err)
	source, err := newOriginSource(originSourceInput{kind: originSourceLocalBGP})
	require.NoError(t, err)
	value, err := newOriginBuildValue(originBuildFixture())
	require.NoError(t, err)
	store, err := newOriginReceiptStore(t.Context())
	require.NoError(t, err)
	return writer, store, producer, source, value
}

// The issuer and actual dissemination/sender checks authenticate one immutable
// original contribution. Guessed IDs, peers and same-name successors cannot use it.
func TestOriginReceiptChecksExactScopeAndIncarnation(t *testing.T) {
	f, store, producer, source, value := originReceiptFixture(t)
	id, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	require.NotZero(t, id)
	write, err := store.admit(id, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	require.NoError(t, write.current())
	require.Same(t, value, write.build())
	write.complete()
	require.Error(t, write.current(), "a completed lease cannot write again")
	_, err = store.admit(id+1, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err)
	wrongIdentity, err := newOriginIdentity(value.family, []byte{24, 10, 9, 0}, 0, true)
	require.NoError(t, err)
	_, err = store.admit(id, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), wrongIdentity.key)
	require.Error(t, err)
	other, _ := originOwnerFixture(t, selectedWriterProcess)
	_, err = store.admit(id, other, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err)
	_, err = store.admit(id, producer.owner, &Peer{}, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err)
	otherSettings := *f.peer.Settings()
	otherPeer := NewPeer(&otherSettings)
	require.Equal(t, f.peer.Settings().Address, otherPeer.Settings().Address)
	require.NotEqual(t, f.peer.sourceOwner, otherPeer.sourceOwner)
	_, err = store.admit(id, producer.owner, otherPeer, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err)
	_, err = store.admit(id, producer.owner, f.peer, nil, value.key)
	require.Error(t, err)
	foreign, _ := originOwnerFixture(t, "unattached-origin-provider")
	_, err = store.issue(t.Context(), producer, source, value, f.peer, foreign)
	require.Error(t, err)
	foreignProducer, err := newProcessOriginProducer(foreign)
	require.NoError(t, err)
	_, err = store.issue(t.Context(), foreignProducer, source, value, f.peer, producer.owner)
	require.Error(t, err, "sender grants must be checked at the actual engine domain boundary")
}

// Replacement and source-owned withdrawal remove only that current source;
// independent source instances survive, and no withdrawn history stays indexed.
func TestOriginReceiptReplacementAndRemovalStayBounded(t *testing.T) {
	f, store, producer, source, value := originReceiptFixture(t)
	first, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	siblingSource, err := newOriginSource(originSourceInput{kind: originSourceLocalBGP, instance: 1})
	require.NoError(t, err)
	sibling, err := store.issue(t.Context(), producer, siblingSource, value, f.peer, producer.owner)
	require.NoError(t, err)
	replacement, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	require.NotEqual(t, first, replacement)
	_, err = store.admit(first, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err)
	require.Len(t, store.receipts, 2)
	require.Len(t, store.current, 2)
	require.NoError(t, store.withdraw(t.Context(), producer, source, value.originIdentity, f.peer))
	_, err = store.admit(replacement, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err)
	good, err := store.admit(sibling, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	good.complete()
	require.Len(t, store.current, 1)
	require.NoError(t, store.withdraw(t.Context(), producer, siblingSource, value.originIdentity, f.peer))
	require.Empty(t, store.current)
	require.Empty(t, store.receipts)
	before := store.next
	require.NoError(t, store.withdraw(t.Context(), producer, source, value.originIdentity, f.peer))
	require.Equal(t, before, store.next, "never-issued withdrawal creates no ID or tombstone")
	store.next = math.MaxUint64
	_, err = store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.Error(t, err)
	require.Empty(t, store.current)
}

func nilOriginCurrentReader(context.Context, rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
	return nil, context.Canceled
}

// Source path-ID presence is independent of identifier zero in issuer identity.
func TestOriginReceiptPathZeroAndAbsentRemainIndependent(t *testing.T) {
	f, store, producer, source, value := originReceiptFixture(t)
	withZero, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	input := originBuildFixture()
	input.hasPathID = false
	withoutPath, err := newOriginBuildValue(input)
	require.NoError(t, err)
	without, err := store.issue(t.Context(), producer, source, withoutPath, f.peer, producer.owner)
	require.NoError(t, err)
	require.NotEqual(t, withZero, without)
	require.Len(t, store.current, 2)
	require.NoError(t, store.withdraw(t.Context(), producer, source, value.originIdentity, f.peer))
	write, err := store.admit(without, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	write.complete()
	require.Len(t, store.current, 1)
}
