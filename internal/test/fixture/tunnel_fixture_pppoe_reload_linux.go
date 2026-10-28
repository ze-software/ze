//go:build linux

// Design: docs/guide/pppoe.md -- "Changing the configuration": what a reload
// applies to a running access concentrator.
// Related: tunnel_fixture_pppoe_linux.go -- the raw AF_PACKET client this drives.
// Related: register_pppoe_reload_linux.go -- registers tunnelPPPoEReload.

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// tunnelPPPoEReloadAnswerWait bounds how long the driver waits for the AC to
// answer with the expected AC-Name. A reload that is never applied keeps
// answering with the boot configuration, so this is where the defect shows as
// a red.
const tunnelPPPoEReloadAnswerWait = 30 * time.Second

// tunnelPPPoEReloadSilence is how many one-second PADI attempts an interface
// the reload removed must leave unanswered.
const tunnelPPPoEReloadSilence = 3

// tunnelPPPoEReloadTarget is one `<iface>:<ac-name>` argument: the subscriber
// end of a veth pair and the AC-Name the PADO on it must carry.
type tunnelPPPoEReloadTarget struct {
	iface  string
	acName string
}

// tunnelPPPoEReload proves a committed `pppoe` edit reaches the running access
// concentrator. Arguments, each optional except after=:
//
//	before=<iface>:<ac-name>  the PADO on iface carries ac-name before the reload
//	after=<iface>:<ac-name>   after the SIGHUP, a PADO on iface carries ac-name
//	silent=<iface>            after the reload, a PADI on iface gets no PADO
//
// It copies config2.conf over ze-bgp.conf and sends the daemon one SIGHUP, the
// path a `ze config commit` reload takes, then stops the daemon.
func tunnelPPPoEReload(ctx context.Context, args []string) error {
	before, after, silent, err := tunnelPPPoEReloadArgs(args)
	if err != nil {
		return err
	}
	if !waitForFile(ctx, "daemon.pid", 300, 100*time.Millisecond) {
		return errors.New("daemon.pid not found")
	}
	if !waitForFile(ctx, "daemon.ready", 300, 100*time.Millisecond) {
		return errors.New("daemon.ready not found")
	}
	pid, err := readPID("daemon.pid")
	if err != nil {
		return err
	}
	if before.iface != "" {
		got, answerErr := tunnelPPPoEReloadAnswer(ctx, before)
		if answerErr != nil {
			return fmt.Errorf("before the reload: %w", answerErr)
		}
		tunnelPPPoEReloadSay("OK: before reload ", before.iface, " answered AC-Name=", got)
	}
	data, err := os.ReadFile(fileConfig2Conf)
	if err != nil {
		return err
	}
	if err := os.WriteFile(fileBGPConf, data, 0o600); err != nil {
		return err
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return err
	}
	got, err := tunnelPPPoEReloadAnswer(ctx, after)
	if err != nil {
		return fmt.Errorf("after the reload: %w", err)
	}
	tunnelPPPoEReloadSay("OK: after reload ", after.iface, " answered AC-Name=", got)
	if silent != "" {
		if err := tunnelPPPoEReloadSilent(ctx, silent); err != nil {
			return err
		}
		tunnelPPPoEReloadSay("OK: after reload ", silent, " left PADI unanswered")
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

// tunnelPPPoEReloadSay prints one progress line on stdout, where the .ci
// expectations read it.
func tunnelPPPoEReloadSay(parts ...string) {
	var line textbuf.Buffer
	line.Reset()
	for _, part := range parts {
		line.Str(part)
	}
	line.Byte('\n')
	_ = line.StdOut() //nolint:errcheck // progress output; a lost line fails the .ci expectation that reads it
}

// tunnelPPPoEReloadArgs parses the key=value arguments tunnelPPPoEReload takes.
func tunnelPPPoEReloadArgs(args []string) (before, after tunnelPPPoEReloadTarget, silent string, err error) {
	for _, arg := range args {
		key, value, found := strings.Cut(arg, "=")
		if !found {
			return before, after, "", fmt.Errorf("argument %q is not key=value", arg)
		}
		switch key {
		case "before", "after":
			name, acName, ok := strings.Cut(value, ":")
			if !ok {
				return before, after, "", fmt.Errorf("argument %q is not %s=<iface>:<ac-name>", arg, key)
			}
			target := tunnelPPPoEReloadTarget{iface: name, acName: acName}
			if key == "before" {
				before = target
			} else {
				after = target
			}
		case "silent":
			silent = value
		default:
			return before, after, "", fmt.Errorf("unknown argument %q: use before=, after= or silent=", key)
		}
	}
	if after.iface == "" {
		return before, after, "", errors.New("after=<iface>:<ac-name> is required")
	}
	return before, after, silent, nil
}

// tunnelPPPoEReloadAnswer sends PADI on target.iface until a PADO carries
// target.acName, or tunnelPPPoEReloadAnswerWait runs out. On a timeout the
// error names the last AC-Name seen, so a red says whether the AC answered with
// the old configuration or not at all.
func tunnelPPPoEReloadAnswer(ctx context.Context, target tunnelPPPoEReloadTarget) (string, error) {
	wire, err := tunnelPPPoEOpen(target.iface)
	if err != nil {
		return "", err
	}
	defer wire.close()

	broadcast := net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	deadline := time.Now().Add(tunnelPPPoEReloadAnswerWait)
	last := "no PADO"
	for time.Now().Before(deadline) {
		packet := tunnelPPPoEPacket(tunnelPPPoEPADI, nil, []byte{0x52, 0x4c}, "")
		_, _, tags, exchangeErr := wire.exchange(ctx, broadcast, packet, tunnelPPPoEPADO, 1)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		if exchangeErr != nil {
			continue
		}
		name := tags[tunnelPPPoEACName]
		if bytes.Equal(name, []byte(target.acName)) {
			return string(name), nil
		}
		last = "AC-Name=" + string(name)
		time.Sleep(200 * time.Millisecond)
	}
	return "", fmt.Errorf("no PADO on %s carried AC-Name=%s within %s; last answer: %s",
		target.iface, target.acName, tunnelPPPoEReloadAnswerWait, last)
}

// tunnelPPPoEReloadSilent fails when a PADI on iface is answered: the reload
// removed that interface from the AC, so nothing may answer on it.
func tunnelPPPoEReloadSilent(ctx context.Context, iface string) error {
	wire, err := tunnelPPPoEOpen(iface)
	if err != nil {
		return err
	}
	defer wire.close()

	broadcast := net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	packet := tunnelPPPoEPacket(tunnelPPPoEPADI, nil, []byte{0x52, 0x53}, "")
	_, _, tags, exchangeErr := wire.exchange(ctx, broadcast, packet, tunnelPPPoEPADO, tunnelPPPoEReloadSilence)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if exchangeErr == nil {
		return fmt.Errorf("after the reload %s still answered PADI with AC-Name=%s: the removed interface is still served",
			iface, tags[tunnelPPPoEACName])
	}
	return nil
}
