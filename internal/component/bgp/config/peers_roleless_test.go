// Design: docs/guide/configuration.md -- filter chain references
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFilterInstanceNameStripsEveryPrefixForm checks the four chain-ref forms
// resolve to one instance name.
//
// VALIDATES: a chain ref finds its policy registry entry whichever form the
// operator wrote and whichever form canonicalization left behind.
// PREVENTS: a ref resolved in the plain form and missed in the canonical one,
// or the reverse.
func TestFilterInstanceNameStripsEveryPrefixForm(t *testing.T) {
	for _, ref := range []string{
		"NO-TRANSIT",
		"reject-asn:NO-TRANSIT",
		"bgp-filter-path-asn:NO-TRANSIT",
	} {
		assert.Equal(t, "NO-TRANSIT", filterInstanceName(ref))
	}
}
