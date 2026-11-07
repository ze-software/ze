// Design: docs/guide/configuration.md -- conntrack helper assignment
// Related: ../../../component/firewall/conntrack_helpers.go -- ConntrackHelperTables, the tables these tests install
// Related: ../../../component/config/system/conntrack_helpers.go -- the helper declarations every probe derives from
// Related: integration_linux_test.go -- withNftNetNS and the backend helper
// Related: counter_integration_linux_test.go -- loopbackUp

//go:build integration && linux

package firewallnft

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/config/system"
	_ "github.com/ze-software/ze/internal/component/config/system/yang" // registers ze-system-conf.yang, which declares the module leaf-list
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/test/userns"
)

// conntrackModulePath is the leaf-list whose enumeration names every helper an
// operator can list. The probes derive from it and from the helper table, so a
// helper the model gains is probed with no edit here.
const conntrackModulePath = "system/conntrack/module"

// ftpWait bounds every socket step of these tests. Each step crosses veths or
// loopback inside one kernel, so a step that takes this long has failed.
const ftpWait = 3 * time.Second

// ctnetlink message and attribute numbers, from the Linux UAPI header
// include/uapi/linux/netfilter/nfnetlink_conntrack.h. golang.org/x/sys/unix
// carries the subsystem numbers and none of these.
const (
	ctnlGet = 1 // IPCTNL_MSG_CT_GET, and IPCTNL_MSG_EXP_GET in the expectation subsystem

	ctaTupleOrig = 1 // CTA_TUPLE_ORIG
	ctaHelp      = 5 // CTA_HELP

	ctaTupleIP    = 1 // CTA_TUPLE_IP
	ctaTupleProto = 2 // CTA_TUPLE_PROTO

	ctaIPv4Src = 1 // CTA_IP_V4_SRC
	ctaIPv4Dst = 2 // CTA_IP_V4_DST
	ctaIPv6Src = 3 // CTA_IP_V6_SRC
	ctaIPv6Dst = 4 // CTA_IP_V6_DST

	ctaProtoNum     = 1 // CTA_PROTO_NUM
	ctaProtoSrcPort = 2 // CTA_PROTO_SRC_PORT
	ctaProtoDstPort = 3 // CTA_PROTO_DST_PORT

	ctaHelpName = 1 // CTA_HELP_NAME

	ctaExpectMaster   = 1 // CTA_EXPECT_MASTER
	ctaExpectHelpName = 6 // CTA_EXPECT_HELP_NAME

	nlaTypeMask = 0x3fff // NLA_TYPE_MASK: the type without NLA_F_NESTED and NLA_F_NET_BYTEORDER
)

// conntrackTrackingConfig is an operator table whose only job is to reference
// conntrack in the test namespace. The kernel registers its tracking hooks in
// a namespace only once a rule references ct, so without it a namespace with no
// helper table tracks nothing, and "no helper on the entry" would be read from
// an entry that never existed.
const conntrackTrackingConfig = `{"firewall":{"table":{"track":{"family":"inet","chain":{"output":{` +
	`"type":"filter","hook":"output","priority":"0","policy":"accept",` +
	`"term":{"seen":{"from":{"connection-state":"new,established,related"},"then":{"accept":""}}}}}}}}}`

// conntrackMasqueradeConfig masquerades what the router sends toward the
// server, as a CPE masquerades its WAN.
const conntrackMasqueradeConfig = `{"firewall":{"table":{"nat":{"family":"inet","chain":{"postrouting":{` +
	`"type":"nat","hook":"postrouting","priority":"100","policy":"accept",` +
	`"term":{"masq":{"from":{"output-interface":"zfts"},"then":{"masquerade":""}}}}}}}}}`

// VALIDATES: spec-home-cpe-3 AC-2, AC-3, AC-4, AC-11, AC-12. Every helper the
// model accepts is listed, its tables are installed through the nft backend,
// and one connection per row, transport, port and family is opened. Each
// connection's conntrack entry, read over ctnetlink, carries the row's kernel
// helper in every family the row names, and an IPv4-only row's IPv6
// connection is tracked and carries none. Every IPv4-only row's object lives in
// an ip table and in no inet table, so no IPv6 rule can assign it.
// PREVENTS: a helper that is listed, built in and declared, but never attached
// to a connection, which is what loading a module alone gives since Linux 6.0.
//
// The method: loopback connections in a throwaway namespace. A locally
// originated connection meets the output chain, so this proves the output
// assignment; TestConntrackHelperFTPThroughMasquerade proves the prerouting
// one on a forwarded connection.
//
// Red walks, each in the producing code:
//   - delete the output base chain from newConntrackHelperTable
//     (internal/component/firewall/conntrack_helpers.go): every probe reads no
//     helper.
//   - in ConntrackHelperTables, put a ConntrackHelperIPv4Only row in the inet
//     table: the object family check fails for irc, netbios-ns, pptp and snmp.
//     The IPv6 probes of those rows stay green under this break, because the
//     kernel holds no IPv6 helper for the object to resolve; the family check
//     is what proves AC-2.
func TestConntrackHelperAssignment(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withNftNetNS(t, func() {
		loopbackUp(t)
		modules := helperModules(t)
		probes := helperProbes(t, modules)

		lowered := installHelperTables(t, newNftIntegrationBackend(t), modules, nil)
		requireHelperObjectFamilies(t, lowered, probes)

		checkProbes(t, "every helper listed", probes, func(p helperProbe) string {
			if p.assigned {
				return p.helper
			}
			return ""
		})
	})
}

