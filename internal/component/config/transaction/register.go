// Design: docs/architecture/config/transaction-protocol.md -- config namespace registration

package transaction

import (
	txevents "codeberg.org/thomas-mangin/ze/internal/component/config/transaction/events"
	coreevents "codeberg.org/thomas-mangin/ze/internal/core/events"
)

func init() {
	_ = coreevents.RegisterNamespace(txevents.Namespace,
		txevents.EventVerify, txevents.EventApply, txevents.EventRollback,
		txevents.EventCommitted, txevents.EventApplied, txevents.EventRolledBack,
		txevents.EventVerifyAbort, txevents.EventVerifyOK, txevents.EventVerifyFailed,
		txevents.EventApplyOK, txevents.EventApplyFailed, txevents.EventRollbackOK,
	)
}
