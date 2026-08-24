// Design: docs/architecture/hub-architecture.md -- SIGHUP config reload orchestration
// Related: main_reload.go -- the reload that rebuilds and swaps the AAA bundle

package hub

import (
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const reloadAAABackendName = "reload-fake-remote"

// errReloadAAAListenerRefused is the listener failure this file uses to reject a
// reload AFTER the AAA rebuild has run, which is the only window in which a
// candidate bundle can be abandoned.
var errReloadAAAListenerRefused = errors.New("listener refused")

// errReloadAAABuildRefused is the backend build failure that drives the
// reload's fail-closed branch.
var errReloadAAABuildRefused = errors.New("backend refused to build")

// reloadAAAChain is one built generation of the fake remote backend: the shared
// secret it was CONSTRUCTED with, and whether its Close has run.
//
// It stands in for RADIUS and TACACS+ because aaa.Default's backends are chosen
// at compile time and a test cannot add one to it. What it reproduces is the
// property those two share and the local backend does not: the secret is read
// once, at Build, and no later call re-reads it. That is the whole reason a
// rotated secret needs a REBUILT bundle rather than a live lookup.
type reloadAAAChain struct {
	secret string
	closed atomic.Bool
}

func (c *reloadAAAChain) Authenticate(request aaa.AuthRequest) (aaa.AuthResult, error) {
	if c.closed.Load() || request.Password != c.secret {
		return aaa.AuthResult{}, aaa.ErrAuthRejected
	}
	return aaa.AuthResult{
		Authenticated: true,
		Source:        reloadAAABackendName,
		Profiles:      []string{"admin"},
	}, nil
}

// reloadAAAChains records every chain the registry built, in build order.
type reloadAAAChains struct {
	mu     sync.Mutex
	chains []*reloadAAAChain
}

func (r *reloadAAAChains) add(c *reloadAAAChain) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chains = append(r.chains, c)
}

func (r *reloadAAAChains) all() []*reloadAAAChain {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*reloadAAAChain(nil), r.chains...)
}

type reloadAAABackend struct {
	chains *reloadAAAChains
	// failFrom makes Build fail from the Nth call onwards, one-based, so a
	// test can let boot succeed and make the RELOAD's build fail. Zero never
	// fails.
	failFrom int
	calls    *int
}

func (reloadAAABackend) Name() string  { return reloadAAABackendName }
func (reloadAAABackend) Priority() int { return 10 }

func (b reloadAAABackend) Build(params aaa.BuildParams) (aaa.Contribution, error) {
	if b.calls != nil {
		*b.calls++
		if b.failFrom > 0 && *b.calls >= b.failFrom {
			return aaa.Contribution{}, errReloadAAABuildRefused
		}
	}
	chain := &reloadAAAChain{secret: reloadAAASecret(params.ConfigTree)}
	b.chains.add(chain)
	return aaa.Contribution{
		Authenticator: chain,
		Close: func() error {
			chain.closed.Store(true)
			return nil
		},
	}, nil
}

// reloadAAASecret reads the shared secret out of a config tree, the way the
// real remote backends read theirs (internal/component/radius/config.go).
func reloadAAASecret(tree *zeconfig.Tree) string {
	if tree == nil {
		return ""
	}
	system := tree.GetContainer("system")
	if system == nil {
		return ""
	}
	authentication := system.GetContainer("authentication")
	if authentication == nil {
		return ""
	}
	remote := authentication.GetContainer("radius")
	if remote == nil {
		return ""
	}
	secret, _ := remote.Get("secret")
	return secret
}

func reloadAAATree(secret string) *zeconfig.Tree {
	remote := zeconfig.NewTree()
	remote.Set("secret", secret)
	authentication := zeconfig.NewTree()
	authentication.SetContainer("radius", remote)
	system := zeconfig.NewTree()
	system.SetContainer("authentication", authentication)
	tree := zeconfig.NewTree()
	tree.SetContainer("system", system)
	return tree
}

// installReloadAAARegistry replaces the compiled-in AAA registry with one that
// holds only the fake remote backend, and restores it when the test ends.
// aaa.Default freezes on its first Build, so a test that registered onto it
// would either be refused or freeze it for every later test in this binary.
func installReloadAAARegistry(t *testing.T, chains *reloadAAAChains) {
	t.Helper()
	installReloadAAARegistryFailingFrom(t, chains, 0)
}

// installReloadAAARegistryFailingFrom is installReloadAAARegistry with the
// backend's Nth build onwards refused.
func installReloadAAARegistryFailingFrom(t *testing.T, chains *reloadAAAChains, failFrom int) {
	t.Helper()
	previous := aaa.Default
	t.Cleanup(func() { aaa.Default = previous })
	aaa.Default = aaa.NewBackendRegistryForTest()
	calls := 0
	require.NoError(t, aaa.Default.Register(reloadAAABackend{chains: chains, failFrom: failFrom, calls: &calls}))
}