// VALIDATES: spec-home-cpe-3 A-1, R-3 and AC-5, with the helpers built into
// the kernel. With the list empty, no connection to any helper's port, in
// either family, carries a helper, and an FTP PORT command over tcp 21 creates
// no expectation. With ftp alone listed, the ftp connections carry "ftp", the
// PORT command creates an ftp expectation, and every other helper's
// connections carry none: each helper is enabled on its own.
// PREVENTS: a kernel or a lowering that assigns helpers the operator did not
// list. RFC 4787 Section 7: "To eliminate interference with UNSAF NAT
// traversal mechanisms and allow integrity protection of UDP communications,
// NAT ALGs for UDP-based protocols SHOULD be turned off."
//
// The ftp leg runs first and is the positive control for the empty leg: the
// same probes and the same PORT command, through the same kernel, do produce a
// helper and an expectation when the helper is listed.
//
// Red walks, each in ConntrackHelperTables
// (internal/component/firewall/conntrack_helpers.go):
//   - lower every helper the model declares when the list is empty: the empty
//     leg reads helpers on every probe and an ftp expectation.
//   - lower every helper the model declares whatever the list holds: the ftp
//     leg reads a helper on every non-ftp probe.
func TestConntrackHelperNotAssignedWhenUnlisted(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withNftNetNS(t, func() {
		loopbackUp(t)
		modules := helperModules(t)
		if !slices.Contains(modules, "ftp") {
			t.Fatalf("the model declares no ftp helper (it declares %v), so the positive control cannot run", modules)
		}
		probes := helperProbes(t, modules)
		tracking, err := firewall.ParseFirewallConfig(conntrackTrackingConfig)
		if err != nil {
			t.Fatalf("parse the tracking table: %v", err)
		}
		b := newNftIntegrationBackend(t)

		installHelperTables(t, b, []string{"ftp"}, tracking)
		checkProbes(t, "ftp listed", probes, func(p helperProbe) string {
			if p.module == "ftp" && p.assigned {
				return p.helper
			}
			return ""
		})
		if got := loopbackFTPExpectations(t); !slices.Equal(got, []string{"ftp"}) {
			t.Errorf("ftp listed: the PORT command created expectations with helpers %q, want exactly one ftp expectation", got)
		}

		flushConntrack(t)

		if lowered := installHelperTables(t, b, nil, tracking); len(lowered) != 0 {
			t.Fatalf("an empty helper list lowered %d tables, want none", len(lowered))
		}
		checkProbes(t, "nothing listed", probes, func(helperProbe) string { return "" })
		if got := loopbackFTPExpectations(t); len(got) != 0 {
			t.Errorf("nothing listed: the PORT command created expectations with helpers %q, want none", got)
		}
	})
}

// VALIDATES: spec-home-cpe-3 AC-6, in both families. A client namespace, a
// router namespace masquerading toward the server, and a server namespace with
// no route back to the client. With ftp listed, the PORT (IPv4) or EPRT (IPv6)
// command reaches the server rewritten to the router's masquerade address, the
// server's active data connection to it succeeds, and the client reads the
// data. Without ftp, the command reaches the server naming the client's own
// address, which the server cannot reach.
// PREVENTS: an FTP helper assigned only to locally originated connections, or a
// helper with no NAT companion, either of which leaves active FTP broken
// behind the masquerade.
//
// The leg without ftp is the red phase the spec names: the same lab, the same
// client and server, with only the configured list changed.
//
// Red walk, in the producing code: delete the prerouting base chain from
// newConntrackHelperTable (internal/component/firewall/conntrack_helpers.go).
// The control connection is forwarded, so it meets prerouting and never
// output; the command then arrives unrewritten and the data connection fails.
func TestConntrackHelperFTPThroughMasquerade(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withNftNetNS(t, func() {
		lab := openFTPLab(t)
		masquerade, err := firewall.ParseFirewallConfig(conntrackMasqueradeConfig)
		if err != nil {
			t.Fatalf("parse the masquerade table: %v", err)
		}
		b := newNftIntegrationBackend(t)

		for _, f := range ftpFamilies {
			installHelperTables(t, b, nil, masquerade)
			flushConntrack(t)
			plain := lab.activeFTP(t, f)
			if plain.named.Addr() != f.client {
				t.Errorf("%s without ftp: the server received a %s naming %s, want the client's own address %s, unrewritten",
					f.name, f.command, plain.named, f.client)
			}
			if plain.dialErr == nil {
				t.Errorf("%s without ftp: the server reached %s, so the lab routes around the masquerade and the leg proves nothing",
					f.name, plain.named)
			}

			installHelperTables(t, b, []string{"ftp"}, masquerade)
			flushConntrack(t)
			got := lab.activeFTP(t, f)
			if got.named.Addr() != f.masquerade {
				t.Errorf("%s with ftp: the server received a %s naming %s, want the masquerade address %s",
					f.name, f.command, got.named, f.masquerade)
			}
			if got.dialErr != nil {
				t.Errorf("%s with ftp: the server's data connection to %s failed: %v", f.name, got.named, got.dialErr)
				continue
			}
			if got.readErr != nil {
				t.Errorf("%s with ftp: the client read no data: %v", f.name, got.readErr)
				continue
			}
			if got.data != ftpPayload {
				t.Errorf("%s with ftp: the client read %q, want %q", f.name, got.data, ftpPayload)
			}
		}
	})
}

