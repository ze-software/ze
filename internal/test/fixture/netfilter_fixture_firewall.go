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
	"syscall"
	"time"
)

func registerTableSnapshot(name, family, table string) {
	Register(name, func(ctx context.Context, _ []string) error {
		pid, err := waitDaemon(ctx, 200)
		if err != nil {
			return err
		}
		var out string
		if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
			out, err = netfilterCommandOutput(ctx, "nft", "list", "table", family, table)
			return err == nil
		}) {
			return fmt.Errorf("firewall object never programmed")
		}
		fmt.Print(out)
		return signalProcess(pid, syscall.SIGTERM)
	})
}

func coppTrusted(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	var out string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		out, err = netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_copp")
		return err == nil
	}) {
		return fmt.Errorf("firewall object never programmed")
	}
	fmt.Print(out)
	// The limiter lives in the regular bgp-neighbor chain, reached only through
	// the input chain's jump, so "trusted before the limiter" is the trusted
	// accept ordered before that jump, with the limiter inside the jump target.
	// Line order across the whole listing says nothing: nft prints the chains
	// in its own order.
	input, err := nftChainLines(out, "input")
	if err != nil {
		return err
	}
	trusted := nftLineIndex(input, "ip saddr 192.0.2.0/24", "accept")
	if trusted < 0 {
		return fmt.Errorf("ORDER_FAIL: input chain holds no accept for trusted 192.0.2.0/24")
	}
	jump := nftLineIndex(input, "jump bgp-neighbor")
	if jump < 0 {
		return fmt.Errorf("ORDER_FAIL: input chain holds no jump to the bgp-neighbor chain")
	}
	if trusted >= jump {
		return fmt.Errorf("ORDER_FAIL: trusted accept after the jump to the limiter")
	}
	neighbor, err := nftChainLines(out, "bgp-neighbor")
	if err != nil {
		return err
	}
	if nftLineIndex(neighbor, "limit rate") < 0 {
		return fmt.Errorf("ORDER_FAIL: bgp-neighbor chain holds no limiter")
	}
	fmt.Fprintln(os.Stderr, "ORDER_OK: trusted accepted before the jump to the bgp-neighbor limiter")
	return signalProcess(pid, syscall.SIGTERM)
}

// coppUnknownSourceDrop proves the CoPP access list from both sides on
// loopback: the configured neighbor 127.0.0.2 completes a TCP handshake to the
// protected port, and 127.0.0.3, neither a neighbor nor trusted, gets no answer
// while unknown-source-drop counts its SYN. The neighbor dials first, so the
// listener is known to be up: with no drop term the kernel would complete the
// unknown source's handshake too, and only the drop explains its silence.
func coppUnknownSourceDrop(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("firewall/copp-unknown-source-drop takes the protected port, got %d arguments", len(args))
	}
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		out, readErr := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_copp")
		return readErr == nil && strings.Contains(out, "jump bgp-neighbor")
	}) {
		return fmt.Errorf("ze_copp never programmed a jump to the bgp-neighbor chain")
	}

	target := net.JoinHostPort("127.0.0.1", args[0])
	var dialErr error
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		dialErr = coppDial(ctx, "127.0.0.2", target, time.Second)
		// refuse-by-default: guards coppUnknownSourceDrop -- dialErr keeps the last dial error, and NEIGHBOR_FAIL reports it when the poll gives up
		return dialErr == nil
	}) {
		return fmt.Errorf("NEIGHBOR_FAIL: configured neighbor 127.0.0.2 never connected to %s: %w", target, dialErr)
	}
	fmt.Fprintln(os.Stderr, "NEIGHBOR_OK: 127.0.0.2 connected to", target)

	dialErr = coppDial(ctx, "127.0.0.3", target, 2*time.Second)
	if dialErr == nil {
		return fmt.Errorf("UNKNOWN_FAIL: unknown source 127.0.0.3 completed a handshake to %s", target)
	}
	var netErr net.Error
	if !errors.As(dialErr, &netErr) {
		return fmt.Errorf("UNKNOWN_FAIL: unknown source 127.0.0.3 was answered, not dropped: %w", dialErr)
	}
	if !netErr.Timeout() {
		return fmt.Errorf("UNKNOWN_FAIL: unknown source 127.0.0.3 was answered, not dropped: %w", dialErr)
	}
	fmt.Fprintln(os.Stderr, "UNKNOWN_DROPPED: 127.0.0.3 got no answer from", target)

	out, err := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_copp")
	if err != nil {
		return fmt.Errorf("read ze_copp after the dials: %w", err)
	}
	fmt.Print(out)
	input, err := nftChainLines(out, "input")
	if err != nil {
		return err
	}
	drop := nftLineIndex(input, "counter packets", "drop")
	if drop < 0 {
		return fmt.Errorf("DROP_FAIL: input chain holds no counted drop term")
	}
	packets, err := nftCounterPackets(input[drop])
	if err != nil {
		return err
	}
	if packets == 0 {
		return fmt.Errorf("DROP_FAIL: unknown-source-drop counted no packet: %q", input[drop])
	}
	fmt.Fprintln(os.Stderr, "DROP_COUNTED: unknown-source-drop packets", packets)
	return signalProcess(pid, syscall.SIGTERM)
}

