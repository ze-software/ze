//go:build linux

// Design: docs/architecture/iface/logical-name-resolution.md -- a mirror destination is resolved when the mirror spec is built
// Related: register_iface_mirror_linux.go -- the names these drivers register under
// Related: routing_fixture_linux.go -- addRoutingLink and routingDaemonPID, shared with the VRRP drivers

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The devices the delete-refused driver creates: one veth pair, whose first end
// mirrors its ingress to the second.
const (
	mirrorDeleteSource = "zmtsrc0"
	mirrorDeleteTarget = "zmtdst0"
)

// The three configs the delete-refused driver walks through, in order. The
// running config mirrors source -> target. The refused one deletes the target's
// entry while the mirror still names it. The accepted one drops the mirror and
// the target together, which is the order the refusal asks for.
const (
	mirrorDeleteConfigBoot = `interface {
	backend kernel;
	ethernet zmtsrc0 {
		unit 0 {
			mirror {
				ingress zmtdst0
			}
		}
	}
	ethernet zmtdst0 {
	}
}
`
	mirrorDeleteConfigRefused = `interface {
	backend kernel;
	ethernet zmtsrc0 {
		unit 0 {
			mirror {
				ingress zmtdst0
			}
		}
	}
}
`
	mirrorDeleteConfigAccepted = `interface {
	backend kernel;
	ethernet zmtsrc0 {
		unit 0 {
		}
	}
}
`
)

// The daemon's own reload outcome lines (cmd/ze/hub/main_reload.go): one
// stable phrase per outcome, and neither a prefix of the other.
const (
	mirrorReloadRefusedLine  = "reload error:"
	mirrorReloadAcceptedLine = "sighup reload complete"
)

// mirrorPollDelay is the cadence of every wait in these drivers. Each wait's
// attempt count comes from the runner's budget (WaitAttempts).
const mirrorPollDelay = 100 * time.Millisecond

