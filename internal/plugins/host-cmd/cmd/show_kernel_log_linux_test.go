// VALIDATES: parseLevelArg maps kmsg level names and 0..7 numerics to their level
// index and falls back to 7 (debug) for anything else, parseKernelLogArgs honors
// the count/level bounds, drainKmsg terminates on BOTH of its exits, and isEAGAIN
// unwraps wrapped syscall.EAGAIN errors.
// PREVENTS: a mis-parsed kernel-log level filter (silently widening/narrowing the
// output), an out-of-range count reaching the reader, or a missed EAGAIN causing a
// non-blocking read to be treated as fatal.

//go:build linux

package cmd

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"
)

func TestParseLevelArg(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"emerg", 0},
		{"err", 3},
		{"warning", 4},
		{"debug", 7},
		{"0", 0},
		{"5", 5},
		{"7", 7},
		{"8", 7},     // numeric out of range → fallback
		{"-1", 7},    // negative → fallback
		{"bogus", 7}, // unknown name → fallback
		{"", 7},
	} {
		if got := parseLevelArg(tc.in); got != tc.want {
			t.Errorf("parseLevelArg(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// recordPair returns a non-blocking (reader, writer) descriptor pair that
// preserves MESSAGE BOUNDARIES, because /dev/kmsg does: one read returns exactly
// one record. A pipe would concatenate the writes into a single read and the
// per-record parsing under test would never be exercised.
func recordPair(t *testing.T) (r, w int) {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_DGRAM|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	return fds[0], fds[1]
}

// parseKernelLogArgs must clamp `count` to 1..maxKernelLogCount and reject
// anything non-numeric, because the value sizes the slice the handler returns to
// an operator over the wire.
func TestParseKernelLogArgsBounds(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		wantCount int
		wantLevel int
	}{
		{"no args", nil, defaultKernelLogCount, 7},
		{"lower bound", []string{"count", "1"}, 1, 7},
		{"below lower bound", []string{"count", "0"}, defaultKernelLogCount, 7},
		{"negative", []string{"count", "-1"}, defaultKernelLogCount, 7},
		{"upper bound", []string{"count", "10000"}, maxKernelLogCount, 7},
		{"above upper bound", []string{"count", "10001"}, defaultKernelLogCount, 7},
		{"non-numeric", []string{"count", "many"}, defaultKernelLogCount, 7},
		{"count with no value", []string{"count"}, defaultKernelLogCount, 7},
		{"level name", []string{"level", "err"}, defaultKernelLogCount, 3},
		{"count and level", []string{"count", "5", "level", "warning"}, 5, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count, level := parseKernelLogArgs(tc.args)
			if count != tc.wantCount {
				t.Errorf("count = %d, want %d", count, tc.wantCount)
			}
			if level != tc.wantLevel {
				t.Errorf("maxLevel = %d, want %d", level, tc.wantLevel)
			}
		})
	}
}

// drainKmsg must RETURN once the descriptor has no more records queued.
//
// The reader it replaced used os.OpenFile + (*os.File).Read, which registers the
// descriptor with the runtime netpoller on Linux; a pollable descriptor never
// surfaces EAGAIN, it parks the goroutine until the fd is readable again. On
// /dev/kmsg that is "until the kernel logs a new message", so the drained case
// blocked forever, the ze-show:system-kernel-log RPC never answered, and
// test/plugin/system-kernel-log-show.ci timed out on any host privileged enough
// to open the device. Both exits are covered here: a drained non-blocking
// descriptor (EAGAIN) and one whose writer has closed (0, nil).
func TestDrainKmsgTerminates(t *testing.T) {
	for _, tc := range []struct {
		name        string
		closeWriter bool
	}{
		{"drained returns EAGAIN", false},
		{"writer closed returns EOF", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, w := recordPair(t)
			defer syscall.Close(r) //nolint:errcheck // test cleanup

			// Two records the reader must collect before it runs dry.
			if _, err := syscall.Write(w, []byte("6,1,100;first message")); err != nil {
				t.Fatalf("write: %v", err)
			}
			if _, err := syscall.Write(w, []byte("3,2,200;second message")); err != nil {
				t.Fatalf("write: %v", err)
			}
			if tc.closeWriter {
				if err := syscall.Close(w); err != nil {
					t.Fatalf("close writer: %v", err)
				}
			} else {
				defer syscall.Close(w) //nolint:errcheck // test cleanup
			}

			done := make(chan []map[string]any, 1)
			go func() { done <- drainKmsg(r, 10, 7) }()

			select {
			case entries := <-done:
				if len(entries) != 2 {
					t.Fatalf("entries = %d, want 2: %v", len(entries), entries)
				}
				if got := entries[0]["message"]; got != "first message" {
					t.Errorf("entries[0].message = %v, want %q", got, "first message")
				}
				if got := entries[1]["level"]; got != "err" {
					t.Errorf("entries[1].level = %v, want %q", got, "err")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("drainKmsg did not return: the drained-descriptor exit is unreachable again, which hangs `show system kernel-log`")
			}
		})
	}
}

// The newest-N trim must keep the LAST count entries, not the first: an operator
// asking for `count 5` wants the five most recent kernel messages.
func TestDrainKmsgKeepsNewest(t *testing.T) {
	r, w := recordPair(t)
	defer syscall.Close(r) //nolint:errcheck // test cleanup

	for i := 1; i <= 4; i++ {
		rec := fmt.Sprintf("6,%d,%d00;message %d", i, i, i)
		if _, err := syscall.Write(w, []byte(rec)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := syscall.Close(w); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	entries := drainKmsg(r, 2, 7)
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if got := entries[0]["message"]; got != "message 3" {
		t.Errorf("entries[0].message = %v, want %q", got, "message 3")
	}
	if got := entries[1]["message"]; got != "message 4" {
		t.Errorf("entries[1].message = %v, want %q", got, "message 4")
	}
}

// A level filter must drop records above the requested maximum.
func TestDrainKmsgFiltersByLevel(t *testing.T) {
	r, w := recordPair(t)
	defer syscall.Close(r) //nolint:errcheck // test cleanup

	if _, err := syscall.Write(w, []byte("3,1,100;an error")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := syscall.Write(w, []byte("7,2,200;a debug line")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := syscall.Close(w); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	entries := drainKmsg(r, 10, 3)
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1 (debug must be filtered out)", len(entries))
	}
	if got := entries[0]["message"]; got != "an error" {
		t.Errorf("entries[0].message = %v, want %q", got, "an error")
	}
}

func TestIsEAGAIN(t *testing.T) {
	if !isEAGAIN(syscall.EAGAIN) {
		t.Error("isEAGAIN(EAGAIN) = false, want true")
	}
	if !isEAGAIN(fmt.Errorf("read failed: %w", syscall.EAGAIN)) {
		t.Error("isEAGAIN(wrapped EAGAIN) = false, want true")
	}
	if isEAGAIN(nil) {
		t.Error("isEAGAIN(nil) = true, want false")
	}
	if isEAGAIN(errors.New("boom")) {
		t.Error("isEAGAIN(unrelated) = true, want false")
	}
}
