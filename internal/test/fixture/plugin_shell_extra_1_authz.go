package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

func extra1AuthzBase(users, profiles string) string {
	return fmt.Sprintf(`bgp {
	peer peer1 {
		connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }
		session { asn { local 65533; remote 65533; } }
	}
}
system {
	authentication { %s }
	authorization { %s }
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port 0; } } }
`, users, profiles)
}

func extra1AuthzAllow(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("authz-allow takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	users := fmt.Sprintf(`
		user admin { password %q; profile [ admin ]; }
		user operator { password %q; profile [ restricted ]; }
	`, extra1AdminHash, extra1OperatorHash)
	profiles := `
		profile admin { run { default-action allow; } edit { default-action allow; } }
		profile restricted {
			run {
				default-action deny;
				entry 10 { action allow; match "show bgp peer list"; }
				entry 20 { action allow; match "show bgp"; }
			}
			edit { default-action deny; }
		}
	`
	daemon, port, err := extra1RunDaemon(ctx, "authz-allow.conf", "daemon.log", extra1AuthzBase(users, profiles), nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", port)
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	if _, err := extra1RequireCommand(port, "admin", "testpass", "show bgp peer list"); err != nil {
		return fmt.Errorf("admin should be allowed to run show bgp peer list: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: admin allowed show bgp peer list")
	if _, err := extra1RequireCommand(port, "operator", "oppass", "show bgp peer list"); err != nil {
		return fmt.Errorf("operator should be allowed to run show bgp peer list: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: operator allowed show bgp peer list (entry match)")
	if _, err := extra1RequireCommand(port, "operator", "oppass", "show bgp"); err != nil {
		return fmt.Errorf("operator should be allowed to run show bgp: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: operator allowed show bgp (entry match)")
	fmt.Fprintln(os.Stderr, "OK: all allow tests passed")
	return nil
}

func extra1AuthzDefault(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("authz-default takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	users := fmt.Sprintf(`
		user operator { password %q; profile [ restricted ]; }
		user unknown { password %q; }
	`, extra1OperatorHash, extra1AdminHash)
	profiles := `profile restricted {
		run { default-action deny; entry 10 { action allow; match "show bgp peer list"; } }
		edit { default-action deny; }
	}`
	daemon, port, err := extra1RunDaemon(ctx, "authz-default.conf", "daemon.log", extra1AuthzBase(users, profiles), nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", port)
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	for _, command := range []string{"show bgp peer list", "show version"} {
		output, err := extra1Command(port, "unknown", "testpass", command)
		if err == nil {
			return fmt.Errorf("unknown user should be denied %s but was allowed: %s", command, output)
		}
		fmt.Fprintf(os.Stderr, "OK: unknown user denied %s (fail closed)\n", command)
	}
	fmt.Fprintln(os.Stderr, "OK: all fail-closed authz tests passed")
	return nil
}

func extra1AuthzDeny(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("authz-deny takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	users := fmt.Sprintf(`user operator { password %q; profile [ restricted ]; }`, extra1OperatorHash)
	profiles := `profile restricted {
		run { default-action deny; entry 10 { action allow; match "show bgp peer list"; } }
		edit { default-action deny; }
	}`
	daemon, port, err := extra1RunDaemon(ctx, "authz-deny.conf", "daemon.log", extra1AuthzBase(users, profiles), nil)
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", port)
	if err := extra1Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	output, err := extra1Command(port, "operator", "oppass", "show version")
	if err == nil {
		return fmt.Errorf("operator should be denied show version but command succeeded: %s", output)
	}
	fmt.Fprintln(os.Stderr, "OK: operator denied show version (exit non-zero)")
	if !strings.Contains(err.Error(), "command restricted by access control") {
		return fmt.Errorf("expected command restricted by access control: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: error message names access control")
	if _, err := extra1Command(port, "operator", "oppass", "show metrics list"); err == nil {
		return fmt.Errorf("operator should be denied show metrics list")
	}
	fmt.Fprintln(os.Stderr, "OK: operator denied show metrics list")
	fmt.Fprintln(os.Stderr, "OK: all deny tests passed")
	return nil
}

func extra1AuthzNoApplicableProfile(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("authz-no-applicable-profile takes no arguments: %q", args)
	}
	if err := extra1TouchReady(); err != nil {
		return err
	}
	config := fmt.Sprintf(`environment {
	ssh { enabled true; server main { ip 127.0.0.1; port 0; } }
}
system {
	authorization {
		profile locked { run { default-action deny; } edit { default-action deny; } }
	}
	authentication { user nobody { password %q; } }
}
`, extra1OperatorHash)
	daemon, port, err := extra1RunDaemon(ctx, "authz-no-applicable-profile.conf", "daemon.log", config, map[string]string{"ze_log_authz": "info"})
	if err != nil {
		return err
	}
	defer daemon.stop()
	fmt.Fprintf(os.Stderr, "ssh on :%s\n", port)
	configDir, err := extra1InitCLI(ctx, port)
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup
	output, err := extra1CLI(ctx, configDir, port, "nobody", "oppass", "show version")
	if err == nil {
		return fmt.Errorf("show version must be refused for a user with no applicable profile: %s", output)
	}
	fmt.Fprintf(os.Stderr, "DENIED-OUTPUT: %s\n", output)
	if !strings.Contains(output, "command restricted by access control") {
		return fmt.Errorf("refusal must name access control: %s", output)
	}
	fmt.Fprintln(os.Stderr, "OK: refusal names access control")
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		return strings.Contains(daemon.contents(), "SSH auth success") && strings.Contains(daemon.contents(), "username=nobody")
	}) {
		return fmt.Errorf("daemon did not record a successful login for nobody: %s", daemon.contents())
	}
	fmt.Fprintln(os.Stderr, "OK: nobody authenticated, then was denied by authorization")
	if !strings.Contains(daemon.contents(), "denied: no applicable profile") {
		return fmt.Errorf("daemon did not log the no-applicable-profile deny reason: %s", daemon.contents())
	}
	fmt.Fprintln(os.Stderr, "OK: daemon logged 'denied: no applicable profile'")
	output, err = extra1CLI(ctx, configDir, port, "nobody", "oppass", "show no-such-command-anywhere")
	if err == nil {
		return fmt.Errorf("a nonexistent command must not succeed: %s", output)
	}
	if !strings.Contains(output, "unknown command") {
		return fmt.Errorf("nonexistent command must report unknown command: %s", output)
	}
	fmt.Fprintln(os.Stderr, "OK: nonexistent command reports unknown command")
	return nil
}
