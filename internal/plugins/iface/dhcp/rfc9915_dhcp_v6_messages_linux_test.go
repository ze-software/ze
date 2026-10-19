// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: dhcp_v6_exchange_linux_test.go -- testWorker and testWorkerAnswer
// Related: dhcp_v6_lease_linux_test.go -- the lease fixtures these tests reuse
// RFC: rfc/short/rfc9915.md

//go:build linux

package ifacedhcp

import (
	"context"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/iana"
)

// TestRFC9915Section16ClientDiscardsOtherMessageTypes offers the reply
// matcher and the Advertise collector every message type a client must
// discard, each with a valid Server ID and this client's Client ID.
//
// VALIDATES: no message type but Reply (and Advertise during Solicit) is
// taken as an answer, a Reconfigure included, and a discarded Reconfigure
// leaves the held lease in place.
func TestRFC9915Section16ClientDiscardsOtherMessageTypes(t *testing.T) {
	// RFC requirement: RFC9915-16.2-1 negative -- a Solicit carrying this client's Client ID and a Server ID is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-16.4-1 negative -- a Request carrying this client's Client ID and a Server ID is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-16.5-1 negative -- a Confirm carrying this client's Client ID and a Server ID is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-16.6-1 negative -- a Renew carrying this client's Client ID and a Server ID is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-16.7-1 negative -- a Rebind carrying this client's Client ID and a Server ID is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-16.8-1 negative -- a Decline carrying this client's Client ID and a Server ID is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-16.9-1 negative -- a Release carrying this client's Client ID and a Server ID is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-16.12-1 negative -- an Information-request carrying this client's Client ID and a Server ID is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-16.13-1 negative -- a Relay-forward is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-16.14-1 negative -- a Relay-reply is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-16.11-2 negative -- a Reconfigure carrying this client's Client ID and a Server ID, with no Authentication option, is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-18.2.11-1 negative -- a Reconfigure with no Authentication option is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-22.3-1 negative -- a Reconfigure with no Authentication option is refused by the reply matcher and the Advertise collector.
	// RFC requirement: RFC9915-18.2-1 positive -- after the reply matcher refuses a Reconfigure, the worker still holds the same lease record.
	w := testWorker(t, dHCPConfig{})
	held := &v6Lease{server: v6ServerA}
	w.lease = held
	for _, kind := range []dhcpv6.MessageType{
		dhcpv6.MessageTypeSolicit,
		dhcpv6.MessageTypeRequest,
		dhcpv6.MessageTypeConfirm,
		dhcpv6.MessageTypeRenew,
		dhcpv6.MessageTypeRebind,
		dhcpv6.MessageTypeDecline,
		dhcpv6.MessageTypeRelease,
		dhcpv6.MessageTypeInformationRequest,
		dhcpv6.MessageTypeRelayForward,
		dhcpv6.MessageTypeRelayReply,
		dhcpv6.MessageTypeReconfigure,
	} {
		msg := testWorkerAnswer(w, kind, lease10x16x30()...)
		if w.replyMatcher()(msg) {
			t.Errorf("%s accepted as a Reply", kind)
		}
		collector := &v6AdvertiseCollector{w: w}
		if collector.accept(msg) {
			t.Errorf("%s accepted by the Advertise collector", kind)
		}
		if collector.windowClosed() != nil {
			t.Errorf("%s chosen as the best Advertise", kind)
		}
	}
	if w.lease != held {
		t.Error("a discarded Reconfigure changed the held lease")
	}
	if !w.replyMatcher()(testWorkerAnswer(w, dhcpv6.MessageTypeReply, lease10x16x30()...)) {
		t.Error("control: a valid Reply was refused")
	}
}

