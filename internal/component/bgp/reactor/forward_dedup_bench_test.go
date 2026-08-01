// Related: forward_dedup_test.go -- the fan-out harness and the (n,g) case list
// Related: forward_dedup.go -- the materialization counters reported per operation

package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// BenchmarkFanoutDedup measures one received UPDATE fanned out to n destinations
// spread over g policy groups.
//
// It exists because the repository's other measurement cannot answer the
// question. ze-perf-bench is a single-peer convergence run with almost no
// fan-out, so forwardUpdateCore and everything under it are absent from a
// 300-node profile: a change that halves per-destination forwarding cost shows
// up there as nothing at all. This benchmark is the only evidence for or against
// the fan-out dedup, and A-1 in plan/spec-wire-edit-5-fanout-dedup.md is settled
// by its numbers rather than by argument.
//
// Read ns/dest, not ns/op: the whole point is per-destination cost, and ns/op
// scales with n by construction. materializations/op is the mechanism -- it must
// fall from n to g for the ns/dest improvement to be the one this child claims.
//
//	go test -tags 'ze_core ze_bgp' -run=^$ -bench=BenchmarkFanoutDedup -benchmem \
//	    ./internal/component/bgp/reactor/
//
// The two arms are INTERLEAVED per case rather than run as two `go test` passes.
// This machine is shared, and a sibling session's QEMU boot moved every number by
// 2x in the middle of a two-pass comparison -- including the one-destination case
// the change provably cannot touch. Two arms under one load answer the question;
// two runs under two loads do not.
func BenchmarkFanoutDedup(b *testing.B) {
	for _, tc := range fanoutCases {
		b.Run(fanoutCaseName(tc.n, tc.g)+"/dedup=off", func(b *testing.B) {
			runFanoutBench(b, tc.n, tc.g, fanoutOpts{modify: true, groups: true, dedupOff: true})
		})
		b.Run(fanoutCaseName(tc.n, tc.g)+"/dedup=on", func(b *testing.B) {
			runFanoutBench(b, tc.n, tc.g, fanoutOpts{modify: true, groups: true})
		})
	}
}

// BenchmarkFanoutFloor measures the same fan-out with no per-peer policy, so no
// destination materializes at all.
//
// It is the floor, and it is what makes the dedup numbers readable. Dedup can
// only ever move the modify=true measurement DOWN towards this line, never below
// it: the destination loop, the forward-facts read, the item construction and
// the pool dispatch are paid per destination whatever sharing happens above
// them. Reporting the win without it would invite reading a 30% improvement as
// though 100% were available.
func BenchmarkFanoutFloor(b *testing.B) {
	for _, tc := range fanoutCases {
		b.Run(fanoutCaseName(tc.n, tc.g), func(b *testing.B) {
			runFanoutBench(b, tc.n, tc.g, fanoutOpts{modify: false, groups: true})
		})
	}
}

// BenchmarkFanoutRebuildOnly measures the two halves of what a shared
// materialization would skip, in isolation: the buildModifiedPayload rebuild,
// and a flat copy of its result.
//
// It decides the SHAPE of the dedup rather than whether to do it. Sharing the
// output BUFFER across destinations removes both halves but gives one buffer
// several referencing forward items, which is the ownership problem A-4 names.
// Sharing only the PLAN -- reusing the first destination's bytes but copying
// them into each destination's own pool buffer -- leaves the one-buffer-one-item
// lifetime exactly as it is today. The gap between these two numbers is what
// that safety costs.
func BenchmarkFanoutRebuildOnly(b *testing.B) {
	payload := fanoutPayload()
	handlers := attrModHandlersWithDefaults()
	nhLegacy := [4]byte{10, 99, 0, 1}
	nhMapped := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF, 10, 99, 0, 1}

	b.Run("rebuild", func(b *testing.B) {
		var mods filterapi.ModAccumulator
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			mods.Reset()
			mods.Op(3, filterapi.AttrModSet, nhLegacy[:])
			mods.Op(14, filterapi.AttrModSet, nhMapped[:])
			mods.Op(40, filterapi.AttrModSuppress, nil)
			out, _, fail := buildModifiedPayload(payload, &mods, handlers, nil, nil)
			if out == nil || fail.failed() {
				b.Fatal("rebuild failed")
			}
		}
	})

	b.Run("copy", func(b *testing.B) {
		var mods filterapi.ModAccumulator
		mods.Op(3, filterapi.AttrModSet, nhLegacy[:])
		mods.Op(14, filterapi.AttrModSet, nhMapped[:])
		mods.Op(40, filterapi.AttrModSuppress, nil)
		src, _, _ := buildModifiedPayload(payload, &mods, handlers, nil, nil)
		dst := make([]byte, len(src))
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			copy(dst, src)
		}
	})
}

func runFanoutBench(b *testing.B, n, g int, opts fanoutOpts) {
	b.Helper()
	h := newFanoutHarnessWith(b, n, g, opts)

	// One untimed pass so the per-peer outgoing pools, the span index pool and
	// the encoding-context registry are warm. Without it the first iteration
	// pays every lazy allocation in the rail and a short benchmark reports that
	// as steady-state cost.
	if err := h.forward(); err != nil {
		b.Fatal(err)
	}

	before := readFanoutCounters()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := h.forward(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	got := readFanoutCounters().since(before)

	perOp := float64(b.N)
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/(perOp*float64(n)), "ns/dest")
	b.ReportMetric(float64(got.materializations)/perOp, "materializations/op")
	b.ReportMetric(float64(got.dedupHits)/perOp, "dedup-hits/op")
}
