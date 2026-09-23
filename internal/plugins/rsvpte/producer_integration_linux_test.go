//go:build integration && linux

// Design: docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md -- native producer proof.
//
// Three complete Ze daemons run in separate network namespaces. H (head-end and
// PLR) reaches M (merge point) over primary, bypass-a and bypass-b veth pairs; M
// reaches E (egress) over tail. Native OSPF on H/M supplies node/address identity,
// not a computed TE path. All RSVP paths come from the configuration below.
//
// Prerequisites: a matching Linux Ze daemon passed with -rsvp-producer-daemon,
// iproute2, CAP_SYS_ADMIN, CAP_NET_ADMIN, CAP_NET_RAW, and CONFIG_NET_NS,
// CONFIG_VETH, CONFIG_PACKET, CONFIG_IP_MULTIPLE_TABLES, CONFIG_MPLS_ROUTING and
// CONFIG_MPLS_IPTUNNEL in Ze's runtime kernel. The daemon needs rsvp-te, ospf,
// interface/netlink, fib-kernel, sysctl, SSH, and external SDK plugin support.
// The test binary itself is its read-only external event observer; both binaries
// MUST be accessible inside the guest. No test callback acknowledges a FIB write.
//
// Run the compiled test in the runtime-kernel guest with:
//
//	rsvpte.test -test.v -test.run '^TestRSVPNativeProducer$' -test.timeout 4m \
//	  -rsvp-producer-daemon /absolute/path/to/ze
//
// The caller MUST build/stage both binaries. An unavailable prerequisite is a
// reported skip, never evidence that the producer or dataplane passed.
package rsvpte

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

var (
	rsvpProducerDaemon   = flag.String("rsvp-producer-daemon", "", "Matching native Ze daemon for the RSVP producer test")
	rsvpProducerObserver = flag.String("rsvp-producer-observer", "", "Internal child observer output path")
)

const (
	rsvpProducerPort              = 49001
	rsvpProducerWait              = 20 * time.Second
	rsvpProducerRefreshMultiplier = 20
)

type rsvpProducerNode struct {
	name    string
	ns      netns.NsHandle
	routes  *netlink.Handle
	id      netip.Addr
	links   []rsvpProducerInterface
	dir     string
	config  string
	command *exec.Cmd
	exited  chan struct{}
	exitErr error
	log     *os.File
	client  *ssh.Client
	control net.Conn
}

type rsvpProducerInterface struct {
	name    string
	address netip.Addr
	peer    netip.Addr
	link    netlink.Link
	capture int
}

type rsvpProducerFrame struct {
	node  string
	link  string
	frame []byte
}

type rsvpProducerLab struct {
	nodes               []*rsvpProducerNode
	head, merge, egress *rsvpProducerNode
	mu                  sync.Mutex
	frames              []rsvpProducerFrame
	captureErr          error
	stop                chan struct{}
	workers             sync.WaitGroup
}

type rsvpProducerSession struct {
	Endpoint string `json:"tunnel-endpoint"`
	TunnelID uint16 `json:"tunnel-id"`
	LSPID    uint16 `json:"lsp-id"`
	Sender   string `json:"sender-address"`
	State    string `json:"state"`
	Role     string `json:"role"`
	InLabel  uint32 `json:"in-label"`
	OutLabel uint32 `json:"out-label"`
}

type rsvpProducerRoute struct {
	Prefix   string `json:"prefix,omitempty"`
	InLabel  uint32 `json:"in-label,omitempty"`
	Labels   []int  `json:"labels,omitempty"`
	Table    int    `json:"table"`
	Link     int    `json:"link"`
	Protocol int    `json:"protocol"`
	MTU      int    `json:"mtu"`
}

type rsvpProducerObservation struct {
	Ready  bool                `json:"ready,omitempty"`
	Event  lSPEvent            `json:"event"`
	Routes []rsvpProducerRoute `json:"routes,omitempty"`
	Error  string              `json:"error,omitempty"`
}