// TestRFC9915Section163AdvertiseValidation offers the Advertise collector
// an Advertise per discard condition, one with no lease, and one whose
// server DUID is of a type this document does not define.
//
// VALIDATES: an Advertise without Server ID or with a foreign Client ID is
// refused; one with no IA Address and no IA Prefix is refused but its
// SOL_MAX_RT is applied even beside a failure status; an unknown DUID type
// is taken.
func TestRFC9915Section163AdvertiseValidation(t *testing.T) {
	// RFC requirement: RFC9915-16.3-1 negative -- an Advertise with no Server ID, and one whose Client ID is another DUID, are each refused and never chosen.
	// RFC requirement: RFC9915-18.2.9-2 negative -- an Advertise that carries no IA Address in an IA_NA and no IA Prefix in an IA_PD is refused and never chosen.
	// RFC requirement: RFC9915-18.2.9-1 positive -- an Advertise with Status Code NoAddrsAvail and SOL_MAX_RT 120 sets the client's SOL_MAX_RT to 120 s.
	// RFC requirement: RFC9915-18.2.9-3 positive -- an Advertise with no lease and SOL_MAX_RT 120 sets the client's SOL_MAX_RT to 120 s although it is refused.
	// RFC requirement: RFC9915-11-2 positive -- an Advertise whose Server ID is a DUID of type 0x7fff, which this document does not define, is taken.
	w := testWorker(t, dHCPConfig{})
	pref255 := &dhcpv6.OptionGeneric{OptionCode: dhcpv6.OptionPreference, OptionData: []byte{255}}

	noServer := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeAdvertise}
	noServer.AddOption(dhcpv6.OptClientID(w.duid))
	for _, o := range lease10x16x30() {
		noServer.AddOption(o)
	}
	noServer.AddOption(pref255)
	foreign := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeAdvertise}
	foreign.AddOption(dhcpv6.OptServerID(v6ServerA))
	foreign.AddOption(dhcpv6.OptClientID(v6ServerB))
	for _, o := range lease10x16x30() {
		foreign.AddOption(o)
	}
	foreign.AddOption(pref255)
	for name, msg := range map[string]*dhcpv6.Message{"no Server ID": noServer, "foreign Client ID": foreign} {
		a := &v6AdvertiseCollector{w: w}
		if a.accept(msg) {
			t.Errorf("Advertise with %s accepted", name)
		}
		if a.windowClosed() != nil {
			t.Errorf("Advertise with %s chosen", name)
		}
	}

	empty := testWorkerAnswer(w, dhcpv6.MessageTypeAdvertise, solMaxRTOption(120),
		&dhcpv6.OptStatusCode{StatusCode: iana.StatusNoAddrsAvail}, pref255)
	a := &v6AdvertiseCollector{w: w}
	if a.accept(empty) {
		t.Error("Advertise with no lease accepted")
	}
	if a.windowClosed() != nil {
		t.Error("Advertise with no lease chosen")
	}
	if w.solMaxRT != 120*time.Second {
		t.Errorf("SOL_MAX_RT %v after a failure Advertise carrying 120, want 120s", w.solMaxRT)
	}

	unknownType := &dhcpv6.DUIDOpaque{Type: dhcpv6.DUIDType(0x7fff), Data: []byte{1, 2, 3, 4}}
	req := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeSolicit}
	req.AddOption(dhcpv6.OptClientID(w.duid))
	odd := testV6Answer(req, dhcpv6.MessageTypeAdvertise, unknownType, append(lease10x16x30(), pref255)...)
	if !(&v6AdvertiseCollector{w: w}).accept(odd) {
		t.Error("Advertise from a server DUID of an unknown type refused")
	}
}

// TestRFC9915Section1821SolicitContents builds a Solicit with and without
// prefix delegation and lists its options.
//
// VALIDATES: the Solicit carries this client's DUID and an Elapsed Time, and
// no option outside Client ID, Elapsed Time, ORO, Rapid Commit, IA_NA and
// IA_PD; in particular no IA_TA.
func TestRFC9915Section1821SolicitContents(t *testing.T) {
	// RFC requirement: RFC9915-18.2.1-1 positive -- the Solicit carries a Client Identifier option equal to the client's DUID.
	// RFC requirement: RFC9915-18.2.1-2 positive -- the Solicit carries an Elapsed Time option.
	// RFC requirement: RFC9915-18.2.1-4 positive -- the Solicit carries no option other than Client ID, Elapsed Time, Option Request, Rapid Commit, IA_NA and IA_PD.
	// RFC requirement: RFC9915-A-1 positive -- the Solicit carries no IA_TA option.
	allowed := []dhcpv6.OptionCode{
		dhcpv6.OptionClientID, dhcpv6.OptionElapsedTime, dhcpv6.OptionORO,
		dhcpv6.OptionRapidCommit, dhcpv6.OptionIANA, dhcpv6.OptionIAPD,
	}
	for _, cfg := range []dHCPConfig{{}, {PDRequested: true, PDLength: 56}} {
		w := testWorker(t, cfg)
		m, err := w.buildSolicit()
		if err != nil {
			t.Fatal(err)
		}
		if id := m.Options.ClientID(); id == nil || !id.Equal(w.duid) {
			t.Errorf("Solicit Client ID %v, want %v", id, w.duid)
		}
		if m.Options.GetOne(dhcpv6.OptionElapsedTime) == nil {
			t.Error("Solicit carries no Elapsed Time")
		}
		for _, o := range m.Options.Options {
			if !slices.Contains(allowed, o.Code()) {
				t.Errorf("Solicit carries option %v", o.Code())
			}
		}
		if m.Options.GetOne(dhcpv6.OptionIATA) != nil {
			t.Error("Solicit carries an IA_TA")
		}
	}
}

