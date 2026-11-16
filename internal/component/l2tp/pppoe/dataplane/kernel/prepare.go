// Design: docs/architecture/l2tp/bng-5-pppoe.md -- kernel access preparation
// Related: factory.go -- kernel session provider

package kernel

import (
	"context"
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// Prepare reads the selected Linux access device through the shared logical
// name resolver. It owns no OS interface and changes no interface state. The
// caller MUST Close the returned access after its sessions and discovery stop.
// Safe for concurrent use, subject to the active iface backend's contract.
func Prepare(ctx context.Context, name string) (dataplane.Access, error) {
	if err := requirePlatform(); err != nil {
		return nil, err
	}
	return prepareWithResolver(ctx, name, func() error {
		_, err := activeBackend()
		return err
	}, func(logical string) (iface.Binding, error) {
		return iface.ResolveForBackend(Name, logical)
	})
}

func prepareWithResolver(ctx context.Context, name string, requireBackend func() error,
	resolve func(string) (iface.Binding, error)) (dataplane.Access, error) {
	if ctx == nil {
		return nil, fmt.Errorf("pppoe: netlink access requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("pppoe: netlink access requires a logical interface name")
	}
	if err := requireBackend(); err != nil {
		return nil, err
	}
	binding, err := resolve(name)
	if err != nil {
		return nil, err
	}
	mac, err := net.ParseMAC(binding.OperMAC)
	if err != nil {
		return nil, fmt.Errorf("pppoe: parse access %q MAC address: %w", name, err)
	}
	if len(mac) != 6 {
		return nil, fmt.Errorf("pppoe: access %q MAC address has %d bytes, want 6", name, len(mac))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info := dataplane.AccessInfo{Device: binding.OsName, ControlDevice: binding.OsName,
		DiscoveryIfIndex: binding.Ifindex, MTU: binding.MTU}
	copy(info.HWAddr[:], mac)
	return dataplane.NewAccess(info, kernelAccessOwner{})
}

// kernelAccessOwner ends only the read-only preparation lease. The configured
// Linux interface belongs to iface, and closing an access MUST NOT delete it.
type kernelAccessOwner struct{}

func (kernelAccessOwner) Close() error { return nil }
