// Design: docs/architecture/config/transaction-protocol.md -- config namespace registration

package transaction

import (
	txevents "github.com/ze-software/ze/internal/component/config/transaction/events"
	coreevents "github.com/ze-software/ze/internal/core/events"
)

func init() {
	_ = coreevents.RegisterNamespace(txevents.Namespace,
		txevents.EventVerify, txevents.EventApply, txevents.EventRollback,
		txevents.EventCommitted, txevents.EventApplied, txevents.EventRolledBack,
		txevents.EventVerifyAbort, txevents.EventVerifyOK, txevents.EventVerifyFailed,
		txevents.EventApplyOK, txevents.EventApplyFailed, txevents.EventRollbackOK,
		txevents.EventOperationDecompose, txevents.EventOperationVerify, txevents.EventOperationApply,
		txevents.EventOperationRollback, txevents.EventOperationCommit,
		txevents.EventOperationDecomposeOK, txevents.EventOperationDecomposeFailed,
		txevents.EventOperationVerifyOK, txevents.EventOperationVerifyFailed,
		txevents.EventOperationApplyOK, txevents.EventOperationApplyFailed,
		txevents.EventOperationRollbackOK, txevents.EventOperationRollbackFailed,
		txevents.EventOperationCommitOK, txevents.EventOperationCommitFailed,
	)
}