// ifaceMirrorTargetDeleteRefused proves, on a real kernel, that a reload which
// deletes a live mirror target is refused and leaves the traffic-control mirror installed,
// and that the reload removing the mirror and the target together retires it.
//
// Method: the driver creates a veth pair and starts its own daemon, because it
// must read the outcome of each reload before it sends the next, and a runner
// daemon's stderr is not readable from a fixture. Every step waits on state:
// the mirred action on the source's ingress hook, then the daemon's refusal
// line, then its completion line. The kernel is read after each outcome line,
// so no assertion rides on a delay.
func ifaceMirrorTargetDeleteRefused(ctx context.Context, args []string) (retErr error) {
	if len(args) != 0 {
		return errors.New("tx-iface-mirror-target-delete-refused takes no arguments")
	}
	if err := addMirrorVethPair(mirrorDeleteSource, mirrorDeleteTarget, nil); err != nil {
		return err
	}
	defer func() {
		// Deleting one end deletes the pair, and the kernel drops its traffic-control
		// state with it.
		err := deleteMirrorLink(mirrorDeleteSource)
		if retErr == nil {
			retErr = err
		}
	}()
	target, err := netlink.LinkByName(mirrorDeleteTarget)
	if err != nil {
		return fmt.Errorf("find %s: %w", mirrorDeleteTarget, err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	configDir, err := os.MkdirTemp(workDir, "mirror-daemon-")
	if err != nil {
		return err
	}
	configPath := filepath.Join(workDir, "mirror-target.conf")
	if err := os.WriteFile(configPath, []byte(mirrorDeleteConfigBoot), 0o600); err != nil {
		return err
	}
	readyPath := filepath.Join(workDir, fileDaemonReady)
	env := miscEnvironment(map[string]string{envConfigDir: configDir, "ZE_READY_FILE": readyPath})
	daemon, err := startFixtureProcess(ctx, env, "ze", actionStart, configPath)
	if err != nil {
		return fmt.Errorf("start ze: %w", err)
	}
	defer stopFixtureProcess(daemon, 5*time.Second)

	// Four waits share the budget, and each ends as soon as its state holds.
	attempts := WaitAttempts(20, mirrorPollDelay, 200)
	if !waitForFile(ctx, readyPath, attempts, mirrorPollDelay) {
		return fmt.Errorf("the daemon wrote no ready file: %s", daemon.output.String())
	}
	var hook mirrorHook
	var readErr error
	if !Poll(ctx, attempts, mirrorPollDelay, func() bool {
		hook, readErr = readMirrorHook(mirrorDeleteSource, netlink.HANDLE_MIN_INGRESS)
		return readErr == nil && hook.mirrorsTo(target.Attrs().Index)
	}) {
		if readErr != nil {
			return fmt.Errorf("the boot config's mirror %s -> %s never reached the kernel (read error %w, ingress hook %s)",
				mirrorDeleteSource, mirrorDeleteTarget, readErr, hook)
		}
		return fmt.Errorf("the boot config's mirror %s -> %s never reached the kernel (ingress hook %s)",
			mirrorDeleteSource, mirrorDeleteTarget, hook)
	}
	if err := mirrorReport("MIRROR-INSTALLED " + mirrorDeleteSource + " " + mirrorDeleteTarget); err != nil {
		return err
	}

	if err := reloadMirrorConfig(ctx, daemon, configPath, mirrorDeleteConfigRefused, mirrorReloadRefusedLine, attempts); err != nil {
		return err
	}
	if strings.Contains(daemon.output.String(), mirrorReloadAcceptedLine) {
		return fmt.Errorf("the reload deleting the mirror target completed: %s", daemon.output.String())
	}
	hook, err = readMirrorHook(mirrorDeleteSource, netlink.HANDLE_MIN_INGRESS)
	if err != nil {
		return err
	}
	if !hook.mirrorsTo(target.Attrs().Index) {
		return fmt.Errorf("the refused reload changed the kernel mirror: ingress hook of %s holds %s, want a mirror to %s",
			mirrorDeleteSource, hook, mirrorDeleteTarget)
	}
	if err := mirrorReport("MIRROR-KEPT-AFTER-REFUSAL " + mirrorDeleteSource + " " + mirrorDeleteTarget); err != nil {
		return err
	}

	if err := reloadMirrorConfig(ctx, daemon, configPath, mirrorDeleteConfigAccepted, mirrorReloadAcceptedLine, attempts); err != nil {
		return err
	}
	for _, parent := range []uint32{netlink.HANDLE_MIN_INGRESS, netlink.HANDLE_MIN_EGRESS} {
		hook, err = readMirrorHook(mirrorDeleteSource, parent)
		if err != nil {
			return err
		}
		if hook.filters != 0 {
			return fmt.Errorf("the reload that removed the mirror left traffic-control state on %s: hook %x holds %s", mirrorDeleteSource, parent, hook)
		}
	}
	return mirrorReport("MIRROR-RETIRED " + mirrorDeleteSource)
}

// reloadMirrorConfig writes config over the running daemon's file, sends one
// SIGHUP, and returns once the daemon printed outcome one more time than it had
// before the signal. It never waits on a delay alone: the outcome line is the
// daemon's own statement that the reload finished.
func reloadMirrorConfig(ctx context.Context, daemon *fixtureProcess, path, config, outcome string, attempts int) error {
	before := strings.Count(daemon.output.String(), outcome)
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		return err
	}
	if err := syscall.Kill(daemon.command.Process.Pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("signal the daemon: %w", err)
	}
	if !Poll(ctx, attempts, mirrorPollDelay, func() bool {
		return strings.Count(daemon.output.String(), outcome) > before
	}) {
		return fmt.Errorf("the reload never printed %q: %s", outcome, daemon.output.String())
	}
	return nil
}

// The devices the VLAN-target selector test builds. The mirror's source is one
// end of a veth pair. The other end is the capture port, which the config
// selects by MAC under the logical name zvmcap. A dummy literally named zvmcap,
// with a VLAN zvmcap.100, is the decoy: it carries the logical name of the
// entry and of its unit, so a mirror installed toward the logical name lands on
// it rather than failing.
const (
	mirrorVLANSource     = "zvms0"
	mirrorVLANCapture    = "zvmt0"
	mirrorVLANCaptureMAC = "02:5a:45:00:4d:01"
	mirrorVLANDecoy      = "zvmcap"
	mirrorVLANID         = 100
	mirrorVLANSuffix     = ".100"
)

// ifaceMirrorVLANTargetSetup builds the devices before the daemon starts, so
// the boot apply binds the selector on its first pass.
func ifaceMirrorVLANTargetSetup(_ context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("iface-mirror-vlan-target-selector-setup takes no arguments")
	}
	mac, err := net.ParseMAC(mirrorVLANCaptureMAC)
	if err != nil {
		return err
	}
	if err := addMirrorVethPair(mirrorVLANSource, mirrorVLANCapture, mac); err != nil {
		return err
	}
	decoyAttrs := netlink.NewLinkAttrs()
	decoyAttrs.Name = mirrorVLANDecoy
	if err := addRoutingLink(&netlink.Dummy{LinkAttrs: decoyAttrs}); err != nil {
		return fmt.Errorf("add decoy %s: %w", mirrorVLANDecoy, err)
	}
	decoy, err := netlink.LinkByName(mirrorVLANDecoy)
	if err != nil {
		return fmt.Errorf("find decoy %s: %w", mirrorVLANDecoy, err)
	}
	vlanAttrs := netlink.NewLinkAttrs()
	vlanAttrs.Name = mirrorVLANDecoy + mirrorVLANSuffix
	vlanAttrs.ParentIndex = decoy.Attrs().Index
	if err := addRoutingLink(&netlink.Vlan{LinkAttrs: vlanAttrs, VlanId: mirrorVLANID}); err != nil {
		return fmt.Errorf("add decoy VLAN %s: %w", vlanAttrs.Name, err)
	}
	fmt.Println("MIRROR-VLAN-SETUP")
	return nil
}

