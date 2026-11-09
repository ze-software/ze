// Design: docs/features/dns-resolver.md -- the dns-source list and the consumers that hand resolvers on
// Related: config_ra.go -- parses the rdnss container whose dns-source this file reads
// Related: reconcile_ra.go -- puts the selection into each unit's advertisement
// Related: register.go -- follows dnssource.SnapshotEvent and wakes the RDNSS worker

package iface

import (
	"encoding/json"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// raRDNSSSourcePath is the YANG path of a unit's RDNSS list, named in its
// refusals and in every address the selection drops.
const raRDNSSSourcePath = "interface unit ipv6 router-advertisement rdnss dns-source"

// raRDNSSServersMax is how many addresses Ze puts in the RDNSS option: the
// max-elements of rdnss server, applied to every class so a learned list
// cannot grow the option past what a configured one can carry.
const raRDNSSServersMax = 8

// raRDNSSDefault is a unit's built-in RDNSS list (D-5): the rdnss server
// entries when any are set, else the IPv6 resolvers Ze learned. The servers
// are handed to hosts on the link, so a loopback or zone-less link-local
// address is never one of them (D-16).
var raRDNSSDefault = dnssource.StatedDefault(raRDNSSSourcePath, dnssource.FamilyIPv6, dnssource.AudienceHandedOn,
	dnssource.ModeFirstUsable, dnssource.ClassConfigured, dnssource.ClassReceived)

// raRDNSSRefusals are the classes a unit's RDNSS list cannot name.
var raRDNSSRefusals = []dnssource.Refusal{
	{
		Class: dnssource.ClassRadius,
		Why:   "radius names the resolvers a RADIUS server gives one PPP subscriber, and a host on this link is not a subscriber",
		Fix: "delete " + raRDNSSSourcePath + " source radius; the unit takes configured (rdnss server), " +
			"received and self",
	},
}

// raParseRDNSSSources reads the dns-source container of an rdnss container,
// or answers the built-in default when it is absent.
func raParseRDNSSSources(rdnss map[string]any) (dnssource.List, error) {
	return dnssource.ParseContainer(rdnss, raRDNSSDefault, dnssource.FamilyIPv6, dnssource.AudienceHandedOn,
		raRDNSSRefusals...)
}

// raRDNSSSelection is what one unit's RDNSS option carries at one reconcile.
type raRDNSSSelection struct {
	// servers are the addresses, at most raRDNSSServersMax.
	servers []netip.Addr
	// expires bounds the advertised lifetime when a received record supplied
	// a server: the earliest end of validity among them. The zero value is
	// a named state: no received server carries an end, so the configured
	// lifetime stands.
	expires time.Time
	// dropped are the host-only addresses the list refused, for the log.
	dropped []netip.Addr
	// offLink are the link-local addresses whose zone is another link than
	// the unit's, refused before the list ran (raOnLink), for the log.
	offLink []netip.Addr
	// cut is how many selected servers did not fit the option.
	cut int
	// chosen is the selection as it is published (publishRASelections), its
	// servers the ones the option carries; its Instance is set by the caller.
	chosen dnssource.ConsumerSelection
}

// raSelectRDNSS applies the unit's list to its candidates: configured is the
// rdnss server leaf-list, received the learned records at now, and self the
// unit's own global or unique-local address (D-6), absent when it holds none.
// device is the kernel device the unit advertises on (raUnitDevice), "" when
// it is bound to none.
func raSelectRDNSS(cfg *raUnitConfig, device string, self netip.Addr, records []dnssource.Record, now time.Time) raRDNSSSelection {
	list := cfg.Sources
	if list.IsZero() {
		list = raRDNSSDefault
	}
	var offLink []netip.Addr
	candidates := dnssource.Candidates{
		dnssource.ClassConfigured: raOnLink(cfg.RDNSS, device, &offLink),
		dnssource.ClassReceived:   raOnLink(list.Received(records, now), device, &offLink),
	}
	if self.IsValid() {
		candidates[dnssource.ClassSelf] = []netip.Addr{self}
	}
	selection := dnssource.Select(list, candidates)
	out := raRDNSSSelection{servers: selection.Servers, dropped: selection.Dropped, offLink: offLink}
	if len(out.servers) > raRDNSSServersMax {
		out.cut = len(out.servers) - raRDNSSServersMax
		out.servers = out.servers[:raRDNSSServersMax]
	}
	out.chosen = dnssource.NewConsumerSelection(list, "", selection)
	out.chosen.Servers = slices.Clone(out.servers)
	out.chosen.Self = slices.DeleteFunc(out.chosen.Self, func(addr netip.Addr) bool { return !slices.Contains(out.servers, addr) })
	if slices.Contains(selection.Used, dnssource.ClassReceived) {
		out.expires = receivedEnd(out.servers, candidates[dnssource.ClassConfigured], self, records, now)
	}
	return out
}

// raOnLink answers addrs without the link-local addresses that belong to
// another link than device, and appends each one it removes to offLink.
//
// RFC 8106 Section 5.1: "When a resolver sends a DNS query message to an
// RDNSS identified by a link-local address, it MUST use the corresponding
// link." A link-local resolver learned from an RA carries the device the RA
// arrived on as its zone (raRDNSS.handle). The RDNSS option has no room for a
// zone, so a host on this unit's link reads the address as one on its own
// link, where it is not the resolver. A zoned link-local is therefore handed
// on only by the unit on its own device. A zone-less one is left to the
// list's host-only check (D-16), which drops it.
func raOnLink(addrs []netip.Addr, device string, offLink *[]netip.Addr) []netip.Addr {
	var kept []netip.Addr
	for _, addr := range addrs {
		if raOffLink(addr, device) {
			*offLink = append(*offLink, addr)
			continue
		}
		kept = append(kept, addr)
	}
	return kept
}

// raOffLink reports whether addr is a link-local address zoned to another
// link than device. With no device every zoned link-local is off the unit's
// link, because the unit has none to share with it.
func raOffLink(addr netip.Addr, device string) bool {
	if !addr.IsLinkLocalUnicast() {
		return false
	}
	if addr.Zone() == "" {
		return false
	}
	return addr.Zone() != device
}

// raUnitDevice answers the kernel device a unit advertises on, through the
// shared resolver, or "" when its interface is bound to no device now.
func raUnitDevice(ifaceName string, u *unitEntry) string {
	parent, err := resolveDeviceName(ifaceName)
	// refuse-by-default: guards raOffLink -- a unit bound to no device has no link, so every zoned link-local is off it
	if err != nil {
		return ""
	}
	return unitOSName(parent, u)
}

// receivedEnd answers the earliest end of validity among the servers that
// came from a received record: a server stays valid while any unexpired
// record lists it, so each server's end is the latest of those records'.
// A server listed by a record valid until withdrawn has no end, and a server
// the unit's own config or self also supplies is not bounded by a lease.
// The zero time means no server carries an end.
func receivedEnd(servers, configured []netip.Addr, self netip.Addr, records []dnssource.Record, now time.Time) time.Time {
	var earliest time.Time
	for _, server := range servers {
		if server == self {
			continue
		}
		if slices.Contains(configured, server) {
			continue
		}
		end, bounded := serverEnd(server, records, now)
		if !bounded {
			continue
		}
		if earliest.IsZero() || end.Before(earliest) {
			earliest = end
		}
	}
	return earliest
}

// serverEnd answers when the last unexpired record listing server ends, and
// false when one of them never ends or none lists it.
func serverEnd(server netip.Addr, records []dnssource.Record, now time.Time) (end time.Time, bounded bool) {
	for i := range records {
		record := &records[i]
		if record.Expired(now) {
			continue
		}
		if !slices.Contains(record.Addresses, server) {
			continue
		}
		if record.ValidUntilWithdrawn() {
			return time.Time{}, false
		}
		if record.Expires.After(end) {
			end = record.Expires
		}
	}
	return end, !end.IsZero()
}

// raSelfAddress is the address self offers on a unit: its first configured
// IPv6 address that a host on the link can reach as a resolver (global or
// unique-local, so neither link-local nor loopback), else the address it took
// from a live delegation. The zero Addr means the unit holds none, and a list
// naming self falls through.
func raSelfAddress(u *unitEntry, delegated []DelegatedAdvert) netip.Addr {
	for _, text := range u.Addresses {
		if addr := raUsableSelf(text); addr.IsValid() {
			return addr
		}
	}
	if u.IPv6 != nil {
		for _, text := range u.IPv6.Addresses {
			if addr := raUsableSelf(text); addr.IsValid() {
				return addr
			}
		}
	}
	for _, d := range delegated {
		if addr := d.Address.Addr(); addr.IsGlobalUnicast() {
			return addr
		}
	}
	return netip.Addr{}
}

// raUsableSelf parses one configured address and answers it when it is an
// IPv6 global unicast or unique-local address; the zero Addr otherwise.
// netip counts unique-local (fc00::/7) as global unicast.
func raUsableSelf(text string) netip.Addr {
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		return netip.Addr{}
	}
	addr := prefix.Addr()
	if !addr.Is6() {
		return netip.Addr{}
	}
	if addr.Is4In6() {
		return netip.Addr{}
	}
	if !addr.IsGlobalUnicast() {
		return netip.Addr{}
	}
	return addr
}

