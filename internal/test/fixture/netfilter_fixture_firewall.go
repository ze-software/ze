package fixture

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func registerTableSnapshot(name, family, table string) {
	Register(name, func(ctx context.Context, _ []string) error {
		pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
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
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
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
	trusted, limit := -1, -1
	for i, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "ip saddr 192.0.2.0/24") && strings.Contains(line, "accept") {
			trusted = i
		}
		if strings.Contains(line, "limit rate") {
			limit = i
		}
	}
	if trusted >= 0 && limit >= 0 {
		if trusted >= limit {
			return fmt.Errorf("ORDER_FAIL: trusted after limit")
		}
		fmt.Fprintln(os.Stderr, "ORDER_OK: trusted before limit")
	}
	return signalProcess(pid, syscall.SIGTERM)
}

func coppWithdraw(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
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
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
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
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
	if err != nil {
		return err
	}
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		conn, dialErr := net.DialTimeout("tcp", "127.0.0.1:2222", 200*time.Millisecond)
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
	defer os.RemoveAll(clientDir)
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
		{"add", "table", "inet", "surfprotect"},
		{"add", "chain", "inet", "surfprotect", "input"},
		{"add", "rule", "inet", "surfprotect", "input", "ip", "saddr", "10.0.0.1", "accept"},
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
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
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
		{"add", "table", "inet", "flowspec"},
		{"add", "chain", "inet", "flowspec", "flowspec-fwd", "{ type filter hook forward priority -1 ; policy accept ; }"},
		{"add", "rule", "inet", "flowspec", "flowspec-fwd", "ip", "daddr", "198.51.100.0/24", "drop"},
		{"add", "table", "ip", "anomaly-shape"},
		{"add", "chain", "ip", "anomaly-shape", "forward", "{ type filter hook forward priority -1 ; policy accept ; }"},
		{"add", "rule", "ip", "anomaly-shape", "forward", "ip", "daddr", "203.0.113.0/24", "drop"},
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
	pid, err := waitDaemon(ctx, 400, 50*time.Millisecond)
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
	if strings.Contains(final, "198.51.100.0/24") {
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
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
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
	if err := copyFile("config2.conf", "ze-bgp.conf"); err != nil {
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
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
	if err != nil {
		return err
	}
	var out string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		out, err = netfilterCommandOutput(ctx, "nft", "list", "set", "inet", "ze_fw10_009", "transient")
		return err == nil && strings.Contains(out, "10.0.0.1") && strings.Contains(out, "10.0.0.2")
	}) {
		return fmt.Errorf("set elements were not programmed")
	}
	fmt.Print(out)
	return signalProcess(pid, syscall.SIGTERM)
}

func firewallPersist(ctx context.Context, table string, sig syscall.Signal, phase string) error {
	defer commandIgnore(context.Background(), "nft", "delete", "table", "inet", table)
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
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