// TestRFC9915Section1822RequestContents builds the Request answering an
// Advertise.
//
// VALIDATES: the Request names the Advertise's server, carries this
// client's DUID, an Elapsed Time and an ORO asking for SOL_MAX_RT.
func TestRFC9915Section1822RequestContents(t *testing.T) {
	// RFC requirement: RFC9915-18.2.2-1 positive -- the Request carries a Server Identifier option equal to the chosen Advertise's server DUID.
	// RFC requirement: RFC9915-18.2.2-2 positive -- the Request carries a Client Identifier option equal to the client's DUID.
	// RFC requirement: RFC9915-18.2.2-3 positive -- the Request carries an Elapsed Time option.
	// RFC requirement: RFC9915-18.2.2-4 positive -- the Request's Option Request option lists SOL_MAX_RT.
	w := testWorker(t, dHCPConfig{})
	m, err := w.buildRequest(testWorkerAnswer(w, dhcpv6.MessageTypeAdvertise, lease10x16x30()...))
	if err != nil {
		t.Fatal(err)
	}
	if sid := m.Options.ServerID(); sid == nil || !sid.Equal(v6ServerA) {
		t.Errorf("Request Server ID %v, want %v", sid, v6ServerA)
	}
	if id := m.Options.ClientID(); id == nil || !id.Equal(w.duid) {
		t.Errorf("Request Client ID %v, want %v", id, w.duid)
	}
	if m.Options.GetOne(dhcpv6.OptionElapsedTime) == nil {
		t.Error("Request carries no Elapsed Time")
	}
	if !slices.Contains(m.Options.RequestedOptions(), dhcpv6.OptionSolMaxRT) {
		t.Errorf("Request ORO %v lacks SOL_MAX_RT", m.Options.RequestedOptions())
	}
}

// v6TestLease is a lease record holding the lease10x16x30 address, applied
// at the Unix epoch.
func v6TestLease(w *v6Worker) *v6Lease {
	lease := &v6Lease{server: v6ServerA}
	lease.apply(testWorkerAnswer(w, dhcpv6.MessageTypeReply, lease10x16x30()...), time.Unix(0, 0), w.iaidNA, w.iaidPD)
	return lease
}