// helperModules answers every helper the model lets an operator list.
func helperModules(t *testing.T) []string {
	t.Helper()
	modules, err := configyang.EnumValues(conntrackModulePath)
	if err != nil {
		t.Fatalf("read the helper modules the model declares: %v", err)
	}
	if len(modules) == 0 {
		t.Fatal("the model declares no conntrack helper, so there is nothing to probe")
	}
	return modules
}

// helperProbe is one connection to open: a row's transport and port, in one
// address family.
type helperProbe struct {
	module   string
	helper   string // the kernel helper the row assigns
	protocol string // "tcp" or "udp"
	port     uint16
	loopback netip.Addr
	assigned bool // the row's helper is registered, and assigned, in this probe's family
}

func (p helperProbe) String() string {
	return p.module + " " + p.protocol + " " + netip.AddrPortFrom(p.loopback, p.port).String()
}

// helperProbes answers one probe per row, transport, port and address family.
func helperProbes(t *testing.T, modules []string) []helperProbe {
	t.Helper()
	v4 := netip.MustParseAddr("127.0.0.1")
	v6 := netip.IPv6Loopback()
	var probes []helperProbe
	for _, module := range modules {
		row, err := system.ConntrackHelperByName(module)
		if err != nil {
			t.Fatalf("the model accepts %q and the helper table does not declare it: %v", module, err)
		}
		var dual bool
		switch row.Family {
		case system.ConntrackHelperBothFamilies:
			dual = true
		case system.ConntrackHelperIPv4Only:
			dual = false
		default:
			t.Fatalf("helper %q declares no address family", module)
		}
		for _, rule := range row.Rules {
			for _, port := range rule.Ports {
				probes = append(probes,
					helperProbe{module, rule.KernelHelper, rule.Protocol, port, v4, true},
					helperProbe{module, rule.KernelHelper, rule.Protocol, port, v6, dual})
			}
		}
	}
	return probes
}

// installHelperTables lowers modules to their helper tables, verifies them
// with the operator tables extra as the daemon does, and applies the whole set
// through the backend. It answers the helper tables alone.
func installHelperTables(t *testing.T, b *backend, modules []string, extra []firewall.Table) []firewall.Table {
	t.Helper()
	lowered, err := firewall.ConntrackHelperTables(modules)
	if err != nil {
		t.Fatalf("lower the helpers %v: %v", modules, err)
	}
	desired := slices.Concat(extra, lowered)
	if err := firewall.ValidateTables(desired); err != nil {
		t.Fatalf("verify the helper tables for %v: %v", modules, err)
	}
	if err := b.Apply(desired); err != nil {
		t.Fatalf("apply the helper tables for %v: %v", modules, err)
	}
	return lowered
}

// helperObjectKey names a ct helper object by what it resolves to in the
// kernel: the helper name and the transport.
type helperObjectKey struct {
	helper  string
	l4proto uint8
}

// requireHelperObjectFamilies reads back every ct helper object of the
// installed helper tables and checks that a row registered for both families
// sits in an inet table only, and an IPv4-only row in an ip table only.
func requireHelperObjectFamilies(t *testing.T, lowered []firewall.Table, probes []helperProbe) {
	t.Helper()
	conn, err := nftables.New()
	if err != nil {
		t.Fatalf("nftables.New: %v", err)
	}
	installed := map[helperObjectKey][]nftables.TableFamily{}
	for _, table := range lowered {
		family, err := lowerFamily(table.Family)
		if err != nil {
			t.Fatalf("lower the family of %s: %v", table.Name, err)
		}
		objects, err := conn.GetNamedObjects(&nftables.Table{Name: table.Name, Family: family})
		if err != nil {
			t.Fatalf("read the objects of %s: %v", table.Name, err)
		}
		for _, object := range objects {
			named, ok := object.(*nftables.NamedObj)
			if !ok || named.Type != nftables.ObjTypeCtHelper {
				continue
			}
			helper, ok := named.Obj.(*expr.CtHelper)
			if !ok {
				t.Fatalf("ct helper object %s in %s decoded as %T", named.Name, table.Name, named.Obj)
			}
			key := helperObjectKey{helper: helper.Name, l4proto: helper.L4Proto}
			installed[key] = append(installed[key], family)
		}
	}
	for _, p := range probes {
		if p.loopback.Is4() {
			continue
		}
		key := helperObjectKey{helper: p.helper, l4proto: protocolNumber(t, p.protocol)}
		want := []nftables.TableFamily{nftables.TableFamilyIPv4}
		if p.assigned {
			want = []nftables.TableFamily{nftables.TableFamilyINet}
		}
		if got := installed[key]; !slices.Equal(got, want) {
			t.Errorf("%s: the %s object sits in tables of family %v, want %v", p, p.helper, got, want)
		}
	}
}

