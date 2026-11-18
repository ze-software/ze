// Design: docs/architecture/storage-backends.md -- audit successful indexed configuration rollback.

package audit

// ActionDataConfigRollback records a stored revision published through the
// daemon's store owner or active-source reload transaction.
const ActionDataConfigRollback = "data-config-rollback"
