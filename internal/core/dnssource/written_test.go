package dnssource

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// routerSelf and routerRadius are the refusals a consumer shaped like the
// router declares. The text is the test's own: this package names no consumer.
var (
	routerSelf   = Refusal{Class: ClassSelf, Why: "Ze cannot be its own upstream", Fix: "delete system dns-source source self"}
	routerRadius = Refusal{Class: ClassRadius, Why: "the router has no subscriber", Fix: "delete system dns-source source radius"}
)

// TestReceivedInterfaceFilter: with received-interface set, received takes
// only the named interfaces' records, ordered by the leaf-list first, then by
// origin rank inside one interface, then by unit; an interface with no record
// adds nothing; an expired record never counts.
//
// VALIDATES: AC-24's selection half and D-11's order (spec-home-cpe-23,
// "Source classes").
// PREVENTS: the filter leaking another interface's resolvers into the router,
// and the leaf-list order losing to the global rank order.
func TestReceivedInterfaceFilter(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	records := []Record{
		{Origin: "dhcpv4", Rank: 100, Interface: "eth0", Unit: "0", Addresses: addrs(t, "192.0.2.53")},
		{Origin: "ppp", Rank: 300, Interface: "eth1", Unit: "", Addresses: addrs(t, "198.51.100.54")},
		{Origin: "dhcpv4", Rank: 100, Interface: "eth1", Unit: "0", Addresses: addrs(t, "198.51.100.53")},
		{Origin: "dhcpv4", Rank: 100, Interface: "eth2", Unit: "0", Addresses: addrs(t, "203.0.113.53"),
			Expires: now.Add(-time.Second)},
	}

	one, err := NewWrittenList("system dns-source", FamilyAll, AudienceZe,
		&Written{Sources: []Class{ClassReceived}, ReceivedInterfaces: []string{"eth1"}}, routerSelf, routerRadius)
	if err != nil {
		t.Fatalf("one-source list with received-interface and no mode refused: %v", err)
	}
	got := Select(one, Candidates{ClassReceived: one.Received(records, now)})
	if want := addrs(t, "198.51.100.53", "198.51.100.54"); !slices.Equal(got.Servers, want) {
		t.Fatalf("eth1 only: servers = %v, want %v (eth1's dhcpv4 before its ppp, eth0 left out)", got.Servers, want)
	}

	// The leaf-list order beats the rank order: eth1's ppp record lists
	// before eth0's dhcpv4 one. eth2 holds only an expired record, eth9 none.
	ordered, err := NewWrittenList("system dns-source", FamilyAll, AudienceZe,
		&Written{Sources: []Class{ClassReceived}, ReceivedInterfaces: []string{"eth9", "eth2", "eth1", "eth0"}})
	if err != nil {
		t.Fatalf("NewWrittenList: %v", err)
	}
	if got, want := ordered.Received(records, now), addrs(t, "198.51.100.53", "198.51.100.54", "192.0.2.53"); !slices.Equal(got, want) {
		t.Fatalf("ordered: received = %v, want %v", got, want)
	}

	// No filter: every interface, by rank first, as ReceivedAddresses answers.
	all := mustList(t, FamilyAll, ModeUnspecified, ClassReceived)
	if got, want := all.Received(records, now), ReceivedAddresses(records, now); !slices.Equal(got, want) {
		t.Fatalf("unfiltered: received = %v, want %v", got, want)
	}
}

// TestReceivedInterfaceWithoutReceivedRefused: received-interface on a list
// that does not name received is refused with the what, why and fix of the
// spec, naming the consumer's path; so is one on a container with no source.
//
// VALIDATES: AC-24's refusal (spec-home-cpe-23, "Source classes").
// PREVENTS: a filter that silently selects nothing being accepted.
func TestReceivedInterfaceWithoutReceivedRefused(t *testing.T) {
	for _, written := range []Written{
		{Sources: []Class{ClassConfigured}, ReceivedInterfaces: []string{"eth0"}},
		{ReceivedInterfaces: []string{"eth0"}},
	} {
		_, err := NewWrittenList("system dns-source", FamilyAll, AudienceZe, &written)
		var refused *ReceivedInterfaceError
		if !errors.As(err, &refused) {
			t.Fatalf("%+v: err = %v, want *ReceivedInterfaceError", written, err)
		}
		for _, want := range []string{
			"system dns-source names received-interface eth0 but its source list does not name received",
			"received-interface only narrows the received source, so without it the filter selects nothing",
			"\nset system dns-source source received, or delete system dns-source received-interface eth0",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("%+v: error %q lacks %q", written, err.Error(), want)
			}
		}
	}

	_, err := NewWrittenList("system dns-source", FamilyAll, AudienceZe,
		&Written{Sources: []Class{ClassConfigured}, ReceivedInterfaces: []string{"eth0", "eth1"}})
	if err == nil || !strings.HasSuffix(err.Error(), "or delete system dns-source received-interface") {
		t.Fatalf("two interfaces: err = %v, want the fix deleting the whole leaf-list", err)
	}
}

