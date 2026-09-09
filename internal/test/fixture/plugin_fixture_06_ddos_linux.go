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
	Register("plugin/ddos-local-cap-survives-reload-driver", fixture06DDOSLocalCapSurvivesReload)
	Register("plugin/ddos-local-config-removed-driver", fixture06DDOSLocalConfigRemoved)
	Register("plugin/ddos-parent-config-removed-driver", fixture06DDOSParentConfigRemoved)
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

// fixture06AnnounceWindow is the period announce-rate-limit states its budget
// over (responder.go announceLimiter). Every assertion that an announcement was
// REFUSED must land inside it: once the window slides past the announcement that
// spent the budget, the limiter frees it and a second announcement is correct
// behavior rather than a defect.
const fixture06AnnounceWindow = 60 * time.Second

// fixture06AnnounceWindowMargin keeps the last poll clear of the window edge.
// The driver takes its start instant when it OBSERVES the announce, which is up
// to one poll and one blast after the announce really went out, so the deadline
// it computes is late by that much. The margin absorbs the lag and leaves the
// asserted window shorter than the real one, which is the safe direction.
const fixture06AnnounceWindowMargin = 8 * time.Second

// fixture06SecondGenerationFlood is the least flood the second generation needs
// for the responder to be ASKED to announce: confirm-duration is 1 evaluation
// and check-interval is 1 second in the .ci, plus the detector's own baseline
// and feed latency. A run with less than this left inside the window has not
// tested the refusal, and says so rather than printing the marker.
const fixture06SecondGenerationFlood = 10 * time.Second

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
		// The budget was spent at or just before this instant, so the window the
		// refusal must be asserted inside closes here plus fixture06AnnounceWindow.
		spent := time.Now()
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

		// The second generation. The responder is asked once at its onset and must
		// refuse: one announcement per minute is what the leaf allows, and the
		// first generation spent it.
		//
		// The loop is bounded by the WINDOW and not by an iteration count. An
		// iteration count is a nominal duration, and a slower guest stretches it
		// past the window, at which point the limiter legitimately frees the budget
		// and a second announcement is correct: the fixture would then report a red
		// against correct code.
		deadline := spent.Add(fixture06AnnounceWindow - fixture06AnnounceWindowMargin)
		floodStart := time.Now()
		for time.Now().Before(deadline) {
			sent += sockets.blast(4000)
			if announced() {
				return fmt.Errorf("a second announcement went out %s into the announce-rate-limit window (sent %d packets)",
					time.Since(spent).Round(time.Second), sent)
			}
			if err := fixture06Wait(ctx, 250*time.Millisecond); err != nil {
				return err
			}
		}
		if flooded := time.Since(floodStart); flooded < fixture06SecondGenerationFlood {
			return fmt.Errorf("only %s of the announce-rate-limit window was left for the second generation, and it needs %s: "+
				"the announce and the withdraw took %s, so the refusal was never really tested (sent %d packets)",
				flooded.Round(time.Second), fixture06SecondGenerationFlood, time.Since(spent).Round(time.Second), sent)
		}
		if _, err := fmt.Fprintf(os.Stderr, "ANNOUNCE-REFUSED %s (sent %d)\n", victim, sent); err != nil {
			return fmt.Errorf("report the refusal: %w", err)
		}
		return nil
	})
}

// fixture06LocalReloadVictim is the box-owned address the reload driver floods.
// Every ddos test floods loopback and the suite serializes them, but each one
// takes an address of its own so a failure names one test.
const fixture06LocalReloadVictim = "127.0.0.10"