// ifaceMirrorVLANTargetDriver waits for the boot apply's mirror on the source's
// egress hook, then proves it copies to the VLAN on the device the selector
// bound, and not to the decoy the logical name names.
func ifaceMirrorVLANTargetDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("iface-mirror-vlan-target-selector-driver takes no arguments")
	}
	if _, err := routingDaemonPID(ctx); err != nil {
		return err
	}
	wantName := mirrorVLANCapture + mirrorVLANSuffix
	decoyName := mirrorVLANDecoy + mirrorVLANSuffix
	decoy, err := netlink.LinkByName(decoyName)
	if err != nil {
		return fmt.Errorf("setup built no decoy %s: %w", decoyName, err)
	}
	attempts := WaitAttempts(50, mirrorPollDelay, 300)
	var hook mirrorHook
	var readErr error
	if !Poll(ctx, attempts, mirrorPollDelay, func() bool {
		hook, readErr = readMirrorHook(mirrorVLANSource, netlink.HANDLE_MIN_EGRESS)
		return readErr == nil && hook.filters > 0
	}) {
		if readErr != nil {
			return fmt.Errorf("no mirror reached the egress hook of %s (read error %w)", mirrorVLANSource, readErr)
		}
		return fmt.Errorf("no mirror reached the egress hook of %s", mirrorVLANSource)
	}
	if hook.mirrorsTo(decoy.Attrs().Index) {
		return fmt.Errorf("the mirror on %s copies to the DECOY %s: the VLAN unit was taken by its logical name instead of through its parent's selector",
			mirrorVLANSource, decoyName)
	}
	want, err := netlink.LinkByName(wantName)
	if err != nil {
		return fmt.Errorf("the daemon built no VLAN %s on the selected device: %w", wantName, err)
	}
	if !hook.mirrorsTo(want.Attrs().Index) {
		return fmt.Errorf("the mirror on %s holds %s, want a copy to %s (ifindex %d)", mirrorVLANSource, hook, wantName, want.Attrs().Index)
	}
	fmt.Println("MIRROR-TO " + wantName)
	fmt.Println("DECOY-UNTOUCHED " + decoyName)
	return nil
}

