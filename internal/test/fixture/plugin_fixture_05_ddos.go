package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/ddos-bps-amplification", p05DDoSBPSAmplification)
	Register("plugin/ddos-detect-characterize", p05DDoSDetectCharacterize)
	Register("plugin/ddos-detect-mitigate", p05DDoSDetectMitigate)
	Register("plugin/ddos-direction", p05DDoSDirection)
	Register("plugin/ddos-firewall-concurrency", p05DDoSFirewallConcurrency)
	Register("plugin/ddos-incident-confidence", p05DDoSIncidentConfidence)
	Register("plugin/ddos-policy", p05DDoSPolicy)
}

type p05FloodSockets struct {
	sender  *net.UDPConn
	sink    *net.UDPConn
	target  *net.UDPAddr
	payload []byte
}

// floodPort05 is the UDP port every flood in these fixtures targets.
const floodPort05 = 9999

func p05OpenFlood(sourceIP, victim string, payloadSize int) (*p05FloodSockets, error) {
	target, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(victim, strconv.Itoa(floodPort05)))
	if err != nil {
		return nil, err
	}
	var source *net.UDPAddr
	if sourceIP != "" {
		source, err = net.ResolveUDPAddr("udp4", net.JoinHostPort(sourceIP, "11211"))
		if err != nil {
			return nil, err
		}
	}
	sender, err := net.ListenUDP("udp4", source)
	if err != nil {
		return nil, err
	}
	sink, err := net.ListenUDP("udp4", target)
	if err != nil {
		sender.Close() //nolint:errcheck // fixture teardown
		return nil, err
	}
	_ = sink.SetReadBuffer(1 << 20)
	return &p05FloodSockets{sender: sender, sink: sink, target: target, payload: make([]byte, payloadSize)}, nil
}

func (sockets *p05FloodSockets) Close() {
	_ = sockets.sender.Close()
	_ = sockets.sink.Close()
}

func (sockets *p05FloodSockets) blast(count int) int {
	sent := 0
	for range count {
		if _, err := sockets.sender.WriteToUDP(sockets.payload, sockets.target); err == nil {
			sent++
		}
	}
	return sent
}

func p05Wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func p05ShowMap(ctx context.Context, plugin *sdk.Plugin, command string) (map[string]any, error) {
	status, value, _, err := p05Dispatch(ctx, plugin, command)
	if err != nil {
		return nil, err
	}
	if status != statusDone {
		return nil, fmt.Errorf("%s status=%s", command, status)
	}
	row := p05Map(value)
	if row == nil {
		return nil, fmt.Errorf("%s returned %T", command, value)
	}
	return row, nil
}

func p05DDoSBPSAmplification(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ddos-bps-probe", func(ctx context.Context, plugin *sdk.Plugin) error {
		for range 12 {
			if err := p05Wait(ctx, time.Second); err != nil {
				return err
			}
		}
		sockets, err := p05OpenFlood("", "127.0.0.5", 16000)
		if err != nil {
			return err
		}
		defer sockets.Close()
		_ = sockets.sender.SetWriteBuffer(8 * 1024 * 1024)
		deadline := time.Now().Add(40 * time.Second)
		sent := 0
		active := float64(0)
		maxActive := float64(0)
		for time.Now().Before(deadline) {
			sent += sockets.blast(1500)
			status, value, _, dispatchErr := p05Dispatch(ctx, plugin, "show ddos status")
			if dispatchErr == nil && status == statusDone {
				row := p05Map(value)
				if row["enabled"] == true {
					active = p05Float(row["active-attacks"])
					if active > maxActive {
						maxActive = active
					}
					if active > 0 {
						break
					}
				}
			}
			if err := p05Wait(ctx, 300*time.Millisecond); err != nil {
				return err
			}
		}
		_, _, raw, _ := p05Dispatch(ctx, plugin, "show ddos status")
		state := string(raw)
		if active <= 0 {
			return fmt.Errorf("ddos-detect did not open an incident on a high-BPS / sub-floor-PPS flood (sent %d 16KB packets, max active-attacks seen=%v); the BPS trigger did not fire. Final state: ddos-status=%s", sent, maxActive, state)
		}
		fmt.Fprintf(os.Stderr, "OK: BPS trigger opened an incident (active-attacks=%v) with the PPS floor unreachable. State: ddos-status=%s\n", active, state)
		return nil
	})
}

