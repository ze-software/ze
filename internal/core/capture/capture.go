// Design: plan/spec-improve-3-event-replay.md -- protocol event capture format (v1)
// Related: writer.go -- the bounded encoder that writes this format
// Related: reader.go -- the decoder the replay harness reads it back with

// Package capture defines the on-disk format of a Ze protocol event capture: a
// JSONL stream whose first line is a version header and whose every later line
// is one event.
//
// The format exists so a session that misbehaves on an operator's box can be fed
// back through the same code on a developer's desk. It therefore records RAW
// WIRE BYTES rather than decoded structures: bytes replay through the real
// decoder, and they survive an internal refactor that would invalidate any
// decoded form.
//
// # What a capture file contains
//
// Every inbound BGP message of a captured session, complete, including the
// 19-byte header, exactly as the peer sent it. That is peer routing data: the
// prefixes it announced, its AS paths, communities, next-hops, its router-id and
// its capabilities. It also contains the config operations the reactor applied
// while the capture ran, with their transaction ids.
//
// No captured MESSAGE can carry a local secret: a TCP-MD5 key authenticates the
// TCP segment and never travels in the BGP payload, so it is not in the bytes
// the peer sent. Every config PAYLOAD passes through RedactPayload, which
// replaces the value of any leaf whose NAME matches redact.isSecretConfigKey and
// any bcrypt-shaped string. That predicate is a name heuristic, so a new
// secret-bearing leaf must spell one of the names it knows. Add the leaf to
// redact.isSecretConfigKey when you add the leaf, and treat a capture file as
// operator data either way.
//
// Treat a capture file as operator data: it is written where the operator
// configured, nothing uploads it, and it should be handled like a routing-table
// dump when shipped in a bug report.
//
// This package is a leaf: standard library plus internal/core/redact only, so
// both the reactor and the replay tool can import it.
package capture

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/core/redact"
)

// Format is the literal value of a header's "format" field. A file whose first
// line does not carry it is not a Ze capture.
const Format = "ze-capture"

// Version is the schema version this package writes. A reader refuses any other
// value rather than guessing at the difference (spec R-2).
const Version = 1

// Event type values, the "type" field of every event line.
const (
	EventMessage = "message"
	EventConfig  = "config"
	EventSession = "session"
)

// DirectionReceived marks an inbound message. v1 captures inbound only; the
// field exists so a later version can add the sent direction without a schema
// break.
const DirectionReceived = "recv"

// Config operation names for a type=config event. They mirror the reactor's own
// config-operation dispatch so a replayed stream names the same operations the
// daemon applied.
const (
	OpReconcile  = "reconcile"
	OpAddPeer    = "add-peer"
	OpModifyPeer = "modify-peer"
	OpRemovePeer = "remove-peer"
	OpVerify     = "verify"
	OpCommit     = "commit"
	OpRollback   = "rollback"
)

// Session event names for a type=session event.
const (
	SessionCaptureStart = "capture-start"
	SessionCaptureStop  = "capture-stop"
	SessionConnect      = "connect"
	SessionDisconnect   = "disconnect"
	SessionDrops        = "drops"
)

// TimeFormat is the timestamp layout of every "ts" and "started" field.
const TimeFormat = time.RFC3339Nano

// Errors a reader returns. Each is wrapped in a message naming the offending
// line, so a caller can both branch on the cause and show the operator where the
// file went wrong.
var (
	// ErrNoHeader means the file has no first line at all.
	ErrNoHeader = errors.New("capture has no header line")
	// ErrBadHeader means the first line is not a readable capture header.
	ErrBadHeader = errors.New("capture header is not readable")
	// ErrUnsupportedVersion means the header names a schema this build cannot read.
	ErrUnsupportedVersion = errors.New("capture schema version is not supported")
	// ErrBadEvent means an event line is malformed, truncated, or incomplete.
	ErrBadEvent = errors.New("capture event line is not readable")
	// ErrSequence means the per-file sequence counter did not advance, which is
	// how a spliced or reordered file is detected.
	ErrSequence = errors.New("capture sequence is not monotonic")
	// ErrEndOfStream is returned by Next when no event remains.
	ErrEndOfStream = errors.New("end of capture stream")
	// ErrLimitReached means the write would push the file past its byte bound.
	// Nothing was written.
	ErrLimitReached = errors.New("capture size limit reached")
)

// Header is the first line of every capture file.
//
// LocalAS, PeerAS and RouterID are the captured session's identity, and they are
// here because a replay must rebuild the SAME session. Without them a replay
// guesses, and a guess changes behavior rather than only labels: an iBGP session
// replayed as eBGP takes a different branch in OPEN validation and in the
// forwarding rules, so the replay stops reproducing the run it was fed.
type Header struct {
	Format        string `json:"format"`
	Version       int    `json:"version"`
	Peer          string `json:"peer"`
	Started       string `json:"started"`
	DaemonVersion string `json:"daemon-version"`
	LocalAS       uint32 `json:"local-as,omitempty"`
	PeerAS        uint32 `json:"peer-as,omitempty"`
	RouterID      uint32 `json:"router-id,omitempty"`
	Coalesce      bool   `json:"coalesce"`
}

// Event is one captured event line. The common fields (Seq, TS, Type) are always
// present; the rest are set only for the matching Type.
type Event struct {
	Seq  uint64 `json:"seq"`
	TS   string `json:"ts"`
	Type string `json:"type"`

	// type=message
	Direction string `json:"direction,omitempty"`
	MsgType   uint8  `json:"msg-type,omitempty"`
	Len       uint16 `json:"len,omitempty"`
	Data      []byte `json:"data,omitempty"`
	SourceID  uint32 `json:"source-id,omitempty"`
	CtxID     uint16 `json:"ctx-id,omitempty"`

	// type=config
	Op      string          `json:"op,omitempty"`
	TxID    string          `json:"tx-id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`

	// type=session
	Event string `json:"event,omitempty"`
	Drops uint64 `json:"drops,omitempty"`
}

// RedactPayload returns a config payload with every secret-bearing value
// replaced, so a capture shipped in a bug report cannot carry an operator's
// TCP-MD5 key or password.
//
// It fails closed: a payload it cannot parse is replaced entirely and the error
// is returned. The returned bytes are always safe to write.
func RedactPayload(payload []byte) ([]byte, error) {
	return redact.JSON(payload)
}
