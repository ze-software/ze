// Design: docs/architecture/api/ipc_protocol.md — multiplexed plugin RPC
// Related: conn.go — Conn type and persistent reader (readFrame)

package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrMuxConnClosed is returned when CallRPC is called on a closed MuxConn.
var ErrMuxConnClosed = fmt.Errorf("mux conn closed")

// ErrAnswerQueueFull ends an answer whose consumer fell answerQueueDepth
// records behind the peer. readLoop reads for every id on one connection, so it
// abandons the one answer nobody is draining rather than stall the connection
// behind it. The consumer sees this error and a truncated verdict, which is why
// the records it did not get are never lost in silence.
var ErrAnswerQueueFull = errors.New("answer queue full: consumer fell behind, answer abandoned")

// ErrAnswerTruncated ends an answer whose line queue closed with no terminator
// and no stated cause. A consumer that sees it received part of an answer and
// MUST NOT read that part as the whole.
var ErrAnswerTruncated = errors.New("answer ended before its terminator")

// maxConsecutiveBadLines is the threshold of consecutive malformed or orphaned
// lines before readLoop closes the connection. Protects against a malicious
// plugin flooding the engine with junk.
const maxConsecutiveBadLines = 100

// answerQueueDepth is the number of answer lines readLoop holds for one
// CallAnswer whose consumer is behind. It is deep enough that a consumer which
// is merely scheduled late never trips it, and bounded because the memory is
// paid for each answer in flight. A full queue ends that answer with
// ErrAnswerQueueFull; readLoop never waits on a consumer.
const answerQueueDepth = 256

// answerCall is the pending entry of one CallAnswer: the queue readLoop
// delivers the answer's lines into, and the fault that ended it.
//
// readLoop is the only sender on lines and the only closer of it. A consumer
// that abandons the answer removes the pending entry and stops reading; it MUST
// NOT close lines, because readLoop can be mid-send. err is written before the
// close and MUST be read only after that close is observed, which is the
// happens-before edge the two goroutines share.
type answerCall struct {
	lines chan AnswerTail
	err   error
}

// pendingKey is the pending-map key for one in-flight call: the request id in
// the text form readLoop cuts out of a line, so the reader routes on the bytes
// it read rather than on a number it parsed.
func pendingKey(id uint64) string {
	var digits [20]byte // A uint64 is at most 20 decimal digits.
	return string(strconv.AppendUint(digits[:0], id, 10))
}

// MuxConn wraps a *Conn to support concurrent CallRPC calls and inbound
// request dispatching on a single bidirectional connection.
//
// A background reader goroutine reads all incoming lines and routes them:
//   - Responses (verb is "ok" or "error") are routed to waiting CallRPC callers by #<id>.
//   - Requests (verb is a method name) are pushed to the Requests() channel.
//
// MuxConn owns the Conn's reader exclusively -- do not call ReadRequest
// on the underlying Conn after creating a MuxConn.
type MuxConn struct {
	conn *Conn

	// pending maps request ID (string) to the call waiting on it: a
	// chan []byte for CallRPC, which is removed as its one response is
	// routed, and an *answerCall for CallAnswer, which lives until the
	// answer ends. Written by the caller, removed by the background reader
	// or by a caller that gives up.
	pending sync.Map

	// requestCh receives inbound requests from the remote side.
	// The readLoop pushes requests here when the verb is not "ok" or "error".
	requestCh chan *Request

	// done is closed when the background reader exits.
	done chan struct{}

	// readerErr stores the terminal read error for late callers.
	readerErr atomic.Pointer[error]

	// closeOnce ensures Close() only runs once.
	closeOnce sync.Once

	// consecutiveBad counts consecutive malformed or orphaned lines in readLoop.
	// Only accessed by readLoop -- no synchronization needed.
	consecutiveBad uint32
}

// NewMuxConn creates a MuxConn wrapping the given Conn.
// Starts a background reader goroutine that routes responses by #<id> prefix
// and inbound requests to the Requests() channel.
func NewMuxConn(conn *Conn) *MuxConn {
	m := &MuxConn{
		conn:      conn,
		requestCh: make(chan *Request, 16),
		done:      make(chan struct{}),
	}
	go m.readLoop()
	return m
}

