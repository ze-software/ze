// Design: docs/architecture/pki/tls-listeners.md -- each looking-glass entry chooses HTTPS or HTTP

package hub

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/selfcert"
)

// lgEntriesTree builds a config whose looking-glass block holds the given
// server entries, keyed by entry name, and the given block leaves.
func lgEntriesTree(block, servers map[string]any) *zeconfig.Tree {
	lg := map[string]any{"server": servers}
	maps.Copy(lg, block)
	return treeFromMap(map[string]any{"environment": map[string]any{"looking-glass": lg}})
}

// lgListenerTLSByAddr indexes listeners by address. The list order of a tree
// built from a map is not fixed, so the tests compare by address.
func lgListenerTLSByAddr(listeners []lgListener) map[string]zeconfig.LGServerTLS {
	out := make(map[string]zeconfig.LGServerTLS, len(listeners))
	for _, listener := range listeners {
		out[listener.addr] = listener.tls
	}
	return out
}

func TestPlanLGTransports(t *testing.T) {
	// VALIDATES: the one rule that gives each looking-glass entry its protocol,
	// for every tls state against every certificate source: `tls false` is
	// HTTP; any other entry is HTTPS when a named certificate or the store
	// can supply one; with neither, an unset leaf is HTTP and reported, and a
	// written `tls true` is refused.
	// PREVENTS: one entry's leaf deciding another's protocol, a demanded TLS
	// approximated by HTTP, and a default that takes down a looking glass on a
	// daemon with no store.
	const (
		public = "0.0.0.0:80"
		secure = "0.0.0.0:8443"
	)
	off := lgListener{addr: public, tls: zeconfig.LGServerTLSOff}
	inherited := lgListener{addr: secure, tls: zeconfig.LGServerTLSInherited}
	required := lgListener{addr: secure, tls: zeconfig.LGServerTLSRequired}

	cases := []struct {
		name       string
		listeners  []lgListener
		certName   string
		hasStore   bool
		want       map[string]bool
		downgraded []string
	}{
		{"HTTP and demanded HTTPS on a named certificate, no store", []lgListener{off, required}, "lan", false,
			map[string]bool{public: false, secure: true}, nil},
		{"HTTP and demanded HTTPS on the store", []lgListener{off, required}, "", true,
			map[string]bool{public: false, secure: true}, nil},
		{"unset leaf with a store serves HTTPS", []lgListener{inherited}, "", true,
			map[string]bool{secure: true}, nil},
		{"unset leaf with a named certificate serves HTTPS", []lgListener{inherited}, "lan", false,
			map[string]bool{secure: true}, nil},
		{"unset leaf with nothing to present serves HTTP and is reported", []lgListener{off, inherited}, "", false,
			map[string]bool{public: false, secure: false}, []string{secure}},
		{"tls false stays HTTP whatever can be presented", []lgListener{off}, "lan", true,
			map[string]bool{public: false}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := planLGTransports(tc.listeners, tc.certName, tc.hasStore)
			require.NoError(t, err)
			require.Equal(t, tc.want, plan.tlsByAddr())
			require.Equal(t, tc.downgraded, plan.downgraded)
			require.Len(t, plan.bindings, len(tc.listeners), "every listener gets a binding, in order")
			for i, listener := range tc.listeners {
				require.Equal(t, listener.addr, plan.bindings[i].addr)
			}
		})
	}

	t.Run("a written tls true with nothing to present is refused", func(t *testing.T) {
		plan, err := planLGTransports([]lgListener{off, required}, "", false)
		require.Error(t, err)
		require.Empty(t, plan.bindings, "a refused config yields no plan")
		for _, want := range []string{secure, "looking glass", "tls true", "self-signed", "ze init", "tls false", "certificate"} {
			require.Contains(t, err.Error(), want, "the refusal says what, why and the fix")
		}
		require.NotContains(t, err.Error(), public, "the refusal names the entry that cannot be served, not its sibling")
	})
}

