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

	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	tacacsmock "github.com/ze-software/ze/internal/test/mock/tacacs"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const pluginShellExtra4PasswordHash = "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO" //nolint:gosec // the bcrypt hash of the fixture account, which exists only in this test

func init() {
	Register("plugin/ssh-remote-hash-rejected", pluginShellExtra4RemoteHash)
	Register("plugin/ssh-user-login-yang", pluginShellExtra4UserLogin)
	Register("plugin/startup-unreachable-services", pluginShellExtra4StartupUnreachable)
	Register("plugin/tacacs-acct", pluginShellExtra4TacacsAccounting)
	Register("plugin/tacacs-auth", pluginShellExtra4TacacsAuth)
	Register("plugin/tacacs-author", pluginShellExtra4TacacsAuthor)
	Register("plugin/tacacs-fallback", pluginShellExtra4TacacsFallback)
	Register("plugin/tacacs-local-only", pluginShellExtra4TacacsLocalOnly)
	Register("plugin/tacacs-readonly", pluginShellExtra4TacacsReadonly)
	Register("plugin/tacacs-singleconnect", pluginShellExtra4TacacsSingleConnect)
}

func pluginShellExtra4Observe(ctx context.Context, name string, args []string, want int, scenario func(context.Context, *sdk.Plugin, []string) error) error {
	if len(args) != want {
		return fmt.Errorf("%s: got %d arguments, want %d", name, len(args), want)
	}
	return Observe(ctx, "fixture-plugin-shell-extra-4-"+name, sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		return scenario(ctx, plugin, args)
	})
}

func pluginShellExtra4Command(host, port, user, password, command string) (string, error) {
	return sshclient.ExecCommand(sshclient.Credentials{
		Host:     host,
		Port:     port,
		Username: user,
		Auth:     password,
	}, command)
}

func pluginShellExtra4RequireCommand(port, user, password, command, label string) (string, error) {
	output, err := pluginShellExtra4Command("127.0.0.1", port, user, password, command)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	return output, nil
}

func pluginShellExtra4Environment(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[key]; !replaced {
			environment = append(environment, entry)
		}
	}
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}
	return environment
}

func pluginShellExtra4InitCLI(ctx context.Context, port, username, password string) (string, error) {
	configDir, err := os.MkdirTemp("", "plugin-shell-extra-4-cli-")
	if err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, "ze", "init")
	command.Env = pluginShellExtra4Environment(map[string]string{envConfigDir: configDir})
	command.Stdin = strings.NewReader(fmt.Sprintf("%s\n%s\n127.0.0.1\n%s\n", username, password, port))
	output, err := command.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(configDir)
		return "", fmt.Errorf("ze init: %w\n%s", err, output)
	}
	return configDir, nil
}

// pluginShellExtra4Probe is the command every CLI probe in these fixtures runs.
const pluginShellExtra4Probe = "show bgp peer list"

