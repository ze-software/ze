// Design: docs/labs/pppoe-interop.md -- AC8 and AC22 through the actual daemon.
// Related: product_plan.go -- common staging, image, kernel and Suite ownership.
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

const productIPv6Traffic = "2001:db8:ffff::4"

func productIPv6Config(access, namespace string) string {
	return strings.Replace(productConfig(access, namespace), "pool { ipv4 {",
		"pool { ipv6-na { start 2001:db8:20::2; end 2001:db8:20::102; prefix 2001:db8:20::/64; } ipv6-pd { block 2001:db8:100::/56; delegation-length 64; } ipv4 {", 1)
}

func productIPv6Plan(root, work, suffix string, mode mechanismMode) interoplab.ScenarioPlan {
	plan := productPlan(root, work, "ipv6-"+suffix, mode, 1)
	plan.Source.Name = "pppoe-vpp-ze-ac-pppd-ipv6-" + mode.name()
	plan.Source.Checker = func(ctx context.Context, check *interoplab.CheckContext) error {
		return checkProductIPv6(ctx, check, mode)
	}
	for index := range plan.Peers {
		plan.Peers[index].Arguments = append(plan.Peers[index].Arguments, "--sysctl", "net.ipv6.conf.all.disable_ipv6=0", "--sysctl", "net.ipv6.conf.default.disable_ipv6=0")
		if plan.Peers[index].Name == clientPeer {
			plan.Peers[index].Mounts = append(plan.Peers[index].Mounts, interoplab.Mount{Source: filepath.Join(work, "le"), Target: productHelper, ReadOnly: true})
		}
		if plan.Peers[index].Name == vppPeer {
			plan.Peers[index].LogFiles = append(plan.Peers[index].LogFiles, "/evidence/product-ipv6-cp.log", "/evidence/product-ipv6-wire.log")
		}
		if plan.Peers[index].Name == trafficPeer {
			plan.Peers[index].Mounts = append(plan.Peers[index].Mounts, interoplab.Mount{Source: work, Target: "/evidence"})
			plan.Peers[index].LogFiles = append(plan.Peers[index].LogFiles, "/evidence/product-family.log")
		}
	}
	return plan
}