// VALIDATES: a reload rebuilds the AAA bundle from the reloaded tree and swaps
// it in, so a rotated remote shared secret authenticates on the running daemon
// and the retired one stops, with no restart.
// PREVENTS: swapAAABundle keeping its two startup callers, which left every
// RADIUS and TACACS+ server change (address, shared secret, timeout, a backend
// added or removed) accepted into the configuration and applied to nothing.
func TestDoReloadRebuildsAAABundleFromReloadedConfig(t *testing.T) {
	resetAAABundleForTest(t)
	chains := &reloadAAAChains{}
	installReloadAAARegistry(t, chains)

	log := slogutil.Logger("hub.aaa.test")
	bootBundle, err := aaa.Default.Build(aaa.BuildParams{ConfigTree: reloadAAATree("boot-secret"), Logger: log})
	require.NoError(t, err)
	swapAAABundle(bootBundle, log)
	t.Cleanup(func() { closeAAABundle(log) })

	live := liveAAABundleAuthenticator{}
	booted, err := live.Authenticate(aaa.AuthRequest{Username: "ops", Password: "boot-secret"})
	require.NoError(t, err)
	require.True(t, booted.Authenticated, "the boot secret must authenticate before the reload")

	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")
	reactor := &reloadTestReactor{tree: map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}, reloadAAATree("rotated-secret"), nil
	}
	require.NoError(t, doReload(server, nil, nil, store, configPath, load, nil))

	rotated, err := live.Authenticate(aaa.AuthRequest{Username: "ops", Password: "rotated-secret"})
	require.NoError(t, err)
	require.True(t, rotated.Authenticated, "the reloaded shared secret must authenticate without a daemon restart")

	retired, err := live.Authenticate(aaa.AuthRequest{Username: "ops", Password: "boot-secret"})
	require.Error(t, err)
	require.False(t, retired.Authenticated, "the retired shared secret must stop authenticating")

	built := chains.all()
	require.Len(t, built, 2, "the reload must build exactly one replacement chain")
	require.True(t, built[0].closed.Load(), "the retired chain must be closed so its socket and worker drain")
	require.False(t, built[1].closed.Load(), "the installed chain must stay open")
}

// VALIDATES: a reload that fails after the AAA rebuild closes the chain it
// built and leaves the running one installed.
// PREVENTS: a rejected commit leaking the candidate's RADIUS socket and TACACS+
// accounting worker, which Build opens before the reload can still fail.
func TestDoReloadClosesTheAAABundleItAbandons(t *testing.T) {
	resetAAABundleForTest(t)
	chains := &reloadAAAChains{}
	installReloadAAARegistry(t, chains)

	log := slogutil.Logger("hub.aaa.test")
	bootBundle, err := aaa.Default.Build(aaa.BuildParams{ConfigTree: reloadAAATree("boot-secret"), Logger: log})
	require.NoError(t, err)
	swapAAABundle(bootBundle, log)
	t.Cleanup(func() { closeAAABundle(log) })

	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")
	reactor := &reloadTestReactor{tree: map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	lm := newListenerMigrator()
	lm.web = &failingReconfigurable{addrs: []string{"127.0.0.1:3443"}, err: errReloadAAAListenerRefused}
	load := func() (map[string]any, *zeconfig.Tree, error) {
		tree := reloadAAATree("rotated-secret")
		tree.SetContainer("environment", reloadWebTree("127.0.0.1", "3444").GetContainer("environment"))
		return map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}, tree, nil
	}
	require.Error(t, doReload(server, nil, nil, store, configPath, load, lm))

	built := chains.all()
	require.Len(t, built, 2, "the reload must have built its candidate chain before failing")
	require.True(t, built[1].closed.Load(), "the abandoned candidate chain must be closed")

	live := liveAAABundleAuthenticator{}
	kept, err := live.Authenticate(aaa.AuthRequest{Username: "ops", Password: "boot-secret"})
	require.NoError(t, err)
	require.True(t, kept.Authenticated, "a rejected reload must leave the running chain installed")
}

