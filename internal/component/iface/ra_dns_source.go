// Design: docs/features/dns-resolver.md -- the dns-source list and the consumers that hand resolvers on
// Related: config_ra.go -- parses the rdnss container whose dns-source this file reads
// Related: reconcile_ra.go -- puts the selection into each unit's advertisement
// Related: register.go -- follows dnssource.SnapshotEvent and wakes the RDNSS worker

package iface

import (
	"log/slog"
	"net/netip"
	"slices"
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
	body, ok := rdnss["dns-source"].(map[string]any)
	if !ok {
		return raRDNSSDefault, nil
	}
	written, err := dnssource.ReadWritten(raRDNSSSourcePath, body)
	if err != nil {
		return dnssource.List{}, err
	}
	return dnssource.NewWrittenList(raRDNSSSourcePath, dnssource.FamilyIPv6, dnssource.AudienceHandedOn, written,
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
	// cut is how many selected servers did not fit the option.
	cut int
}

// raSelectRDNSS applies the unit's list to its candidates: configured is the
// rdnss server leaf-list, received the learned records at now, and self the
// unit's own global or unique-local address (D-6), absent when it holds none.
func raSelectRDNSS(cfg *raUnitConfig, self netip.Addr, records []dnssource.Record, now time.Time) raRDNSSSelection {
	list := cfg.Sources
	if list.IsZero() {
		list = raRDNSSDefault
	}
	candidates := dnssource.Candidates{
		dnssource.ClassConfigured: cfg.RDNSS,
		dnssource.ClassReceived:   list.Received(records, now),
	}
	if self.IsValid() {
		candidates[dnssource.ClassSelf] = []netip.Addr{self}
	}
	selection := dnssource.Select(list, candidates)
	out := raRDNSSSelection{servers: selection.Servers, dropped: selection.Dropped}
	if len(out.servers) > raRDNSSServersMax {
		out.cut = len(out.servers) - raRDNSSServersMax
		out.servers = out.servers[:raRDNSSServersMax]
	}
	if slices.Contains(selection.Used, dnssource.ClassReceived) {
		out.expires = receivedEnd(out.servers, candidates[dnssource.ClassConfigured], self, records, now)
	}
	return out
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
// dropped and each server that did not fit the option (D-16), once per
// change from the previous selection, so a steady state does not repeat the
// lines on every reconcile.
func logRDNSSSelection(log *slog.Logger, key raUnitKey, selection, previous raRDNSSSelection) {
	if slices.Equal(selection.dropped, previous.dropped) && selection.cut == previous.cut {
		return
	}
	for _, addr := range selection.dropped {
		log.Warn("interface: host-only resolver not advertised in RDNSS",
			"path", raRDNSSSourcePath, "iface", key.ifaceName, "unit", key.unit, "address", addr)
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
