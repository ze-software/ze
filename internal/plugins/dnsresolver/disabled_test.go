package dnsresolver

import (
	"bytes"
	"context"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// newTestTx builds the plugin's config transaction the way register.go does:
// sdk.ConfigTx over parseResolverSections and the installer, so a test drives
// the boot configure and the reload verify and apply the daemon drives.
func newTestTx(t *testing.T) (*resolverInstaller, *sdk.ConfigTx[resolverConfig]) {
	t.Helper()
	res := newTestResolver(t, defaultConfig(), newTestClock())
	log := slogutil.DiscardLogger()
	in := &resolverInstaller{srv: newResolverServer(log, res), res: res, log: log}
	t.Cleanup(in.srv.stop)
	return in, sdk.NewConfigTx(defaultConfig(), parseResolverSections, in.install)
}

// serviceSection wraps the JSON of the service root as one delivery.
func serviceSection(data string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootService, Data: data}}
}

// resolverJSON answers a service root whose dns-resolver has enabled set as
// given, one listener on 127.0.0.1 port listen, allow-from 127.0.0.0/8 and
// one forward server on 127.0.0.1 port forward.
func resolverJSON(enabled bool, listen, forward uint16) string {
	return `{"service":{"dns-resolver":{"enabled":"` + strconv.FormatBool(enabled) + `",` +
		`"listener":{"lan":{"ip":"127.0.0.1","port":"` + strconv.Itoa(int(listen)) + `"}},` +
		`"allow-from":["127.0.0.0/8"],` +
		`"forward":{"server":[{"ip":"127.0.0.1","port":"` + strconv.Itoa(int(forward)) + `"}]}}}}`
}

// bindBoth binds addr over TCP and UDP and releases both at once. It answers
// the first bind error, so a nil answer means nothing held the address on
// either transport.
func bindBoth(addr string) error {
	var lc net.ListenConfig
	ctx := context.Background()
	tcp, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	_ = tcp.Close() //nolint:errcheck // probe socket
	udp, err := lc.ListenPacket(ctx, "udp", addr)
	if err != nil {
		return err
	}
	_ = udp.Close() //nolint:errcheck // probe socket
	return nil
}

// TestDisabledBindsNoListener proves AC-29's listener half: a resolver that
// is absent or `enabled false` binds no socket. Method: each delivery goes
// through the real config transaction, at boot (lenient binds) and as a
// reload (strict binds); afterwards another socket binds the address the
// disabled config names, over TCP and UDP. A resolver that ignored `enabled`
// would hold that address and the bind would fail.
func TestDisabledBindsNoListener(t *testing.T) {
	port := freeLoopbackPort(t)
	addr := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), port).String()
	cases := []struct {
		name     string
		sections []sdk.ConfigSection
	}{
		{"no service section", nil},
		{"service without dns-resolver", serviceSection(`{"service":{}}`)},
		{"enabled false with a listener", serviceSection(resolverJSON(false, port, 53))},
	}
	for _, tc := range cases {
		t.Run(tc.name+" at boot", func(t *testing.T) {
			in, tx := newTestTx(t)
			if err := in.configureAtBoot(tx, tc.sections); err != nil {
				t.Fatalf("configure: %v", err)
			}
			if err := bindBoth(addr); err != nil {
				t.Fatalf("bind %s after a disabled boot: %v, want free (no listener bound)", addr, err)
			}
		})
		t.Run(tc.name+" on reload", func(t *testing.T) {
			_, tx := newTestTx(t)
			if err := tx.Verify(tc.sections); err != nil {
				t.Fatalf("verify: %v", err)
			}
			if err := tx.Apply(); err != nil {
				t.Fatalf("apply: %v", err)
			}
			if err := bindBoth(addr); err != nil {
				t.Fatalf("bind %s after a disabled reload: %v, want free (no listener bound)", addr, err)
			}
		})
	}
}

