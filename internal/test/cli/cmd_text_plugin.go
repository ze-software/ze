// Design: docs/architecture/testing/ci-format.md — test plugin runner


package cli

import (
	"codeberg.org/thomas-mangin/ze/internal/core/subdispatch"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

func init() {
	Register("text-plugin", cmdTextPlugin, subdispatch.SubMeta{Desc: "Run minimal text-mode plugin (for .ci tests)"})
}

func cmdTextPlugin(_ []string) int {
	p, err := sdk.NewFromEnv("text-test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "text-plugin: init: %v\n", err)
		return 1
	}

	p.OnEvent(func(event string) error {
		fmt.Fprintf(os.Stderr, "text-plugin: event: %s\n", event)
		return nil
	})

	p.OnBye(func(reason string) {
		fmt.Fprintf(os.Stderr, "text-plugin: bye: %s\n", reason)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	p.SetStartupSubscriptions([]string{"update"}, nil, "")

	reg := sdk.Registration{}
	if err := p.Run(ctx, reg); err != nil {
		fmt.Fprintf(os.Stderr, "text-plugin: run: %v\n", err)
		return 1
	}

	return 0
}
