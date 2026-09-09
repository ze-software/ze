// Design: docs/architecture/config/apply-ordering.md -- the ordered apply path
// Detail: ../../component/config/transaction/solver.go -- tryRelaxCycle, the relaxation the swap driver observes
// Related: misc_fixture_shellports.go -- ifaceAddressSwapDriver, the single-interface renumber driver
//
// The two drivers behind test/reload/config-apply-ordering-address-swap.ci and
// test/reload/config-apply-ordering-mixed-root.ci.
//
// Both read the kernel's own netlink event stream through `ip monitor`. The
// order a transaction applied is not recoverable from the final state, and no
// function on the apply path writes one line per operation, so the event stream
// is the only place the order survives. It is what makes the dual-presence
// window a measurement instead of an inference from "the swap landed at all".
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	Register("reload/config-apply-ordering-address-swap-driver", configApplyOrderingSwapDriver)
	Register("reload/config-apply-ordering-mixed-root-driver", configApplyOrderingMixedRootDriver)
}

// The swap fixture's two interfaces and the two addresses that trade places.
// Each address changes interface, so the iface decomposer emits a create and a
// destroy for each one and the graph closes a four-node cycle.
const (
	swapDeviceLeft   = "zdual0"
	swapDeviceRight  = "zdual1"
	swapAddressLeft  = "10.90.0.1/24"
	swapAddressRight = "10.91.0.1/24"
)

// The mixed-root fixture's one interface, its renumber, and the static route
// that can only be programmed once the new address exists: the gateway sits in
// the new prefix, so the kernel answers ENETUNREACH to a route installed first.
const (
	mixedDevice     = "zmix0"
	mixedAddressOld = "10.92.0.1/24"
	mixedAddressNew = "10.93.0.1/24"
	mixedRoute      = "172.30.0.0/24"
)

// monitorProbeAddress is added and removed on a device the test already owns,
// only so the driver can see its own event come back and know the monitor is
// listening. It is a /32 in a prefix no fixture configures, so it creates no
// route and no interface state the reload will reconcile.
const monitorProbeAddress = "10.99.0.1/32"