func p05WaitDaemonFiles(ctx context.Context) (int, error) {
	if !Poll(ctx, 400, 50*time.Millisecond, func() bool {
		_, pidErr := os.Stat("daemon.pid")
		_, readyErr := os.Stat("daemon.ready")
		return pidErr == nil && readyErr == nil
	}) {
		return 0, errors.New("daemon.pid/ready never appeared")
	}
	raw, err := os.ReadFile("daemon.pid")
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func p05NFTRuleset(ctx context.Context) (string, error) {
	output, err := exec.CommandContext(ctx, "nft", "list", "ruleset").Output()
	return string(output), err
}

func p05DDoSDetectCharacterize(ctx context.Context, args []string) error {
	if err := p05NoArgs(args); err != nil {
		return err
	}
	pid, err := p05WaitDaemonFiles(ctx)
	if err != nil {
		return err
	}
	sockets, err := p05OpenFlood("127.0.1.2", "127.0.0.2", 64)
	if err != nil {
		return err
	}
	defer sockets.Close()
	for range 3 {
		sockets.blast(50)
		if err := p05Wait(ctx, time.Second); err != nil {
			return err
		}
	}
	deadline := time.Now().Add(30 * time.Second)
	sent := 0
	ruleset := ""
	narrowed := false
	for time.Now().Before(deadline) {
		sent += sockets.blast(4000)
		ruleset, _ = p05NFTRuleset(ctx)
		if strings.Contains(ruleset, "ddos-local") && strings.Contains(ruleset, "127.0.0.2") && strings.Contains(ruleset, "11211") {
			narrowed = true
			break
		}
		if err := p05Wait(ctx, 200*time.Millisecond); err != nil {
			return err
		}
	}
	fmt.Fprint(os.Stdout, ruleset) //nolint:errcheck // progress output
	_ = syscall.Kill(pid, syscall.SIGTERM)
	if !narrowed {
		return fmt.Errorf("ddos-local did not narrow to udp sport 11211 -> 127.0.0.2 (sent %d packets); rule stayed coarse or classifier missed the flow", sent)
	}
	fmt.Fprintln(os.Stderr, "OK: ddos-local narrowed the drop to the reflection source port 11211")
	return nil
}

func p05DDoSDetectMitigate(ctx context.Context, args []string) error {
	if err := p05NoArgs(args); err != nil {
		return err
	}
	pid, err := p05WaitDaemonFiles(ctx)
	if err != nil {
		return err
	}
	sockets, err := p05OpenFlood("", "127.0.0.4", 64)
	if err != nil {
		return err
	}
	defer sockets.Close()
	sent := 0
	ruleset := ""
	installed := Poll(ctx, 125, 200*time.Millisecond, func() bool {
		sent += sockets.blast(4000)
		ruleset, _ = p05NFTRuleset(ctx)
		return strings.Contains(ruleset, "ddos-local") && strings.Contains(ruleset, "127.0.0.4")
	})
	fmt.Fprint(os.Stdout, ruleset) //nolint:errcheck // progress output
	_ = syscall.Kill(pid, syscall.SIGTERM)
	if !installed {
		return fmt.Errorf("ddos-local drop rule for 127.0.0.4 not installed (sent %d packets)", sent)
	}
	return nil
}

func p05IncidentRows(ctx context.Context, plugin *sdk.Plugin) []any {
	row, err := p05ShowMap(ctx, plugin, "show ddos incidents")
	if err != nil {
		return nil
	}
	return p05List(row["incidents"])
}

func p05DDoSDirection(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ddos-direction-probe", func(ctx context.Context, plugin *sdk.Plugin) error {
		sockets, err := p05OpenFlood("", "127.0.0.3", 64)
		if err != nil {
			return err
		}
		defer sockets.Close()
		deadline := time.Now().Add(45 * time.Second)
		sent := 0
		var incidents []any
		foundLocal := false
		for time.Now().Before(deadline) {
			sent += sockets.blast(4000)
			incidents = p05IncidentRows(ctx, plugin)
			for _, candidate := range incidents {
				if p05Map(candidate)["direction"] == directionLocal {
					foundLocal = true
				}
			}
			if foundLocal {
				break
			}
			if err := p05Wait(ctx, 300*time.Millisecond); err != nil {
				return err
			}
		}
		if !foundLocal {
			return fmt.Errorf("no incident with direction=local opened for a box-owned victim (sent %d packets, incidents=%v)", sent, incidents)
		}
		fmt.Fprintf(os.Stderr, "OK: box-owned victim 127.0.0.3 classified direction=local on the incident (sent %d packets)\n", sent)
		var local map[string]any
		dropDeadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(dropDeadline) {
			sent += sockets.blast(4000)
			local, _ = p05ShowMap(ctx, plugin, "show ddos local")
			if local["active"] == true {
				break
			}
			if err := p05Wait(ctx, 300*time.Millisecond); err != nil {
				return err
			}
		}
		if local["active"] != true {
			return fmt.Errorf("direction=local was classified but ddos-local installed no on-host drop for 127.0.0.3 within 30s (sent %d packets, show ddos local=%v)", sent, local)
		}
		target := p05Map(local["target"])["dst-prefix"]
		if target != "127.0.0.3/32" {
			return fmt.Errorf("the on-host drop covers %q, not the victim 127.0.0.3/32: %v", target, local)
		}
		fmt.Fprintln(os.Stderr, "LOCAL-DROP-INSTALLED 127.0.0.3")
		return nil
	})
}

