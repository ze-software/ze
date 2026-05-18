package dns

import "testing"

func TestDNSCacheStats(t *testing.T) {
	c := newCache(10, 300)

	stats := c.Stats()
	if stats.Entries != 0 || stats.Hits != 0 || stats.Misses != 0 || stats.Evictions != 0 {
		t.Fatal("expected zero stats on new cache")
	}
	if stats.Capacity != 10 {
		t.Errorf("capacity = %d, want 10", stats.Capacity)
	}

	c.put("example.com", 1, []string{"1.2.3.4"}, 60)
	stats = c.Stats()
	if stats.Entries != 1 {
		t.Errorf("entries = %d after put, want 1", stats.Entries)
	}

	_, ok := c.get("example.com", 1)
	if !ok {
		t.Fatal("expected cache hit")
	}
	stats = c.Stats()
	if stats.Hits != 1 {
		t.Errorf("hits = %d after hit, want 1", stats.Hits)
	}

	_, ok = c.get("missing.com", 1)
	if ok {
		t.Fatal("expected cache miss")
	}
	stats = c.Stats()
	if stats.Misses != 1 {
		t.Errorf("misses = %d after miss, want 1", stats.Misses)
	}

	// Fill cache to trigger eviction.
	for i := range 11 {
		name := "evict" + string(rune('a'+i)) + ".com"
		c.put(name, 1, []string{"1.2.3.4"}, 60)
	}
	stats = c.Stats()
	if stats.Evictions == 0 {
		t.Error("expected evictions after filling cache beyond capacity")
	}
	if stats.Entries > 10 {
		t.Errorf("entries = %d, should not exceed capacity 10", stats.Entries)
	}
}
