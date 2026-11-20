// Design: docs/labs/pppoe-interop.md -- a RADIUS Framed-Route through a native VPP session.
// Related: product_check.go -- the shared product dial, session and teardown helpers.
// Related: product_state.go -- productOwnedPath, the source-specific native path oracle.
package testvpppppoe

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	radiusPeer = "radius"
	// radiusHost is FreeRADIUS's host number on the scenario network.
	radiusHost = 5
	// radiusNASHost numbers the macvlan Ze's RADIUS client binds in the VPP
	// peer. prepareProduct gives eth0's address to VPP and flushes it from
	// Linux, so Ze needs a Linux address of its own on the same segment. The
	// macvlan's own MAC keeps its frames out of VPP's L3 input, which drops a
	// destination MAC that is not the AF_PACKET interface's.
	radiusNASHost   = 6
	radiusNASDevice = "rad0"
	radiusNASMAC    = "02:00:00:52:00:06"
	// framedLinuxDropPriority orders the eth0 blackhole rule after the local
	// table's rule (0), so the container's own addresses still answer.
	framedLinuxDropPriority = "100"
	// radiusSecret is a fixture value published in this repository. It never
	// protects anything, and it must not resemble an operator secret.
	radiusSecret = "ze-vpp-framed-fixture-secret"

	// framedDestination, framedGateway and framedMetric are the one
	// Framed-Route FreeRADIUS returns for alice. The gateway is neither
	// 0.0.0.0 nor the subscriber's pool address, so Ze installs the route as
	// a device route through the session interface (newFramedRouteReceipt),
	// and binaryAPI.route sends VPP a path with only the session's interface
	// and the metric as its preference.
	framedDestination = "203.0.113.128/25"
	framedGateway     = "203.0.113.77"
	framedMetric      = 1
	// framedSubscriber is an address inside the framed destination that the
	// subscriber holds on its loopback: only the framed route reaches it.
	framedSubscriber = "203.0.113.129"
)

// productFramedPlan is the plain single-client product topology with a
// FreeRADIUS peer that authenticates alice and returns one Framed-Route.
func productFramedPlan(root, work, suffix string) (interoplab.ScenarioPlan, error) {
	plan := productPlan(root, work, suffix, modePlain, 1)
	plan.Source.Name = "pppoe-vpp-ze-ac-radius-framed-route"
	plan.Source.Checker = checkProductFramed
	clients := filepath.Join(work, "radius-clients.conf")
	if err := os.WriteFile(clients, []byte(framedClients(plan.Network.Candidates)), 0o644); err != nil { //nolint:gosec // FreeRADIUS reads it as its unprivileged user
		return plan, err
	}
	users := filepath.Join(work, "radius-users")
	if err := os.WriteFile(users, []byte(framedUsers()), 0o644); err != nil { //nolint:gosec // FreeRADIUS reads it as its unprivileged user
		return plan, err
	}
	// The subscriber lab's virtual server and detail module answer
	// Access-Requests with CHAP against the users file and accept Ze's
	// accounting, so this lab declares no second copy of either.
	shared := filepath.Join(root, "test", "interop-radius-subscriber")
	plan.Peers = append(plan.Peers, interoplab.PeerConfig{
		Name: radiusPeer, Container: "ze-vpp-pppoe-radius-product-" + suffix, Image: interoplab.FreeRADIUSImageName, Host: radiusHost,
		Mounts: []interoplab.Mount{
			{Source: clients, Target: "/etc/raddb/clients.conf", ReadOnly: true},
			{Source: filepath.Join(shared, "site-default"), Target: "/etc/raddb/sites-enabled/default", ReadOnly: true},
			{Source: filepath.Join(shared, "mods-ze-detail"), Target: "/etc/raddb/mods-enabled/ze_detail", ReadOnly: true},
			{Source: users, Target: "/etc/raddb/mods-config/files/authorize", ReadOnly: true},
		},
		// -X keeps every decoded request and reply on the container log, so
		// a failed run names what the server saw and sent.
		Command: []string{"radiusd", "-f", "-X"},
		Ready: &interoplab.ReadyProbe{
			// 0714 is 1812, the authentication listener, and 0715 is 1813,
			// the accounting listener (site-default).
			Command:  []string{"sh", "-c", "grep -q ':0714 ' /proc/net/udp && grep -q ':0715 ' /proc/net/udp"},
			Timeout:  90 * time.Second,
			Interval: time.Second,
		},
	})
	return plan, nil
}

// framedClients admits Ze as the one NAS on whichever candidate network the
// suite chose. require_message_authenticator makes FreeRADIUS discard an
// Access-Request without a valid Message-Authenticator, so a login also
// proves the server verified the signature Ze computed.
func framedClients(candidates []interoplab.Subnet) string {
	var text textbuf.Buffer
	for index, candidate := range candidates {
		text.Str("client ze-vpp-bng-").Int(int64(index)).Str(" {\n\tipaddr = ").Str(candidate.IPv4.String()).
			Str("\n\tsecret = ").Str(radiusSecret).Str("\n\tnas_type = other\n\trequire_message_authenticator = yes\n}\n")
	}
	return text.String()
}

