// Design: docs/labs/pppoe-interop.md -- genuine configured Ze, native VPP and pppd.
// Related: product_state.go -- strict observations, never an exit-code-only oracle.
package testvpppppoe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func checkProduct(ctx context.Context, check *interoplab.CheckContext, mode mechanismMode, count int) error {
	if check == nil || check.Lab == nil {
		return errors.New("product checker has no peers")
	}
	if count != 1 && count != 100 {
		return fmt.Errorf("unsupported product population %d", count)
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Minute)
	defer cancel()
	lab := check.Lab
	version, err := lab.Exec(ctx, clientPeer, []string{pppdCommand, "--version"}, nil)
	if err != nil {
		return err
	}
	if !productField(version.Stdout+version.Stderr, "2.5.5") {
		return fmt.Errorf("the independent peer is not pinned pppd 2.5.5: %s%s", version.Stdout, version.Stderr)
	}
	access, control, namespace, err := prepareProduct(ctx, check, mode)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(check.Source.Directory, "product.conf"), []byte(productConfig(access, namespace)), 0o600); err != nil {
		return err
	}
	if err := lab.ExecDetached(ctx, vppPeer, []string{"sh", "-c", "exec /product/ze start /evidence/product.conf > /evidence/product-ze.log 2>&1"}, nil); err != nil {
		return err
	}
	if err := waitProductInterfaces(ctx, lab, access); err != nil {
		return err
	}
	if _, err := waitProductSessions(ctx, lab, access, 0); err != nil {
		return err
	}
	if err := noProductKernelPPP(ctx, lab, namespace); err != nil {
		return err
	}
	device := accessDevice
	if mode == modeVLAN {
		device = vlanAccessDevice
		if _, err := query(ctx, lab, clientPeer, "ip", "link", "add", "link", accessDevice, nameToken, device, "type", "vlan", "id", "100"); err != nil {
			return err
		}
		if _, err := query(ctx, lab, clientPeer, "ip", "link", "set", device, "up"); err != nil {
			return err
		}
	}
	if err := startProductDiscovery(ctx, lab); err != nil {
		return err
	}
	if count == 100 {
		for index := range 100 {
			name, mac := productClient(index)
			if _, err := query(ctx, lab, clientPeer, "ip", "link", "add", "link", device, nameToken, name, addressToken, mac, "type", "macvlan", "mode", "bridge"); err != nil {
				return err
			}
			if _, err := query(ctx, lab, clientPeer, "ip", "link", "set", name, "up"); err != nil {
				return err
			}
			if err := dialProduct(ctx, lab, name, "s3cr3t", index); err != nil {
				return err
			}
		}
	} else if err := dialProduct(ctx, lab, device, "s3cr3t", 0); err != nil {
		return err
	}
	sessions, err := waitProductSessions(ctx, lab, access, count)
	if err != nil {
		return err
	}
	native, err := productNativeSessions(ctx, lab, sessions)
	if err != nil {
		return err
	}
	links, err := waitProductLinks(ctx, lab, count)
	if err != nil {
		return err
	}
	if err := productPopulation(sessions, native, links, count); err != nil {
		return err
	}
	if err := finishProductDiscovery(ctx, lab, sessions); err != nil {
		return err
	}
	for index := range count {
		log, err := query(ctx, lab, clientPeer, readFileCommand, productLog(index))
		if err != nil {
			return err
		}
		if err := productAuth(log, true); err != nil {
			return fmt.Errorf("pppd %d: %w", index, err)
		}
	}
	if err := noProductKernelPPP(ctx, lab, namespace); err != nil {
		return err
	}
	if err := startProductCapture(ctx, lab, control, namespace); err != nil {
		return err
	}
	addresses := make(map[uint32]productLink, len(sessions))
	if err := productTraffic(ctx, check, links, sessions, count, addresses); err != nil {
		return err
	}
	if count == 100 {
		// The 101st distinct physical peer must be refused while every
		// original identity remains live. Mere configuration readback does
		// not prove that max-sessions is enforced.
		if err := startProductCapacity(ctx, lab); err != nil {
			return err
		}
		name, mac := productClient(100)
		if _, err := query(ctx, lab, clientPeer, "ip", "link", "add", "link", device, nameToken, name, addressToken, mac, "type", "macvlan", "mode", "bridge"); err != nil {
			return err
		}
		if _, err := query(ctx, lab, clientPeer, "ip", "link", "set", name, "up"); err != nil {
			return err
		}
		if err := dialProduct(ctx, lab, name, "s3cr3t", 100); err != nil {
			return err
		}
		// maxfail one plus the peer discovery deadline joins the refused
		// dial; state is checked after exit, rather than after a sleep.
		if err := waitProductDialExit(ctx, lab, 100); err != nil {
			return err
		}
		if err := finishProductCapacity(ctx, lab); err != nil {
			return err
		}
		remaining, err := waitProductSessions(ctx, lab, access, 100)
		if err != nil {
			return err
		}
		if err := sameProductOwners(sessions, remaining); err != nil {
			return err
		}
		if _, err := productNativeSessions(ctx, lab, sessions); err != nil {
			return err
		}
		if _, err := waitProductLinks(ctx, lab, 100); err != nil {
			return err
		}
		// End one exact admitted peer while the other 99 remain active.
		// Re-read every survivor identity and run actual traffic again; a
		// process-wide failure must not masquerade as successful cleanup.
		_, removedMAC := productClient(0)
		var removed productLink
		expected := make([]productSession, 0, 99)
		for _, owner := range sessions {
			if owner.MAC == removedMAC {
				removed = addresses[owner.Index]
			} else {
				expected = append(expected, owner)
			}
		}
		if len(expected) != 99 || removed.Name == "" || removed.Address == "" {
			return errors.New("the exact terminated peer has no unique observed native route/peer identity")
		}
		if err := signalProductDial(ctx, lab, 0); err != nil {
			return err
		}
		if _, err := joinProductDial(ctx, lab, 0); err != nil {
			return err
		}
		survivors, err := waitProductSessions(ctx, lab, access, 99)
		if err != nil {
			return err
		}
		if err := sameProductOwners(expected, survivors); err != nil {
			return err
		}
		survivingNative, err := productNativeSessions(ctx, lab, survivors)
		if err != nil {
			return err
		}
		survivingLinks, err := waitProductLinks(ctx, lab, 99)
		if err != nil {
			return err
		}
		if err := productPopulation(survivors, survivingNative, survivingLinks, 99); err != nil {
			return err
		}
		if err := productRoutesGone(ctx, lab, []productLink{removed}); err != nil {
			return err
		}
		if err := productTraffic(ctx, check, survivingLinks, survivors, 99, nil); err != nil {
			return err
		}
	}
	if err := finishProductCapture(ctx, lab); err != nil {
		return err
	}
	// pppd owns every process in this isolated client container. SIGTERM is
	// a real peer close, joined by both daemon/native absence and peer links.
	if _, err := query(ctx, lab, clientPeer, "pkill", "-TERM", "-x", pppdCommand); err != nil {
		return err
	}
	if err := waitProductEmpty(ctx, lab, access, namespace); err != nil {
		return err
	}
	if err := productRoutesGone(ctx, lab, links); err != nil {
		return err
	}
	if count == 100 {
		return nil
	}
	// Reuse the same access after cleanup. A failed credential must traverse
	// CHAP and terminate; neither an undialled peer nor a transport failure is
	// accepted as a successful refusal.
	if err := dialProduct(ctx, lab, device, "wrong-secret", 1); err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, productWait("wrong-secret CHAP refusal"), func(ctx context.Context) (string, error) {
		return query(ctx, lab, clientPeer, readFileCommand, productLog(1))
	}, func(log string) bool { return productAuth(log, false) == nil })
	if err != nil {
		return err
	}
	if err := waitProductDialExit(ctx, lab, 1); err != nil {
		return err
	}
	if err := waitProductEmpty(ctx, lab, access, namespace); err != nil {
		return err
	}
	// A fresh accepted dial must recover the first released pool address.
	// This also exercises the real provider again after a refused peer.
	if err := dialProduct(ctx, lab, device, "s3cr3t", 2); err != nil {
		return err
	}
	reused, err := waitProductSessions(ctx, lab, access, 1)
	if err != nil {
		return err
	}
	reusedNative, err := productNativeSessions(ctx, lab, reused)
	if err != nil {
		return err
	}
	reusedLinks, err := waitProductLinks(ctx, lab, 1)
	if err != nil {
		return err
	}
	if err := productPopulation(reused, reusedNative, reusedLinks, 1); err != nil {
		return err
	}
	if reusedLinks[0].Address != peerAddress {
		return fmt.Errorf("released first pool address was not reusable: %+v", reusedLinks)
	}
	if _, err := query(ctx, lab, clientPeer, "pkill", "-TERM", "-x", pppdCommand); err != nil {
		return err
	}
	if err := waitProductEmpty(ctx, lab, access, namespace); err != nil {
		return err
	}
	return productRoutesGone(ctx, lab, reusedLinks)
}

