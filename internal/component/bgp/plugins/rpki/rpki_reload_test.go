package rpki

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/adj_rib_in"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rpki_decorator"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Exercise the actual Adj-RIB-In receiver and the RPKI reload producer. A
// disabled validator releases its own denial; a rollback reinstates it from
// received bytes, and a queued verdict from the old policy cannot cross back.
func TestRPKIReloadRetainsAndRevalidatesReceivedRoutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	adj := startRPKIReloadReceiver(t, ctx)
	bridge := rpc.NewDirectBridge()
	bridge.SetBatchValidate(rpc.GetBatchValidator())
	bridge.SetEmitEvent(func(_, _, _, _, _ string) (int, error) { return 1, nil })
	bridge.SetDispatchCommandArgs(func(ctx context.Context, command string, args []string, peer string) (*rpc.DispatchCommandOutput, error) {
		out, err := adj.ExecuteCommand(ctx, "reload", command, args, peer)
		if err != nil {
			return nil, err
		}
		return &rpc.DispatchCommandOutput{Status: out.Status, Data: out.Data}, nil
	})
	bridge.SetReady()
	client, engine := net.Pipe()
	plugin := sdk.NewWithConn("bgp-rpki", rpc.NewBridgedConn(client, bridge))
	t.Cleanup(func() { _ = plugin.Close(); _ = engine.Close() })
	rp := &rPKIPlugin{
		plugin: plugin, cache: newROACache(), aspaCache: newASPACache(),
		aspaTracker: newASPATracker(), originTracker: newOriginTracker(),
		validateCh: make(chan validationRequest, 4096), stopCh: make(chan struct{}),
	}
	t.Cleanup(func() {
		close(rp.stopCh)
		rp.stopSessions()
		if rp.dataLease != nil {
			rp.dataLease.stop()
		}
	})
	fixture := newRTRTLSFixture(t)
	peer := startRTRTLSPeer(t, fixture.serverConfig(tls.VersionTLS13))
	accept := &rpkiConfig{
		CacheServers: []cacheServerConfig{{Address: "127.0.0.1", Port: peer.port, Preference: 100,
			TLS: &rtrTLSSettings{CACertificate: "cache-ca", Certificate: "router", ServerName: "cache.rtr.test"}}},
		OriginInvalidAction: ASPAPolicyAccept, OriginNotFoundAction: ASPAPolicyAccept,
		pkiConfig: fixture.store,
	}
	rp.startSessions(accept)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for rp.cache.Validate("192.0.2.0/24", 64500) != ValidationValid {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("authenticated cache did not synchronize")
		}
	}
	rotated := *accept
	rotated.CacheServers = append([]cacheServerConfig(nil), accept.CacheServers...)
	wrongName := *rotated.CacheServers[0].TLS
	wrongName.ServerName = "another-cache.invalid"
	rotated.CacheServers[0].TLS = &wrongName
	if err := rp.replaceConfig(accept, &rotated); err != nil {
		t.Fatal(err)
	}
	if summary := commandJSON(t, rp, "show bgp rpki summary"); summary["validation-enabled"] != true || summary["sessions-synced"] != float64(0) {
		t.Fatalf("transport rotation lost the unexpired data or credited an unauthenticated session: %v", summary)
	}
	if err := rp.replaceConfig(&rotated, accept); err != nil {
		t.Fatal(err)
	}
	// The cache authorizes AS 64500, but this route's real AS_SEQUENCE ends
	// in AS 64501. The prefix is Invalid, not merely pending or absent.
	body := []byte{0, 0, 0, 24, 0x40, 1, 1, 0, 0x40, 2, 10, 2, 2,
		0, 0, 0xfd, 0xe9, 0, 0, 0xfb, 0xf5, 0x40, 3, 4, 192, 0, 2, 1, 24, 192, 0, 2}
	wu := wireu.NewWireUpdate(body, bgpctx.APIContextID)
	attrs, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	if err := adj.DeliverStructured([]any{&rpc.StructuredEvent{
		EventType: rpc.EventKindUpdate, PeerAddress: "192.0.2.1", PeerAS: 65001, LocalAS: 65000,
		PeerName: "customer", PeerGroup: "customers", MessageID: 10,
		RawMessage: &bgptypes.RawMessage{MessageID: 10, RawBytes: body, WireUpdate: wu, AttrsWire: attrs},
	}}); err != nil {
		t.Fatal(err)
	}
	key := ribevents.ValidationRoute{Peer: netip.MustParseAddr("192.0.2.1"), Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	reject := *accept
	reject.OriginInvalidAction = ASPAPolicyReject
	if err := rp.replaceConfig(accept, &reject); err != nil {
		t.Fatal(err)
	}
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationInvalid, hex.EncodeToString(body[4:28]))
	stale := validationRequest{peerAddr: "192.0.2.1", family: "ipv4/unicast", prefix: "192.0.2.0/24",
		msgID: 10, state: ValidationInvalid, aspaState: aspaStateNone, originAS: 64501,
		generation: rp.validationGeneration.Load()}
	disabled := &rpkiConfig{}
	if err := rp.replaceConfig(&reject, disabled); err != nil {
		t.Fatal(err)
	}
	assertRPKIReloadRoute(t, ctx, adj, key, true, ValidationNotValidated, hex.EncodeToString(body[4:28]))
	if err := rp.replaceConfig(disabled, &reject); err != nil {
		t.Fatal(err)
	}
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationInvalid, hex.EncodeToString(body[4:28]))
	// The old worker snapshot had NotFound under an accept policy. It must
	// not reopen the route after rollback reinstates the Invalid rejection.
	stale.state = ValidationNotFound
	rp.dispatchBatch([]validationRequest{stale})
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationInvalid, hex.EncodeToString(body[4:28]))
	if err := rp.replaceConfig(&reject, accept); err != nil {
		t.Fatal(err)
	}
	assertRPKIReloadRoute(t, ctx, adj, key, true, ValidationInvalid, hex.EncodeToString(body[4:28]))
}