// coppDial opens one TCP connection from source to target and closes it.
func coppDial(ctx context.Context, source, target string, timeout time.Duration) error {
	dialer := net.Dialer{Timeout: timeout, LocalAddr: &net.TCPAddr{IP: net.ParseIP(source)}}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return err
	}
	return conn.Close()
}

// nftChainLines answers the trimmed rule lines of one chain in an
// `nft list table` rendering, or an error when the listing holds no such
// chain or never closes it.
func nftChainLines(out, chain string) ([]string, error) {
	header := "chain " + chain + " {"
	var lines []string
	inside := false
	for line := range strings.SplitSeq(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if !inside {
			inside = trimmed == header
			continue
		}
		if trimmed == "}" {
			return lines, nil
		}
		lines = append(lines, trimmed)
	}
	return nil, fmt.Errorf("nft listing holds no complete chain %q", chain)
}

// nftLineIndex answers the index of the first line holding every part, or -1.
func nftLineIndex(lines []string, parts ...string) int {
	for i, line := range lines {
		if nftLineHoldsAll(line, parts) {
			return i
		}
	}
	return -1
}

func nftLineHoldsAll(line string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(line, part) {
			return false
		}
	}
	return true
}

// nftCounterPackets reads the packet count of a rendered `counter packets N`.
func nftCounterPackets(line string) (uint64, error) {
	fields := strings.Fields(line)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "packets" {
			return strconv.ParseUint(fields[i+1], 10, 64)
		}
	}
	return 0, fmt.Errorf("no packet counter in nft rule %q", line)
}

func coppWithdraw(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	present := func() bool {
		_, e := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_copp")
		return e == nil
	}
	if !Poll(ctx, 100, 50*time.Millisecond, present) {
		return fmt.Errorf("BEFORE_SHUTDOWN: table missing")
	}
	fmt.Fprintln(os.Stderr, "BEFORE_SHUTDOWN: table present")
	if err := signalProcess(pid, syscall.SIGTERM); err != nil {
		return err
	}
	if !Poll(ctx, 200, 50*time.Millisecond, func() bool { return !present() }) {
		return fmt.Errorf("AFTER_SHUTDOWN: table still present (FAIL)")
	}
	fmt.Fprintln(os.Stderr, "AFTER_SHUTDOWN: table removed (OK)")
	return nil
}

