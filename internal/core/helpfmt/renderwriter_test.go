package helpfmt

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errBoom = errors.New("boom")

// failWriter fails on the (after+1)-th Write. after=0 fails on the first write.
type failWriter struct {
	after  int
	writes int
}

func (f *failWriter) Write(p []byte) (int, error) {
	f.writes++
	if f.writes > f.after {
		return 0, errBoom
	}
	return len(p), nil
}

// TestRenderWriterHappyPath verifies Str/Line produce the expected bytes and
// report no error over a healthy writer.
func TestRenderWriterHappyPath(t *testing.T) {
	var buf bytes.Buffer
	rw := NewRenderWriter(&buf)
	rw.Line("hello")
	rw.Str("world")
	rw.Line("")

	assert.Equal(t, "hello\nworld\n", buf.String())
	assert.NoError(t, rw.Err())
	assert.Equal(t, 0, rw.ExitCode())
}

// TestRenderWriterCapturesError verifies the first write error is recorded and
// surfaces as a non-zero ExitCode -- the core AC-10 contract.
func TestRenderWriterCapturesError(t *testing.T) {
	rw := NewRenderWriter(&failWriter{after: 0})
	rw.Line("this write fails")

	require.Error(t, rw.Err())
	assert.ErrorIs(t, rw.Err(), errBoom)
	assert.Equal(t, 1, rw.ExitCode())
}

// TestRenderWriterShortCircuits verifies that once an error is recorded no
// further syscalls hit the underlying writer (a print loop over a broken pipe
// stops writing).
func TestRenderWriterShortCircuits(t *testing.T) {
	fw := &failWriter{after: 0}
	rw := NewRenderWriter(fw)
	rw.Str("a") // fails here (writes==1)
	rw.Str("b") // must not reach the writer
	rw.Line("c")
	rw.Str("d")

	assert.Equal(t, 1, fw.writes, "underlying writer must be hit exactly once")
	assert.ErrorIs(t, rw.Err(), errBoom)
}

// TestRenderWriterErrorMidStream verifies an error on a later write is captured
// and the first error is preserved (not overwritten by a later one).
func TestRenderWriterErrorMidStream(t *testing.T) {
	// Fail on the 3rd write.
	rw := NewRenderWriter(&failWriter{after: 2})
	rw.Str("one")   // write 1 ok
	rw.Str("two")   // write 2 ok
	rw.Str("three") // write 3 fails
	assert.ErrorIs(t, rw.Err(), errBoom)
	assert.Equal(t, 1, rw.ExitCode())
}

// TestRenderWriterWriteInterfaces verifies fmt.Fprintln (via Write) and
// io.WriteString (via WriteString) both funnel through the writer and capture
// errors, since converted call sites rely on both paths.
func TestRenderWriterWriteInterfaces(t *testing.T) {
	var buf bytes.Buffer
	rw := NewRenderWriter(&buf)
	_, err := fmt.Fprintln(rw, "via-fprintln")
	require.NoError(t, err)
	_, err = io.WriteString(rw, "via-writestring") //nolint:gocritic // exercising the io.WriteString -> StringWriter dispatch on purpose
	require.NoError(t, err)
	assert.Equal(t, "via-fprintln\nvia-writestring", buf.String())

	// Error capture through the io.Writer surface (fmt.Fprintln).
	rwFail := NewRenderWriter(&failWriter{after: 0})
	_, err = fmt.Fprintln(rwFail, "x")
	assert.Error(t, err)
	assert.ErrorIs(t, rwFail.Err(), errBoom)

	// Error capture through the io.StringWriter surface (io.WriteString).
	rwFail2 := NewRenderWriter(&failWriter{after: 0})
	_, err = io.WriteString(rwFail2, "x") //nolint:gocritic // exercising the io.WriteString -> StringWriter dispatch on purpose
	assert.Error(t, err)
	assert.ErrorIs(t, rwFail2.Err(), errBoom)
}