// checkProbes opens every probe's connection and compares the helper its
// conntrack entry carries with want(probe), "" meaning none.
func checkProbes(t *testing.T, leg string, probes []helperProbe, want func(helperProbe) string) {
	t.Helper()
	for _, p := range probes {
		got, found := probeHelper(t, p)
		if !found {
			t.Errorf("%s: %s: the conntrack table holds no entry for the connection, so its helper cannot be read", leg, p)
			continue
		}
		if expected := want(p); got != expected {
			t.Errorf("%s: %s: the conntrack entry carries helper %q, want %q", leg, p, got, expected)
		}
	}
}

// probeHelper opens the probe's connection, reads the helper of its conntrack
// entry while the connection is live, and closes it, so the next probe can
// bind the same port.
func probeHelper(t *testing.T, p helperProbe) (string, bool) {
	t.Helper()
	ctx := t.Context()
	target := netip.AddrPortFrom(p.loopback, p.port)
	var lc net.ListenConfig
	d := net.Dialer{Timeout: ftpWait}

	switch p.protocol {
	case "tcp":
		listener, err := lc.Listen(ctx, "tcp", target.String())
		if err != nil {
			t.Fatalf("%s: listen: %v", p, err)
		}
		defer listener.Close() //nolint:errcheck // the namespace goes away with the test

		client, err := d.DialContext(ctx, "tcp", target.String())
		if err != nil {
			t.Fatalf("%s: connect: %v", p, err)
		}
		defer client.Close() //nolint:errcheck // the namespace goes away with the test

		return connectionHelper(t, unix.IPPROTO_TCP, localAddrPort(t, client), target)
	case "udp":
		listener, err := lc.ListenPacket(ctx, "udp", target.String())
		if err != nil {
			t.Fatalf("%s: listen: %v", p, err)
		}
		defer listener.Close() //nolint:errcheck // the namespace goes away with the test

		client, err := d.DialContext(ctx, "udp", target.String())
		if err != nil {
			t.Fatalf("%s: connect: %v", p, err)
		}
		defer client.Close() //nolint:errcheck // the namespace goes away with the test

		if _, err := client.Write([]byte("x")); err != nil {
			t.Fatalf("%s: send: %v", p, err)
		}
		// Reading the datagram back proves it crossed every hook, rather than
		// proving only that sendto returned.
		if err := listener.SetReadDeadline(time.Now().Add(ftpWait)); err != nil {
			t.Fatalf("%s: set the read deadline: %v", p, err)
		}
		var buf [16]byte
		if _, _, err := listener.ReadFrom(buf[:]); err != nil {
			t.Fatalf("%s: the datagram never arrived: %v", p, err)
		}
		return connectionHelper(t, unix.IPPROTO_UDP, localAddrPort(t, client), target)
	}
	t.Fatalf("%s: unknown transport %q", p, p.protocol)
	return "", false
}

// loopbackFTPExpectations runs an FTP control exchange over tcp 21 on loopback,
// ending in a PORT command, and answers the helper of every expectation the
// kernel then holds for that control connection.
func loopbackFTPExpectations(t *testing.T) []string {
	t.Helper()
	ctx := t.Context()
	loopback := netip.MustParseAddr("127.0.0.1")
	control := netip.AddrPortFrom(loopback, 21)
	var lc net.ListenConfig

	serverListener, err := lc.Listen(ctx, "tcp", control.String())
	if err != nil {
		t.Fatalf("listen on %s: %v", control, err)
	}
	defer serverListener.Close() //nolint:errcheck // the namespace goes away with the test

	dataListener, err := lc.Listen(ctx, "tcp", netip.AddrPortFrom(loopback, 0).String())
	if err != nil {
		t.Fatalf("listen for the data connection: %v", err)
	}
	defer dataListener.Close() //nolint:errcheck // the namespace goes away with the test

	d := net.Dialer{Timeout: ftpWait}
	client, err := d.DialContext(ctx, "tcp", control.String())
	if err != nil {
		t.Fatalf("connect to %s: %v", control, err)
	}
	defer client.Close() //nolint:errcheck // the namespace goes away with the test

	server := acceptWithin(t, serverListener)
	defer server.Close() //nolint:errcheck // the namespace goes away with the test

	ftpControlExchange(t, client, server, ftpDataCommand(ipv4Family, listenerAddrPort(t, dataListener)))
	return expectationHelpers(t, localAddrPort(t, client))
}

// ftpFamily is one address family of the masquerade lab.
type ftpFamily struct {
	name       string
	command    string // the active-mode command the client sends: PORT or EPRT
	client     netip.Addr
	server     netip.Addr
	masquerade netip.Addr // the router's address toward the server
}

var (
	ipv4Family = ftpFamily{
		name: "IPv4", command: "PORT",
		client:     netip.MustParseAddr("192.0.2.10"),
		server:     netip.MustParseAddr("198.51.100.20"),
		masquerade: netip.MustParseAddr("198.51.100.1"),
	}
	ipv6Family = ftpFamily{
		name: "IPv6", command: "EPRT",
		client:     netip.MustParseAddr("2001:db8:a::10"),
		server:     netip.MustParseAddr("2001:db8:b::20"),
		masquerade: netip.MustParseAddr("2001:db8:b::1"),
	}
	ftpFamilies = []ftpFamily{ipv4Family, ipv6Family}
)

