package bgpconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/bgp/asn"

	// The reload closure parses the file, which builds the YANG schema. This
	// package's test binary registered no module of its own. ze-bgp-conf then
	// resolved to nothing, and every test that reached the parser failed on
	// the schema rather than on its subject. Measured on 2026-09-08: the
	// import takes this package from 124 failing tests to 48, and adds none.
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
)

// reloadCandidate is a config whose bgp block selects asdot+. The test writes
// it to disk, and the reload closure then reads it the way the daemon does.
const reloadCandidate = `bgp {
	router-id 1.2.3.4;
	as-notation asdot+;
	session { asn { local 65000; } }
	peer peer1 {
		connection { remote { ip 10.0.0.1; } local { ip auto; } }
		session { asn { remote 65001; } }
	}
}
`

// TestReloadVerifyLeavesTheRenderedNotation proves the reload coordinator's
// VERIFY phase changes nothing about what this process renders. It runs over
// the branch the daemon takes.
//
// The branch is the point. loadPeersFullOrTree (../reactor/reactor_api.go)
// re-reads and re-parses the file through createReloadFunc's closure when the
// reactor has a config path AND a reload function. It falls back to
// PeersFromTree when it has neither.
//
// A reactor built without them takes that fallback, which reaches neither
// ResolveBGPTree nor the closure. Those are the two places this write lived in
// rounds 1 and 2, so a test on the fallback cannot see the defect return. This
// test sets both.
//
// VALIDATES: nothing inside the reload closure records the notation.
// PREVENTS: the write returning to ResolveBGPTree or to createReloadFunc,
// where the coordinator's verify phase reaches it before any refusal can.
func TestReloadVerifyLeavesTheRenderedNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })
	configureNotation(t, asn.NotationPlain)

	configPath := filepath.Join(t.TempDir(), "ze.conf")
	if err := os.WriteFile(configPath, []byte(reloadCandidate), 0o600); err != nil {
		t.Fatalf("write candidate: %v", err)
	}

	r := reactor.New(&reactor.Config{Port: 0, Standalone: true})
	if err := r.Start(); err != nil {
		t.Fatalf("start reactor: %v", err)
	}
	defer r.Stop()

	// Both, or loadPeersFullOrTree takes the PeersFromTree fallback and this
	// test proves nothing about the file-reading branch.
	//
	// The wrapper is what says the branch was taken. PeersFromTree answers
	// (nil, nil) for the empty tree below. A condition that changed under this
	// test would then let verify succeed on the fallback, and the test would
	// stay green over nothing.
	reload := createReloadFunc(storage.NewFilesystem(), r)
	called := false
	r.SetConfigPath(configPath)
	r.SetReloadFunc(func(path string) ([]*reactor.PeerSettings, error) {
		called = true
		return reload(path)
	})

	api, ok := r.ReactorLifecycleAdapter().(interface {
		VerifyConfig(map[string]any) error
	})
	if !ok {
		t.Fatal("the reactor lifecycle adapter does not verify a config")
	}

	// The tree argument is ignored on this branch: the closure re-reads the
	// file. Passing an empty one proves the file is what was read.
	if err := api.VerifyConfig(map[string]any{}); err != nil {
		t.Fatalf("verifying the candidate failed for an unrelated reason: %v", err)
	}
	if !called {
		t.Fatal("the reload function was never called. Verify took the PeersFromTree fallback, so this test guards nothing")
	}

	// Nothing calls SetConfigTree, so the commit is refused. An operator whose
	// commit was refused must still read the running notation.
	if got := asn.Configured(); got != asn.NotationPlain {
		t.Errorf("the verify phase changed the rendered notation to %v, want %v unchanged", got, asn.NotationPlain)
	}
}
