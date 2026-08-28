package fixture

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func reloadFirewallIRR07(ctx context.Context, marker string) error {
	for {
		ready := true
		for _, name := range []string{"daemon.pid", "daemon.ready", marker} {
			if _, err := os.Stat(name); err != nil {
				ready = false
				break
			}
		}
		if ready {
			break
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	rawPID, err := os.ReadFile("daemon.pid")
	if err != nil {
		return fmt.Errorf("read daemon pid: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil {
		return fmt.Errorf("parse daemon pid: %w", err)
	}
	config, err := os.ReadFile("config2.conf")
	if err != nil {
		return fmt.Errorf("read replacement config: %w", err)
	}
	if err := os.WriteFile("ze-bgp.conf", config, 0o644); err != nil {
		return fmt.Errorf("install replacement config: %w", err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("reload daemon: %w", err)
	}
	if err := os.WriteFile("reload.done", nil, 0o644); err != nil {
		return fmt.Errorf("write reload marker: %w", err)
	}
	return nil
}