// ftpPayload is what the server sends on the data connection.
const ftpPayload = "ze-active-ftp-data"

// ftpSide is one side of the router: its veth, the peer namespace's addresses
// and the router's own.
type ftpSide struct {
	veth             string
	routerV4, peerV4 netip.Prefix
	routerV6, peerV6 netip.Prefix
	defaultRoute     bool // the peer routes everything through the router
}

var (
	// The client routes through the router, as a LAN host does.
	ftpClientSide = ftpSide{
		veth:     "zftc",
		routerV4: netip.MustParsePrefix("192.0.2.1/24"), peerV4: netip.MustParsePrefix("192.0.2.10/24"),
		routerV6: netip.MustParsePrefix("2001:db8:a::1/64"), peerV6: netip.MustParsePrefix("2001:db8:a::10/64"),
		defaultRoute: true,
	}
	// The server has no route to the client's network: only an address the
	// masquerade rewrote can reach the client.
	ftpServerSide = ftpSide{
		veth:     "zfts",
		routerV4: netip.MustParsePrefix("198.51.100.1/24"), peerV4: netip.MustParsePrefix("198.51.100.20/24"),
		routerV6: netip.MustParsePrefix("2001:db8:b::1/64"), peerV6: netip.MustParsePrefix("2001:db8:b::20/64"),
	}
)

// ftpLab is three namespaces: the test's own, which routes and masquerades,
// and a client and a server behind it. Every socket is opened on the test's
// locked thread after a switch into the namespace that owns it, so no second
// goroutine ever runs in a lab namespace.
type ftpLab struct {
	router netns.NsHandle
	client netns.NsHandle
	server netns.NsHandle
}

// openFTPLab builds the lab around the calling thread's namespace, which
// becomes the router. The caller MUST have locked its OS thread.
func openFTPLab(t *testing.T) ftpLab {
	t.Helper()
	router, err := netns.Get()
	if err != nil {
		t.Fatalf("read the router namespace: %v", err)
	}
	t.Cleanup(func() { router.Close() }) //nolint:errcheck // best-effort cleanup

	lab := ftpLab{router: router}
	lab.client = lab.newNamespace(t)
	lab.server = lab.newNamespace(t)
	lab.connect(t, lab.client, ftpClientSide)
	lab.connect(t, lab.server, ftpServerSide)

	for _, knob := range []string{"/proc/sys/net/ipv4/ip_forward", "/proc/sys/net/ipv6/conf/all/forwarding"} {
		if err := os.WriteFile(knob, []byte("1"), 0o644); err != nil { //nolint:gosec // a sysctl of the test's own namespace
			t.Fatalf("enable forwarding in the router namespace (%s): %v", knob, err)
		}
	}
	return lab
}

// newNamespace creates an empty namespace and returns the thread to the
// router.
func (l ftpLab) newNamespace(t *testing.T) netns.NsHandle {
	t.Helper()
	ns, err := netns.New()
	if err != nil {
		t.Fatalf("create a lab namespace: %v", err)
	}
	t.Cleanup(func() { ns.Close() }) //nolint:errcheck // best-effort cleanup
	if err := netns.Set(l.router); err != nil {
		t.Fatalf("return to the router namespace: %v", err)
	}
	return ns
}

// in runs fn with the thread in ns, then returns it to the router.
func (l ftpLab) in(t *testing.T, ns netns.NsHandle, fn func()) {
	t.Helper()
	if err := netns.Set(ns); err != nil {
		t.Fatalf("enter a lab namespace: %v", err)
	}
	defer func() {
		if err := netns.Set(l.router); err != nil {
			t.Errorf("return to the router namespace: %v", err)
		}
	}()
	fn()
}

// connect joins the router to ns with a veth pair, addressing both ends in
// both families.
func (l ftpLab) connect(t *testing.T, ns netns.NsHandle, side ftpSide) {
	t.Helper()
	attrs := netlink.NewLinkAttrs()
	attrs.Name = side.veth
	peerName := side.veth + "p"
	if err := netlink.LinkAdd(&netlink.Veth{LinkAttrs: attrs, PeerName: peerName}); err != nil {
		t.Fatalf("create the veth %s: %v", side.veth, err)
	}
	peer, err := netlink.LinkByName(peerName)
	if err != nil {
		t.Fatalf("read %s: %v", peerName, err)
	}
	if err := netlink.LinkSetNsFd(peer, int(ns)); err != nil {
		t.Fatalf("move %s into its namespace: %v", peerName, err)
	}
	upWithAddresses(t, side.veth, side.routerV4, side.routerV6)

	l.in(t, ns, func() {
		link := upWithAddresses(t, peerName, side.peerV4, side.peerV6)
		if !side.defaultRoute {
			return
		}
		for _, gateway := range []netip.Addr{side.routerV4.Addr(), side.routerV6.Addr()} {
			route := &netlink.Route{LinkIndex: link.Attrs().Index, Gw: net.IP(gateway.AsSlice())}
			if err := netlink.RouteAdd(route); err != nil {
				t.Fatalf("add the default route via %s on %s: %v", gateway, peerName, err)
			}
		}
	})
}

