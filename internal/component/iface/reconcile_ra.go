// Design: docs/features/interfaces.md -- Router Advertisement sender lifecycle
// Related: config_ra.go -- parses the router-advertisement container this file reconciles
// Related: register.go -- calls reconcileRA beside reconcileDHCP, and stops senders on shutdown

package iface

import (
	"log/slog"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/ndp"
)

// RAStopper is the part of the iface-ra plugin's sender the interface
// component uses. Declaring it here keeps the component free of any import of
// the plugin, so removing the plugin leaves the factory unset and reconcileRA a
// no-op.
type RAStopper interface {
	// Stop ends the sender. RFC 4861 Section 6.2.5 asks a router that stops
	// advertising to send a final advertisement with a Router Lifetime of
	// zero, so the implementation sends that before it closes its socket.
	Stop()
}

// RASenderSpec is everything the iface-ra plugin needs to run one sender. The
// component resolves nothing here: the plugin turns Interface into an OS
// device through iface.Resolve, and fills the Source Link-layer Address option
// from the resolved binding.
type RASenderSpec struct {
	// Interface is the logical interface name, for iface.Resolve.
	Interface string
	// Unit is the unit label the advertisement was configured on.
	Unit string
	// Advertisement is the message to send, with every field the operator
	// configured. SourceLinkLayerAddress is left empty: only the plugin knows
	// the resolved device's address.
	Advertisement ndp.RAConfig
	// MinimumInterval and MaximumInterval bound the random wait between
	// unsolicited advertisements (RFC 4861 Section 6.2.4).
	MinimumInterval time.Duration
	MaximumInterval time.Duration
}

// Equal reports whether two specs describe the same advertisement, so reconcile
// restarts a sender only when something an operator changed reaches the wire.
// The generated == is unavailable because the advertisement holds slices.
func (s RASenderSpec) Equal(other RASenderSpec) bool {
	return s.Interface == other.Interface &&
		s.Unit == other.Unit &&
		s.MinimumInterval == other.MinimumInterval &&
		s.MaximumInterval == other.MaximumInterval &&
		raAdvertisementEqual(s.Advertisement, other.Advertisement)
}

// raAdvertisementEqual compares two encoder inputs field by field.
func raAdvertisementEqual(a, b ndp.RAConfig) bool {
	return a.CurHopLimit == b.CurHopLimit &&
		a.Managed == b.Managed &&
		a.OtherConfig == b.OtherConfig &&
		a.RouterLifetime == b.RouterLifetime &&
		a.ReachableTime == b.ReachableTime &&
		a.RetransTimer == b.RetransTimer &&
		a.RDNSSLifetime == b.RDNSSLifetime &&
		slices.Equal(a.SourceLinkLayerAddress, b.SourceLinkLayerAddress) &&
		slices.Equal(a.RDNSS, b.RDNSS) &&
		slices.Equal(a.Prefixes, b.Prefixes)
}

// raSenderFactory is set by the ifacera package at init time through
// SetRASenderFactory. It returns a started sender or an error. Leaving it nil
// is the supported state when the plugin is not built in.
var raSenderFactory func(spec RASenderSpec) (RAStopper, error)

// SetRASenderFactory registers the factory the interface component calls to
// start Router Advertisement senders. Called from the ifacera plugin's init.
func SetRASenderFactory(f func(RASenderSpec) (RAStopper, error)) {
	raSenderFactory = f
}

// raUnitKey identifies one advertising unit.
type raUnitKey struct {
	ifaceName string
	unit      string
}

// raEntry tracks a running sender and the spec it was started with, so
// reconcile can tell a changed advertisement from an unchanged one.
type raEntry struct {
	sender RAStopper
	spec   RASenderSpec
}

