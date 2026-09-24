package crashlog

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// gatedWriter is a downstream that takes nothing until its gate opens, standing
// in for a stalled syslog socket or a serial console.
type gatedWriter struct {
	gate <-chan struct{}
	mu   sync.Mutex
	got  bytes.Buffer
}

func (g *gatedWriter) Write(p []byte) (int, error) {
	<-g.gate
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.got.Write(p)
}

func (g *gatedWriter) bytes() []byte {
	g.mu.Lock()
	defer g.mu.Unlock()
	return bytes.Clone(g.got.Bytes())
}

// TestRelayDoesNotBlockWriterOnStalledDownstream proves a burst many times a
// pipe buffer drains through the relay while the downstream takes nothing, and
// that every byte arrives once the downstream moves again.
//
// VALIDATES: a 1.6 MiB write into the relay pipe completes while the
// downstream is stalled, and the downstream then receives the burst unchanged.
// PREVENTS: the relay writing downstream inline, which let a stalled syslog or
// console fill the pipe and block every writer of stderr behind it.
func TestRelayDoesNotBlockWriterOnStalledDownstream(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	sink := &gatedWriter{gate: gate}
	done := startRelay(pr, sink, nil, "")

	burst := bytes.Repeat([]byte(strings.Repeat("y", 99)+"\n"), 16*1024)
	if len(burst) >= relayQueueLimit {
		t.Fatalf("burst %d bytes does not fit the queue limit %d; the test proves nothing about drops", len(burst), relayQueueLimit)
	}
	wrote := make(chan error, 1)
	go func() {
		_, werr := pw.Write(burst)
		wrote <- werr
	}()
	select {
	case werr := <-wrote:
		if werr != nil {
			t.Fatalf("burst write failed: %v", werr)
		}
	case <-time.After(10 * time.Second):
		close(gate)
		t.Fatal("the writer blocked on a stalled downstream")
	}

	close(gate)
	if err := pw.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the relay did not finish after the pipe closed")
	}
	if got := sink.bytes(); !bytes.Equal(got, burst) {
		t.Fatalf("downstream got %d bytes, want %d", len(got), len(burst))
	}
}

// TestStderrQueueDropsPastLimitAndSaysSo proves the queue never blocks past its
// limit and reports what it dropped where it dropped it.
//
// VALIDATES: a write past the limit returns at once; the notice carrying the
// count sits between the kept bytes and the next kept write; a write while the
// queue is still over its limit is dropped; a drop still pending at close is
// reported before the end.
// PREVENTS: an unbounded queue, and a drop that reads as output never written.
func TestStderrQueueDropsPastLimitAndSaysSo(t *testing.T) {
	q := newStderrQueue(10)
	if n, err := q.Write([]byte("0123456789ABCDEF")); n != 16 || err != nil {
		t.Fatalf("Write = %d, %v; want 16, nil", n, err)
	}
	head := make([]byte, 10)
	if _, err := io.ReadFull(q, head); err != nil || string(head) != "0123456789" {
		t.Fatalf("read %q, %v; want the ten bytes that fit", head, err)
	}
	if _, err := q.Write([]byte("next\n")); err != nil {
		t.Fatal(err)
	}
	// The notice and "next\n" already exceed the limit, so the relay is behind
	// and all ten of these are dropped.
	if _, err := q.Write([]byte("0123456789")); err != nil {
		t.Fatal(err)
	}
	q.close(nil)
	rest, err := io.ReadAll(q)
	if err != nil {
		t.Fatal(err)
	}
	notice := func(n string) string {
		return "\ncrashlog: " + n + " bytes of stderr dropped: the relay fell behind its output\n"
	}
	if want := notice("6") + "next\n" + notice("10"); string(rest) != want {
		t.Fatalf("after the first read got %q, want %q", rest, want)
	}
}