func ddosLocalWithdraw(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	tablePresent := func() bool {
		_, e := netfilterCommandOutput(ctx, "nft", "list", "table", "ip", "ze_ddos-local")
		return e == nil
	}
	if !Poll(ctx, 200, 50*time.Millisecond, tablePresent) {
		return fmt.Errorf("MITIGATION: ze_ddos-local never installed (FAIL)")
	}
	fmt.Fprintln(os.Stderr, "MITIGATION: ze_ddos-local installed")
	file, err := os.Create("ddos-fake.clear")
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	daemonDied := false
	if !Poll(ctx, 200, 50*time.Millisecond, func() bool {
		if !tablePresent() {
			return true
		}
		if !processAlive(pid) {
			daemonDied = true
			return true
		}
		return false
	}) {
		return fmt.Errorf("WITHDRAW: ze_ddos-local still present after clear (FAIL)")
	}
	if daemonDied {
		return fmt.Errorf("WITHDRAW: daemon exited before clearing the table (FAIL)")
	}
	fmt.Fprintln(os.Stderr, "WITHDRAW: ze_ddos-local removed")
	return signalProcess(pid, syscall.SIGTERM)
}

func firewallCLIShow(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		dialer := net.Dialer{Timeout: 200 * time.Millisecond}
		conn, dialErr := dialer.DialContext(ctx, "tcp", "127.0.0.1:2222")
		if dialErr != nil {
			return false
		}
		_ = conn.Close()
		return true
	}) {
		return fmt.Errorf("SSH CLI server never listened on 127.0.0.1:2222")
	}
	clientDir, err := os.MkdirTemp("", "ze-cli-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(clientDir) //nolint:errcheck // fixture cleanup
	env := append(os.Environ(), "ze.config.dir="+clientDir, "ze.ssh.insecure=true")
	initCmd := exec.CommandContext(ctx, "ze", "init")
	initCmd.Env = env
	initCmd.Stdin = strings.NewReader("operator\ntestpass\n127.0.0.1\n2222\n")
	if out, runErr := initCmd.CombinedOutput(); runErr != nil {
		return fmt.Errorf("ze init failed: %s: %w", out, runErr)
	}
	env = append(env, "ze.ssh.password=testpass")
	var last string
	for attempt := range 10 {
		attemptCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		cmd := exec.CommandContext(attemptCtx, "ze", "cli", "--user", "operator", "-c", "show firewall ruleset fw10_004")
		cmd.Env = env
		out, runErr := cmd.CombinedOutput()
		cancel()
		if runErr == nil {
			fmt.Print(string(out))
			return signalProcess(pid, syscall.SIGTERM)
		}
		last = fmt.Sprintf("attempt %d: %s: %v", attempt, out, runErr)
		if !sleepContext(ctx, 100*time.Millisecond) {
			return ctx.Err()
		}
	}
	return fmt.Errorf("ze cli never succeeded: %s", last)
}