// Requests returns a channel of inbound requests from the remote side.
// Requests are lines where the verb is a method name (not "ok" or "error").
// The caller should read from this channel in a dispatch loop.
func (m *MuxConn) Requests() <-chan *Request {
	return m.requestCh
}

// SendResult sends a successful RPC response for an inbound request.
func (m *MuxConn) SendResult(ctx context.Context, id uint64, data any) error {
	return m.conn.SendResult(ctx, id, data)
}

// SendOK sends an empty successful RPC response for an inbound request.
func (m *MuxConn) SendOK(ctx context.Context, id uint64) error {
	return m.conn.SendOK(ctx, id)
}

// SendError sends an error RPC response for an inbound request.
func (m *MuxConn) SendError(ctx context.Context, id uint64, message string) error {
	return m.conn.SendError(ctx, id, message)
}

// AnswerWriter returns the writer one answer sequence for an inbound request is
// written to. See Conn.AnswerWriter for what each Write owes.
func (m *MuxConn) AnswerWriter(ctx context.Context) io.Writer {
	return m.conn.AnswerWriter(ctx)
}

// Close stops the background reader and closes the underlying connection.
// All pending CallRPC callers will unblock with an error.
// Safe to call multiple times.
func (m *MuxConn) Close() error {
	var err error
	m.closeOnce.Do(func() {
		err = m.conn.Close()
	})
	return err
}

// CallRPC sends an RPC request and waits for the matching response.
// Returns the result JSON payload on success, or an *RPCCallError on RPC error.
// Safe for concurrent use by multiple goroutines. Each caller gets its
// own response channel keyed by request ID.
func (m *MuxConn) CallRPC(ctx context.Context, method string, params any) (json.RawMessage, error) {
	// Check if reader is already dead.
	if errPtr := m.readerErr.Load(); errPtr != nil {
		return nil, fmt.Errorf("mux conn read error: %w", *errPtr)
	}

	// Generate request ID.
	id := m.conn.NextID()
	idStr := pendingKey(id)

	// Create buffered response channel (capacity 1 so reader never blocks).
	respCh := make(chan []byte, 1)
	m.pending.Store(idStr, respCh)

	// Marshal params.
	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			m.pending.Delete(idStr)
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		paramsRaw = b
	}

	// Send request line: #<id> <method> [<json>]\n (appended into pool buffer).
	writeErr := m.conn.writeAppended(ctx, func(buf []byte) []byte {
		return AppendRequest(buf, id, method, paramsRaw)
	})
	if writeErr != nil {
		m.pending.Delete(idStr)
		return nil, fmt.Errorf("send request: %w", writeErr)
	}

	// Wait for response, context cancellation, or reader death.
	select {
	case body := <-respCh:
		return interpretResponse(body)
	case <-ctx.Done():
		m.pending.Delete(idStr)
		return nil, ctx.Err()
	case <-m.done:
		m.pending.Delete(idStr)
		return nil, m.closedErr()
	}
}

// closedErr states why the background reader stopped: the terminal read error
// when one was stored, and ErrMuxConnClosed when the connection was closed
// with none.
func (m *MuxConn) closedErr() error {
	if errPtr := m.readerErr.Load(); errPtr != nil {
		return fmt.Errorf("mux conn read error: %w", *errPtr)
	}
	return ErrMuxConnClosed
}

// CallAnswer sends an RPC request and returns the answer the peer writes as a
// head line, zero or more record lines, and a terminator. It is the call for a
// peer that has negotiated the answer shape; CallRPC stays the call for one
// that has not, and its wire is unchanged.
//
// It returns once the head has arrived, so the caller learns the answer's
// status before its first record. The caller MUST then range over
// Answer.Records to the end, and MUST read Answer.Verdict and Answer.Err after
// that range: a range left unfinished detaches from the answer, and the lines
// the peer still writes for it are discarded.
//
// Safe for concurrent use by multiple goroutines.
func (m *MuxConn) CallAnswer(ctx context.Context, method string, params any) (*Answer, error) {
	if errPtr := m.readerErr.Load(); errPtr != nil {
		return nil, fmt.Errorf("mux conn read error: %w", *errPtr)
	}

	id := m.conn.NextID()
	idStr := pendingKey(id)

	// The entry lives until the answer ends rather than until its first line,
	// which is what delivers every record to the one caller waiting on this id.
	call := &answerCall{lines: make(chan AnswerTail, answerQueueDepth)}
	m.pending.Store(idStr, call)

	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			m.pending.Delete(idStr)
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		paramsRaw = b
	}

	writeErr := m.conn.writeAppended(ctx, func(buf []byte) []byte {
		return AppendRequest(buf, id, method, paramsRaw)
	})
	if writeErr != nil {
		m.pending.Delete(idStr)
		return nil, fmt.Errorf("send request: %w", writeErr)
	}

	// The reader closes every queue it holds as it exits, and it can have
	// exited between the check above and the Store. This re-check is what makes
	// the entry either swept by that exit or abandoned here: the reader closes
	// m.done before it sweeps, so an entry the sweep did not see was stored
	// after that close.
	if m.readerStopped() {
		m.pending.Delete(idStr)
		return nil, m.closedErr()
	}

	head, headErr := m.awaitAnswerHead(ctx, idStr, call)
	if headErr != nil {
		return nil, headErr
	}

	answer := &Answer{Status: head.Status, Key: head.Key}
	answer.Records = m.answerRecords(ctx, idStr, call, answer)
	return answer, nil
}