func assertRPKIReloadRoute(t *testing.T, ctx context.Context, adj *rpc.DirectBridge, key ribevents.ValidationRoute, eligible bool, state uint8, attributes string) {
	t.Helper()
	out, err := adj.ExecuteCommand(ctx, "observe", "show bgp adj-rib-in", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Peers map[string][]struct {
			Key        string `json:"key"`
			State      uint8  `json:"validation-state"`
			Ineligible bool   `json:"ineligible"`
			Attributes string `json:"attr-hex"`
		} `json:"adj-rib-in"`
	}
	if err := json.Unmarshal(out.Data, &data); err != nil {
		t.Fatal(err)
	}
	for _, route := range data.Peers[key.Peer.String()] {
		if route.Key == "ipv4/unicast:192.0.2.0/24" {
			if route.State != state || route.Ineligible == eligible || route.Attributes != attributes {
				t.Fatalf("retained route state=%d ineligible=%t attrs=%s", route.State, route.Ineligible, route.Attributes)
			}
			if got := ribevents.RouteEligible(key, 10); got != eligible {
				t.Fatalf("selection eligibility=%t, want %t", got, eligible)
			}
			return
		}
	}
	t.Fatal("configuration change discarded the received route")
}

func startRPKIReloadReceiver(t *testing.T, ctx context.Context) *rpc.DirectBridge {
	t.Helper()
	registration := registry.Lookup("bgp-adj-rib-in")
	if registration == nil {
		t.Fatal("Adj-RIB-In registration missing")
	}
	previous := rpc.GetBatchValidator()
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	done := make(chan int, 1)
	go func() { done <- registration.RunEngine(rpc.NewBridgedConn(pluginEnd, bridge)) }()
	t.Cleanup(func() {
		bridge.CloseCallbacks()
		_ = mux.Close()
		_ = pluginEnd.Close()
		_ = engineEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Adj-RIB-In did not stop")
		}
		rpc.RegisterBatchValidator(previous)
	})
	for stage := range 3 {
		select {
		case request := <-mux.Requests():
			if request == nil {
				t.Fatal("startup connection closed")
			}
			if err := mux.SendOK(ctx, request.ID); err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		switch stage {
		case 0:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{}); err != nil {
				t.Fatal(err)
			}
		case 1:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := bridge.SendCallback(ctx, "ze-plugin-callback:post-startup", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	return bridge
}

// MUTATION: retaining previous across verification makes the first pre-apply
// rollback below reopen the denied route and discard the committed credentials.
func TestRPKIConfigCallbacksRollbackBelongsToCurrentTransaction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	adj := startRPKIReloadReceiver(t, ctx)
	initialPKI := rpkiReloadPKISection(t, "initial-router")
	committedPKI := rpkiReloadPKISection(t, "committed-router")
	bridge := startRPKIReloadPlugin(t, ctx, adj, []rpc.ConfigSection{
		{Root: configRootBGP, Data: `{}`}, initialPKI,
	})
	key, attributes := receiveRPKIReloadRoute(t, adj)
	assertRPKIReloadRoute(t, ctx, adj, key, true, ValidationNotValidated, attributes)
	active := rpkiReloadRejectSection(t, ctx)
	verify := func(sections ...rpc.ConfigSection) {
		rpkiReloadCallback(t, ctx, bridge, "config-verify", rpc.ConfigVerifyInput{Sections: sections}, false)
	}
	apply := func() {
		rpkiReloadCallback(t, ctx, bridge, "config-apply", rpc.ConfigApplyInput{}, false)
	}
	rollback := func() {
		rpkiReloadCallback(t, ctx, bridge, "config-rollback", map[string]string{"transaction-id": "reload"}, false)
	}
	verify(active, committedPKI)
	apply()
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationNotFound, attributes)

	// A later transaction verifies, but another participant fails before this
	// plugin applies. Rollback must not undo the already committed generation.
	verify(rpc.ConfigSection{Root: configRootBGP, Data: `{}`}, initialPKI)
	rollback()
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationNotFound, attributes)
	assertRPKIReloadPrivatePKI(t, ctx, bridge, "committed-router", "initial-router")
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationNotFound, attributes)

	// Once this transaction applies, rollback must restore BOTH its own
	// predecessor's validation policy and its private credential namespace.
	verify(rpc.ConfigSection{Root: configRootBGP, Data: `{}`}, initialPKI)
	apply()
	assertRPKIReloadRoute(t, ctx, adj, key, true, ValidationNotValidated, attributes)
	rollback()
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationNotFound, attributes)
	assertRPKIReloadPrivatePKI(t, ctx, bridge, "committed-router", "initial-router")
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationNotFound, attributes)
}

