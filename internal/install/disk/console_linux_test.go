// Design: docs/architecture/appliance/installer-initrd.md -- console fan-out tests

//go:build linux && ze_installer

package disk

import (
	"bytes"
	"testing"
)

func TestConsoleWriterFansToAll(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	cw := &consoleWriter{writers: []writerEntry{
		{w: &buf1},
		{w: &buf2},
	}}

	msg := []byte("[ze-install] test message\n")
	n, err := cw.Write(msg)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(msg) {
		t.Fatalf("Write returned %d, want %d", n, len(msg))
	}
	if buf1.String() != string(msg) {
		t.Fatalf("writer 1: got %q, want %q", buf1.String(), string(msg))
	}
	if buf2.String() != string(msg) {
		t.Fatalf("writer 2: got %q, want %q", buf2.String(), string(msg))
	}
}

func TestConsoleWriterSurvivesFailedWriter(t *testing.T) {
	var good bytes.Buffer
	bad := &failWriter{}
	cw := &consoleWriter{writers: []writerEntry{
		{w: bad},
		{w: &good},
	}}

	msg := []byte("still delivered\n")
	n, err := cw.Write(msg)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(msg) {
		t.Fatalf("Write returned %d, want %d", n, len(msg))
	}
	if good.String() != string(msg) {
		t.Fatalf("good writer: got %q, want %q", good.String(), string(msg))
	}
}

type failWriter struct{}

func (fw *failWriter) Write([]byte) (int, error) {
	return 0, bytes.ErrTooLarge
}
