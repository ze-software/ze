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

type reloadSignalPlan struct {
	source, destination string
	before, after       time.Duration
	hups                int
	terminate           bool
	requireReady        bool
}

func init() {
	for name, plan := range map[string]reloadSignalPlan{
		"reload/config-apply-ordering-create-trigger": {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/reload-add-bgp-trigger":               {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/reload-add-peer-trigger":              {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/reload-dynamic-peer-survives-trigger": {source: fileConfig2Conf, destination: fileBGPConf, before: 3 * time.Second, after: 8 * time.Second, hups: 1, terminate: true, requireReady: true},
		"reload/reload-plugin-only-no-change-trigger": {before: 200 * time.Millisecond, after: time.Second, hups: 1, terminate: true},
		"reload/tx-bgp-rollback-trigger":              {source: "bad-config.conf", destination: fileBGPConf, before: 2 * time.Second, after: 2 * time.Second, hups: 1, terminate: true, requireReady: true},
		"reload/tx-iface-apply-trigger":               {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/tx-iface-bgp-chain-trigger":           {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/tx-iface-tunnel-create-trigger":       {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/tx-iface-tunnel-modify-key-trigger":   {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/tx-iface-tunnel-remove-trigger":       {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/tx-iface-wireguard-apply-trigger":     {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/tx-iface-wireguard-modify-trigger":    {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/tx-iface-wireguard-remove-trigger":    {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/tx-protocol-exclusion-trigger":        {source: fileConfig2Conf, destination: fileBGPConf, hups: 2, requireReady: true},
		"reload/tx-protocol-external-plugin-trigger":  {source: "updated.conf", destination: fileBGPConf, before: 2 * time.Second, after: 2 * time.Second, hups: 1, terminate: true, requireReady: true},
		"reload/tx-protocol-rollback-trigger":         {source: "bad-config.conf", destination: fileBGPConf, before: 2 * time.Second, after: 2 * time.Second, hups: 1, terminate: true, requireReady: true},
		"reload/tx-protocol-sighup-trigger":           {source: fileConfig2Conf, destination: fileBGPConf, hups: 1, requireReady: true},
		"reload/pki-reference-reload-trigger":         {source: "addref.conf", destination: "hub.conf", after: 3 * time.Second, hups: 1, terminate: true},
		"reload/pki-reference-reload-broken-trigger":  {source: "broken.conf", destination: "hub.conf", after: 3 * time.Second, hups: 1, terminate: true},
	} {
		Register(name, reloadSignalDriver(plan))
	}
	Register("reload/tx-iface-address-swap-driver", ifaceAddressSwapDriver)
	Register("reload/reload-aaa-radius-secret-rotation", radiusSecretRotationDriver)
	Register("runner/stop-background-holder", stopBackgroundHolder)
	Register("runner/stop-background-wait", stopBackgroundWait)
	Register("runner/stop-background-assert", stopBackgroundAssert)
}

func reloadSignalDriver(plan reloadSignalPlan) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return errors.New("reload trigger takes no arguments")
		}
		if !waitForFile(ctx, "daemon.pid", 300, 100*time.Millisecond) {
			return errors.New("daemon.pid not found")
		}
		if plan.requireReady && !waitForFile(ctx, "daemon.ready", 300, 100*time.Millisecond) {
			return errors.New("daemon.ready not found")
		}
		pid, err := readPID("daemon.pid")
		if err != nil {
			return err
		}
		if plan.before != 0 {
			if err := waitDuration(ctx, plan.before); err != nil {
				return err
			}
		}
		if plan.source != "" {
			data, err := os.ReadFile(plan.source)
			if err != nil {
				return err
			}
			if err := os.WriteFile(plan.destination, data, 0o600); err != nil {
				return err
			}
		}
		for range plan.hups {
			if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
				return err
			}
		}
		if plan.after != 0 {
			if err := waitDuration(ctx, plan.after); err != nil {
				return err
			}
		}
		if plan.terminate {
			return syscall.Kill(pid, syscall.SIGTERM)
		}
		return nil
	}
}

func waitDuration(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func ifaceAddressSwapDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("address swap driver takes no arguments")
	}
	if !waitForFile(ctx, "daemon.pid", 300, 100*time.Millisecond) || !waitForFile(ctx, "daemon.ready", 300, 100*time.Millisecond) {
		return errors.New("daemon readiness files missing")
	}
	addressPresent := func(want string) bool {
		out, _ := exec.CommandContext(ctx, "ip", "-4", "addr", "show", "zswap0").CombinedOutput()
		return strings.Contains(string(out), want)
	}
	if !Poll(ctx, 300, 100*time.Millisecond, func() bool { return addressPresent("10.77.0.1/24") }) {
		return errors.New("initial address did not appear")
	}
	data, err := os.ReadFile("config2.conf")
	if err != nil {
		return err
	}
	if err := os.WriteFile("ze-bgp.conf", data, 0o600); err != nil {
		return err
	}
	pid, err := readPID("daemon.pid")
	if err != nil {
		return err
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return err
	}
	Poll(ctx, 100, 100*time.Millisecond, func() bool { return addressPresent("10.77.0.2/24") })
	out, _ := exec.CommandContext(ctx, "ip", "-4", "addr", "show", "zswap0").CombinedOutput()
	if err := os.WriteFile("addr-after.txt", out, 0o600); err != nil {
		return err
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

func stopBackgroundHolder(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("holder takes no arguments")
	}
	if err := os.WriteFile("holder.pid", []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}
func stopBackgroundWait(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("wait takes no arguments")
	}
	if !waitForFile(ctx, "holder.pid", 2_000_000, time.Microsecond) {
		return errors.New("holder.pid never appeared")
	}
	return nil
}
func stopBackgroundAssert(_ context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("assert takes no arguments")
	}
	pid, err := readPID("holder.pid")
	if err != nil {
		return err
	}
	if err := syscall.Kill(pid, 0); err == nil || errors.Is(err, syscall.EPERM) {
		return fmt.Errorf("holder-still-alive pid=%d", pid)
	}
	fmt.Fprintln(os.Stdout, "holder-stopped") //nolint:errcheck // progress output
	return nil
}

func radiusSecretRotationDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("RADIUS rotation takes no arguments")
	}
	if err := os.Remove("mock.addr"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.WriteFile("daemon.ready", nil, 0o600); err != nil {
		return err
	}
	mock, err := startFixtureProcess(ctx, os.Environ(), "", "ze-test", "radius-mock", "--port", "0", "--key", "ze-mock-key", "--user", "admin:testpass:admin", "--addr-file", "mock.addr")
	if err != nil {
		return err
	}
	defer stopFixtureProcess(mock, 2*time.Second)
	if !Poll(ctx, 30, 100*time.Millisecond, func() bool { info, e := os.Stat("mock.addr"); return e == nil && info.Size() > 0 }) {
		return errors.New("RADIUS mock did not report address")
	}
	address, _ := os.ReadFile("mock.addr")
	host, port, err := net.SplitHostPort(strings.TrimSpace(string(address)))
	if err != nil {
		return err
	}
	daemonDir, err := os.MkdirTemp("", "radius-daemon-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(daemonDir) //nolint:errcheck // fixture cleanup
	bgpPort := 10000 + os.Getpid()%20000
	config := func(key string) string {
		return fmt.Sprintf("bgp { peer peer1 { connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } } session { asn { local 65533; remote 65533; } } } }\nsystem { authentication { radius { server %s { port %s; key %q; } timeout 2; } } authorization { profile admin { run { default-action allow; } edit { default-action allow; } } } }\nenvironment { ssh { enabled true; server main { ip 127.0.0.1; port 0; } } }\n", host, port, key)
	}
	path := "reload-aaa-radius-rotation.conf"
	if err := os.WriteFile(path, []byte(config("wrong-key")), 0o600); err != nil {
		return err
	}
	env := miscEnvironment(map[string]string{envConfigDir: daemonDir, envTestBGPPort: strconv.Itoa(bgpPort)})
	var daemon lockedBuffer
	command := exec.CommandContext(ctx, "ze", "start", path)
	command.Env = env
	command.Stderr = &daemon
	command.Stdout = &daemon
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	defer stopManagedProcess(command, done)
	sshPort := ""
	readSSHPort := func() (string, bool) {
		found := ""
		for field := range strings.FieldsSeq(daemon.String()) {
			if after, ok := strings.CutPrefix(field, "address=127.0.0.1:"); ok {
				found = strings.Trim(after, "\"")
			}
		}
		return found, found != ""
	}
	findPort := func() bool {
		sshPort, _ = readSSHPort()
		return sshPort != ""
	}
	if !Poll(ctx, 100, 200*time.Millisecond, findPort) {
		return fmt.Errorf("SSH server did not start: %s", daemon.String())
	}
	adminDir, err := os.MkdirTemp("", "radius-admin-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(adminDir) //nolint:errcheck // fixture cleanup
	adminEnv := miscEnvironment(map[string]string{envConfigDir: adminDir})
	if _, err := commandOutput(ctx, "", adminEnv, "admin\ntestpass\n127.0.0.1\n"+sshPort+"\n", "ze", "init"); err != nil {
		return err
	}
	loginEnv := miscEnvironment(map[string]string{envConfigDir: adminDir, envSSHPassword: valueTestPassword})
	if _, err := commandOutput(ctx, "", loginEnv, "", "ze", "cli", "-c", "show bgp"); err == nil {
		return errors.New("admin authenticated while configured secret was wrong")
	}
	fmt.Fprintln(os.Stderr, "OK: the wrong shared secret is refused")
	if err := os.WriteFile(path, []byte(config("ze-mock-key")), 0o600); err != nil {
		return err
	}
	if err := syscall.Kill(command.Process.Pid, syscall.SIGHUP); err != nil {
		return err
	}
	if !Poll(ctx, 100, 200*time.Millisecond, func() bool { return strings.Contains(daemon.String(), "sighup reload complete") }) {
		return fmt.Errorf("reload did not complete: %s", daemon.String())
	}
	fmt.Fprintln(os.Stderr, "OK: reload complete")
	reloadedPort, ok := readSSHPort()
	if !ok {
		return fmt.Errorf("no SSH listener after reload: %s", daemon.String())
	}
	if reloadedPort != sshPort {
		sshPort = reloadedPort
		adminDir, err = os.MkdirTemp("", "radius-admin-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(adminDir) //nolint:errcheck // fixture cleanup
		adminEnv = miscEnvironment(map[string]string{envConfigDir: adminDir})
		if _, err := commandOutput(ctx, "", adminEnv, "admin\ntestpass\n127.0.0.1\n"+sshPort+"\n", "ze", "init"); err != nil {
			return err
		}
		loginEnv = miscEnvironment(map[string]string{envConfigDir: adminDir, envSSHPassword: valueTestPassword})
		fmt.Fprintf(os.Stderr, "ssh moved to :%s\n", sshPort)
	}
	if _, err := commandOutput(ctx, "", loginEnv, "", "ze", "cli", "-c", "show bgp"); err != nil {
		return fmt.Errorf("rotated secret did not authenticate: %w\n%s", err, daemon.String())
	}
	fmt.Fprintln(os.Stderr, "OK: the rotated shared secret authenticates without a restart")
	fmt.Fprint(os.Stderr, daemon.String())
	return nil
}
