package attrpool

import (
	"fmt"
	"testing"
)

func benchIntern(b *testing.B, p *Pool, data []byte) Handle {
	b.Helper()
	h, err := p.Intern(data)
	if err != nil {
		b.Fatal(err)
	}
	return h
}

// BenchmarkInternExisting measures performance of interning existing data.
// Target: < 100ns per operation.
func BenchmarkInternExisting(b *testing.B) {
	p := New(1024 * 1024)
	benchIntern(b, p, []byte("benchmark-data"))

	for b.Loop() {
		benchIntern(b, p, []byte("benchmark-data"))
	}
}

// BenchmarkInternNew measures performance of interning new unique data.
// Target: < 500ns per operation.
func BenchmarkInternNew(b *testing.B) {
	p := New(1024 * 1024 * 100) // 100MB to avoid reallocation

	for i := 0; b.Loop(); i++ {
		benchIntern(b, p, fmt.Appendf(nil, "data-%d", i))
	}
}

// BenchmarkGet measures performance of retrieving data.
// Target: < 50ns per operation.
func BenchmarkGet(b *testing.B) {
	p := New(1024)
	h := benchIntern(b, p, []byte("benchmark-data"))

	for b.Loop() {
		if _, err := p.Get(h); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRelease measures performance of releasing handles.
// Target: < 100ns per operation.
func BenchmarkRelease(b *testing.B) {
	p := New(1024 * 1024 * 100) // 100MB
	handles := make([]Handle, b.N)

	// Pre-allocate unique handles
	for i := range b.N {
		handles[i] = benchIntern(b, p, fmt.Appendf(nil, "data-%d", i))
	}

	b.ResetTimer()
	for i := range b.N {
		_ = p.Release(handles[i])
	}
}

// BenchmarkLength measures performance of getting data length.
func BenchmarkLength(b *testing.B) {
	p := New(1024)
	h := benchIntern(b, p, []byte("benchmark-data"))

	for b.Loop() {
		if _, err := p.Length(h); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMetrics measures performance of getting metrics.
func BenchmarkMetrics(b *testing.B) {
	p := New(1024)
	// Add some entries
	for i := range 100 {
		benchIntern(b, p, fmt.Appendf(nil, "data-%d", i))
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
			handles[j] = benchIntern(b, p, fmt.Appendf(nil, "data-%d", j))
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
			benchIntern(b, p, fmt.Appendf(nil, "data-%d", i))
			i++
		}
	})
}

// BenchmarkConcurrentGet measures Get performance under concurrent load.
func BenchmarkConcurrentGet(b *testing.B) {
	p := New(1024)
	h := benchIntern(b, p, []byte("benchmark-data"))

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := p.Get(h); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkDeduplication measures deduplication hit rate performance.
func BenchmarkDeduplication(b *testing.B) {
	p := New(1024 * 1024)

	// Pre-populate with some entries
	for i := range 100 {
		benchIntern(b, p, fmt.Appendf(nil, "data-%d", i))
	}

	for i := 0; b.Loop(); i++ {
		// 50% hit rate
		benchIntern(b, p, fmt.Appendf(nil, "data-%d", i%100))
	}
}