// awaitAnswerHead waits for the answer's first line and checks that it opens
// the answer. It removes the pending entry on every path that returns an error,
// so a call that returns no Answer leaves readLoop nothing to route to.
//
// It waits on the line queue and on the caller's context, and on nothing else:
// the reader closes every queue it still holds before it exits, and a receive
// takes the queued lines before it sees that close. A peer that wrote the whole
// answer and then closed the connection therefore delivers it.
func (m *MuxConn) awaitAnswerHead(ctx context.Context, idStr string, call *answerCall) (AnswerTail, error) {
	select {
	case head, open := <-call.lines:
		if !open {
			return AnswerTail{}, answerEndedErr(call)
		}
		if head.IsTerminator() {
			m.pending.Delete(idStr)
			return AnswerTail{}, fmt.Errorf("answer for id %s opens with its terminator", idStr)
		}
		// The head states what a consumer renders the answer as, so a head that
		// states nothing is refused rather than read as done.
		switch head.Status {
		case StatusDone, StatusError:
			return head, nil
		default:
			m.pending.Delete(idStr)
			return AnswerTail{}, fmt.Errorf("answer head for id %s states status=%q", idStr, truncate(head.Status, 40))
		}
	case <-ctx.Done():
		m.pending.Delete(idStr)
		return AnswerTail{}, ctx.Err()
	}
}

// readerStopped reports whether the background reader has already exited.
func (m *MuxConn) readerStopped() bool {
	select {
	case <-m.done:
		return true
	default:
		return false
	}
}

// answerRecords yields the answer's records in arrival order and ends at its
// terminator. It writes how the answer ended into answer, which Verdict and Err
// report once the range has returned.
//
// The queue is the one thing it waits on, apart from the caller's own context:
// the reader ends every answer it still holds before it exits, and a receive
// takes the queued records before it sees that end, so a peer that wrote
// records and then died delivers them rather than losing them to a race
// between two ready channels.
//
// The sequence is single-use, and stopping it detaches from the answer: the
// pending entry goes, and readLoop discards what the peer still writes for that
// id rather than counting it as junk (AC-16).
func (m *MuxConn) answerRecords(ctx context.Context, idStr string, call *answerCall, answer *Answer) iter.Seq[Record] {
	return func(yield func(Record) bool) {
		// Bounded by the answer: each pass either takes one line the peer wrote
		// and readLoop bounded, or leaves on the terminator, on a closed queue,
		// or on the consumer stopping.
		for {
			select {
			case line, open := <-call.lines:
				if !open {
					answer.err = answerEndedErr(call)
					return
				}
				if line.IsTerminator() {
					terminator := line
					answer.terminator = &terminator
					return
				}
				if !yield(Record{Item: line.Item, Fault: line.Fault}) {
					m.pending.Delete(idStr)
					return
				}
			case <-ctx.Done():
				m.pending.Delete(idStr)
				answer.err = ctx.Err()
				return
			}
		}
	}
}

// answerEndedErr states why a closed line queue ended its answer. readLoop
// stores the cause before it closes the queue, and a close carrying no cause is
// still an answer that stopped short of its terminator.
func answerEndedErr(call *answerCall) error {
	if call.err != nil {
		return call.err
	}
	return ErrAnswerTruncated
}

