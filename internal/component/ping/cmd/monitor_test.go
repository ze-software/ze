package cmd

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// TestWritePingRowsStreamsEachProbe proves the streaming route of `monitor
// ping` writes the header and one line per row the session produces, the text
// `ze monitor ping` printed before the route was shared. Method: feed a closed
// channel holding an answered and a lost probe and compare the text.
func TestWritePingRowsStreamsEachProbe(t *testing.T) {
	ch := make(chan map[string]any, 2)
	ch <- map[string]any{"seq": 1, fieldStatus: "ok", "rtt-ms": 1.25}
	ch <- map[string]any{"seq": 2, fieldStatus: "timeout"}
	close(ch)

	var out strings.Builder
	mp := monitorPingArgs{Dest: netip.MustParseAddr("192.0.2.1")}
	if err := writePingRows(&out, mp, ch); err != nil {
		t.Fatalf("writePingRows: %v", err)
	}
	want := "PING 192.0.2.1 (Ctrl-C to stop)\n  seq=1  rtt=1.250ms\n  seq=2  timeout\n"
	if out.String() != want {
		t.Errorf("stream text:\n%q\nwant\n%q", out.String(), want)
	}
}

// failingWriter refuses every write, as a closed SSH channel does.
type failingWriter struct{}

var errClosedChannel = errors.New("channel closed")

func (failingWriter) Write([]byte) (int, error) { return 0, errClosedChannel }

// TestWritePingRowsStopsOnWriteError proves a caller that went away ends the
// stream with the write error rather than a silent success.
func TestWritePingRowsStopsOnWriteError(t *testing.T) {
	ch := make(chan map[string]any)
	close(ch)
	mp := monitorPingArgs{Dest: netip.MustParseAddr("192.0.2.1")}
	if err := writePingRows(failingWriter{}, mp, ch); !errors.Is(err, errClosedChannel) {
		t.Fatalf("writePingRows on a closed channel: %v, want %v", err, errClosedChannel)
	}
}