// TestRSVPNativeProducer exercises configured strict/loose signaling, the real
// shared bus/native FIB acknowledgment, rejection, MPLS delivery, facility repair,
// protected control traffic, isolated withdrawal, and daemon shutdown.
func TestRSVPNativeProducer(t *testing.T) {
	lab := newRSVPProducerLab(t)
	h, m, e := lab.head, lab.merge, lab.egress
	for _, node := range lab.nodes {
		node.start(t, node.configuration(t, ""))
	}
	// Three Full adjacencies are a positive control for the native identity
	// producer. The protected MP router-id is not an interface next hop.
	for _, node := range []*rsvpProducerNode{h, m} {
		rsvpProducerUntil(t, "native OSPF adjacencies on "+node.name, func() bool {
			body, err := node.show("show ospf neighbor")
			return err == nil && bytes.Count(bytes.ToLower(body), []byte(`"full"`)) == 3
		})
	}

	// Occupy the first egress in-label with a foreign native route. This is an
	// actual kernel EEXIST refusal, not a substitute forwarding acknowledgment.
	foreignLabel := firstDynamicLabel
	foreign := &netlink.Route{Family: unix.AF_MPLS, MPLSDst: &foreignLabel,
		Protocol: 100, LinkIndex: e.iface("tail").link.Attrs().Index,
		NewDst: &netlink.MPLSDestination{Labels: []int{900}},
		Via:    &netlink.Via{AddrFamily: unix.AF_INET, Addr: m.iface("tail").address.AsSlice()}}
	if err := e.routes.RouteAdd(foreign); err != nil {
		t.Fatalf("install foreign in-label: %v", err)
	}
	mark := lab.mark()
	h.reload(t, h.configuration(t, rsvpProducerTunnel("denied", 91, e.id, "strict", true)))
	lab.awaitMessage(t, mark, "m", "tail", MsgTypePathErr, 91, nil, nil)
	lab.awaitMessages(t, mark, "m", "primary", MsgTypePath, 91, 2)
	for _, node := range lab.nodes {
		node.requireNoUp(t, 91)
	}
	lab.requireNoMessage(t, mark, MsgTypeResv, 91)
	rows, err := e.routes.RouteList(nil, unix.AF_MPLS)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(rows, func(r netlink.Route) bool {
		return r.MPLSDst != nil && *r.MPLSDst == foreignLabel && r.Protocol == 100
	}) {
		t.Fatal("rejected egress installation replaced the foreign in-label")
	}
	h.reload(t, h.configuration(t, ""))
	if err := e.routes.RouteDel(foreign); err != nil {
		t.Fatal(err)
	}

	// A loose hop beyond M expands through the native route. A separate strict
	// path names every adjacent abstract node. Neither asks a CSPF to find a path.
	looseEndpoint := netip.MustParseAddr("198.18.3.2")
	h.reload(t, h.configuration(t, rsvpProducerTunnel("loose", 92, looseEndpoint, "loose", false)))
	loose := h.waitUp(t, 92)
	looseM, looseE := m.waitUp(t, 92), e.waitUp(t, 92)
	h.requireUpRoute(t, loose, 0)
	m.requireUpRoute(t, looseM, 0)
	e.requireUpRoute(t, looseE, 0)
	lab.deliver(t, looseEndpoint, 0, "loose-native", "primary", []uint32{loose.OutLabel}, looseE.InLabel)
	h.reload(t, h.configuration(t, ""))
	h.waitAbsent(t, 92)

	bypasses := rsvpProducerBypass("a", "10.12.0.2") + rsvpProducerBypass("b", "10.13.0.2")
	protected := rsvpProducerTunnel("protected", 93, e.id, "strict", true)
	h.reload(t, h.configuration(t, bypasses+protected))
	aID := bypassTunnelIDBase | bypassNameHash("a")
	bID := bypassTunnelIDBase | bypassNameHash("b")
	a, b, primary := h.waitUp(t, aID), h.waitUp(t, bID), h.waitUp(t, 93)
	primaryM, primaryE := m.waitUp(t, 93), e.waitUp(t, 93)
	aTable, bTable := uint32(0x5a000000)|uint32(aID), uint32(0x5a000000)|uint32(bID)
	if a.OutLabel == b.OutLabel {
		t.Fatal("two live same-MP bypasses advertised the same local label")
	}
	for _, check := range []struct {
		session rsvpProducerSession
		table   uint32
	}{{a, aTable}, {b, bTable}, {primary, 0}} {
		h.requireUpRoute(t, check.session, check.table)
	}
	m.requireUpRoute(t, primaryM, 0)
	e.requireUpRoute(t, primaryE, 0)
	h.requirePush(t, m.id, aTable, "bypass-a", []uint32{a.OutLabel})
	h.requirePush(t, m.id, bTable, "bypass-b", []uint32{b.OutLabel})
	lab.deliver(t, m.id, aTable, "context-a", "bypass-a", []uint32{a.OutLabel}, 0)
	lab.deliver(t, m.id, bTable, "context-b", "bypass-b", []uint32{b.OutLabel}, 0)
	lab.deliver(t, e.id, 0, "primary-native", "primary", []uint32{primary.OutLabel}, primaryE.InLabel)

	// Hold the replacement's unmarked explicit-route lookup until repair is
	// observed. Direct-neighbor bypass refreshes and marked carriage remain clear.
	hold := netlink.NewRule()
	hold.Family = unix.AF_INET
	hold.Priority = 1
	hold.Type = unix.RTN_BLACKHOLE
	mask := ^uint32(0)
	hold.Mask = &mask
	hold.Dst = &net.IPNet{IP: net.IP(m.id.AsSlice()), Mask: net.CIDRMask(32, 32)}
	if err := h.routes.RuleAdd(hold); err != nil {
		t.Fatalf("hold ordinary replacement signaling: %v", err)
	}

	mark = lab.mark()
	repairMark := mark
	if err := h.routes.LinkSetDown(h.iface("primary").link); err != nil {
		t.Fatal(err)
	}
	// The ordinary route remains usable through b. A stale a mark MUST NOT
	// inherit it, and repaired control/data MUST still choose a.
	h.route(t, "198.18.2.0/24", "bypass-b", true)
	h.route(t, "198.18.3.0/24", "bypass-b", true)
	rsvpProducerUntil(t, "native link-down facility repair", func() bool {
		body, showErr := h.show("show rsvp-te fast-reroute")
		if showErr != nil {
			return false
		}
		var rows []struct {
			TunnelID uint16 `json:"tunnel-id"`
			InUse    bool   `json:"protection-in-use"`
		}
		if err := json.Unmarshal(body, &rows); err != nil {
			t.Fatalf("fast-reroute JSON: %v: %s", err, body)
		}
		return slices.ContainsFunc(rows, func(row struct {
			TunnelID uint16 `json:"tunnel-id"`
			InUse    bool   `json:"protection-in-use"`
		}) bool {
			return row.TunnelID == 93 && row.InUse
		})
	})
	backupPath := lab.awaitMessage(t, mark, "m", "bypass-a", MsgTypePath, 93, []uint32{a.OutLabel}, nil)
	backup := rsvpProducerDecode(t, backupPath.frame)
	if backup.SenderTemplate.SenderAddr == h.id {
		t.Fatal("head-end repair reused its original sender address")
	}
	if !slices.ContainsFunc(h.links, func(link rsvpProducerInterface) bool {
		return link.address == backup.SenderTemplate.SenderAddr
	}) {
		t.Fatalf("repair sender %s is not assigned to H", backup.SenderTemplate.SenderAddr)
	}
	aliasReply := lab.awaitMessage(t, mark, "h", "bypass-a", MsgTypeResv, 93, nil, nil)
	_, aliasIP := rsvpWireIP(aliasReply.frame)
	if netip.AddrFrom4([4]byte(aliasIP[12:16])) != m.iface("bypass-a").address {
		t.Fatal("MP did not reply from the selected alternate interface")
	}
	h.requirePush(t, e.id, 0, "bypass-a", []uint32{a.OutLabel, primaryM.InLabel})
	lab.deliver(t, e.id, 0, "repaired-native", "bypass-a", []uint32{a.OutLabel, primaryM.InLabel}, primaryE.InLabel)

	// Add only RESV_CONFIRM to the captured, genuine MP reservation. Its label,
	// FLOWSPEC, filter and source remain the values negotiated by the daemons.
	// This supplies an external request, never forwarding state or an ACK.
	mark = lab.mark()
	lab.requestConfirmation(t, aliasReply)
	lab.awaitMessage(t, mark, "m", "bypass-a", MsgTypeResvConf, 93, []uint32{a.OutLabel}, nil)
	lab.awaitMessage(t, mark, "e", "tail", MsgTypeResvConf, 93, nil, nil)
	lab.requireNoSelectedControl(t, mark, "bypass-b", 93, primary.LSPID)
	if err := h.routes.RuleDel(hold); err != nil {
		t.Fatalf("release ordinary replacement signaling: %v", err)
	}

	// Automatic make-before-break retires the protected generation before
	// configuration withdrawal. Its tear must still use the selected bypass.
	lab.awaitMessage(t, repairMark, "m", "bypass-a", MsgTypePathTear, 93, []uint32{a.OutLabel}, &backup.SenderTemplate)
	var replacement rsvpProducerSession
	rsvpProducerUntil(t, "replacement generation established", func() bool {
		for _, row := range h.sessions(t) {
			if row.TunnelID == 93 && row.State == "up" && row.LSPID != primary.LSPID && row.Sender == h.id.String() {
				replacement = row
				return true
			}
		}
		return false
	})
	h.requirePush(t, e.id, 0, "bypass-b", []uint32{replacement.OutLabel})
	retiring := []struct {
		node    *rsvpProducerNode
		session rsvpProducerSession
	}{{node: m}, {node: e}}
	rsvpProducerUntil(t, "replacement generation at transit and egress", func() bool {
		for i := range retiring {
			found := false
			for _, row := range retiring[i].node.sessions(t) {
				if row.TunnelID == replacement.TunnelID && row.LSPID == replacement.LSPID &&
					row.Sender == replacement.Sender && row.State == "up" && row.InLabel != 0 {
					retiring[i].session = row
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	})
	for _, at := range retiring {
		at.node.requireUpRoute(t, at.session, 0)
	}
	lab.deliver(t, e.id, 0, "replacement-native", "bypass-b", []uint32{replacement.OutLabel}, retiring[1].session.InLabel)

	// Observe a fresh PATH at both downstream nodes. Label removal must finish
	// before either advertised soft-state lifetime can expire.
	refreshStart := time.Now()
	mark = lab.mark()
	var softDeadline time.Time
	replacementSender := senderTemplateIPv4{SenderAddr: h.id, LSPID: replacement.LSPID}
	for _, at := range []struct{ node, link string }{{"m", "bypass-b"}, {"e", "tail"}} {
		fresh := lab.awaitMessage(t, mark, at.node, at.link, MsgTypePath, 93, nil, &replacementSender)
		msg := rsvpProducerDecode(t, fresh.frame)
		if !msg.HasTimeValues || msg.TimeValues.RefreshPeriod == 0 {
			t.Fatalf("fresh replacement PATH in %s: %+v", at.node, msg)
		}
		deadline := refreshStart.Add(time.Duration(msg.TimeValues.RefreshPeriod) * time.Millisecond * rsvpProducerRefreshMultiplier)
		if softDeadline.IsZero() || deadline.Before(softDeadline) {
			softDeadline = deadline
		}
	}

	// Withdraw the surviving generation while both contexts remain.
	// Then removing a alone must leave b delivering packets.
	mark = lab.mark()
	h.reload(t, h.configuration(t, bypasses))
	for _, at := range []struct{ node, link string }{{"m", "bypass-b"}, {"e", "tail"}} {
		lab.awaitMessage(t, mark, at.node, at.link, MsgTypePathTear, 93, nil, &replacementSender)
	}
	h.waitAbsent(t, 93)
	rsvpProducerUntil(t, "withdrawn transit and egress kernel labels", func() bool {
		for _, at := range retiring {
			routes, err := at.node.routes.RouteList(nil, unix.AF_MPLS)
			if err != nil {
				t.Fatalf("withdrawn label lookup in %s: %v", at.node.name, err)
			}
			for _, route := range routes {
				if route.MPLSDst != nil && uint32(*route.MPLSDst) == at.session.InLabel {
					return false
				}
			}
		}
		return true
	})
	if !time.Now().Before(softDeadline) {
		t.Fatal("downstream label removal did not precede soft-state expiry")
	}
	lab.requireNoSelectedControl(t, mark, "bypass-a", 93, replacement.LSPID)
	h.reload(t, h.configuration(t, rsvpProducerBypass("b", "10.13.0.2")))
	h.waitAbsent(t, aID)
	h.requirePush(t, m.id, bTable, "bypass-b", []uint32{b.OutLabel})
	lab.deliver(t, m.id, bTable, "b-survives-reload", "bypass-b", []uint32{b.OutLabel}, 0)
	lab.requireStaleBlocked(t, aTable, "a-withdrawn")
	lab.deliver(t, m.id, 0, "ordinary-positive-control", "bypass-b", nil, 0)

	// Stop the actual producer/owner lifecycle, not only an engine object. Its
	// surviving stale marks still cannot enter the ordinary b route.
	h.stopDaemon(t)
	lab.requireStaleBlocked(t, aTable, "a-after-shutdown")
	lab.requireStaleBlocked(t, bTable, "b-after-shutdown")
	lab.deliver(t, m.id, 0, "ordinary-after-shutdown", "bypass-b", nil, 0)
}

func newRSVPProducerLab(t *testing.T) *rsvpProducerLab {
	t.Helper()
	if *rsvpProducerDaemon == "" {
		t.Skip("pass -rsvp-producer-daemon with the matching Linux Ze artifact; this is not a pass")
	}
	if !filepath.IsAbs(*rsvpProducerDaemon) {
		t.Fatal("-rsvp-producer-daemon must be absolute")
	}
	if _, err := os.Stat(*rsvpProducerDaemon); err != nil {
		t.Fatalf("daemon artifact: %v", err)
	}
	if _, err := exec.LookPath("ip"); err != nil {
		t.Skipf("namespace daemon launch requires iproute2: %v", err)
	}
	if _, err := os.Stat("/proc/sys/net/mpls/platform_labels"); err != nil {
		t.Skipf("runtime kernel needs MPLS routing and IP tunnel support loaded: %v", err)
	}
	lab := &rsvpProducerLab{stop: make(chan struct{})}
	for index, name := range []string{"h", "m", "e"} {
		node := &rsvpProducerNode{name: name, id: netip.AddrFrom4([4]byte{198, 18, byte(index + 1), 1}), dir: t.TempDir()}
		node.name = fmt.Sprintf("rsvp-%d-%s", os.Getpid(), name)
		createCtx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		output, err := exec.CommandContext(createCtx, "ip", "netns", "add", node.name).CombinedOutput()
		cancel()
		if err != nil {
			t.Skipf("network namespace creation requires CAP_SYS_ADMIN/CAP_NET_ADMIN: %v: %s", err, output)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if output, err := exec.CommandContext(ctx, "ip", "netns", "delete", node.name).CombinedOutput(); err != nil {
				t.Errorf("delete namespace %s: %v: %s", node.name, err, output)
			}
		})
		node.ns, err = netns.GetFromName(node.name)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := node.ns.Close(); err != nil {
				t.Error(err)
			}
		})
		node.routes = rsvpWireNetlink(t, node.ns)
		lo := rsvpWireFindLink(t, node.routes, "lo")
		if err := node.routes.AddrAdd(lo, &netlink.Addr{IPNet: rsvpWirePrefix(node.id)}); err != nil {
			t.Fatal(err)
		}
		lab.nodes = append(lab.nodes, node)
	}
	lab.head, lab.merge, lab.egress = lab.nodes[0], lab.nodes[1], lab.nodes[2]
	for i, name := range []string{"primary", "bypass-a", "bypass-b"} {
		lab.link(t, lab.head, lab.merge, name, byte(11+i))
	}
	lab.link(t, lab.merge, lab.egress, "tail", 14)
	lo := rsvpWireFindLink(t, lab.egress.routes, "lo")
	if err := lab.egress.routes.AddrAdd(lo, &netlink.Addr{IPNet: rsvpWirePrefix(netip.MustParseAddr("198.18.3.2"))}); err != nil {
		t.Fatal(err)
	}
	for _, node := range lab.nodes {
		rsvpProducerInNS(t, node.ns, func() {
			settings := map[string]string{
				"/proc/sys/net/ipv4/ip_forward":             "1",
				"/proc/sys/net/ipv4/conf/all/rp_filter":     "0",
				"/proc/sys/net/ipv4/conf/default/rp_filter": "0",
				// Local MPLS pop re-enters IPv4 on lo with a remote source.
				"/proc/sys/net/ipv4/conf/lo/rp_filter": "0",
				"/proc/sys/net/mpls/platform_labels":   "1048575",
				"/proc/sys/net/mpls/conf/lo/input":     "1",
			}
			for _, link := range node.links {
				settings["/proc/sys/net/mpls/conf/"+link.name+"/input"] = "1"
				settings["/proc/sys/net/ipv4/conf/"+link.name+"/rp_filter"] = "0"
			}
			for path, value := range settings {
				if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
					t.Fatalf("configure %s: %v", path, err)
				}
			}
			for i := range node.links {
				node.links[i].capture = rsvpWireCapture(t, node.links[i].link.Attrs().Index)
			}
		})
	}
	lab.head.route(t, "198.18.2.0/24", "primary", false)
	lab.head.route(t, "198.18.3.0/24", "primary", false)
	lab.merge.route(t, "198.18.1.0/24", "bypass-b", false)
	lab.merge.route(t, "198.18.3.0/24", "tail", false)
	lab.egress.route(t, "198.18.1.0/24", "tail", false)
	lab.egress.route(t, "198.18.2.0/24", "tail", false)
	lab.workers.Go(lab.capture)
	t.Cleanup(func() { close(lab.stop); lab.workers.Wait() })
	return lab
}

