package process

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestStderrTailRing(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var r stderrTailRing
		got := r.Snapshot()
		if got == nil {
			t.Fatal("Snapshot() = nil, want non-nil empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("len(Snapshot()) = %d, want 0", len(got))
		}
		raw, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if string(raw) != "[]" {
			t.Fatalf("JSON = %s, want []", string(raw))
		}
	})

	t.Run("exact-capacity", func(t *testing.T) {
		var r stderrTailRing
		for i := range stderrTailMaxLines {
			r.Add("line-" + strconv.Itoa(i))
		}
		got := r.Snapshot()
		if len(got) != stderrTailMaxLines {
			t.Fatalf("len(Snapshot()) = %d, want %d", len(got), stderrTailMaxLines)
		}
		for i := range stderrTailMaxLines {
			want := "line-" + strconv.Itoa(i)
			if got[i] != want {
				t.Fatalf("Snapshot()[%d] = %q, want %q", i, got[i], want)
			}
		}
	})

	t.Run("overflow-keeps-newest-32-in-order", func(t *testing.T) {
		var r stderrTailRing
		const total = 50
		for i := range total {
			r.Add("line-" + strconv.Itoa(i))
		}
		got := r.Snapshot()
		if len(got) != stderrTailMaxLines {
			t.Fatalf("len(Snapshot()) = %d, want %d", len(got), stderrTailMaxLines)
		}
		first := total - stderrTailMaxLines
		for i := range stderrTailMaxLines {
			want := "line-" + strconv.Itoa(first+i)
			if got[i] != want {
				t.Fatalf("Snapshot()[%d] = %q, want %q", i, got[i], want)
			}
		}
	})

	t.Run("truncates-long-ascii-and-utf8-cleanly", func(t *testing.T) {
		var r stderrTailRing
		longASCII := strings.Repeat("a", 400)
		r.Add(longASCII)

		// 255 ASCII bytes + a 3-byte UTF-8 rune ("€") straddling byte 256.
		straddleUTF8 := strings.Repeat("b", 255) + "€tail"
		r.Add(straddleUTF8)

		got := r.Snapshot()
		if len(got) != 2 {
			t.Fatalf("len(Snapshot()) = %d, want 2", len(got))
		}
		if len(got[0]) != stderrTailMaxBytes {
			t.Errorf("len(ascii) = %d, want %d", len(got[0]), stderrTailMaxBytes)
		}
		if len(got[1]) > stderrTailMaxBytes {
			t.Errorf("len(utf8) = %d, want <= %d", len(got[1]), stderrTailMaxBytes)
		}
		if !utf8.ValidString(got[1]) {
			t.Errorf("truncated UTF-8 line is invalid UTF-8: %q", got[1])
		}
		if got[1] != strings.Repeat("b", 255) {
			t.Errorf("truncated UTF-8 line = %q, want 255 'b's", got[1])
		}
	})
}

func TestRelayStderrFromCapturesTailWhenRelayDisabled(t *testing.T) {
	t.Cleanup(env.ResetCache)
	t.Setenv("ze.log.relay", "disabled")
	env.ResetCache()
	if _, enabled := slogutil.RelayLevel(); enabled {
		t.Fatal("expected slogutil.RelayLevel() to report disabled")
	}

	p := NewProcess(plugin.PluginConfig{Name: "ext-quiet"})
	input := "first line\nsecond line\nthird line\n"
	p.relayStderrFrom(strings.NewReader(input))

	snap := p.HostInspect()
	want := []string{"first line", "second line", "third line"}
	if len(snap.StderrTail) != len(want) {
		t.Fatalf("len(StderrTail) = %d, want %d (%v)", len(snap.StderrTail), len(want), snap.StderrTail)
	}
	for i := range want {
		if snap.StderrTail[i] != want[i] {
			t.Errorf("StderrTail[%d] = %q, want %q", i, snap.StderrTail[i], want[i])
		}
	}
}

func TestHostInspectLivePID(t *testing.T) {
	p := NewProcess(plugin.PluginConfig{Name: "ext-live"})
	p.isExternal.Store(true)
	selfPID := os.Getpid()
	if selfPID <= 0 || selfPID > math.MaxInt32 {
		t.Fatalf("unexpected os.Getpid() = %d", selfPID)
	}
	p.pid.Store(int32(selfPID)) //nolint:gosec // G115: checked above
	p.running.Store(true)
	p.SetStage(plugin.StageRunning)
	p.stderrTail.Add("plugin ready")

	if p.PID() != selfPID {
		t.Fatalf("PID() = %d, want %d", p.PID(), selfPID)
	}
	if !p.IsExternal() {
		t.Fatal("IsExternal() = false, want true")
	}

	snap := p.HostInspect()
	if snap.PID != selfPID {
		t.Errorf("snap.PID = %d, want %d", snap.PID, selfPID)
	}
	if !snap.Running {
		t.Error("snap.Running = false, want true")
	}
	if snap.Stage != "Running" {
		t.Errorf("snap.Stage = %q, want %q", snap.Stage, "Running")
	}
	if !snap.External {
		t.Error("snap.External = false, want true")
	}
	if len(snap.StderrTail) != 1 || snap.StderrTail[0] != "plugin ready" {
		t.Errorf("snap.StderrTail = %v, want [plugin ready]", snap.StderrTail)
	}

	if runtime.GOOS == "linux" {
		if snap.ProcError != "" {
			t.Fatalf("snap.ProcError = %q, want empty on Linux", snap.ProcError)
		}
		if snap.Proc == nil {
			t.Fatal("snap.Proc = nil, want non-nil on Linux")
		}
		if snap.Proc.Threads <= 0 {
			t.Errorf("snap.Proc.Threads = %d, want > 0", snap.Proc.Threads)
		}
		if snap.Proc.VmRSSKB <= 0 {
			t.Errorf("snap.Proc.VmRSSKB = %d, want > 0", snap.Proc.VmRSSKB)
		}
		if snap.Proc.FDCount == nil || *snap.Proc.FDCount <= 0 {
			t.Errorf("snap.Proc.FDCount = %v, want > 0", snap.Proc.FDCount)
		}
	} else if snap.ProcError == "" {
		t.Error("snap.ProcError is empty on non-Linux, want unsupported error")
	}

	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(raw)
	if strings.Contains(s, `"External"`) || strings.Contains(s, `"external"`) {
		t.Errorf("JSON unexpectedly serialized External field: %s", s)
	}
	if !strings.Contains(s, `"stderr-tail":["plugin ready"]`) {
		t.Errorf("JSON missing stderr-tail: %s", s)
	}
}

