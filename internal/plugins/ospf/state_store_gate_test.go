// Design: ai/rules/architecture.md -- runtime state persists in the managed
// zefs store, and only where the operator pinned one.
// Related: auth_keystore.go -- pinnedStateDir and the boot-count store opener.

package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: neither OSPF store opener resolves a config dir unless the operator
// pinned one with ze.config.dir, and both degrade rather than fail without it.
// PREVENTS: the contention behind the SIGBUS in plan/learned/1293. Unpinned,
// paths.DefaultConfigDir resolves the binary-relative etc/ze that EVERY `ze` on the
// host shares; the OSPF engine is its own process and zefs's lock is an in-process
// sync.RWMutex, so opening it put 64 functional-test daemons on one file.
// cmd/ze/hub/main.go applies this same gate to runtime-state persistence and spells
// out why -- these two callers were the ones bypassing it.
//
// The resolver is injected because under `go test` the real
// paths.DefaultConfigDir returns "" anyway (the test binary is in a build temp
// dir), so a gate that did nothing would be indistinguishable from one that works.
func TestOSPFStateDirRequiresAPinnedConfigDir(t *testing.T) {
	const shared = "/opt/ze/etc/ze" // what the unpinned resolver would hand back
	resolve := func() string { return shared }

	t.Setenv("ze.config.dir", "")
	env.ResetCache()
	if got := pinnedStateDir(resolve); got != "" {
		t.Errorf("unpinned: pinnedStateDir = %q, want empty. Returning the shared "+
			"binary-relative dir is what put every ze process on one database.zefs", got)
	}
	// Degradation, not failure: the boot count still advances this boot.
	if got := loadOSPFBootCount(nil); got == 0 {
		t.Error("unpinned: loadOSPFBootCount(nil) must fall back to the clock seed, got 0")
	}

	pinned := t.TempDir()
	t.Setenv("ze.config.dir", pinned)
	env.ResetCache()
	if got := pinnedStateDir(func() string { return pinned }); got != pinned {
		t.Errorf("pinned: pinnedStateDir = %q, want %q", got, pinned)
	}
}

// VALIDATES: the two store openers go through pinnedStateDir, not
// paths.DefaultConfigDir directly.
// PREVENTS: a future caller reintroducing the unpinned resolve. The gate is only
// worth anything if every opener honors it, and the injected-resolver test above
// cannot see which function a caller actually reaches for.
func TestOSPFStoreOpenersHonorTheGate(t *testing.T) {
	t.Setenv("ze.config.dir", "")
	env.ResetCache()
	if s := openBootCountStore(); s != nil {
		t.Error("unpinned: openBootCountStore opened a store")
	}
	if _, ok := openGRStore(); ok {
		t.Error("unpinned: openGRStore opened a store")
	}
}
