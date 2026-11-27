// Design: docs/architecture/pki/pki-store.md -- audit PKI private-key bundle exports.

package audit

// ActionPKIPrivateKeyExport records an authorized, denied, or failed attempt
// to export a stored device certificate bundle containing its private key.
const ActionPKIPrivateKeyExport = "pki-private-key-export"
