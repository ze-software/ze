package policyview

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
)

// sessionPeer is a route-server client on an eBGP session, in a group.
func sessionPeer() filterapi.PeerFilterInfo {
	return filterapi.PeerFilterInfo{
		Address:        netip.MustParseAddr("192.0.2.1"),
		PeerAS:         65001,
		LocalAS:        65000,
		Name:           "edge",
		GroupName:      "clients",
		RSClient:       true,
		RouteServerAS:  64999,
		LLGRAdvertised: true,
	}
}

// TestSessionFactsReadAndWritten proves every session fact field reads the
// fact the reactor filled and reaches the filter-update fields object, so an
// in-process member and a forked filter see the same value.
//
// VALIDATES: spec-filter-wire-6-stage-filters phase 2 (session facts, D-7):
// peer-name, peer-group, peer-address, local-as, peer-as, route-server-as,
// ibgp, rs-client, llgr-advertised on import; the source peer's facts on
// export.
// PREVENTS: a filter matching on a fact the view never carried.
func TestSessionFactsReadAndWritten(t *testing.T) {
	source := filterapi.PeerFilterInfo{
		Address:   netip.MustParseAddr("2001:db8::7"),
		PeerAS:    65007,
		Name:      "upstream",
		GroupName: "transit",
		IsIBGP:    true,
	}
	tests := []struct {
		name    string
		session Session
		fields  []string
		want    string
	}{
		{
			name:    "import facts of the sending peer",
			session: ImportSession(sessionPeer()),
			fields: []string{"peer-name", "peer-group", "peer-address", "local-as", "peer-as",
				"route-server-as", "ibgp", "rs-client", "llgr-advertised"},
			want: `{"peer-name":"edge","peer-group":"clients","peer-address":"192.0.2.1","local-as":65000,` +
				`"peer-as":65001,"route-server-as":64999,"ibgp":false,"rs-client":true,"llgr-advertised":true}`,
		},
		{
			name:    "export facts of the destination and of the route's source",
			session: ExportSession(sessionPeer(), source, true),
			fields: []string{"peer-name", "llgr-advertised", "source-peer-name", "source-peer-group",
				"source-peer-address", "source-peer-as", "source-ibgp", "source-rs-client"},
			want: `{"peer-name":"edge","source-peer-name":"upstream","source-peer-group":"transit",` +
				`"source-peer-address":"2001:db8::7","source-peer-as":65007,"source-ibgp":true,"source-rs-client":false}`,
		},
		{
			name:    "export of an originated route has no source",
			session: ExportSession(sessionPeer(), filterapi.PeerFilterInfo{}, false),
			fields:  []string{"source-peer-name", "source-ibgp", "rs-client"},
			want:    `{"rs-client":true}`,
		},
		{
			name:    "no session facts known",
			session: Session{EBGP: true},
			fields:  []string{"peer-name", "ibgp", "rs-client", "source-peer-as"},
			want:    `{}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var view View
			view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), tt.session)
			got, err := AppendFields(nil, &view, tt.fields)
			if err != nil {
				t.Fatalf("AppendFields: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("fields\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

// TestSessionFactAbsentIsNotZero proves a fact the session does not hold is
// absent, never a zero: a standalone peer has no group, a session that is
// not a route-server client has no route-server-as.
//
// VALIDATES: ai/rules/principles.md, a zero is never an answer.
// PREVENTS: a member matching route-server-as 0 or peer-group "".
func TestSessionFactAbsentIsNotZero(t *testing.T) {
	peer := sessionPeer()
	peer.GroupName = ""
	peer.RSClient = false
	peer.RouteServerAS = 0
	var view View
	view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), ImportSession(peer))
	for _, field := range []string{"peer-group", "route-server-as"} {
		reader, err := ReaderFor(sessionField(t, field))
		if err != nil {
			t.Fatal(err)
		}
		value, err := view.Read(reader)
		if err != nil {
			t.Fatal(err)
		}
		if value.Present() {
			t.Errorf("%s is present on a session that holds none", field)
		}
	}
}

// sessionField answers the registered session fact field named name.
func sessionField(t *testing.T, name string) policy.Field {
	t.Helper()
	for _, fact := range sessionFacts {
		if fact.field.String() == name {
			return fact.field
		}
	}
	t.Fatalf("no session fact field %s", name)
	return policy.Field{}
}

// TestSourceLLGRAdvertisedOnRebuiltRoute proves source-llgr-advertised
// answers the source session's LLGR fact on a route rebuilt from storage, the
// one export rail that fills it (the relay's resolveRelaySource), and is
// absent everywhere else rather than a false standing in for an unknown: a
// live export (no rebuilt metadata), metadata saying not rebuilt, a route
// with no known source, and a received route.
//
// VALIDATES: spec-filter-wire-6-stage-filters phase 6 (the LLGR egress step's
// read of the source's llgr-advertised, R-5), RFC 9494 Section 4.3.
// PREVENTS: an egress step reading false on a live rail and treating a
// stale route from an LLGR session as fresh.
func TestSourceLLGRAdvertisedOnRebuiltRoute(t *testing.T) {
	source := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("192.0.2.9"), PeerAS: 65009, LLGRAdvertised: true}
	silent := source
	silent.LLGRAdvertised = false
	rebuilt := map[string]any{filterapi.MetaKeyRebuilt: true}
	tests := []struct {
		name    string
		session Session
		want    string
	}{
		{"rebuilt route, source advertised LLGR", ExportSession(sessionPeer(), source, true).WithExportMetadata(rebuilt),
			`{"source-llgr-advertised":true}`},
		{"rebuilt route, source did not", ExportSession(sessionPeer(), silent, true).WithExportMetadata(rebuilt),
			`{"source-llgr-advertised":false}`},
		{"live export", ExportSession(sessionPeer(), source, true), `{}`},
		{"metadata says not rebuilt", ExportSession(sessionPeer(), source, true).
			WithExportMetadata(map[string]any{filterapi.MetaKeyRebuilt: false}), `{}`},
		{"no known source", ExportSession(sessionPeer(), source, false).WithExportMetadata(rebuilt), `{}`},
		{"received route", ImportSession(sessionPeer()).WithImportMetadata(rebuilt), `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var view View
			view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), tt.session)
			got, err := AppendFields(nil, &view, []string{"source-llgr-advertised"})
			if err != nil {
				t.Fatalf("AppendFields: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("fields\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}