// configApplyOrderingSwapDriver proves the dual-presence window on the real
// path: two interfaces trade addresses in one commit, and both addresses stay
// present while they do.
//
// AC-4 says "both addresses are present at the same time" and "neither
// interface is left without an address at any observable point". The observable
// points are the netlink notifications, so the driver replays them over the
// known initial state and checks the two claims on every step.
func configApplyOrderingSwapDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("address swap driver takes no arguments")
	}
	pid, err := awaitOrderingDaemon(ctx)
	if err != nil {
		return err
	}
	settled := func() bool {
		return deviceHasAddress(ctx, swapDeviceLeft, swapAddressLeft) &&
			deviceHasAddress(ctx, swapDeviceRight, swapAddressRight)
	}
	if !Poll(ctx, 300, 100*time.Millisecond, settled) {
		return fmt.Errorf("the interfaces never carried their initial addresses %s and %s", swapAddressLeft, swapAddressRight)
	}

	monitor, err := startNetlinkMonitor(ctx, "address")
	if err != nil {
		return err
	}
	defer monitor.stop()

	if err := awaitMonitorListening(ctx, monitor, swapDeviceLeft); err != nil {
		return err
	}
	if err := reloadOrderingConfig(pid); err != nil {
		return err
	}

	swapped := func() bool {
		return deviceHasAddress(ctx, swapDeviceLeft, swapAddressRight) &&
			!deviceHasAddress(ctx, swapDeviceLeft, swapAddressLeft) &&
			deviceHasAddress(ctx, swapDeviceRight, swapAddressLeft) &&
			!deviceHasAddress(ctx, swapDeviceRight, swapAddressRight)
	}
	if !Poll(ctx, 300, 100*time.Millisecond, swapped) {
		return fmt.Errorf("the addresses never swapped\n%s", monitor.text())
	}

	events := parseKernelEvents(monitor.text())
	if err := writeOrderingEvents("swap-events.txt", events); err != nil {
		return err
	}
	if err := assertSwapWindow(events); err != nil {
		return err
	}
	if err := writeSwapFinalState(ctx, "swap-final.txt"); err != nil {
		return err
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

// assertSwapWindow replays the address notifications over the known initial
// state and checks AC-4 on every step.
//
// Two claims, and each fails a different way. Neither interface is ever left
// bare, which a break-before-make order breaks. Both addresses are present at
// once, which is the window itself: it exists only while every create has run
// and no destroy has.
func assertSwapWindow(events []kernelEvent) error {
	held := map[string]map[string]bool{
		swapDeviceLeft:  {swapAddressLeft: true},
		swapDeviceRight: {swapAddressRight: true},
	}
	window := false
	for _, event := range events {
		if event.kind != kindAddress {
			continue
		}
		addresses, ours := held[event.device]
		if !ours || (event.target != swapAddressLeft && event.target != swapAddressRight) {
			continue
		}
		if event.deleted {
			delete(addresses, event.target)
		} else {
			addresses[event.target] = true
		}
		if len(addresses) == 0 {
			return fmt.Errorf("interface %s was left with no address after %s", event.device, event)
		}
		if len(held[swapDeviceLeft]) == 2 && len(held[swapDeviceRight]) == 2 {
			window = true
		}
	}
	if !window {
		return errors.New("the two addresses were never present at the same time: no dual-presence window")
	}
	var tb textbuf.Buffer
	tb.Str("OK: dual-presence window observed, ").
		Str(swapAddressLeft).Str(" and ").Str(swapAddressRight).
		Str(" both present while the interfaces swapped\n").StdErr() //nolint:errcheck // fixture progress output
	return nil
}

// configApplyOrderingMixedRootDriver proves AC-1: one commit changes an
// interface address and a static route, and the address ordering survives the
// second root joining the transaction.
//
// The static plugin registers no decomposer, so it is the coarse node. The
// order the graph produces is create-address, then the coarse node, then
// destroy-address, and each of the three is a netlink notification.
func configApplyOrderingMixedRootDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("mixed root driver takes no arguments")
	}
	pid, err := awaitOrderingDaemon(ctx)
	if err != nil {
		return err
	}
	if !Poll(ctx, 300, 100*time.Millisecond, func() bool { return deviceHasAddress(ctx, mixedDevice, mixedAddressOld) }) {
		return fmt.Errorf("interface %s never carried its initial address %s", mixedDevice, mixedAddressOld)
	}

	monitor, err := startNetlinkMonitor(ctx, "address", "route")
	if err != nil {
		return err
	}
	defer monitor.stop()

	if err := awaitMonitorListening(ctx, monitor, mixedDevice); err != nil {
		return err
	}
	if err := reloadOrderingConfig(pid); err != nil {
		return err
	}

	applied := func() bool {
		return deviceHasAddress(ctx, mixedDevice, mixedAddressNew) &&
			!deviceHasAddress(ctx, mixedDevice, mixedAddressOld) &&
			routePresent(ctx, mixedRoute)
	}
	if !Poll(ctx, 300, 100*time.Millisecond, applied) {
		return fmt.Errorf("the renumber and the static route did not both land\n%s", monitor.text())
	}

	events := parseKernelEvents(monitor.text())
	if err := writeOrderingEvents("mixed-events.txt", events); err != nil {
		return err
	}
	if err := assertMixedRootOrder(events); err != nil {
		return err
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

// assertMixedRootOrder checks the three notifications arrived in the order the
// operation graph declares, which is what AC-1 states in kernel terms.
func assertMixedRootOrder(events []kernelEvent) error {
	created := indexOfEvent(events, kernelEvent{kind: kindAddress, device: mixedDevice, target: mixedAddressNew})
	installed := indexOfEvent(events, kernelEvent{kind: kindRoute, device: mixedDevice, target: mixedRoute})
	destroyed := indexOfEvent(events, kernelEvent{kind: kindAddress, device: mixedDevice, target: mixedAddressOld, deleted: true})
	switch {
	case created < 0:
		return fmt.Errorf("no notification created %s on %s", mixedAddressNew, mixedDevice)
	case installed < 0:
		return fmt.Errorf("no notification installed the static route %s", mixedRoute)
	case destroyed < 0:
		return fmt.Errorf("no notification removed %s from %s", mixedAddressOld, mixedDevice)
	case created > installed:
		return fmt.Errorf("the static route %s was installed before its address %s existed", mixedRoute, mixedAddressNew)
	case installed > destroyed:
		return fmt.Errorf("the old address %s was removed before the static route %s was installed", mixedAddressOld, mixedRoute)
	}
	var tb textbuf.Buffer
	tb.Str("OK: address ordering held across two roots, ").
		Str(mixedAddressNew).Str(" then ").Str(mixedRoute).Str(" then ").Str(mixedAddressOld).
		Byte('\n').StdErr() //nolint:errcheck // fixture progress output
	return nil
}

// awaitOrderingDaemon waits for the daemon both drivers reload and answers its
// pid.
func awaitOrderingDaemon(ctx context.Context) (int, error) {
	if !waitForFile(ctx, "daemon.pid", 300, 100*time.Millisecond) {
		return 0, errors.New("daemon.pid not found")
	}
	if !waitForFile(ctx, "daemon.ready", 300, 100*time.Millisecond) {
		return 0, errors.New("daemon.ready not found")
	}
	return readPID("daemon.pid")
}

// reloadOrderingConfig installs the second config and asks the daemon to reload
// it, which is the one commit both tests are about.
func reloadOrderingConfig(pid int) error {
	data, err := os.ReadFile(fileConfig2Conf)
	if err != nil {
		return err
	}
	if err := os.WriteFile(fileBGPConf, data, 0o600); err != nil {
		return err
	}
	return syscall.Kill(pid, syscall.SIGHUP)
}

// kindAddress and kindRoute name what a notification is about.
const (
	kindAddress = "address"
	kindRoute   = "route"
)

// kernelEvent is one netlink notification `ip monitor` printed, reduced to the
// four facts these drivers assert on.
type kernelEvent struct {
	deleted bool
	kind    string
	device  string
	target  string
}

func (e kernelEvent) String() string {
	var tb textbuf.Buffer
	verb := "add"
	if e.deleted {
		verb = "del"
	}
	return tb.Str(verb).Byte(' ').Str(e.kind).Byte(' ').Str(e.target).Str(" dev ").Str(e.device).String()
}

// netlinkMonitor is one `ip monitor` child and the text it has printed so far.
//
// The caller MUST call stop, and stop MUST NOT be called before the last read
// of text: the child is what keeps the pipe open.
type netlinkMonitor struct {
	command *exec.Cmd
	output  *lockedBuffer
}

// startNetlinkMonitor starts `ip -4 monitor <objects...>`.
//
// iproute2 flushes stdout after each notification (ip/ipmonitor.c, accept_msg),
// so a reader sees a whole line as soon as the kernel produced it. BusyBox has
// no `ip monitor` at all, which is why the tests behind these drivers declare
// option=needs-linux and the QEMU run installs iproute2.
func startNetlinkMonitor(ctx context.Context, objects ...string) (*netlinkMonitor, error) {
	arguments := append([]string{"-4", "monitor"}, objects...)
	command := exec.CommandContext(ctx, "ip", arguments...) //nolint:gosec // the fixture chooses the program and every argument
	output := new(lockedBuffer)
	command.Stdout = output
	command.Stderr = output
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start ip monitor: %w", err)
	}
	return &netlinkMonitor{command: command, output: output}, nil
}