// prepareProduct supplies physical AF_PACKET and LCP infrastructure only.
// In particular it never creates a PPPoE map, session, host route, session
// feature, MTU or rate. Those must all come from the configured real provider.
func prepareProduct(ctx context.Context, check *interoplab.CheckContext, mode mechanismMode) (access, control, namespace string, err error) {
	lab := check.Lab
	if _, err = query(ctx, lab, vppPeer, "mkdir", "-p", "/run/vpp", "/etc/ze"); err != nil {
		return access, control, namespace, err
	}
	if err := lab.ExecDetached(ctx, vppPeer, []string{"sh", "-c", "exec vpp -c /evidence/startup.conf > /evidence/vpp-start.log 2>&1"}, nil); err != nil {
		return access, control, namespace, err
	}
	if _, err = waitText(ctx, lab, []string{vppctlCommand, "-s", vppCLIPath, showCommand, versionCommand}, "vpp"); err != nil {
		return access, control, namespace, err
	}
	var mac, address string
	mac, err = interfaceMAC(ctx, lab)
	if err != nil {
		return access, control, namespace, err
	}
	address, err = networkHost(check.Network.IPv4, 2)
	if err != nil {
		return access, control, namespace, err
	}
	if _, err = query(ctx, lab, vppPeer, "ip", "-4", addressToken, "flush", "dev", accessDevice); err != nil {
		return access, control, namespace, err
	}
	if _, err = cli(ctx, lab, "create", "host-interface", nameToken, accessDevice, "hw-addr", mac); err != nil {
		return access, control, namespace, err
	}
	if _, err = cli(ctx, lab, "set", "interface", "state", hostAccessInterface, "up"); err != nil {
		return access, control, namespace, err
	}
	if _, err = cli(ctx, lab, "set", "interface", "ip", addressToken, hostAccessInterface, address+"/24"); err != nil {
		return access, control, namespace, err
	}
	access, control = hostAccessInterface, "cp0"
	arguments := []string{"lcp", "create", access, "host-if", control}
	if mode == modeNamespace {
		namespace = controlNamespace
		if _, err = query(ctx, lab, vppPeer, "ip", "netns", "add", namespace); err != nil {
			return access, control, namespace, err
		}
		arguments = append(arguments, "netns", namespace)
	}
	if _, err = cli(ctx, lab, arguments...); err != nil {
		return access, control, namespace, err
	}
	if mode == modeVLAN {
		access, control = "host-eth0.100", "cp0.100"
		if _, err = cli(ctx, lab, "create", "sub-interfaces", hostAccessInterface, "100"); err != nil {
			return access, control, namespace, err
		}
		if _, err = cli(ctx, lab, "set", "interface", "state", access, "up"); err != nil {
			return access, control, namespace, err
		}
		if _, err = cli(ctx, lab, "lcp", "create", access, "host-if", control); err != nil {
			return access, control, namespace, err
		}
	}
	arguments = append([]string{"ip"}, namespaceIPArguments(namespace)...)
	arguments = append(arguments, "link", "set", "cp0", "up")
	if _, err = query(ctx, lab, vppPeer, arguments...); err != nil {
		return access, control, namespace, err
	}
	if mode == modeVLAN {
		_, err = query(ctx, lab, vppPeer, "ip", "link", "set", control, "up")
	}
	return access, control, namespace, err
}

