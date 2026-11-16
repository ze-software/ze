// Design: docs/architecture/container/runtime.md -- the kernel tunable a container's network namespace needs
// Related: supervise.go -- tunableClaim, the claim made before every create
// Related: container.go -- the engine entry, which hands the supervisor this claim
//
// A kernel with tunnel drivers built in, as Ze's own kernel has, gives every
// new network namespace one fallback device of each tunnel kind (gre0,
// gretap0, erspan0, ip6gre0, ip6tnl0, sit0, tunl0) unless
// net.core.fb_tunnels_only_for_init_net says otherwise: 0 creates them in
// every namespace, 1 in the initial namespace only, 2 in none. The kernel
// reads the value when it creates a namespace, so the claim is in force
// before every create.

package container

import (
	"context"
	"encoding/json"

	"github.com/ze-software/ze/internal/component/sysctl"
	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The tunable and the value the plugin asks for. 1 keeps the fallback devices
// in the initial namespace, where the host's own configuration may use them,
// and out of every namespace created after it.
const (
	fallbackTunnelsKey   = "net.core.fb_tunnels_only_for_init_net"
	fallbackTunnelsValue = "1"
	// fallbackTunnelsNone is the other value that keeps a container's
	// namespace free of the devices: none is created anywhere.
	fallbackTunnelsNone = "2"
)

// fallbackTunnelsFix is the config statement that gives the plugin the
// tunable back when the configuration holds another value for it.
const fallbackTunnelsFix = "set sysctl setting " + fallbackTunnelsKey + " value " + fallbackTunnelsValue

// sysctlClaim claims the tunable through the sysctl plugin's default event,
// under the plugin's own name as the source, so the sysctl plugin writes it
// unless the configuration sets the key, and restores the kernel's value when
// the claim is cleared or the daemon stops. The emit reaches the sysctl
// plugin's handler before it returns, internal and forked alike, so the claim
// reads the kernel's value back to learn whether it is in force.
type sysctlClaim struct {
	plugin *sdk.Plugin
	// read answers the kernel's current value of a key.
	read func(key string) (string, error)
}

// newSysctlClaim answers the claim the engine entry hands the supervisor.
func newSysctlClaim(plugin *sdk.Plugin) sysctlClaim {
	return sysctlClaim{plugin: plugin, read: sysctl.Read}
}

// sysctlDefault is the payload of the sysctl plugin's default event.
type sysctlDefault struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

// sysctlSource is the payload of the sysctl plugin's clear event.
type sysctlSource struct {
	Source string `json:"source"`
}

func (c sysctlClaim) claim(ctx context.Context) error {
	payload, err := json.Marshal(sysctlDefault{Key: fallbackTunnelsKey, Value: fallbackTunnelsValue, Source: pluginName})
	if err != nil {
		return err
	}
	if _, err := c.plugin.EmitEvent(ctx, sysctlevents.Namespace, sysctlevents.EventDefault, "", "", string(payload)); err != nil {
		return opererr.Wrap(err,
			"the kernel setting "+fallbackTunnelsKey+" could not be asked of the sysctl plugin",
			"without it a container's network namespace holds the kernel's fallback tunnel devices, so no container starts",
			"ze show doctor",
		)
	}
	value, err := c.read(fallbackTunnelsKey)
	if err != nil {
		return opererr.Wrap(err,
			"the kernel setting "+fallbackTunnelsKey+" could not be read",
			"a container's network namespace holds the kernel's fallback tunnel devices unless it is 1 or 2, so no container starts without knowing it",
			"ze show doctor",
		)
	}
	switch value {
	case fallbackTunnelsValue, fallbackTunnelsNone:
		return nil
	}
	return opererr.New(
		"the kernel setting "+fallbackTunnelsKey+" is "+value+", so a container's network namespace would hold the kernel's fallback tunnel devices",
		"a container sees only the links Ze gives it; the configuration sets this key, and it outranks the value the container plugin asks for",
		fallbackTunnelsFix,
	)
}

func (c sysctlClaim) release(ctx context.Context) error {
	payload, err := json.Marshal(sysctlSource{Source: pluginName})
	if err != nil {
		return err
	}
	_, err = c.plugin.EmitEvent(ctx, sysctlevents.Namespace, sysctlevents.EventClearSourceDefaults, "", "", string(payload))
	return err
}