// TestDisableReleasesListener proves AC-29 across a reload: a running
// resolver taken to `enabled false` releases its socket. Method: an enabled
// config is configured at boot and answers a query on its listener, then a
// reload that changes only `enabled` to false goes through the transaction's
// verify and apply (resolverInstaller.install); another socket must then bind
// the listener's address over TCP and UDP.
func TestDisableReleasesListener(t *testing.T) {
	up := aUpstream(t, 0)
	upPort := up.upstream().Port
	port := freeLoopbackPort(t)
	addr := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), port).String()
	in, tx := newTestTx(t)
	if err := in.configureAtBoot(tx, serviceSection(resolverJSON(true, port, upPort))); err != nil {
		t.Fatalf("configure enabled: %v", err)
	}
	c := &dns.Client{Net: "udp", Timeout: 2 * time.Second}
	reply, _, err := c.Exchange(query("a.test", dns.TypeA), addr)
	if err != nil {
		t.Fatalf("query the enabled listener: %v", err)
	}
	if reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("enabled listener answered %s, want NOERROR", dns.RcodeToString[reply.Rcode])
	}
	if err := bindBoth(addr); err == nil {
		t.Fatalf("bind %s while enabled succeeded, want the address held by the listener", addr)
	}

	if err := tx.Verify(serviceSection(resolverJSON(false, port, upPort))); err != nil {
		t.Fatalf("verify enabled false: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply enabled false: %v", err)
	}
	if err := bindBoth(addr); err != nil {
		t.Fatalf("bind %s after enabled false: %v, want the listener released", addr, err)
	}
}

// TestResolverLeavesSystemNameServers proves AC-29's other half: the resolver
// never changes `system name-server` or the stub resolver's file. Those are
// owned by internal/component/config/system: PublishNameServers holds the
// running leaf-list and ProcessResolvers writes resolv.conf. Method: both are
// set in this process (a temporary resolv.conf), then the plugin's config
// transaction runs absent, enabled and disabled configs; the published name
// servers, the stub's servers and the file's bytes must all be unchanged.
// The dnsresolver package does not depend on config/system at all (`go list
// -deps`), so an external plugin process cannot reach them either; this test
// keeps an in-process write from arriving unseen.
func TestResolverLeavesSystemNameServers(t *testing.T) {
	stub := []string{"192.0.2.53"}
	previous, published := system.NameServers()
	system.PublishNameServers(stub)
	t.Cleanup(func() {
		if published {
			system.PublishNameServers(previous)
		}
	})
	resolvConf := filepath.Join(t.TempDir(), "resolv.conf")
	if err := system.ProcessResolvers().Configure(resolvConf, stub); err != nil {
		t.Fatalf("configure the stub: %v", err)
	}
	t.Cleanup(func() {
		if err := system.ProcessResolvers().Configure("", nil); err != nil {
			t.Errorf("reset the stub: %v", err)
		}
	})
	fileBefore, err := os.ReadFile(resolvConf)
	if err != nil {
		t.Fatalf("read resolv.conf: %v", err)
	}

	up := aUpstream(t, 0)
	port := freeLoopbackPort(t)
	in, tx := newTestTx(t)
	if err := in.configureAtBoot(tx, nil); err != nil {
		t.Fatalf("configure absent: %v", err)
	}
	for _, enabled := range []bool{true, false} {
		if err := tx.Verify(serviceSection(resolverJSON(enabled, port, up.upstream().Port))); err != nil {
			t.Fatalf("verify enabled %v: %v", enabled, err)
		}
		if err := tx.Apply(); err != nil {
			t.Fatalf("apply enabled %v: %v", enabled, err)
		}
	}

	if got, ok := system.NameServers(); !ok || !slices.Equal(got, stub) {
		t.Fatalf("published name servers %v (published %v), want %v unchanged", got, ok, stub)
	}
	if got := system.ProcessResolvers().Servers(); !slices.Equal(got, stub) {
		t.Fatalf("stub servers %v, want %v unchanged", got, stub)
	}
	fileAfter, err := os.ReadFile(resolvConf)
	if err != nil {
		t.Fatalf("read resolv.conf after: %v", err)
	}
	if !bytes.Equal(fileAfter, fileBefore) {
		t.Fatalf("resolv.conf changed:\n%s\nwant:\n%s", fileAfter, fileBefore)
	}
}
