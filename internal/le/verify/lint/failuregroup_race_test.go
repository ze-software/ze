// Related: failuregroup.go -- pathCollector, the watch half of the lint stage
//
// VALIDATES: one collector survives the two goroutines os/exec gives it, and
// still keeps every path both of them wrote.
// PREVENTS: the panic that killed a whole verify run rather than the stage it
// was watching: `slice bounds out of range [32893:70]`, raised from the
// re-slice in Write when a concurrent append had moved the buffer underneath.

package verifylint

import (
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
)

// TestOneCollectorSurvivesTwoWriters drives the collector the way os/exec does:
// cmd.Stdout and cmd.Stderr each wrap the SAME collector, and the copying
// goroutines run at once. Run under -race it fails on the write conflict; run
// without, it fails on the re-slice, which is how the defect reached a user.
//
// The chunk is bigger than the buffer the collector starts with, because the
// panic needed an append that reallocated while the other goroutine held the
// old backing array.
func TestOneCollectorSurvivesTwoWriters(t *testing.T) {
	collector := newPathCollector()
	writer := io.MultiWriter(io.Discard, collector)

	const writers = 8
	const rounds = 64
	var group sync.WaitGroup
	for index := range writers {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			line := "internal/le/verify/lint/f" + string(rune('a'+index)) + ".go:1:1: finding (staticcheck)\n"
			for range rounds {
				if _, err := writer.Write([]byte(strings.Repeat(line, 128))); err != nil {
					t.Errorf("write: %v", err)
					return
				}
			}
		}(index)
	}
	group.Wait()

	paths := collector.paths()
	if len(paths) == 0 {
		t.Fatal("collected no paths, so this test would pass against a collector that kept nothing")
	}
	for index := range writers {
		want := "internal/le/verify/lint/f" + string(rune('a'+index)) + ".go"
		if !slices.Contains(paths, want) {
			t.Errorf("path %q was written and not kept: %v", want, paths)
		}
	}
}
