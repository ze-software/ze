// Design: docs/architecture/api/commands.md -- bounded shared stream event pipes.

package command

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunStreamPipesEventLimit accepts the exact bound and refuses the adjacent
// byte, including when an oversized event arrives across separate writes.
func TestRunStreamPipesEventLimit(t *testing.T) {
	for _, split := range []bool{false, true} {
		for _, extra := range []int{0, 1} {
			t.Run(fmtStreamPipeBoundary(split, extra), func(t *testing.T) {
				var output bytes.Buffer
				calls := 0
				length := 0
				err := RunStreamPipes(&output, func(event string) string {
					calls++
					length = len(event)
					return "retained"
				}, func(writer io.Writer) error {
					input := []byte(strings.Repeat("x", streamPipeEventBytesMax+extra))
					if split {
						written, err := writer.Write(input[:streamPipeEventBytesMax])
						require.NoError(t, err)
						assert.Equal(t, streamPipeEventBytesMax, written)
						input = input[streamPipeEventBytesMax:]
					}
					_, err := writer.Write(input)
					return err
				})
				if extra == 0 {
					require.NoError(t, err)
					assert.Equal(t, 1, calls)
					assert.Equal(t, streamPipeEventBytesMax, length)
					assert.Equal(t, "retained\n", output.String())
					return
				}
				require.ErrorContains(t, err, "4 MiB")
				assert.Zero(t, calls, "an oversized event must never be formatted")
				assert.Empty(t, output.String(), "a partial event must never be emitted")
			})
		}
	}
}

func fmtStreamPipeBoundary(split bool, extra int) string {
	name := "single/"
	if split {
		name = "split/"
	}
	if extra == 0 {
		return name + "exact"
	}
	return name + "oversized"
}

// TestRunStreamPipesProducerFailure preserves the producer's failure without
// publishing its final incomplete event as successfully formatted output.
func TestRunStreamPipesProducerFailure(t *testing.T) {
	var output bytes.Buffer
	want := errors.New("producer failed")
	calls := 0
	err := RunStreamPipes(&output, func(string) string {
		calls++
		return "incorrect flush"
	}, func(writer io.Writer) error {
		_, err := io.WriteString(writer, "incomplete")
		require.NoError(t, err)
		return want
	})
	assert.ErrorIs(t, err, want)
	assert.Zero(t, calls)
	assert.Empty(t, output.String())
}

// TestRunStreamPipesWriterFailure stops before another event and retains the
// transport failure even when a producer incorrectly returns success.
func TestRunStreamPipesWriterFailure(t *testing.T) {
	want := errors.New("transport failed")
	calls := 0
	err := RunStreamPipes(streamPipeFailureWriter{failure: want}, func(event string) string {
		calls++
		return event
	}, func(writer io.Writer) error {
		n, err := io.WriteString(writer, "first\nsecond\n")
		assert.Equal(t, len("first\n"), n)
		assert.ErrorIs(t, err, want)
		n, err = io.WriteString(writer, "third\n")
		assert.Zero(t, n)
		assert.ErrorIs(t, err, want)
		return nil
	})
	assert.ErrorIs(t, err, want)
	assert.Equal(t, 1, calls)
}

type streamPipeFailureWriter struct{ failure error }

func (w streamPipeFailureWriter) Write([]byte) (int, error) {
	return 0, w.failure
}

// TestRunStreamPipesShortWrite returns io.ErrShortWrite instead of treating a
// truncated transport write as a successfully delivered event.
func TestRunStreamPipesShortWrite(t *testing.T) {
	err := RunStreamPipes(streamPipeShortWriter{}, func(event string) string { return event }, func(writer io.Writer) error {
		_, err := io.WriteString(writer, "first\n")
		return err
	})
	assert.ErrorIs(t, err, io.ErrShortWrite)
}

type streamPipeShortWriter struct{}

func (streamPipeShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

// TestRunStreamPipesLateRefusal never publishes a formatter refusal as data or
// runs a later event after the first answer's shape made the chain invalid.
func TestRunStreamPipesLateRefusal(t *testing.T) {
	var output bytes.Buffer
	calls := 0
	err := RunStreamPipes(&output, func(string) string {
		calls++
		return pipeError("count needs rows")
	}, func(writer io.Writer) error {
		_, err := io.WriteString(writer, "first\nsecond\n")
		return err
	})
	assert.ErrorContains(t, err, "count needs rows")
	assert.Equal(t, 1, calls)
	assert.Empty(t, output.String())
}
