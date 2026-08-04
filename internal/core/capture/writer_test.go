package capture

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// VALIDATES: AC-2 -- each inbound message appears as one JSONL event carrying the
// full wire bytes plus arrival metadata.
// PREVENTS: a capture stream that loses the header, the sequence, or the bytes.
func TestWriterRoundTripMessage(t *testing.T) {
	var sink bytes.Buffer
	w := NewWriter(&sink, 0)
	if err := w.WriteHeader(testHeader()); err != nil {
		t.Fatalf("write header: %v", err)
	}
	wire := []byte{0xff, 0x00, 0x13, 0x04}
	if err := w.WriteMessage(fixedTime(t), DirectionReceived, 4, wire, 7, 9); err != nil {
		t.Fatalf("write message: %v", err)
	}

	r, hdr, err := NewReader(bytes.NewReader(sink.Bytes()))
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	if hdr.Peer != "192.0.2.1" || hdr.Version != Version || hdr.Format != Format {
		t.Fatalf("header round-trip mismatch: %+v", hdr)
	}
	if !hdr.Coalesce {
		t.Fatalf("coalesce flag lost: %+v", hdr)
	}

	ev, err := r.Next()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if ev.Type != EventMessage {
		t.Fatalf("type = %q, want %q", ev.Type, EventMessage)
	}
	if ev.Seq != 1 {
		t.Fatalf("seq = %d, want 1", ev.Seq)
	}
	if ev.TS != "2026-08-03T10:11:12.123456789Z" {
		t.Fatalf("ts = %q", ev.TS)
	}
	if ev.Direction != DirectionReceived {
		t.Fatalf("direction = %q", ev.Direction)
	}
	if ev.MsgType != 4 {
		t.Fatalf("msg-type = %d, want 4", ev.MsgType)
	}
	if ev.Len != uint16(len(wire)) {
		t.Fatalf("len = %d, want %d", ev.Len, len(wire))
	}
	if !bytes.Equal(ev.Data, wire) {
		t.Fatalf("data = %x, want %x", ev.Data, wire)
	}
	if ev.SourceID != 7 || ev.CtxID != 9 {
		t.Fatalf("source-id/ctx-id = %d/%d, want 7/9", ev.SourceID, ev.CtxID)
	}
	if _, err := r.Next(); !errors.Is(err, ErrEndOfStream) {
		t.Fatalf("trailing next err = %v, want ErrEndOfStream", err)
	}
}

// VALIDATES: AC-6 -- config transaction events are recorded with their txID.
// PREVENTS: a config event whose transaction identity is dropped at encode time.
func TestWriterRoundTripConfigAndSession(t *testing.T) {
	var sink bytes.Buffer
	w := NewWriter(&sink, 0)
	if err := w.WriteHeader(testHeader()); err != nil {
		t.Fatalf("write header: %v", err)
	}
	payload := json.RawMessage(`{"peer":"192.0.2.1"}`)
	if err := w.WriteConfig(fixedTime(t), OpAddPeer, "tx-42", payload); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := w.WriteSession(fixedTime(t), SessionDrops, 5); err != nil {
		t.Fatalf("write session: %v", err)
	}

	r, _, err := NewReader(bytes.NewReader(sink.Bytes()))
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	cfg, err := r.Next()
	if err != nil {
		t.Fatalf("next config: %v", err)
	}
	if cfg.Type != EventConfig || cfg.Op != OpAddPeer || cfg.TxID != "tx-42" {
		t.Fatalf("config event = %+v", cfg)
	}
	if string(cfg.Payload) != `{"peer":"192.0.2.1"}` {
		t.Fatalf("payload = %s", cfg.Payload)
	}
	ses, err := r.Next()
	if err != nil {
		t.Fatalf("next session: %v", err)
	}
	if ses.Type != EventSession || ses.Event != SessionDrops || ses.Drops != 5 {
		t.Fatalf("session event = %+v", ses)
	}
	if ses.Seq != 2 {
		t.Fatalf("seq = %d, want 2", ses.Seq)
	}
}

