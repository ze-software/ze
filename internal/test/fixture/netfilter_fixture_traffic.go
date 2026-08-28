package fixture

import (
	"context"
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
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
	if err != nil {
		return err
	}
	if !sleepContext(ctx, 500*time.Millisecond) {
		return ctx.Err()
	}
	before, _ := netfilterCommandOutput(ctx, "tc", "qdisc", "show", "dev", "eth0")
	if err := os.WriteFile("tc-before.txt", []byte(before), 0o600); err != nil {
		return err
	}
	if err := copyFile("config2.conf", "ze-bgp.conf"); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}
	if !sleepContext(ctx, 2*time.Second) {
		return ctx.Err()
	}
	after, _ := netfilterCommandOutput(ctx, "tc", "qdisc", "show", "dev", "eth0")
	if err := os.WriteFile("tc-after.txt", []byte(after), 0o600); err != nil {
		return err
	}
	return signalProcess(pid, syscall.SIGTERM)
}
