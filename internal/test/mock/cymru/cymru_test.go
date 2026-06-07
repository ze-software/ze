// Design: docs/architecture/testing/ci-format.md -- cymru mock signature test

package cymru

import "testing"

func TestRunSignature(t *testing.T) {
	t.Parallel()
	var fn func([]string) int = Run
	if fn == nil {
		t.Fatal("Run is nil")
	}
}