func (lab *rsvpProducerLab) link(t *testing.T, left, right *rsvpProducerNode, name string, subnet byte) {
	t.Helper()
	veth := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: name, MTU: 1500}, PeerName: name + "-peer", PeerNamespace: netlink.NsFd(right.ns), PeerMTU: 1500}
	if err := left.routes.LinkAdd(veth); err != nil {
		t.Fatal(err)
	}
	peer := rsvpWireFindLink(t, right.routes, name+"-peer")
	if err := right.routes.LinkSetName(peer, name); err != nil {
		t.Fatal(err)
	}
	for side, node := range []*rsvpProducerNode{left, right} {
		link := rsvpWireFindLink(t, node.routes, name)
		address := netip.AddrFrom4([4]byte{10, subnet, 0, byte(side + 1)})
		rsvpWireAddress(t, node.routes, link, address)
		node.links = append(node.links, rsvpProducerInterface{name: name, address: address,
			peer: netip.AddrFrom4([4]byte{10, subnet, 0, byte(2 - side)}), link: link})
	}
	rsvpWireNeighbor(t, left.routes, left.iface(name).link, right.iface(name).address, right.iface(name).link.Attrs().HardwareAddr)
	rsvpWireNeighbor(t, right.routes, right.iface(name).link, left.iface(name).address, left.iface(name).link.Attrs().HardwareAddr)
}

