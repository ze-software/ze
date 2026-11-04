// Design: docs/architecture/meta/role.md -- source export policy observes roleless destinations.
package role

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRolelessDestinationOpenDrivesExportSet proves a roleless destination's
// actual OPEN, not a test-seeded learned role, decides the source's export set.
// Method: configure through the transaction, observe Customer, accept its route,
// change only the source export set, then reject it; a reconnect without Role
// records explicit absence rather than keeping the previous Customer value.
func TestRolelessDestinationOpenDrivesExportSet(t *testing.T) {
	clearFilterState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(rolelessPeerSections(true, "first")); err != nil {
		t.Fatal(err)
	}
	installed := installedRoleConfig()
	if installed.nameToIP["plain"] != "192.0.2.3" {
		t.Fatal("roleless destination must share the configured name-to-address index")
	}
	input := openWithRole("plain", 3)
	input.Generation = 4
	if out := applyValidateOpen(installed.peers, installed.nameToIP, input); !out.Accept {
		t.Fatalf("roleless destination must not undergo role-pair validation: %+v", out)
	}
	if !remoteRoleRecorded("192.0.2.3", "plain") {
		t.Fatal("the destination OPEN was not observed")
	}
	payload := buildTestPayload(buildTestAttrs(0), []byte{24, 10, 1, 0})
	fields := `{"peer-name":"plain","peer-address":"192.0.2.3","local-as":64512,` +
		`"source-peer-name":"edge","source-peer-address":"192.0.2.1"}`
	out, err := evaluateRoleFilter(otcForkedRoute(otcEgressFilterName, payload, fields))
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != sdk.FilterAccept || len(out.Sets) != 0 {
		t.Fatalf("Customer allowed by source export set, without RFC stamping: %+v", out)
	}
	changed := rolelessPeerSections(true, "second")
	changed[0].Data = strings.ReplaceAll(changed[0].Data, `"export":["customer"]`, `"export":["provider"]`)
	commitRoleReload(t, tx, changed)
	out, err = evaluateRoleFilter(otcForkedRoute(otcEgressFilterName, payload, fields))
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != sdk.FilterReject {
		t.Fatalf("Customer excluded by source export set: %+v", out)
	}
	installed = installedRoleConfig()
	applyValidateOpen(installed.peers, installed.nameToIP, openWithoutRole("plain"))
	_, learned := getFilterConfig("192.0.2.3", "plain", "")
	if learned != "" || !remoteRoleRecorded("192.0.2.3", "plain") {
		t.Fatalf("reconnect must record explicit role absence, learned %q", learned)
	}
}

// TestRolelessDestinationObservationDoesNotEnableRFCGates proves a remote Role
// alone cannot enable RFC 9234 Section 5 on a locally unconfigured destination.
// Method: observe Provider and Customer OPENs, allow each in the source's export
// set, and pass both marked and unmarked routes through the real evaluator.
func TestRolelessDestinationObservationDoesNotEnableRFCGates(t *testing.T) {
	for _, tc := range []struct {
		name string
		wire uint8
	}{
		{name: "provider", wire: 0},
		{name: "customer", wire: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearFilterState(t)
			tx := newRoleConfigTx()
			sections := rolelessPeerSections(true, "first")
			sections[0].Data = strings.ReplaceAll(sections[0].Data, `"export":["customer"]`, `"export":["`+tc.name+`"]`)
			if err := tx.configure(sections); err != nil {
				t.Fatal(err)
			}
			installed := installedRoleConfig()
			if out := applyValidateOpen(installed.peers, installed.nameToIP, openWithRole("plain", tc.wire)); !out.Accept {
				t.Fatalf("unconfigured local role rejected OPEN: %+v", out)
			}
			for _, otc := range []uint32{0, 65009} {
				payload := buildTestPayload(buildTestAttrs(otc), []byte{24, 10, 1, 0})
				out, err := evaluateRoleFilter(otcForkedRoute(otcEgressFilterName, payload,
					`{"peer-name":"plain","peer-address":"192.0.2.3","local-as":64512,"src-role":"customer",`+
						`"source-peer-name":"edge","source-peer-address":"192.0.2.1"}`))
				if err != nil {
					t.Fatal(err)
				}
				if out.Action != sdk.FilterAccept || len(out.Sets) != 0 {
					t.Fatalf("remote %s alone activated an RFC gate for OTC %d: %+v", tc.name, otc, out)
				}
			}
		})
	}
}