// fixture06LocalReloadConfig is the config the reload installs. It differs from
// the .ci's own block in one ddos local leaf and nothing else: confidence-min
// gates a CHARACTERIZED mitigation, and the .ci sets characterize-enable false,
// so the change reaches the plugin and touches no drop rule.
//
// The whole file is rewritten because the daemon reloads the whole file. The
// plugin block has to come with it: a reload that dropped it would stop this
// probe through autoStopForRemovedConfigPaths, and the test would end with no
// assertion having run.
const fixture06LocalReloadConfig = `ddos {
	detect {
		enabled true
		absolute-floor 1000
		confirm-duration 1
		clear-consecutive-checks 100
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
		max-mitigation-duration 20
		confidence-min 1
	}
}

traffic {
	usage {
		enabled true
		track-ip true
		interfaces {
			interface lo {
				enabled true
			}
		}
	}
}

plugin {
	external ddos-local-cap-survives-reload-probe {
		run "ze-test fixture plugin/ddos-local-cap-survives-reload-driver"
		encoder json
	}
}
`

// fixture06ReloadDaemon rewrites the daemon's config file and signals it, which
// is the path an operator's commit takes. The runner materializes the .ci's
// stdin=ze-bgp block as ze-bgp.conf in the work directory and the daemon reads
// that file back on SIGHUP (internal/test/runner/runner_config.go).
func fixture06ReloadDaemon(config string) error {
	if err := os.WriteFile("ze-bgp.conf", []byte(config), 0o600); err != nil {
		return fmt.Errorf("rewrite the daemon config: %w", err)
	}
	pidBytes, err := os.ReadFile("daemon.pid")
	if err != nil {
		return fmt.Errorf("read the daemon pid: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return fmt.Errorf("parse the daemon pid %q: %w", pidBytes, err)
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("signal the daemon to reload: %w", err)
	}
	return nil
}

// fixture06DDOSLocalCapSurvivesReload proves an operator's unrelated commit
// cannot orphan a live drop rule.
//
// It installs a drop under an unbroken flood, reloads the daemon on a ddos local
// leaf that has nothing to do with the rule, and then watches for two outcomes
// at once. The ORPHAN is the kernel still holding the rule while `show ddos
// local` reports no mitigation: that state is only reachable after the apply,
// and from it neither the cap nor a clear can remove the rule ever again. The
// PASS is the cap removing the rule after the reload, read back from the kernel
// and from the daemon.
//
// The flood never stops, so no AttackCleared can arrive: the .ci puts
// clear-consecutive-checks at 100. The cap is therefore the only path that could
// remove the rule, which is what makes the removal discriminate.
func fixture06DDOSLocalCapSurvivesReload(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ddos-local-cap-survives-reload-probe", func(ctx context.Context, plugin *sdk.Plugin) error {
		victim := fixture06LocalReloadVictim
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
		if _, err := fmt.Fprintf(os.Stderr, "RELOAD-CAP-INSTALLED %s (sent %d)\n", victim, sent); err != nil {
			return fmt.Errorf("report the install: %w", err)
		}

		if err := fixture06ReloadDaemon(fixture06LocalReloadConfig); err != nil {
			return err
		}

		orphan := ""
		var lastShow map[string]any
		removed := Poll(ctx, 160, 250*time.Millisecond, func() bool {
			sent += sockets.blast(4000)
			// The daemon is read BEFORE the kernel, and the order is what keeps
			// the orphan arm below honest. removeMitigation (responder.go) takes
			// the rule out of the kernel first and publishes the idle status
			// after, so a cap that fires between two reads taken the other way
			// round yields live=true with active=false -- the orphan pattern
			// exactly, produced by correct code. Read this way the intermediate
			// state is active=true with live=false, which matches neither arm and
			// only costs the poll one more turn.
			row, showErr := p05ShowMap(ctx, plugin, "show ddos local")
			if showErr != nil {
				return false
			}
			lastShow = row
			live, _, state, stateErr := fixture06DDOSDropState(victim)
			if stateErr != nil {
				return false
			}
			summary = state
			if live && row["active"] == false {
				// The apply built a responder that knows nothing of the rule the
				// kernel is enforcing. Stop the poll and let the caller name it.
				orphan = state
				return true
			}
			return !live && row["active"] == false
		})
		if orphan != "" {
			return fmt.Errorf("the config apply orphaned the drop for %s: the kernel still holds the rule while show ddos local reports no mitigation, "+
				"so neither max-mitigation-duration nor an AttackCleared can remove it again (sent %d packets):\n%s", victim, sent, orphan)
		}
		if !removed {
			return fmt.Errorf("the drop for %s was still installed 40s after a config apply, with max-mitigation-duration set and the flood still running "+
				"(sent %d packets, show ddos local=%v):\n%s", victim, sent, lastShow, summary)
		}
		if _, err := fmt.Fprintf(os.Stderr, "RELOAD-CAP-REMOVED %s (sent %d)\n", victim, sent); err != nil {
			return fmt.Errorf("report the removal: %w", err)
		}
		return nil
	})
}