func (node *rsvpProducerNode) iface(name string) *rsvpProducerInterface {
	for i := range node.links {
		if node.links[i].name == name {
			return &node.links[i]
		}
	}
	panic("BUG: producer topology names an absent interface")
}

func (node *rsvpProducerNode) route(t *testing.T, prefix, link string, replace bool) {
	t.Helper()
	p := netip.MustParsePrefix(prefix)
	route := &netlink.Route{Dst: &net.IPNet{IP: p.Addr().AsSlice(), Mask: net.CIDRMask(p.Bits(), 32)},
		LinkIndex: node.iface(link).link.Attrs().Index, Gw: node.iface(link).peer.AsSlice(), Protocol: 100}
	var err error
	if replace {
		err = node.routes.RouteReplace(route)
	} else {
		err = node.routes.RouteAdd(route)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// rsvpProducerInNS creates namespace-bound sockets only; workers use those FDs
// after the calling thread has returned to its original namespace.
func rsvpProducerInNS(t *testing.T, ns netns.NsHandle, operation func()) {
	t.Helper()
	runtime.LockOSThread()
	origin, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	defer func() {
		if err := netns.Set(origin); err != nil {
			t.Errorf("restore namespace: %v", err)
			return
		}
		if err := origin.Close(); err != nil {
			t.Error(err)
		}
		runtime.UnlockOSThread()
	}()
	if err := netns.Set(ns); err != nil {
		t.Fatal(err)
	}
	operation()
}

func rsvpProducerTunnel(name string, id uint16, endpoint netip.Addr, mode string, protect bool) string {
	var config strings.Builder
	fmt.Fprintf(&config, "tunnel %s { destination %s; tunnel-id %d; bandwidth 1000;\n", name, endpoint, id)
	if mode == "strict" {
		fmt.Fprintln(&config, "explicit-route 1 { address 198.18.2.1/32; type strict; }")
		fmt.Fprintf(&config, "explicit-route 2 { address %s/32; type loose; }\n", endpoint)
	} else {
		fmt.Fprintf(&config, "explicit-route 1 { address %s/32; type loose; }\n", endpoint)
	}
	if protect {
		fmt.Fprintln(&config, "fast-reroute { backup facility; node-protection false; }")
	}
	fmt.Fprintln(&config, "}")
	return config.String()
}

func rsvpProducerBypass(name, hop string) string {
	return fmt.Sprintf("bypass %s { merge-point 198.18.2.1; explicit-route 1 { address %s/32; type strict; } }\n", name, hop)
}

func (node *rsvpProducerNode) configuration(t *testing.T, tunnels string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	observer := self + " -test.run=^TestRSVPNativeProducerObserver$ -rsvp-producer-observer=" + filepath.Join(node.dir, "events.jsonl")
	var config strings.Builder
	fmt.Fprintf(&config, "plugin { external native-proof { run %q; encoder json; } }\n", observer)
	fmt.Fprintln(&config, `fib { kernel { flush-on-stop true; } }
system {
 authentication { user admin {
  password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO";
  profile [ admin ];
 } }
 authorization { profile admin { run { default-action allow; } edit { default-action allow; } } }
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port 2222; } } }`)
	fmt.Fprintf(&config, "rsvp-te { router-id %s; refresh-period 1; refresh-multiplier %d;\n", node.id, rsvpProducerRefreshMultiplier)
	for _, link := range node.links {
		fmt.Fprintf(&config, "interface %s { address %s/24; max-bandwidth 1e9; max-reservable-bandwidth 1e9; }\n", link.name, link.address)
	}
	config.WriteString(tunnels)
	fmt.Fprintln(&config, "}")
	if len(node.links) > 1 {
		fmt.Fprintf(&config, "ospf { router-id %s; areas { area 0.0.0.0 { } } interfaces {\n", node.id)
		for _, link := range node.links {
			if link.name == "tail" {
				continue
			}
			fmt.Fprintf(&config, "interface %s { area 0.0.0.0; network-type point-to-point; hello-interval 1; dead-interval 4; }\n", link.name)
		}
		fmt.Fprintln(&config, "} }")
	}
	return config.String()
}

func (node *rsvpProducerNode) start(t *testing.T, config string) {
	t.Helper()
	node.config = filepath.Join(node.dir, "ze.conf")
	if err := os.WriteFile(node.config, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	var err error
	node.log, err = os.Create(filepath.Join(node.dir, "daemon.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := node.log.Close(); err != nil {
			t.Error(err)
		}
	})
	// The test context is canceled before cleanup. Let stopDaemon own the
	// shutdown signal so cancellation cannot force a second-signal exit.
	node.command = exec.Command("ip", "netns", "exec", node.name, *rsvpProducerDaemon, "start", node.config)
	node.command.Dir = node.dir
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(strings.ToLower(strings.ReplaceAll(key, "_", ".")), "ze.") {
			continue
		}
		node.command.Env = append(node.command.Env, item)
	}
	node.command.Env = append(node.command.Env, "ze.config.dir="+node.dir,
		"ze.log.rsvp.te=debug", "ze.log.fib.kernel=debug")
	node.command.Stdout, node.command.Stderr = node.log, node.log
	if err := node.command.Start(); err != nil {
		t.Fatal(err)
	}
	node.exited = make(chan struct{})
	go func() { node.exitErr = node.command.Wait(); close(node.exited) }()
	t.Cleanup(func() {
		node.stopDaemon(t)
		if t.Failed() {
			body, err := os.ReadFile(filepath.Join(node.dir, "daemon.log"))
			if err != nil {
				t.Error(err)
			} else {
				t.Logf("%s daemon output:\n%s", node.name, body)
			}
		}
	})
	rsvpProducerUntil(t, "daemon and observer readiness in "+node.name, func() bool {
		select {
		case <-node.exited:
			t.Fatalf("%s exited: %v", node.name, node.exitErr)
		default:
		}
		rows := node.observations(t)
		return len(rows) != 0 && rows[0].Ready
	})
	rsvpProducerUntil(t, "SSH readiness in "+node.name, func() bool {
		node.client, err = node.connect(t)
		return err == nil
	})
	t.Cleanup(func() {
		if node.client != nil {
			_ = node.client.Close()
		}
	})
}

func (node *rsvpProducerNode) connect(t *testing.T) (*ssh.Client, error) {
	var connection net.Conn
	var connectErr error
	rsvpProducerInNS(t, node.ns, func() {
		fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
		if err != nil {
			connectErr = err
			return
		}
		if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_SNDTIMEO, &unix.Timeval{Sec: 3}); err != nil {
			_ = unix.Close(fd)
			connectErr = err
			return
		}
		if err := unix.Connect(fd, &unix.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}, Port: 2222}); err != nil {
			_ = unix.Close(fd)
			connectErr = err
			return
		}
		file := os.NewFile(uintptr(fd), "namespace-ssh")
		connection, connectErr = net.FileConn(file)
		if err := file.Close(); err != nil {
			connectErr = errors.Join(connectErr, err)
		}
	})
	if connectErr != nil {
		if connection != nil {
			_ = connection.Close()
		}
		return nil, connectErr
	}
	if err := connection.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		_ = connection.Close()
		return nil, err
	}
	client, channels, requests, err := ssh.NewClientConn(connection, "127.0.0.1:2222", &ssh.ClientConfig{
		User: "admin", Auth: []ssh.AuthMethod{ssh.Password("testpass")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // isolated test namespace
	})
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		_ = client.Close()
		return nil, err
	}
	node.control = connection
	return ssh.NewClient(client, channels, requests), nil
}

