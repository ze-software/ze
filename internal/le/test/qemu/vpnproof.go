// Design: docs/architecture/testing/qemu-integration.md -- VPN proof on Ze's own system.
// Related: dockerlab.go -- native hardware acceleration, image provenance and teardown.
// Related: ../deployment/vpnproof.go -- negotiated ESP transforms and WireGuard traffic.
package testqemu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/appliance"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// vpnGuestPreparation runs only in the disposable snapshot this action boots.
// Disable loading before checking the module list to close the autoload race.
// An already-loaded module refuses the proof rather than supplying hidden support.
const vpnGuestPreparation = `echo 1 > /proc/sys/kernel/modules_disabled
test "$(cat /proc/sys/kernel/modules_disabled)" = 1
test -z "$(cat /proc/modules)"
rm -f /sbin/modprobe
test ! -e /sbin/modprobe
test ! -L /sbin/modprobe
`

// runVPNKernelHere deliberately has no image or kernel override. The shared
// docker-host resolver boots Ze's kernel and base for the native host architecture.
func runVPNKernelHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if err := vpnKernelConfig(root); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	runArgs := leaction.Arguments{keywordLab: {"test deployment vpn-kernel-test"}}
	if timeout := args.One(keywordTimeout); timeout != "" {
		runArgs[keywordTimeout] = []string{timeout}
	}
	ctx, cancel := guestContext()
	defer cancel()
	return runDockerLab(ctx, root, runArgs, vpnGuestPreparation)
}

// vpnKernelConfig checks the emitted config of the same native cache resolver
// EnsureImage uses, not the requested fragment or another staged kernel.
func vpnKernelConfig(root string) error {
	tree, err := appliance.ImageRuntimeKernelTree(GuestArch(), func(arch string) string {
		return "./le setup docker host build arch " + arch
	})
	if err != nil {
		return err
	}
	config, err := os.ReadFile(filepath.Join(tree, "config"))
	if err != nil {
		return fmt.Errorf("read VPN proof emitted kernel config: %w", err)
	}
	for _, manifest := range []string{"kernel.require", "runtime.require"} {
		required, err := os.ReadFile(filepath.Join(root, "gokrazy", "kernel", manifest))
		if err != nil {
			return fmt.Errorf("read VPN proof kernel requirements: %w", err)
		}
		if err := requireVPNBuiltins(string(config), string(required)); err != nil {
			return fmt.Errorf("%s: %w", tree, err)
		}
	}
	return nil
}

func requireVPNBuiltins(config, required string) error {
	enabled := make(map[string]bool)
	for actual := range strings.SplitSeq(config, "\n") {
		if symbol, found := strings.CutSuffix(actual, "=y"); found {
			enabled[symbol] = true
		}
	}
	for line := range strings.SplitSeq(required, "\n") {
		symbol := strings.TrimSpace(line)
		if symbol == "" {
			continue
		}
		if strings.HasPrefix(symbol, "#") {
			continue
		}
		symbol = strings.TrimSuffix(symbol, "=y")
		if !enabled[symbol] {
			return fmt.Errorf("VPN proof requires emitted %s=y", symbol)
		}
	}
	return nil
}
