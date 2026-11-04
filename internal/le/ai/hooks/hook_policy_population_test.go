package aihooks

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProductionPolicyChangeStalesHookProof exercises the actual source digest,
// not the intentionally stale stored seal. specproof's classification reads
// tier.DisableableNonProdPrefixes, so changing or losing that authority must
// refuse comparison with the intact proof even when hookruntime is untouched.
func TestProductionPolicyChangeStalesHookProof(t *testing.T) {
	root := copyHookSources(t)
	const relative = "internal/le/arch/tier/gates.go"
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
	changed := append(body, '\n')
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := hookSourceContentDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if after == before {
		t.Error("production classification authority changed without invalidating the native hook proof")
	}
	hookAuthorityRefuse(t, root, before)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if digest, err := hookSourceContentDigest(root); err == nil && digest == before {
		t.Error("missing production classification authority retained its original proof digest")
	}
	hookAuthorityRefuse(t, root, before)
}