// logRDNSSSelection logs, at WARN, each host-only address the selection
// dropped, each link-local address of another link it refused, and each server that did not fit the option (D-16), once per
// change from the previous selection, so a steady state does not repeat the
// lines on every reconcile.
func logRDNSSSelection(log *slog.Logger, key raUnitKey, selection, previous raRDNSSSelection) {
	if slices.Equal(selection.dropped, previous.dropped) && selection.cut == previous.cut &&
		slices.Equal(selection.offLink, previous.offLink) {
		return
	}
	for _, addr := range selection.dropped {
		log.Warn("interface: host-only resolver not advertised in RDNSS",
			"path", raRDNSSSourcePath, "iface", key.ifaceName, "unit", key.unit, "address", addr)
	}
	for _, addr := range selection.offLink {
		log.Warn("interface: link-local resolver of another link not advertised in RDNSS; "+
			"a host on this link cannot reach it, so name a global resolver in rdnss server",
			"path", raRDNSSSourcePath, "iface", key.ifaceName, "unit", key.unit,
			"address", addr.WithZone(""), "link", addr.Zone())
	}
	if selection.cut > 0 {
		log.Warn("interface: resolvers beyond what the RDNSS option carries not advertised",
			"path", raRDNSSSourcePath, "iface", key.ifaceName, "unit", key.unit,
			"advertised", raRDNSSServersMax, "left-out", selection.cut)
	}
}

