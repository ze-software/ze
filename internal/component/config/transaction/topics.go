// Design: docs/architecture/config/transaction-protocol.md -- config transaction bus topics

// Package transaction implements the bus-based config transaction protocol.
// It defines topic constants, event payload types, and the transaction
// orchestrator that coordinates verify/apply/rollback across plugins.
package transaction

// TopicPrefix is the shared prefix for all config transaction events.
const TopicPrefix = "config/"

// Per-plugin topics (engine -> plugin).
// The engine publishes to TopicVerifyPrefix+pluginName to send verify
// events to a specific plugin. Same pattern for apply.
const (
	TopicVerifyPrefix = "config/verify/" // + plugin name
	TopicApplyPrefix  = "config/apply/"  // + plugin name
)

// Broadcast topics (engine -> all plugins).
const (
	// TopicVerifyAbort is published when any plugin fails verify.
	// All plugins discard their verify state.
	TopicVerifyAbort = "config/verify/abort"

	// TopicRollback is published when any plugin fails apply.
	// All plugins that applied must undo via their journal.
	TopicRollback = "config/rollback"

	// TopicCommitted is published after all plugins apply successfully.
	// Plugins discard their journals (undo no longer needed).
	TopicCommitted = "config/committed"

	// TopicApplied is published after config file is written (or write attempted).
	// Observers use this for notification. Metadata includes saved status.
	TopicApplied = "config/applied"
)

// Ack topics (plugin -> engine).
// These use a separate "config/ack/" namespace to avoid prefix collisions
// with per-plugin topics under "config/verify/" and "config/apply/".
const (
	TopicAckVerifyOK     = "config/ack/verify/ok"
	TopicAckVerifyFailed = "config/ack/verify/failed"
	TopicAckApplyOK      = "config/ack/apply/ok"
	TopicAckApplyFailed  = "config/ack/apply/failed"
	TopicAckRollbackOK   = "config/ack/rollback/ok"
)

// TopicVerifyFor returns the per-plugin verify topic for the named plugin.
func TopicVerifyFor(plugin string) string {
	return TopicVerifyPrefix + plugin
}

// TopicApplyFor returns the per-plugin apply topic for the named plugin.
func TopicApplyFor(plugin string) string {
	return TopicApplyPrefix + plugin
}

// Failure codes for transaction ack events.
// Plugins include a code in their ack to indicate the severity of failure.
const (
	CodeOK        = "ok"        // Success.
	CodeTimeout   = "timeout"   // Plugin did not respond in time (set by engine).
	CodeTransient = "transient" // Temporary failure, retry may succeed.
	CodeError     = "error"     // Permanent failure, rollback needed.
	CodeBroken    = "broken"    // Plugin state is corrupt, restart needed.
)

// MaxBudgetSeconds is the maximum allowed verify or apply budget.
// Budgets exceeding this are capped to this value.
const MaxBudgetSeconds = 600