// VALIDATES: AC-4 -- the writer refuses to grow a capture past its byte limit, and
// refuses the whole line rather than writing a partial one.
// PREVENTS: a replay log that can fill a disk.
func TestWriterLimitIsHard(t *testing.T) {
	var sink bytes.Buffer
	w := NewWriter(&sink, 0)
	if err := w.WriteHeader(testHeader()); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if err := w.WriteMessage(fixedTime(t), DirectionReceived, 4, []byte{1, 2, 3, 4}, 0, 0); err != nil {
		t.Fatalf("write message: %v", err)
	}
	exact := int64(sink.Len())

	// One byte short of the two lines: the second write must be refused whole.
	var bounded bytes.Buffer
	lw := NewWriter(&bounded, exact-1)
	if err := lw.WriteHeader(testHeader()); err != nil {
		t.Fatalf("write header: %v", err)
	}
	headerOnly := bounded.Len()
	err := lw.WriteMessage(fixedTime(t), DirectionReceived, 4, []byte{1, 2, 3, 4}, 0, 0)
	if !errors.Is(err, ErrLimitReached) {
		t.Fatalf("err = %v, want ErrLimitReached", err)
	}
	if bounded.Len() != headerOnly {
		t.Fatalf("refused write still emitted %d bytes", bounded.Len()-headerOnly)
	}
	// test-relax: Writer.Written() was removed as exported API with no non-test
	// caller (ai/rules/completion.md). The byte-level fact it asserted is the
	// same one bounded.Len() asserts three lines above, against the sink the
	// Writer actually wrote to, so no coverage is lost.

	// At exactly the limit the write is accepted: the bound is inclusive.
	var atLimit bytes.Buffer
	ew := NewWriter(&atLimit, exact)
	if err := ew.WriteHeader(testHeader()); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if err := ew.WriteMessage(fixedTime(t), DirectionReceived, 4, []byte{1, 2, 3, 4}, 0, 0); err != nil {
		t.Fatalf("write at limit: %v", err)
	}
	if int64(atLimit.Len()) != exact {
		t.Fatalf("at-limit size = %d, want %d", atLimit.Len(), exact)
	}
}

// VALIDATES: a config payload larger than the reader's line bound still produces
// a line the reader accepts, with the dropped size named.
// PREVENTS: an unreadable capture. A reconcile records the WHOLE config tree as
// one payload, so a few hundred peers pass MaxLineLen; the reader refuses at the
// FIRST long line and stops, so one oversized reconcile would cost the operator
// every event after it.
func TestWriterBoundsAnOversizeConfigPayload(t *testing.T) {
	var sink bytes.Buffer
	w := NewWriter(&sink, 0)
	if err := w.WriteHeader(testHeader()); err != nil {
		t.Fatalf("write header: %v", err)
	}

	payload := make([]byte, 0, MaxLineLen+1024)
	payload = append(payload, '"')
	for range MaxLineLen + 1022 {
		payload = append(payload, 'x')
	}
	payload = append(payload, '"')
	if err := w.WriteConfig(fixedTime(t), OpReconcile, "tx-big", payload); err != nil {
		t.Fatalf("write oversize config: %v", err)
	}
	if err := w.WriteSession(fixedTime(t), SessionCaptureStop, 0); err != nil {
		t.Fatalf("write stop: %v", err)
	}

	r, _, err := NewReader(bytes.NewReader(sink.Bytes()))
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	cfg, err := r.Next()
	if err != nil {
		t.Fatalf("read the oversize config line: %v", err)
	}
	if cfg.Op != OpReconcile || cfg.TxID != "tx-big" {
		t.Fatalf("op/tx = %q/%q, want %q/%q", cfg.Op, cfg.TxID, OpReconcile, "tx-big")
	}
	if !bytes.Contains(cfg.Payload, []byte("capture-omitted-payload-bytes")) {
		t.Fatalf("payload does not name the omission: %s", cfg.Payload)
	}
	// The events AFTER it must still be reachable: that is the property a long
	// line destroys.
	stop, err := r.Next()
	if err != nil {
		t.Fatalf("read the event after the oversize line: %v", err)
	}
	if stop.Event != SessionCaptureStop {
		t.Fatalf("event after the oversize line = %q, want %q", stop.Event, SessionCaptureStop)
	}
}

