// Design: docs/features/interfaces.md -- Router Advertisement sender lifecycle
// Related: config_ra.go -- parses the router-advertisement container this file reconciles
// Related: register.go -- calls reconcileRA beside reconcileDHCP, and stops senders on shutdown
// Related: config_apply.go -- forEachEnabledUnit, the units both reconciles serve

package iface

import (
	"log/slog"
	"slices"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/ndp"
)

// RASender is the part of the iface-ra plugin's sender the interface
// component uses. Declaring it here keeps the component free of any import of
// the plugin, so removing the plugin leaves the factory unset and reconcileRA a
// no-op.
type RASender interface {
	// Update replaces the advertisement a running sender sends, without
	// stopping it. A restart would send the final zero Router Lifetime of
	// Stop, withdrawing the router from every host on each lease renew.
	// Only the reconcile calls it, under raMu; the latest spec wins.
	Update(spec RASenderSpec)
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
	// WANs are the delegated-prefix sources the unit names. A unit with one
	// is in the CE role: its Router Lifetime follows the WAN default route
	// and its prefixes (RFC 7084 G-4, L-4). Empty keeps the configured value.
	WANs []string
	// Delegated is the /64 the unit holds from each live delegation. The
	// sender advertises each with the lifetimes that remain at every send.
	Delegated []DelegatedAdvert
	// RDNSSExpires is the end of validity of the learned records that
	// supplied Advertisement.RDNSS. The sender advertises the lesser of the
	// RDNSS lifetime and what remains to it at every send. The zero value is
	// a named state: no learned server carries an end, so the lifetime stands.
	RDNSSExpires time.Time
}

// Equal reports whether two specs describe the same advertisement, so reconcile
// updates a sender only when something reaching the wire changed.
// The generated == is unavailable because the advertisement holds slices.
func (s RASenderSpec) Equal(other RASenderSpec) bool {
	return s.Interface == other.Interface &&
		s.Unit == other.Unit &&
		s.MinimumInterval == other.MinimumInterval &&
		s.MaximumInterval == other.MaximumInterval &&
		slices.Equal(s.WANs, other.WANs) &&
		slices.EqualFunc(s.Delegated, other.Delegated, DelegatedAdvert.Equal) &&
		s.RDNSSExpires.Equal(other.RDNSSExpires) &&
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
		slices.Equal(a.RDNSSWithdrawn, b.RDNSSWithdrawn) &&
		slices.Equal(a.Prefixes, b.Prefixes) &&
		slices.Equal(a.Routes, b.Routes)
}

// raSenderFactory is set by the ifacera package at init time through
// SetRASenderFactory. It returns a started sender or an error. Leaving it nil
// is the supported state when the plugin is not built in.
var raSenderFactory func(spec RASenderSpec) (RASender, error)

// SetRASenderFactory registers the factory the interface component calls to
// start Router Advertisement senders. Called from the ifacera plugin's init.
func SetRASenderFactory(f func(RASenderSpec) (RASender, error)) {
	raSenderFactory = f
}

// raLearned is the Follower of the running engine, whose records fill each
// unit's RDNSS list. runEngine sets it through setRALearned, the way
// SetRASenderFactory sets raSenderFactory. Safe for concurrent use.
var raLearned atomic.Pointer[dnssource.Follower]

// setRALearned installs the engine's Follower. A respawned engine replaces
// the previous one's.
func setRALearned(f *dnssource.Follower) {
	raLearned.Store(f)
	raPublished.mu.Lock()
	raPublished.body = ""
	raPublished.mu.Unlock()
}

// raLearnedRecords answers the records the installed Follower holds.
func raLearnedRecords() []dnssource.Record {
	// refuse-by-default: guards reconcileRA -- with no engine installed (a unit test) no resolver is learned, which is the state before any lease
	f := raLearned.Load()
	if f == nil {
		return nil
	}
	return f.Records()
}

// raUnitKey identifies one advertising unit.
type raUnitKey struct {
	ifaceName string
	unit      string
}

// raEntry tracks a running sender and the spec it was started with, so
// reconcile can tell a changed advertisement from an unchanged one.
type raEntry struct {
	sender RASender
	spec   RASenderSpec
	// rdnss is the selection the spec's RDNSS came from, kept so a drop is
	// logged once per change rather than on every reconcile.
	rdnss raRDNSSSelection
}

// reconcileRA starts senders for units that newly advertise, stops senders for
// units that stopped advertising, and updates a running sender whose
// advertisement changed. Called from OnConfigure and OnConfigApply beside
// reconcileDHCP, by the delegation worker after a lease changes, and by the
// RDNSS worker after the learned resolvers change. The records of the newest
// system-dns snapshot (raLearned) fill each unit's RDNSS list, and every
// unit's selection is published (publishRASelections).
func reconcileRA(cfg *ifaceConfig, active map[raUnitKey]raEntry, log *slog.Logger) {
	if raSenderFactory == nil {
		return
	}

	now := time.Now()
	learned := raLearnedRecords()
	desired := make(map[raUnitKey]RASenderSpec)
	selections := make(map[raUnitKey]raRDNSSSelection)
	forEachEnabledUnit(cfg, func(ifaceName string, u *unitEntry) {
		if u.IPv6 == nil || u.IPv6.RouterAdvertisement == nil || !u.IPv6.RouterAdvertisement.Enabled {
			return
		}
		key := raUnitKey{ifaceName: ifaceName, unit: u.Label}
		spec := raSpecFor(ifaceName, u.Label, u.IPv6.RouterAdvertisement)
		for _, dp := range u.IPv6.DelegatedPrefixes {
			spec.WANs = append(spec.WANs, dp.Source)
		}
		spec.Delegated, _ = claimDelegatedAdverts(u, now) // the delegation worker reports refusals
		self := raSelfAddress(u, spec.Delegated)
		selection := raSelectRDNSS(u.IPv6.RouterAdvertisement, self, learned, now)
		spec.Advertisement.RDNSS = selection.servers
		spec.RDNSSExpires = selection.expires
		desired[key] = spec
		selections[key] = selection
	})
	publishRASelections(selections, log)

	for key := range active {
		entry := active[key]
		newSpec, stillDesired := desired[key]
		if !stillDesired {
			log.Info("interface: stopping router advertisement sender",
				"iface", key.ifaceName, "unit", key.unit)
			entry.sender.Stop()
			delete(active, key)
			continue
		}
		logRDNSSSelection(log, key, selections[key], entry.rdnss)
		entry.rdnss = selections[key]
		active[key] = entry
		if entry.spec.Equal(newSpec) {
			continue
		}
		log.Info("interface: updating router advertisement sender",
			"iface", key.ifaceName, "unit", key.unit)
		entry.sender.Update(newSpec)
		entry.spec = newSpec
		active[key] = entry
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
		logRDNSSSelection(log, key, selections[key], raRDNSSSelection{})
		active[key] = raEntry{sender: sender, spec: spec, rdnss: selections[key]}
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

// raSpecFor turns one unit's parsed configuration into the sender spec. The
// RDNSS it carries is the configured list; reconcileRA replaces it with the
// unit's dns-source selection.
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