// fixture06LocalConfigRemovedVictim is the box-owned address the config-removal
// driver floods. Every ddos test floods loopback and the suite serializes them,
// but each one takes an address of its own so a failure names one test.
const fixture06LocalConfigRemovedVictim = "127.0.0.11"

// fixture06LocalConfigRemovedConfig is what the operator commits: the same file
// with the `ddos local` block deleted and nothing else touched.
//
// detect and observe stay, so the removal is of `ddos/local` alone. That is the
// path this test exists for: the reload delivers an empty body for the removed
// root and then STOPS the plugin, because ddos-local declares
// ConfigRoots ["ddos/local"] (Server.collectProcessesForRemovedConfigPaths,
// internal/component/plugin/server/startup_autoload.go).
//
// max-mitigation-duration is 3600 in the config this replaces, not the 20 its
// sibling test uses, so the cap cannot expire inside this test's window. The
// withdrawal is then the ONLY thing that can take the rule out of the kernel,
// which is what makes the assertion discriminate.
//
// The control-plane-protection block stays, and it is load-bearing rather than
// scenery: copp declares the firewall plugin as a dependency, so it keeps the
// firewall engine running after ddos-local stops. Without a second dependent the
// reload stops firewall too (Server.collectOrphanedDependencies), and the engine
// flushes every ze-owned table on its way out (firewall.FlushAllTables), which
// removes the orphaned drop by accident. The .ci header carries the measurement.
const fixture06LocalConfigRemovedConfig = `ddos {
	detect {
		enabled true
		absolute-floor 1000
		confirm-duration 1
		clear-consecutive-checks 100
		startup-grace 0
		baseline-window 10
		check-interval 1
		characterize-enable false
	}
	observe {
		incident-ring-size 100
	}
}

control-plane-protection {
	bgp {
		rate 100/second
		burst 20
	}
}

traffic {
	usage {
		enabled true
		track-ip true
		interfaces {
			interface lo {
				enabled true
			}
		}
	}
}

plugin {
	external ddos-local-config-removed-probe {
		run "ze-test fixture plugin/ddos-local-config-removed-driver"
		encoder json
	}
}
`

