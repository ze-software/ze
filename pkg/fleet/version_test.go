package fleet

import (
	"testing"
)

func TestVersionHash(t *testing.T) {
	// VALIDATES: SHA-256 truncated hash is deterministic (16 hex chars).
	// PREVENTS: Non-deterministic hashing producing different results for same input.
	hash := VersionHash([]byte("bgp { peer 10.0.0.1 { } }"))
	if len(hash) != 16 {
		t.Fatalf("expected 16 hex chars, got %d: %q", len(hash), hash)
	}
	// Run again -- must be identical.
	hash2 := VersionHash([]byte("bgp { peer 10.0.0.1 { } }"))
	if hash != hash2 {
		t.Fatalf("not deterministic: %q != %q", hash, hash2)
	}
}

func TestVersionHashSameContent(t *testing.T) {
	// VALIDATES: Identical content produces identical hash.
	// PREVENTS: Hash including timestamps or random salt.
	content := []byte("plugin { hub { server local { host 127.0.0.1; port 1790; } } }")
	h1 := VersionHash(content)
	h2 := VersionHash(content)
	if h1 != h2 {
		t.Fatalf("same content, different hash: %q != %q", h1, h2)
	}
}

func TestVersionHashDifferentContent(t *testing.T) {
	// VALIDATES: Different content produces different hash.
	// PREVENTS: Hash function ignoring input.
	h1 := VersionHash([]byte("config version 1"))
	h2 := VersionHash([]byte("config version 2"))
	if h1 == h2 {
		t.Fatalf("different content, same hash: %q", h1)
	}
}

func TestVersionHashEmpty(t *testing.T) {
	// VALIDATES: Empty input produces a valid 16-char hash (not panic or empty string).
	// PREVENTS: Nil/empty slice causing crash.
	hash := VersionHash(nil)
	if len(hash) != 16 {
		t.Fatalf("expected 16 hex chars for nil input, got %d: %q", len(hash), hash)
	}
	hash2 := VersionHash([]byte{})
	if len(hash2) != 16 {
		t.Fatalf("expected 16 hex chars for empty input, got %d: %q", len(hash2), hash2)
	}
}