func (node *rsvpProducerNode) show(command string) (body []byte, err error) {
	if err := node.control.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, node.control.SetDeadline(time.Time{})) }()
	session, err := node.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Close() }()
	return session.Output(command + " | json")
}

func (node *rsvpProducerNode) reload(t *testing.T, config string) {
	t.Helper()
	before, err := os.ReadFile(filepath.Join(node.dir, "daemon.log"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(node.config, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := node.command.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	rsvpProducerUntil(t, "SIGHUP reload in "+node.name, func() bool {
		body, err := os.ReadFile(filepath.Join(node.dir, "daemon.log"))
		if err != nil {
			t.Fatal(err)
		}
		return bytes.Count(body, []byte("sighup reload complete")) > bytes.Count(before, []byte("sighup reload complete"))
	})
}

func (node *rsvpProducerNode) stopDaemon(t *testing.T) {
	t.Helper()
	if node.command == nil {
		return
	}
	select {
	case <-node.exited:
		if node.exitErr != nil {
			t.Errorf("%s shutdown: %v", node.name, node.exitErr)
		}
		return
	default:
	}
	if err := node.command.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Error(err)
	}
	select {
	case <-node.exited:
		if node.exitErr != nil {
			t.Errorf("%s shutdown: %v", node.name, node.exitErr)
		}
	case <-time.After(10 * time.Second):
		if err := node.command.Process.Kill(); err != nil {
			t.Error(err)
		}
		<-node.exited
		t.Errorf("%s did not finish native shutdown", node.name)
	}
}

func rsvpProducerUntil(t *testing.T, reason string, condition func() bool) {
	t.Helper()
	deadline := time.NewTimer(rsvpProducerWait)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		if condition() {
			return
		}
		select {
		case <-t.Context().Done():
			t.Fatalf("%s: %v", reason, t.Context().Err())
		case <-deadline.C:
			t.Fatalf("timed out waiting for %s", reason)
		case <-tick.C:
		}
	}
}

func (node *rsvpProducerNode) sessions(t *testing.T) []rsvpProducerSession {
	t.Helper()
	body, err := node.show("show rsvp-te session")
	if err != nil {
		t.Fatalf("session show in %s: %v: %s", node.name, err, body)
	}
	var rows []rsvpProducerSession
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("session JSON: %v: %s", err, body)
	}
	return rows
}

func (node *rsvpProducerNode) waitUp(t *testing.T, tunnel uint16) rsvpProducerSession {
	t.Helper()
	var found rsvpProducerSession
	rsvpProducerUntil(t, fmt.Sprintf("tunnel %d Up in %s", tunnel, node.name), func() bool {
		for _, row := range node.sessions(t) {
			if row.TunnelID == tunnel && row.State == "up" {
				found = row
				return true
			}
		}
		return false
	})
	return found
}