// readLoop is the background reader goroutine. It reads response lines
// from the connection and routes them to waiting callers by #<id> prefix.
// Runs until the connection is closed or a read error occurs.
//
// Uses conn.readFrame() to consume from the persistent reader's channel,
// ensuring only one goroutine ever reads from the underlying FrameReader.
// Done returns a channel that is closed when the background reader exits.
func (m *MuxConn) Done() <-chan struct{} {
	return m.done
}

func (m *MuxConn) readLoop() {
	defer close(m.requestCh) // Unblock ReadRequest callers.
	defer m.endPendingAnswers()
	defer close(m.done) // Runs first: CallAnswer re-checks it after it registers.

	for {
		data, err := m.conn.readFrame(context.Background())
		if err != nil {
			m.readerErr.Store(&err)
			return
		}

		line := string(data)

		// Extract #<id> prefix using simple string operations (no JSON parsing).
		if !strings.HasPrefix(line, "#") {
			slog.Warn("mux conn: line missing # prefix", "line", truncate(line, 80))
			if m.badLine() {
				return
			}
			continue
		}

		idStr, body, ok := strings.Cut(line[1:], " ")
		if !ok {
			slog.Warn("mux conn: line has no body after ID", "line", truncate(line, 80))
			if m.badLine() {
				return
			}
			continue
		}

		// Determine if this is a response or an inbound request.
		// Responses have verb "ok" or "error"; requests have a method name.
		verb, _, _ := strings.Cut(body, " ")
		isResponse := verb == StatusOK || verb == StatusError

		if isResponse {
			if m.routeResponse(idStr, body, verb) {
				return
			}
			continue
		}

		// Inbound request from the remote side -- parse and dispatch.
		id, parseErr := strconv.ParseUint(idStr, 10, 64)
		if parseErr != nil {
			slog.Warn("mux conn: bad request ID", "id", idStr)
			if m.badLine() {
				return
			}
			continue
		}

		_, payload, _ := strings.Cut(body, " ")
		req := &Request{
			ID:     id,
			Method: verb,
		}
		if payload != "" {
			req.Params = json.RawMessage(payload)
		}

		m.consecutiveBad = 0
		if !m.sendRequest(req) {
			slog.Warn("mux conn: request channel full, dropping inbound request",
				"id", id, "method", verb)
		}
	}
}

// badLine counts one malformed or orphaned line and reports whether readLoop
// must close the connection. The count is consecutive, so a well-formed line
// resets it and only a flood closes the connection.
func (m *MuxConn) badLine() bool {
	m.consecutiveBad++
	if m.consecutiveBad <= maxConsecutiveBadLines {
		return false
	}
	err := fmt.Errorf("mux conn: %d consecutive malformed lines, closing", m.consecutiveBad)
	m.readerErr.Store(&err)
	return true
}

// routeResponse hands one response line to the call waiting on its id and
// reports whether readLoop must close the connection.
//
// The pending entry states which of the two calls is waiting. A CallRPC entry
// answers on its first line and is removed as it is routed, which is the
// unchanged path a peer that has not negotiated the answer shape takes. A
// CallAnswer entry lives until its answer ends.
func (m *MuxConn) routeResponse(idStr, body, verb string) bool {
	val, found := m.pending.Load(idStr)
	if !found {
		_, payload, _ := strings.Cut(body, " ")
		return m.discardOrphanResponse(idStr, payload)
	}

	switch call := val.(type) {
	case chan []byte:
		// Removed before the send: the channel holds one slot, and this is the
		// one send readLoop ever makes for this id, so it never blocks here.
		m.pending.Delete(idStr)
		m.consecutiveBad = 0
		call <- []byte(body)
		return false
	case *answerCall:
		_, payload, _ := strings.Cut(body, " ")
		return m.routeAnswerLine(idStr, call, verb, payload)
	default:
		// A pending value of an unknown type is this package's own defect, and
		// leaving the entry would route every later line to it as well.
		m.pending.Delete(idStr)
		slog.Warn("mux conn: pending entry of unknown type", "id", idStr)
		return false
	}
}

