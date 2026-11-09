// Design: docs/features/dns-resolver.md -- the dns-source list and the learned record
// Related: register.go -- handle offers the selection in the IPCP response; setEventBus follows the snapshots
// Related: pool.go -- ipv4Pool holds each pool's list

package l2tppool

import (
	"log/slog"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/core/dnssource"
)

// The YANG paths of the two subscriber lists, named in their refusals and in
// every address a selection drops.
const (
	poolDNSSourcePath      = "l2tp pool ipv4 dns-source"
	namedPoolDNSSourcePath = "l2tp pool named-pool dns-source"
)

// ipcpDNSSlots is how many resolvers IPCP carries: the Primary DNS option,
// then the Secondary (RFC 1877 Section 1.1 and 1.3).
const ipcpDNSSlots = 2

// Each pool's built-in subscriber list (D-5): what RADIUS names for the
// session, else the pool's dns-primary and dns-secondary, else the resolvers
// Ze learned. The servers are handed to a subscriber, so a loopback address
// is never one of them (D-16).
var (
	poolDNSDefault = dnssource.StatedDefault(poolDNSSourcePath, dnssource.FamilyIPv4, dnssource.AudienceHandedOn,
		dnssource.ModeFirstUsable, dnssource.ClassRadius, dnssource.ClassConfigured, dnssource.ClassReceived)
	namedPoolDNSDefault = dnssource.StatedDefault(namedPoolDNSSourcePath, dnssource.FamilyIPv4,
		dnssource.AudienceHandedOn, dnssource.ModeFirstUsable,
		dnssource.ClassRadius, dnssource.ClassConfigured, dnssource.ClassReceived)
)

// parsePoolSources reads a pool's dns-source container, or answers the
// built-in default when it is absent; builtin's path names the list. Every class is allowed: the pool is
// the one consumer with a subscriber, so radius has a meaning here, and self
// is the pool's gateway on the subscriber's link (D-6).
func parsePoolSources(block map[string]any, builtin dnssource.List) (dnssource.List, error) {
	return dnssource.ParseContainer(block, builtin, dnssource.FamilyIPv4, dnssource.AudienceHandedOn)
}

// poolDNSCandidates is what every class but radius offers pool's list at
// now: radius is per session, and selectSubscriberDNS adds it.
func poolDNSCandidates(pool *ipv4Pool, records []dnssource.Record, now time.Time) dnssource.Candidates {
	return dnssource.Candidates{
		dnssource.ClassConfigured: slotAddresses(pool.dnsPrimary, pool.dnsSecondary),
		dnssource.ClassSelf:       slotAddresses(pool.gateway, netip.Addr{}),
		dnssource.ClassReceived:   pool.sources.Received(records, now),
	}
}

// subscriberDNS is the pair of resolvers one IPCP response offers. The zero
// Addr in a slot means the response carries no such option.
type subscriberDNS struct {
	primary   netip.Addr
	secondary netip.Addr
}

// selectSubscriberDNS evaluates pool's list for one session. meta is the
// session's RADIUS metadata (nil when the session has none) and records the
// newest learned records. The selection fills the Primary slot, then the
// Secondary, in list order; what it drops, the host-only addresses and the
// servers past the second, is logged with the list's path.
func selectSubscriberDNS(pool *ipv4Pool, meta *l2tp.AuthMetadata, records []dnssource.Record, now time.Time,
	log *slog.Logger,
) subscriberDNS {
	list := pool.sources
	candidates := poolDNSCandidates(pool, records, now)
	if meta != nil {
		radius := slotAddresses(meta.DNSPrimary, meta.DNSSecondary)
		// R-6: a RADIUS server that names 0.0.0.0, a multicast address or the
		// limited broadcast names no resolver; Select skips it, and the
		// operator reads why the subscriber did not get it.
		for _, addr := range radius {
			if !dnssource.Usable(addr) {
				log.Warn("l2tp-pool: RADIUS MS DNS server is not a usable resolver and is not offered",
					"path", list.Path(), "address", addr)
			}
		}
		candidates[dnssource.ClassRadius] = radius
	}
	selection := dnssource.Select(list, candidates)
	for _, addr := range selection.Dropped {
		log.Warn("l2tp-pool: host-only resolver not offered to a subscriber; the next usable one is used",
			"path", list.Path(), "address", addr)
	}
	servers := selection.Servers
	if len(servers) > ipcpDNSSlots {
		log.Info("l2tp-pool: IPCP carries two resolvers; the later ones in the selection are not offered",
			"path", list.Path(), "dropped", servers[ipcpDNSSlots:])
		servers = servers[:ipcpDNSSlots]
	}
	var offered subscriberDNS
	if len(servers) > 0 {
		offered.primary = servers[0]
	}
	if len(servers) > 1 {
		offered.secondary = servers[1]
	}
	return offered
}

// slotAddresses answers the set slots of a primary and secondary pair, in
// slot order; an unset slot contributes nothing.
func slotAddresses(primary, secondary netip.Addr) []netip.Addr {
	var out []netip.Addr
	if primary.IsValid() {
		out = append(out, primary)
	}
	if secondary.IsValid() {
		out = append(out, secondary)
	}
	return out
}