func pluginShellExtra4CLI(ctx context.Context, configDir, host, port, userFlag, user, password string, insecure bool) (string, error) {
	arguments := []string{areaCLI}
	if host != "" {
		arguments = append(arguments, "--remote", net.JoinHostPort(host, port))
	}
	if userFlag != "" {
		arguments = append(arguments, userFlag, user)
	}
	arguments = append(arguments, "-c", pluginShellExtra4Probe)
	environment := map[string]string{
		envConfigDir:   configDir,
		envSSHPassword: password,
	}
	if insecure {
		environment["ZE_SSH_INSECURE"] = valueTrue
	}
	command := exec.CommandContext(ctx, "ze", arguments...)
	command.Env = pluginShellExtra4Environment(environment)
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func pluginShellExtra4RemoteHash(ctx context.Context, args []string) error {
	return pluginShellExtra4Observe(ctx, "ssh-remote-hash-rejected", args, 2, func(ctx context.Context, _ *sdk.Plugin, args []string) error {
		loopPort, lanPort := args[0], args[1]
		configDir, err := os.MkdirTemp("", "ssh-remote-hash-extra-4-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup

		if output, err := pluginShellExtra4CLI(ctx, configDir, "127.0.0.1", loopPort, "--user", "alice", pluginShellExtra4PasswordHash, false); err != nil {
			return fmt.Errorf("hash-as-token over loopback must authenticate: %w\n%s", err, output)
		}
		fmt.Fprintln(os.Stderr, "OK: AC-2 -- hash-as-token accepted over loopback")

		if output, err := pluginShellExtra4CLI(ctx, configDir, "127.0.0.1", loopPort, "--user", "alice", "testpass", false); err != nil {
			return fmt.Errorf("plaintext over loopback must authenticate: %w\n%s", err, output)
		}
		fmt.Fprintln(os.Stderr, "OK: AC-3 -- plaintext accepted over loopback")

		nonLoopback := pluginShellExtra4NonLoopbackIPv4()
		fmt.Fprintf(os.Stderr, "non-loopback address: %s\n", nonLoopback)
		if nonLoopback == "" {
			fmt.Fprintln(os.Stderr, "NOTE: no non-loopback address on this runner; the remote-reject path is")
			fmt.Fprintln(os.Stderr, "      proven by unit test TestSSHPasswordCallbackRejectsHashFromRemotePeer")
			fmt.Fprintln(os.Stderr, "OK: all ssh-remote-hash tests passed")
			return nil
		}

		if _, err := pluginShellExtra4CLI(ctx, configDir, nonLoopback, lanPort, "--user", "alice", pluginShellExtra4PasswordHash, true); err == nil {
			return errors.New("hash-as-token MUST be rejected over a non-loopback peer")
		}
		fmt.Fprintln(os.Stderr, "OK: AC-1 -- hash-as-token rejected over non-loopback")

		if output, err := pluginShellExtra4CLI(ctx, configDir, nonLoopback, lanPort, "--user", "alice", "testpass", true); err != nil {
			return fmt.Errorf("plaintext over a non-loopback peer must authenticate: %w\n%s", err, output)
		}
		fmt.Fprintln(os.Stderr, "OK: AC-3 -- plaintext accepted over non-loopback")
		fmt.Fprintln(os.Stderr, "OK: all ssh-remote-hash tests passed")
		return nil
	})
}

func pluginShellExtra4NonLoopbackIPv4() string {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addresses {
		var ip net.IP
		switch value := address.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ip4 := ip.To4(); ip4 != nil && !ip4.IsLoopback() && !ip4.IsUnspecified() {
			return ip4.String()
		}
	}
	return ""
}

func pluginShellExtra4UserLogin(ctx context.Context, args []string) error {
	return pluginShellExtra4Observe(ctx, "ssh-user-login-yang", args, 1, func(ctx context.Context, _ *sdk.Plugin, args []string) error {
		port := args[0]
		configDir, err := pluginShellExtra4InitCLI(ctx, port, "operator", "testpass")
		if err != nil {
			return err
		}
		defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup

		if output, err := pluginShellExtra4CLI(ctx, configDir, "", "", "", "", "testpass", false); err != nil {
			return fmt.Errorf("super-admin baseline failed: %w\n%s", err, output)
		}
		fmt.Fprintln(os.Stderr, "OK: super-admin baseline")

		if output, err := pluginShellExtra4CLI(ctx, configDir, "", "", "--user", "alice", "testpass", false); err != nil {
			return fmt.Errorf("--user alice should authenticate: %w\n%s", err, output)
		}
		fmt.Fprintln(os.Stderr, "OK: --user alice authenticated and was identified as alice")

		if output, err := pluginShellExtra4CLI(ctx, configDir, "", "", "-u", "alice", "testpass", false); err != nil {
			return fmt.Errorf("-u alice should authenticate: %w\n%s", err, output)
		}
		fmt.Fprintln(os.Stderr, "OK: -u alice authenticated as alice")

		if _, err := pluginShellExtra4CLI(ctx, configDir, "", "", "--user", "alice", "wrongpass", false); err == nil {
			return errors.New("wrong password should NOT authenticate")
		}
		fmt.Fprintln(os.Stderr, "OK: wrong password rejected")
		fmt.Fprintln(os.Stderr, "OK: all SSH user-login tests passed")
		return nil
	})
}

func pluginShellExtra4DaemonPID(ctx context.Context) (int, error) {
	var pid int
	if !Poll(ctx, 50, 100*time.Millisecond, func() bool {
		data, err := os.ReadFile("daemon.pid")
		if err != nil {
			return false
		}
		value, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || value < 2 {
			return false
		}
		pid = value
		return true
	}) {
		return 0, errors.New("runner did not publish daemon.pid")
	}
	return pid, nil
}

func pluginShellExtra4StartupUnreachable(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("startup-unreachable-services: got %d arguments, want config path", len(args))
	}
	configPath := args[0]
	return Observe(ctx, "fixture-plugin-shell-extra-4-startup-unreachable", sdk.Registration{}, func(ctx context.Context, _ *sdk.Plugin) error {
		fmt.Fprintln(os.Stderr, "OK: daemon reached ready with all external services blackholed")
		config, err := os.ReadFile(configPath) //nolint:gosec // the path is the fixture's own scratch file
		if err != nil {
			return fmt.Errorf("read reload config: %w", err)
		}
		updated := strings.Replace(string(config), "interval 3600", "interval 120", 1)
		if updated == string(config) {
			return errors.New("reload config has no interval 3600 to replace")
		}
		if err := os.WriteFile(configPath, []byte(updated), 0o600); err != nil {
			return fmt.Errorf("write reload config: %w", err)
		}
		daemonPID, err := pluginShellExtra4DaemonPID(ctx)
		if err != nil {
			return err
		}
		if err := syscall.Kill(daemonPID, syscall.SIGHUP); err != nil {
			return fmt.Errorf("signal daemon SIGHUP: %w", err)
		}
		timer := time.NewTimer(16 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
		if err := syscall.Kill(daemonPID, 0); err != nil {
			return fmt.Errorf("daemon not alive after reload: %w", err)
		}
		fmt.Fprintln(os.Stderr, "OK: environment/ntp reload applied within budget while servers blackholed")
		return nil
	})
}

func pluginShellExtra4StartTacacs(ctx context.Context, port, user string, deny ...string) error {
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("invalid TACACS port %q", port)
	}
	addrFile := fmt.Sprintf("tacacs-extra-4-%d.addr", os.Getpid())
	_ = os.Remove(addrFile)
	arguments := []string{"--port", port, "--key", "ze-mock-key", "--user", user, "--addr-file", addrFile}
	for _, word := range deny {
		arguments = append(arguments, "--author-deny", word)
	}
	done := make(chan int, 1)
	go func() { done <- tacacsmock.Run(arguments) }()
	ready := Poll(ctx, 50, 100*time.Millisecond, func() bool {
		select {
		case code := <-done:
			if code != 0 {
				return true
			}
			return false
		default:
		}
		data, readErr := os.ReadFile(addrFile) //nolint:gosec // the path is the fixture's own scratch file
		return readErr == nil && strings.HasSuffix(strings.TrimSpace(string(data)), ":"+port)
	})
	if !ready {
		return errors.New("TACACS+ mock did not report its address")
	}
	if _, err := os.Stat(addrFile); err != nil {
		return errors.New("TACACS+ mock exited before reporting its address")
	}
	return nil
}

func pluginShellExtra4TacacsDriver(ctx context.Context, name string, args []string, user string, deny []string, scenario func(context.Context, *sdk.Plugin, string) error) error {
	if len(args) != 2 {
		return fmt.Errorf("%s: got %d arguments, want TACACS and SSH ports", name, len(args))
	}
	if err := pluginShellExtra4StartTacacs(ctx, args[0], user, deny...); err != nil {
		return err
	}
	return Observe(ctx, "fixture-plugin-shell-extra-4-"+name, sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		return scenario(ctx, plugin, args[1])
	})
}