func TestResolveLGListeners(t *testing.T) {
	// VALIDATES: the listeners a config describes carry the tls leaf of their
	// own server entry, and an address with no entry (ze.looking-glass.listen)
	// takes the leaf's default.
	// PREVENTS: an environment variable or a block-wide setting choosing the
	// protocol of an entry, which the owner ruling removed.
	servers := map[string]any{
		"public": map[string]any{"ip": "127.0.0.1", "port": "3080", "tls": "false"},
		"secure": map[string]any{"ip": "127.0.0.1", "port": "3443"},
		"strict": map[string]any{"ip": "127.0.0.1", "port": "3444", "tls": "true"},
	}

	t.Run("an enabled block yields each entry with its own leaf", func(t *testing.T) {
		listeners, err := resolveLGListeners(lgEntriesTree(map[string]any{"enabled": "true"}, servers))
		require.NoError(t, err)
		require.Equal(t, map[string]zeconfig.LGServerTLS{
			"127.0.0.1:3080": zeconfig.LGServerTLSOff,
			"127.0.0.1:3443": zeconfig.LGServerTLSInherited,
			"127.0.0.1:3444": zeconfig.LGServerTLSRequired,
		}, lgListenerTLSByAddr(listeners))
	})

	t.Run("a block that does not say enabled starts nothing", func(t *testing.T) {
		listeners, err := resolveLGListeners(lgEntriesTree(nil, servers))
		require.NoError(t, err)
		require.Empty(t, listeners)

		listeners, err = resolveLGListeners(nil)
		require.NoError(t, err)
		require.Empty(t, listeners)
	})

	t.Run("ze.looking-glass.enabled starts the entries of a dormant block", func(t *testing.T) {
		require.NoError(t, env.Set("ze.looking-glass.enabled", "true"))
		t.Cleanup(func() { _ = env.Set("ze.looking-glass.enabled", "") })

		listeners, err := resolveLGListeners(lgEntriesTree(nil, servers))
		require.NoError(t, err)
		require.Equal(t, zeconfig.LGServerTLSOff, lgListenerTLSByAddr(listeners)["127.0.0.1:3080"],
			"an env-started looking glass still serves HTTP where the entry says tls false")

		listeners, err = resolveLGListeners(nil)
		require.NoError(t, err)
		require.Equal(t, []lgListener{{addr: lgDefaultListen, tls: zeconfig.LGServerTLSInherited}}, listeners,
			"with no block the default endpoint takes the default protocol")
	})

	t.Run("ze.looking-glass.listen addresses take the default protocol", func(t *testing.T) {
		require.NoError(t, env.Set("ze.looking-glass.listen", "127.0.0.1:3080"))
		t.Cleanup(func() { _ = env.Set("ze.looking-glass.listen", "") })

		listeners, err := resolveLGListeners(lgEntriesTree(map[string]any{"enabled": "true"}, servers))
		require.NoError(t, err)
		require.Equal(t, []lgListener{{addr: "127.0.0.1:3080", tls: zeconfig.LGServerTLSInherited}}, listeners,
			"an address from the environment has no server entry, so no tls leaf")
	})
}

func TestUpdateLGTransportAppliesThePlan(t *testing.T) {
	// VALIDATES: a reload hands the running looking glass the protocol of each
	// described listener and, only when one presents a certificate, the
	// certificate; and refuses, before installing anything, the config a start
	// refuses.
	// PREVENTS: a certificate installed for a looking glass that serves none, a
	// reload that fails over the self-signed pair of an entry that left tls
	// unset, and a `tls true` entry quietly served over HTTP.
	loadTwoCertStore(t)
	const other = "127.0.0.1:3080"

	t.Run("no name, no store, unset leaf: HTTP, with a warning", func(t *testing.T) {
		var logged strings.Builder
		fake := &fakeTLSUpdatable{}
		lm := &listenerMigrator{logger: slog.New(slog.NewTextHandler(&logged, nil))}
		lm.setLGTransport(fake)

		cfg := lgListenerConfig{listeners: []lgListener{{addr: fakeListenAddr, tls: zeconfig.LGServerTLSInherited}}}
		require.NoError(t, lm.checkLGTransport(cfg))
		require.NoError(t, lm.updateLGTransport(cfg))
		require.Zero(t, fake.calls, "a looking glass with no TLS listener is handed no certificate")
		require.Equal(t, map[string]bool{fakeListenAddr: false}, fake.requestedTLS)
		for _, want := range []string{"plain HTTP", fakeListenAddr, "ze init", "tls false"} {
			require.Contains(t, logged.String(), want, "the downgrade is reported with its fix")
		}
	})

	t.Run("no name, no store, written tls true: refused", func(t *testing.T) {
		fake := &fakeTLSUpdatable{}
		lm := &listenerMigrator{}
		lm.setLGTransport(fake)

		cfg := lgListenerConfig{listeners: []lgListener{{addr: fakeListenAddr, tls: zeconfig.LGServerTLSRequired}}}
		checkErr := lm.checkLGTransport(cfg)
		require.Error(t, checkErr, "the reload gate refuses before anything is applied")
		err := lm.updateLGTransport(cfg)
		require.Error(t, err)
		require.Equal(t, checkErr.Error(), err.Error())
		require.Contains(t, err.Error(), "ze init")
		require.Zero(t, fake.calls)
		require.Nil(t, fake.requestedTLS, "a refused config changes no listener")
	})

	t.Run("HTTP and HTTPS entries on a named certificate", func(t *testing.T) {
		fake := &fakeTLSUpdatable{}
		lm := &listenerMigrator{}
		lm.setLGTransport(fake)

		require.NoError(t, lm.updateLGTransport(lgListenerConfig{
			listeners: []lgListener{
				{addr: other, tls: zeconfig.LGServerTLSOff},
				{addr: fakeListenAddr, tls: zeconfig.LGServerTLSInherited},
			},
			certName: "lg-cert",
		}))
		require.Equal(t, 1, fake.calls)
		require.Equal(t, "lg leaf", installedLeafCN(t, fake))
		require.Equal(t, map[string]bool{other: false, fakeListenAddr: true}, fake.requestedTLS)
	})

	t.Run("every entry on HTTP installs no certificate", func(t *testing.T) {
		fake := &fakeTLSUpdatable{}
		lm := &listenerMigrator{}
		lm.setLGTransport(fake)

		require.NoError(t, lm.updateLGTransport(lgListenerConfig{
			listeners: []lgListener{{addr: fakeListenAddr, tls: zeconfig.LGServerTLSOff}},
			certName:  "typo-cert",
		}))
		require.Zero(t, fake.calls, "a name no listener presents is not resolved")
		require.Equal(t, map[string]bool{fakeListenAddr: false}, fake.requestedTLS)
	})

	t.Run("a config that describes no listener leaves an HTTP looking glass alone", func(t *testing.T) {
		fake := &fakeTLSUpdatable{listenerTLS: map[string]bool{fakeListenAddr: false}}
		lm := &listenerMigrator{}
		lm.setLGTransport(fake)

		require.NoError(t, lm.updateLGTransport(lgListenerConfig{certName: "lg-cert"}))
		require.Zero(t, fake.calls, "no running listener presents a certificate")
		require.Empty(t, fake.requestedTLS, "the running listeners keep their protocol")
	})
}