// upWithAddresses addresses the link in both families and brings it up. The
// IPv6 address skips duplicate address detection, so it is usable at once.
func upWithAddresses(t *testing.T, name string, v4, v6 netip.Prefix) netlink.Link {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	for _, prefix := range []netip.Prefix{v4, v6} {
		addr := &netlink.Addr{IPNet: &net.IPNet{
			IP:   net.IP(prefix.Addr().AsSlice()),
			Mask: net.CIDRMask(prefix.Bits(), prefix.Addr().BitLen()),
		}}
		if prefix.Addr().Is6() {
			addr.Flags = unix.IFA_F_NODAD
		}
		if err := netlink.AddrAdd(link, addr); err != nil {
			t.Fatalf("add %s to %s: %v", prefix, name, err)
		}
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("bring %s up: %v", name, err)
	}
	return link
}

// ftpTransfer is what one active FTP attempt through the lab produced.
type ftpTransfer struct {
	named   netip.AddrPort // the data address the PORT or EPRT named when it reached the server
	dialErr error          // the server's data connection to named
	readErr error          // the client's accept and read of the data connection
	data    string         // what the client read
}

// activeFTP runs one active-mode FTP control exchange from the client to the
// server through the router, then has the server open the data connection to
// the address its command named, as an FTP server does.
func (l ftpLab) activeFTP(t *testing.T, f ftpFamily) ftpTransfer {
	t.Helper()
	ctx := t.Context()
	control := netip.AddrPortFrom(f.server, 21)
	var lc net.ListenConfig
	d := net.Dialer{Timeout: ftpWait}

	var serverListener, dataListener net.Listener
	var client net.Conn
	l.in(t, l.server, func() {
		var err error
		if serverListener, err = lc.Listen(ctx, "tcp", control.String()); err != nil {
			t.Fatalf("%s: listen on %s: %v", f.name, control, err)
		}
	})
	defer serverListener.Close() //nolint:errcheck // the namespace goes away with the test

	l.in(t, l.client, func() {
		var err error
		if dataListener, err = lc.Listen(ctx, "tcp", netip.AddrPortFrom(f.client, 0).String()); err != nil {
			t.Fatalf("%s: listen for the data connection: %v", f.name, err)
		}
		if client, err = d.DialContext(ctx, "tcp", control.String()); err != nil {
			t.Fatalf("%s: connect to %s through the router: %v", f.name, control, err)
		}
	})
	defer dataListener.Close() //nolint:errcheck // the namespace goes away with the test
	defer client.Close()       //nolint:errcheck // the namespace goes away with the test

	server := acceptWithin(t, serverListener)
	defer server.Close() //nolint:errcheck // the namespace goes away with the test

	received := ftpControlExchange(t, client, server, ftpDataCommand(f, listenerAddrPort(t, dataListener)))
	named, err := parseFTPDataCommand(received)
	if err != nil {
		t.Fatalf("%s: the server received %q: %v", f.name, received, err)
	}

	result := ftpTransfer{named: named}
	var data net.Conn
	l.in(t, l.server, func() {
		data, result.dialErr = d.DialContext(ctx, "tcp", named.String())
	})
	if result.dialErr != nil {
		return result
	}
	defer data.Close() //nolint:errcheck // the namespace goes away with the test

	if _, err := io.WriteString(data, ftpPayload); err != nil {
		result.readErr = err
		return result
	}
	if err := data.Close(); err != nil {
		result.readErr = err
		return result
	}
	accepted, err := acceptDeadline(dataListener)
	if err != nil {
		result.readErr = err
		return result
	}
	defer accepted.Close() //nolint:errcheck // the namespace goes away with the test
	if err := accepted.SetReadDeadline(time.Now().Add(ftpWait)); err != nil {
		result.readErr = err
		return result
	}
	payload, err := io.ReadAll(accepted)
	result.data, result.readErr = string(payload), err
	return result
}

// ftpControlExchange plays the start of an FTP session: the server's
// greeting, a USER command and its reply, then command. It answers command as
// the server received it.
//
// The USER line is load-bearing: nf_conntrack_ftp parses a client segment only
// when it starts right after a newline the helper saw in that direction, so
// the first client line of a session is never parsed for PORT or EPRT.
func ftpControlExchange(t *testing.T, client, server net.Conn, command string) string {
	t.Helper()
	deadline := time.Now().Add(ftpWait)
	for _, conn := range []net.Conn{client, server} {
		if err := conn.SetDeadline(deadline); err != nil {
			t.Fatalf("set the control deadline: %v", err)
		}
	}
	clientReader := bufio.NewReader(client)
	serverReader := bufio.NewReader(server)

	ftpLine(t, server, clientReader, "220 ze\r\n")
	ftpLine(t, client, serverReader, "USER anonymous\r\n")
	ftpLine(t, server, clientReader, "331 ze\r\n")
	return ftpLine(t, client, serverReader, command+"\r\n")
}

