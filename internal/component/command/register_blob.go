// Design: docs/architecture/api/commands.md — text and bytes answers
// Overview: blob.go — the Blob answer this renders

package command

import "github.com/ze-software/ze/internal/core/events"

// A Blob's text rendering is its content, so `| text` and the default format of
// a command declaring ShapeBlob print the bytes the operator asked for rather
// than a table of the envelope. The command component also owns the "cli" bus
// message namespace for command-path prefixes.
func init() {
	MustRegisterTextRenderer(blobEnvelopeKey, renderBlobText)
	if err := events.RegisterNamespace(events.NamespaceCLI); err != nil {
		panic("BUG: command: register cli namespace: " + err.Error())
	}
}