func TestRPKIConfigCallbacksPartialRootsRemovalAndFailedVerify(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	adj := startRPKIReloadReceiver(t, ctx)
	initialPKI := rpkiReloadPKISection(t, "initial-router")
	rotatedPKI := rpkiReloadPKISection(t, "rotated-router")
	bridge := startRPKIReloadPlugin(t, ctx, adj, []rpc.ConfigSection{
		{Root: configRootBGP, Data: `{}`}, initialPKI,
	})
	key, attributes := receiveRPKIReloadRoute(t, adj)
	active := rpkiReloadRejectSection(t, ctx)
	commit := func(sections ...rpc.ConfigSection) {
		rpkiReloadCallback(t, ctx, bridge, "config-verify", rpc.ConfigVerifyInput{Sections: sections}, false)
		rpkiReloadCallback(t, ctx, bridge, "config-apply", rpc.ConfigApplyInput{}, false)
	}

	// A BGP-only delivery must retain PKI; a PKI-only delivery must retain
	// the rejecting BGP policy. Credential resolution uses registered verify.
	commit(active)
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationNotFound, attributes)
	assertRPKIReloadPrivatePKI(t, ctx, bridge, "initial-router", "rotated-router")
	commit(rotatedPKI)
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationNotFound, attributes)
	assertRPKIReloadPrivatePKI(t, ctx, bridge, "rotated-router", "initial-router")

	// An explicitly empty BGP root disables RPKI; omission above did not.
	commit(rpc.ConfigSection{Root: configRootBGP, Data: `{}`})
	assertRPKIReloadRoute(t, ctx, adj, key, true, ValidationNotValidated, attributes)
	assertRPKIReloadPrivatePKI(t, ctx, bridge, "rotated-router", "initial-router")
	commit(active)
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationNotFound, attributes)

	// Plaintext transport permits removing all PKI. Unlike omission, removal
	// must make a subsequent TLS candidate unable to resolve the old identity.
	commit(rpc.ConfigSection{Root: configRootPKI, Data: `{}`})
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationNotFound, attributes)
	rpkiReloadCallback(t, ctx, bridge, "config-verify", rpc.ConfigVerifyInput{
		Sections: []rpc.ConfigSection{rpkiReloadTLSSection("rotated-router")},
	}, true)
	rpkiReloadCallback(t, ctx, bridge, "config-apply", rpc.ConfigApplyInput{}, true)
	rpkiReloadCallback(t, ctx, bridge, "config-rollback", nil, false)
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationNotFound, attributes)
	commit(rotatedPKI)
	assertRPKIReloadPrivatePKI(t, ctx, bridge, "rotated-router", "initial-router")

	// A rejected verification must discard an earlier, still-unapplied
	// candidate, rather than let a later apply silently install that candidate.
	rpkiReloadCallback(t, ctx, bridge, "config-verify", rpc.ConfigVerifyInput{
		Sections: []rpc.ConfigSection{{Root: configRootBGP, Data: `{}`}},
	}, false)
	rpkiReloadCallback(t, ctx, bridge, "config-verify", rpc.ConfigVerifyInput{
		Sections: []rpc.ConfigSection{{Root: configRootBGP, Data: `{`}},
	}, true)
	rpkiReloadCallback(t, ctx, bridge, "config-apply", rpc.ConfigApplyInput{}, true)
	rpkiReloadCallback(t, ctx, bridge, "config-rollback", nil, false)
	assertRPKIReloadRoute(t, ctx, adj, key, false, ValidationNotFound, attributes)
	assertRPKIReloadPrivatePKI(t, ctx, bridge, "rotated-router", "initial-router")
}

