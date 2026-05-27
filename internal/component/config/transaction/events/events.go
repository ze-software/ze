// Design: docs/architecture/config/transaction-protocol.md -- config transaction event types

// Package events defines event constants for config transactions.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for config transactions.
const Namespace = "config"

// Config transaction event types.
const (
	EventVerify                   = "verify"                     // Engine -> plugin: validate candidate
	EventApply                    = "apply"                      // Engine -> plugin: apply changes
	EventRollback                 = "rollback"                   // Engine -> plugins: undo changes
	EventCommitted                = "committed"                  // Engine -> plugins: discard journals
	EventApplied                  = "applied"                    // Engine -> observers: transaction committed
	EventRolledBack               = "rolled-back"                // Engine -> observers: transaction rolled back
	EventVerifyAbort              = "verify-abort"               // Engine -> plugins: verify phase aborted
	EventOperationDecompose       = "operation-decompose"        // Engine -> plugin: decompose root diff
	EventOperationVerify          = "operation-verify"           // Engine -> plugin: verify one operation
	EventOperationApply           = "operation-apply"            // Engine -> plugin: apply one operation
	EventOperationRollback        = "operation-rollback"         // Engine -> plugin: roll back operations
	EventOperationCommit          = "operation-commit"           // Engine -> plugin: finalize operation journal
	EventVerifyOK                 = "verify-ok"                  // Plugin -> engine: verification passed
	EventVerifyFailed             = "verify-failed"              // Plugin -> engine: verification rejected
	EventApplyOK                  = "apply-ok"                   // Plugin -> engine: apply succeeded
	EventApplyFailed              = "apply-failed"               // Plugin -> engine: apply failed, trigger rollback
	EventRollbackOK               = "rollback-ok"                // Plugin -> engine: rollback complete
	EventOperationDecomposeOK     = "operation-decompose-ok"     // Plugin -> engine: decomposition passed
	EventOperationDecomposeFailed = "operation-decompose-failed" // Plugin -> engine: decomposition failed
	EventOperationVerifyOK        = "operation-verify-ok"        // Plugin -> engine: operation verify passed
	EventOperationVerifyFailed    = "operation-verify-failed"    // Plugin -> engine: operation verify failed
	EventOperationApplyOK         = "operation-apply-ok"         // Plugin -> engine: operation apply succeeded
	EventOperationApplyFailed     = "operation-apply-failed"     // Plugin -> engine: operation apply failed
	EventOperationRollbackOK      = "operation-rollback-ok"      // Plugin -> engine: operation rollback succeeded
	EventOperationRollbackFailed  = "operation-rollback-failed"  // Plugin -> engine: operation rollback failed
	EventOperationCommitOK        = "operation-commit-ok"        // Plugin -> engine: operation commit succeeded
	EventOperationCommitFailed    = "operation-commit-failed"    // Plugin -> engine: operation commit failed
)
