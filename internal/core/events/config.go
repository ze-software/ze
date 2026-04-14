// Design: docs/architecture/config/transaction-protocol.md -- config transaction event types

package events

// Config transaction event types.
// Engine emits per-plugin verify/apply events. Plugins ack with broadcast events.
const (
	EventConfigVerify       = "verify"        // Engine -> plugin: validate candidate (per-plugin variant: "verify-<plugin>")
	EventConfigApply        = "apply"         // Engine -> plugin: apply changes (per-plugin variant: "apply-<plugin>")
	EventConfigRollback     = "rollback"      // Engine -> plugins: undo changes
	EventConfigCommitted    = "committed"     // Engine -> plugins: discard journals
	EventConfigApplied      = "applied"       // Engine -> observers: transaction committed
	EventConfigRolledBack   = "rolled-back"   // Engine -> observers: transaction rolled back
	EventConfigVerifyAbort  = "verify-abort"  // Engine -> plugins: verify phase aborted
	EventConfigVerifyOK     = "verify-ok"     // Plugin -> engine: verification passed
	EventConfigVerifyFailed = "verify-failed" // Plugin -> engine: verification rejected
	EventConfigApplyOK      = "apply-ok"      // Plugin -> engine: apply succeeded
	EventConfigApplyFailed  = "apply-failed"  // Plugin -> engine: apply failed, trigger rollback
	EventConfigRollbackOK   = "rollback-ok"   // Plugin -> engine: rollback complete
)

// ValidConfigEvents is the set of valid config transaction event types.
// Per-plugin variants ("verify-<plugin>", "apply-<plugin>") are registered
// dynamically as plugins start, via RegisterEventType(NamespaceConfig, ...).
var ValidConfigEvents = map[string]bool{
	EventConfigVerify:       true,
	EventConfigApply:        true,
	EventConfigRollback:     true,
	EventConfigCommitted:    true,
	EventConfigApplied:      true,
	EventConfigRolledBack:   true,
	EventConfigVerifyAbort:  true,
	EventConfigVerifyOK:     true,
	EventConfigVerifyFailed: true,
	EventConfigApplyOK:      true,
	EventConfigApplyFailed:  true,
	EventConfigRollbackOK:   true,
}
