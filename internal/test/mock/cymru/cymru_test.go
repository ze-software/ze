// Design: docs/architecture/testing/ci-format.md -- cymru mock signature test

package cymru

import "testing"

// Compile-time signature check: Run must be func([]string) int.
var _ func([]string) int = Run //nolint:staticcheck // intentional type annotation for signature verification

func TestRunRegistered(t *testing.T) {
	t.Parallel()
}