func (node *rsvpProducerNode) waitAbsent(t *testing.T, tunnel uint16) {
	t.Helper()
	rsvpProducerUntil(t, fmt.Sprintf("tunnel %d withdrawal in %s", tunnel, node.name), func() bool {
		return !slices.ContainsFunc(node.sessions(t), func(row rsvpProducerSession) bool { return row.TunnelID == tunnel })
	})
}

func (node *rsvpProducerNode) requireNoUp(t *testing.T, tunnel uint16) {
	t.Helper()
	for _, row := range node.sessions(t) {
		if row.TunnelID == tunnel && row.State == "up" {
			t.Fatalf("rejected tunnel %d is Up in %s", tunnel, node.name)
		}
	}
	for _, row := range node.observations(t) {
		if row.Event.TunnelID == tunnel {
			t.Fatalf("rejected tunnel %d emitted Up in %s", tunnel, node.name)
		}
	}
}

func (node *rsvpProducerNode) requirePush(t *testing.T, endpoint netip.Addr, table uint32, link string, labels []uint32) {
	t.Helper()
	routes, err := node.routes.RouteGetWithOptions(endpoint.AsSlice(), &netlink.RouteGetOptions{Mark: table, FIBMatch: true})
	if err != nil || len(routes) != 1 {
		t.Fatalf("native push lookup: %v: %+v", err, routes)
	}
	encap, ok := routes[0].Encap.(*netlink.MPLSEncap)
	want := make([]int, len(labels))
	for i, label := range labels {
		want[i] = int(label)
	}
	if !ok || !slices.Equal(encap.Labels, want) || routes[0].LinkIndex != node.iface(link).link.Attrs().Index {
		t.Fatalf("push lookup for %s mark %#x selected %+v, want %s labels %v", endpoint, table, routes, link, labels)
	}
	// FIBMatch returns the stored frame budget. Linux subtracts LWT label
	// headroom when it derives the inner IP limit, not when it stores RTAX_MTU.
	if routes[0].Protocol != rtproto.FIBKernel || routes[0].MTU != 1500 {
		t.Fatalf("push owner=%d frame MTU=%d, want owner=%d MTU=1500: %+v",
			routes[0].Protocol, routes[0].MTU, rtproto.FIBKernel, routes[0])
	}
}

func (node *rsvpProducerNode) requireUpRoute(t *testing.T, session rsvpProducerSession, table uint32) {
	t.Helper()
	rsvpProducerUntil(t, "native FIB snapshot at lsp-up", func() bool {
		for _, event := range node.observations(t) {
			if event.Event.TunnelID != session.TunnelID || event.Event.LSPID != session.LSPID {
				continue
			}
			for _, route := range event.Routes {
				if route.Protocol != rtproto.FIBKernel {
					continue
				}
				if session.Role == "ingress" {
					wantTable := int(table)
					if table == 0 {
						wantTable = unix.RT_TABLE_MAIN
					}
					if route.Prefix == session.Endpoint+"/32" && route.Table == wantTable && slices.Equal(route.Labels, []int{int(session.OutLabel)}) {
						return true
					}
				} else if route.InLabel == session.InLabel && session.InLabel != 0 {
					switch session.Role {
					case "transit":
						if session.OutLabel != 0 && slices.Equal(route.Labels, []int{int(session.OutLabel)}) &&
							route.Link == node.iface("tail").link.Attrs().Index {
							return true
						}
					case "egress":
						if len(route.Labels) == 0 && route.Link == rsvpWireFindLink(t, node.routes, "lo").Attrs().Index {
							return true
						}
					}
				}
			}
			t.Fatalf("tunnel %d advertised Up without its installed kernel route: %+v", session.TunnelID, event)
		}
		return false
	})
}

func (lab *rsvpProducerLab) capture() {
	var polls []unix.PollFd
	var owners []rsvpProducerFrame
	for _, node := range lab.nodes {
		for _, link := range node.links {
			polls = append(polls, unix.PollFd{Fd: int32(link.capture), Events: unix.POLLIN})
			owners = append(owners, rsvpProducerFrame{node: node.name[len(node.name)-1:], link: link.name})
		}
	}
	var buffer [2048]byte
	for {
		select {
		case <-lab.stop:
			return
		default:
		}
		_, err := unix.Poll(polls, 100)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			lab.mu.Lock()
			lab.captureErr = err
			lab.mu.Unlock()
			return
		}
		for i := range polls {
			if polls[i].Revents&unix.POLLIN == 0 {
				continue
			}
			n, address, err := unix.Recvfrom(int(polls[i].Fd), buffer[:], unix.MSG_DONTWAIT)
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
				continue
			}
			if err != nil {
				lab.mu.Lock()
				lab.captureErr = err
				lab.mu.Unlock()
				return
			}
			from, ok := address.(*unix.SockaddrLinklayer)
			if !ok || from.Pkttype == unix.PACKET_OUTGOING {
				continue
			}
			_, ip := rsvpWireIP(buffer[:n])
			if len(ip) < 20 {
				continue
			}
			if ip[9] != rsvpProtocol && ip[9] != unix.IPPROTO_UDP {
				continue
			}
			lab.mu.Lock()
			if len(lab.frames) == 16384 {
				lab.captureErr = errors.New("producer packet capture exceeded 16384 frames")
				lab.mu.Unlock()
				return
			}
			frame := owners[i]
			frame.frame = bytes.Clone(buffer[:n])
			lab.frames = append(lab.frames, frame)
			lab.mu.Unlock()
		}
	}
}

func (lab *rsvpProducerLab) mark() int { lab.mu.Lock(); defer lab.mu.Unlock(); return len(lab.frames) }

func (lab *rsvpProducerLab) since(t *testing.T, mark int) []rsvpProducerFrame {
	t.Helper()
	lab.mu.Lock()
	defer lab.mu.Unlock()
	if lab.captureErr != nil {
		t.Fatal(lab.captureErr)
	}
	return slices.Clone(lab.frames[mark:])
}

func rsvpProducerMessage(frame []byte) (*ParsedMessage, error) {
	_, ip := rsvpWireIP(frame)
	if len(ip) < 20 || ip[9] != rsvpProtocol {
		return nil, errors.New("not RSVP")
	}
	offset := int(ip[0]&15) * 4
	length := int(binary.BigEndian.Uint16(ip[2:4]))
	if offset < 20 || length > len(ip) || length < offset {
		return nil, errors.New("invalid captured IPv4 length")
	}
	return DecodeMessage(ip[offset:length])
}

