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

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/ddos-transit-forward-drop-setup", fixture06DDOSTransitSetup)
	Register("plugin/ddos-transit-forward-drop-driver", fixture06DDOSTransitDriver)
	Register("plugin/ddos-local-max-duration-driver", fixture06DDOSLocalMaxDuration)
	Register("plugin/ddos-announce-rate-limit-driver", fixture06DDOSAnnounceRateLimit)
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

// fixture06DDOSDropState reads the kernel back and reports whether ze_ddos-local
// holds a drop matching victim, and whether that drop sits on the FORWARD hook.
// The victim is a parameter because two tests read this state for two different
// addresses: the transit driver for a remote victim, the max-duration driver for
// a box-owned one.
func fixture06DDOSDropState(victimIP string) (installed, forward bool, summary string, err error) {
	conn := new(nftables.Conn)
	tables, err := conn.ListTables()
	if err != nil {
		return false, false, "", err
	}
	chains, err := conn.ListChains()
	if err != nil {
		return false, false, "", err
	}
	victim := net.ParseIP(victimIP).To4()
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

// fixture06Flood is a UDP flood aimed at a REMOTE victim.
//
// A remote address cannot be bound, so this DIALS rather than listening:
// p05OpenFlood also opens a sink socket on the victim address, which is right
// for a box-owned victim and answers "bind: cannot assign requested address"
// for an address the box does not hold.
type fixture06Flood struct {
	socket  *net.UDPConn
	payload []byte
}

func fixture06OpenRemoteFlood(victim string) (*fixture06Flood, error) {
	socket, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP(victim), Port: floodPort05})
	if err != nil {
		return nil, err
	}
	return &fixture06Flood{socket: socket, payload: bytes.Repeat([]byte{'x'}, 64)}, nil
}

func (f *fixture06Flood) blast(count int) int {
	sent := 0
	for range count {
		if _, err := f.socket.Write(f.payload); err == nil {
			sent++
		}
	}
	return sent
}

func (f *fixture06Flood) Close() { _ = f.socket.Close() }

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
	flood, err := fixture06OpenRemoteFlood("203.0.113.9")
	if err != nil {
		return err
	}
	defer flood.Close()
	blast := flood.blast
	sent := 0
	var lastSummary string
	var lastForward bool
	if !Poll(ctx, 150, 300*time.Millisecond, func() bool {
		sent += blast(4000)
		installed, forward, summary, stateErr := fixture06DDOSDropState("203.0.113.9")
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
		installed, _, summary, stateErr := fixture06DDOSDropState("203.0.113.9")
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
		installed, _, summary, stateErr := fixture06DDOSDropState("203.0.113.9")
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

// fixture06LocalMaxDurationVictim is the box-owned address this driver floods.
// Every ddos test floods loopback and the suite serializes them, but each one
// takes an address of its own so a failure names one test.
const fixture06LocalMaxDurationVictim = "127.0.0.9"

// fixture06DDOSLocalMaxDuration proves the ddos local max-mitigation-duration
// cap removes a live drop rule while the attack is still running.
//
// It floods a box-owned victim and never stops, so no AttackCleared can arrive:
// the clear path needs clear-consecutive-checks evaluations under the threshold,
// which the .ci puts at 100. The cap worker is therefore the only thing that can
// remove the rule, which is what makes the second wait discriminate.
//
// Both halves are read twice, from the kernel and from the daemon: the nft
// ruleset says what the box is really doing, and `show ddos local` says what the
// responder published. AC-6 promises both.
func fixture06DDOSLocalMaxDuration(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ddos-local-max-duration-probe", func(ctx context.Context, plugin *sdk.Plugin) error {
		victim := fixture06LocalMaxDurationVictim
		sockets, err := p05OpenFlood("", victim, 64)
		if err != nil {
			return err
		}
		defer sockets.Close()

		sent := 0
		summary := ""
		installed := Poll(ctx, 200, 250*time.Millisecond, func() bool {
			sent += sockets.blast(4000)
			live, _, state, stateErr := fixture06DDOSDropState(victim)
			summary = state
			if stateErr != nil || !live {
				return false
			}
			row, showErr := p05ShowMap(ctx, plugin, "show ddos local")
			return showErr == nil && row["active"] == true
		})
		if !installed {
			return fmt.Errorf("ddos-local installed no drop for %s within 50s of an unbroken flood (sent %d packets):\n%s",
				victim, sent, summary)
		}
		// The markers go to stderr, not stdout: this fixture runs as a plugin of
		// the daemon, and a plugin's stdout is the JSON protocol channel the
		// encoder owns.
		if _, err := fmt.Fprintf(os.Stderr, "LOCAL-CAP-INSTALLED %s (sent %d)\n", victim, sent); err != nil {
			return fmt.Errorf("report the install: %w", err)
		}

		var lastShow map[string]any
		removed := Poll(ctx, 160, 250*time.Millisecond, func() bool {
			sent += sockets.blast(4000)
			live, _, state, stateErr := fixture06DDOSDropState(victim)
			summary = state
			if stateErr != nil || live {
				return false
			}
			row, showErr := p05ShowMap(ctx, plugin, "show ddos local")
			lastShow = row
			return showErr == nil && row["active"] == false
		})
		if !removed {
			return fmt.Errorf("the drop for %s was still installed 40s after it went in, with max-mitigation-duration set and the flood still running "+
				"(sent %d packets, show ddos local=%v):\n%s", victim, sent, lastShow, summary)
		}
		if _, err := fmt.Fprintf(os.Stderr, "LOCAL-CAP-REMOVED %s (sent %d)\n", victim, sent); err != nil {
			return fmt.Errorf("report the removal: %w", err)
		}
		return nil
	})
}

