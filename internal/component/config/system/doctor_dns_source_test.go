package system

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/dnssource"
)

// findingCodes answers the codes of diags, in order.
func findingCodes(diags []diagnostic.Diagnostic) []string {
	codes := make([]string, len(diags))
	for i := range diags {
		codes[i] = diags[i].Code
	}
	return codes
}

// TestDNSSourceDoctorFindings proves each of the three dns-source doctor
// findings fires on the state that owes it and on no other. Method: build a
// snapshot and publications by hand, one fault per case, and compare the
// finding codes.
//
// VALIDATES: the Integration Checklist doctor row of spec-home-cpe-23b (empty
// selection while received is named, D-11 silent received-interface, D-6/R-9
// self not served).
// PREVENTS: a consumer silently offering no resolver, a received-interface
// typo going unseen, or a self address handed to clients that no listener
// answers on.
func TestDNSSourceDoctorFindings(t *testing.T) {
	lanAddr := netip.MustParseAddr("192.0.2.1")
	learned := dnssource.Record{Interface: "eth0", Addresses: []netip.Addr{netip.MustParseAddr("198.51.100.53")}}
	router := dnssource.ConsumerSelection{
		Path:    routerDNSSourcePath,
		Sources: []dnssource.Class{dnssource.ClassReceived, dnssource.ClassConfigured},
		Servers: []netip.Addr{netip.MustParseAddr("198.51.100.53")},
	}
	subnet := dnssource.ConsumerSelection{
		Path:     "service dhcp-server subnet dns-source",
		Instance: "192.0.2.0/24",
		Sources:  []dnssource.Class{dnssource.ClassSelf},
		Servers:  []netip.Addr{lanAddr},
		Self:     []netip.Addr{lanAddr},
	}
	resolver := dnssource.ConsumerSelection{
		Path:    "service dns-resolver forward dns-source",
		Sources: []dnssource.Class{dnssource.ClassReceived},
		Servers: []netip.Addr{netip.MustParseAddr("198.51.100.53")},
		Serves:  []netip.Addr{lanAddr},
	}
	healthy := []dnssource.Publication{
		{Consumer: "dhcp-server", Selections: []dnssource.ConsumerSelection{subnet}},
		{Consumer: "dns-resolver", Selections: []dnssource.ConsumerSelection{resolver}},
	}
	snapshot := dnssource.Snapshot{Records: []dnssource.Record{learned}, Router: router}

	if got := dnsSourceFindings(snapshot, healthy); len(got) != 0 {
		t.Fatalf("healthy state: findings %v, want none", findingCodes(got))
	}

	// Empty: the router names received and selected nothing.
	empty := snapshot
	empty.Router.Servers = nil
	if got := findingCodes(dnsSourceFindings(empty, healthy)); !slices.Equal(got, []string{diagnostic.CodeDoctorDNSSourceEmpty}) {
		t.Errorf("empty router selection: findings %v, want [%s]", got, diagnostic.CodeDoctorDNSSourceEmpty)
	}
	// An empty selection of a list that does not name received is the
	// operator's choice, not a finding.
	noReceived := empty
	noReceived.Router.Sources = []dnssource.Class{dnssource.ClassConfigured}
	if got := dnsSourceFindings(noReceived, healthy); len(got) != 0 {
		t.Errorf("empty configured-only selection: findings %v, want none", findingCodes(got))
	}

	// Silent interface: received-interface names one with no record.
	silent := snapshot
	silent.Router.ReceivedInterfaces = []string{"eth0", "wan9"}
	got := dnsSourceFindings(silent, healthy)
	if !slices.Equal(findingCodes(got), []string{diagnostic.CodeDoctorDNSSourceInterfaceSilent}) {
		t.Fatalf("silent interface: findings %v, want [%s]", findingCodes(got), diagnostic.CodeDoctorDNSSourceInterfaceSilent)
	}
	if want := routerDNSSourcePath + ": received-interface wan9 holds no learned resolver, so the list never selects from it; run show system dns"; got[0].Message != want {
		t.Errorf("silent interface message = %q, want %q", got[0].Message, want)
	}

	// Self not served: the resolver listens elsewhere.
	elsewhere := resolver
	elsewhere.Serves = []netip.Addr{netip.MustParseAddr("192.0.2.254")}
	unserved := []dnssource.Publication{healthy[0], {Consumer: "dns-resolver", Selections: []dnssource.ConsumerSelection{elsewhere}}}
	got = dnsSourceFindings(snapshot, unserved)
	if !slices.Equal(findingCodes(got), []string{diagnostic.CodeDoctorDNSSourceSelfUnserved}) {
		t.Fatalf("self unserved: findings %v, want [%s]", findingCodes(got), diagnostic.CodeDoctorDNSSourceSelfUnserved)
	}
	if want := "service dhcp-server subnet dns-source (192.0.2.0/24): hands out 192.0.2.1 through self"; len(got[0].Message) < len(want) || got[0].Message[:len(want)] != want {
		t.Errorf("self unserved message = %q, want prefix %q", got[0].Message, want)
	}

	// Disabled resolver: it publishes an empty selection set, binds nothing
	// and answers nobody, so the self address is unserved and its received
	// list, which nothing uses, owes no empty finding.
	disabled := []dnssource.Publication{healthy[0], {Consumer: "dns-resolver", Selections: []dnssource.ConsumerSelection{}}}
	if got := findingCodes(dnsSourceFindings(snapshot, disabled)); !slices.Equal(got, []string{diagnostic.CodeDoctorDNSSourceSelfUnserved}) {
		t.Errorf("disabled resolver: findings %v, want [%s]", got, diagnostic.CodeDoctorDNSSourceSelfUnserved)
	}
}

// TestDNSSourceDoctorCheckRunsOnlyOnTheDaemonRoute proves the check is
// registered as a daemon doctor check and as no ordinary one, so the local
// ze doctor, whose registry is empty, never answers "fine" for it.
func TestDNSSourceDoctorCheckRunsOnlyOnTheDaemonRoute(t *testing.T) {
	found := false
	for _, check := range diagnostic.DaemonDoctorChecks() {
		if check.Name == dnsSourceDoctorCheckName {
			found = true
		}
	}
	if !found {
		t.Fatalf("daemon doctor checks lack %s", dnsSourceDoctorCheckName)
	}
	if slices.Contains(diagnostic.DoctorCheckNames(), dnsSourceDoctorCheckName) {
		t.Errorf("%s is also an ordinary doctor check: the local route would run it over an empty registry", dnsSourceDoctorCheckName)
	}
	diagnostic.RegisterBuiltinCodes()
	for _, code := range dnsSourceDoctorCodes {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("code %s does not resolve through diagnostic.Lookup", code)
		}
	}
}