// mirrorReport prints one progress token on stdout, where the .ci asserts it.
// A token carries no colon, which the .ci directive grammar would split on.
func mirrorReport(line string) error {
	var tb textbuf.Buffer
	return tb.Str(line).Byte('\n').StdOut()
}

// addMirrorVethPair creates the veth pair name <-> peer, sets peer's address to
// peerMAC when one is given, and brings both ends up. A pair that exists is
// kept, as addRoutingLink keeps any device.
func addMirrorVethPair(name, peer string, peerMAC net.HardwareAddr) error {
	attrs := netlink.NewLinkAttrs()
	attrs.Name = name
	if err := addRoutingLink(&netlink.Veth{LinkAttrs: attrs, PeerName: peer, PeerHardwareAddr: peerMAC}); err != nil {
		return fmt.Errorf("add veth %s <-> %s: %w", name, peer, err)
	}
	for _, end := range []string{name, peer} {
		link, err := netlink.LinkByName(end)
		if err != nil {
			return fmt.Errorf("find %s: %w", end, err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("bring %s up: %w", end, err)
		}
	}
	return nil
}

// deleteMirrorLink deletes one device the driver created. A device already gone
// is not an error: the end state is the one asked for.
func deleteMirrorLink(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		if _, notFound := errors.AsType[netlink.LinkNotFoundError](err); notFound {
			return nil
		}
		return fmt.Errorf("find %s to delete it: %w", name, err)
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("delete %s: %w", name, err)
	}
	return nil
}

// mirrorHook is what one clsact hook of a device holds: how many tc filters,
// and the ifindex of every device a mirred action among them copies to.
type mirrorHook struct {
	filters  int
	mirredTo []int
}

// mirrorsTo reports whether a mirred action on the hook copies to ifindex.
func (h mirrorHook) mirrorsTo(ifindex int) bool {
	return slices.Contains(h.mirredTo, ifindex)
}

// String renders the hook for a failure message.
func (h mirrorHook) String() string {
	var tb textbuf.Buffer
	tb.Int(int64(h.filters)).Str(" filter(s), mirred to ifindex [")
	for i, index := range h.mirredTo {
		if i > 0 {
			tb.Byte(' ')
		}
		tb.Int(int64(index))
	}
	return tb.Byte(']').String()
}

// readMirrorHook reads one clsact hook of the device called name. A device with
// no clsact qdisc, or a hook with no filter, holds nothing. Every other failure
// to read is an error, never an empty hook: a caller asserting that the mirror
// is gone would read an empty answer as the proof.
func readMirrorHook(name string, parent uint32) (mirrorHook, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return mirrorHook{}, fmt.Errorf("find %s: %w", name, err)
	}
	filters, err := netlink.FilterList(link, parent)
	if err != nil {
		if errors.Is(err, syscall.ENOENT) {
			return mirrorHook{}, nil
		}
		if errors.Is(err, syscall.EINVAL) {
			return mirrorHook{}, nil
		}
		return mirrorHook{}, fmt.Errorf("list tc filters on %s hook %x: %w", name, parent, err)
	}
	hook := mirrorHook{filters: len(filters)}
	for _, filter := range filters {
		matchAll, ok := filter.(*netlink.MatchAll)
		if !ok {
			continue
		}
		for _, action := range matchAll.Actions {
			mirred, ok := action.(*netlink.MirredAction)
			if !ok {
				continue
			}
			hook.mirredTo = append(hook.mirredTo, mirred.Ifindex)
		}
	}
	return hook, nil
}