func rpkiReloadPKISection(t *testing.T, name string) rpc.ConfigSection {
	t.Helper()
	fixture := newRTRTLSFixture(t)
	fixture.store.Certificates[name] = fixture.store.Certificates["router"]
	delete(fixture.store.Certificates, "router")
	return rpc.ConfigSection{Root: configRootPKI, Data: rtrPKISection(t, fixture.store)}
}

func rpkiReloadTLSSection(identity string) rpc.ConfigSection {
	return rpc.ConfigSection{Root: configRootBGP, Data: fmt.Sprintf(
		`{"bgp":{"rpki":{"cache-server":{"127.0.0.1":{"tls":{"ca-certificate":"cache-ca","certificate":%q,"server-name":"cache.rtr.test"}}}}}}`, identity)}
}

// The verifier consumes the committed private PKI when only BGP is delivered.
// One accepted and one rejected identity distinguish restoration from either
// merging both generations or falling back to a process-wide PKI store.
func assertRPKIReloadPrivatePKI(t *testing.T, ctx context.Context, bridge *rpc.DirectBridge, present, absent string) {
	t.Helper()
	rpkiReloadCallback(t, ctx, bridge, "config-verify", rpc.ConfigVerifyInput{
		Sections: []rpc.ConfigSection{rpkiReloadTLSSection(present)},
	}, false)
	rpkiReloadCallback(t, ctx, bridge, "config-rollback", nil, false)
	rpkiReloadCallback(t, ctx, bridge, "config-verify", rpc.ConfigVerifyInput{
		Sections: []rpc.ConfigSection{rpkiReloadTLSSection(absent)},
	}, true)
	rpkiReloadCallback(t, ctx, bridge, "config-rollback", nil, false)
}