// routeAnswerLine delivers one line of an answer to the CallAnswer waiting for
// it and reports whether readLoop must close the connection.
//
// The answer ends at its terminator, at an error verb, at a line this build
// cannot read, or at a queue its consumer fell behind. Each of those removes
// the pending entry and closes the line queue, and nothing else closes it.
func (m *MuxConn) routeAnswerLine(idStr string, call *answerCall, verb, payload string) bool {
	if verb == StatusError {
		// Not understood is the whole answer: one error line, no terminator.
		m.consecutiveBad = 0
		m.endAnswer(idStr, call, parseRPCError([]byte(payload)))
		return false
	}

	// The payload is copied out of the frame here, so a record the consumer
	// forwards keeps referencing a buffer only that record owns.
	tail, parseErr := ParseAnswerTail([]byte(payload))
	if parseErr != nil {
		slog.Warn("mux conn: unreadable answer line", "id", idStr, "error", parseErr)
		m.endAnswer(idStr, call, fmt.Errorf("answer line for id %s: %w", idStr, parseErr))
		return m.badLine()
	}

	m.consecutiveBad = 0
	select {
	case call.lines <- tail:
	default:
		// The consumer is answerQueueDepth lines behind. readLoop reads for
		// every id on this connection, so it ends this one answer rather than
		// wait: the consumer is told, and no other id stops (R-5, AC-17).
		slog.Warn("mux conn: answer queue full, abandoning answer",
			"id", idStr, "depth", answerQueueDepth)
		m.endAnswer(idStr, call, ErrAnswerQueueFull)
		return false
	}

	if tail.IsTerminator() {
		m.endAnswer(idStr, call, nil)
	}
	return false
}

// endPendingAnswers ends every answer still registered when the reader stops.
// A CallAnswer consumer waits on its line queue and on nothing else, so the
// reader that owns that queue MUST close it before it exits. The records
// already queued are delivered first: a receive takes them before it sees the
// close.
//
// A CallRPC entry needs none of this. Its caller waits on m.done as well, which
// this function's caller closes next.
func (m *MuxConn) endPendingAnswers() {
	cause := m.closedErr()
	m.pending.Range(func(key, val any) bool {
		call, isAnswer := val.(*answerCall)
		if !isAnswer {
			return true
		}
		idStr, isString := key.(string)
		if !isString {
			return true
		}
		m.endAnswer(idStr, call, cause)
		return true
	})
}

// endAnswer removes the answer's pending entry and closes its line queue, after
// storing the fault that ended it. cause is nil when the terminator ended it.
//
// The store MUST come before the close: the close is the edge that publishes it
// to the consumer, which reads it only once the close is observed.
func (m *MuxConn) endAnswer(idStr string, call *answerCall, cause error) {
	m.pending.Delete(idStr)
	call.err = cause
	close(call.lines)
}

// discardOrphanResponse handles a response line whose id has no caller waiting
// and reports whether readLoop must close the connection.
//
// A line that reads as an answer tail is a record of an answer whose consumer
// has already gone: expected debris of a canceled or completed answer, and it
// MUST NOT count toward the flood guard (AC-16, R-2). Every other unmatched
// line still counts, so the guard narrows rather than weakens.
func (m *MuxConn) discardOrphanResponse(idStr, payload string) bool {
	if payload != "" {
		if _, parseErr := ParseAnswerTail([]byte(payload)); parseErr == nil {
			slog.Debug("mux conn: record for an answer nobody is waiting for", "id", idStr)
			return false
		}
	}
	slog.Warn("mux conn: orphaned response", "id", idStr)
	return m.badLine()
}

// sendRequest attempts a non-blocking send to requestCh. Returns false if the
// channel is full (consumer is stalled). This prevents readLoop from blocking
// and starving all pending CallRPC callers.
func (m *MuxConn) sendRequest(req *Request) bool {
	select {
	case m.requestCh <- req:
		return true
	case <-time.After(time.Second):
		return false
	}
}

// interpretResponse parses the body after #<id> (e.g., "ok {...}" or "error {...}")
// and returns the result payload on success or an RPCCallError on error.
func interpretResponse(body []byte) (json.RawMessage, error) {
	s := string(body)
	verb, payload, _ := strings.Cut(s, " ")

	if verb == StatusOK {
		if payload == "" {
			return nil, nil
		}
		return json.RawMessage(payload), nil
	}
	if verb == StatusError {
		return nil, parseRPCError([]byte(payload))
	}
	return nil, fmt.Errorf("unexpected response verb %q", verb)
}