// reconcileRA starts senders for units that newly advertise, stops senders for
// units that stopped advertising, and restarts a sender whose advertisement
// changed. Called from OnConfigure and OnConfigApply beside reconcileDHCP.
func reconcileRA(cfg *ifaceConfig, active map[raUnitKey]raEntry, log *slog.Logger) {
	if raSenderFactory == nil {
		return
	}

	desired := make(map[raUnitKey]RASenderSpec)
	forEachConfiguredUnit(cfg, func(ifaceName string, u *unitEntry) {
		if u.IPv6 == nil || u.IPv6.RouterAdvertisement == nil || !u.IPv6.RouterAdvertisement.Enabled {
			return
		}
		key := raUnitKey{ifaceName: ifaceName, unit: u.Label}
		desired[key] = raSpecFor(ifaceName, u.Label, u.IPv6.RouterAdvertisement)
	})

	for key := range active {
		entry := active[key]
		newSpec, stillDesired := desired[key]
		if stillDesired && entry.spec.Equal(newSpec) {
			continue
		}
		if stillDesired {
			log.Info("interface: restarting router advertisement sender (config changed)",
				"iface", key.ifaceName, "unit", key.unit)
		} else {
			log.Info("interface: stopping router advertisement sender",
				"iface", key.ifaceName, "unit", key.unit)
		}
		entry.sender.Stop()
		delete(active, key)
	}

	for key, spec := range desired {
		if _, running := active[key]; running {
			continue
		}
		sender, err := raSenderFactory(spec)
		if err != nil {
			log.Warn("interface: router advertisement sender failed to start",
				"iface", key.ifaceName, "unit", key.unit, "error", err)
			continue
		}
		active[key] = raEntry{sender: sender, spec: spec}
		log.Info("interface: router advertisement sender started",
			"iface", key.ifaceName, "unit", key.unit,
			"prefixes", len(spec.Advertisement.Prefixes))
	}
}

// stopAllRASenders stops every running sender and empties the map. Called on
// component shutdown, where RFC 4861 Section 6.2.5 asks each sender for a final
// advertisement with a Router Lifetime of zero.
func stopAllRASenders(active map[raUnitKey]raEntry, log *slog.Logger) {
	for key := range active {
		log.Debug("interface: stopping router advertisement sender on shutdown",
			"iface", key.ifaceName, "unit", key.unit)
		active[key].sender.Stop()
		delete(active, key)
	}
}

// raSpecFor turns one unit's parsed configuration into the sender spec.
func raSpecFor(ifaceName, unit string, cfg *raUnitConfig) RASenderSpec {
	spec := RASenderSpec{
		Interface:       ifaceName,
		Unit:            unit,
		MinimumInterval: time.Duration(cfg.MinimumInterval) * time.Second,
		MaximumInterval: time.Duration(cfg.MaximumInterval) * time.Second,
		Advertisement: ndp.RAConfig{
			CurHopLimit:    cfg.HopLimit,
			Managed:        cfg.Managed,
			OtherConfig:    cfg.OtherConfig,
			RouterLifetime: cfg.RouterLifetime,
			ReachableTime:  cfg.ReachableTime,
			RetransTimer:   cfg.RetransmitTimer,
			RDNSS:          cfg.RDNSS,
			RDNSSLifetime:  cfg.EffectiveRDNSSLifetime(),
		},
	}
	for _, p := range cfg.Prefixes {
		spec.Advertisement.Prefixes = append(spec.Advertisement.Prefixes, ndp.PrefixInfo{
			Prefix:            p.Prefix,
			OnLink:            p.OnLink,
			Autonomous:        p.Autonomous,
			ValidLifetime:     p.ValidLifetime,
			PreferredLifetime: p.PreferredLifetime,
		})
	}
	return spec
}

// forEachConfiguredUnit calls fn for every unit of every interface kind that
// carries units, with the interface's logical name. One list keeps a new
// interface kind from reaching some per-unit services and not others.
func forEachConfiguredUnit(cfg *ifaceConfig, fn func(ifaceName string, u *unitEntry)) {
	visit := func(name string, units []unitEntry) {
		for i := range units {
			fn(name, &units[i])
		}
	}
	for i := range cfg.Ethernet {
		visit(cfg.Ethernet[i].Name, cfg.Ethernet[i].Units)
	}
	for i := range cfg.Dummy {
		visit(cfg.Dummy[i].Name, cfg.Dummy[i].Units)
	}
	for i := range cfg.Veth {
		visit(cfg.Veth[i].Name, cfg.Veth[i].Units)
	}
	for i := range cfg.Bridge {
		visit(cfg.Bridge[i].Name, cfg.Bridge[i].Units)
	}
	for i := range cfg.Tunnel {
		visit(cfg.Tunnel[i].Name, cfg.Tunnel[i].Units)
	}
	for i := range cfg.Wireguard {
		visit(cfg.Wireguard[i].Name, cfg.Wireguard[i].Units)
	}
	for i := range cfg.XFRM {
		visit(cfg.XFRM[i].Name, cfg.XFRM[i].Units)
	}
	if cfg.Loopback != nil {
		visit("lo", cfg.Loopback.Units)
	}
}