func checkProductIPv6(ctx context.Context, check *interoplab.CheckContext, mode mechanismMode) error {
	if check == nil || check.Lab == nil {
		return errors.New("IPv6 product checker has no peers")
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Minute)
	defer cancel()
	lab := check.Lab
	version, err := query(ctx, lab, clientPeer, pppdCommand, "--version")
	if err != nil {
		return err
	}
	// pppd reports its version on stderr on some builds; require the same exact
	// upstream release using the combined successful command observation.
	if !productField(version, "2.5.5") {
		result, err := lab.Exec(ctx, clientPeer, []string{pppdCommand, "--version"}, nil)
		if err != nil {
			return err
		}
		if !productField(result.Stdout+result.Stderr, "2.5.5") {
			return errors.New("IPv6 product peer is not pinned pppd2.5.5")
		}
	}
	traffic4, err := networkHost(check.Network.IPv4, 4)
	if err != nil {
		return err
	}
	access, control, namespace, err := prepareProduct(ctx, check, mode)
	if err != nil {
		return err
	}
	ip := append([]string{"ip"}, namespaceIPArguments(namespace)...)
	// Suppress only automatic TAP link-local generation before Ze starts.
	// No service/data address is installed there; later readback must stay empty.
	if _, err := query(ctx, lab, vppPeer, append(ip, "link", "set", "dev", control, "addrgenmode", "none")...); err != nil {
		return err
	}
	if _, err := query(ctx, lab, vppPeer, append(ip, "-6", addressToken, "flush", "dev", control)...); err != nil {
		return err
	}
	if _, err := query(ctx, lab, vppPeer, "ip", "-6", addressToken, "flush", "dev", accessDevice); err != nil {
		return err
	}
	if _, err := cli(ctx, lab, "set", "interface", "ip", addressToken, hostAccessInterface, "2001:db8:ffff::2/64"); err != nil {
		return err
	}
	if _, err := query(ctx, lab, trafficPeer, "ip", "-6", addressToken, "add", productIPv6Traffic+"/64", "dev", accessDevice); err != nil {
		return err
	}
	for _, prefix := range []string{productIPv6LAN.String(), productIPv6PD.String()} {
		if _, err := query(ctx, lab, trafficPeer, "ip", "-6", "route", "replace", prefix, "via", "2001:db8:ffff::2", "dev", accessDevice); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(check.Source.Directory, "product.conf"), []byte(productIPv6Config(access, namespace)), 0o600); err != nil {
		return err
	}
	if err := lab.ExecDetached(ctx, vppPeer, []string{"sh", "-c", "exec /product/ze start /evidence/product.conf > /evidence/product-ze.log 2>&1"}, nil); err != nil {
		return err
	}
	if err := waitProductInterfaces(ctx, lab, access); err != nil {
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
	// Each cycle starts a real dual-stack pppd. One ends IPv6 alone, the other
	// ends IPv4 alone. The same daemon/access survives both lifetimes.
	for _, closed := range []string{"6", "4"} {
		if err := dialProductIPv6(ctx, lab, device); err != nil {
			return err
		}
		sessions, err := waitProductSessions(ctx, lab, access, 1)
		if err != nil {
			return err
		}
		if _, err := productNativeSessions(ctx, lab, sessions); err != nil {
			return err
		}
		links, err := waitProductLinks(ctx, lab, 1)
		if err != nil {
			return err
		}
		local, router, err := waitProductIPv6CP(ctx, lab)
		if err != nil {
			return err
		}
		if err := noProductKernelPPP(ctx, lab, namespace); err != nil {
			return err
		}
		if err := checkProductIPv6TAP(ctx, lab, ip, control); err != nil {
			return err
		}
		// The helper is a CPE client only: normal PPP frames are sent via pppd's
		// negotiated pppN; it cannot install Ze/VPP state or provide a server reply.
		output, err := query(ctx, lab, clientPeer, productHelper, "test", "deployment", "vpp-pppoe", "product-ipv6-peer", "interface", links[0].Name, "local", local.String())
		if err != nil {
			return err
		}
		var wire productIPv6WireResult
		if err := json.Unmarshal([]byte(output), &wire); err != nil {
			return err
		}
		if wire.Local != local || wire.Router != router || wire.Device != links[0].Name || !wire.WrongServerIgnored || !wire.MalformedIgnored {
			return errors.New("IPv6 CPE wire witness does not match negotiated session")
		}
		if err := productIPv6RA(wire.RA, router, wire.RAHop); err != nil {
			return err
		}
		binding, err := productIPv6Lease(wire.Reply, 7, [3]byte{4, 5, 6}, wire.ClientID, wire.ServerID)
		if err != nil {
			return err
		}
		if binding != wire.Binding {
			return errors.New("DHCPv6 wire differs from installed binding witness")
		}
		if err := os.WriteFile(filepath.Join(check.Source.Directory, "product-ipv6-wire.log"), []byte(output), 0o600); err != nil {
			return err
		}
		owner := sessions[0]
		for _, prefix := range []netip.Prefix{netip.PrefixFrom(binding.Address, 128), binding.Prefix} {
			fib, err := cli(ctx, lab, showCommand, "ip6", "fib", prefix.String())
			if err != nil {
				return err
			}
			if err := productIPv6Route(fib, prefix, owner.Name, local); err != nil {
				return err
			}
		}
		if _, err := query(ctx, lab, clientPeer, "ip", "-6", addressToken, "add", binding.Address.String()+"/128", "dev", links[0].Name); err != nil {
			return err
		}
		pd := binding.Prefix.Addr().Next().Next()
		if _, err := query(ctx, lab, clientPeer, "ip", "-6", addressToken, "add", pd.String()+"/128", "dev", "lo"); err != nil {
			return err
		}
		if _, err := query(ctx, lab, clientPeer, "ip", "-6", "route", "replace", productIPv6Traffic+"/128", "dev", links[0].Name); err != nil {
			return err
		}
		if err := startProductIPv6Capture(ctx, lab, control, namespace); err != nil {
			return err
		}
		if err := productIPv6TrafficBoth(ctx, lab, owner, binding.Address, pd); err != nil {
			return err
		}
		if err := productTraffic(ctx, check, links, sessions, 1, nil); err != nil {
			return err
		}
		mac, err := interfaceMAC(ctx, lab)
		if err != nil {
			return err
		}
		// Positive raw probes establish the exact tuple before their negative is
		// used to distinguish native receive gating from the CPE kernel's DROP.
		source, target := binding.Address.String(), productIPv6Traffic
		if closed == "4" {
			source, target = links[0].Address, traffic4
		}
		if err := startProductFamilyCapture(ctx, lab, closed, source, target); err != nil {
			return err
		}
		if err := productFamilyCheck(ctx, lab, device, mac, owner, "probe4", links[0].Address, traffic4, true); err != nil {
			return err
		}
		if err := productFamilyCheck(ctx, lab, device, mac, owner, "probe6", binding.Address.String(), productIPv6Traffic, true); err != nil {
			return err
		}
		positiveCapture, err := joinProductIPv6Capture(ctx, lab, trafficPeer, "product-family")
		if err != nil {
			return err
		}
		if err := productFamilyCapture(positiveCapture, true); err != nil {
			return err
		}
		// Hold only the closing NCP's subsequent Configure-Requests. Its matching
		// Terminate-Ack still reaches pppd and triggers its own down callback.
		if _, err := query(ctx, lab, clientPeer, "tc", "qdisc", "add", "dev", device, "clsact"); err != nil {
			return err
		}
		protocol := "0x8057"
		if closed == "4" {
			protocol = "0x8021"
		}
		if _, err := query(ctx, lab, clientPeer, "tc", "filter", "add", "dev", device, "egress", "protocol", "0x8864", "pref", "10", "u32", "match", "u16", protocol, "0xffff", "at", "6", "match", "u8", "1", "0xff", "at", "8", "action", "drop"); err != nil {
			return err
		}
		if err := productFamilyCheck(ctx, lab, device, mac, owner, "close"+closed, "::", "::", true); err != nil {
			return err
		}
		if err := productIPv6WaitWithdrawal(ctx, lab, closed, binding, local, links[0].Address); err != nil {
			return err
		}
		remaining, err := waitProductSessions(ctx, lab, access, 1)
		if err != nil {
			return err
		}
		if err := sameProductOwners(sessions, remaining); err != nil {
			return err
		}
		if err := startProductFamilyCapture(ctx, lab, closed, source, target); err != nil {
			return err
		}
		if closed == "6" {
			if err := productFamilyCheck(ctx, lab, device, mac, owner, "probe6", binding.Address.String(), productIPv6Traffic, false); err != nil {
				return err
			}
			if err := productTraffic(ctx, check, links, sessions, 1, nil); err != nil {
				return err
			}
		} else {
			if err := productFamilyCheck(ctx, lab, device, mac, owner, "probe4", links[0].Address, traffic4, false); err != nil {
				return err
			}
			if err := productIPv6TrafficBoth(ctx, lab, owner, binding.Address, pd); err != nil {
				return err
			}
		}
		negativeCapture, err := joinProductIPv6Capture(ctx, lab, trafficPeer, "product-family")
		if err != nil {
			return err
		}
		if err := productFamilyCapture(negativeCapture, false); err != nil {
			return err
		}
		if err := checkProductIPv6TAP(ctx, lab, ip, control); err != nil {
			return err
		}
		capture, err := joinProductIPv6Capture(ctx, lab, vppPeer, "product-ipv6-cp")
		if err != nil {
			return err
		}
		if err := productCaptureEmpty(capture); err != nil {
			return err
		}
		if err := signalProductDial(ctx, lab, 0); err != nil {
			return err
		}
		if _, err := joinProductDial(ctx, lab, 0); err != nil {
			return err
		}
		if err := waitProductEmpty(ctx, lab, access, namespace); err != nil {
			return err
		}
		if err := productIPv6WaitWithdrawal(ctx, lab, "6", binding, local, links[0].Address); err != nil {
			return err
		}
		if err := productRoutesGone(ctx, lab, links); err != nil {
			return err
		}
		if _, err := query(ctx, lab, clientPeer, "tc", "qdisc", "del", "dev", device, "clsact"); err != nil {
			return err
		}
		if _, err := query(ctx, lab, clientPeer, "ip", "-6", addressToken, "del", pd.String()+"/128", "dev", "lo"); err != nil {
			return err
		}
	}
	return nil
}

func dialProductIPv6(ctx context.Context, lab interoplab.CheckerLab, device string) error {
	args := []string{"sh", "-c", `"$@" & child=$!; printf '%s\n' "$child" > /var/log/ppp/product-0.log.pid; wait "$child"; result=$?; printf '%s\n' "$result" > /var/log/ppp/product-0.log.done`, "product-ipv6-dial", pppdCommand, pppdPluginOption, pppdPPPoEPlugin, pppdNICPrefix + device, "user", "alice", "password", "s3cr3t", "rp_pppoe_service", productService, pppdNoAuthOption, "refuse-pap", "refuse-eap", "refuse-mschap", "refuse-mschap-v2", "noipdefault", pppdNoDefaultRoute, pppdNoACCompOption, pppdNoPCompOption, pppdNoCCPOption, pppdNoVJOption, "+ipv6", "ipv6cp-accept-local", "ipv6cp-accept-remote", "nodefaultroute6", mtuToken, "1492", pppdMRUOption, "1492", pppdMaxFailOption, "1", pppdNoDetachOption, pppdDebugOption, pppdLogfileOption, productLog(0)}
	if _, err := query(ctx, lab, clientPeer, "rm", "-f", productLog(0), productLog(0)+".done", productLog(0)+".pid"); err != nil {
		return err
	}
	return lab.ExecDetached(ctx, clientPeer, args, nil)
}

func waitProductIPv6CP(ctx context.Context, lab interoplab.CheckerLab) (netip.Addr, netip.Addr, error) {
	type pair struct{ local, remote netip.Addr }
	value, _, err := interoplab.Wait(ctx, productWait("pppd IPv6CP opened"), func(ctx context.Context) (pair, error) {
		log, err := query(ctx, lab, clientPeer, readFileCommand, productLog(0))
		if err != nil {
			return pair{}, err
		}
		if err := productAuth(log, true); err != nil {
			return pair{}, err
		}
		var p pair
		for line := range strings.SplitSeq(log, "\n") {
			fields := strings.Fields(line)
			if len(fields) == 4 && fields[1] == "LL" && fields[2] == addressToken {
				a, err := netip.ParseAddr(fields[3])
				if err != nil {
					return p, err
				}
				if fields[0] == "local" {
					p.local = a
				}
				if fields[0] == "remote" {
					p.remote = a
				}
			}
		}
		return p, nil
	}, func(p pair) bool {
		return p.local.IsLinkLocalUnicast() && p.remote.IsLinkLocalUnicast() && p.local != p.remote
	})
	return value.local, value.remote, err
}

func checkProductIPv6TAP(ctx context.Context, lab interoplab.CheckerLab, ip []string, device string) error {
	output, err := query(ctx, lab, vppPeer, append(ip, "-j", "-6", addressToken, showCommand, "dev", device)...)
	if err != nil {
		return err
	}
	return productIPv6NoTAPAddress(output, device)
}

func productIPv6TrafficBoth(ctx context.Context, lab interoplab.CheckerLab, owner productSession, addresses ...netip.Addr) error {
	for _, address := range addresses {
		beforeText, err := cli(ctx, lab, showCommand, "interface", owner.Name)
		if err != nil {
			return err
		}
		before, err := readPacketCounters(beforeText)
		if err != nil {
			return err
		}
		if _, err := query(ctx, lab, trafficPeer, pingCommand, "-6", "-n", "-c", "2", "-W", "3", address.String()); err != nil {
			return err
		}
		if _, err := query(ctx, lab, clientPeer, pingCommand, "-6", "-n", "-c", "2", "-W", "3", "-I", address.String(), productIPv6Traffic); err != nil {
			return err
		}
		afterText, err := cli(ctx, lab, showCommand, "interface", owner.Name)
		if err != nil {
			return err
		}
		after, err := readPacketCounters(afterText)
		if err != nil {
			return err
		}
		if err := counterIncrease(before, after); err != nil {
			return err
		}
	}
	return nil
}

func productFamilyCheck(ctx context.Context, lab interoplab.CheckerLab, device, mac string, owner productSession, operation, source, target string, want bool) error {
	output, err := query(ctx, lab, clientPeer, productHelper, "test", "deployment", "vpp-pppoe", "product-family-peer", "interface", device, "peer", mac, "sid", strconv.Itoa(int(owner.SID)), "operation", operation, "source", source, "target", target)
	if err != nil {
		return err
	}
	var wire productFamilyWire
	if err := json.Unmarshal([]byte(output), &wire); err != nil {
		return err
	}
	if len(wire.Request) < 26 || wire.Received != want || (want && len(wire.Response) < 26) {
		return fmt.Errorf("exact-session %s received=%t want=%t", operation, wire.Received, want)
	}
	return nil
}

func productIPv6WaitWithdrawal(ctx context.Context, lab interoplab.CheckerLab, family string, binding productIPv6Binding, local netip.Addr, ipv4 string) error {
	command, defaultRoute := "ip6", "::/0"
	prefixes := []string{netip.PrefixFrom(local, 128).String(), netip.PrefixFrom(binding.Address, 128).String(), binding.Prefix.String()}
	if family == "4" {
		command, defaultRoute = "ip", "0.0.0.0/0"
		prefixes = []string{ipv4 + "/32"}
	}
	_, _, err := interoplab.Wait(ctx, productWait("closed family native routes withdrawn"), func(ctx context.Context) (string, error) { return cli(ctx, lab, showCommand, command, "fib") }, func(output string) bool {
		if !productField(output, defaultRoute) {
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

func startProductIPv6Capture(ctx context.Context, lab interoplab.CheckerLab, device, namespace string) error {
	if _, err := query(ctx, lab, vppPeer, "rm", "-f", "/evidence/product-ipv6-cp.done", "/evidence/product-ipv6-cp.log", "/evidence/product-ipv6-cp.pid"); err != nil {
		return err
	}
	command := `rm -f /evidence/product-ipv6-cp.done /evidence/product-ipv6-cp.log; timeout --signal=INT 600s "$@" > /evidence/product-ipv6-cp.log 2>&1 & child=$!; echo "$child" > /evidence/product-ipv6-cp.pid; wait "$child"; status=$?; echo "$status" > /evidence/product-ipv6-cp.done`
	args := []string{"sh", "-c", command, "product-ipv6-capture"}
	if namespace != "" {
		args = append(args, "ip", "netns", "exec", namespace)
	}
	args = append(args, "tcpdump", "-U", "-n", "-i", device, "ether proto 0x8864 and (ether[20:2] = 0x0021 or (ether[20:2] = 0x0057 and (ether[30:2] = 0x2001 or ether[46:2] = 0x2001)))")
	if err := lab.ExecDetached(ctx, vppPeer, args, nil); err != nil {
		return err
	}
	_, err := waitText(ctx, lab, []string{readFileCommand, "/evidence/product-ipv6-cp.log"}, "listening on "+device)
	return err
}

func startProductFamilyCapture(ctx context.Context, lab interoplab.CheckerLab, family, source, target string) error {
	if _, err := query(ctx, lab, trafficPeer, "rm", "-f", "/evidence/product-family.done", "/evidence/product-family.log", "/evidence/product-family.pid"); err != nil {
		return err
	}
	filter := "icmp6 and ip6[40] = 128 and ip6[44:2] = 0x7070"
	if family == "4" {
		filter = "icmp and icmp[0] = 8 and icmp[4:2] = 0x7070"
	}
	filter += " and src host " + source + " and dst host " + target
	command := `rm -f /evidence/product-family.done /evidence/product-family.log; timeout --signal=INT 60s "$@" > /evidence/product-family.log 2>&1 & child=$!; echo "$child" > /evidence/product-family.pid; wait "$child"; status=$?; echo "$status" > /evidence/product-family.done`
	if err := lab.ExecDetached(ctx, trafficPeer, []string{"sh", "-c", command, "product-family-capture", "tcpdump", "-U", "-n", "-i", accessDevice, filter}, nil); err != nil {
		return err
	}
	_, _, err := interoplab.Wait(ctx, productWait("independent traffic capture ready"), func(ctx context.Context) (string, error) {
		return query(ctx, lab, trafficPeer, readFileCommand, "/evidence/product-family.log")
	}, func(output string) bool { return strings.Contains(output, "listening on eth0") })
	return err
}

func productFamilyCapture(output string, want bool) error {
	expected := "0"
	if want {
		expected = "1"
	}
	summaries := 0
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && (fields[1] == "packet" || fields[1] == packetsToken) && fields[2] == "captured" {
			summaries++
			if fields[0] != expected {
				return errors.New("independent traffic capture contradicts expected family gate")
			}
		}
	}
	if summaries != 1 || !strings.Contains(output, "listening on eth0") {
		return errors.New("independent traffic capture lacks a unique complete summary")
	}
	if want {
		output = strings.Replace(output, "\n1 packet captured\n", "\n0 packets captured\n", 1)
	}
	return productCaptureEmpty(output)
}

func joinProductIPv6Capture(ctx context.Context, lab interoplab.CheckerLab, peer, stem string) (string, error) {
	ownedControl := peer == vppPeer && stem == "product-ipv6-cp"
	ownedTraffic := peer == trafficPeer && stem == "product-family"
	ownedCapture := ownedControl || ownedTraffic
	if !ownedCapture {
		return "", errors.New("unowned IPv6 capture")
	}
	prefix := "/evidence/" + stem
	text, err := query(ctx, lab, peer, readFileCommand, prefix+".pid")
	if err != nil {
		return "", err
	}
	pid, err := strconv.ParseUint(strings.TrimSpace(text), 10, 31)
	if err != nil || pid < 2 {
		return "", errors.New("owned IPv6 capture has invalid PID")
	}
	if _, err := query(ctx, lab, peer, "kill", "-INT", strconv.FormatUint(pid, 10)); err != nil {
		return "", err
	}
	status, _, err := interoplab.Wait(ctx, productWait(stem+" join"), func(ctx context.Context) (string, error) {
		return query(ctx, lab, peer, readFileCommand, prefix+".done")
	}, func(text string) bool { return strings.TrimSpace(text) != "" })
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(status) != "0" && strings.TrimSpace(status) != "130" {
		return "", fmt.Errorf("IPv6 capture exited unsuccessfully: %q", status)
	}
	return query(ctx, lab, peer, readFileCommand, prefix+".log")
}