func dialProduct(ctx context.Context, lab interoplab.CheckerLab, device, password string, index int) error {
	if _, err := query(ctx, lab, clientPeer, "rm", "-f", productLog(index), productLog(index)+".done", productLog(index)+".pid"); err != nil {
		return err
	}
	arguments := []string{"sh", "-c", `"$@" & child=$!; printf '%s\n' "$child" > ` + productLog(index) + `.pid; wait "$child"; result=$?; printf '%s\n' "$result" > ` + productLog(index) + ".done", "product-dial", pppdCommand, pppdPluginOption, pppdPPPoEPlugin, pppdNICPrefix + device,
		"user", "alice", "password", password, "rp_pppoe_service", productService,
		"pppoe-host-uniq", productHostUniq(index),
		pppdNoAuthOption, "refuse-pap", "refuse-eap", "refuse-mschap", "refuse-mschap-v2",
		"noipdefault", pppdNoDefaultRoute, pppdNoACCompOption, pppdNoPCompOption, pppdNoCCPOption, pppdNoVJOption, "noipv6",
		mtuToken, "1492", pppdMRUOption, "1492", pppdMaxFailOption, "1", pppdNoDetachOption, pppdDebugOption, pppdLogfileOption, productLog(index)}
	return lab.ExecDetached(ctx, clientPeer, arguments, nil)
}