// VALIDATES: a reload whose AAA chain cannot be built is REFUSED, and the
// running chain stays installed.
// PREVENTS: the permissive reading of a build failure. Treating it as "no AAA
// this time" would either leave the operator's commit reporting success over a
// chain that does not match the configuration, or install nothing and lock
// every remote account out with no error on the commit.
func TestDoReloadRefusesWhenTheAAABundleCannotBeBuilt(t *testing.T) {
	resetAAABundleForTest(t)
	chains := &reloadAAAChains{}
	// The first build is boot's and succeeds; the reload's is refused.
	installReloadAAARegistryFailingFrom(t, chains, 2)

	log := slogutil.Logger("hub.aaa.test")
	bootBundle, err := aaa.Default.Build(aaa.BuildParams{ConfigTree: reloadAAATree("boot-secret"), Logger: log})
	require.NoError(t, err)
	swapAAABundle(bootBundle, log)
	t.Cleanup(func() { closeAAABundle(log) })

	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")
	reactor := &reloadTestReactor{tree: map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}, reloadAAATree("rotated-secret"), nil
	}
	reloadErr := doReload(server, nil, nil, store, configPath, load, nil)
	require.Error(t, reloadErr, "a reload whose AAA chain cannot be built must be refused")
	require.ErrorIs(t, reloadErr, errReloadAAABuildRefused)

	live := liveAAABundleAuthenticator{}
	kept, err := live.Authenticate(aaa.AuthRequest{Username: "ops", Password: "boot-secret"})
	require.NoError(t, err)
	require.True(t, kept.Authenticated, "the running chain must survive a refused reload")
	require.Len(t, chains.all(), 1, "the refused build must contribute no chain")
	require.False(t, chains.all()[0].closed.Load(), "the running chain must not be closed by a refused reload")
}

// buildReloadAAABundle builds one bundle from a private registry and returns it
// beside the chain the backend constructed for it, whose closed flag reports
// whether Close has run.
func buildReloadAAABundle(t *testing.T, secret string) (*aaa.Bundle, *reloadAAAChain) {
	t.Helper()
	chains := &reloadAAAChains{}
	registry := aaa.NewBackendRegistryForTest()
	require.NoError(t, registry.Register(reloadAAABackend{chains: chains}))
	bundle, err := registry.Build(aaa.BuildParams{ConfigTree: reloadAAATree(secret)})
	require.NoError(t, err)
	built := chains.all()
	require.Len(t, built, 1)
	return bundle, built[0]
}

// VALIDATES: when two reloads reach the acceptance tail in the opposite order to
// the one they read their configurations in, the chain built from the
// configuration read LAST is the one installed, and the one left open.
// PREVENTS: a superseded chain winning. The plugin transaction lock releases
// inside Server.reloadConfig, so nothing holds two reloads apart over the tail:
// the interleave build C1, build C2, install C2, install C1 left the daemon
// authenticating against the retired secret while the install CLOSED C2, the
// chain that matched the accepted configuration.
func TestAcceptReloadedAAAInstallsTheConfigurationReadLast(t *testing.T) {
	resetAAABundleForTest(t)

	bootBundle, bootChain := buildReloadAAABundle(t, "boot-secret")
	swapAAABundle(bootBundle, nil)
	t.Cleanup(func() { closeAAABundle(nil) })

	firstOrder := nextAAAConfigReadOrder()
	secondOrder := nextAAAConfigReadOrder()
	firstBundle, firstChain := buildReloadAAABundle(t, "first-secret")
	secondBundle, secondChain := buildReloadAAABundle(t, "second-secret")

	retired, installed := acceptReloadedAAA(nil, secondBundle, secondOrder)
	require.True(t, installed)
	closeRetiredAAABundle(retired, nil)
	require.True(t, bootChain.closed.Load(), "an accepted chain retires the running one")

	retired, installed = acceptReloadedAAA(nil, firstBundle, firstOrder)
	assert.False(t, installed, "a chain built from a superseded configuration must not be installed")
	assert.Nil(t, retired, "a refused acceptance retires nothing")
	// What runReloadContext's deferred cleanup does for a candidate it did not
	// install: the candidate still owns a socket and a started worker.
	require.NoError(t, firstBundle.Close())
	assert.True(t, firstChain.closed.Load())

	assert.Same(t, secondBundle, aaaBundle.Load(),
		"the configuration read LAST must be the one the daemon authenticates against")
	assert.False(t, secondChain.closed.Load(), "the installed chain must stay open")
}

// VALIDATES: a reload reaching the acceptance tail after shutdown installs
// nothing and leaves the slot empty.
// PREVENTS: a swap landing after closeAAABundle, which reinstalls a live chain
// nothing is left to close and leaves its TACACS+ accounting worker running past
// exit.
func TestAcceptReloadedAAARefusesAfterShutdown(t *testing.T) {
	resetAAABundleForTest(t)

	bootBundle, bootChain := buildReloadAAABundle(t, "boot-secret")
	swapAAABundle(bootBundle, nil)
	closeAAABundle(nil)
	require.True(t, bootChain.closed.Load())

	order := nextAAAConfigReadOrder()
	candidate, candidateChain := buildReloadAAABundle(t, "late-secret")
	retired, installed := acceptReloadedAAA(nil, candidate, order)

	assert.False(t, installed, "a reload landing after shutdown must install nothing")
	assert.Nil(t, retired)
	assert.Nil(t, aaaBundle.Load(), "shutdown must leave the slot empty")
	require.NoError(t, candidate.Close())
	assert.True(t, candidateChain.closed.Load())
}

