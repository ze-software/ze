//go:build linux

package transport

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// panicOnceClock is the real clock, except that its first Now panics. UDP.Clock
// is the reader's time source for the ifindex cache, read once per single-hop
// packet that names its ingress interface, so the panic fires inside the
// receive task on the first packet and never again.
type panicOnceClock struct {
	clock.RealClock
	fired atomic.Bool
}

func (c *panicOnceClock) Now() time.Time {
	if c.fired.CompareAndSwap(false, true) {
		panic("boom")
	}
	return c.RealClock.Now()
}

// TestBFDReceivePanicContained proves the BFD socket reader survives a Ze
// defect met while reading one packet: the packet is dropped, the reader reads
// on, the slot the packet held goes back to the pool, and the panic is logged,
// counted and crash-recorded.
// METHOD: a real single-hop UDP transport on 127.0.0.1 whose Clock (the
// existing time-source field) panics on its first read, which ingressInterface
// makes for the first packet. Send one packet, then rxPoolSize more and hold
// every Inbound unreleased: all of them arrive only if no slot was lost.
// PREVENTS: a panic in the reader killing the process, or ending the reader
// and closing RX, which stops the express loop and every session on the socket.
func TestBFDReceivePanicContained(t *testing.T) {
	restoreCrashDir := crashlog.SetCrashDirForTest(t.TempDir())
	prior := receiveGuard
	var logs bytes.Buffer
	receiveGuard = taskguard.New("bfd", slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() {
		receiveGuard = prior
		restoreCrashDir()
	})
	guard := receiveGuard

	u := &UDP{
		Bind:  netip.MustParseAddrPort("127.0.0.1:0"),
		Mode:  api.SingleHop,
		VRF:   api.DefaultVRF,
		Clock: &panicOnceClock{},
	}
	if err := u.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		if err := u.Stop(); err != nil {
			t.Errorf("Stop: %v", err)
		}
	}()

	local, ok := u.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("LocalAddr is %T, want *net.UDPAddr", u.conn.LocalAddr())
	}
	sender, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer func() {
		if err := sender.Close(); err != nil {
			t.Errorf("sender.Close: %v", err)
		}
	}()

	if _, err := sender.Write([]byte("defect")); err != nil {
		t.Fatalf("write the packet that meets the defect: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if count, _ := guard.Panics(taskReceive); count == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the receive task never contained the panic")
		}
		time.Sleep(5 * time.Millisecond)
	}

	held := make([]Inbound, 0, rxPoolSize)
	defer func() {
		for _, in := range held {
			in.Release()
		}
	}()
	for i := range rxPoolSize {
		if _, err := sender.Write([]byte{'p', byte(i)}); err != nil {
			t.Fatalf("write packet %d: %v", i, err)
		}
		select {
		case in, open := <-u.RX():
			if !open {
				t.Fatalf("RX closed after the contained panic: the reader ended")
			}
			if !bytes.Equal(in.Bytes, []byte{'p', byte(i)}) {
				t.Fatalf("packet %d bytes = %q, want the packet sent after the panic", i, in.Bytes)
			}
			held = append(held, in)
		case <-time.After(5 * time.Second):
			t.Fatalf("packet %d never arrived: the reader stopped or lost the panicking packet's slot", i)
		}
	}

	if count, declared := guard.Panics(taskReceive); count != 1 || !declared {
		t.Errorf("Panics(receive) = %d, %v, want 1, true", count, declared)
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %q", len(lines), logs.String())
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &line); err != nil {
		t.Fatalf("log line is not JSON: %v: %q", err, lines[0])
	}
	want := map[string]string{
		"level": "ERROR", "component": "bfd", "task": taskReceive, "panic": "boom",
		"unit": "single-hop socket " + local.String() + " in vrf default",
	}
	for key, value := range want {
		if line[key] != value {
			t.Errorf("log field %s = %v, want %q", key, line[key], value)
		}
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 {
		t.Fatalf("ListCrashes returned %d records, want 1", len(crashes))
	}
	if crashes[0].Kind != crashlog.KindRecovered {
		t.Errorf("crash record kind %q, want %q", crashes[0].Kind, crashlog.KindRecovered)
	}
}
