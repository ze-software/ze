package fixture

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"
)

func sleepFixture(duration time.Duration) Driver {
	return func(ctx context.Context, _ []string) error {
		if !sleepContext(ctx, duration) {
			return ctx.Err()
		}
		return nil
	}
}

func trafficReloadQdisc(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	if !sleepContext(ctx, 500*time.Millisecond) {
		return ctx.Err()
	}
	// A failed show MUST stop the fixture: an empty tc-after.txt would satisfy
	// the .ci's not-contains=htb with no kernel state read.
	before, err := netfilterCommandOutput(ctx, "tc", "qdisc", "show", "dev", "eth0")
	if err != nil {
		return fmt.Errorf("tc qdisc show dev eth0 before reload: %w", err)
	}
	if err := os.WriteFile("tc-before.txt", []byte(before), 0o600); err != nil {
		return err
	}
	if err := stageReloadConfig(); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}
	if !sleepContext(ctx, 2*time.Second) {
		return ctx.Err()
	}
	after, err := netfilterCommandOutput(ctx, "tc", "qdisc", "show", "dev", "eth0")
	if err != nil {
		return fmt.Errorf("tc qdisc show dev eth0 after reload: %w", err)
	}
	if err := os.WriteFile("tc-after.txt", []byte(after), 0o600); err != nil {
		return err
	}
	return signalProcess(pid, syscall.SIGTERM)
}