func rpkiReloadRejectSection(t *testing.T, ctx context.Context) rpc.ConfigSection {
	t.Helper()
	// A listening but unanswered cache keeps the VRP set empty, making
	// NotFound policy independent of RTR timing or external infrastructure.
	var listen net.ListenConfig
	listener, err := listen.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is %T", listener.Addr())
	}
	port := tcpAddr.Port
	return rpc.ConfigSection{Root: configRootBGP, Data: fmt.Sprintf(
		`{"bgp":{"rpki":{"cache-server":{"127.0.0.1":{"port":"%d","trusted-network":"true"}},"action":{"not-found":"reject"}}}}`, port)}
}

func receiveRPKIReloadRoute(t *testing.T, adj *rpc.DirectBridge) (ribevents.ValidationRoute, string) {
	t.Helper()
	body := []byte{0, 0, 0, 24, 0x40, 1, 1, 0, 0x40, 2, 10, 2, 2,
		0, 0, 0xfd, 0xe9, 0, 0, 0xfb, 0xf5, 0x40, 3, 4, 192, 0, 2, 1, 24, 192, 0, 2}
	wu := wireu.NewWireUpdate(body, bgpctx.APIContextID)
	attrs, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	if err := adj.DeliverStructured([]any{&rpc.StructuredEvent{
		EventType: rpc.EventKindUpdate, PeerAddress: "192.0.2.1", PeerAS: 65001, LocalAS: 65000,
		MessageID:  10,
		RawMessage: &bgptypes.RawMessage{MessageID: 10, RawBytes: body, WireUpdate: wu, AttrsWire: attrs},
	}}); err != nil {
		t.Fatal(err)
	}
	return ribevents.ValidationRoute{Peer: netip.MustParseAddr("192.0.2.1"), Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("192.0.2.0/24")}, hex.EncodeToString(attrs.Packed())
}

func rpkiReloadCallback(t *testing.T, ctx context.Context, bridge *rpc.DirectBridge, method string, input any, wantError bool) {
	t.Helper()
	params, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := bridge.SendCallback(ctx, "ze-plugin-callback:"+method, params)
	if err != nil {
		t.Fatalf("%s transport failed: %v", method, err)
	}
	if method == "config-rollback" {
		return
	}
	var out rpc.ConfigVerifyOutput
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("%s response: %v", method, err)
	}
	wantStatus := "ok"
	if wantError {
		wantStatus = "error"
	}
	if out.Status != wantStatus {
		t.Fatalf("%s status=%q error=%q, want %q", method, out.Status, out.Error, wantStatus)
	}
}

