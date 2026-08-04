package capture

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/redact"
)

func fixedTime(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, "2026-08-03T10:11:12.123456789Z")
	if err != nil {
		t.Fatalf("parse fixture time: %v", err)
	}
	return ts
}

func testHeader() Header {
	return Header{
		Format:        Format,
		Version:       Version,
		Peer:          "192.0.2.1",
		Started:       "2026-08-03T10:11:12.123456789Z",
		DaemonVersion: "test",
		Coalesce:      true,
	}
}

// VALIDATES: a config payload holding an operator secret is redacted before it
// reaches the capture file (R-1, data sensitivity).
// PREVENTS: shipping a bug report that leaks a TCP-MD5 key.
func TestRedactConfigPayload(t *testing.T) {
	in := []byte(`{"peer":{"connection":{"md5":"s3cret","remote":{"ip":"192.0.2.1"}},"password":"hunter2"}}`)
	out, err := RedactPayload(in)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	got := string(out)
	if strings.Contains(got, "s3cret") || strings.Contains(got, "hunter2") {
		t.Fatalf("secret survived redaction: %s", got)
	}
	if !strings.Contains(got, "192.0.2.1") {
		t.Fatalf("redaction destroyed non-secret content: %s", got)
	}
	if strings.Count(got, redact.Placeholder) != 2 {
		t.Fatalf("expected two redacted values, got: %s", got)
	}
}

// VALIDATES: RedactPayload fails closed -- input it cannot parse is replaced
// entirely rather than passed through.
// PREVENTS: an unparseable payload smuggling a secret into the capture.
func TestRedactPayloadFailsClosed(t *testing.T) {
	out, err := RedactPayload([]byte(`{"md5":"s3cret"`))
	if err == nil {
		t.Fatal("expected an error for malformed payload")
	}
	if bytes.Contains(out, []byte("s3cret")) {
		t.Fatalf("failed-closed output still carries the secret: %s", out)
	}
}

// VALIDATES: the writer allocates nothing per event once warm, so an enabled
// capture does not add GC pressure per message.
// PREVENTS: a per-message allocation creeping into the encode path.
func BenchmarkWriteMessage(b *testing.B) {
	wire := make([]byte, 4096)
	w := NewWriter(discardWriter{}, 0)
	if err := w.WriteHeader(testHeader()); err != nil {
		b.Fatalf("write header: %v", err)
	}
	ts := time.Unix(0, 0).UTC()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := w.WriteMessage(ts, DirectionReceived, 2, wire, 1, 1); err != nil {
			b.Fatalf("write: %v", err)
		}
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
