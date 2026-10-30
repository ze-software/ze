package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBGPTransactionRejectionUndoesBeforeReceipt proves the BGP plugin's
// rejection receipt undoes every operation of the transaction the executor's
// rollback did not reach before it delivers the source-policy receipt, last
// operation first, and leaks no journal. Method: two journals of tx-a hold
// recorded undos and a third belongs to tx-b; the receipt for tx-a is
// rejected, and a trace records the undos and the source-policy receipt.
// VALIDATES: undo of the later operation, then the earlier, then the receipt;
// only tx-b's journal remains. On acceptance the receipt runs and no undo.
// PREVENTS: an invalidation not yet undone being aborted without its settings
// restore while the candidate's default-reject policy stays published, so
// the owners restore routes the published policy rejects (RFC 8212
// Section 3), and the journal kept for ever.
func TestBGPTransactionRejectionUndoesBeforeReceipt(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		var trace []string
		journals := map[string]*bgpOperationJournal{}
		record := func(txID, opID string) {
			j := newBGPOperationJournal(txID, 0)
			require.NoError(t, j.Record(func() error { return nil }, func() error {
				trace = append(trace, "undo "+txID+"/"+opID)
				return nil
			}))
			journals[operationJournalKey(txID, opID)] = j
		}
		record("tx-a", "modify-edge")
		record("tx-a", "reconcile")
		record("tx-b", "modify-other")
		complete := func(_ context.Context, txID string, ok bool) error {
			require.Equal(t, accepted, ok)
			trace = append(trace, "receipt "+txID)
			return nil
		}
		require.NoError(t, completeBGPTransaction(t.Context(), "tx-a", accepted, journals, complete))
		want := []string{"undo tx-a/reconcile", "undo tx-a/modify-edge", "receipt tx-a"}
		if accepted {
			want = []string{"receipt tx-a"}
		}
		require.Equal(t, want, trace, "accepted=%t", accepted)
		require.Len(t, journals, 1, "tx-a journals leaked")
		require.Contains(t, journals, operationJournalKey("tx-b", "modify-other"))
	}
}
