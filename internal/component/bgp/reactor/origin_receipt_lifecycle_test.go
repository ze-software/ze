// Design: docs/architecture/api/process-protocol.md -- origin write revocation.
package reactor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Revocation cancels an admitted writer before joining it. A canceled join
// keeps only its genuine in-flight obligation; completion frees the record.
func TestOriginReceiptRevocationJoinsAdmittedWriter(t *testing.T) {
	f, store, producer, source, value := originReceiptFixture(t)
	id, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	write, err := store.admit(id, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, store.revokeOwner(canceled, producer.owner), context.Canceled)
	require.Error(t, write.current())
	select {
	case <-write.done():
	default:
		t.Fatal("revocation did not cancel the admitted write")
	}
	require.Empty(t, store.current)
	require.Len(t, store.receipts, 1)
	write.complete()
	require.NoError(t, store.revokeOwner(t.Context(), producer.owner))
	require.Empty(t, store.receipts)
}

// Session publication changes invalidate writes, while current origin state
// stays one contribution until explicit producer/Peer removal.
func TestOriginReceiptSessionAndProcessFences(t *testing.T) {
	f, store, producer, source, value := originReceiptFixture(t)
	id, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	write, err := store.admit(id, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	facts := *f.peer.forwardFacts()
	snapshot := *facts.dissemination
	snapshot.session = &Session{}
	facts.dissemination = &snapshot
	f.peer.fwdFacts.Store(&facts)
	require.Error(t, write.current(), "old session publication must not authorize output")
	write.complete()
	require.Len(t, store.current, 1, "a session change does not invent or discard a local source candidate")
	require.NoError(t, store.revokePeer(t.Context(), f.peer))
	require.Empty(t, store.current)
	require.Empty(t, store.receipts)
	stopped, stop := originOwnerFixture(t, selectedWriterProcess)
	processProducer, err := newProcessOriginProducer(stopped)
	require.NoError(t, err)
	stop()
	_, err = store.issue(t.Context(), processProducer, source, value, f.peer, producer.owner)
	require.Error(t, err)
}

// An in-place token reset cannot leave an old guard usable on that publication.
func TestOriginReceiptDestinationTokenRevokesOldWrite(t *testing.T) {
	f, store, producer, source, value := originReceiptFixture(t)
	id, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	write, err := store.admit(id, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	f.peer.session.initialReplay.Add(1)
	require.Error(t, write.current())
	write.complete()
	require.NoError(t, store.close(t.Context()))
	require.Empty(t, store.current)
	require.Empty(t, store.receipts)
	_, err = store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.Error(t, err)
}

// A closed producer lifetime cannot admit a write even before cleanup runs.
// Explicit revocation removes current records and preserves another source.
func TestOriginReceiptProducerStopAndGrantRevocation(t *testing.T) {
	f, store, originalProducer, source, value := originReceiptFixture(t)
	producerOwner, stop := originOwnerFixture(t, selectedWriterProcess)
	producer, err := newProcessOriginProducer(producerOwner)
	require.NoError(t, err)
	id, err := store.issue(t.Context(), producer, source, value, f.peer, originalProducer.owner)
	require.NoError(t, err)
	operator := operatorOriginProducer()
	survivor, err := store.issue(t.Context(), operator, source, value, f.peer, originalProducer.owner)
	require.NoError(t, err)
	stop()
	_, err = store.admit(id, originalProducer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err)
	require.NoError(t, store.revokeOwner(t.Context(), producerOwner))
	kept, err := store.admit(survivor, originalProducer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	kept.complete()
	require.NoError(t, store.revokeProducerPeer(t.Context(), operator, f.peer))
	require.Empty(t, store.current)
	require.Empty(t, store.receipts)
}

func TestOriginIssuerRequiresConstruction(t *testing.T) {
	_, err := newOriginReceiptStore(context.Background())
	require.Error(t, err)
	var zero originReceiptStore
	require.Error(t, zero.close(t.Context()))
	var missing *originReceiptStore
	require.Error(t, missing.close(t.Context()))
}

// One real old writer plus current source state is the entire retained bound;
// replacement churn and never-issued withdrawals create no history-only rows.
func TestOriginIssuerRetainsOnlyCurrentAndInflightRecords(t *testing.T) {
	f, store, producer, source, value := originReceiptFixture(t)
	id, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	old, err := store.admit(id, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	for range 128 {
		_, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
		require.NoError(t, err)
		require.Len(t, store.current, 1)
		require.Len(t, store.receipts, 2)
	}
	require.Error(t, old.current())
	old.complete()
	require.Len(t, store.receipts, 1)
	require.NoError(t, store.withdraw(t.Context(), producer, source, value.originIdentity, f.peer))
	before := store.next
	for i := range 128 {
		identity, err := newOriginIdentity(value.family, []byte{32, 10, 4, 0, byte(i)}, 0, false)
		require.NoError(t, err)
		require.NoError(t, store.withdraw(t.Context(), producer, source, identity, f.peer))
	}
	require.Equal(t, before, store.next)
	require.Empty(t, store.current)
	require.Empty(t, store.receipts)
}

// A copied completion token releases only its own write, once; exhausting that
// record's sequence must leave an already admitted independent writer useful.
func TestOriginWriteCopiedCompletionKeepsOtherWrite(t *testing.T) {
	f, store, producer, source, value := originReceiptFixture(t)
	id, err := store.issue(t.Context(), producer, source, value, f.peer, producer.owner)
	require.NoError(t, err)
	first, err := store.admit(id, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	second, err := store.admit(id, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.NoError(t, err)
	copied := first
	copied.complete()
	first.complete()
	require.Error(t, copied.current())
	require.NoError(t, second.current())
	store.receipts[id].nextWrite = ^uint64(0)
	_, err = store.admit(id, producer.owner, f.peer, f.peer.selectedOwnerSnapshot(), value.key)
	require.Error(t, err)
	require.NoError(t, second.current())
	second.complete()
	require.NoError(t, store.withdraw(t.Context(), producer, source, value.originIdentity, f.peer))
	require.Empty(t, store.receipts)
}
