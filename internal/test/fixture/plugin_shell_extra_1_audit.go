package fixture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func extra1AuditConfig(port int) string {
	return fmt.Sprintf(`system {
	authentication { user admin { password %q; } }
}
environment {
	ssh { enabled true; server main { ip 127.0.0.1; port %d; } }
}
`, extra1AdminHash, port)
}

func extra1AuditAuthFail(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("audit-auth-fail takes no arguments: %q", args)
	}
	config := extra1AuditConfig(0)
	daemon, sshPort, err := extra1RunDaemon(ctx, "audit-auth-fail.conf", config, nil)
	if err != nil {
		return err
	}
	defer func() {
		if daemon != nil {
			daemon.stop()
		}
	}()
	configDir, err := extra1InitCLI(ctx, sshPort)
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup
	if _, err := extra1CLI(ctx, configDir, sshPort, "admin", "wrongpass", "show bgp"); err == nil {
		return fmt.Errorf("wrong SSH password succeeded")
	}
	fmt.Fprintln(os.Stderr, "OK: wrong SSH password denied")
	output, err := extra1CLI(ctx, configDir, sshPort, "admin", "testpass", "show audit action auth-fail")
	if err != nil {
		return fmt.Errorf("show audit failed: %w\n%s", err, output)
	}
	if !extra1ContainsBoth(output, "auth-fail", "admin") {
		return fmt.Errorf("show audit missing auth-fail/admin: %s", output)
	}
	fmt.Fprintln(os.Stderr, "OK: show audit includes auth-fail for admin")
	daemon.stop()
	daemon = nil
	fmt.Fprintln(os.Stderr, "OK: daemon stopped")
	return nil
}

func extra1AuditPersistence(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("audit-persistence takes no arguments: %q", args)
	}
	workDir, err := os.MkdirTemp("", "plugin-shell-extra-1-audit-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir) //nolint:errcheck // fixture cleanup
	configPath := "audit-persistence.conf"
	sshListenPort := 20000 + os.Getpid()%30000
	config := extra1AuditConfig(sshListenPort)
	start := func(logPath string) (*extra1Daemon, string, error) {
		daemon, sshPort, err := extra1RunDaemonIn(ctx, workDir, configPath, logPath, config, nil)
		if err != nil {
			return nil, "", err
		}
		if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
			_, commandErr := extra1Command(sshPort, "admin", "testpass", "show version")
			return commandErr == nil
		}) {
			daemon.stop()
			return nil, "", fmt.Errorf("SSH server did not become ready: %s", daemon.contents())
		}
		return daemon, sshPort, nil
	}
	daemon, sshPort, err := start("daemon1.log")
	if err != nil {
		return err
	}
	configDir, err := extra1InitCLI(ctx, sshPort)
	if err != nil {
		daemon.stop()
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup
	if _, err := extra1CLI(ctx, configDir, sshPort, "admin", "wrongpass", "show bgp"); err == nil {
		daemon.stop()
		return fmt.Errorf("wrong SSH password succeeded")
	}
	fmt.Fprintln(os.Stderr, "OK: wrong SSH password denied")
	auditContents, err := os.ReadFile(filepath.Join(workDir, "audit-persistence.audit.jsonl")) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil || !strings.Contains(string(auditContents), "auth-fail") {
		daemon.stop()
		return fmt.Errorf("audit file missing auth-fail before restart: %s: %w", auditContents, err)
	}
	daemon.stop()

	daemon, sshPort, err = start("daemon2.log")
	if err != nil {
		return err
	}
	defer daemon.stop()
	output, err := extra1CLI(ctx, configDir, sshPort, "admin", "testpass", "show audit action auth-fail")
	if err != nil {
		return fmt.Errorf("show audit failed after restart: %w\n%s", err, output)
	}
	if !extra1ContainsBoth(output, "auth-fail", "admin") {
		return fmt.Errorf("restarted daemon did not load auth-fail: %s", output)
	}
	fmt.Fprintln(os.Stderr, "OK: restarted daemon loaded persisted auth-fail")
	if reloadOutput, err := extra1CLI(ctx, configDir, sshPort, "admin", "testpass", "request reload"); err != nil {
		return fmt.Errorf("request reload failed: %w\n%s", err, reloadOutput)
	}
	reloadAudit, err := extra1CLI(ctx, configDir, sshPort, "admin", "testpass", "show audit action daemon-reload")
	if err != nil {
		return fmt.Errorf("show audit daemon-reload failed: %w\n%s", err, reloadAudit)
	}
	if !extra1ContainsBoth(reloadAudit, "daemon-reload", "admin") {
		return fmt.Errorf("show audit missing daemon-reload/admin: %s", reloadAudit)
	}
	fmt.Fprintln(os.Stderr, "OK: show audit includes daemon reload")
	return nil
}