func rsvpProducerDecode(t *testing.T, frame []byte) *ParsedMessage {
	t.Helper()
	message, err := rsvpProducerMessage(frame)
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func (lab *rsvpProducerLab) awaitMessage(t *testing.T, mark int, node, link string, kind uint8, tunnel uint16, labels []uint32, sender *senderTemplateIPv4) rsvpProducerFrame {
	t.Helper()
	var found rsvpProducerFrame
	rsvpProducerUntil(t, fmt.Sprintf("RSVP type %d tunnel %d at %s/%s", kind, tunnel, node, link), func() bool {
		for _, frame := range lab.since(t, mark) {
			if frame.node != node || frame.link != link {
				continue
			}
			message, err := rsvpProducerMessage(frame.frame)
			if err != nil || message.Header.MsgType != kind || message.Session.TunnelID != tunnel {
				continue
			}
			if sender != nil && (!message.HasSenderTemplate || message.SenderTemplate != *sender) {
				continue
			}
			stack, ip := rsvpWireIP(frame.frame)
			if !slices.Equal(stack, labels) {
				t.Fatalf("RSVP type %d selected labels %v, want %v", kind, stack, labels)
			}
			if kind == MsgTypePath || kind == MsgTypePathTear || kind == MsgTypeResvConf {
				if len(ip) < 24 || !bytes.Equal(ip[20:24], []byte{0x94, 4, 0, 0}) {
					t.Fatal("protected control packet lost Router Alert")
				}
			}
			found = frame
			return true
		}
		return false
	})
	return found
}

func (lab *rsvpProducerLab) awaitMessages(t *testing.T, mark int, node, link string, kind uint8, tunnel uint16, count int) {
	t.Helper()
	rsvpProducerUntil(t, "repeated native signaling after installation refusal", func() bool {
		seen := 0
		for _, frame := range lab.since(t, mark) {
			if frame.node != node || frame.link != link {
				continue
			}
			message, err := rsvpProducerMessage(frame.frame)
			if err == nil && message.Header.MsgType == kind && message.Session.TunnelID == tunnel {
				seen++
			}
		}
		return seen >= count
	})
}

func (lab *rsvpProducerLab) requireNoMessage(t *testing.T, mark int, kind uint8, tunnel uint16) {
	t.Helper()
	for _, frame := range lab.since(t, mark) {
		message, err := rsvpProducerMessage(frame.frame)
		if err == nil && message.Header.MsgType == kind && message.Session.TunnelID == tunnel {
			t.Fatalf("rejected reservation advertised at %s/%s", frame.node, frame.link)
		}
	}
}

func (lab *rsvpProducerLab) requireNoSelectedControl(t *testing.T, mark int, link string, tunnel, generation uint16) {
	t.Helper()
	for _, frame := range lab.since(t, mark) {
		if frame.node != "m" || frame.link != link {
			continue
		}
		message, err := rsvpProducerMessage(frame.frame)
		if err != nil || message.Session.TunnelID != tunnel {
			continue
		}
		matches := message.HasSenderTemplate && message.SenderTemplate.LSPID == generation
		for _, descriptor := range message.FlowDescriptors {
			for _, filter := range descriptor.Filters {
				matches = matches || filter.Filter.LSPID == generation
			}
		}
		if matches {
			t.Fatalf("RSVP tunnel %d LSP %d used %s", tunnel, generation, link)
		}
	}
}

func rsvpProducerSocket(t *testing.T, node *rsvpProducerNode, address netip.Addr, port int, mark uint32) int {
	t.Helper()
	fd := -1
	rsvpProducerInNS(t, node.ns, func() {
		var err error
		fd, err = unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, unix.IPPROTO_UDP)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := unix.Close(fd); err != nil {
				t.Error(err)
			}
		})
		if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_MARK, int(mark)); err != nil {
			t.Fatal(err)
		}
		if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: address.As4(), Port: port}); err != nil {
			t.Fatal(err)
		}
	})
	return fd
}

func (lab *rsvpProducerLab) deliver(t *testing.T, destination netip.Addr, table uint32, payload, link string, labels []uint32, tailLabel uint32) {
	t.Helper()
	receiver := lab.egress
	if destination == lab.merge.id {
		receiver = lab.merge
	}
	// Each datagram has its own dynamically assigned receiver port so a stale
	// datagram or a sibling probe cannot satisfy this exact-payload assertion.
	rx := rsvpProducerSocket(t, receiver, destination, 0, 0)
	address, err := unix.Getsockname(rx)
	if err != nil {
		t.Fatal(err)
	}
	port := address.(*unix.SockaddrInet4).Port
	tx := rsvpProducerSocket(t, lab.head, lab.head.id, 0, table)
	mark := lab.mark()
	if err := unix.Sendto(tx, []byte(payload), 0, &unix.SockaddrInet4{Addr: destination.As4(), Port: port}); err != nil {
		t.Fatalf("send %s: %v", payload, err)
	}
	poll := []unix.PollFd{{Fd: int32(rx), Events: unix.POLLIN}}
	if n, err := unix.Poll(poll, 3000); err != nil || n != 1 {
		t.Fatalf("inner delivery %s: poll=%d err=%v", payload, n, err)
	}
	var buffer [2048]byte
	n, from, err := unix.Recvfrom(rx, buffer[:], 0)
	if err != nil {
		t.Fatalf("inner delivery %s: %v", payload, err)
	}
	if n < 0 || n > len(buffer) {
		t.Fatalf("inner delivery %s: invalid receive length %d", payload, n)
	}
	if !bytes.Equal(buffer[:n], []byte(payload)) {
		t.Fatalf("inner delivery %s: %q", payload, buffer[:n])
	}
	source, ok := from.(*unix.SockaddrInet4)
	if !ok || source.Addr != lab.head.id.As4() {
		t.Fatal("inner source changed across MPLS")
	}
	rsvpProducerUntil(t, "captured inner datagram "+payload, func() bool {
		mergeSeen, tailSeen := false, receiver == lab.merge
		for _, frame := range lab.since(t, mark) {
			atMerge := frame.node == "m" && frame.link == link
			atTail := receiver == lab.egress && frame.node == "e" && frame.link == "tail"
			if !atMerge && !atTail {
				continue
			}
			stack, ip := rsvpWireIP(frame.frame)
			if len(ip) < 28 || ip[9] != unix.IPPROTO_UDP {
				continue
			}
			header := int(ip[0]&15) * 4
			offset := header + 8
			length := int(binary.BigEndian.Uint16(ip[2:4]))
			if header < 20 || offset > length || length > len(ip) ||
				!bytes.Equal(ip[offset:length], []byte(payload)) ||
				netip.AddrFrom4([4]byte(ip[12:16])) != lab.head.id ||
				netip.AddrFrom4([4]byte(ip[16:20])) != destination ||
				int(binary.BigEndian.Uint16(ip[header:header+2])) != source.Port ||
				int(binary.BigEndian.Uint16(ip[header+2:header+4])) != port {
				continue
			}
			wantLabels := labels
			wantMAC := lab.merge.iface(link).link.Attrs().HardwareAddr
			if atTail {
				wantLabels = []uint32{tailLabel}
				wantMAC = lab.egress.iface("tail").link.Attrs().HardwareAddr
			}
			if !slices.Equal(stack, wantLabels) {
				t.Fatalf("%s at %s/%s carried labels %v, want %v", payload, frame.node, frame.link, stack, wantLabels)
			}
			if !bytes.Equal(frame.frame[:6], wantMAC) {
				t.Fatal("inner frame reached the wrong link MAC")
			}
			mergeSeen = mergeSeen || atMerge
			tailSeen = tailSeen || atTail
		}
		return mergeSeen && tailSeen
	})
}

