// Design: docs/architecture/api/architecture.md -- authenticated bus sender stamping and spoof prevention
// Related: bus_filter.go -- CompiledBus.ResolveAuthenticatedSender

package server

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestBusSenderStampedFromAuthenticatedProcess verifies that an endpoint
// cannot spoof another participant as sender, while a participant-owning
// process may stamp only participants it registered (canonically or via a
// secondary lookup path).
//
// VALIDATES: AC-35, S25.
// PREVENTS: Spoofed message content bypassing receive > from filters.
func TestBusSenderStampedFromAuthenticatedProcess(t *testing.T) {
	registerTestBusMessages(t)
	nsProto := events.LookupNamespaceID(testNSProto)
	etState := events.LookupEventTypeID("state")

	endpointA := BusParticipantInput{
		Path:         "/bus/endpoint/a",
		EndpointName: "a",
		Sends:        []rpc.MessageSendDecl{{Message: testMsgState}},
	}
	endpointB := BusParticipantInput{
		Path:         "/bus/endpoint/b",
		EndpointName: "b",
		Sends:        []rpc.MessageSendDecl{{Message: testMsgState}},
	}
	ownedNode1 := BusParticipantInput{
		Path:           testPathEdge1,
		OwnerEndpoint:  "owner-alpha",
		SecondaryPaths: []string{testPathEdge1IP},
		Sends:          []rpc.MessageSendDecl{{Message: testMsgState}},
	}
	ownedNode2 := BusParticipantInput{
		Path:          testPathLab,
		OwnerEndpoint: "owner-beta",
		Sends:         []rpc.MessageSendDecl{{Message: testMsgState}},
	}
	collector := BusParticipantInput{
		Path:         testPathCollector,
		EndpointName: "collector",
		Receives:     []rpc.MessageReceiveDecl{{Message: testMsgState}},
		OwnBlock: plugin.BusBlock{
			Receive: plugin.BusDirectionBlock{
				TargetsPresent: true,
				Targets:        []string{"/bus/endpoint/a", testPathEdge1},
			},
		},
	}

	bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{
		endpointA,
		endpointB,
		ownedNode1,
		ownedNode2,
		collector,
	})
	if err != nil {
		t.Fatalf("CompileBus: %v", err)
	}

	// Empty authenticated endpoint is refused.
	if _, err := bus.ResolveAuthenticatedSender("", "/bus/endpoint/a"); err == nil {
		t.Fatal("expected error for empty authenticated endpoint, got nil")
	}

	// Endpoint b attempting to spoof /bus/endpoint/a is stamped back to /bus/endpoint/b.
	spoofRes, err := bus.ResolveAuthenticatedSender("b", "/bus/endpoint/a")
	if err != nil {
		t.Fatalf("ResolveAuthenticatedSender spoof: %v", err)
	}
	if !spoofRes.Spoofed || spoofRes.StampedPath != "/bus/endpoint/b" {
		t.Fatalf("spoof result = %+v, want {StampedPath:/bus/endpoint/b Spoofed:true}", spoofRes)
	}
	// Delivering with the stamped path must NOT reach collector (which allows /bus/endpoint/a only).
	if got := bus.Receivers(spoofRes.StampedPath, nsProto, etState, events.DirUnspecified, nil, nil); len(got) != 0 {
		t.Fatalf("collector received spoofed message from b: %v", got)
	}

	// Endpoint a speaking as itself (empty or explicit own path) is not spoofed and reaches collector.
	for _, req := range []string{"", "/bus/endpoint/a"} {
		res, err := bus.ResolveAuthenticatedSender("a", req)
		if err != nil {
			t.Fatalf("ResolveAuthenticatedSender(a, %q): %v", req, err)
		}
		if res.Spoofed || res.StampedPath != "/bus/endpoint/a" {
			t.Fatalf("legit a (%q) = %+v, want {StampedPath:/bus/endpoint/a Spoofed:false}", req, res)
		}
		if got := bus.Receivers(res.StampedPath, nsProto, etState, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"collector"}) {
			t.Fatalf("collector receivers for a = %v, want [collector]", got)
		}
	}

	// Owner process may stamp its own registered child participant by canonical or secondary path.
	for _, req := range []string{testPathEdge1, testPathEdge1IP} {
		res, err := bus.ResolveAuthenticatedSender("owner-alpha", req)
		if err != nil {
			t.Fatalf("ResolveAuthenticatedSender(owner-alpha, %q): %v", req, err)
		}
		if res.Spoofed || res.StampedPath != testPathEdge1 {
			t.Fatalf("owner-alpha (%q) = %+v, want {StampedPath:%s Spoofed:false}", req, res, testPathEdge1)
		}
		if got := bus.Receivers(res.StampedPath, nsProto, etState, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"collector"}) {
			t.Fatalf("collector receivers for owned node = %v, want [collector]", got)
		}
	}

	// Owner process CANNOT stamp a child participant registered by another owner.
	crossOwner, err := bus.ResolveAuthenticatedSender("owner-alpha", testPathLab)
	if err != nil {
		t.Fatalf("ResolveAuthenticatedSender cross-owner: %v", err)
	}
	if !crossOwner.Spoofed || crossOwner.StampedPath != "/bus/endpoint/owner-alpha" {
		t.Fatalf("cross-owner stamp = %+v, want {StampedPath:/bus/endpoint/owner-alpha Spoofed:true}", crossOwner)
	}

	// Ordinary endpoint CANNOT stamp an owned child participant.
	epSpoofOwned, err := bus.ResolveAuthenticatedSender("b", testPathEdge1)
	if err != nil {
		t.Fatalf("ResolveAuthenticatedSender endpoint->owned: %v", err)
	}
	if !epSpoofOwned.Spoofed || epSpoofOwned.StampedPath != "/bus/endpoint/b" {
		t.Fatalf("endpoint spoofing owned node = %+v, want {StampedPath:/bus/endpoint/b Spoofed:true}", epSpoofOwned)
	}
}