// framedUsers is FreeRADIUS's authorize file. In FreeRADIUS 3.2.10,
// rlm_files' file_common moves a matching entry's reply items into the
// Access-Accept (radius_pairmove), and radius.c's rad_vp2rfc encodes
// Framed-Route, RFC 2865 attribute 22 of type string, as its text.
func framedUsers() string {
	var text textbuf.Buffer
	return text.Str("alice\tCleartext-Password := \"s3cr3t\"\n\tFramed-Protocol = PPP,\n\tFramed-Route = \"").
		Str(framedDestination).Str(" ").Str(framedGateway).Str(" ").Int(framedMetric).Str("\"\n").String()
}

// framedAuth is the l2tp authentication block that sends every subscriber
// to FreeRADIUS from the NAS address on the macvlan.
func framedAuth(nas, server string) string {
	var text textbuf.Buffer
	return text.Str("auth { radius { source-address ").Str(nas).Str("; nas-identifier ze-vpp-bng; server freeradius { address ").
		Str(server).Str("; port 1812; shared-key ").Str(radiusSecret).Str("; } } }").String()
}

// checkProductFramed proves the RADIUS Framed-Route of one real subscriber
// reaches native VPP as an attached route through its session, carries
// traffic both ways, and leaves with the session.
func checkProductFramed(ctx context.Context, check *interoplab.CheckContext) error {
	if check == nil || check.Lab == nil {
		return errors.New("framed-route checker has no peers")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	lab := check.Lab
	access, _, namespace, err := prepareProduct(ctx, check, modePlain)
	if err != nil {
		return err
	}
	auth, err := framedNAS(ctx, check)
	if err != nil {
		return err
	}
	if err := startProductDaemon(ctx, check, productConfigWith(access, namespace, auth)); err != nil {
		return err
	}
	if err := waitProductInterfaces(ctx, lab, access); err != nil {
		return err
	}
	if _, err := waitProductSessions(ctx, lab, access, 0); err != nil {
		return err
	}
	if err := dialProduct(ctx, lab, accessDevice, "s3cr3t", 0); err != nil {
		return err
	}
	sessions, err := waitProductSessions(ctx, lab, access, 1)
	if err != nil {
		return err
	}
	native, err := productNativeSessions(ctx, lab, sessions)
	if err != nil {
		return err
	}
	links, err := waitProductLinks(ctx, lab, 1)
	if err != nil {
		return err
	}
	if err := productPopulation(sessions, native, links, 1); err != nil {
		return err
	}
	// Only RADIUS authenticates here, so CHAP success is FreeRADIUS's accept.
	log, err := query(ctx, lab, clientPeer, readFileCommand, productLog(0))
	if err != nil {
		return err
	}
	if err := productAuth(log, true); err != nil {
		return err
	}
	if err := waitFramedRoute(ctx, lab, sessions[0].Name); err != nil {
		return err
	}
	if err := framedTraffic(ctx, check, links[0], sessions[0].Name); err != nil {
		return err
	}
	// pppd's SIGTERM is a real peer close; the framed route must leave with
	// the session, beside the subscriber's own host route.
	if _, err := query(ctx, lab, clientPeer, "pkill", "-TERM", "-x", pppdCommand); err != nil {
		return err
	}
	if err := waitProductEmpty(ctx, lab, access, namespace); err != nil {
		return err
	}
	if err := productRoutesGone(ctx, lab, links); err != nil {
		return err
	}
	return productPrefixesGone(ctx, lab, "released framed route", []string{framedDestination})
}

// framedNAS gives Ze's RADIUS client its own Linux address in the VPP peer
// and answers the authentication block that names it and the server.
func framedNAS(ctx context.Context, check *interoplab.CheckContext) (string, error) {
	nas, err := networkHost(check.Network.IPv4, radiusNASHost)
	if err != nil {
		return "", err
	}
	server, err := networkHost(check.Network.IPv4, radiusHost)
	if err != nil {
		return "", err
	}
	lab := check.Lab
	if _, err := query(ctx, lab, vppPeer, "ip", "link", "add", "link", accessDevice, nameToken, radiusNASDevice, addressToken, radiusNASMAC, "type", "macvlan", "mode", "bridge"); err != nil {
		return "", err
	}
	if _, err := query(ctx, lab, vppPeer, "ip", addressToken, "add", nas+"/"+strconv.Itoa(check.Network.IPv4.Bits()), "dev", radiusNASDevice); err != nil {
		return "", err
	}
	if _, err := query(ctx, lab, vppPeer, "ip", "link", "set", radiusNASDevice, "up"); err != nil {
		return "", err
	}
	// VPP reads eth0 through AF_PACKET with eth0's own MAC, so Linux in this
	// container receives every frame sent to VPP as well. With the macvlan's
	// address it would route them, and answer a ping to a subscriber with
	// "Net Unreachable" from that address before VPP's reply arrives. IPv4
	// forwarding is inherited from the Docker host and /proc/sys is read-only
	// here, so a rule discards what Linux would route from eth0 instead; a
	// blackhole rule sends no ICMP, and VPP's AF_PACKET copy is untouched.
	if _, err := query(ctx, lab, vppPeer, "ip", "rule", "add", "iif", accessDevice, "priority", framedLinuxDropPriority, "blackhole"); err != nil {
		return "", err
	}
	return framedAuth(nas, server), nil
}

// waitFramedRoute waits for the framed destination's owned attached path. A
// timeout re-reads the entry once, so the failure names what VPP holds.
func waitFramedRoute(ctx context.Context, lab interoplab.CheckerLab, session string) error {
	_, _, err := interoplab.Wait(ctx, productWait("framed route through the session"), func(ctx context.Context) (string, error) {
		return cli(ctx, lab, showCommand, "ip", "fib", framedDestination)
	}, func(output string) bool { return productFramedRoute(output, session) == nil })
	if err == nil {
		return nil
	}
	output, readErr := cli(ctx, lab, showCommand, "ip", "fib", framedDestination)
	if readErr != nil {
		return errors.Join(err, readErr)
	}
	return errors.Join(err, productFramedRoute(output, session))
}

// productFramedRoute requires the framed destination as one ze-pppoe path
// attached to the session interface, with the route's metric as its
// preference, and no trace of the RADIUS gateway anywhere in the entry. VPP
// v25.10 fib_path_create makes a path with an interface and a zero next hop
// FIB_PATH_TYPE_ATTACHED, and one with a next hop
// FIB_PATH_TYPE_ATTACHED_NEXT_HOP: a gateway route reads attached-nexthop
// and names the gateway, so either check refuses it.
func productFramedRoute(output, session string) error {
	if productField(output, framedGateway) {
		return fmt.Errorf("native entry %s names the RADIUS gateway %s, where a device route through the session belongs: %s", framedDestination, framedGateway, output)
	}
	return productOwnedPath(output, netip.MustParsePrefix(framedDestination), session, framedMetric, productPathAttachedOnly)
}

// framedTraffic sends traffic from the independent host to the address only
// the framed route reaches, and requires both native session counters to
// rise: the request leaves VPP through the session and the reply enters it.
func framedTraffic(ctx context.Context, check *interoplab.CheckContext, link productLink, session string) error {
	lab := check.Lab
	via, err := networkHost(check.Network.IPv4, 2)
	if err != nil {
		return err
	}
	traffic, err := networkHost(check.Network.IPv4, 4)
	if err != nil {
		return err
	}
	if _, err := query(ctx, lab, clientPeer, "ip", addressToken, "add", framedSubscriber+"/32", "dev", "lo"); err != nil {
		return err
	}
	// The reply must return through the PPP link, never the shared Docker
	// segment, or it would reach the traffic host around VPP.
	if _, err := query(ctx, lab, clientPeer, "ip", "route", "replace", traffic+"/32", "dev", link.Name); err != nil {
		return err
	}
	route, err := query(ctx, lab, clientPeer, "ip", "-j", "route", "get", traffic, "from", framedSubscriber)
	if err != nil {
		return err
	}
	if err := routeUsesPPP(route, link.Name); err != nil {
		return err
	}
	if _, err := query(ctx, lab, trafficPeer, "ip", "route", "replace", framedDestination, "via", via); err != nil {
		return err
	}
	// VPP omits zero counters from this CLI output, so one round trip makes
	// both counter rows observable before the measured ping.
	if _, err := query(ctx, lab, trafficPeer, pingCommand, "-n", "-c", "1", "-W", "3", framedSubscriber); err != nil {
		return fmt.Errorf("traffic peer to framed %s: %w", framedSubscriber, err)
	}
	beforeText, err := cli(ctx, lab, showCommand, "interface", session)
	if err != nil {
		return err
	}
	before, err := readPacketCounters(beforeText)
	if err != nil {
		return err
	}
	if _, err := query(ctx, lab, trafficPeer, pingCommand, "-n", "-c", "2", "-W", "3", framedSubscriber); err != nil {
		return fmt.Errorf("traffic peer to framed %s: %w", framedSubscriber, err)
	}
	afterText, err := cli(ctx, lab, showCommand, "interface", session)
	if err != nil {
		return err
	}
	after, err := readPacketCounters(afterText)
	if err != nil {
		return err
	}
	if after.received <= before.received {
		return fmt.Errorf("native %s received no framed reply: before=%+v after=%+v", session, before, after)
	}
	if after.sent <= before.sent {
		return fmt.Errorf("native %s sent no framed request: before=%+v after=%+v", session, before, after)
	}
	return nil
}
