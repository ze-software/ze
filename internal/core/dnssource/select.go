// Design: docs/features/dns-resolver.md -- where Ze's resolver servers come from
// Related: record.go -- the learned resolver record and the received class's order

// Package dnssource holds the rule every consumer of resolver addresses uses
// to choose them: the router (resolv.conf and Ze's own lookups), and every
// path that hands resolvers to an end user. A consumer names an ordered list
// of source classes and a mode; Select walks the list over the addresses the
// consumer gathered for each class and answers the servers it hands on.
//
// The package is a core leaf: it imports the standard library only, holds no
// state, and knows no consumer. Each consumer gathers its own classes' entries
// (its configured servers, the learned records, its own address) and states its
// own list, so a new consumer never edits this package.
package dnssource

import (
	"errors"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Class names the role of a source of resolver addresses in a consumer's
// list. The words are the `dns-source-class` vocabulary an operator writes.
// A class names a role, never a protocol: a learned protocol registers its
// own origin and its records appear inside ClassReceived.
type Class string

const (
	// ClassReceived is every unexpired learned record, ordered by
	// ReceivedAddresses.
	ClassReceived Class = "received"
	// ClassConfigured is the consumer's own static list (for the router,
	// system name-server).
	ClassConfigured Class = "configured"
	// ClassSelf is Ze's own address on the interface the client is on.
	ClassSelf Class = "self"
	// ClassRadius is a subscriber's RADIUS-supplied resolvers.
	ClassRadius Class = "radius"
)

// Mode is how a list with more than one source combines them. The zero value
// is ModeUnspecified, which a list of two or more sources may not carry: Ze
// will not choose between a backup and an addition for the operator.
type Mode uint8

const (
	// ModeUnspecified means no mode was written. Only a one-source list may
	// hold it, because both modes give that list the same answer.
	ModeUnspecified Mode = iota
	// ModeFirstUsable: the first source with at least one usable entry wins,
	// and its entries are the answer. A later source is a backup.
	ModeFirstUsable
	// ModeMerge: every source's usable entries, in list order, a server
	// already listed not repeated. A later source is an addition.
	ModeMerge
)

// ParseClass reads the word an operator writes for a class. A word that
// names no class answers false, and the caller refuses it rather than guess.
func ParseClass(word string) (Class, bool) {
	if slices.Contains(classes, Class(word)) {
		return Class(word), true
	}
	return "", false
}

// classes is every class, in the order the YANG enumeration lists them.
var classes = []Class{ClassReceived, ClassConfigured, ClassSelf, ClassRadius}

// Classes answers every class an operator can write, for a message that
// lists them. The slice is a copy.
func Classes() []Class { return slices.Clone(classes) }

// String answers the word an operator writes for the mode. It is for a human
// and a JSON field; nothing compares against it.
func (m Mode) String() string {
	switch m {
	case ModeFirstUsable:
		return "first-usable"
	case ModeMerge:
		return "merge"
	case ModeUnspecified:
		return "unspecified"
	}
	return "invalid"
}

// ParseMode reads the word an operator writes for a mode. A word that names
// no mode answers false; an absent leaf is ModeUnspecified, which the caller
// states itself rather than passing an empty word.
func ParseMode(word string) (Mode, bool) {
	switch word {
	case "first-usable":
		return ModeFirstUsable, true
	case "merge":
		return ModeMerge, true
	}
	return ModeUnspecified, false
}

// Family is the set of address families a consumer hands on: the router
// takes both, DHCPv4 option 6 takes IPv4 alone. The zero value names no
// family and NewList refuses it, so a List never selects nothing by accident.
type Family uint8

const (
	// FamilyIPv4 admits IPv4 addresses.
	FamilyIPv4 Family = 1 << iota
	// FamilyIPv6 admits IPv6 addresses.
	FamilyIPv6
	// FamilyAll admits both.
	FamilyAll = FamilyIPv4 | FamilyIPv6
)

// Audience says who receives a list's servers, which decides whether an
// address only Ze itself can reach is usable. The zero value names no
// audience and NewList refuses it, so every consumer states its own.
type Audience uint8

const (
	// AudienceUnspecified is the zero value; NewList refuses it.
	AudienceUnspecified Audience = iota
	// AudienceZe is a list whose servers Ze itself asks: the router's and the
	// LAN resolver's upstreams. A loopback or link-local server is reachable.
	AudienceZe
	// AudienceHandedOn is a list whose servers Ze hands to another host: DHCPv4
	// option 6, RA RDNSS, a subscriber's IPCP. Select drops a loopback address
	// and an IPv6 link-local address with no zone from it, because neither
	// names a server the receiving host can reach.
	AudienceHandedOn
)

// admits reports whether addr is of a family in f. An IPv4-mapped IPv6
// address counts as IPv4, the family a client reaching it uses.
func (f Family) admits(addr netip.Addr) bool {
	if addr.Unmap().Is4() {
		return f&FamilyIPv4 != 0
	}
	return f&FamilyIPv6 != 0
}

// List is one consumer's validated source list: the classes in the order the
// consumer falls through them, the mode, and the families it hands on. Only
// NewList and StatedDefault build one, so Select never meets a list of two
// sources with no mode. A List is immutable and safe for concurrent use.
type List struct {
	path    string
	sources []Class
	mode    Mode
	family  Family
	// audience decides whether a host-only address is usable (Audience).
	audience Audience
	stated   bool // a built-in default, not an operator-written list
	// interfaces narrows the received class to the records learned on these
	// interfaces, in this order (received-interface). Empty: every interface.
	interfaces []string
}

// errNoFamily refuses a list that admits no address family: it could select
// nothing, whatever the sources held. Only a Ze defect builds one.
var errNoFamily = errors.New("BUG: a dns-source list admits no address family")

// errNoAudience refuses a list that does not say who receives its servers.
// Only a Ze defect builds one.
var errNoAudience = errors.New("BUG: a dns-source list states no audience")

// NewList validates the list written at path (the config path of the
// consumer's dns-source container, as an operator types it in a set command)
// and answers it. Two or more sources with ModeUnspecified are refused with
// a *ModeRequiredError. A list naming no source, or one class twice, is
// refused too. audience states who receives the servers (Audience).
func NewList(path string, family Family, audience Audience, mode Mode, sources ...Class) (List, error) {
	if family&FamilyAll == 0 {
		return List{}, errNoFamily
	}
	if audience == AudienceUnspecified {
		return List{}, errNoAudience
	}
	if len(sources) == 0 {
		return List{}, &EmptyListError{Path: path}
	}
	for i, source := range sources {
		if slices.Contains(sources[:i], source) {
			return List{}, &RepeatedSourceError{Path: path, Source: source}
		}
	}
	if len(sources) > 1 && mode == ModeUnspecified {
		return List{}, &ModeRequiredError{Path: path, Sources: slices.Clone(sources)}
	}
	return List{path: path, sources: slices.Clone(sources), mode: mode, family: family, audience: audience}, nil
}

// StatedDefault answers a consumer's built-in default list: the list that
// applies while the operator wrote no dns-source container. A default always
// states its mode, even for one source. The consumer MUST call it with a list
// NewList accepts and a mode; anything else is a Ze defect in that consumer's
// stated default, found at package initialization, and it panics.
func StatedDefault(path string, family Family, audience Audience, mode Mode, sources ...Class) List {
	if mode == ModeUnspecified {
		panic("BUG: built-in dns-source default at " + path + " states no mode")
	}
	list, err := NewList(path, family, audience, mode, sources...)
	if err != nil {
		panic("BUG: built-in dns-source default at " + path + " is invalid: " + err.Error())
	}
	list.stated = true
	return list
}

// Path answers the config path of the list's dns-source container.
func (l List) Path() string { return l.path }

// Sources answers the classes in list order. The slice is a copy.
func (l List) Sources() []Class { return slices.Clone(l.sources) }

// Mode answers the list's mode: ModeUnspecified only for a one-source list
// an operator wrote without one.
func (l List) Mode() Mode { return l.mode }

// Stated reports whether the list is the consumer's built-in default.
func (l List) Stated() bool { return l.stated }

// ReceivedInterfaces answers the interfaces the received class is narrowed
// to, in order; empty means every interface. The slice is a copy.
func (l List) ReceivedInterfaces() []string { return slices.Clone(l.interfaces) }

// IsZero reports whether l was built by neither NewList nor StatedDefault,
// which refuse a list naming no source. A consumer that holds a zero List
// holds no list at all, and MUST refuse to select from it.
func (l List) IsZero() bool { return len(l.sources) == 0 }

// Written is a dns-source container as the operator wrote it, read from the
// config tree by its consumer. A nil Written means the container is absent.
type Written struct {
	Sources            []Class
	Mode               Mode
	ReceivedInterfaces []string
}

// Refusal is a class a consumer cannot use, with the reason and the fix the
// operator reads when a list names it. Each consumer declares its own, so
// this package names no consumer.
type Refusal struct {
	Class Class
	Why   string
	Fix   string
}

// NewWrittenList validates the dns-source container an operator wrote at
// path and answers the list. It refuses, in this order: a class in refused
// (*ClassRefusedError), a received-interface with no received source
// (*ReceivedInterfaceError), then what NewList refuses. The consumer calls
// it from every place its config is checked, so validate, commit and boot
// give one verdict.
func NewWrittenList(path string, family Family, audience Audience, written *Written, refused ...Refusal) (List, error) {
	for _, source := range written.Sources {
		for i := range refused {
			if refused[i].Class == source {
				return List{}, &ClassRefusedError{Path: path, Refusal: refused[i]}
			}
		}
	}
	if len(written.ReceivedInterfaces) > 0 && !slices.Contains(written.Sources, ClassReceived) {
		return List{}, &ReceivedInterfaceError{Path: path, Interfaces: slices.Clone(written.ReceivedInterfaces)}
	}
	list, err := NewList(path, family, audience, written.Mode, written.Sources...)
	if err != nil {
		return List{}, err
	}
	list.interfaces = slices.Clone(written.ReceivedInterfaces)
	return list, nil
}

// Received answers the received class's entries for this list over records
// at now. With no received-interface it is ReceivedAddresses. With one, only
// the records learned on the named interfaces count, ordered by the
// interface's place in the list, then origin rank, then unit; an interface
// that holds no record contributes nothing (it may learn one later).
func (l List) Received(records []Record, now time.Time) []netip.Addr {
	if len(l.interfaces) == 0 {
		return ReceivedAddresses(records, now)
	}
	var narrowed []Record
	for _, name := range l.interfaces {
		var held []Record
		for i := range records {
			if records[i].Interface == name {
				held = append(held, records[i])
			}
		}
		slices.SortStableFunc(held, func(a, b Record) int { return CompareRecords(&a, &b) })
		narrowed = append(narrowed, held...)
	}
	return liveAddresses(narrowed, now)
}

// Candidates holds, for each class a consumer can name, the addresses that
// class offers it, in the class's own order. A class absent from the map
// offers nothing, so a list naming it falls through to the next.
type Candidates map[Class][]netip.Addr

// Selection is what a list chose: the servers in the order the consumer hands
// them on, and the classes that contributed at least one of them, in list
// order. An empty Servers means the consumer offers no resolver.
type Selection struct {
	Servers []netip.Addr
	Used    []Class
	// Self are the Servers entries the self class contributed, in Servers
	// order. They are Ze's own addresses, so they reach a client only while
	// some DNS service of Ze answers on them, which the doctor checks.
	Self []netip.Addr
	// Dropped are the host-only entries an AudienceHandedOn list refused,
	// in the order met. The consumer MUST log each with the list's path.
	Dropped []netip.Addr
}

// Select applies list to the candidates and answers the selection.
//
// An entry counts only when Usable and of a family the list admits. On an
// AudienceHandedOn list it must also be reachable from another host
// (HostOnly), and each host-only entry dropped is answered in Dropped so the
// consumer logs it with its path. Under
// ModeFirstUsable the first class holding such an entry wins, and a later
// class is used only when every earlier one holds none. Under ModeMerge every
// class contributes. Either way a server already selected is not repeated.
// A one-source list with ModeUnspecified answers that source's entries, which
// is what both modes answer for it.
func Select(list List, candidates Candidates) Selection {
	var selection Selection
	for _, class := range list.sources {
		before := len(selection.Servers)
		selection.Servers, selection.Dropped = appendUsable(selection.Servers, selection.Dropped, candidates[class], &list)
		if len(selection.Servers) == before {
			continue
		}
		selection.Used = append(selection.Used, class)
		if class == ClassSelf {
			selection.Self = append(selection.Self, selection.Servers[before:]...)
		}
		if list.mode != ModeMerge {
			break
		}
	}
	return selection
}

// appendUsable appends each usable address of entries, of a family list
// admits, that out does not already hold. On a handed-on list a host-only
// address goes to dropped instead.
func appendUsable(out, dropped, entries []netip.Addr, list *List) (servers, refused []netip.Addr) {
	for _, addr := range entries {
		if !Usable(addr) {
			continue
		}
		if !list.family.admits(addr) {
			continue
		}
		if list.audience == AudienceHandedOn && HostOnly(addr) {
			if !slices.Contains(dropped, addr) {
				dropped = append(dropped, addr)
			}
			continue
		}
		if slices.Contains(out, addr) {
			continue
		}
		out = append(out, addr)
	}
	return out, dropped
}

// HostOnly reports whether addr names a server only Ze itself can reach: a
// loopback address (127.0.0.0/8, ::1), or an IPv6 link-local address with no
// zone, which another host cannot tell which link to send on. IPv4
// link-local (169.254.0.0/16) is not host-only: it carries no zone to lose,
// and a server at such an address on the client's own link is reachable.
func HostOnly(addr netip.Addr) bool {
	unmapped := addr.Unmap()
	if unmapped.IsLoopback() {
		return true
	}
	if !unmapped.Is6() {
		return false
	}
	if !unmapped.IsLinkLocalUnicast() {
		return false
	}
	return unmapped.Zone() == ""
}

// limitedBroadcast is 255.255.255.255, which never names a host.
var limitedBroadcast = netip.AddrFrom4([4]byte{255, 255, 255, 255})

// Usable reports whether addr can be a resolver: a valid address that is not
// unspecified, not multicast and not the IPv4 limited broadcast. Loopback and
// link-local stay usable here: a local resolver such as 127.0.0.53 is a
// legitimate server for Ze's own lookups. A list handed to another host
// drops them in Select (HostOnly, AudienceHandedOn).
func Usable(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	if addr.IsUnspecified() {
		return false
	}
	if addr.IsMulticast() {
		return false
	}
	return addr.Unmap() != limitedBroadcast
}

// ModeRequiredError refuses a list of two or more sources that states no
// mode. What, Why and Fix are the three parts of the operator message.
type ModeRequiredError struct {
	Path    string
	Sources []Class
}

// What says what the operator wrote.
func (e *ModeRequiredError) What() string {
	names := make([]string, len(e.Sources))
	for i, source := range e.Sources {
		names[i] = string(source)
	}
	return e.Path + " names " + strconv.Itoa(len(e.Sources)) + " sources (" +
		strings.Join(names, ", ") + ") and no mode"
}

// Why says why Ze refuses it.
func (e *ModeRequiredError) Why() string {
	return "Ze cannot tell whether a later source is a backup or an addition"
}

// Fix gives the two commands that settle it.
func (e *ModeRequiredError) Fix() string {
	return "set " + e.Path + " mode first-usable (a later source is used only when the earlier ones have no usable server), or set " +
		e.Path + " mode merge (every source, in list order)"
}

// Error renders the three parts, the fix on a line of its own.
func (e *ModeRequiredError) Error() string {
	return e.What() + ": " + e.Why() + "\n" + e.Fix()
}

// EmptyListError refuses a dns-source container whose source list is empty:
// it would select no server whatever the network or the config held.
type EmptyListError struct {
	Path string
}

// Error renders what, why and the fix.
func (e *EmptyListError) Error() string {
	return e.Path + " names no source: a consumer with an empty source list offers no resolver\n" +
		"set " + e.Path + " source <class>, or delete " + e.Path + " to use its built-in default"
}

// RepeatedSourceError refuses a list naming one class twice: the second
// place can never contribute, so the list does not say what it reads.
type RepeatedSourceError struct {
	Path   string
	Source Class
}

// Error renders what, why and the fix.
func (e *RepeatedSourceError) Error() string {
	return e.Path + " names the source " + string(e.Source) + " twice: its second place can never contribute a server\n" +
		"delete the second " + e.Path + " source " + string(e.Source)
}

// ClassRefusedError refuses a list naming a class its consumer cannot use.
// The consumer's Refusal carries the reason and the fix.
type ClassRefusedError struct {
	Path    string
	Refusal Refusal
}

// What says what the operator wrote.
func (e *ClassRefusedError) What() string {
	return e.Path + " names " + string(e.Refusal.Class)
}

// Error renders what, why and the fix, the fix on a line of its own.
func (e *ClassRefusedError) Error() string {
	return e.What() + ": " + e.Refusal.Why + "\n" + e.Refusal.Fix
}

// ReceivedInterfaceError refuses a received-interface on a list that does
// not name received: the filter narrows only that class, so it would select
// nothing.
type ReceivedInterfaceError struct {
	Path       string
	Interfaces []string
}

// What says what the operator wrote.
func (e *ReceivedInterfaceError) What() string {
	return e.Path + " names received-interface " + strings.Join(e.Interfaces, ", ") +
		" but its source list does not name received"
}

// Why says why Ze refuses it.
func (e *ReceivedInterfaceError) Why() string {
	return "received-interface only narrows the received source, so without it the filter selects nothing"
}

// Fix gives the two commands that settle it. With one interface the delete
// names it; with several it deletes the whole leaf-list.
func (e *ReceivedInterfaceError) Fix() string {
	remove := "delete " + e.Path + " received-interface"
	if len(e.Interfaces) == 1 {
		remove += " " + e.Interfaces[0]
	}
	return "set " + e.Path + " source received, or " + remove
}

// Error renders the three parts, the fix on a line of its own.
func (e *ReceivedInterfaceError) Error() string {
	return e.What() + ": " + e.Why() + "\n" + e.Fix()
}