func TestHostInspectClearedAndInternalPID(t *testing.T) {
	t.Run("external-cleared-pid", func(t *testing.T) {
		p := NewProcess(plugin.PluginConfig{Name: "ext-exited"})
		p.isExternal.Store(true)
		p.pid.Store(0)

		snap := p.HostInspect()
		if snap.PID != 0 {
			t.Errorf("snap.PID = %d, want 0", snap.PID)
		}
		if snap.Proc != nil {
			t.Errorf("snap.Proc = %+v, want nil when pid == 0", snap.Proc)
		}
		if snap.ProcError != "process has no live OS pid" {
			t.Errorf("snap.ProcError = %q, want %q", snap.ProcError, "process has no live OS pid")
		}
		raw, err := json.Marshal(snap)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if !strings.Contains(string(raw), `"stderr-tail":[]`) {
			t.Errorf("JSON = %s, want non-null empty stderr-tail array", string(raw))
		}
	})

	t.Run("internal-process", func(t *testing.T) {
		p := NewProcess(plugin.PluginConfig{Name: "int-plugin"})
		if p.IsExternal() {
			t.Fatal("IsExternal() = true for internal process, want false")
		}
		snap := p.HostInspect()
		if snap.PID != 0 {
			t.Errorf("snap.PID = %d, want 0", snap.PID)
		}
		if snap.Proc != nil {
			t.Errorf("snap.Proc = %+v, want nil", snap.Proc)
		}
		if snap.ProcError != "" {
			t.Errorf("snap.ProcError = %q, want empty for internal process", snap.ProcError)
		}
	})
}

func TestMonitorCmdClearsPIDAndRetainsStderrTail(t *testing.T) {
	p := NewProcess(plugin.PluginConfig{Name: "ext-exit"})
	p.ctx, p.cancel = context.WithCancel(t.Context())
	defer p.cancel()

	cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", "printf 'final-diagnostic-line\\n' >&2") //nolint:gosec // fixed argv, no user input
	read, write, err := attachStderrRelay(cmd)
	if err != nil {
		t.Fatalf("attachStderrRelay: %v", err)
	}
	if startErr := cmd.Start(); startErr != nil {
		write.Close() //nolint:errcheck,gosec // test cleanup
		read.Close()  //nolint:errcheck,gosec // test cleanup
		t.Fatalf("cmd.Start: %v", startErr)
	}
	if closeErr := write.Close(); closeErr != nil {
		t.Fatalf("close write end: %v", closeErr)
	}

	p.isExternal.Store(true)
	if cmd.Process.Pid <= 0 || cmd.Process.Pid > math.MaxInt32 {
		t.Fatalf("unexpected child pid %d", cmd.Process.Pid)
	}
	p.pid.Store(int32(cmd.Process.Pid)) //nolint:gosec // G115: checked above
	p.running.Store(true)
	p.externalDone = make(chan struct{})

	relayDone := make(chan struct{})
	reaped := make(chan struct{})
	relayRead := newStderrRelayReader(read, stderrDrainGrace)
	go func() {
		defer close(relayDone)
		p.relayStderrFrom(relayRead)
	}()

	pg := &pluginGroup{name: p.config.Name, pid: cmd.Process.Pid}
	p.monitorCmd(cmd, relayDone, relayRead, reaped, pg)
	<-p.externalDone

	if p.PID() != 0 {
		t.Fatalf("PID() after monitorCmd = %d, want 0", p.PID())
	}
	if p.Running() {
		t.Fatal("Running() after monitorCmd = true, want false")
	}

	snap := p.HostInspect()
	if snap.PID != 0 || snap.Proc != nil {
		t.Fatalf("HostInspect after reap = %+v, want PID=0 and Proc=nil", snap)
	}
	if snap.ProcError != "process has no live OS pid" {
		t.Errorf("snap.ProcError = %q, want %q", snap.ProcError, "process has no live OS pid")
	}
	if !slices.Contains(snap.StderrTail, "final-diagnostic-line") {
		t.Errorf("snap.StderrTail = %v, want line %q", snap.StderrTail, "final-diagnostic-line")
	}
}
