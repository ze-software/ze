// Design: docs/architecture/traffic/cos-plugin.md -- missing providers cannot accept their own schema leaves.

package cos

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestMissingResolverCorrectionRemovesProviderOwnedSetting proves the proposed
// correction remains possible when the absent plugin's schema is absent too.
func TestMissingResolverCorrectionRemovesProviderOwnedSetting(t *testing.T) {
	mu.Lock()
	held := resolver
	resolver = nil
	mu.Unlock()
	t.Cleanup(func() { RegisterResolver(held) })
	_, _, err := Resolve(nil, "residential", "", false)
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		t.Fatalf("missing resolver did not return an operator error: %v", err)
	}
	if refusal.Fix() != "remove class-of-service from this interface and its units" {
		t.Fatalf("correction still needs the absent provider's schema: %q", refusal.Fix())
	}
}
