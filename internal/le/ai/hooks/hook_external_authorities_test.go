package aihooks

import (
	"os"
	"path/filepath"
	"testing"
)

// TestChangedExternalAuthoritiesInvalidateHookProof covers the concrete reached
// proposal/approval helpers and the shared metadata grammar. The actual source
// digest must reject their mutation or loss without consulting the stale seal.
func TestChangedExternalAuthoritiesInvalidateHookProof(t *testing.T) {
	for _, relative := range []string{
		"internal/le/test/weakened/testweakened.go",
		"internal/le/test/weakened/ledger.go",
		"internal/le/spec/status/specstatus.go",
	} {
		t.Run(relative, func(t *testing.T) {
			root := copyHookSources(t)
			body, err := os.ReadFile(filepath.Join(hookCheckout(t), filepath.FromSlash(relative)))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := hookSourceContentDigest(root)
			if err != nil {
				t.Fatal(err)
			}
			hookAuthorityAccept(t, root, before)
			if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			after, err := hookSourceContentDigest(root)
			if err != nil {
				t.Fatal(err)
			}
			if after == before {
				t.Error("changed external policy authority did not invalidate the native hook proof")
			}
			hookAuthorityRefuse(t, root, before)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if digest, err := hookSourceContentDigest(root); err == nil && digest == before {
				t.Error("missing external policy authority retained its original proof digest")
			}
			hookAuthorityRefuse(t, root, before)
		})
	}
}
