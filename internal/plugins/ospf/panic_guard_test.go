// Design: docs/architecture/ospf/ospf-4-component-config.md -- panic containment in the engine
//
// VALIDATES: a panic in a registered consumer, builder or show renderer is never silent:
// the engine's guard logs, counts and crash-records it whatever the caller passes.
package ospf

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// enginePanicBaseline reads task's count on the REAL engine guard and points crash
// records at a fresh directory, so requireEngineContained can assert the delta.
func enginePanicBaseline(t *testing.T, task string) uint64 {
	t.Helper()
	before, declared := enginePanics().guard.Panics(task)
	if !declared {
		t.Fatalf("task %q is not declared on the ospf engine boundary", task)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	return before
}

// requireEngineContained asserts one contained panic of task: counted once, and one
// recovered crash record naming the component, the task and the unit.
func requireEngineContained(t *testing.T, before uint64, task, unit string) {
	t.Helper()
	if n, _ := enginePanics().guard.Panics(task); n != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d: the panic was not counted", task, n, before+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
	want := "ospf " + task + ", unit " + unit
	if record := crashlog.ReadCrash(crashes[0].Name); !strings.Contains(record, want) {
		t.Fatalf("crash record lacks %q:\n%s", want, record)
	}
}

// TestOSPFRenderPanicNeverSilent: renderExtSubTLV had no counter to hand the recover
// (safeOpaqueCall(nil, ...)), so a panicking Render was swallowed with no log line, metric
// or record. It now goes through the show-decode task, and the caller still gets "" (the
// type/length/hex fallback).
func TestOSPFRenderPanicNeverSilent(t *testing.T) {
	before := enginePanicBaseline(t, "show-decode")
	reg := map[uint16]extSubTLVCodec{5: {Render: func([]byte) string { panic("test: Render seam panics") }}}

	if got := renderExtSubTLV(reg, packet.ExtSubTLV{Type: 5, Value: []byte{1}}); got != "" {
		t.Fatalf("renderExtSubTLV after a panic = %q, want the empty fallback", got)
	}
	requireEngineContained(t, before, "show-decode", "extended sub-TLV renderer")
}

// TestOSPFBuilderPanicRecorded: a panicking Router Information TLV builder contributes no
// TLVs, steps the builder's own error counter, and is logged, counted and recorded by the
// guard (before, only the caller's counter saw it).
func TestOSPFBuilderPanicRecorded(t *testing.T) {
	before := enginePanicBaseline(t, "consumer")
	steps := 0
	entry := riTLVEntry{build: func(types.RouterID) []packet.RITLV { panic("test: RI builder seam panics") }}

	if tlvs := invokeRITLVBuilder(entry, types.RouterID{}, func() { steps++ }); tlvs != nil {
		t.Fatalf("invokeRITLVBuilder after a panic = %v, want no TLVs", tlvs)
	}
	if steps != 1 {
		t.Fatalf("builder error counter stepped %d times, want 1", steps)
	}
	requireEngineContained(t, before, "consumer", "router-information TLV builder")
}