// fixture06DDOSLocalConfigRemoved proves an operator who deletes the `ddos local`
// config block gets the drop rule out of the kernel.
//
// It installs a drop under an unbroken flood, commits a config with the block
// deleted, and then reads the kernel back. Deleting the block STOPS the plugin
// once the reload transaction commits, and the plugin is in-process, so the
// firewall registry it wrote to is the daemon's own and outlives the engine: a
// rule still installed at that point has no responder to see it and no cap
// worker to remove it, for the life of the daemon.
//
// The kernel is the only witness after the reload, and that is not a shortcut:
// `show ddos local` belongs to the plugin the removal stops, so the daemon has
// no answer left to give. It is also the reading that matters, because the
// orphan is precisely a rule the daemon does not know it is enforcing.
//
// The flood never stops and the cap is 3600, so neither an AttackCleared nor
// max-mitigation-duration can account for a removal this test observes.
func fixture06DDOSLocalConfigRemoved(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ddos-local-config-removed-probe", func(ctx context.Context, plugin *sdk.Plugin) error {
		victim := fixture06LocalConfigRemovedVictim
		sockets, err := p05OpenFlood("", victim, 64)
		if err != nil {
			return err
		}
		defer sockets.Close()

		sent := 0
		summary := ""
		installed := Poll(ctx, 200, 250*time.Millisecond, func() bool {
			sent += sockets.blast(4000)
			row, showErr := p05ShowMap(ctx, plugin, "show ddos local")
			if showErr != nil || row["active"] != true {
				return false
			}
			live, _, state, stateErr := fixture06DDOSDropState(victim)
			summary = state
			return stateErr == nil && live
		})
		if !installed {
			return fmt.Errorf("ddos-local installed no drop for %s within 50s of an unbroken flood (sent %d packets):\n%s",
				victim, sent, summary)
		}
		// The markers go to stderr, not stdout: this fixture runs as a plugin of
		// the daemon, and a plugin's stdout is the JSON protocol channel the
		// encoder owns.
		if _, err := fmt.Fprintf(os.Stderr, "CONFIG-REMOVED-INSTALLED %s (sent %d)\n", victim, sent); err != nil {
			return fmt.Errorf("report the install: %w", err)
		}

		if err := fixture06ReloadDaemon(fixture06LocalConfigRemovedConfig); err != nil {
			return err
		}

		withdrawn := Poll(ctx, 160, 250*time.Millisecond, func() bool {
			sent += sockets.blast(4000)
			live, _, state, stateErr := fixture06DDOSDropState(victim)
			if stateErr != nil {
				return false
			}
			summary = state
			return !live
		})
		if !withdrawn {
			return fmt.Errorf("deleting the ddos local config block left the drop for %s in the kernel 40s later, under a flood that never stopped "+
				"and a cap of 3600 seconds: the plugin that installed it is stopped, so no responder and no cap worker is left to remove it and the box "+
				"blackholes the victim for the life of the daemon (sent %d packets):\n%s", victim, sent, summary)
		}
		if _, err := fmt.Fprintf(os.Stderr, "CONFIG-REMOVED-WITHDRAWN %s (sent %d)\n", victim, sent); err != nil {
			return fmt.Errorf("report the withdrawal: %w", err)
		}
		return nil
	})
}

// fixture06ParentConfigRemovedVictim is the box-owned address the parent-removal
// driver floods. Every ddos test floods loopback and the suite serializes them,
// but each one takes an address of its own so a failure names one test.
const fixture06ParentConfigRemovedVictim = "127.0.0.12"

// fixture06ParentConfigRemovedConfig is what the operator commits: the same file
// with the WHOLE `ddos` block deleted.
//
// That gesture is not the sibling test's, and the difference is the point.
// diffMapsRecursive (internal/component/config/diff.go) records ONE removed key
// for a deleted subtree, so this commit puts "ddos" in the diff and nothing
// under it. rootHasChanges (plugin/server/reload.go) matches only the root
// itself or a key beneath it, so "ddos" matches neither "ddos/local" nor
// "ddos/local/": ddos-local is not among the affected plugins and receives NO
// verify and NO apply. parentRemoved (plugin/server/startup_autoload.go) DOES
// match an ancestor, so the plugin is stopped anyway. The config-boundary
// withdrawal cannot fire when no config arrives, so the plugin's exit path is
// the only thing left that can take the rule out.
//
// The control-plane-protection block stays, and it is load-bearing rather than
// scenery: copp declares the firewall plugin as a dependency, so it keeps the
// firewall engine running after ddos-local stops. Without a second dependent the
// reload stops firewall too (Server.collectOrphanedDependencies), and the engine
// flushes every ze-owned table on its way out (firewall.FlushAllTables), which
// removes the orphaned drop by accident. The .ci header carries the measurement.
const fixture06ParentConfigRemovedConfig = `control-plane-protection {
	bgp {
		rate 100/second
		burst 20
	}
}

traffic {
	usage {
		enabled true
		track-ip true
		interfaces {
			interface lo {
				enabled true
			}
		}
	}
}

plugin {
	external ddos-parent-config-removed-probe {
		run "ze-test fixture plugin/ddos-parent-config-removed-driver"
		encoder json
	}
}
`