// runRDNSSWorker re-runs the RA reconcile each time a system-dns snapshot
// changed the learned records, outside the EventBus Emit goroutine, whose
// handlers MUST NOT block. ch coalesces: one pending wake covers any number
// of snapshots. The owner closes stop to end it and MUST wait on done before
// it stops the RA senders, because a pass may start one.
func runRDNSSWorker(ch, stop <-chan struct{}, done chan<- struct{}, refresh func()) {
	defer close(done)
	for {
		select {
		case <-ch:
			refresh()
		case <-stop:
			return
		}
	}
}

// raConsumer is the name the interface component publishes its units' RDNSS
// selections under.
const raConsumer = "interface"

// raPublished is the body of the last RDNSS publication sent, so a reconcile
// that changed no selection sends nothing. setRALearned clears it, because a
// respawned engine MUST publish its whole set again. Safe for concurrent use.
var raPublished struct {
	mu   sync.Mutex
	body string
}

// raInstance names one unit's list among the component's lists.
func raInstance(key raUnitKey) string {
	return key.ifaceName + " unit " + key.unit
}

// publishRASelections publishes the RDNSS selection of every advertising unit
// for show system dns and ze doctor, as the whole set: a unit that stopped
// advertising leaves by being absent. It sends only a set that differs from
// the last one sent, and nothing while no event bus is installed (a unit
// test of the reconcile alone).
func publishRASelections(selections map[raUnitKey]raRDNSSSelection, log *slog.Logger) {
	eb := GetEventBus()
	// refuse-by-default: guards reconcileRA -- with no bus installed nobody can read a publication, and the next reconcile after the bus is set publishes the whole set
	if eb == nil {
		return
	}
	keys := make([]raUnitKey, 0, len(selections))
	for key := range selections {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b raUnitKey) int {
		if c := strings.Compare(a.ifaceName, b.ifaceName); c != 0 {
			return c
		}
		return strings.Compare(a.unit, b.unit)
	})
	publication := &dnssource.Publication{Consumer: raConsumer, Selections: make([]dnssource.ConsumerSelection, 0, len(keys))}
	for _, key := range keys {
		current := selections[key].chosen
		current.Instance = raInstance(key)
		publication.Selections = append(publication.Selections, current)
	}
	body, err := json.Marshal(publication)
	if err != nil {
		log.Warn("interface: RDNSS selections not published; show system dns shows the previous ones", "path", raRDNSSSourcePath, "error", err)
		return
	}
	raPublished.mu.Lock()
	defer raPublished.mu.Unlock()
	if string(body) == raPublished.body {
		return
	}
	if _, err := dnssource.PublicationEvent.Emit(eb, publication); err != nil {
		log.Warn("interface: RDNSS selections not published; show system dns shows the previous ones", "path", raRDNSSSourcePath, "error", err)
		return
	}
	raPublished.body = string(body)
}
