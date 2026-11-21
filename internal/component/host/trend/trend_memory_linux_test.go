//go:build linux

package trend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The fixture box: 8 GiB of memory, so the rule's minimum rise is 81.92 MiB.
// Kernel memory starts at 200 MiB with 4 GiB still available, and what the
// kernel takes comes out of what is available, so the rule's limit stays at
// 4296 MiB whatever the level.
const (
	fixtureTotalKiB         = 8 << 20
	fixtureUnreclaimableKiB = 200 << 10
	fixtureAvailableKiB     = 4 << 20
	mebibyteKiB             = 1 << 10
)

// kernelMemoryBox is a fixture /proc tree whose meminfo the test rewrites
// before each step.
type kernelMemoryBox struct {
	t    *testing.T
	root string
}

func newKernelMemoryBox(t *testing.T) *kernelMemoryBox {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "proc"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &kernelMemoryBox{t: t, root: root}
}

// set writes a meminfo whose SUnreclaim is grownMiB above the start, taken
// from the memory available.
func (b *kernelMemoryBox) set(grownMiB uint64) {
	b.t.Helper()
	var tb textbuf.Buffer
	tb.Str("MemTotal:       ").Uint(fixtureTotalKiB).Str(" kB\n").
		Str("MemFree:        ").Uint(fixtureAvailableKiB / 2).Str(" kB\n").
		Str("MemAvailable:   ").Uint(fixtureAvailableKiB - grownMiB*mebibyteKiB).Str(" kB\n").
		Str("SReclaimable:   ").Uint(100 * mebibyteKiB).Str(" kB\n").
		Str("SUnreclaim:     ").Uint(fixtureUnreclaimableKiB + grownMiB*mebibyteKiB).Str(" kB\n")
	if err := os.WriteFile(filepath.Join(b.root, "proc", "meminfo"), []byte(tb.String()), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// VALIDATES: AC-12, AC-17 -- the proof of the first version. A kernel memory
// leak raises the kernel-memory warning on the report bus with the rule's
// code, its name as subject and its fix; a later bucket close refreshes it and
// keeps its raise time; and it clears after the growth stops for 3 judgments.
// Method: the real kernel-memory rule and the real run loop over a fixture
// /proc in t.TempDir, on a fake clock stepped one sample period (60s) at a
// time. Kernel memory grows 1 MiB a minute for 600 minutes, then holds. The
// first judgment comes at hour 7 (7 closed buckets) and confirms at hour 9;
// the floor stops rising with the bucket closed at hour 12, which clears at
// hour 14.
func TestTrendMonitorRaisesAndClearsThroughRunLoop(t *testing.T) {
	isolateTrend(t, []rule{kernelMemoryRuleOnly(t)})
	box := newKernelMemoryBox(t)
	box.set(0)
	run := startTrend(t, &host.Detector{Root: box.root})
	minute := 0
	advanceTo := func(target int) {
		t.Helper()
		for minute < target {
			minute++
			box.set(uint64(min(minute, 600)))
			run.step(time.Minute)
		}
	}

	advanceTo(9*60 - 1)
	if _, raised := warningOf(CodeKernelMemoryGrowth, kernelMemoryRuleName); raised {
		t.Fatal("the warning was raised before the third holding judgment")
	}
	if state := statusNamed(t, kernelMemoryRuleName).State(); state != StateConfirming {
		t.Fatalf("state before the third holding judgment = %s, want confirming", state)
	}

	advanceTo(9 * 60)
	first, raised := warningOf(CodeKernelMemoryGrowth, kernelMemoryRuleName)
	if !raised {
		t.Fatal("no kernel-memory warning after three holding judgments")
	}
	if first.Fix != trendFix {
		t.Errorf("fix = %q, want %q", first.Fix, trendFix)
	}
	wantStart := "Kernel memory that cannot be reclaimed grew by 360 MiB over the last 6 hours and is still growing. " +
		"At this rate the box runs out of memory in about "
	if !strings.HasPrefix(first.Message, wantStart) {
		t.Errorf("message = %q, want it to start %q", first.Message, wantStart)
	}
	if !strings.HasSuffix(first.Message, "a kernel update fixes it, and a reboot frees the memory until it grows again.") {
		t.Errorf("message = %q, want the leak explanation at its end", first.Message)
	}
	status := statusNamed(t, kernelMemoryRuleName)
	if status.State() != StateActive {
		t.Errorf("state = %s, want active", status.State())
	}
	if exhaustion, ok := status.Exhaustion(); !ok || exhaustion <= 0 || exhaustion > kernelMemoryHorizon {
		t.Errorf("exhaustion = %s (ok %v), want a projection within the horizon", exhaustion, ok)
	}

	advanceTo(10 * 60)
	refreshed, raised := warningOf(CodeKernelMemoryGrowth, kernelMemoryRuleName)
	if !raised {
		t.Fatal("the warning vanished at the next bucket close while the growth holds")
	}
	if !refreshed.Raised.Equal(first.Raised) {
		t.Errorf("raise time moved on refresh: %s, was %s", refreshed.Raised, first.Raised)
	}
	if refreshed.Updated.Before(first.Updated) {
		t.Errorf("update time %s is before the first raise's %s", refreshed.Updated, first.Updated)
	}

	advanceTo(13 * 60)
	if _, raised := warningOf(CodeKernelMemoryGrowth, kernelMemoryRuleName); !raised {
		t.Fatal("the warning cleared after two failing judgments, before the third")
	}
	if state := statusNamed(t, kernelMemoryRuleName).State(); state != StateActive {
		t.Errorf("state while counting toward the clear = %s, want active", state)
	}

	advanceTo(14 * 60)
	if _, raised := warningOf(CodeKernelMemoryGrowth, kernelMemoryRuleName); raised {
		t.Fatal("the warning did not clear after three failing judgments")
	}
	if state := statusNamed(t, kernelMemoryRuleName).State(); state != StateQuiet {
		t.Errorf("state after the clear = %s, want quiet", state)
	}
}

// VALIDATES: AC-12 survivor -- kernel memory that rises for 2 hours (a routing
// table loading) and then holds raises nothing, ever, over a long run.
// Method: the real rule and run loop over a fixture /proc on a fake clock;
// kernel memory grows 5 MiB a minute for 120 minutes, far above the minimum
// rise, then holds for 48 hours. Every step checks the bus and the state.
func TestKernelMemoryRuleQuietOnGrowthThatSettles(t *testing.T) {
	isolateTrend(t, []rule{kernelMemoryRuleOnly(t)})
	box := newKernelMemoryBox(t)
	box.set(0)
	run := startTrend(t, &host.Detector{Root: box.root})
	for minute := 1; minute <= 50*60; minute++ {
		box.set(uint64(5 * min(minute, 120)))
		run.step(time.Minute)
		if _, raised := warningOf(CodeKernelMemoryGrowth, kernelMemoryRuleName); raised {
			t.Fatalf("minute %d: growth that settled raised the warning", minute)
		}
		state := statusNamed(t, kernelMemoryRuleName).State()
		if state == StateConfirming {
			t.Fatalf("minute %d: growth that settled reads confirming", minute)
		}
		if state == StateActive {
			t.Fatalf("minute %d: growth that settled reads active", minute)
		}
	}
	if state := statusNamed(t, kernelMemoryRuleName).State(); state != StateQuiet {
		t.Errorf("state after the long run = %s, want quiet", state)
	}
}

// kernelMemoryRuleOnly answers the registered kernel-memory rule, so a test
// can run it without any other rule.
func kernelMemoryRuleOnly(t *testing.T) rule {
	t.Helper()
	for _, r := range registeredRules() {
		if r.name == kernelMemoryRuleName {
			return r
		}
	}
	t.Fatal("the kernel-memory rule is not registered")
	return rule{}
}
