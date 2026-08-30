package fixture

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func p12ReloadTrigger(ctx context.Context, _ []string) error {
	for _, path := range []string{fileDaemonPID, fileDaemonReady, "observer.initial-ok"} {
		if err := p12WaitForFile(ctx, path); err != nil {
			return err
		}
	}
	if err := p12CopyFile("config2.conf", "ze-bgp.conf"); err != nil {
		return err
	}
	if err := p12SignalDaemon("daemon.pid"); err != nil {
		return err
	}
	return os.WriteFile("reload.done", nil, 0o600)
}

func p12WaitForFile(ctx context.Context, path string) error {
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func p12CopyFile(source, destination string) error {
	data, err := os.ReadFile(source) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
	}
	if err := os.WriteFile(destination, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}
	return nil
}

func p12SignalDaemon(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("signal daemon %d: %w", pid, err)
	}
	return nil
}

func p12ReloadL2TP(field string, beforeWant, afterWant int) p12Scenario {
	return func(ctx context.Context, plugin *sdk.Plugin) error {
		read := func() (int, bool) {
			status, data, err := p12DispatchObject(ctx, plugin, "show l2tp config")
			if err != nil || status != statusDone {
				return 0, false
			}
			value, exists := data[field]
			if !exists {
				return 0, false
			}
			return p12Number(value)
		}
		var before int
		var ok bool
		if !Poll(ctx, 40, 100*time.Millisecond, func() bool {
			before, ok = read()
			return ok
		}) {
			return fmt.Errorf("before: show l2tp config did not complete")
		}
		if before != beforeWant {
			return fmt.Errorf("before SIGHUP: %s=%d want %d", field, before, beforeWant)
		}
		if err := os.WriteFile("observer.initial-ok", []byte("ok"), 0o600); err != nil {
			return fmt.Errorf("write observer.initial-ok: %w", err)
		}
		if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
			_, err := os.Stat("reload.done")
			return err == nil
		}) {
			return fmt.Errorf("trigger did not create reload.done marker")
		}
		var after int
		if !Poll(ctx, 50, 100*time.Millisecond, func() bool {
			after, ok = read()
			return ok && after == afterWant
		}) {
			return fmt.Errorf("after SIGHUP: %s=%d want %d (reload did not apply)", field, after, afterWant)
		}
		return nil
	}
}