type p05DispatchWatch struct {
	mu         sync.Mutex
	command    string
	entered    time.Time
	maxLatency time.Duration
	maxCommand string
	calls      int
}

func (watch *p05DispatchWatch) dispatch(ctx context.Context, plugin *sdk.Plugin, command string) (string, any, error) {
	entered := time.Now()
	watch.mu.Lock()
	watch.command = command
	watch.entered = entered
	watch.mu.Unlock()
	status, value, _, err := p05Dispatch(ctx, plugin, command)
	latency := time.Since(entered)
	watch.mu.Lock()
	watch.command = ""
	watch.calls++
	if latency > watch.maxLatency {
		watch.maxLatency = latency
		watch.maxCommand = command
	}
	watch.mu.Unlock()
	if latency > 3*time.Second {
		return status, value, fmt.Errorf("dispatch %q took %.2fs, over the 3s bound, while ddos-local mitigated under a flood with a firewall block loaded", command, latency.Seconds())
	}
	return status, value, err
}

func p05DaemonPID() int {
	pid := os.Getppid()
	for range 6 {
		if pid <= 1 {
			break
		}
		name, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			break
		}
		if strings.TrimSpace(string(name)) == "ze" {
			return pid
		}
		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			break
		}
		parts := strings.SplitN(string(stat), ") ", 2)
		if len(parts) != 2 {
			break
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 2 {
			break
		}
		pid, _ = strconv.Atoi(fields[1])
	}
	return 0
}

func p05WatchDispatch(ctx context.Context, watch *p05DispatchWatch, stop <-chan struct{}) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			watch.mu.Lock()
			command, entered := watch.command, watch.entered
			watch.mu.Unlock()
			if command != "" && time.Since(entered) > 20*time.Second {
				pid := p05DaemonPID()
				fmt.Fprintf(os.Stderr, "STALLED: dispatch %q has been open for %.1fs; sending SIGQUIT to ze (pid=%d) for a goroutine dump\n", command, time.Since(entered).Seconds(), pid)
				if pid != 0 {
					_ = syscall.Kill(pid, syscall.SIGQUIT)
				}
				return
			}
		}
	}
}

func p05DDoSFirewallConcurrency(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ddos-firewall-concurrency-probe", func(ctx context.Context, plugin *sdk.Plugin) error {
		sockets, err := p05OpenFlood("", "127.0.0.8", 64)
		if err != nil {
			return err
		}
		defer sockets.Close()
		var sent atomic.Uint64
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-stop:
					return
				default:
					sent.Add(uint64(sockets.blast(2000)))
				}
			}
		}()
		watch := new(p05DispatchWatch)
		go p05WatchDispatch(ctx, watch, stop)
		probe := func() (map[string]any, map[string]any, error) {
			status, localValue, err := watch.dispatch(ctx, plugin, "show ddos local")
			if err != nil {
				return nil, nil, err
			}
			local := p05Map(localValue)
			if status != statusDone {
				local = nil
			}
			if _, _, err := watch.dispatch(ctx, plugin, "show ddos incidents"); err != nil {
				return local, nil, err
			}
			status, rulesetValue, err := watch.dispatch(ctx, plugin, "show firewall ruleset fwconc")
			if err != nil {
				return local, nil, err
			}
			if status != statusDone {
				return local, nil, fmt.Errorf("show firewall ruleset fwconc: status=%s data=%v", status, rulesetValue)
			}
			return local, p05Map(rulesetValue), nil
		}
		controlDeadline := time.Now().Add(20 * time.Second)
		var ruleset map[string]any
		for time.Now().Before(controlDeadline) {
			_, ruleset, err = probe()
			if err == nil && ruleset["table"] == "fwconc" {
				break
			}
		}
		if ruleset["table"] != "fwconc" {
			return fmt.Errorf("CONTROL: the firewall block's table %q never reached the kernel, so this test could not have detected losing it: %v", "fwconc", ruleset)
		}
		fmt.Fprintln(os.Stderr, "CONTROL: fwconc applied and readable from the kernel before the flood")
		installDeadline := time.Now().Add(60 * time.Second)
		var local map[string]any
		for time.Now().Before(installDeadline) {
			local, _, err = probe()
			if err != nil || local["active"] == true {
				break
			}
		}
		if err != nil {
			return err
		}
		if local["active"] != true {
			return fmt.Errorf("ddos-local never installed a drop for 127.0.0.8 within 60s (sent %d packets, last show ddos local=%v)", sent.Load(), local)
		}
		soakDeadline := time.Now().Add(12 * time.Second)
		for time.Now().Before(soakDeadline) {
			if _, _, err := probe(); err != nil {
				return fmt.Errorf("the firewall engine's table did not survive a concurrent ddos-local reconcile: %w", err)
			}
		}
		watch.mu.Lock()
		maxLatency, calls, maxCommand := watch.maxLatency, watch.calls, watch.maxCommand
		watch.mu.Unlock()
		fmt.Fprintf(os.Stderr, "DISPATCH-BOUNDED max=%.3fs calls=%d slowest-command=%q bound=3s\n", maxLatency.Seconds(), calls, maxCommand)
		fmt.Fprintln(os.Stderr, "DROP-INSTALLED 127.0.0.8")
		fmt.Fprintln(os.Stderr, "FIREWALL-TABLE-INTACT fwconc")
		return nil
	})
}