func (lab *rsvpProducerLab) requireStaleBlocked(t *testing.T, table uint32, payload string) {
	t.Helper()
	tx := rsvpProducerSocket(t, lab.head, lab.head.id, 0, table)
	mark := lab.mark()
	err := unix.Sendto(tx, []byte(payload), 0, &unix.SockaddrInet4{Addr: lab.merge.id.As4(), Port: rsvpProducerPort})
	if !errors.Is(err, unix.ENETUNREACH) {
		t.Fatalf("stale context %#x send error=%v, want ENETUNREACH", table, err)
	}
	// A positive-control ordinary datagram traverses the same destination/link,
	// giving the capture worker a packet barrier rather than a sleep.
	lab.deliver(t, lab.merge.id, 0, payload+"-control", "bypass-b", nil, 0)
	for _, frame := range lab.since(t, mark) {
		_, ip := rsvpWireIP(frame.frame)
		if len(ip) < 28 || ip[9] != unix.IPPROTO_UDP {
			continue
		}
		offset, length := int(ip[0]&15)*4+8, int(binary.BigEndian.Uint16(ip[2:4]))
		if offset <= length && length <= len(ip) && bytes.Equal(ip[offset:length], []byte(payload)) {
			t.Fatal("stale selected traffic escaped through ordinary routing")
		}
	}
}

func (lab *rsvpProducerLab) requestConfirmation(t *testing.T, captured rsvpProducerFrame) {
	t.Helper()
	_, ip := rsvpWireIP(captured.frame)
	offset, length := int(ip[0]&15)*4, int(binary.BigEndian.Uint16(ip[2:4]))
	payload := bytes.Clone(ip[offset:length])
	var confirm [8]byte
	encodeResvConfirm(confirm[:], lab.egress.id)
	// RESV_CONFIRM precedes STYLE; all captured descriptors remain byte-exact.
	insert := rsvpHdrLen
	for insert+objHdrLen <= len(payload) {
		if payload[insert+2] == ClassStyle {
			break
		}
		insert += int(binary.BigEndian.Uint16(payload[insert : insert+2]))
	}
	payload = slices.Insert(payload, insert, confirm[:]...)
	binary.BigEndian.PutUint16(payload[6:8], uint16(len(payload)))
	payload[2], payload[3] = 0, 0
	binary.BigEndian.PutUint16(payload[2:4], internetChecksum(payload))
	fd := -1
	rsvpProducerInNS(t, lab.merge.ns, func() {
		var err error
		fd, err = unix.Socket(unix.AF_INET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, rsvpProtocol)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := unix.Close(fd); err != nil {
				t.Error(err)
			}
		})
		if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: [4]byte(ip[12:16])}); err != nil {
			t.Fatal(err)
		}
	})
	if err := unix.Sendto(fd, payload, 0, &unix.SockaddrInet4{Addr: [4]byte(ip[16:20])}); err != nil {
		t.Fatal(err)
	}
}

// TestRSVPNativeProducerObserver is an external SDK plugin in the real daemon's
// namespace. It only subscribes and reads native routes; it never installs a
// route, emits an event, or acknowledges an MPLS owner request.
func TestRSVPNativeProducerObserver(t *testing.T) {
	if *rsvpProducerObserver == "" {
		t.Skip("child observer entry point")
	}
	if err := rsvpProducerObserve(*rsvpProducerObserver); err != nil {
		t.Fatal(err)
	}
}

func rsvpProducerObserve(path string) error {
	plugin, err := sdk.NewFromTLSEnv(env.Get("ze.plugin.name"))
	if err != nil {
		return err
	}
	defer func() { _ = plugin.Close() }()
	handle, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	defer handle.Close()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	var mu sync.Mutex
	write := func(row rsvpProducerObservation) error {
		mu.Lock()
		defer mu.Unlock()
		return json.NewEncoder(file).Encode(row)
	}
	plugin.SetStartupSubscriptionsIn(Namespace, []string{EventLSPUp}, []string{"*"}, "full")
	plugin.OnAllPluginsReady(func() error { return write(rsvpProducerObservation{Ready: true}) })
	plugin.OnEvent(func(body string) error {
		var row rsvpProducerObservation
		if err := json.Unmarshal([]byte(body), &row.Event); err != nil {
			row.Error = err.Error()
			return write(row)
		}
		if row.Event.State != "up" {
			row.Error = "observer received a non-Up event: " + body
			return write(row)
		}
		for _, family := range []int{unix.AF_INET, unix.AF_MPLS} {
			routes, err := handle.RouteListFiltered(family, &netlink.Route{Table: unix.RT_TABLE_UNSPEC}, netlink.RT_FILTER_TABLE)
			if err != nil {
				row.Error = err.Error()
				return write(row)
			}
			for _, route := range routes {
				item := rsvpProducerRoute{Table: route.Table, Link: route.LinkIndex, Protocol: int(route.Protocol), MTU: route.MTU}
				if route.Dst != nil {
					item.Prefix = route.Dst.String()
				}
				if route.MPLSDst != nil {
					item.InLabel = uint32(*route.MPLSDst)
				}
				if encap, ok := route.Encap.(*netlink.MPLSEncap); ok {
					item.Labels = encap.Labels
				}
				if destination, ok := route.NewDst.(*netlink.MPLSDestination); ok {
					item.Labels = destination.Labels
				}
				row.Routes = append(row.Routes, item)
			}
		}
		return write(row)
	})
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	return plugin.Run(ctx, sdk.Registration{})
}

func (node *rsvpProducerNode) observations(t *testing.T) []rsvpProducerObservation {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(node.dir, "events.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	// Each JSON record is one write. Ignore only a final incomplete write while
	// the live observer is appending; malformed complete records are failures.
	last := bytes.LastIndexByte(body, '\n')
	if last < 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(body[:last+1]))
	var rows []rsvpProducerObservation
	for {
		var row rsvpProducerObservation
		if err := decoder.Decode(&row); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if row.Error != "" {
			t.Fatal(row.Error)
		}
		rows = append(rows, row)
	}
	return rows
}