// ftpLine writes line on from, reads one line on to, and answers it without
// its line ending.
func ftpLine(t *testing.T, from net.Conn, to *bufio.Reader, line string) string {
	t.Helper()
	if _, err := io.WriteString(from, line); err != nil {
		t.Fatalf("send %q: %v", strings.TrimSpace(line), err)
	}
	got, err := to.ReadString('\n')
	if err != nil {
		t.Fatalf("read the line after sending %q: %v", strings.TrimSpace(line), err)
	}
	return strings.TrimRight(got, "\r\n")
}

// ftpDataCommand writes the active-mode command naming data: PORT for IPv4
// (RFC 959), EPRT for IPv6 (RFC 2428).
func ftpDataCommand(f ftpFamily, data netip.AddrPort) string {
	if f.command == "EPRT" {
		return "EPRT |2|" + data.Addr().String() + "|" + strconv.Itoa(int(data.Port())) + "|"
	}
	octets := data.Addr().As4()
	fields := make([]string, 0, 6)
	for _, octet := range octets {
		fields = append(fields, strconv.Itoa(int(octet)))
	}
	fields = append(fields, strconv.Itoa(int(data.Port()>>8)), strconv.Itoa(int(data.Port()&0xff)))
	return "PORT " + strings.Join(fields, ",")
}

// parseFTPDataCommand reads the data address out of a PORT or EPRT line.
func parseFTPDataCommand(line string) (netip.AddrPort, error) {
	if rest, ok := strings.CutPrefix(line, "PORT "); ok {
		fields := strings.Split(rest, ",")
		if len(fields) != 6 {
			return netip.AddrPort{}, errors.New("a PORT command carries six fields")
		}
		var values [6]byte
		for i, field := range fields {
			v, err := strconv.ParseUint(field, 10, 8)
			if err != nil {
				return netip.AddrPort{}, err
			}
			values[i] = byte(v)
		}
		addr := netip.AddrFrom4([4]byte(values[:4]))
		return netip.AddrPortFrom(addr, uint16(values[4])<<8|uint16(values[5])), nil
	}
	if rest, ok := strings.CutPrefix(line, "EPRT "); ok {
		if rest == "" {
			return netip.AddrPort{}, errors.New("an EPRT command carries a delimiter")
		}
		fields := strings.Split(rest[1:], rest[:1])
		if len(fields) != 4 {
			return netip.AddrPort{}, errors.New("an EPRT command carries a protocol, an address and a port")
		}
		addr, err := netip.ParseAddr(fields[1])
		if err != nil {
			return netip.AddrPort{}, err
		}
		port, err := strconv.ParseUint(fields[2], 10, 16)
		if err != nil {
			return netip.AddrPort{}, err
		}
		return netip.AddrPortFrom(addr, uint16(port)), nil
	}
	return netip.AddrPort{}, errors.New("not a PORT or EPRT command")
}

// acceptWithin accepts one connection on listener, failing the test when none
// arrives in time.
func acceptWithin(t *testing.T, listener net.Listener) net.Conn {
	t.Helper()
	conn, err := acceptDeadline(listener)
	if err != nil {
		t.Fatalf("accept on %s: %v", listener.Addr(), err)
	}
	return conn
}

// acceptDeadline accepts one connection on a TCP listener within ftpWait.
func acceptDeadline(listener net.Listener) (net.Conn, error) {
	tcp, ok := listener.(*net.TCPListener)
	if !ok {
		return nil, errors.New("not a TCP listener")
	}
	if err := tcp.SetDeadline(time.Now().Add(ftpWait)); err != nil {
		return nil, err
	}
	return tcp.Accept()
}

// localAddrPort answers the local end of conn, unmapped, as conntrack records it.
func localAddrPort(t *testing.T, conn net.Conn) netip.AddrPort {
	t.Helper()
	ap, err := netip.ParseAddrPort(conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("read the local address of the connection: %v", err)
	}
	return netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())
}

// listenerAddrPort answers the address listener is bound to, unmapped.
func listenerAddrPort(t *testing.T, listener net.Listener) netip.AddrPort {
	t.Helper()
	ap, err := netip.ParseAddrPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("read the listener address: %v", err)
	}
	return netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())
}

func protocolNumber(t *testing.T, protocol string) uint8 {
	t.Helper()
	switch protocol {
	case "tcp":
		return unix.IPPROTO_TCP
	case "udp":
		return unix.IPPROTO_UDP
	}
	t.Fatalf("unknown transport %q", protocol)
	return 0
}

// flushConntrack empties the conntrack table of the thread's namespace, and
// with it every expectation, so a leg reads only what its own connections
// produced. ctnetlink flushes every family when the request carries version 0.
func flushConntrack(t *testing.T) {
	t.Helper()
	if err := netlink.ConntrackTableFlush(netlink.ConntrackTable); err != nil {
		t.Fatalf("flush the conntrack table: %v", err)
	}
	if err := netlink.ConntrackTableFlush(netlink.ConntrackExpectTable); err != nil {
		t.Fatalf("flush the expectation table: %v", err)
	}
}

// ctTuple is one direction of a conntrack entry.
type ctTuple struct {
	source   netip.AddrPort
	target   netip.AddrPort
	protocol uint8
}

