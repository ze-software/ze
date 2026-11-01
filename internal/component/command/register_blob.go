// Design: docs/architecture/api/commands.md — text and bytes answers
// Overview: blob.go — the Blob answer this renders

package command

// A Blob's text rendering is its content, so `| text` and the default format of
// a command declaring ShapeBlob print the bytes the operator asked for rather
// than a table of the envelope.
func init() {
	MustRegisterTextRenderer(blobEnvelopeKey, renderBlobText)
}