func productLog(index int) string { return "/var/log/ppp/product-" + strconv.Itoa(index) + ".log" }

// Locally administered unicast MACs let one independent peer provide a
// hundred genuine PPPoE clients without a hundred containers or one MAC's cap.
func productClient(index int) (string, string) {
	return fmt.Sprintf("cpe%03d", index), fmt.Sprintf("02:00:00:40:%02x:%02x", index/256, index%256)
}

func productWait(description string) interoplab.WaitOptions {
	return interoplab.WaitOptions{Timeout: 90 * time.Second, Interval: 250 * time.Millisecond, Description: "product " + description}
}

func readProduct(ctx context.Context, lab interoplab.CheckerLab, command string, target any) error {
	output, err := query(ctx, lab, vppPeer, productHelper, "test", "deployment", "vpp-pppoe", "observe", "command", command)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(output), target); err != nil {
		return fmt.Errorf("decode %s: %w; %.300s", command, err, output)
	}
	return nil
}

func waitProductInterfaces(ctx context.Context, lab interoplab.CheckerLab, access string) error {
	_, _, err := interoplab.Wait(ctx, productWait("configured VPP access"), func(ctx context.Context) ([]productAccess, error) {
		var rows []productAccess
		err := readProduct(ctx, lab, "show pppoe interfaces", &rows)
		return rows, err
	}, func(rows []productAccess) bool {
		return len(rows) == 1 && rows[0].Name == access && rows[0].Index > 0 && rows[0].Max == 100
	})
	return err
}

func waitProductSessions(ctx context.Context, lab interoplab.CheckerLab, access string, count int) ([]productSession, error) {
	rows, _, err := interoplab.Wait(ctx, productWait("daemon session population"), func(ctx context.Context) ([]productSession, error) {
		var rows []productSession
		if err := readProduct(ctx, lab, "show pppoe sessions", &rows); err != nil {
			return nil, err
		}
		if rows == nil {
			return nil, errors.New("daemon session answer is null")
		}
		return rows, nil
	}, func(rows []productSession) bool { return validateProductSessions(rows, access, count) == nil })
	return rows, err
}

func noProductKernelPPP(ctx context.Context, lab interoplab.CheckerLab, namespace string) error {
	for _, selected := range []string{"", namespace} {
		arguments := append([]string{"ip"}, namespaceIPArguments(selected)...)
		arguments = append(arguments, "-o", "link", showCommand, "type", "ppp")
		output, err := query(ctx, lab, vppPeer, arguments...)
		if err != nil {
			return err
		}
		if strings.TrimSpace(output) != "" {
			return fmt.Errorf("the Ze daemon created a Linux PPP unit in %q: %s", selected, output)
		}
		if namespace == "" {
			break
		}
	}
	return nil
}

func productNativeSessions(ctx context.Context, lab interoplab.CheckerLab, sessions []productSession) ([]productNative, error) {
	rows, _, err := interoplab.Wait(ctx, productWait("native session identity"), func(ctx context.Context) ([]productNative, error) {
		text, err := cli(ctx, lab, showCommand, "pppoe", "session")
		if err != nil {
			return nil, err
		}
		return parseProductNative(text)
	}, func(rows []productNative) bool { return matchProductNative(sessions, rows) == nil })
	return rows, err
}

func waitProductLinks(ctx context.Context, lab interoplab.CheckerLab, count int) ([]productLink, error) {
	rows, _, err := interoplab.Wait(ctx, productWait("peer IPCP addresses"), func(ctx context.Context) ([]productLink, error) {
		output, err := query(ctx, lab, clientPeer, "ip", "-j", "-4", addressToken, showCommand, "type", "ppp")
		if err != nil {
			return nil, err
		}
		return parseProductLinks(output)
	}, func(rows []productLink) bool { return len(rows) == count })
	return rows, err
}