// connectionHelper answers the kernel helper the conntrack entry of the
// connection from client to server carries, "" for none, and false when the
// table holds no entry for that connection at all.
func connectionHelper(t *testing.T, protocol uint8, client, server netip.AddrPort) (string, bool) {
	t.Helper()
	want := ctTuple{source: client, target: server, protocol: protocol}
	for _, msg := range ctnetlinkDump(t, unix.NFNL_SUBSYS_CTNETLINK) {
		attrs := msg[nl.SizeofNfgenmsg:]
		original, ok := nfAttr(attrs, ctaTupleOrig)
		if !ok {
			continue
		}
		tuple, ok := parseCtTuple(original)
		if !ok || tuple != want {
			continue
		}
		help, ok := nfAttr(attrs, ctaHelp)
		if !ok {
			return "", true
		}
		name, ok := nfAttr(help, ctaHelpName)
		if !ok {
			t.Fatalf("the conntrack entry of %s -> %s carries CTA_HELP with no helper name", client, server)
		}
		return strings.TrimRight(string(name), "\x00"), true
	}
	return "", false
}

// expectationHelpers answers the helper of every expectation whose master
// connection starts at client.
func expectationHelpers(t *testing.T, client netip.AddrPort) []string {
	t.Helper()
	var helpers []string
	for _, msg := range ctnetlinkDump(t, unix.NFNL_SUBSYS_CTNETLINK_EXP) {
		attrs := msg[nl.SizeofNfgenmsg:]
		master, ok := nfAttr(attrs, ctaExpectMaster)
		if !ok {
			continue
		}
		tuple, ok := parseCtTuple(master)
		if !ok || tuple.source != client {
			continue
		}
		name, ok := nfAttr(attrs, ctaExpectHelpName)
		if !ok {
			helpers = append(helpers, "")
			continue
		}
		helpers = append(helpers, strings.TrimRight(string(name), "\x00"))
	}
	return helpers
}

// ctnetlinkDump dumps one ctnetlink subsystem of the thread's namespace, every
// family, and answers each message after its netlink header.
func ctnetlinkDump(t *testing.T, subsystem int) [][]byte {
	t.Helper()
	req := nl.NewNetlinkRequest(subsystem<<8|ctnlGet, unix.NLM_F_DUMP)
	req.AddData(&nl.Nfgenmsg{NfgenFamily: unix.AF_UNSPEC, Version: nl.NFNETLINK_V0})
	msgs, err := req.Execute(unix.NETLINK_NETFILTER, 0)
	if err != nil {
		t.Fatalf("dump ctnetlink subsystem %d: %v", subsystem, err)
	}
	dumped := msgs[:0]
	for _, msg := range msgs {
		if len(msg) >= nl.SizeofNfgenmsg {
			dumped = append(dumped, msg)
		}
	}
	return dumped
}

// parseCtTuple reads a CTA_TUPLE_* nest. It answers false for a tuple with no
// ports, which none of these tests opens.
func parseCtTuple(b []byte) (ctTuple, bool) {
	ip, ok := nfAttr(b, ctaTupleIP)
	if !ok {
		return ctTuple{}, false
	}
	proto, ok := nfAttr(b, ctaTupleProto)
	if !ok {
		return ctTuple{}, false
	}
	source, ok := nfAddr(ip, ctaIPv4Src, ctaIPv6Src)
	if !ok {
		return ctTuple{}, false
	}
	target, ok := nfAddr(ip, ctaIPv4Dst, ctaIPv6Dst)
	if !ok {
		return ctTuple{}, false
	}
	number, ok := nfAttr(proto, ctaProtoNum)
	if !ok || len(number) != 1 {
		return ctTuple{}, false
	}
	sourcePort, ok := nfAttr(proto, ctaProtoSrcPort)
	if !ok || len(sourcePort) != 2 {
		return ctTuple{}, false
	}
	targetPort, ok := nfAttr(proto, ctaProtoDstPort)
	if !ok || len(targetPort) != 2 {
		return ctTuple{}, false
	}
	return ctTuple{
		source:   netip.AddrPortFrom(source, binary.BigEndian.Uint16(sourcePort)),
		target:   netip.AddrPortFrom(target, binary.BigEndian.Uint16(targetPort)),
		protocol: number[0],
	}, true
}

// nfAddr reads the IPv4 or the IPv6 attribute of a CTA_TUPLE_IP nest.
func nfAddr(b []byte, v4, v6 uint16) (netip.Addr, bool) {
	if v, ok := nfAttr(b, v4); ok && len(v) == 4 {
		return netip.AddrFrom4([4]byte(v)), true
	}
	if v, ok := nfAttr(b, v6); ok && len(v) == 16 {
		return netip.AddrFrom16([16]byte(v)), true
	}
	return netip.Addr{}, false
}

// nfAttr answers the payload of the first netlink attribute of kind in b. The
// bound is b itself: each step consumes at least one aligned attribute header.
func nfAttr(b []byte, kind uint16) ([]byte, bool) {
	for len(b) >= 4 {
		size := int(binary.NativeEndian.Uint16(b[0:2]))
		if size < 4 || size > len(b) {
			return nil, false
		}
		if binary.NativeEndian.Uint16(b[2:4])&nlaTypeMask == kind {
			return b[4:size], true
		}
		next := (size + 3) &^ 3
		if next >= len(b) {
			return nil, false
		}
		b = b[next:]
	}
	return nil, false
}
