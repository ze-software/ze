// Design: docs/features/dns-resolver.md -- where Ze's resolver servers come from
// Related: select.go -- the selection rule that reads these records

package dnssource

import (
	"cmp"
	"net/netip"
	"slices"
	"strings"
	"time"
)

// Namespace is the EventBus namespace the resolver registry publishes on.
// A consumer registers its typed handles with these names; Register is
// idempotent for the same payload type, so producer and consumers each hold
// their own handle (internal/core/events).
const Namespace = "system-dns"

// SnapshotEventType carries a *Snapshot: every learned record and the
// router's selection, after every change.
const SnapshotEventType = "snapshot"

// SnapshotRequestEventType is a signal a consumer emits after it subscribes;
// the registry answers it by publishing the current snapshot again, so a
// consumer that starts late or restarts never waits for the next change.
const SnapshotRequestEventType = "snapshot-request"

// PublicationEventType carries a *Publication: one consumer's current
// selections, which the registry keeps for show system dns and ze doctor.
const PublicationEventType = "selection"

// Record is one learned set of resolver addresses: what one client of one
// registered origin learned on one interface and unit. It is a value: it
// holds no pointer into the producer's state, so it crosses a process
// boundary as it is.
type Record struct {
	// Origin is the registered origin's name (dhcpv4, dhcpv6, ppp, ...).
	Origin string `json:"origin"`
	// Rank is the origin's registered rank: a lower rank lists first.
	Rank uint16 `json:"rank"`
	// Interface and Unit name the lease owner; both are empty for a record
	// that belongs to no interface (the host's own resolv.conf).
	Interface string `json:"interface"`
	Unit      string `json:"unit"`
	// Addresses are the servers in the order the source sent them.
	Addresses []netip.Addr `json:"addresses"`
	// Received is when the producer last set the record.
	Received time.Time `json:"received"`
	// Expires is the absolute end of validity. Its zero value is a named
	// state, not a missing one: valid until the producer withdraws it.
	Expires time.Time `json:"expires,omitzero"`
	// Fallback marks a record of an origin registered as a fallback (the
	// host's own resolv.conf): the received class lists it only while no
	// other record would list an address. The origin declares it at
	// registration, so the rule names no origin.
	Fallback bool `json:"fallback,omitempty"`

	// The fields below are reserved for RFC 9463 (DNR) records. A record
	// with a non-empty Transport is not plain DNS on port 53, and a consumer
	// that speaks plain DNS skips it (ReceivedAddresses does).

	// Priority is the DNR Service Priority; 0 means not a DNR record.
	Priority uint16 `json:"priority,omitempty"`
	// Name is the DNR authentication domain name.
	Name string `json:"name,omitempty"`
	// Transport names the encrypted transports; empty means DNS over UDP
	// and TCP on port 53.
	Transport []string `json:"transport,omitempty"`
	// Port and Path are the DNR SvcParams port and dohpath.
	Port uint16 `json:"port,omitempty"`
	Path string `json:"path,omitempty"`
}

// ValidUntilWithdrawn reports whether the record carries no expiry.
func (r *Record) ValidUntilWithdrawn() bool { return r.Expires.IsZero() }

// Expired reports whether the record's validity ended at or before now.
func (r *Record) Expired(now time.Time) bool {
	if r.ValidUntilWithdrawn() {
		return false
	}
	return !now.Before(r.Expires)
}

// CompareRecords orders records the way the received class lists them: by
// origin rank, then interface, then unit.
func CompareRecords(a, b *Record) int {
	return cmp.Or(
		cmp.Compare(a.Rank, b.Rank),
		strings.Compare(a.Interface, b.Interface),
		strings.Compare(a.Unit, b.Unit),
	)
}

// ReceivedAddresses answers the received class's entries over records at
// now: each unexpired plain-DNS record, ordered by CompareRecords, each
// record's addresses in the order its source sent them. A fallback record
// counts only when no other record lists an address (liveAddresses). Repeats
// are kept; Select removes them. records is not modified.
func ReceivedAddresses(records []Record, now time.Time) []netip.Addr {
	ordered := slices.Clone(records)
	slices.SortStableFunc(ordered, func(a, b Record) int { return CompareRecords(&a, &b) })
	return liveAddresses(ordered, now)
}