func waitProductEmpty(ctx context.Context, lab interoplab.CheckerLab, access, namespace string) error {
	if _, err := waitProductSessions(ctx, lab, access, 0); err != nil {
		return err
	}
	_, _, err := interoplab.Wait(ctx, productWait("native session deletion"), func(ctx context.Context) (string, error) {
		return cli(ctx, lab, showCommand, "pppoe", "session")
	}, productNativeEmpty)
	if err != nil {
		return err
	}
	if _, err := waitProductLinks(ctx, lab, 0); err != nil {
		return err
	}
	return noProductKernelPPP(ctx, lab, namespace)
}

func productTraffic(ctx context.Context, check *interoplab.CheckContext, links []productLink, sessions []productSession, count int, addresses map[uint32]productLink) error {
	lab := check.Lab
	via, err := networkHost(check.Network.IPv4, 2)
	if err != nil {
		return err
	}
	traffic, err := networkHost(check.Network.IPv4, 4)
	if err != nil {
		return err
	}
	if _, err := query(ctx, lab, trafficPeer, "ip", "route", "replace", "10.20.0.0/24", "via", via); err != nil {
		return err
	}
	// Source policy is needed with multiple PPP interfaces: a single default
	// route would let replies from 99 sessions escape through the first one.
	used := make(map[uint32]bool, len(sessions))
	for _, link := range links {
		address, err := netip.ParseAddr(link.Address)
		if err != nil || !address.Is4() {
			return fmt.Errorf("subscriber policy route has invalid IPv4 address %q", link.Address)
		}
		bytes := address.As4()
		table := strconv.Itoa(2000 + int(bytes[3]))
		if _, err := query(ctx, lab, clientPeer, "ip", "route", "replace", traffic+"/32", "dev", link.Name, "table", table); err != nil {
			return err
		}
		if _, err := query(ctx, lab, clientPeer, "ip", "rule", "add", "from", link.Address+"/32", "lookup", table); err != nil {
			return err
		}
		route, err := query(ctx, lab, clientPeer, "ip", "-j", "route", "get", traffic, "from", link.Address)
		if err != nil {
			return err
		}
		if err := routeUsesPPP(route, link.Name); err != nil {
			return err
		}
		fib, err := cli(ctx, lab, showCommand, "ip", "fib", link.Address+"/32")
		if err != nil {
			return err
		}
		owner, err := productOwner(fib, link.Address, sessions)
		if err != nil {
			return err
		}
		if used[owner.Index] {
			return fmt.Errorf("multiple peer addresses share native owner %s", owner.Name)
		}
		used[owner.Index] = true
		if addresses != nil {
			addresses[owner.Index] = link
		}
		if _, err := query(ctx, lab, trafficPeer, pingCommand, "-n", "-c", "1", "-W", "3", link.Address); err != nil {
			return fmt.Errorf("traffic peer to %s: %w", link.Address, err)
		}
		beforeText, err := cli(ctx, lab, showCommand, "interface", owner.Name)
		if err != nil {
			return err
		}
		before, err := readPacketCounters(beforeText)
		if err != nil {
			return err
		}
		if _, err := query(ctx, lab, clientPeer, pingCommand, "-n", "-c", "2", "-W", "3", "-I", link.Address, traffic); err != nil {
			return fmt.Errorf("subscriber %s to traffic peer: %w", link.Address, err)
		}
		afterText, err := cli(ctx, lab, showCommand, "interface", owner.Name)
		if err != nil {
			return err
		}
		after, err := readPacketCounters(afterText)
		if err != nil {
			return err
		}
		if after.received <= before.received || after.sent <= before.sent {
			return fmt.Errorf("native %s counters did not move both ways: before=%+v after=%+v", owner.Name, before, after)
		}
	}
	if count == 1 {
		if len(sessions) != 1 || sessions[0].Name != sessionInterface || links[0].Address != peerAddress {
			return errors.New("single-subscriber MTU proof requires observed first native interface and first pool address")
		}
		// Existing mechanism MTU oracle reads native counters and measured ICMP
		// errors. The product has already installed its own routes and MTU.
		if _, err := query(ctx, lab, clientPeer, "ip", "route", "replace", traffic+"/32", "dev", links[0].Name); err != nil {
			return err
		}
		if _, err := query(ctx, lab, trafficPeer, pingCommand, "-n", "-c", "2", "-W", "3", "-M", "do", "-s", "1464", peerAddress); err != nil {
			return fmt.Errorf("1492-byte product positive control: %w", err)
		}
		result, err := lab.Exec(ctx, trafficPeer, []string{pingCommand, "-n", "-c", "1", "-W", "3", "-M", "do", "-s", "1465", peerAddress}, nil)
		code, exited := interoplab.ExecExitCode(err)
		if !exited || code != 1 {
			return errors.Join(fmt.Errorf("1493-byte product packet lacked the measured 1492 MTU refusal: exit=%d output=%s%s", code, result.Stdout, result.Stderr), err)
		}
		if err := productMTURefusal(result.Stdout + result.Stderr); err != nil {
			return err
		}
		return checkIndependentMTU(ctx, lab, links[0].Name)
	}
	return nil
}

