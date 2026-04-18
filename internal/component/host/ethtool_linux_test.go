//go:build linux

package host

import (
	"testing"
	"unsafe"
)

// VALIDATES: ifreqEthtool struct layout matches the kernel's
// `struct ifreq` on 64-bit Linux (amd64/arm64/other lp64). The
// kernel reads 40 bytes at the ETHTOOL ioctl; any smaller struct
// causes it to read into adjacent stack memory (undefined behavior).
// PREVENTS: a future refactor shrinking the padding or dropping
// the uintptr alignment anchor without noticing that the kernel
// ABI broke.
func TestIfreqEthtoolSize(t *testing.T) {
	const want = 40 // kernel struct ifreq on 64-bit Linux: IFNAMSIZ(16) + ifru union(24)
	if got := unsafe.Sizeof(ifreqEthtool{}); got != want {
		t.Fatalf("unsafe.Sizeof(ifreqEthtool{}) = %d, want %d — SIOCETHTOOL copy_from_user reads past struct end", got, want)
	}
	// Also guard the offset of `data` — the kernel reads a pointer at
	// offset 16 of struct ifreq (first byte of the union).
	var ifr ifreqEthtool
	offset := unsafe.Offsetof(ifr.data)
	if offset != 16 {
		t.Fatalf("unsafe.Offsetof(ifr.data) = %d, want 16 — kernel reads pointer at offset 16", offset)
	}
}
