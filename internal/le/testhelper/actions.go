// Design: docs/architecture/core-design.md -- native development command composition
//
// Package testhelper owns long-running stdout producers used by protocol tests.
package testhelper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/ze-software/ze/internal/le/leaction"
)

// Area is the root command name.
const Area = "test-helper"

const dynamicFlow = `flow route {\n match {\n source 10.0.0.1/32;\n destination 1.2.3.4/32;\n }\n then {\n discard;\n }\n }\n`

// Action is one native test-helper command.
type Action struct {
	Action string `json:"action"`
	Usage  string `json:"usage"`
}

// Actions is the structured command inventory.
type Actions struct {
	Actions []Action `json:"actions"`
}

// Answer runs a protocol test helper. SIGINT is deliberately ignored so the
// parent protocol process can perform its normal SIGTERM shutdown sequence.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return actions(), 0
	}
	if len(args) != 1 {
		return refuse("expected exactly one action"), 2
	}

	signal.Ignore(os.Interrupt)
	ctx := context.Background()

	var err error
	switch args[0] {
	case "dynamic":
		err = streamDynamic(ctx, os.Stdout, wait)
	case "watchdog":
		err = streamWatchdog(ctx, os.Stdout, wait)
	default:
		return refuse("unknown action " + args[0]), 2
	}
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return nil, 0
}

func actions() Actions {
	return Actions{Actions: []Action{
		{Action: "dynamic", Usage: "dynamic"},
		{Action: "watchdog", Usage: "watchdog"},
	}}
}

func refuse(message string) any {
	leaction.ReportError(errors.New("test-helper: " + message))
	return nil
}

type waiter func(context.Context, time.Duration) bool

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func streamDynamic(ctx context.Context, out io.Writer, pause waiter) error {
	for {
		if err := writeLine(out, "announce "+dynamicFlow); err != nil {
			return err
		}
		if !pause(ctx, 10*time.Second) {
			return nil
		}
		if err := writeLine(out, "update text nhop set 10.0.0.1 nlri ipv4/unicast add 192.0.2.1/32"); err != nil {
			return err
		}
		if !pause(ctx, 10*time.Second) {
			return nil
		}
		if err := writeLine(out, "update text nlri ipv4/unicast del 192.0.2.1/32"); err != nil {
			return err
		}
		if !pause(ctx, 10*time.Second) {
			return nil
		}
		if err := writeLine(out, "withdraw "+dynamicFlow); err != nil {
			return err
		}
		if !pause(ctx, 10*time.Second) {
			return nil
		}
	}
}

func streamWatchdog(ctx context.Context, out io.Writer, pause waiter) error {
	for {
		if !pause(ctx, 10*time.Second) {
			return nil
		}
		if err := writeLine(out, "bgp watchdog withdraw"); err != nil {
			return err
		}
		if !pause(ctx, 5*time.Second) {
			return nil
		}
		if err := writeLine(out, "bgp watchdog withdraw watchdog-one"); err != nil {
			return err
		}
		if !pause(ctx, 5*time.Second) {
			return nil
		}
		if err := writeLine(out, "bgp watchdog announce"); err != nil {
			return err
		}
		if !pause(ctx, 5*time.Second) {
			return nil
		}
		if err := writeLine(out, "bgp watchdog announce watchdog-one"); err != nil {
			return err
		}
		if !pause(ctx, 5*time.Second) {
			return nil
		}
		if err := writeLine(out, "bgp watchdog announce watchdog-two"); err != nil {
			return err
		}
		if err := writeLine(out, "bgp watchdog withdraw watchdog-two"); err != nil {
			return err
		}
	}
}

func writeLine(out io.Writer, line string) error {
	if _, err := fmt.Fprintln(out, line); err != nil {
		return fmt.Errorf("write test-helper output: %w", err)
	}
	return nil
}