// TestSenderIsStampedByTheBus proves S25 end to end: when endpoint b sends a
// message claiming endpoint a as sender, the bus stamps b and filters delivery
// against receive > from [ /bus/endpoint/a ] so collector records nothing.
//
// VALIDATES: AC-35, S25.
// PREVENTS: Receiver from-filters trusting sender fields inside message content.
func TestSenderIsStampedByTheBus(t *testing.T) {
	registerTestBusMessages(t)
	nsAcme := events.LookupNamespaceID(testNSAcme)
	etAlert := events.LookupEventTypeID("alert")

	bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{
		{
			Path:         "/bus/endpoint/a",
			EndpointName: "a",
			Sends:        []rpc.MessageSendDecl{{Message: testMsgAcmeAlert}},
		},
		{
			Path:         "/bus/endpoint/b",
			EndpointName: "b",
			Sends:        []rpc.MessageSendDecl{{Message: testMsgAcmeAlert}},
		},
		{
			Path:         testPathCollector,
			EndpointName: "collector",
			Receives:     []rpc.MessageReceiveDecl{{Message: testMsgAcmeAlert}},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{"/bus/endpoint/a"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CompileBus: %v", err)
	}

	bStamped, err := bus.ResolveAuthenticatedSender("b", "/bus/endpoint/a")
	if err != nil {
		t.Fatalf("ResolveAuthenticatedSender(b): %v", err)
	}
	if !bStamped.Spoofed || bStamped.StampedPath != "/bus/endpoint/b" {
		t.Fatalf("b stamped = %+v, want Spoofed=true StampedPath=/bus/endpoint/b", bStamped)
	}
	if got := bus.Receivers(bStamped.StampedPath, nsAcme, etAlert, events.DirUnspecified, nil, nil); len(got) != 0 {
		t.Fatalf("collector received alert from stamped b: %v", got)
	}

	aStamped, err := bus.ResolveAuthenticatedSender("a", "")
	if err != nil {
		t.Fatalf("ResolveAuthenticatedSender(a): %v", err)
	}
	if aStamped.Spoofed || aStamped.StampedPath != "/bus/endpoint/a" {
		t.Fatalf("a stamped = %+v, want Spoofed=false StampedPath=/bus/endpoint/a", aStamped)
	}
	if got := bus.Receivers(aStamped.StampedPath, nsAcme, etAlert, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"collector"}) {
		t.Fatalf("collector receivers for stamped a = %v, want [collector]", got)
	}
}
