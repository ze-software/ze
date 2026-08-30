//go:build linux

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/vishvananda/netlink"
)

func init() {
	Register("plugin/ddos-transit-forward-drop-setup", fixture06DDOSTransitSetup)
	Register("plugin/ddos-transit-forward-drop-driver", fixture06DDOSTransitDriver)
}

func fixture06DDOSTransitSetup(context.Context, []string) error {
	_ = netlink.LinkAdd(&netlink.Veth{Name: "zdd0", PeerName: "zdd0p"})
	link, err := netlink.LinkByName("zdd0")
	if err == nil {
		address, _ := netlink.ParseAddr("203.0.113.1/24")
		_ = netlink.AddrAdd(link, address)
		_ = netlink.LinkSetUp(link)
		hardware, _ := net.ParseMAC("02:00:00:00:00:01")
		_ = netlink.NeighAdd(&netlink.Neigh{LinkIndex: link.Attrs().Index, IP: net.ParseIP("203.0.113.9"), HardwareAddr: hardware, State: netlink.NUD_PERMANENT})
	}
	if peer, peerErr := netlink.LinkByName("zdd0p"); peerErr == nil {
		_ = netlink.LinkSetUp(peer)
	}
	_ = os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0o600)
	return nil
}

const fixture06DDOSConfigOff = `ddos {
	detect {
		enabled true
		absolute-floor 1000
		confirm-duration 1
		startup-grace 0
		baseline-window 10
		check-interval 1
		characterize-enable false
	}
	observe {
		incident-ring-size 100
	}
	local {
		response-level enforce
		forward-mitigation false
	}
}

traffic {
	usage {
		enabled true
		track-ip true
		interfaces {
			interface zdd0p {
				enabled true
			}
			interface zdd0 {
				enabled true
			}
		}
	}
}
`

func fixture06DDOSDropState() (installed, forward bool, summary string, err error) {
	conn := new(nftables.Conn)
	tables, err := conn.ListTables()
	if err != nil {
		return false, false, "", err
	}
	chains, err := conn.ListChains()
	if err != nil {
		return false, false, "", err
	}
	victim := net.ParseIP("203.0.113.9").To4()
	var details []string
	for _, table := range tables {
		if table.Name != "ze_ddos-local" {
			continue
		}
		for _, chain := range chains {
			if chain.Table == nil || chain.Table.Name != table.Name || chain.Table.Family != table.Family {
				continue
			}
			rules, rulesErr := conn.GetRules(table, chain)
			if rulesErr != nil {
				return false, false, strings.Join(details, "\n"), rulesErr
			}
			details = append(details, fmt.Sprintf("table=%s chain=%s rules=%d", table.Name, chain.Name, len(rules)))
			for _, rule := range rules {
				for _, expression := range rule.Exprs {
					comparison, ok := expression.(*expr.Cmp)
					if !ok || !bytes.Equal(comparison.Data, victim) {
						continue
					}
					installed = true
					if chain.Hooknum != nil && *chain.Hooknum == *nftables.ChainHookForward {
						forward = true
					}
				}
			}
		}
	}
	return installed, forward, strings.Join(details, "\n"), nil
}

func fixture06DDOSTransitDriver(ctx context.Context, _ []string) error {
	if !Poll(ctx, 400, 50*time.Millisecond, func() bool {
		_, pidErr := os.Stat("daemon.pid")
		_, readyErr := os.Stat("daemon.ready")
		return pidErr == nil && readyErr == nil
	}) {
		return errors.New("daemon readiness files missing")
	}
	pidBytes, err := os.ReadFile("daemon.pid")
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return fmt.Errorf("parse daemon pid: %w", err)
	}
	socket, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP("203.0.113.9"), Port: 9999})
	if err != nil {
		return err
	}
	defer func() { _ = socket.Close() }()
	payload := bytes.Repeat([]byte{'x'}, 64)
	blast := func(count int) int {
		sent := 0
		for range count {
			if _, writeErr := socket.Write(payload); writeErr == nil {
				sent++
			}
		}
		return sent
	}
	sent := 0
	var lastSummary string
	var lastForward bool
	if !Poll(ctx, 150, 300*time.Millisecond, func() bool {
		sent += blast(4000)
		installed, forward, summary, stateErr := fixture06DDOSDropState()
		lastSummary, lastForward = summary, forward
		return stateErr == nil && installed
	}) {
		return fmt.Errorf("no ddos-local drop for remote victim 203.0.113.9 after %d packets:\n%s", sent, lastSummary)
	}
	if !lastForward {
		return fmt.Errorf("drop installed for 203.0.113.9 but not on the FORWARD hook:\n%s", lastSummary)
	}
	if _, err := fmt.Fprintln(os.Stdout, "FORWARD-DROP-INSTALLED 203.0.113.9"); err != nil {
		return fmt.Errorf("report forward drop: %w", err)
	}
	if !Poll(ctx, 300, 300*time.Millisecond, func() bool {
		installed, _, summary, stateErr := fixture06DDOSDropState()
		lastSummary = summary
		return stateErr == nil && !installed
	}) {
		return fmt.Errorf("mitigation never cleared after the flood stopped:\n%s", lastSummary)
	}
	if err := os.WriteFile("ze-bgp.conf", []byte(fixture06DDOSConfigOff), 0o600); err != nil {
		return err
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return err
	}
	sent2 := 0
	for range 80 {
		sent2 += blast(4000)
		installed, _, summary, stateErr := fixture06DDOSDropState()
		if stateErr != nil {
			return stateErr
		}
		if installed {
			return fmt.Errorf("forward-mitigation OFF but a drop was installed for remote victim 203.0.113.9:\n%s", summary)
		}
		if err := fixture06Wait(ctx, 300*time.Millisecond); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(os.Stdout, "REMOTE-DEFER-NO-DROP 203.0.113.9 (sent %d)\n", sent2); err != nil {
		return fmt.Errorf("report remote defer: %w", err)
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return err
	}
	Poll(ctx, 100, 50*time.Millisecond, func() bool { return syscall.Kill(pid, 0) != nil })
	return nil
}