// TestRolelessDestinationObservationOwnsItsLifecycle proves observation is
// bounded by configured peers and dynamic groups and uses existing removal
// generations. Method: reject unowned identities, observe a configured dynamic
// member, preserve its newer OPEN across an old removal, free it on removal,
// then observe a changed role while inactive and enforce it after reactivation.
func TestRolelessDestinationObservationOwnsItsLifecycle(t *testing.T) {
	clearFilterState(t)
	tx := newRoleConfigTx()
	sections := rolelessPeerSections(true, "first")
	sections[0].Data = strings.ReplaceAll(sections[0].Data, `{"bgp":{`, `{"bgp":{"group":{"plain-dynamic":{"connection":{"remote":{"ip":"dynamic","range":"198.51.100.0/24"}}}},`)
	if err := tx.configure(sections); err != nil {
		t.Fatal(err)
	}
	installed := installedRoleConfig()
	for _, group := range []string{"", "unowned"} {
		input := openWithRole("unowned-peer", 3)
		input.Group = group
		applyValidateOpen(installed.peers, installed.nameToIP, input)
		if remoteRoleRecorded("", input.Peer) {
			t.Fatalf("recorded an OPEN outside the configured census, group %q", group)
		}
	}
	input := openWithRole("dyn-198.51.100.9", 3)
	input.Group = "plain-dynamic"
	input.Generation = 5
	applyValidateOpen(installed.peers, installed.nameToIP, input)
	if !remoteRoleRecorded("198.51.100.9", input.Peer) {
		t.Fatal("configured dynamic member was not observed")
	}
	remove := &rpc.StructuredEvent{PeerName: input.Peer, PeerAddress: "198.51.100.9",
		EventType: rpc.EventKindState, State: rpc.SessionStateDown,
		Reason: rpc.ReasonPeerRemoved, SessionGeneration: 5}
	if err := handleRoleStructuredEvents([]any{remove}); err != nil {
		t.Fatal(err)
	}
	if !remoteRoleRecorded("198.51.100.9", input.Peer) {
		t.Fatal("old removal deleted a newer OPEN observation")
	}
	remove.SessionGeneration = 6
	if err := handleRoleStructuredEvents([]any{remove}); err != nil {
		t.Fatal(err)
	}
	if remoteRoleRecorded("198.51.100.9", input.Peer) {
		t.Fatal("current removal retained the observation")
	}
	input.Generation = 7
	applyValidateOpen(installed.peers, installed.nameToIP, input)
	inactive := []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{"group":{"plain-dynamic":{"connection":{"remote":{"ip":"dynamic","range":"198.51.100.0/24"}}}}}}`}}
	commitRoleReload(t, tx, inactive)
	if !remoteRoleRecorded("198.51.100.9", input.Peer) {
		t.Fatal("deactivating source policy dropped a live destination observation")
	}
	installed = installedRoleConfig()
	changed := openWithRole(input.Peer, 0) // Provider, not Customer.
	changed.Group = input.Group
	changed.Generation = 8
	applyValidateOpen(installed.peers, installed.nameToIP, changed)
	commitRoleReload(t, tx, sections)
	out, err := evaluateRoleFilter(otcForkedRoute(otcEgressFilterName,
		buildTestPayload(buildTestAttrs(0), []byte{24, 10, 1, 0}),
		`{"peer-name":"dyn-198.51.100.9","peer-address":"198.51.100.9","peer-group":"plain-dynamic",`+
			`"local-as":64512,"source-peer-name":"edge","source-peer-address":"192.0.2.1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != sdk.FilterReject {
		t.Fatalf("reactivated Customer-only export set used a stale dynamic role: %+v", out)
	}
	remove.SessionGeneration = 9
	if err := handleRoleStructuredEvents([]any{remove}); err != nil {
		t.Fatal(err)
	}
	if remoteRoleRecorded("198.51.100.9", input.Peer) {
		t.Fatal("newer removal did not free an observation retained across inactive policy")
	}
}

// TestRolelessDestinationOpenBeforePolicyActivation proves source-policy
// activation reads the live destination's earlier OPEN, and a later inactive
// reconnect cannot leave the old advertised role behind. Method: real config
// transactions and OPEN callbacks, followed by the consumer's egress verdict.
func TestRolelessDestinationOpenBeforePolicyActivation(t *testing.T) {
	clearFilterState(t)
	tx := newRoleConfigTx()
	inactive := []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{"peer":{
		"edge":{"connection":{"remote":{"ip":"192.0.2.1"}}},
		"plain":{"connection":{"remote":{"ip":"192.0.2.3"}}}
	}}}`}}
	if err := tx.configure(inactive); err != nil {
		t.Fatal(err)
	}
	installed := installedRoleConfig()
	if out := applyValidateOpen(installed.peers, installed.nameToIP, openWithRole("plain", 3)); !out.Accept {
		t.Fatalf("initial roleless OPEN rejected: %+v", out)
	}
	active := rolelessPeerSections(true, "active")
	commitRoleReload(t, tx, active)
	payload := buildTestPayload(buildTestAttrs(0), []byte{24, 10, 1, 0})
	fields := `{"peer-name":"plain","peer-address":"192.0.2.3","local-as":64512,` +
		`"source-peer-name":"edge","source-peer-address":"192.0.2.1"}`
	out, err := evaluateRoleFilter(otcForkedRoute(otcEgressFilterName, payload, fields))
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != sdk.FilterAccept || len(out.Sets) != 0 {
		t.Fatalf("activating source policy lost the existing Customer OPEN: %+v", out)
	}
	commitRoleReload(t, tx, inactive)
	installed = installedRoleConfig()
	applyValidateOpen(installed.peers, installed.nameToIP, openWithRole("plain", 0))
	commitRoleReload(t, tx, active)
	out, err = evaluateRoleFilter(otcForkedRoute(otcEgressFilterName, payload, fields))
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != sdk.FilterReject {
		t.Fatalf("Provider reconnect during inactive policy retained stale Customer permission: %+v", out)
	}
}