// TestWrittenListRefusesConsumerClasses: a class the consumer refuses is
// refused with the consumer's own why and fix, before any other check; a
// list naming only allowed classes passes through to NewList's checks.
//
// VALIDATES: the class half of AC-11 for a consumer shaped like the router.
// PREVENTS: a refused class being accepted and selecting nothing, or the
// mode refusal hiding the class refusal.
func TestWrittenListRefusesConsumerClasses(t *testing.T) {
	_, err := NewWrittenList("system dns-source", FamilyAll, AudienceZe,
		&Written{Sources: []Class{ClassReceived, ClassSelf}}, routerSelf, routerRadius)
	var refused *ClassRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("self: err = %v, want *ClassRefusedError ahead of the mode refusal", err)
	}
	if want := "system dns-source names self: Ze cannot be its own upstream\ndelete system dns-source source self"; err.Error() != want {
		t.Fatalf("self: error = %q, want %q", err.Error(), want)
	}

	_, err = NewWrittenList("system dns-source", FamilyAll, AudienceZe, &Written{Sources: []Class{ClassRadius}}, routerSelf, routerRadius)
	if !errors.As(err, &refused) || refused.Refusal.Class != ClassRadius {
		t.Fatalf("radius: err = %v, want the radius refusal", err)
	}

	_, err = NewWrittenList("system dns-source", FamilyAll, AudienceZe,
		&Written{Sources: []Class{ClassReceived, ClassConfigured}}, routerSelf, routerRadius)
	var mode *ModeRequiredError
	if !errors.As(err, &mode) {
		t.Fatalf("two allowed sources, no mode: err = %v, want *ModeRequiredError", err)
	}
}

// TestDefaultListIsStatedNotLent: a stated default carries its mode and the
// stated mark; an operator list written over it carries only what was
// written, so a one-source list keeps ModeUnspecified and is not stated.
//
// VALIDATES: D-5's "the default never lends its mode" (spec-home-cpe-23,
// "Built-in default lists").
// PREVENTS: show and the selection reporting a written list as the default,
// or the default's first-usable leaking into a written list.
func TestDefaultListIsStatedNotLent(t *testing.T) {
	stated := StatedDefault("system dns-source", FamilyAll, AudienceZe, ModeFirstUsable, ClassConfigured, ClassReceived)
	if !stated.Stated() || stated.Mode() != ModeFirstUsable {
		t.Fatalf("default: stated=%v mode=%v, want stated first-usable", stated.Stated(), stated.Mode())
	}
	written, err := NewWrittenList("system dns-source", FamilyAll, AudienceZe, &Written{Sources: []Class{ClassReceived}})
	if err != nil {
		t.Fatalf("NewWrittenList: %v", err)
	}
	if written.Stated() || written.Mode() != ModeUnspecified {
		t.Fatalf("written: stated=%v mode=%v, want not stated, unspecified", written.Stated(), written.Mode())
	}
	var zero List
	if !zero.IsZero() || written.IsZero() || stated.IsZero() {
		t.Fatalf("IsZero: zero=%v written=%v stated=%v, want true false false", zero.IsZero(), written.IsZero(), stated.IsZero())
	}
}

// TestParseClass: the four operator words parse, anything else is refused.
func TestParseClass(t *testing.T) {
	for _, word := range []string{"received", "configured", "self", "radius"} {
		if class, ok := ParseClass(word); !ok || string(class) != word {
			t.Fatalf("ParseClass(%q) = %q, %v", word, class, ok)
		}
	}
	if _, ok := ParseClass("dhcpv4"); ok {
		t.Fatal(`ParseClass("dhcpv4") accepted a protocol name as a class`)
	}
}
