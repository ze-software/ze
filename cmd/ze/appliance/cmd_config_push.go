// Design: plan/learned/677-appliance-2-remote.md — config push to device via SSH

package appliance

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const sshDialTimeout = 10 * time.Second

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

	addr := net.JoinHostPort(cfg.Device.Address, "22")

	result := sshExecFunc(addr, "config stage", merged)
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "error: device %s unreachable at %s: %v\n", name, addr, result.Err)
		return exitError
	}

	result = sshExecFunc(addr, "config validate staged", "")
	if result.Err != nil {
		sshExecFunc(addr, "config discard staged", "")
		fmt.Fprintf(os.Stderr, "error: device rejected config (validation failed: %s)\n", strings.TrimSpace(result.Output))
		return exitError
	}

	result = sshExecFunc(addr, "config apply staged", "")
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

func sshExecReal(addr, command, stdin string) sshResult {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return sshResult{Err: fmt.Errorf("SSH_AUTH_SOCK not set (start ssh-agent or use eval $(ssh-agent))")}
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

	config := &ssh.ClientConfig{
		User: "admin",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(agentClient.Signers),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // appliance uses self-signed host keys
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