// liveAddresses answers the addresses of each unexpired plain-DNS record, in
// the order records holds them. A record with a Transport is skipped: it is
// not plain DNS on port 53. A Fallback record is skipped while any other
// record lists an address, so the host's resolv.conf stands in only for a
// lease Ze does not hold, and returns when the last lease expires.
func liveAddresses(records []Record, now time.Time) []netip.Addr {
	leased := false
	for i := range records {
		if !records[i].Fallback && records[i].lists(now) {
			leased = true
			break
		}
	}
	var out []netip.Addr
	for i := range records {
		record := &records[i]
		if !record.lists(now) {
			continue
		}
		if record.Fallback && leased {
			continue
		}
		out = append(out, record.Addresses...)
	}
	return out
}

// lists reports whether the record offers a plain-DNS address at now: it is
// unexpired, carries no Transport, and holds at least one address.
func (r *Record) lists(now time.Time) bool {
	if r.Expired(now) {
		return false
	}
	if len(r.Transport) > 0 {
		return false
	}
	return len(r.Addresses) > 0
}

// ConsumerSelection is one consumer's answer as it is published: the list it
// evaluated and what that list chose. The router's lives in Snapshot; every
// other consumer publishes its own in a Publication.
type ConsumerSelection struct {
	// Path is the config path of the consumer's dns-source container.
	Path string `json:"path"`
	// Instance names the list among the consumer's lists at Path: the
	// subnet, the interface unit or the pool. Empty for a consumer with one
	// list (the router, the LAN resolver).
	Instance string `json:"instance,omitempty"`
	// Sources and Mode are the list evaluated, Mode in its operator word.
	Sources []Class `json:"sources"`
	Mode    string  `json:"mode"`
	// Default is true while the built-in default list applies.
	Default bool `json:"default"`
	// ReceivedInterfaces narrows received, in order; empty: every interface.
	ReceivedInterfaces []string `json:"received-interfaces,omitempty"`
	// Used are the classes that contributed a server.
	Used []Class `json:"used"`
	// Servers are the consumer's resolvers, in the order it hands them on.
	Servers []netip.Addr `json:"servers"`
	// Self are the Servers entries the self class contributed: Ze's own
	// addresses, reachable only while some consumer Serves them.
	Self []netip.Addr `json:"self,omitempty"`
	// Serves are the addresses the consumer itself answers DNS queries on
	// (the LAN resolver's listeners); empty for a consumer that answers none.
	// A self entry another consumer hands on is reachable only when some
	// consumer serves it.
	Serves []netip.Addr `json:"serves,omitempty"`
}

// NewConsumerSelection pairs list with what it selected, for the list named
// instance at list's path.
func NewConsumerSelection(list List, instance string, selection Selection) ConsumerSelection {
	return ConsumerSelection{
		Path:               list.Path(),
		Instance:           instance,
		Sources:            list.Sources(),
		Mode:               list.Mode().String(),
		Default:            list.Stated(),
		ReceivedInterfaces: list.ReceivedInterfaces(),
		Used:               slices.Clone(selection.Used),
		Servers:            slices.Clone(selection.Servers),
		Self:               slices.Clone(selection.Self),
	}
}

// Publication is every current selection of one consumer. A consumer MUST
// publish its whole set each time any of its selections changes, and once
// after it starts, because the registry replaces the consumer's previous set
// with each publication: a list the operator removed leaves by being absent.
// Consumer names the publisher (its plugin name); the registry keys by it and
// names no consumer itself.
type Publication struct {
	Consumer   string              `json:"consumer"`
	Selections []ConsumerSelection `json:"selections"`
}

// Snapshot is the registry's published state: every learned record, in
// received-class order, and the router's selection. Each snapshot is a fresh
// value; a subscriber may keep it and MUST NOT modify it, because in process
// every subscriber receives the same pointer.
//
// Sequence grows by one on every change. A snapshot can be published twice
// (once for the change, once again to answer a snapshot request), and the
// two can reach a subscriber out of order, so a subscriber MUST drop a
// snapshot whose Sequence is below the last one it applied.
type Snapshot struct {
	Sequence uint64            `json:"sequence"`
	Records  []Record          `json:"records"`
	Router   ConsumerSelection `json:"router"`
}
