// Design: docs/architecture/config/transaction-protocol.md -- config transaction event types

// Package events defines event constants for config transactions.
// This is a leaf package with no dependencies, safe to import from anywhere.
package events

// Namespace is the event namespace for config transactions.
const Namespace = "config"

// Config transaction event types.
const (
	EventVerify       = "verify"        // Engine -> plugin: validate candidate
	EventApply        = "apply"         // Engine -> plugin: apply changes
	EventRollback     = "rollback"      // Engine -> plugins: undo changes
	EventCommitted    = "committed"     // Engine -> plugins: discard journals
	EventApplied      = "applied"       // Engine -> observers: transaction committed
	EventRolledBack   = "rolled-back"   // Engine -> observers: transaction rolled back
	EventVerifyAbort  = "verify-abort"  // Engine -> plugins: verify phase aborted
	EventVerifyOK     = "verify-ok"     // Plugin -> engine: verification passed
	EventVerifyFailed = "verify-failed" // Plugin -> engine: verification rejected
	EventApplyOK      = "apply-ok"      // Plugin -> engine: apply succeeded
	EventApplyFailed  = "apply-failed"  // Plugin -> engine: apply failed, trigger rollback
	EventRollbackOK   = "rollback-ok"   // Plugin -> engine: rollback complete
)