// TestRFC9915Section217OptionRequestEveryMessage builds every message the
// client sends from a held lease and lists its Option Request option.
//
// VALIDATES: Solicit, Request, Renew and Rebind each carry an ORO holding
// exactly DNS Recursive Name Server and SOL_MAX_RT, and every message,
// Release included, carries an Elapsed Time.
func TestRFC9915Section217OptionRequestEveryMessage(t *testing.T) {
	// RFC requirement: RFC9915-21.7-1 positive -- Solicit, Request, Renew and Rebind each carry an Option Request option.
	// RFC requirement: RFC9915-21.7-2 positive -- the Option Request option of Solicit, Request, Renew and Rebind holds exactly codes 23 and 82, so none of the codes the section forbids.
	// RFC requirement: RFC9915-21.23-2 positive -- the Option Request option of Solicit, Request, Renew and Rebind does not list the Information Refresh Time option.
	// RFC requirement: RFC9915-21.9-1 positive -- Solicit, Request, Renew, Rebind and Release each carry an Elapsed Time option.
	w := testWorker(t, dHCPConfig{})
	lease := v6TestLease(w)
	now := time.Unix(1, 0)
	solicit, err := w.buildSolicit()
	if err != nil {
		t.Fatal(err)
	}
	request, err := w.buildRequest(testWorkerAnswer(w, dhcpv6.MessageTypeAdvertise, lease10x16x30()...))
	if err != nil {
		t.Fatal(err)
	}
	withORO := []*dhcpv6.Message{solicit, request}
	for _, kind := range []dhcpv6.MessageType{dhcpv6.MessageTypeRenew, dhcpv6.MessageTypeRebind} {
		m, err := w.buildFromLease(kind, lease, now)
		if err != nil {
			t.Fatal(err)
		}
		withORO = append(withORO, m)
	}
	want := []dhcpv6.OptionCode{dhcpv6.OptionDNSRecursiveNameServer, dhcpv6.OptionSolMaxRT}
	for _, m := range withORO {
		if m.Options.GetOne(dhcpv6.OptionORO) == nil {
			t.Errorf("%s carries no Option Request option", m.MessageType)
			continue
		}
		if got := m.Options.RequestedOptions(); !slices.Equal(got, want) {
			t.Errorf("%s ORO %v, want %v", m.MessageType, got, want)
		}
		if slices.Contains(m.Options.RequestedOptions(), dhcpv6.OptionInformationRefreshTime) {
			t.Errorf("%s ORO asks for the Information Refresh Time", m.MessageType)
		}
	}
	release, err := w.buildFromLease(dhcpv6.MessageTypeRelease, lease, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range append(withORO, release) {
		if m.Options.GetOne(dhcpv6.OptionElapsedTime) == nil {
			t.Errorf("%s carries no Elapsed Time", m.MessageType)
		}
	}
}

// TestRFC9915Section1827ReleaseContents builds the Release for a held
// address.
//
// VALIDATES: the Release names the leasing server, carries this client's
// DUID, an Elapsed Time, and the held address in its IA_NA.
func TestRFC9915Section1827ReleaseContents(t *testing.T) {
	// RFC requirement: RFC9915-18.2.7-1 positive -- the Release carries a Server Identifier option equal to the leasing server's DUID.
	// RFC requirement: RFC9915-18.2.7-2 positive -- the Release carries a Client Identifier option equal to the client's DUID.
	// RFC requirement: RFC9915-18.2.7-3 positive -- the Release carries an Elapsed Time option.
	// RFC requirement: RFC9915-18.2.7-4 positive -- the Release's IA_NA carries the held address.
	w := testWorker(t, dHCPConfig{})
	m, err := w.buildFromLease(dhcpv6.MessageTypeRelease, v6TestLease(w), time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if sid := m.Options.ServerID(); sid == nil || !sid.Equal(v6ServerA) {
		t.Errorf("Release Server ID %v, want %v", sid, v6ServerA)
	}
	if id := m.Options.ClientID(); id == nil || !id.Equal(w.duid) {
		t.Errorf("Release Client ID %v, want %v", id, w.duid)
	}
	if m.Options.GetOne(dhcpv6.OptionElapsedTime) == nil {
		t.Error("Release carries no Elapsed Time")
	}
	na := m.Options.OneIANA()
	if na == nil {
		t.Fatal("Release carries no IA_NA")
	}
	if addrs := na.Options.Addresses(); len(addrs) != 1 || !addrs[0].IPv6Addr.Equal(v6TestAddr) {
		t.Errorf("Release IA Addresses %v, want %v", addrs, v6TestAddr)
	}
}

// TestRFC9915Section1824RenewOmitsExpiredLeases builds a Renew before and
// after the held address's 30 s valid lifetime runs out.
//
// VALIDATES: a Renew built after the valid lifetime carries no address;
// the control built before it carries the address.
func TestRFC9915Section1824RenewOmitsExpiredLeases(t *testing.T) {
	// RFC requirement: RFC9915-18.2.4-7 negative -- a Renew built 31 s after a lease with valid lifetime 30 s carries no IA Address, while one built at 1 s carries it.
	w := testWorker(t, dHCPConfig{})
	lease := v6TestLease(w)
	for _, tc := range []struct {
		at    time.Time
		addrs int
	}{{time.Unix(1, 0), 1}, {time.Unix(31, 0), 0}} {
		m, err := w.buildFromLease(dhcpv6.MessageTypeRenew, lease, tc.at)
		if err != nil {
			t.Fatal(err)
		}
		na := m.Options.OneIANA()
		if na == nil {
			t.Fatalf("Renew at %v carries no IA_NA", tc.at)
		}
		if got := len(na.Options.Addresses()); got != tc.addrs {
			t.Errorf("Renew at %v carries %d addresses, want %d", tc.at.Unix(), got, tc.addrs)
		}
	}
}

// TestRFC9915Section2122PrefixPreferredAboveValidDiscarded applies an IA_PD
// whose prefix has preferred 40 s above valid 30 s, then a valid one.
//
// VALIDATES: the first prefix is not recorded; the control is.
func TestRFC9915Section2122PrefixPreferredAboveValidDiscarded(t *testing.T) {
	// RFC requirement: RFC9915-21.22-2 negative -- an IA Prefix with preferred lifetime 40 s and valid lifetime 30 s is not recorded, while one with 20 s and 30 s is.
	w := testWorker(t, dHCPConfig{PDRequested: true})
	for _, tc := range []struct {
		preferred time.Duration
		recorded  bool
	}{{40 * time.Second, false}, {20 * time.Second, true}} {
		pd := &dhcpv6.OptIAPD{IaId: w.iaidPD, T1: 10 * time.Second, T2: 16 * time.Second}
		pd.Options.Add(&dhcpv6.OptIAPrefix{
			PreferredLifetime: tc.preferred,
			ValidLifetime:     30 * time.Second,
			Prefix:            &net.IPNet{IP: net.ParseIP("2001:db8:dddd::"), Mask: net.CIDRMask(56, 128)},
		})
		lease := &v6Lease{server: v6ServerA}
		lease.apply(testWorkerAnswer(w, dhcpv6.MessageTypeReply, pd), time.Unix(0, 0), w.iaidNA, w.iaidPD)
		if got := lease.holds(v6KindPrefix); got != tc.recorded {
			t.Errorf("preferred %v valid 30s: prefix recorded %v, want %v", tc.preferred, got, tc.recorded)
		}
	}
}

// TestRFC9915Section141LimitIsPerInterface fills one interface's window and
// asks another interface's client for a transmission.
//
// VALIDATES: the second client admits at once while the first is at its
// limit, so the limit is held per interface.
func TestRFC9915Section141LimitIsPerInterface(t *testing.T) {
	// RFC requirement: RFC9915-14.1-2 positive -- with one interface's client at its 20-message limit, a second interface's client admits its first transmission without waiting.
	full := testWorker(t, dHCPConfig{})
	for i := range v6RateMessages {
		if !full.admitTransmission(context.Background()) {
			t.Fatalf("transmission %d refused", i)
		}
	}
	other := testWorker(t, dHCPConfig{})
	admitted := make(chan time.Time, 1)
	go other.admitAsync(admitted)
	select {
	case <-admitted:
	case <-time.After(2 * time.Second):
		t.Fatal("the other interface waited on the first interface's limit")
	}
}

// TestRFC9915Section12IAIDStable derives the IAID twice from one MAC and
// once from another.
//
// VALIDATES: the IAID depends on the MAC alone, so a restarted client
// derives the same one.
func TestRFC9915Section12IAIDStable(t *testing.T) {
	// RFC requirement: RFC9915-12-2 positive -- two derivations of the IAID from the same MAC are equal, and a different MAC gives a different IAID.
	again := slices.Clone(v6TestMAC)
	if v6IAIDFromMAC(v6TestMAC) != v6IAIDFromMAC(again) {
		t.Error("one MAC gave two IAIDs")
	}
	if got := v6IAIDFromMAC(v6TestMAC); got != v6ClientIAID {
		t.Errorf("IAID %v, want %v", got, v6ClientIAID)
	}
	if v6IAIDFromMAC(net.HardwareAddr{2, 0, 0, 0, 0, 1}) == v6IAIDFromMAC(v6TestMAC) {
		t.Error("two MACs gave one IAID")
	}
}