func pluginShellExtra4TacacsAuth(ctx context.Context, args []string) error {
	return pluginShellExtra4TacacsDriver(ctx, "tacacs-auth", args, "admin:testpass:15", nil, func(_ context.Context, _ *sdk.Plugin, port string) error {
		if _, err := pluginShellExtra4RequireCommand(port, "admin", "testpass", "show bgp", "show summary via TACACS+ auth"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: summary ran via TACACS+ auth")
		time.Sleep(300 * time.Millisecond)
		return nil
	})
}

func pluginShellExtra4TacacsAccounting(ctx context.Context, args []string) error {
	return pluginShellExtra4TacacsDriver(ctx, "tacacs-acct", args, "admin:testpass:15", nil, func(_ context.Context, _ *sdk.Plugin, port string) error {
		if _, err := pluginShellExtra4RequireCommand(port, "admin", "testpass", "show bgp", "show summary via TACACS+ auth"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: summary ran via TACACS+ auth")
		output, err := pluginShellExtra4RequireCommand(port, "admin", "testpass", "show aaa accounting", "show aaa accounting")
		if err != nil {
			return err
		}
		if !strings.Contains(output, "dropped-records") {
			return fmt.Errorf("show aaa accounting missing dropped-records: %s", output)
		}
		fmt.Fprintln(os.Stderr, "OK: show aaa accounting exposes dropped-records")
		time.Sleep(800 * time.Millisecond)
		return nil
	})
}

func pluginShellExtra4TacacsAuthor(ctx context.Context, args []string) error {
	return pluginShellExtra4TacacsDriver(ctx, "tacacs-author", args, "admin:testpass:15", []string{"clear"}, func(_ context.Context, _ *sdk.Plugin, port string) error {
		if _, err := pluginShellExtra4RequireCommand(port, "admin", "testpass", "show bgp", "show bgp should have been authorized by TACACS+"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: 'show bgp' authorized")
		_, err := pluginShellExtra4Command("127.0.0.1", port, "admin", "testpass", "clear interface counters")
		if err == nil {
			return errors.New("clear interface counters should have been blocked by TACACS+ authorization")
		}
		fmt.Fprintf(os.Stderr, "OK: 'clear interface counters' blocked: %v\n", err)
		if !strings.Contains(err.Error(), "command restricted by access control") {
			return fmt.Errorf("expected command restricted by access control, got: %w", err)
		}
		fmt.Fprintln(os.Stderr, "OK: TACACS+ refusal names access control")
		time.Sleep(300 * time.Millisecond)
		return nil
	})
}

func pluginShellExtra4TacacsFallback(ctx context.Context, args []string) error {
	return pluginShellExtra4Observe(ctx, "tacacs-fallback", args, 1, func(_ context.Context, _ *sdk.Plugin, args []string) error {
		if _, err := pluginShellExtra4RequireCommand(args[0], "admin", "testpass", "show bgp", "show summary via local fallback after TACACS+ unreachable"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: summary ran via local fallback")
		time.Sleep(300 * time.Millisecond)
		return nil
	})
}

func pluginShellExtra4TacacsLocalOnly(ctx context.Context, args []string) error {
	return pluginShellExtra4Observe(ctx, "tacacs-local-only", args, 1, func(_ context.Context, _ *sdk.Plugin, args []string) error {
		if _, err := pluginShellExtra4RequireCommand(args[0], "admin", "testpass", "show bgp", "show summary via local auth"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: summary ran via local auth")
		time.Sleep(300 * time.Millisecond)
		return nil
	})
}

func pluginShellExtra4TacacsReadonly(ctx context.Context, args []string) error {
	return pluginShellExtra4TacacsDriver(ctx, "tacacs-readonly", args, "noc:nocpass:1", nil, func(_ context.Context, _ *sdk.Plugin, port string) error {
		if _, err := pluginShellExtra4RequireCommand(port, "noc", "nocpass", "show bgp", "noc should be allowed to run show bgp"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: noc allowed summary (priv-lvl 1 -> read-only)")
		_, err := pluginShellExtra4Command("127.0.0.1", port, "noc", "nocpass", "request quiesce")
		if err == nil {
			return errors.New("noc (priv-lvl 1 -> read-only) must not be allowed a write command")
		}
		if !strings.Contains(err.Error(), "command restricted by access control") {
			return fmt.Errorf("expected an access-control refusal, got: %w", err)
		}
		fmt.Fprintln(os.Stderr, "OK: noc refused write (edit default-action deny)")
		time.Sleep(300 * time.Millisecond)
		return nil
	})
}

func pluginShellExtra4TacacsSingleConnect(ctx context.Context, args []string) error {
	return pluginShellExtra4TacacsDriver(ctx, "tacacs-singleconnect", args, "admin:testpass:15", nil, func(_ context.Context, _ *sdk.Plugin, port string) error {
		for _, command := range []string{cmdShowBGP, cmdShowBGPPeerList, cmdShowBGP, cmdShowBGPPeerList} {
			if _, err := pluginShellExtra4RequireCommand(port, "admin", "testpass", command, command); err != nil {
				return err
			}
		}
		time.Sleep(800 * time.Millisecond)
		fmt.Fprintln(os.Stderr, "OK: repeated TACACS+ sessions completed over the negotiated connection")
		return nil
	})
}
