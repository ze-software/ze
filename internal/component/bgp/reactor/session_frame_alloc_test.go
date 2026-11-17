// Design: docs/architecture/buffer-architecture.md -- pooled wire reads without per-message wrappers.
// Related: session_read.go -- readFrame transfers the original buffer handle.
package reactor

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestReadFrameUsesPooledStorage measures a warmed complete-frame read and
// return, independently of semantic dispatch and socket setup allocations.
func TestReadFrameUsesPooledStorage(t *testing.T) {
	for _, extended := range []bool{false, true} {
		name := "standard"
		if extended {
			name = "extended"
		}
		t.Run(name, func(t *testing.T) {
			session := &Session{extendedMessage: extended}
			wire := message.PackTo(&message.Keepalive{}, nil)
			input := bytes.NewReader(wire)
			reader := bufio.NewReader(input)
			var readErr error
			reads := 0
			read := func() {
				input.Reset(wire)
				reader.Reset(input)
				frame, err := session.readFrame(nil, reader)
				if err != nil {
					readErr = err
					return
				}
				reads++
				session.returnReadBuffer(frame.buffer)
			}
			read() // Allocate the pool's first block before measuring reuse.
			allocations := testing.AllocsPerRun(100, read)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if reads != 102 {
				t.Fatalf("complete pooled reads=%d, want 102", reads)
			}
			if allocations != 0 {
				t.Fatalf("complete pooled frame read allocated %.1f times, want zero", allocations)
			}
		})
	}
}