// text answers everything the monitor has printed so far.
func (m *netlinkMonitor) text() string { return m.output.String() }

// stop ends the child. It MUST be called after the last read of text.
func (m *netlinkMonitor) stop() {
	if m.command.Process == nil {
		return
	}
	_ = m.command.Process.Kill() //nolint:errcheck // the child is going away either way
	_ = m.command.Wait()         //nolint:errcheck // the exit status of a killed monitor says nothing
}

// awaitMonitorListening makes the monitor's readiness a fact rather than a
// wait.
//
// A monitor started but not yet subscribed misses the notifications the reload
// produces, and the test then fails with "the addresses never swapped" for a
// reason that has nothing to do with ordering. So the driver provokes one
// notification of its own on a device the test owns and waits to see it come
// back, then takes it away again.
func awaitMonitorListening(ctx context.Context, monitor *netlinkMonitor, device string) error {
	if out, err := runIP(ctx, "addr", "add", monitorProbeAddress, "dev", device); err != nil {
		return fmt.Errorf("add monitor probe address: %w: %s", err, out)
	}
	seen := func() bool { return strings.Contains(monitor.text(), monitorProbeAddress) }
	if !Poll(ctx, 200, 50*time.Millisecond, seen) {
		return errors.New("ip monitor never reported the probe address, so it was not listening")
	}
	if out, err := runIP(ctx, "addr", "del", monitorProbeAddress, "dev", device); err != nil {
		return fmt.Errorf("remove monitor probe address: %w: %s", err, out)
	}
	gone := func() bool { return !deviceHasAddress(ctx, device, monitorProbeAddress) }
	if !Poll(ctx, 200, 50*time.Millisecond, gone) {
		return errors.New("the monitor probe address stayed on the device")
	}
	return nil
}

