//go:build !ze_bgp

// Design: docs/architecture/testing/interop.md -- compiled interop helper ownership.
// Related: helper_atomic_policy.go -- the decoder belongs to the BGP feature.
package bgp

import "github.com/ze-software/ze/internal/core/opererr"

func runAtomicPolicy(string) error {
	return opererr.New("The atomic-policy fixture has no BGP decoder.",
		"This binary was built without ze_bgp and cannot judge its UPDATE inputs.",
		atomicPolicyRetry)
}
