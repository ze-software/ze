// Design: docs/architecture/testing/ci-format.md — text-mode test plugin

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// textPluginCmd runs a minimal text-mode external plugin for functional testing.
// Uses the SDK's text protocol path (NewTextFromEnv → Run with text framing).
// Registers for update events and logs them to stderr.
func textPluginCmd() int {
	p, err := sdk.NewTextFromEnv("text-test")
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

	// Subscribe to update events at startup (included in stage 5 "ready" message).
	p.SetStartupSubscriptions([]string{"update"}, nil, "")

	reg := sdk.Registration{}
	if err := p.Run(ctx, reg); err != nil {
		fmt.Fprintf(os.Stderr, "text-plugin: run: %v\n", err)
		return 1
	}

	return 0
}