// VALIDATES: concurrent acceptances leave exactly one chain open, and it is the
// installed one.
// PREVENTS: the read-decide-store sequence running unserialized. Two acceptances
// interleaving there install two chains and close neither, or close the chain
// the other has just installed.
func TestAcceptReloadedAAALeavesOneChainOpenUnderConcurrentReloads(t *testing.T) {
	resetAAABundleForTest(t)

	bootBundle, bootChain := buildReloadAAABundle(t, "boot-secret")
	swapAAABundle(bootBundle, nil)
	t.Cleanup(func() { closeAAABundle(nil) })

	const reloads = 8
	bundles := make([]*aaa.Bundle, reloads)
	built := make([]*reloadAAAChain, reloads)
	for i := range bundles {
		bundles[i], built[i] = buildReloadAAABundle(t, "secret-"+strconv.Itoa(i))
	}

	var wg sync.WaitGroup
	for i := range bundles {
		wg.Add(1)
		go func(bundle *aaa.Bundle) {
			defer wg.Done()
			order := nextAAAConfigReadOrder()
			retired, installed := acceptReloadedAAA(nil, bundle, order)
			if !installed {
				_ = bundle.Close()
				return
			}
			closeRetiredAAABundle(retired, nil)
		}(bundles[i])
	}
	wg.Wait()

	live := aaaBundle.Load()
	require.NotNil(t, live)
	assert.True(t, bootChain.closed.Load(), "the boot chain must be retired")
	open := 0
	for i, chain := range built {
		if chain.closed.Load() {
			continue
		}
		open++
		assert.Same(t, live, bundles[i], "the only open chain must be the installed one")
	}
	assert.Equal(t, 1, open, "exactly one chain may be left open")
}

// reloadAAAMap lowers one AAA configuration the way a reload does, plus the BGP
// root the plugin server diffs against.
func reloadAAAMap(secret string) map[string]any {
	lowered := reloadAAATree(secret).ToPluginMap()
	lowered["bgp"] = map[string]any{"router-id": "1.1.1.1"}
	return lowered
}

// VALIDATES: a reload that changes nothing under `system authentication` leaves
// the running chain in place, and one that rotates the shared secret rebuilds.
// PREVENTS: an unrelated `ze config commit` bouncing a live chain. The rebuild
// closes the RADIUS socket and drains the TACACS+ accounting worker, so a
// rebuild on every reload retires a working chain for an edit it never read.
func TestDoReloadRebuildsTheAAABundleOnlyWhenAuthenticationChanges(t *testing.T) {
	resetAAABundleForTest(t)
	chains := &reloadAAAChains{}
	installReloadAAARegistry(t, chains)

	log := slogutil.Logger("hub.aaa.test")
	bootBundle, err := aaa.Default.Build(aaa.BuildParams{ConfigTree: reloadAAATree("boot-secret"), Logger: log})
	require.NoError(t, err)
	swapAAABundle(bootBundle, log)
	t.Cleanup(func() { closeAAABundle(log) })

	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")
	reactor := &reloadTestReactor{tree: reloadAAAMap("boot-secret")}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	// The provider carries the running configuration, which is what the reload
	// compares its own against.
	cp := zeconfig.NewProvider()
	for root, subtree := range reloadAAAMap("boot-secret") {
		if sub, ok := subtree.(map[string]any); ok {
			cp.SetRoot(root, sub)
		}
	}

	unchanged := func() (map[string]any, *zeconfig.Tree, error) {
		return reloadAAAMap("boot-secret"), reloadAAATree("boot-secret"), nil
	}
	require.NoError(t, doReload(server, nil, cp, store, configPath, unchanged, nil))
	require.Len(t, chains.all(), 1, "a reload that changes no authentication config must build no chain")
	require.False(t, chains.all()[0].closed.Load(), "the running chain must stay open")
	require.Same(t, bootBundle, aaaBundle.Load(), "the running bundle must stay installed")

	rotated := func() (map[string]any, *zeconfig.Tree, error) {
		return reloadAAAMap("rotated-secret"), reloadAAATree("rotated-secret"), nil
	}
	require.NoError(t, doReload(server, nil, cp, store, configPath, rotated, nil))
	built := chains.all()
	require.Len(t, built, 2, "a rotated shared secret must rebuild the chain")
	assert.True(t, built[0].closed.Load(), "the retired chain must be closed")
	assert.False(t, built[1].closed.Load(), "the installed chain must stay open")
}
