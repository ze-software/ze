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
	routerSelf   = Refusal{Class: ClassSelf, Why: "Ze cannot be its own upstream", Fix: deleteSourceFix(ClassSelf)}
	routerRadius = Refusal{Class: ClassRadius, Why: "the router has no subscriber", Fix: deleteSourceFix(ClassRadius)}
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

// TestReadWrittenFromPluginJSON: a plugin reads its dns-source container from
// config JSON, a leaf-list as an array or a bare string, and an unknown class
// or mode is refused naming the path and the fix.
//
// VALIDATES: ReadWritten, the plugin consumers' reader (spec-home-cpe-23b,
// AC-16b's input). PREVENTS: a consumer reading a one-entry leaf-list as
// absent and silently taking its default.
func TestReadWrittenFromPluginJSON(t *testing.T) {
	const path = "service dns-resolver forward dns-source"
	written, err := ReadWritten(path, map[string]any{
		"source": []any{"received", "configured"}, "mode": "merge", "received-interface": "eth0",
	})
	if err != nil {
		t.Fatalf("ReadWritten: %v", err)
	}
	if want := []Class{ClassReceived, ClassConfigured}; !slices.Equal(written.Sources, want) {
		t.Fatalf("sources = %v, want %v", written.Sources, want)
	}
	if written.Mode != ModeMerge {
		t.Fatalf("mode = %v, want merge", written.Mode)
	}
	if want := []string{"eth0"}; !slices.Equal(written.ReceivedInterfaces, want) {
		t.Fatalf("received-interface = %v, want %v", written.ReceivedInterfaces, want)
	}
	one, err := ReadWritten(path, map[string]any{"source": "received"})
	if err != nil || !slices.Equal(one.Sources, []Class{ClassReceived}) || one.Mode != ModeUnspecified {
		t.Fatalf("bare-string source: %+v, %v", one, err)
	}
	for _, body := range []map[string]any{
		{"source": []any{"bogus"}},
		{"source": "received", "mode": "sometimes"},
	} {
		_, err := ReadWritten(path, body)
		if err == nil {
			t.Fatalf("ReadWritten(%v) accepted", body)
		}
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("refusal %q does not name %q", err, path)
		}
	}
}

// deleteSourceFix answers a Refusal fix that deletes class from the list at
// the path it is given, as each consumer's fix starts.
func deleteSourceFix(class Class) func(path string) string {
	return func(path string) string { return "delete " + path + " source " + string(class) }
}

// TestParseContainerAbsentWrittenAndMalformed: an absent container answers
// the built-in default, a written one the written list under the default's
// schema path, and a container that is not an object is refused naming the
// instance path, never read as absent.
//
// VALIDATES: ParseContainer, the one reader every consumer's dns-source goes
// through. PREVENTS: the default silently standing in for a list the operator
// wrote, which the per-consumer type assertions did; a refusal naming the
// schema path, which holds no list key and so is no node the operator can
// set or delete.
func TestParseContainerAbsentWrittenAndMalformed(t *testing.T) {
	const at = "service dhcp-server shared-network LAN subnet 192.0.2.0/24 dns-source"
	builtin := StatedDefault("service dhcp-server shared-network subnet dns-source", FamilyIPv4, AudienceHandedOn,
		ModeFirstUsable, ClassConfigured, ClassReceived)

	got, err := ParseContainer(map[string]any{}, at, builtin, FamilyIPv4, AudienceHandedOn)
	if err != nil {
		t.Fatalf("absent container refused: %v", err)
	}
	if !got.Stated() {
		t.Errorf("absent container: got a written list, want the built-in default")
	}

	got, err = ParseContainer(map[string]any{ContainerKey: map[string]any{"source": "self"}}, at, builtin, FamilyIPv4, AudienceHandedOn)
	if err != nil {
		t.Fatalf("written container refused: %v", err)
	}
	if got.Stated() || len(got.Sources()) != 1 || got.Sources()[0] != ClassSelf {
		t.Errorf("written container: sources %v default %v, want [self] written", got.Sources(), got.Stated())
	}
	if got.Path() != builtin.Path() {
		t.Errorf("written container: path %q, want the schema path %q its publications carry", got.Path(), builtin.Path())
	}

	_, err = ParseContainer(map[string]any{ContainerKey: "self"}, at, builtin, FamilyIPv4, AudienceHandedOn)
	if err == nil {
		t.Fatal("a dns-source that is not a container was accepted")
	}
	if !strings.Contains(err.Error(), at+" is not a container") {
		t.Errorf("refusal %q does not name the instance path %q", err, at)
	}
}

// TestParseContainerRefusalNamesInstance: every refusal of a written list
// names the instance path the consumer passed, in what it says and in the
// fix command, for a refused class, a missing mode and an unknown word.
//
// Method: one consumer with a keyed instance path that differs from its
// default's schema path; each body is refused, and the error must hold the
// instance path and a fix line on it, and must not hold the schema path.
//
// VALIDATES: the refusals of a keyed consumer (a unit, a subnet, a named
// pool) give a fix the operator can run as printed.
// PREVENTS: a fix command built from the schema path, which names no list
// key, so the CLI refuses it and the operator cannot tell which instance was
// refused.
func TestParseContainerRefusalNamesInstance(t *testing.T) {
	const at = "interface ethernet eth0 unit 0 ipv6 router-advertisement rdnss dns-source"
	const schema = "interface unit ipv6 router-advertisement rdnss dns-source"
	builtin := StatedDefault(schema, FamilyIPv6, AudienceHandedOn, ModeFirstUsable, ClassConfigured, ClassReceived)
	radius := Refusal{Class: ClassRadius, Why: "a host is not a subscriber", Fix: deleteSourceFix(ClassRadius)}
	cases := []struct {
		name string
		body map[string]any
		fix  string
	}{
		{name: "refused class", body: map[string]any{"source": "radius"}, fix: "delete " + at + " source radius"},
		{name: "no mode", body: map[string]any{"source": []any{"received", "configured"}}, fix: "set " + at + " mode first-usable"},
		{name: "unknown class", body: map[string]any{"source": "dhcp"}, fix: "delete " + at + " source dhcp"},
	}
	for _, tc := range cases {
		_, err := ParseContainer(map[string]any{ContainerKey: tc.body}, at, builtin, FamilyIPv6, AudienceHandedOn, radius)
		if err == nil {
			t.Fatalf("%s: accepted", tc.name)
		}
		lines := strings.Split(err.Error(), "\n")
		if len(lines) != 2 {
			t.Fatalf("%s: error %q has %d lines, want what and why, then the fix", tc.name, err, len(lines))
		}
		if !strings.HasPrefix(lines[0], at+" ") {
			t.Errorf("%s: first line %q does not open with the instance path %q", tc.name, lines[0], at)
		}
		if !strings.HasPrefix(lines[1], tc.fix) {
			t.Errorf("%s: fix line %q, want it to start %q", tc.name, lines[1], tc.fix)
		}
		if strings.Contains(err.Error(), schema) {
			t.Errorf("%s: error %q names the schema path %q, which no command accepts", tc.name, err, schema)
		}
	}
}
