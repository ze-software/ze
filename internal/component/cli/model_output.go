// Design: docs/guide/monitoring.md -- bounded interactive display scrollback.
// Detail: docs/architecture/cli/command-completion.md -- retention ownership and projection.
// Related: model.go -- both constructors; model_monitor.go -- event append path.

package cli

import (
	"strings"
	"unicode/utf8"
)

const (
	modelOutputBytesMax = 512 * 1024
	modelOutputOmitted  = "[earlier output omitted]\n"
)

// outputBuffer owns display scrollback, never raw output or pipe-save bytes.
// The model update loop alone mutates it; it is not safe for concurrent use.
// Model copies MUST share its pointer. String returns a detached display
// snapshot, so a later write cannot change text retained by the viewport.
type outputBuffer struct {
	data    []byte
	head    int
	size    int
	omitted bool
}

// WriteString keeps the latest bytes without shifting the whole retained tail.
// The first nonempty write allocates one fixed-size session buffer; a large
// answer copies only its suffix, never an allocation proportional to the answer.
func (b *outputBuffer) WriteString(text string) {
	if text == "" {
		return
	}
	if b.data == nil {
		b.data = make([]byte, modelOutputBytesMax)
	}
	limit := modelOutputBytesMax
	if !b.omitted {
		if len(text) > limit-b.size {
			b.omitted = true
		}
	}
	if b.omitted {
		limit -= len(modelOutputOmitted)
	}
	if len(text) >= limit {
		text = outputTextSuffix(text, limit)
		b.size = copy(b.data, text)
		b.head = 0
		return
	}
	if drop := b.size - (limit - len(text)); drop > 0 {
		b.discard(drop)
	}
	tail := (b.head + b.size) % len(b.data)
	written := copy(b.data[tail:], text)
	copy(b.data, text[written:])
	b.size += len(text)
	if b.omitted {
		// Eviction can land inside a rune. At most the bounded retained tail
		// is examined, including a source consisting only of continuation bytes.
		for b.size > 0 && !utf8.RuneStart(b.data[b.head]) {
			b.discard(1)
		}
	}
}

func (b *outputBuffer) discard(count int) {
	b.head = (b.head + count) % len(b.data)
	b.size -= count
}

// String applies the existing terminal sanitizer before bounding the projected
// text: replacing a control byte can expand it to a three-byte rune. The final
// copy includes the notice, so an evicted prefix is neither hidden nor retained
// by a small suffix string referring to a larger backing allocation.
func (b *outputBuffer) String() string {
	if b.size == 0 {
		if b.omitted {
			return modelOutputOmitted
		}
		return ""
	}
	var snapshot strings.Builder
	snapshot.Grow(b.size)
	first := min(b.size, len(b.data)-b.head)
	snapshot.Write(b.data[b.head : b.head+first])
	snapshot.Write(b.data[:b.size-first])
	shown := sanitizeForDisplay(snapshot.String())
	if !b.omitted {
		if len(shown) <= modelOutputBytesMax {
			return shown
		}
	}
	return modelOutputOmitted + outputTextSuffix(shown, modelOutputBytesMax-len(modelOutputOmitted))
}

// Len reports retained source bytes, including the notice when raw bytes were
// evicted. Sanitization may produce a different, independently bounded length.
func (b *outputBuffer) Len() int {
	if b.omitted {
		return b.size + len(modelOutputOmitted)
	}
	return b.size
}

// Cap exposes the backing-storage bound independently of the String projection.
func (b *outputBuffer) Cap() int { return cap(b.data) }

// Reset empties both the retained text and omission state. The bounded session
// allocation stays reusable; old bytes are cleared rather than left in it.
func (b *outputBuffer) Reset() {
	clear(b.data)
	b.head = 0
	b.size = 0
	b.omitted = false
}

// outputTextSuffix keeps at most limit bytes and starts at a rune boundary.
// The scan is bounded by limit, even when the source is malformed UTF-8.
func outputTextSuffix(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	start := len(text) - limit
	for start < len(text) && !utf8.RuneStart(text[start]) {
		start++
	}
	return text[start:]
}
