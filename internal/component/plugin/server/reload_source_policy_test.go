package server

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestReloadRolledBackTransactionCompletesAfterSectionRestore proves the
// server half of RFC 8212 AC-13's ordering: a transaction the orchestrator
// rolls back inside Execute (an operation apply failure, or a refused
// operation commit) sends its completion owners an acknowledged
// config-complete(false) for that transaction ID, and only after every
// section-owned participant was told to restore its running config. A BGP
// import invalidation keeps its session's input paused until that receipt, so
// the first resumed UPDATE meets the restored filter definitions.
//
// VALIDATES: runTxCoordinator completes a rolled-back transaction, after the
// section restore, with Accepted false and the transaction's own ID.
// PREVENTS: input resuming against the abandoned candidate's filters, or a
// paused session waiting for a receipt nobody sends.
func TestReloadRolledBackTransactionCompletesAfterSectionRestore(t *testing.T) {
	for _, failure := range []string{"operation-apply", "operation-commit"} {
		t.Run(failure, func(t *testing.T) {
			stamp := time.Now().UnixNano()
			opRoot := fmt.Sprintf("oproot-source-policy-%s-%d", failure, stamp)
			sectionRoot := fmt.Sprintf("sectionroot-source-policy-%s-%d", failure, stamp)
			owner := "opowner-source-policy"
			op := transaction.ConfigOperation{
				ID: "op-source-policy-1", Root: opRoot, Owner: owner, Type: testOpSetProperty,
				Verb: transaction.VerbModify, Target: transaction.ResourceRef{Kind: testResourceSysctl, Name: "test"},
			}
			oldTree := map[string]any{opRoot: map[string]any{"value": "old"}, sectionRoot: map[string]any{"filter": "old"}}
			newTree := map[string]any{opRoot: map[string]any{"value": "new"}, sectionRoot: map[string]any{"filter": "new"}}
			trace := &orderRecorder{}
			plugins := []pluginDef{
				{
					name: owner, roots: []string{opRoot}, configCompletion: true, trace: trace,
					configOps: []rpc.ConfigOperationDecl{{
						Root: opRoot, Decompose: true, Operations: []rpc.ConfigOperationType{testOpSetProperty},
					}},
				},
				{name: "section-filter-owner", roots: []string{sectionRoot}, trace: trace},
			}
			s := newTestReloadServer(t, &mockReloadReactor{tree: oldTree}, plugins)
			responder := plugins[0].responder
			responder.mu.Lock()
			responder.opDecomposeResp = &rpc.ConfigOperationDecomposeOutput{Status: rpc.StatusOK, Operations: []rpc.ConfigOperation{op}}
			if failure == "operation-apply" {
				responder.opApplyResp = &rpc.ConfigOperationApplyOutput{Status: rpc.StatusError, Error: "apply refused"}
			} else {
				responder.opCommitResp = &rpc.ConfigOperationCommitOutput{Status: rpc.StatusError, Error: "commit refused"}
			}
			responder.mu.Unlock()

			err := s.ReloadConfig(context.Background(), newTree)
			require.Error(t, err, "the rolled-back transaction must fail the reload")

			want := "complete:" + owner + ":false"
			require.Eventually(t, func() bool { return slices.Contains(trace.snapshot(), want) }, 5*time.Second, 10*time.Millisecond,
				"a rolled-back transaction sent its completion owner no config-complete(false); trace %v", trace.snapshot())
			events := trace.snapshot()
			restore := slices.Index(events, "rollback:section-filter-owner")
			require.GreaterOrEqual(t, restore, 0, "section owner never restored: %v", events)
			require.Less(t, restore, slices.Index(events, want), "completion sent before the section restore: %v", events)
			require.Equal(t, 1, countOf(events, want), "completion sent more than once: %v", events)
			require.NotContains(t, events, "complete:section-filter-owner:false", "a participant without completion got a receipt")

			responder.mu.Lock()
			inputs := slices.Clone(responder.completeInputs)
			applied := responder.opApplyInput.TransactionID
			responder.mu.Unlock()
			require.Len(t, inputs, 1)
			require.False(t, inputs[0].Accepted)
			require.NotEmpty(t, inputs[0].TransactionID)
			require.Equal(t, applied, inputs[0].TransactionID, "receipt names another transaction")
			require.NotZero(t, inputs[0].DeadlineMS, "receipt carries no deadline")
		})
	}
}

func countOf(events []string, want string) int {
	n := 0
	for _, event := range events {
		if event == want {
			n++
		}
	}
	return n
}
