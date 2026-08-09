// Design: docs/architecture/appliance/remote-operations.md -- config push to device via SSH

package appliance

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var errSshAuthSockNotSetStart = errors.New("SSH_AUTH_SOCK not set (start ssh-agent or use eval $(ssh-agent))")

var errNoKnownHostsPath = errors.New("cannot locate known_hosts: $HOME is unset, so the appliance host key cannot be verified")

const (
	sshDialTimeout  = 10 * time.Second
	defaultSSHPort  = "22"
	knownHostsDir   = ".ssh"
	knownHostsFile  = "known_hosts"
	keyscanHintHead = "ssh-keyscan -H "
)

// userKnownHostsPath returns the operator's OpenSSH known_hosts file. Appliance
// host keys are verified against the same file the operator's own ssh(1) uses,
// so there is no second trust store to keep in sync.
func userKnownHostsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, knownHostsDir, knownHostsFile)
}

// applianceHostKeyCallback verifies appliance host keys against the known_hosts
// file at path.
//
// It fails closed: an unreadable known_hosts file, a host that is not listed, a
// revoked key, and a key that does not match the pinned one all refuse the
// connection. An appliance presents a self-signed host key, which is the reason
// it must be pinned rather than trusted: there is no CA to fall back on, so an
// unverified key is an unauthenticated peer, and the config being pushed is
// readable by whoever answers.
func applianceHostKeyCallback(path string) (ssh.HostKeyCallback, error) {
	if path == "" {
		return nil, errNoKnownHostsPath
	}
	verify, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("read known_hosts %s: %w", path, err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := verify(hostname, remote, key)

		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}
		// Want non-empty means the host IS pinned but presented a different key.
		// Never suggest re-scanning here: that would talk the operator through
		// overwriting the pin, which is the one thing an attacker needs.
		if len(keyErr.Want) > 0 {
			return fmt.Errorf("host key mismatch for %s: presented %s, which is not the key pinned in %s",
				hostname, ssh.FingerprintSHA256(key), path)
		}
		return fmt.Errorf("host key for %s not in %s: verify %s out of band, then pin it with %s",
			hostname, path, ssh.FingerprintSHA256(key), keyscanHint(hostname, path))
	}, nil
}

// keyscanHint builds the ssh-keyscan command that pins hostname into path.
func keyscanHint(hostname, path string) string {
	var tb textbuf.Buffer
	tb.Str(keyscanHintHead)

	host, port, err := net.SplitHostPort(hostname)
	switch {
	case err != nil:
		tb.Str(hostname)
	case port == defaultSSHPort:
		tb.Str(host)
	default:
		tb.Str("-p ").Str(port).Byte(' ').Str(host)
	}

	return tb.Str(" >> ").Str(path).String()
}

func init() {
	cmdConfigPush = runConfigPush
}

type sshResult struct {
	Output string
	Err    error
}

var sshExecFunc = sshExecReal

func runConfigPush(args []string) int {
	fs := flag.NewFlagSet("appliance config-push", flag.ContinueOnError)
	allFlag := fs.Bool("all", false, "Push config to all appliances with device.address set")
	dryRunFlag := fs.Bool("dry-run", false, "Print merged config without connecting")
	parallelFlag := fs.Int("parallel", 1, "Number of concurrent pushes (with --all)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance config-push [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ze appliance config-push lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance config-push --dry-run lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance config-push --all\n")
		fmt.Fprintf(os.Stderr, "  ze appliance config-push --all --parallel 4\n")
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if *allFlag {
		return configPushAll(*dryRunFlag, *parallelFlag)
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name> or --all\n")
		fs.Usage()
		return exitError
	}

	return configPushOne(fs.Arg(0), *dryRunFlag)
}

func configPushOne(name string, dryRun bool) int {
	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	merged, err := resolveSeedConfig(dir, name, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if merged == "" {
		fmt.Fprintf(os.Stderr, "error: no config to push (no base, no overlay)\n")
		return exitError
	}

	if dryRun {
		fmt.Print(merged)
		if merged[len(merged)-1] != '\n' {
			fmt.Println()
		}
		return exitOK
	}

	if cfg.Device.Address == "" {
		fmt.Fprintf(os.Stderr, "error: device %s has no address configured\n", name)
		return exitError
	}

	port := cfg.SSH.Port
	if port == "" {
		port = "22"
	}
	addr := net.JoinHostPort(cfg.Device.Address, port)
	user := cfg.Credentials.Username
	if user == "" {
		user = defaultUsername
	}

	result := sshExecFunc(addr, user, "config stage", merged)
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "error: device %s unreachable at %s: %v\n", name, addr, result.Err)
		return exitError
	}

	result = sshExecFunc(addr, user, "config validate staged", "")
	if result.Err != nil {
		sshExecFunc(addr, user, "config discard staged", "")
		fmt.Fprintf(os.Stderr, "error: device rejected config (validation failed: %s)\n", strings.TrimSpace(result.Output))
		return exitError
	}

	result = sshExecFunc(addr, user, "config apply staged", "")
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "error: device %s failed to apply config: %v\n", name, result.Err)
		return exitError
	}

	fmt.Printf("config applied to %s\n", name)
	return exitOK
}

func configPushAll(dryRun bool, parallel int) int {
	names, code := listAddressedAppliances()
	if code != exitOK {
		return code
	}

	return runParallel(names, parallel, func(name string) int {
		fmt.Fprintf(os.Stderr, "pushing config to %s...\n", name)
		return configPushOne(name, dryRun)
	})
}

func sshExecReal(addr, user, command, stdin string) sshResult {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return sshResult{Err: errSshAuthSockNotSetStart}
	}

	var d net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), sshDialTimeout)
	defer cancel()

	agentConn, err := d.DialContext(ctx, "unix", sock)
	if err != nil {
		return sshResult{Err: fmt.Errorf("connect to ssh-agent: %w", err)}
	}
	defer agentConn.Close() //nolint:errcheck // best-effort cleanup

	agentClient := agent.NewClient(agentConn)

	hostKeys, err := applianceHostKeyCallback(userKnownHostsPath())
	if err != nil {
		return sshResult{Err: err}
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(agentClient.Signers),
		},
		HostKeyCallback: hostKeys,
		Timeout:         sshDialTimeout,
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return sshResult{Err: fmt.Errorf("connect to %s: %w", addr, err)}
	}
	defer client.Close() //nolint:errcheck // best-effort cleanup

	session, err := client.NewSession()
	if err != nil {
		return sshResult{Err: fmt.Errorf("create session: %w", err)}
	}
	defer session.Close() //nolint:errcheck // best-effort cleanup

	if stdin != "" {
		session.Stdin = strings.NewReader(stdin)
	}

	output, err := session.CombinedOutput(command)
	if err != nil {
		return sshResult{
			Output: string(output),
			Err:    err,
		}
	}

	return sshResult{Output: strings.TrimSpace(string(output))}
}