func startRPKIReloadPlugin(t *testing.T, ctx context.Context, adj *rpc.DirectBridge, sections []rpc.ConfigSection) *rpc.DirectBridge {
	t.Helper()
	registration := registry.Lookup("bgp-rpki")
	if registration == nil {
		t.Fatal("RPKI registration missing")
	}
	bridge := rpc.NewDirectBridge()
	bridge.SetBatchValidate(rpc.GetBatchValidator())
	bridge.SetEmitEvent(func(_, _, _, _, _ string) (int, error) { return 1, nil })
	bridge.SetDispatchCommandArgs(func(ctx context.Context, command string, args []string, peer string) (*rpc.DispatchCommandOutput, error) {
		out, err := adj.ExecuteCommand(ctx, "reload", command, args, peer)
		if err != nil {
			return nil, err
		}
		return &rpc.DispatchCommandOutput{Status: out.Status, Data: out.Data}, nil
	})
	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	done := make(chan int, 1)
	go func() { done <- registration.RunEngine(rpc.NewBridgedConn(pluginEnd, bridge)) }()
	t.Cleanup(func() {
		bridge.CloseCallbacks()
		_ = mux.Close()
		_ = pluginEnd.Close()
		_ = engineEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("RPKI did not stop")
		}
	})
	for stage := range 3 {
		select {
		case request := <-mux.Requests():
			if request == nil {
				t.Fatal("startup connection closed")
			}
			if err := mux.SendOK(ctx, request.ID); err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		switch stage {
		case 0:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{Sections: sections}); err != nil {
				t.Fatal(err)
			}
		case 1:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := bridge.SendCallback(ctx, "ze-plugin-callback:post-startup", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	return bridge
}

// A timeout-only reload must reach the consumer even when every validation
// action and cache endpoint stays unchanged.
func TestRPKIReloadChangesPendingRouteTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	adj := startRPKIReloadReceiver(t, ctx)
	active := rpkiReloadRejectSection(t, ctx)
	bridge := startRPKIReloadPlugin(t, ctx, adj, []rpc.ConfigSection{active})
	var candidate map[string]map[string]map[string]any
	if err := json.Unmarshal([]byte(active.Data), &candidate); err != nil {
		t.Fatal(err)
	}
	candidate["bgp"]["rpki"]["validation-timeout"] = "1"
	raw, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	rpkiReloadCallback(t, ctx, bridge, "config-verify", rpc.ConfigVerifyInput{
		Sections: []rpc.ConfigSection{{Root: configRootBGP, Data: string(raw)}},
	}, false)
	rpkiReloadCallback(t, ctx, bridge, "config-apply", rpc.ConfigApplyInput{}, false)

	// Deliver only to Adj-RIB-In. With no matching validator event, this route
	// must stay pending until the reconfigured deadline releases it fail-open.
	key, attributes := receiveRPKIReloadRoute(t, adj)
	if ribevents.RouteEligible(key, 10) {
		t.Fatal("undecided route bypassed the validation gate")
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for !ribevents.RouteEligible(key, 10) {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("timeout-only reload did not release the pending route before the old 30s deadline")
		}
	}
	assertRPKIReloadRoute(t, ctx, adj, key, true, ValidationNotValidated, attributes)
}

// PKI-only commits and rollback must change the identity and trust anchor used
// on the next real connection, not just the candidate verifier's private copy.
func TestRPKIReloadRotatesAndRestoresTLSIdentity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	adj := startRPKIReloadReceiver(t, ctx)
	initial, rotated := newRTRTLSFixture(t), newRTRTLSFixture(t)
	var active atomic.Pointer[tls.Config]
	active.Store(initial.serverConfig(tls.VersionTLS13))
	server := &tls.Config{
		MinVersion: tls.VersionTLS13,
		GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
			return active.Load(), nil
		},
	}
	peer := startRTRTLSPeer(t, server)
	bgpSection := rpc.ConfigSection{Root: configRootBGP, Data: fmt.Sprintf(
		`{"bgp":{"rpki":{"cache-server":{"127.0.0.1":{"port":"%d","tls":{"ca-certificate":"cache-ca","certificate":"router","server-name":"cache.rtr.test"}}}}}}`, peer.port)}
	bridge := startRPKIReloadPlugin(t, ctx, adj, []rpc.ConfigSection{
		bgpSection, {Root: configRootPKI, Data: rtrPKISection(t, initial.store)},
	})
	waitSynced := func() {
		t.Helper()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			out, err := bridge.ExecuteCommand(ctx, "observe", "show bgp rpki status", nil, "")
			if err != nil {
				t.Fatal(err)
			}
			var status struct {
				Synced int `json:"sessions-synced"`
			}
			if err := json.Unmarshal(out.Data, &status); err != nil {
				t.Fatal(err)
			}
			if status.Synced == 1 {
				return
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				t.Fatal("the committed PKI generation did not complete an authenticated sync")
			}
		}
	}
	waitSynced()
	active.Store(rotated.serverConfig(tls.VersionTLS13))
	rpkiReloadCallback(t, ctx, bridge, "config-verify", rpc.ConfigVerifyInput{
		Sections: []rpc.ConfigSection{{Root: configRootPKI, Data: rtrPKISection(t, rotated.store)}},
	}, false)
	rpkiReloadCallback(t, ctx, bridge, "config-apply", rpc.ConfigApplyInput{}, false)
	waitSynced()
	active.Store(initial.serverConfig(tls.VersionTLS13))
	rpkiReloadCallback(t, ctx, bridge, "config-rollback", nil, false)
	waitSynced()

	observed := peer.stop()
	if len(observed) != 3 {
		t.Fatalf("authenticated connections=%d, want initial, rotated, and rollback", len(observed))
	}
	for i, expected := range []*rtrTLSFixture{initial, rotated, initial} {
		got := observed[i]
		if got.err != nil {
			t.Fatalf("connection %d: %v", i, got.err)
		}
		if len(got.state.PeerCertificates) == 0 {
			t.Fatalf("connection %d did not authenticate a router identity", i)
		}
		if !bytes.Equal(got.state.PeerCertificates[0].Raw, expected.store.Certificates["router"].Raw) {
			t.Errorf("connection %d authenticated a stale router identity", i)
		}
	}
}

