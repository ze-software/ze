package fixture

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func extra1RadiusAdmin(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("aaa-radius-admin takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	mock, address, err := extra1StartRadiusMock(ctx)
	if err != nil {
		return err
	}
	defer extra1StopRadiusMock(mock)
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "mock at %s\n", address)
	config := fmt.Sprintf(`bgp {
	peer peer1 {
		connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }
		session { asn { local 65533; remote 65533; } }
	}
}
system {
	authentication {
		radius {
			server %s { port %s; key "ze-mock-key"; }
			timeout 2;
		}
	}
	authorization {
		profile admin { run { default-action allow; } edit { default-action allow; } }
	}
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port 0; } } }
`, host, port)
	daemon, sshPort, err := extra1RunDaemon(ctx, "aaa-radius-admin.conf", "daemon.log", config, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "ssh on :%s\n", sshPort)
	configDir, err := extra1InitCLI(ctx, sshPort)
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	if output, err := extra1CLI(ctx, configDir, sshPort, "admin", "testpass", "show bgp"); err != nil {
		return fmt.Errorf("show summary via RADIUS auth: %w\nOUTPUT: %s\n%s", err, output, daemon.contents())
	}
	fmt.Fprintln(os.Stderr, "OK: summary ran via RADIUS auth")
	if err := extra1Wait(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		return strings.Contains(daemon.contents(), "auth success") && strings.Contains(daemon.contents(), "source=radius")
	}) {
		return fmt.Errorf("daemon did not record RADIUS authentication: %s", daemon.contents())
	}
	fmt.Fprint(os.Stderr, daemon.contents())
	return nil
}

func extra1RadiusFallback(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("aaa-radius-fallback takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	config := fmt.Sprintf(`bgp {
	peer peer1 {
		connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }
		session { asn { local 65533; remote 65533; } }
	}
}
system {
	authentication {
		user admin { password %q; profile [ admin ]; }
		radius {
			server 127.0.0.1 { port 1; key "ignored"; }
			timeout 1;
			retries 1;
		}
	}
	authorization {
		profile admin { run { default-action allow; } edit { default-action allow; } }
	}
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port 0; } } }
`, extra1AdminHash)
	daemon, sshPort, err := extra1RunDaemon(ctx, "aaa-radius-fallback.conf", "daemon.log", config, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "ssh on :%s\n", sshPort)
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	if _, err := extra1RequireCommand(sshPort, "admin", "testpass", "show bgp"); err != nil {
		return fmt.Errorf("show summary via local fallback after RADIUS unreachable: %w\n%s", err, daemon.contents())
	}
	fmt.Fprintln(os.Stderr, "OK: summary ran via local fallback")
	if err := extra1Wait(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		return strings.Contains(daemon.contents(), "auth success") && strings.Contains(daemon.contents(), "source=local")
	}) {
		return fmt.Errorf("daemon did not record local fallback authentication: %s", daemon.contents())
	}
	if strings.Contains(daemon.contents(), "auth success") && strings.Contains(daemon.contents(), "source=radius") {
		return fmt.Errorf("RADIUS silently succeeded: %s", daemon.contents())
	}
	fmt.Fprint(os.Stderr, daemon.contents())
	return nil
}

func extra1AnswerUnknownCommand(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("answer-unknown-command takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	config := fmt.Sprintf(`bgp {
	peer peer1 {
		connection { remote { ip 127.0.0.2; } local { ip 127.0.0.1; accept false; } }
		session { asn { local 65533; remote 65533; } }
	}
}
system {
	authentication { user admin { password %q; profile [ admin ]; } }
	authorization { profile admin { run { default-action allow; } edit { default-action allow; } } }
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port 0; } } }
`, extra1AdminHash)
	daemon, sshPort, err := extra1RunDaemon(ctx, "answer-unknown-command.conf", "daemon.log", config, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", sshPort)
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	output, commandErr := extra1Command(sshPort, "admin", "testpass", "shwo bgp peers")
	if commandErr == nil {
		return fmt.Errorf("an unknown command succeeded: %s", output)
	}
	if output != "" {
		return fmt.Errorf("an unknown command wrote to stdout: %s", output)
	}
	if !strings.Contains(commandErr.Error(), "unknown command") {
		return fmt.Errorf("failure does not name the unknown command: %w", commandErr)
	}
	for _, prefix := range []string{"top ", "row ", "bad ", "end ", "nay "} {
		if strings.HasPrefix(commandErr.Error(), prefix) {
			return fmt.Errorf("a frame line reached the operator: %w", commandErr)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: the typo was reported as an unknown command")
	if _, err := extra1RequireCommand(sshPort, "admin", "testpass", "show version"); err != nil {
		return fmt.Errorf("a command that exists did not answer: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: a command that exists is not reported as unknown")
	return nil
}
