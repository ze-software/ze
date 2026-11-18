// Design: docs/architecture/api/ipc_protocol.md -- record-answer completion.
package rpc

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A streamed answer's failed downstream head write stops its original walk
// before exhaustion. Its completion snapshot must not be read at that point.
func TestRecordAnswerFinalSkipsFailedWrite(t *testing.T) {
	stopped := errors.New("downstream transport stopped")
	produced := 0
	finalCalls := 0
	row := RawRow(`{"ready":true}`)
	err := WriteRecordAnswerFinal(answerFinalFailedWriter{stopped}, 1, AnswerTail{}, func(yield func(RowRecord) bool) {
		for range AnswerBufferThreshold + 20 {
			produced++
			if !yield(RowRecord{Item: &row}) {
				return
			}
		}
	}, func() AnswerTail {
		finalCalls++
		return AnswerTail{Message: "unearned completion"}
	})
	require.ErrorIs(t, err, stopped)
	assert.Equal(t, AnswerBufferThreshold+1, produced, "failed head write did not stop the producer")
	assert.Zero(t, finalCalls, "unfinished producer's tail callback was read")
}

// Header validation precedes both the producer and its completion callback;
// refusing a reserved envelope cannot run either side effect.
func TestRecordAnswerFinalRefusesBeforeProducer(t *testing.T) {
	produced := false
	completed := false
	err := WriteRecordAnswerFinal(io.Discard, 1, AnswerTail{Key: AnswerErrorsKey}, func(func(RowRecord) bool) {
		produced = true
	}, func() AnswerTail {
		completed = true
		return AnswerTail{}
	})
	require.ErrorIs(t, err, ErrReservedEnvelopeKey)
	assert.False(t, produced)
	assert.False(t, completed)
}

// This writer fails before accepting bytes, making its error independent of
// framing or the reader's scheduling. It owns no resource or goroutine.
type answerFinalFailedWriter struct{ failure error }

func (writer answerFinalFailedWriter) Write([]byte) (int, error) { return 0, writer.failure }
