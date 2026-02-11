package pool

import (
	"fmt"
	"testing"
)

// BenchmarkInternExisting measures performance of interning existing data.
// Target: < 100ns per operation.
func BenchmarkInternExisting(b *testing.B) {
	p := New(1024 * 1024)
	p.Intern([]byte("benchmark-data"))

	for b.Loop() {
		p.Intern([]byte("benchmark-data"))
	}
}

// BenchmarkInternNew measures performance of interning new unique data.
// Target: < 500ns per operation.
func BenchmarkInternNew(b *testing.B) {
	p := New(1024 * 1024 * 100) // 100MB to avoid reallocation

	for i := 0; b.Loop(); i++ {
		p.Intern(fmt.Appendf(nil, "data-%d", i))
	}
}

// BenchmarkGet measures performance of retrieving data.
// Target: < 50ns per operation.
func BenchmarkGet(b *testing.B) {
	p := New(1024)
	h := p.Intern([]byte("benchmark-data"))

	for b.Loop() {
		_, _ = p.Get(h)
	}
}

// BenchmarkRelease measures performance of releasing handles.
// Target: < 100ns per operation.
func BenchmarkRelease(b *testing.B) {
	p := New(1024 * 1024 * 100) // 100MB
	handles := make([]Handle, b.N)

	// Pre-allocate unique handles
	for i := range b.N {
		handles[i] = p.Intern(fmt.Appendf(nil, "data-%d", i))
	}

	b.ResetTimer()
	for i := range b.N {
		_ = p.Release(handles[i])
	}
}

// BenchmarkLength measures performance of getting data length.
func BenchmarkLength(b *testing.B) {
	p := New(1024)
	h := p.Intern([]byte("benchmark-data"))

	for b.Loop() {
		_, _ = p.Length(h)
	}
}

// BenchmarkMetrics measures performance of getting metrics.
func BenchmarkMetrics(b *testing.B) {
	p := New(1024)
	// Add some entries
	for i := range 100 {
		p.Intern(fmt.Appendf(nil, "data-%d", i))
	}

	for b.Loop() {
		_ = p.Metrics()
	}
}

// BenchmarkCompact measures performance of compaction.
func BenchmarkCompact(b *testing.B) {

	for b.Loop() {
		p := New(1024 * 1024)
		// Create entries
		handles := make([]Handle, 1000)
		for j := range 1000 {
			handles[j] = p.Intern(fmt.Appendf(nil, "data-%d", j))
		}
		// Release half
		for j := range 500 {
			_ = p.Release(handles[j])
		}

		b.StartTimer()
		p.Compact()
		b.StopTimer()
	}
}

// BenchmarkConcurrentIntern measures performance under concurrent load.
func BenchmarkConcurrentIntern(b *testing.B) {
	p := New(1024 * 1024 * 100)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			p.Intern(fmt.Appendf(nil, "data-%d", i))
			i++
		}
	})
}

// BenchmarkConcurrentGet measures Get performance under concurrent load.
func BenchmarkConcurrentGet(b *testing.B) {
	p := New(1024)
	h := p.Intern([]byte("benchmark-data"))

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = p.Get(h)
		}
	})
}

// BenchmarkDeduplication measures deduplication hit rate performance.
func BenchmarkDeduplication(b *testing.B) {
	p := New(1024 * 1024)

	// Pre-populate with some entries
	for i := range 100 {
		p.Intern(fmt.Appendf(nil, "data-%d", i))
	}

	for i := 0; b.Loop(); i++ {
		// 50% hit rate
		p.Intern(fmt.Appendf(nil, "data-%d", i%100))
	}
}
