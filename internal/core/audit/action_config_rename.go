// Design: docs/architecture/storage-backends.md -- audit successful stored-config renames.

package audit

// ActionDataConfigRename records a complete stored-configuration identity
// change, including its version pointers and saved editor work.
const ActionDataConfigRename = "data-config-rename"