// Union consumes one secondary per peer and MsgID. Every producer boundary
// must publish all retained prefixes and families before that consumption.
func TestRPKIUpdatePublicationReachesDecorator(t *testing.T) {
	for _, boundary := range []string{"startup-snapshot", "structured", "json", "roa-change", "aspa-change"} {
		t.Run(boundary, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			rp, bridge := aspaRolePlugin(t, "provider")
			rp.cache.Add(makeVRP("10.0.1.0/24", 24, 300))
			rp.aspaCache.Set(200, []uint32{100})
			rp.aspaCache.Set(300, []uint32{200})

			// One UPDATE carries two legacy IPv4 NLRIs and an IPv6 MP_REACH.
			nextHop := netip.MustParseAddr("2001:db8::1").As16()
			body := aspaMPReachBody(2, 1, nextHop[:], []byte{32, 0x20, 0x01, 0x0d, 0xb8})
			body = append(body, 0x40, 3, 4, 192, 0, 2, 50)
			binary.BigEndian.PutUint16(body[2:4], uint16(len(body)-4)) //nolint:gosec // fixed test attributes fit two bytes
			body = append(body, 24, 10, 0, 1, 24, 10, 0, 2)
			wu := wireu.NewWireUpdate(body, bgpctx.APIContextID)
			attrs, err := wu.Attrs()
			if err != nil {
				t.Fatal(err)
			}
			update := &rpc.StructuredEvent{
				EventType: rpc.EventKindUpdate, PeerAddress: "192.0.2.50",
				PeerName: "ix-192.0.2.50", PeerGroup: "ix", PeerAS: 100, LocalAS: 65000, MessageID: 71,
				RawMessage: &bgptypes.RawMessage{MessageID: 71, RawBytes: body, WireUpdate: wu, AttrsWire: attrs},
			}
			primary := fmt.Sprintf(`{"type":"bgp","bgp":{
				"peer":{"name":"ix-192.0.2.50","group":"ix","remote":{"address":"192.0.2.50","as":100},"local":{"as":65000}},
				"message":{"type":"update","id":71,"direction":"received"},
				"raw":{"attributes":%q},
				"update":{"nlri":{
					"ipv4/unicast":[{"action":"add","next-hop":"192.0.2.50","nlri":["10.0.1.0/24","10.0.2.0/24"]}],
					"ipv6/unicast":[{"action":"add","next-hop":"2001:db8::1","nlri":["2001:db8::/32"]}]
				}}}}`, hex.EncodeToString(attrs.Packed()))

			var adj *rpc.DirectBridge
			switch boundary {
			case "startup-snapshot":
				adj = startRPKIReloadReceiver(t, ctx)
				if err := adj.DeliverStructured([]any{update}); err != nil {
					t.Fatal(err)
				}
				bridge.SetBatchValidate(rpc.GetBatchValidator())
				bridge.SetDispatchCommandArgs(func(ctx context.Context, command string, args []string, peer string) (*rpc.DispatchCommandOutput, error) {
					out, err := adj.ExecuteCommand(ctx, "startup", command, args, peer)
					if err != nil {
						return nil, err
					}
					return &rpc.DispatchCommandOutput{Status: out.Status, Data: out.Data}, nil
				})
			case "roa-change", "aspa-change":
				rp.handleStructuredUpdate(update)
				drainRequests(rp.validateCh)
			}

			decorated := make(chan string, 8)
			decorator := startRPKIPublicationDecorator(t, ctx, decorated)
			if err := decorator.DeliverEvents([]string{primary}); err != nil {
				t.Fatal(err)
			}
			emissions := 0
			bridge.SetEmitEvent(func(_, _, _, _, event string) (int, error) {
				emissions++
				return 1, decorator.DeliverEvents([]string{event})
			})
			origin, aspa := "valid", "valid"
			switch boundary {
			case "startup-snapshot":
				config := oneCache(refusedPort(t), 100)
				config.ASPAValidation = true
				config.PeerActions = *rp.perPeerActions.Load()
				t.Cleanup(func() {
					rp.stopSessions()
					if rp.dataLease != nil {
						rp.dataLease.stop()
					}
				})
				if err := rp.replaceConfig(nil, config); err != nil {
					t.Fatal(err)
				}
			case "structured":
				rp.handleStructuredUpdate(update)
			case "json":
				event, err := bgp.ParseEvent([]byte(primary))
				if err != nil {
					t.Fatal(err)
				}
				rp.handleEvent(event)
			case "roa-change":
				rp.cache.Clear()
				rp.cache.Add(makeVRP("10.0.1.0/24", 24, 999))
				rp.handleROAChange()
				origin = "invalid"
			case "aspa-change":
				rp.aspaCache.Set(300, []uint32{999})
				rp.handleASPAChange([]uint32{300})
				aspa = "invalid"
			}
			if emissions != 1 {
				t.Fatalf("published %d secondary events for one UPDATE, want one", emissions)
			}
			select {
			case raw := <-decorated:
				var event struct {
					BGP struct {
						Message struct {
							ID   uint64 `json:"id"`
							Type string `json:"type"`
						} `json:"message"`
						RPKI struct {
							IPv4 map[string]string `json:"ipv4/unicast"`
							IPv6 map[string]string `json:"ipv6/unicast"`
							ASPA string            `json:"aspa-state"`
						} `json:"rpki"`
					} `json:"bgp"`
				}
				if err := json.Unmarshal([]byte(raw), &event); err != nil {
					t.Fatal(err)
				}
				got := event.BGP
				if got.Message.ID != 71 || got.Message.Type != "update-rpki" ||
					got.RPKI.IPv4["10.0.1.0/24"] != origin || got.RPKI.IPv4["10.0.2.0/24"] != "not-found" ||
					got.RPKI.IPv6["2001:db8::/32"] != "not-found" || got.RPKI.ASPA != aspa {
					t.Fatalf("decorator lost a correlated verdict: %s", raw)
				}
			case <-ctx.Done():
				t.Fatal("decorator did not deliver the correlated UPDATE")
			}
		})
	}
}

func startRPKIPublicationDecorator(t *testing.T, ctx context.Context, events chan<- string) *rpc.DirectBridge {
	t.Helper()
	registration := registry.Lookup("bgp-rpki-decorator")
	if registration == nil {
		t.Fatal("RPKI decorator registration missing")
	}
	bridge := rpc.NewDirectBridge()
	bridge.SetEmitEvent(func(_, _, _, _, event string) (int, error) {
		events <- event
		return 1, nil
	})
	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	done := make(chan int, 1)
	go func() { done <- registration.RunEngine(rpc.NewBridgedConn(pluginEnd, bridge)) }()
	t.Cleanup(func() {
		bridge.CloseCallbacks()
		_ = mux.Close()
		_ = pluginEnd.Close()
		_ = engineEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("RPKI decorator did not stop")
		}
	})
	for stage := range 3 {
		select {
		case request := <-mux.Requests():
			if request == nil {
				t.Fatal("decorator startup connection closed")
			}
			if err := mux.SendOK(ctx, request.ID); err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		switch stage {
		case 0:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{}); err != nil {
				t.Fatal(err)
			}
		case 1:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := bridge.SendCallback(ctx, "ze-plugin-callback:post-startup", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	return bridge
}