func p05IncidentConfidence(ctx context.Context, plugin *sdk.Plugin) float64 {
	row, err := p05ShowMap(ctx, plugin, "show ddos incidents")
	if err != nil || row["enabled"] != true {
		return 0
	}
	best := float64(0)
	for _, candidate := range p05List(row["incidents"]) {
		confidence := p05Float(p05Map(candidate)["confidence"])
		if confidence > best {
			best = confidence
		}
	}
	return best
}

func p05DDoSState(ctx context.Context, plugin *sdk.Plugin, victim string) string {
	commands := []string{"show ddos status", "show ddos incidents", "show traffic usage name lo", "show flow recent", "show flow recent dst " + victim}
	parts := make([]string, 0, len(commands))
	for _, command := range commands {
		_, _, raw, _ := p05Dispatch(ctx, plugin, command)
		parts = append(parts, command+"="+string(raw))
	}
	return strings.Join(parts, " ")
}

func p05DDoSIncidentConfidence(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ddos-conf-probe", func(ctx context.Context, plugin *sdk.Plugin) error {
		sockets, err := p05OpenFlood("127.0.1.6", "127.0.0.6", 64)
		if err != nil {
			return err
		}
		defer sockets.Close()
		sent := 0
		for range 3 {
			sent += sockets.blast(50)
			if err := p05Wait(ctx, time.Second); err != nil {
				return err
			}
		}
		confidence := float64(0)
		deadline := time.Now().Add(40 * time.Second)
		for time.Now().Before(deadline) {
			sent += sockets.blast(4000)
			confidence = p05IncidentConfidence(ctx, plugin)
			if confidence > 0 {
				break
			}
			if err := p05Wait(ctx, 200*time.Millisecond); err != nil {
				return err
			}
		}
		state := p05DDoSState(ctx, plugin, "127.0.0.6")
		if confidence <= 0 {
			return fmt.Errorf("show ddos incidents never reported a non-zero confidence on a reflection flood (sent %d packets); the characterized confidence did not reach the incident store. Final state: %s", sent, state)
		}
		fmt.Fprintf(os.Stderr, "OK: show ddos incidents reported confidence=%v for the reflection attack\n", confidence)
		return nil
	})
}

func p05DDoSPolicy(ctx context.Context, args []string) error {
	return p05Observe(ctx, args, "ddos-policy-probe", func(ctx context.Context, plugin *sdk.Plugin) error {
		sockets, err := p05OpenFlood("", "127.0.0.7", 64)
		if err != nil {
			return err
		}
		defer sockets.Close()
		sent := 0
		detected := false
		deadline := time.Now().Add(45 * time.Second)
		for time.Now().Before(deadline) {
			sent += sockets.blast(4000)
			status, err := p05ShowMap(ctx, plugin, "show ddos status")
			if err == nil && status["enabled"] == true && p05Float(status["active-attacks"]) > 0 {
				detected = true
				break
			}
			if err := p05Wait(ctx, 300*time.Millisecond); err != nil {
				return err
			}
		}
		local, _ := p05ShowMap(ctx, plugin, "show ddos local")
		if !detected {
			return fmt.Errorf("detection never opened an incident under the allow/mitigation policy (sent %d packets); the policy or detector is broken", sent)
		}
		if local["active"] == true {
			return fmt.Errorf("allow/mitigation policy did not exempt mitigation: ddos-local is active after %d packets", sent)
		}
		fmt.Fprintf(os.Stderr, "OK: detection opened an incident but ddos-local is not active -- the allow/mitigation policy exempted mitigation (sent %d packets)\n", sent)
		return nil
	})
}