// bindRecordingLG stands for the running looking glass through a whole reload:
// it is the listener mover (Reconfigurable) and the protocol seam
// (lgTransportUpdatable) at once, as lg.LGServer is, and it keeps the part of
// that server's contract the reload order depends on. UpdateListenerTLS records
// the protocol of an address, bound or not. Reconfigure binds each address it
// adds with the protocol recorded for it at that moment, which boundTLS keeps,
// and an address it binds with no record goes to boundUnrecorded.
type bindRecordingLG struct {
	addrs           []string
	recorded        map[string]bool
	boundTLS        map[string]bool
	boundUnrecorded []string
}

func (f *bindRecordingLG) Addresses() []string { return slices.Clone(f.addrs) }

func (f *bindRecordingLG) Reconfigure(_ context.Context, newAddrs []string) error {
	for _, addr := range newAddrs {
		if slices.Contains(f.addrs, addr) {
			continue
		}
		serveTLS, recorded := f.recorded[addr]
		if !recorded {
			f.boundUnrecorded = append(f.boundUnrecorded, addr)
			continue
		}
		f.boundTLS[addr] = serveTLS
	}
	f.addrs = slices.Clone(newAddrs)
	return nil
}

func (f *bindRecordingLG) ListenerTLS() map[string]bool {
	out := make(map[string]bool, len(f.addrs))
	for _, addr := range f.addrs {
		out[addr] = f.recorded[addr]
	}
	return out
}

func (f *bindRecordingLG) UpdateListenerTLS(tlsByAddr map[string]bool) error {
	maps.Copy(f.recorded, tlsByAddr)
	return nil
}

func (f *bindRecordingLG) UpdateTLSCertificate([]byte, []byte) error { return nil }

func (f *bindRecordingLG) selfSignedSource() (certStore selfcert.CertStore, listenAddr string) {
	return nil, fakeListenAddr
}

func TestReloadBindsAnAddedHTTPEntryOnHTTP(t *testing.T) {
	// VALIDATES: a reload that adds a `tls false` server entry to a running
	// looking glass has the new listener bound on plain HTTP: the protocol the
	// reloaded config gives the address is recorded before the listeners move,
	// so the first connection the new socket accepts is answered in the clear.
	// PREVENTS: the reload order that moved the listeners first and named their
	// protocols after, which bound the new address with no protocol of its own.
	// The looking glass served TLS there until the reload named it HTTP, so the
	// new HTTP endpoint answered TLS for a moment.
	resetAAABundleForTest(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	const public = "127.0.0.1:3080"
	tree := map[string]any{"environment": map[string]any{"looking-glass": map[string]any{
		"enabled": "true",
		"server": map[string]any{
			"secure": map[string]any{"ip": "127.0.0.1", "port": "3443"},
			"public": map[string]any{"ip": "127.0.0.1", "port": "3080", "tls": "false"},
		},
	}}}
	srv, cp := reloadDriver(t, tree)

	running := &bindRecordingLG{
		addrs:    []string{fakeListenAddr},
		recorded: map[string]bool{fakeListenAddr: true},
		boundTLS: map[string]bool{},
	}
	lm := newListenerMigrator()
	lm.setLG(running)
	lm.setLGTransport(running)
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return tree, treeFromMap(tree), nil
	}

	require.NoError(t, runReload(srv, cp, load, lm))
	require.Empty(t, running.boundUnrecorded,
		"an address the reload adds must have its protocol recorded before it is bound")
	require.Equal(t, map[string]bool{public: false}, running.boundTLS,
		"the added `tls false` entry must be bound on plain HTTP")
	require.Equal(t, []string{public, fakeListenAddr}, slices.Sorted(slices.Values(running.addrs)),
		"the reload moves the looking glass onto both entries")
}