// fixture06AnnounceRateLimitVictim is the REMOTE (box-unowned) address this
// driver floods. The flowspec responder defers a local victim to on-host
// mitigation, so only a remote one reaches announce at all.
const fixture06AnnounceRateLimitVictim = "203.0.113.9"

// fixture06DDOSAnnounceRateLimit proves ddos flowspec refuses a second
// announcement inside one announce-rate-limit window.
//
// Two attack generations are required. AttackDetected is emitted once per
// generation at onset, so the responder is ASKED to announce once per
// generation: the driver floods, waits for the announce, stops so the detector
// clears and max-mitigation-duration withdraws it, then floods again inside the
// same 60-second window. The refusal is asserted only AFTER a second incident
// proves the responder was asked, because the absence of an announce would also
// be true of a responder nobody ever called.
func fixture06DDOSAnnounceRateLimit(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ddos-announce-rate-limit-probe", func(ctx context.Context, plugin *sdk.Plugin) error {
		victim := fixture06AnnounceRateLimitVictim
		sockets, err := fixture06OpenRemoteFlood(victim)
		if err != nil {
			return err
		}
		defer sockets.Close()

		announced := func() bool {
			row, showErr := p05ShowMap(ctx, plugin, "show ddos flowspec")
			return showErr == nil && row["active"] == true
		}

		sent := 0
		if !Poll(ctx, 200, 250*time.Millisecond, func() bool {
			sent += sockets.blast(4000)
			return announced()
		}) {
			return fmt.Errorf("ddos-flowspec announced nothing for remote victim %s within 50s of an unbroken flood (sent %d packets)", victim, sent)
		}
		if _, err := fmt.Fprintf(os.Stderr, "ANNOUNCED %s (sent %d)\n", victim, sent); err != nil {
			return fmt.Errorf("report the announce: %w", err)
		}

		// The flood stops here, and nothing floods again until the announce is
		// withdrawn: max-mitigation-duration lifts it, and the detector clears the
		// first generation so a second one can open.
		if !Poll(ctx, 160, 250*time.Millisecond, func() bool { return !announced() }) {
			return fmt.Errorf("the first announce was never withdrawn, so the second generation could not be asked for (sent %d packets)", sent)
		}
		// A quiet period long enough for the detector's own state machine to
		// clear: clear-consecutive-checks is 2 and check-interval is 1, so two
		// evaluations under the threshold are two seconds. Nothing the probe can
		// READ says the detector cleared -- the observe store's incident stays
		// open because the detector's Cleared event carries an empty target and
		// matches nothing (the defect stale-incident-timeout exists for) -- so
		// this waits rather than polls. A detector that did NOT clear emits no
		// second AttackDetected, the responder is never asked, and the .ci's
		// refusal-line expectation is what catches it.
		if err := fixture06Wait(ctx, 15*time.Second); err != nil {
			return err
		}

		// The second generation. The responder is asked once at its onset and
		// must refuse: one announcement per minute is what the leaf allows, and
		// the first generation spent it. The whole window below stays inside the
		// 60 seconds the budget takes to return, or the announce would be allowed
		// on its own terms and prove nothing.
		for range 100 {
			sent += sockets.blast(4000)
			if announced() {
				return fmt.Errorf("a second announcement went out inside the announce-rate-limit window (sent %d packets)", sent)
			}
			if err := fixture06Wait(ctx, 250*time.Millisecond); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(os.Stderr, "ANNOUNCE-REFUSED %s (sent %d)\n", victim, sent); err != nil {
			return fmt.Errorf("report the refusal: %w", err)
		}
		return nil
	})
}