// tcpdump is bounded and joined before accepting its zero. The marker is
// emitted only after wait(2) observes the exact owned capture process exit.
func startProductCapture(ctx context.Context, lab interoplab.CheckerLab, device, namespace string) error {
	command := `rm -f /evidence/product-cp.done /evidence/product-cp.log; timeout --signal=INT 600s "$@" > /evidence/product-cp.log 2>&1 & child=$!; echo "$child" > /evidence/product-cp.pid; wait "$child"; status=$?; echo "$status" > /evidence/product-cp.done`
	arguments := []string{"sh", "-c", command, "product-capture"}
	if namespace != "" {
		arguments = append(arguments, "ip", "netns", "exec", namespace)
	}
	arguments = append(arguments, "tcpdump", "-U", "-n", "-i", device, "ether proto 0x8864 and ether[20:2] = 0x0021")
	if err := lab.ExecDetached(ctx, vppPeer, arguments, nil); err != nil {
		return err
	}
	_, err := waitText(ctx, lab, []string{readFileCommand, "/evidence/product-cp.log"}, "listening on "+device)
	return err
}

func finishProductCapture(ctx context.Context, lab interoplab.CheckerLab) error {
	log, err := joinProductCapture(ctx, lab, "product-cp")
	if err != nil {
		return err
	}
	return productCaptureEmpty(log)
}

func productRoutesGone(ctx context.Context, lab interoplab.CheckerLab, rows []productLink) error {
	prefixes := make([]string, 0, len(rows))
	for _, row := range rows {
		prefixes = append(prefixes, row.Address+"/32")
	}
	return productPrefixesGone(ctx, lab, "released native host routes", prefixes)
}

// productPrefixesGone waits until no listed prefix remains in the native
// IPv4 table, each prefix written as format_fib_entry prints it.
func productPrefixesGone(ctx context.Context, lab interoplab.CheckerLab, description string, prefixes []string) error {
	_, _, err := interoplab.Wait(ctx, productWait(description), func(ctx context.Context) (string, error) {
		return cli(ctx, lab, showCommand, "ip", "fib")
	}, func(output string) bool {
		// A positive default-route control proves the native table was read.
		// Blank output or an unknown command cannot prove withdrawal.
		if !productField(output, "0.0.0.0/0") {
			return false
		}
		for _, prefix := range prefixes {
			if productField(output, prefix) {
				return false
			}
		}
		return true
	})
	return err
}

// The owned shell publishes completion only after wait(2) joins this exact
// independent dial. A missing log or an arbitrary pppd process list is not a
// completion barrier, especially with a hundred other active peer processes.
func waitProductDialExit(ctx context.Context, lab interoplab.CheckerLab, index int) error {
	code, err := joinProductDial(ctx, lab, index)
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("refused dial %d exited successfully", index)
	}
	return nil
}

func joinProductDial(ctx context.Context, lab interoplab.CheckerLab, index int) (uint64, error) {
	status, _, err := interoplab.Wait(ctx, productWait("independent dial exit"), func(ctx context.Context) (string, error) {
		return query(ctx, lab, clientPeer, readFileCommand, productLog(index)+".done")
	}, func(output string) bool { return strings.TrimSpace(output) != "" })
	if err != nil {
		return 0, err
	}
	code, err := strconv.ParseUint(strings.TrimSpace(status), 10, 8)
	if err != nil {
		return 0, fmt.Errorf("dial %d has malformed exit marker %q", index, status)
	}
	return code, nil
}

func signalProductDial(ctx context.Context, lab interoplab.CheckerLab, index int) error {
	text, err := query(ctx, lab, clientPeer, readFileCommand, productLog(index)+".pid")
	if err != nil {
		return err
	}
	pid, err := strconv.ParseUint(strings.TrimSpace(text), 10, 31)
	if err != nil || pid < 2 {
		return fmt.Errorf("owned pppd dial has invalid PID %q", text)
	}
	_, err = query(ctx, lab, clientPeer, "kill", "-TERM", strconv.FormatUint(pid, 10))
	return err
}
