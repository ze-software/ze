// Design: docs/architecture/config/transaction-protocol.md -- transaction event payloads
// Related: orchestrator.go -- marshals/unmarshals these types via the gateway
// Related: topics.go -- event type constants under which these payloads are delivered

package transaction

// VerifyEvent is published by the engine to a per-plugin topic.
// Contains the config diffs filtered to roots this plugin owns or watches.
type VerifyEvent struct {
	TransactionID string        `json:"transaction-id"`
	Diffs         []DiffSection `json:"diffs"`
	DeadlineMS    int64         `json:"deadline-ms"` // Unix millis for verify deadline.
}

// ApplyEvent is published by the engine to a per-plugin topic.
// Contains the same diffs as verify, plus the computed apply deadline.
type ApplyEvent struct {
	TransactionID string        `json:"transaction-id"`
	Diffs         []DiffSection `json:"diffs"`
	DeadlineMS    int64         `json:"deadline-ms"` // Unix millis for apply deadline.
}

// ConfigOperationApplyEvent asks one plugin to apply one ordered operation.
type ConfigOperationApplyEvent struct {
	TransactionID string          `json:"transaction-id"`
	Operation     ConfigOperation `json:"operation"`
	DeadlineMS    int64           `json:"deadline-ms,omitempty"`
}

// ConfigOperationDecomposeEvent asks one plugin to decompose a root diff.
type ConfigOperationDecomposeEvent struct {
	TransactionID string      `json:"transaction-id"`
	Root          string      `json:"root"`
	ActiveRoot    string      `json:"active-root,omitempty"`
	CandidateRoot string      `json:"candidate-root,omitempty"`
	Diff          DiffSection `json:"diff"`
	DeadlineMS    int64       `json:"deadline-ms,omitempty"`
}

// ConfigOperationVerifyEvent asks one plugin to verify one operation.
type ConfigOperationVerifyEvent struct {
	TransactionID string          `json:"transaction-id"`
	Operation     ConfigOperation `json:"operation"`
	DeadlineMS    int64           `json:"deadline-ms,omitempty"`
}

// ConfigOperationCommitEvent asks one plugin to finalize operation journals.
type ConfigOperationCommitEvent struct {
	TransactionID string `json:"transaction-id"`
	DeadlineMS    int64  `json:"deadline-ms,omitempty"`
}

// RollbackEvent is published by the engine to all participants.
type RollbackEvent struct {
	TransactionID string `json:"transaction-id"`
	Reason        string `json:"reason"`
}

// AbortEvent is published by the engine when verify fails.
type AbortEvent struct {
	TransactionID string `json:"transaction-id"`
	Reason        string `json:"reason"`
}

// CommittedEvent is published by the engine after all plugins apply successfully.
// Plugins discard their journals upon receiving this.
type CommittedEvent struct {
	TransactionID string `json:"transaction-id"`
}

// AppliedEvent is published by the engine after the config file write attempt.
type AppliedEvent struct {
	TransactionID string `json:"transaction-id"`
	Saved         bool   `json:"saved"` // False if config file write failed.
}

// VerifyAck is published by a plugin to acknowledge verify.
type VerifyAck struct {
	TransactionID    string `json:"transaction-id"`
	Plugin           string `json:"plugin"`
	Status           string `json:"status"` // CodeOK or CodeError.
	Error            string `json:"error,omitempty"`
	ApplyBudgetSecs  int    `json:"apply-budget-secs,omitempty"`  // Estimated apply time.
	VerifyBudgetSecs int    `json:"verify-budget-secs,omitempty"` // Updated verify estimate.
}

// ApplyAck is published by a plugin to acknowledge apply.
type ApplyAck struct {
	TransactionID    string `json:"transaction-id"`
	Plugin           string `json:"plugin"`
	Status           string `json:"status"` // CodeOK or CodeError.
	Error            string `json:"error,omitempty"`
	VerifyBudgetSecs int    `json:"verify-budget-secs,omitempty"` // Updated for next tx.
	ApplyBudgetSecs  int    `json:"apply-budget-secs,omitempty"`  // Updated for next tx.
}

// ConfigOperationApplyAck acknowledges one operation apply callback.
type ConfigOperationApplyAck struct {
	TransactionID string                     `json:"transaction-id"`
	Plugin        string                     `json:"plugin"`
	OperationID   string                     `json:"operation-id"`
	Status        string                     `json:"status"`
	Error         string                     `json:"error,omitempty"`
	Readiness     []ConfigOperationReadiness `json:"readiness,omitempty"`
}

// ConfigOperationDecomposeAck acknowledges one operation decompose callback.
type ConfigOperationDecomposeAck struct {
	TransactionID string            `json:"transaction-id"`
	Plugin        string            `json:"plugin"`
	Root          string            `json:"root"`
	Status        string            `json:"status"`
	Error         string            `json:"error,omitempty"`
	Operations    []ConfigOperation `json:"operations,omitempty"`
}

// ConfigOperationVerifyAck acknowledges one operation verify callback.
type ConfigOperationVerifyAck struct {
	TransactionID string `json:"transaction-id"`
	Plugin        string `json:"plugin"`
	OperationID   string `json:"operation-id"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
}

// ConfigOperationCommitAck acknowledges one operation commit callback.
type ConfigOperationCommitAck struct {
	TransactionID string `json:"transaction-id"`
	Plugin        string `json:"plugin"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
}

// ConfigOperationRollbackEvent asks one plugin to roll back ordered operations.
type ConfigOperationRollbackEvent struct {
	TransactionID string            `json:"transaction-id"`
	Operations    []ConfigOperation `json:"operations,omitempty"`
	DeadlineMS    int64             `json:"deadline-ms,omitempty"`
}

// ConfigOperationRollbackAck acknowledges one operation rollback callback.
type ConfigOperationRollbackAck struct {
	TransactionID string `json:"transaction-id"`
	Plugin        string `json:"plugin"`
	OperationID   string `json:"operation-id,omitempty"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
}

// RollbackAck is published by a plugin to acknowledge rollback.
type RollbackAck struct {
	TransactionID string `json:"transaction-id"`
	Plugin        string `json:"plugin"`
	Code          string `json:"code"`            // CodeOK or CodeBroken.
	Error         string `json:"error,omitempty"` // Details if broken.
}

// DiffSection describes what changed in a single config root.
// Mirrors rpc.ConfigDiffSection but lives in the transaction package
// to avoid an import cycle between transaction and plugin/rpc.
type DiffSection struct {
	Root    string `json:"root"`
	Added   string `json:"added,omitempty"`
	Removed string `json:"removed,omitempty"`
	Changed string `json:"changed,omitempty"`
}