func firewallCoexistenceSetup(ctx context.Context, _ []string) error {
	commandIgnore(ctx, "nft", "delete", "table", "inet", "surfprotect")
	commands := [][]string{
		{argAdd, nftTable, nftFamilyInet, nftTableSurfprotect},
		{argAdd, nftChain, nftFamilyInet, nftTableSurfprotect, nftChainInput},
		{argAdd, nftRule, nftFamilyInet, nftTableSurfprotect, nftChainInput, "ip", "saddr", addrPeerOne, nftVerdictAccept},
	}
	for _, args := range commands {
		if _, err := netfilterCommandOutput(ctx, "nft", args...); err != nil {
			return fmt.Errorf("nft %s: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

func firewallCoexistence(ctx context.Context, _ []string) error {
	defer commandIgnore(ctx, "nft", "delete", "table", "inet", "surfprotect")
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	var zeTable string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		zeTable, err = netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_fw10_003")
		return err == nil
	}) {
		return fmt.Errorf("ze_fw10_003 never programmed")
	}
	surfTable, err := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "surfprotect")
	if err != nil {
		return err
	}
	fmt.Printf("ZE:\n%s\nSURF:\n%s", zeTable, surfTable)
	return signalProcess(pid, syscall.SIGTERM)
}

func firewallLegacySeed(ctx context.Context, _ []string) error {
	commands := [][]string{
		{argAdd, nftTable, nftFamilyInet, nftTableFlowspec},
		{argAdd, nftChain, nftFamilyInet, nftTableFlowspec, nftChainFlowspecForward, nftForwardHookSpec},
		{argAdd, nftRule, nftFamilyInet, nftTableFlowspec, nftChainFlowspecForward, "ip", nftMatchDestination, prefixTestNet2, nftVerdictDrop},
		{argAdd, nftTable, "ip", nftTableAnomalyShape},
		{argAdd, nftChain, "ip", nftTableAnomalyShape, nftChainForward, nftForwardHookSpec},
		{argAdd, nftRule, "ip", nftTableAnomalyShape, nftChainForward, "ip", nftMatchDestination, "203.0.113.0/24", nftVerdictDrop},
	}
	for _, args := range commands {
		if _, err := netfilterCommandOutput(ctx, "nft", args...); err != nil {
			return fmt.Errorf("nft %s: %w", strings.Join(args, " "), err)
		}
	}
	out, err := netfilterCommandOutput(ctx, "nft", "list", "ruleset")
	if err == nil {
		fmt.Print(out)
	}
	return err
}

func firewallLegacySweep(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 400)
	if err != nil {
		return err
	}
	var final string
	ok := Poll(ctx, 300, 100*time.Millisecond, func() bool {
		final, _ = netfilterCommandOutput(ctx, "nft", "list", "ruleset")
		return !strings.Contains(final, "table inet flowspec {")
	})
	fmt.Print(final)
	_ = signalProcess(pid, syscall.SIGTERM)
	if !ok {
		return fmt.Errorf("the table an older ze build wrote survived a daemon with no firewall config")
	}
	if strings.Contains(final, prefixTestNet2) {
		return fmt.Errorf("the rule an older ze build installed is still enforcing")
	}
	if !strings.Contains(final, "table ip anomaly-shape {") {
		return fmt.Errorf("ze deleted a table another tool owns, matching on the name alone")
	}
	if !strings.Contains(final, "203.0.113.0/24") {
		return fmt.Errorf("ze deleted another tool's rule")
	}
	return nil
}

func firewallReload(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	var before string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		before, err = netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_fw10_002")
		return err == nil && strings.Contains(before, "dport 22 accept")
	}) {
		return fmt.Errorf("initial state missing dport 22 rule")
	}
	if err := stageReloadConfig(); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}
	var after string
	if !Poll(ctx, 200, 50*time.Millisecond, func() bool {
		after, err = netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_fw10_002")
		return err == nil && !strings.Contains(after, "dport 22 accept") && strings.Contains(after, "dport 443 accept")
	}) {
		return fmt.Errorf("reload did not converge; got:\n%s", after)
	}
	fmt.Printf("reload converged: before=%d bytes, after=%d bytes\n", len(before), len(after))
	return signalProcess(pid, syscall.SIGTERM)
}

func firewallSetElementTimeout(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	var out string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		out, err = netfilterCommandOutput(ctx, "nft", "list", "set", "inet", "ze_fw10_009", "transient")
		return err == nil && strings.Contains(out, "10.0.0.1") && strings.Contains(out, addrPeerTwo)
	}) {
		return fmt.Errorf("set elements were not programmed")
	}
	fmt.Print(out)
	return signalProcess(pid, syscall.SIGTERM)
}

func firewallPersist(ctx context.Context, table string, sig syscall.Signal, phase string) error {
	defer commandIgnore(context.Background(), "nft", "delete", "table", "inet", table)
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	present := func() bool {
		_, e := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", table)
		return e == nil
	}
	if !Poll(ctx, 100, 50*time.Millisecond, present) {
		return fmt.Errorf("BEFORE_%s: table missing", phase)
	}
	fmt.Fprintf(os.Stderr, "BEFORE_%s: table present\n", phase)
	if err := signalProcess(pid, sig); err != nil {
		return err
	}
	waitDead(ctx, pid)
	if !present() {
		return fmt.Errorf("AFTER_%s: table removed (FAIL)", phase)
	}
	fmt.Fprintf(os.Stderr, "AFTER_%s: table persisted (OK)\n", phase)
	return nil
}
