//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- live scope admission.
// Related: kernel_scope_caps_linux_test.go -- malformed and partial receipt parsing.

package dataplane

// RFC naming: untagged -- it asserts only the kernel's scope capability receipt, an admission precondition no RFC 4301 row states; the packet tests prove each enforcement.

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestRFC4301KernelAdvertisesCompleteStateScope reads the running namespace's
// GETDEFAULT receipt. A stock kernel lacks the prerequisite; a Ze kernel with
// an incomplete receipt fails its advertised enforcement contract explicitly.
// This proves admission only. Packet tests separately prove each enforcement.
//
//ze:guest-root
func TestRFC4301KernelAdvertisesCompleteStateScope(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	caps, err := readXFRMZECapabilities()
	if errors.Is(err, errXFRMZECapabilitiesAbsent) {
		t.Skip("the running kernel has no Ze scope capability receipt")
	}
	if err != nil {
		t.Fatalf("read the actual kernel's scope capability receipt: %v", err)
	}
	if caps == nil {
		t.Fatal("GETDEFAULT returned no capability receipt without an error")
	}
	if caps.ABI != xfrmZEABI {
		t.Fatalf("kernel scope ABI = %#x, want %#x", caps.ABI, xfrmZEABI)
	}
	if caps.MaxSelectors != saStateScopeMaxSelectors {
		t.Fatalf("kernel scope selector limit = %d, want %d", caps.MaxSelectors, saStateScopeMaxSelectors)
	}
	const required = xfrmZECapStateScope | xfrmZECapICMPQuote |
		xfrmZECapFragmentContext | xfrmZECapFragmentGuard
	if !caps.Features.Has(required) {
		t.Fatalf("Ze kernel scope features = %#x, want all enforcement bits %#x; missing %#x; a partial receipt cannot authorize scoped SAs",
			caps.Features, required, required & ^caps.Features)
	}
	if caps.GuardRefs != 0 {
		t.Fatalf("fresh namespace owns %d fragment guard references, want zero", caps.GuardRefs)
	}
}