// VALIDATES: the payload bound is exact -- the largest payload that fits is
// written whole, and the smallest that does not is replaced.
// PREVENTS: an off-by-one in `len(w.buf)+len(payload)+2 > MaxLineLen` that emits
// a line the package's own Reader refuses. A test that only uses a payload a
// kilobyte past the bound stays green through that mistake.
func TestWriterPayloadBoundIsExact(t *testing.T) {
	// Measure the line overhead for this exact op and tx by writing an empty
	// payload, so the boundary is derived rather than hard-coded.
	var probe bytes.Buffer
	pw := NewWriter(&probe, 0)
	if err := pw.WriteConfig(fixedTime(t), OpReconcile, "tx", nil); err != nil {
		t.Fatalf("probe: %v", err)
	}
	// probe holds the whole line including the trailing newline. The payload
	// field adds `,"payload":` plus the payload itself.
	overhead := probe.Len() + len(`,"payload":`)

	for _, tc := range []struct {
		name    string
		size    int
		omitted bool
	}{
		{"largest that fits", MaxLineLen - overhead, false},
		{"smallest that does not", MaxLineLen - overhead + 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := make([]byte, 0, tc.size)
			payload = append(payload, '"')
			for range tc.size - 2 {
				payload = append(payload, 'x')
			}
			payload = append(payload, '"')

			var sink bytes.Buffer
			w := NewWriter(&sink, 0)
			if err := w.WriteConfig(fixedTime(t), OpReconcile, "tx", payload); err != nil {
				t.Fatalf("write: %v", err)
			}
			if sink.Len() > MaxLineLen {
				t.Fatalf("emitted a %d-byte line, past the %d-byte reader bound", sink.Len(), MaxLineLen)
			}
			omitted := bytes.Contains(sink.Bytes(), []byte("capture-omitted-payload-bytes"))
			if omitted != tc.omitted {
				t.Fatalf("omitted = %v, want %v (line is %d bytes)", omitted, tc.omitted, sink.Len())
			}
		})
	}
}

// VALIDATES: a quoted field value cannot push a line past the reader's bound.
// PREVENTS: captureOperationPhase's default branch, which passes an operation
// name straight through, emitting an unreadable line. WriteConfig bounds the
// payload only.
func TestWriterBoundsAQuotedField(t *testing.T) {
	var sink bytes.Buffer
	w := NewWriter(&sink, 0)
	if err := w.WriteHeader(testHeader()); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if err := w.WriteConfig(fixedTime(t), strings.Repeat("o", MaxLineLen*2), "tx", nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	if sink.Len() > MaxLineLen {
		t.Fatalf("emitted a %d-byte line, past the %d-byte reader bound", sink.Len(), MaxLineLen)
	}
	r, _, err := NewReader(bytes.NewReader(sink.Bytes()))
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	if _, err := r.Next(); err != nil {
		t.Fatalf("reader refused the line: %v", err)
	}
}

// VALIDATES: the header itself is bounded, so a limit smaller than one header
// fails closed instead of writing an unreadable file.
// PREVENTS: a size cap that the header alone can exceed.
func TestWriterHeaderRespectsLimit(t *testing.T) {
	var sink bytes.Buffer
	w := NewWriter(&sink, 4)
	if err := w.WriteHeader(testHeader()); !errors.Is(err, ErrLimitReached) {
		t.Fatalf("err = %v, want ErrLimitReached", err)
	}
	if sink.Len() != 0 {
		t.Fatalf("refused header still emitted %d bytes", sink.Len())
	}
}

// VALIDATES: every wire byte value survives the JSONL encoding, including the
// 0xff marker bytes and every non-printable byte.
// PREVENTS: an encoder that mangles binary content.
func TestWriterEncodesEveryByteValue(t *testing.T) {
	wire := make([]byte, 256)
	for i := range wire {
		wire[i] = byte(i)
	}
	var sink bytes.Buffer
	w := NewWriter(&sink, 0)
	if err := w.WriteHeader(testHeader()); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if err := w.WriteMessage(fixedTime(t), DirectionReceived, 2, wire, 0, 0); err != nil {
		t.Fatalf("write message: %v", err)
	}
	if bytes.Count(sink.Bytes(), []byte{'\n'}) != 2 {
		t.Fatalf("encoded stream is not two lines: %d newlines", bytes.Count(sink.Bytes(), []byte{'\n'}))
	}

	r, _, err := NewReader(bytes.NewReader(sink.Bytes()))
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if !bytes.Equal(ev.Data, wire) {
		t.Fatalf("byte round-trip mismatch")
	}
}