// fixture06DDOSParentConfigRemoved proves an operator who deletes the whole
// `ddos` block, rather than `ddos local` alone, gets the drop rule out of the
// kernel.
//
// It installs a drop under an unbroken flood, commits a config with the parent
// block deleted, and then reads the kernel back. This commit tells ddos-local
// nothing: no verify and no apply reaches it, because the diff records one key
// for the whole subtree and the affected-plugin walk does not match an ancestor.
// The plugin is stopped all the same. So a rule still installed at that point has
// no responder to see it, no cap worker to remove it and no config left that
// mentions it, for the life of the daemon.
//
// The kernel is the only witness after the reload, and that is not a shortcut:
// `show ddos local` belongs to the plugin the removal stops, so the daemon has
// no answer left to give. It is also the reading that matters, because the
// orphan is precisely a rule the daemon does not know it is enforcing.
//
// The flood never stops and the cap is 3600, so neither an AttackCleared nor
// max-mitigation-duration can account for a removal this test observes.
func fixture06DDOSParentConfigRemoved(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ddos-parent-config-removed-probe", func(ctx context.Context, plugin *sdk.Plugin) error {
		victim := fixture06ParentConfigRemovedVictim
		sockets, err := p05OpenFlood("", victim, 64)
		if err != nil {
			return err
		}
		defer sockets.Close()

		sent := 0
		summary := ""
		installed := Poll(ctx, 200, 250*time.Millisecond, func() bool {
			sent += sockets.blast(4000)
			row, showErr := p05ShowMap(ctx, plugin, "show ddos local")
			if showErr != nil || row["active"] != true {
				return false
			}
			live, _, state, stateErr := fixture06DDOSDropState(victim)
			summary = state
			return stateErr == nil && live
		})
		if !installed {
			return fmt.Errorf("ddos-local installed no drop for %s within 50s of an unbroken flood (sent %d packets):\n%s",
				victim, sent, summary)
		}
		// The markers go to stderr, not stdout: this fixture runs as a plugin of
		// the daemon, and a plugin's stdout is the JSON protocol channel the
		// encoder owns.
		if _, err := fmt.Fprintf(os.Stderr, "PARENT-REMOVED-INSTALLED %s (sent %d)\n", victim, sent); err != nil {
			return fmt.Errorf("report the install: %w", err)
		}

		if err := fixture06ReloadDaemon(fixture06ParentConfigRemovedConfig); err != nil {
			return err
		}

		withdrawn := Poll(ctx, 160, 250*time.Millisecond, func() bool {
			sent += sockets.blast(4000)
			live, _, state, stateErr := fixture06DDOSDropState(victim)
			if stateErr != nil {
				return false
			}
			summary = state
			return !live
		})
		if !withdrawn {
			return fmt.Errorf("deleting the parent ddos config block left the drop for %s in the kernel 40s later, under a flood that never stopped "+
				"and a cap of 3600 seconds: the removal delivers no config to ddos-local, so its config-boundary withdrawal never runs, and the plugin "+
				"is stopped anyway -- no responder and no cap worker is left to remove the rule and the box blackholes the victim for the life of the "+
				"daemon (sent %d packets):\n%s", victim, sent, summary)
		}
		if _, err := fmt.Fprintf(os.Stderr, "PARENT-REMOVED-WITHDRAWN %s (sent %d)\n", victim, sent); err != nil {
			return fmt.Errorf("report the withdrawal: %w", err)
		}
		return nil
	})
}
