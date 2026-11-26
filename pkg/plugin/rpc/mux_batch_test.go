// Design: docs/architecture/api/ipc_protocol.md -- pooled MuxConn deliver-batch framing.
// Related: mux.go -- MuxConn.CallBatchRPC under test.
// Related: batch.go -- WriteBatchFrame and ParseBatchEvents.

package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testMethodDeliverBatch = "ze-plugin-callback:deliver-batch"
	testPipeClient         = "clientEnd"
	testPipeServer         = "serverEnd"
)

func pendingCount(m *MuxConn) int {
	n := 0
	m.pending.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}

func closeMux(t *testing.T, m *MuxConn) {
	t.Helper()
	if err := m.Close(); err != nil {
		t.Logf("mux close: %v", err)
	}
}

// TestMuxConn_CallBatchRPC_PooledAndSplit proves MuxConn.CallBatchRPC writes
// exact `#<id> ze-plugin-callback:deliver-batch {"events":[...]}\n` frames via
// pooled WriteBatchFrame, splits oversized batches in FIFO order at
// MaxMessageSize, rejects oversized single events without writing or leaking
// pending entries, stays race-free beside concurrent CallRPC goroutines, and
// cleans up pending entries on context cancellation or write failure.
//
// VALIDATES: AC-3 -- pooled WriteBatchFrame wire format, MaxMessageSize FIFO
// batch splitting, single-event size refusal, concurrency safety with CallRPC,
// and zero leaked pending entries on error paths.
// PREVENTS: frame interleaving, oversized frames rejected by FrameReader,
// dropped FIFO ordering across split chunks, or leaked pending entries.
func TestMuxConn_CallBatchRPC_PooledAndSplit(t *testing.T) {
	t.Parallel()

	t.Run("empty_events_noop", func(t *testing.T) {
		t.Parallel()
		clientEnd, serverEnd := net.Pipe()
		defer closePipe(t, testPipeClient, clientEnd)
		defer closePipe(t, testPipeServer, serverEnd)

		mux := NewMuxConn(NewConn(clientEnd, clientEnd))
		defer closeMux(t, mux)

		resp, err := mux.CallBatchRPC(t.Context(), nil)
		require.NoError(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, 0, pendingCount(mux))
	})

	t.Run("ordinary_batch_exact_wire_frame", func(t *testing.T) {
		t.Parallel()
		clientEnd, serverEnd := net.Pipe()
		defer closePipe(t, testPipeClient, clientEnd)
		defer closePipe(t, testPipeServer, serverEnd)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		rawLineCh := make(chan []byte, 1)
		errCh := make(chan error, 1)
		go func() {
			fr := NewFrameReader(serverEnd)
			line, readErr := fr.Read()
			if readErr != nil {
				errCh <- readErr
				return
			}
			rawLineCh <- line
			id, _, _, parseErr := ParseLine(line)
			if parseErr != nil {
				errCh <- parseErr
				return
			}
			fw := NewFrameWriter(serverEnd)
			if sendErr := fw.Write(AppendOK(nil, id)); sendErr != nil {
				errCh <- sendErr
				return
			}
		}()

		mux := NewMuxConn(NewConn(clientEnd, clientEnd))
		defer closeMux(t, mux)

		ev1 := []byte(`"upd 14 65000 65001 1 8:10.0.0.1 8:10.0.0.2 7:1.1.1.1 0: 8:AAAAAA=="`)
		ev2 := []byte(`{"type":"bgp","bgp":{"message":{"type":"keepalive"}}}`)
		_, err := mux.CallBatchRPC(ctx, [][]byte{ev1, ev2})
		require.NoError(t, err)
		assert.Equal(t, 0, pendingCount(mux))

		var rawLine []byte
		select {
		case readErr := <-errCh:
			t.Fatalf("server read failed: %v", readErr)
		case rawLine = <-rawLineCh:
		case <-ctx.Done():
			t.Fatalf("timed out waiting for raw frame: %v", ctx.Err())
		}

		id, verb, payload, err := ParseLine(rawLine)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), id)
		assert.Equal(t, testMethodDeliverBatch, verb)

		var expected bytes.Buffer
		require.NoError(t, WriteBatchFrame(&expected, id, [][]byte{ev1, ev2}))
		assert.Equal(t, bytes.TrimSuffix(expected.Bytes(), []byte("\n")), rawLine)

		parsed, err := ParseBatchEvents(payload)
		require.NoError(t, err)
		require.Len(t, parsed, 2)
		assert.Equal(t, string(ev1), string(parsed[0]))
		assert.Equal(t, string(ev2), string(parsed[1]))
	})

	t.Run("oversized_batch_splits_in_fifo_order", func(t *testing.T) {
		t.Parallel()
		clientEnd, serverEnd := net.Pipe()
		defer closePipe(t, testPipeClient, clientEnd)
		defer closePipe(t, testPipeServer, serverEnd)

		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		// Build events totaling > MaxMessageSize so CallBatchRPC must split.
		const eventBodySize = 512 * 1024
		padding := bytes.Repeat([]byte("a"), eventBodySize)
		const numEvents = 36 // 36 * ~512 KiB > 18 MiB > 16 MiB MaxMessageSize.
		events := make([][]byte, numEvents)
		for i := range numEvents {
			var b []byte
			b = append(b, `{"seq":`...)
			b = strconv.AppendInt(b, int64(i), 10)
			b = append(b, `,"pad":"`...)
			b = append(b, padding...)
			b = append(b, `"}`...)
			events[i] = b
		}

		var frameCount int
		var maxFrameLen int
		var receivedSeqs []int
		serverErrCh := make(chan error, 1)
		serverDone := make(chan struct{})
		go func() {
			defer close(serverDone)
			fr := NewFrameReader(serverEnd)
			fw := NewFrameWriter(serverEnd)
			for {
				line, readErr := fr.Read()
				if readErr != nil {
					return
				}
				frameCount++
				if len(line) > maxFrameLen {
					maxFrameLen = len(line)
				}
				id, verb, payload, parseErr := ParseLine(line)
				if parseErr != nil {
					serverErrCh <- parseErr
					return
				}
				if verb != testMethodDeliverBatch {
					serverErrCh <- errors.New("unexpected verb: " + verb)
					return
				}
				evts, batchErr := ParseBatchEvents(payload)
				if batchErr != nil {
					serverErrCh <- batchErr
					return
				}
				for _, rawEv := range evts {
					var item struct {
						Seq int `json:"seq"`
					}
					if uErr := json.Unmarshal(rawEv, &item); uErr != nil {
						serverErrCh <- uErr
						return
					}
					receivedSeqs = append(receivedSeqs, item.Seq)
				}
				if sendErr := fw.Write(AppendOK(nil, id)); sendErr != nil {
					serverErrCh <- sendErr
					return
				}
			}
		}()

		mux := NewMuxConn(NewConn(clientEnd, clientEnd))
		_, err := mux.CallBatchRPC(ctx, events)
		require.NoError(t, err)
		assert.Equal(t, 0, pendingCount(mux))

		require.NoError(t, mux.Close())
		<-serverDone

		select {
		case srvErr := <-serverErrCh:
			t.Fatalf("server error: %v", srvErr)
		default:
		}

		assert.Greater(t, frameCount, 1, "oversized batch must split into multiple frames")
		assert.LessOrEqual(t, maxFrameLen, MaxMessageSize, "every split frame must stay <= MaxMessageSize")
		require.Len(t, receivedSeqs, numEvents)
		for i := range numEvents {
			assert.Equal(t, i, receivedSeqs[i], "FIFO order must hold across split chunks")
		}
	})

	t.Run("single_event_exceeding_max_refused_without_writing", func(t *testing.T) {
		t.Parallel()
		clientEnd, serverEnd := net.Pipe()
		defer closePipe(t, testPipeClient, clientEnd)
		defer closePipe(t, testPipeServer, serverEnd)

		// Do not read from serverEnd: net.Pipe is synchronous, so any write
		// attempt would block until timeout. Returning promptly proves nothing
		// was written to the wire.
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		mux := NewMuxConn(NewConn(clientEnd, clientEnd))
		defer closeMux(t, mux)

		tooBig := make([]byte, MaxMessageSize+1)
		for i := range tooBig {
			tooBig[i] = 'x'
		}
		_, err := mux.CallBatchRPC(ctx, [][]byte{[]byte(`"ok"`), tooBig})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrMessageTooLong)
		assert.Equal(t, 0, pendingCount(mux))
	})

	t.Run("concurrent_CallRPC_and_CallBatchRPC_race_free", func(t *testing.T) {
		t.Parallel()
		clientEnd, serverEnd := net.Pipe()
		defer closePipe(t, testPipeClient, clientEnd)
		defer closePipe(t, testPipeServer, serverEnd)

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()

		fakeEngine(ctx, serverEnd, func(req *Request) any {
			if req.Method == testMethodDeliverBatch {
				evts, err := ParseBatchEvents(req.Params)
				if err != nil {
					return map[string]any{"error": err.Error()}
				}
				return map[string]int{"count": len(evts)}
			}
			return map[string]string{"method": req.Method}
		})

		mux := NewMuxConn(NewConn(clientEnd, clientEnd))
		defer closeMux(t, mux)

		const workers = 16
		const rounds = 10
		var wg sync.WaitGroup
		errCh := make(chan error, workers*rounds*2)

		for w := range workers {
			wg.Add(2)
			go func(id int) {
				defer wg.Done()
				method := "ping-" + strconv.Itoa(id)
				for range rounds {
					raw, callErr := mux.CallRPC(ctx, method, map[string]int{"w": id})
					if callErr != nil {
						errCh <- callErr
						return
					}
					var res struct {
						Method string `json:"method"`
					}
					if uErr := json.Unmarshal(raw, &res); uErr != nil {
						errCh <- uErr
						return
					}
					if res.Method != method {
						errCh <- errors.New("method mismatch: " + res.Method)
						return
					}
				}
			}(w)

			go func(id int) {
				defer wg.Done()
				ev := []byte(`"upd 14 65000 65001 ` + strconv.Itoa(id) + ` 8:10.0.0.1 8:10.0.0.2 7:1.1.1.1 0: 8:AAAAAA=="`)
				for range rounds {
					raw, callErr := mux.CallBatchRPC(ctx, [][]byte{ev, ev})
					if callErr != nil {
						errCh <- callErr
						return
					}
					var res struct {
						Count int `json:"count"`
					}
					if uErr := json.Unmarshal(raw, &res); uErr != nil {
						errCh <- uErr
						return
					}
					if res.Count != 2 {
						errCh <- errors.New("unexpected batch count")
						return
					}
				}
			}(w)
		}

		wg.Wait()
		close(errCh)
		for err := range errCh {
			require.NoError(t, err)
		}
		assert.Equal(t, 0, pendingCount(mux))
	})

	t.Run("canceled_context_and_write_error_leave_pending_empty", func(t *testing.T) {
		t.Parallel()
		readClient, readServer := net.Pipe()
		writeClient, writeServer := net.Pipe()
		defer closePipe(t, "readClient", readClient)
		defer closePipe(t, "readServer", readServer)
		defer closePipe(t, "writeClient", writeClient)
		defer closePipe(t, "writeServer", writeServer)

		mux := NewMuxConn(NewConn(readClient, writeClient))
		defer closeMux(t, mux)

		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := mux.CallBatchRPC(canceledCtx, [][]byte{[]byte(`"ev"`)})
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, pendingCount(mux))

		// Close only the write pipe while readLoop stays alive to exercise the
		// write-failure pending cleanup path deterministically.
		require.NoError(t, writeServer.Close())
		writeCtx, writeCancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer writeCancel()

		_, err = mux.CallBatchRPC(writeCtx, [][]byte{[]byte(`"ev"`)})
		require.Error(t, err)
		assert.Equal(t, 0, pendingCount(mux))
	})
}
