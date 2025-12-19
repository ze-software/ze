package pool

import (
	"fmt"
	"testing"
)

// BenchmarkInternExisting measures performance of interning existing data.
// Target: < 100ns per operation
func BenchmarkInternExisting(b *testing.B) {
	p := New(1024 * 1024)
	p.Intern([]byte("benchmark-data"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Intern([]byte("benchmark-data"))
	}
}

// BenchmarkInternNew measures performance of interning new unique data.
// Target: < 500ns per operation
func BenchmarkInternNew(b *testing.B) {
	p := New(1024 * 1024 * 100) // 100MB to avoid reallocation

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Intern([]byte(fmt.Sprintf("data-%d", i)))
	}
}

// BenchmarkGet measures performance of retrieving data.
// Target: < 50ns per operation
func BenchmarkGet(b *testing.B) {
	p := New(1024)
	h := p.Intern([]byte("benchmark-data"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Get(h)
	}
}

// BenchmarkRelease measures performance of releasing handles.
// Target: < 100ns per operation
func BenchmarkRelease(b *testing.B) {
	p := New(1024 * 1024 * 100) // 100MB
	handles := make([]Handle, b.N)

	// Pre-allocate unique handles
	for i := 0; i < b.N; i++ {
		handles[i] = p.Intern([]byte(fmt.Sprintf("data-%d", i)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Release(handles[i])
	}
}

// BenchmarkLength measures performance of getting data length.
func BenchmarkLength(b *testing.B) {
	p := New(1024)
	h := p.Intern([]byte("benchmark-data"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Length(h)
	}
}

// BenchmarkMetrics measures performance of getting metrics.
func BenchmarkMetrics(b *testing.B) {
	p := New(1024)
	// Add some entries
	for i := 0; i < 100; i++ {
		p.Intern([]byte(fmt.Sprintf("data-%d", i)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Metrics()
	}
}

// BenchmarkCompact measures performance of compaction.
func BenchmarkCompact(b *testing.B) {
	b.StopTimer()

	for i := 0; i < b.N; i++ {
		p := New(1024 * 1024)
		// Create entries
		handles := make([]Handle, 1000)
		for j := 0; j < 1000; j++ {
			handles[j] = p.Intern([]byte(fmt.Sprintf("data-%d", j)))
		}
		// Release half
		for j := 0; j < 500; j++ {
			p.Release(handles[j])
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
			p.Intern([]byte(fmt.Sprintf("data-%d", i)))
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
			_ = p.Get(h)
		}
	})
}

// BenchmarkDeduplication measures deduplication hit rate performance.
func BenchmarkDeduplication(b *testing.B) {
	p := New(1024 * 1024)

	// Pre-populate with some entries
	for i := 0; i < 100; i++ {
		p.Intern([]byte(fmt.Sprintf("data-%d", i)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 50% hit rate
		p.Intern([]byte(fmt.Sprintf("data-%d", i%100)))
	}
}