// parseKernelEvents reads what `ip monitor` printed.
//
// A notification starts in column zero and its continuation lines are indented,
// so an indented line is skipped. A removal is the same line under a "Deleted "
// prefix.
func parseKernelEvents(text string) []kernelEvent {
	var events []kernelEvent
	for line := range strings.SplitSeq(text, "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		deleted := false
		if after, found := strings.CutPrefix(line, "Deleted "); found {
			deleted, line = true, after
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if at := slices.Index(fields, "inet"); at >= 0 && at+1 < len(fields) {
			events = append(events, kernelEvent{
				deleted: deleted,
				kind:    kindAddress,
				device:  strings.TrimSuffix(fields[1], ":"),
				target:  fields[at+1],
			})
			continue
		}
		at := slices.Index(fields, "dev")
		if at >= 0 && at+1 < len(fields) && strings.Contains(fields[0], "/") {
			events = append(events, kernelEvent{
				deleted: deleted,
				kind:    kindRoute,
				device:  fields[at+1],
				target:  fields[0],
			})
		}
	}
	return events
}

// indexOfEvent answers where want first appears, or -1.
func indexOfEvent(events []kernelEvent, want kernelEvent) int {
	return slices.Index(events, want)
}

// writeOrderingEvents records the ordered stream beside the test's other
// artifacts, so a failure is read rather than reproduced.
func writeOrderingEvents(path string, events []kernelEvent) error {
	var tb textbuf.Buffer
	for _, event := range events {
		tb.Str(event.String()).Byte('\n')
	}
	return os.WriteFile(path, []byte(tb.String()), 0o600)
}

// writeSwapFinalState records which of the four device-and-address pairs the
// kernel still holds once the reload settled, one `device=cidr` line each.
//
// The raw `ip addr show` output would name both addresses whichever way the
// swap went, so the test could assert only that something is there. The pair
// form lets it assert the two that must be present and the two that must not.
func writeSwapFinalState(ctx context.Context, path string) error {
	var tb textbuf.Buffer
	for _, device := range []string{swapDeviceLeft, swapDeviceRight} {
		for _, address := range []string{swapAddressLeft, swapAddressRight} {
			if !deviceHasAddress(ctx, device, address) {
				continue
			}
			tb.Str(device).Byte('=').Str(address).Byte('\n')
		}
	}
	return os.WriteFile(path, []byte(tb.String()), 0o600)
}

// deviceHasAddress reports whether device carries cidr right now.
func deviceHasAddress(ctx context.Context, device, cidr string) bool {
	out, err := runIP(ctx, "-4", "addr", "show", device)
	if err != nil {
		return false
	}
	return strings.Contains(out, cidr)
}

// routePresent reports whether prefix is in the main table right now.
func routePresent(ctx context.Context, prefix string) bool {
	out, err := runIP(ctx, "-4", "route", "show", prefix)
	if err != nil {
		return false
	}
	return strings.Contains(out, prefix)
}

// runIP answers what `ip` printed, on both streams, so a failure carries the
// kernel's own words.
func runIP(ctx context.Context, arguments ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "ip", arguments...).CombinedOutput() //nolint:gosec // the fixture chooses the program and every argument
	return string(out), err
}
