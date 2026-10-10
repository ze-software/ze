// Design: docs/architecture/testing/interop.md -- a refusal read from the end of docker's stderr
//
// Package tail keeps the last octets a command wrote, so a tool that streams
// a long-running command's output to the operator can still read the one
// line the command wrote just before it failed (Docker's refusal to start a
// container) without holding the whole output.
package tail

// Buffer keeps the last Limit octets written to it. Build it with New. Not
// safe for concurrent use: exec copies one stream into it from one goroutine.
type Buffer struct {
	data  []byte
	limit int
}

// New answers a Buffer that keeps the last limit octets. A limit below one
// keeps nothing.
func New(limit int) *Buffer {
	return &Buffer{limit: max(limit, 0)}
}

// Write keeps p's last octets within the limit and always reports p whole, so
// the writer copying a command's output never stops on it.
func (b *Buffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	if excess := len(b.data) - b.limit; excess > 0 {
		b.data = append(b.data[:0], b.data[excess:]...)
	}
	return len(p), nil
}

// String answers the octets kept.
func (b *Buffer) String() string {
	return string(b.data)
}
