package dnsresolver

import (
	"context"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/plugins/dnsresolver/yang"
)

// TestDoctorCodes reads the doctor checks this plugin's init() registered and
// looks up every code they declare, so `ze show diagnostic <code>` answers
// for each. The codes come from the registration, not from a list here, so a
// check added or given another code is judged with no edit to this file.
func TestDoctorCodes(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	checks, codes := 0, 0
	for _, check := range registry.PluginDoctorChecks() {
		if check.PluginName != "dnsresolver" {
			continue
		}
		checks++
		for _, code := range check.Codes {
			codes++
			if diagnostic.Lookup(code) == nil {
				t.Errorf("doctor check %q declares code %q, which the code registry does not hold", check.Name, code)
			}
		}
	}
	if checks != 2 {
		t.Fatalf("the dnsresolver registration answered %d doctor checks (%d codes), want 2: listeners and upstreams", checks, codes)
	}
}

// doctorTree parses text with the resolver's YANG into the tree ze doctor
// hands each check.
func doctorTree(t *testing.T, text string) registry.DoctorCheckContext {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(text, map[string]string{
		"ze-dns-resolver-conf.yang": yang.ZeDNSResolverConfYANG,
	})
	if err != nil {
		t.Fatalf("parse tree: %v", err)
	}
	return registry.DoctorCheckContext{Tree: tree}
}

// resolverText is an enabled resolver config with one listener lan on
// 127.0.0.1:port forwarding to forward.
func resolverText(enabled bool, port uint16, forward netip.AddrPort) string {
	return "service {\n dns-resolver {\n  enabled " + strconv.FormatBool(enabled) + "\n" +
		"  listener lan {\n   ip 127.0.0.1\n   port " + strconv.Itoa(int(port)) + "\n  }\n" +
		"  allow-from 127.0.0.0/8\n" +
		"  forward {\n   server " + forward.Addr().String() + " {\n    port " + strconv.Itoa(int(forward.Port())) + "\n   }\n  }\n" +
		" }\n}\n"
}

// TestDoctorListenerPortUnavailable proves the listener check through the
// tree ze doctor reads. Method: stand in a bind the system refuses with
// EACCES and expect one warning naming the listener, the endpoint and the
// fix; the same refusal under a disabled resolver warns nothing. Then the
// real probe: a port another socket holds is left to ze doctor's listener
// probe (doctor-listen-unavailable), so this check stays silent, and a free
// port is silent too.
func TestDoctorListenerPortUnavailable(t *testing.T) {
	up := aUpstream(t, 0)
	port := freeLoopbackPort(t)
	endpoint := "127.0.0.1:" + strconv.Itoa(int(port))

	listenerBind = func(netip.AddrPort) error {
		return &net.OpError{Op: "listen", Net: "udp", Err: os.NewSyscallError("bind", syscall.EACCES)}
	}
	t.Cleanup(func() { listenerBind = probeBindable })
	diags := checkResolverListeners(doctorTree(t, resolverText(true, port, up.upstream().addrPort())))
	if len(diags) != 1 {
		t.Fatalf("refused bind: %d diagnostics %v, want 1", len(diags), diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorDNSResolverPortUnavailable {
		t.Errorf("code %q, want %q", diags[0].Code, diagnostic.CodeDoctorDNSResolverPortUnavailable)
	}
	for _, want := range []string{"listener lan", endpoint, "CAP_NET_BIND_SERVICE", "service dns-resolver listener lan port"} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("message %q does not name %q", diags[0].Message, want)
		}
	}
	if diags := checkResolverListeners(doctorTree(t, resolverText(false, port, up.upstream().addrPort()))); len(diags) != 0 {
		t.Errorf("disabled resolver: %v, want no diagnostic", diags)
	}

	listenerBind = probeBindable
	var lc net.ListenConfig
	holder, err := lc.Listen(context.Background(), "tcp", endpoint)
	if err != nil {
		t.Fatalf("hold port: %v", err)
	}
	if err := probeBindable(netip.MustParseAddrPort(endpoint)); err == nil {
		t.Fatal("the probe bound a port another socket holds")
	}
	if diags := checkResolverListeners(doctorTree(t, resolverText(true, port, up.upstream().addrPort()))); len(diags) != 0 {
		t.Errorf("busy port: %v, want no diagnostic (doctor-listen-unavailable reports it)", diags)
	}
	if err := holder.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	if diags := checkResolverListeners(doctorTree(t, resolverText(true, port, up.upstream().addrPort()))); len(diags) != 0 {
		t.Errorf("free port: %v, want no diagnostic", diags)
	}
}

// TestDoctorUpstreamUnreachable proves the upstream check. Method: a silent
// upstream alone warns, naming the address it tried; a silent upstream
// before one that answers does not, because failover reaches the second;
// an answering upstream through the tree does not either.
func TestDoctorUpstreamUnreachable(t *testing.T) {
	silent := newFakeUpstream(t, func(*dns.Msg, bool) []fakeReply { return nil })
	answering := aUpstream(t, 0)
	timeout := 300 * time.Millisecond

	diags := upstreamDiagnostics([]upstream{silent.upstream()}, timeout)
	if len(diags) != 1 {
		t.Fatalf("silent upstream: %d diagnostics %v, want 1", len(diags), diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorDNSResolverUpstreamUnreachable {
		t.Errorf("code %q, want %q", diags[0].Code, diagnostic.CodeDoctorDNSResolverUpstreamUnreachable)
	}
	if want := silent.upstream().addrPort().String(); !strings.Contains(diags[0].Message, want) {
		t.Errorf("message %q does not name the upstream %s", diags[0].Message, want)
	}
	if diags := upstreamDiagnostics([]upstream{silent.upstream(), answering.upstream()}, timeout); len(diags) != 0 {
		t.Errorf("second upstream answers: %v, want no diagnostic", diags)
	}
	ctx := doctorTree(t, resolverText(true, freeLoopbackPort(t), answering.upstream().addrPort()))
	if diags := checkResolverUpstreams(ctx); len(diags) != 0 {
		t.Errorf("answering upstream through the tree: %v, want no diagnostic", diags)
	}
	if answering.queries.Load() == 0 {
		t.Error("the answering upstream saw no probe query")
	}
}
