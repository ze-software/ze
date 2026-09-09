//go:build integration && linux

// Design: docs/architecture/core-design.md -- selection, and the FIB write that follows it
// Related: ../../../component/sysrib/fibimport.go -- fibPermitted, the gate this test drives
// Related: ../../../component/sysrib/sysrib.go -- recomputeBest, which reads that gate
// Related: integration_linux_test.go -- withNetNS, addLoopback, zeRoutes, newTestBackend
//
// The kernel's own answer to `rib { fib-withhold [ bgp ] }`. Every other test of
// that feature reads a Ze map or a Ze event, so each of them would still pass
// with the netlink path broken. This one runs the real system RIB and the real
// FIB writer in one process and then reads the forwarding table.

package fibkernel

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/component/sysrib" // registers the "rib" plugin this test runs
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

const (
	// The owner's own example: BGP withheld from the FIB, OSPF kept.
	withholdPrefixBGP  = "10.98.0.0/24"
	withholdPrefixOSPF = "10.97.0.0/24"

	// The loopback next-hop every other integration test in this file's package
	// uses. The kernel refuses a route whose next-hop it cannot reach, and the
	// absence this test asserts would then say nothing about the gate.
	withholdNextHop = "127.0.0.1"

	// sysribRoot is the system RIB's registration name and its config root.
	sysribRoot = "rib"

	// withholdWait bounds one startup stage and one command. The whole chain is
	// in-process, so this is margin for a loaded machine, not a wait on a peer.
	withholdWait = 20 * time.Second
)

// withholdConfigText is the operator's configuration. It carries both distances
// because the two prefixes must be won by their own protocols: withholding
// decides what is PROGRAMMED and never what is SELECTED.
const withholdConfigText = `rib {
	distance {
		ebgp 20
		ospf 110
	}
	fib-withhold [ bgp ]
}
`

// TestFIBWithholdLeavesNoKernelRoute proves the withhold decision reaches the
// forwarding table. With `fib-withhold [ bgp ]` configured, the kernel carries
// the OSPF prefix and carries nothing for the BGP one, and the system RIB still
// reports BGP as the winner of the prefix it withheld.
//
// Method: the real system-RIB engine runs in this process over an RPC pipe this
// test drives as the daemon's engine drives it, configured with the section the
// real config chain produces. The real FIB writer runs beside it on a netlink
// handle bound to this test's network namespace. Two paths then enter the
// Loc-RIB the way a protocol producer enters them, and the assertions read the
// kernel through netlink and the system RIB through the plugin's own command.
//
// MUTATION: comment out the fibPermitted gate in recomputeBest
// (internal/component/sysrib/sysrib.go) and the BGP prefix is programmed, so the
// second assertion goes red.
func TestFIBWithholdLeavesNoKernelRoute(t *testing.T) {
	withNetNS(t, func() {
		handle, err := netlink.NewHandle()
		if err != nil {
			t.Fatalf("netlink handle: %v", err)
		}
		defer handle.Close()

		addLoopback(t, handle)

		// The names the two producers register: internal/component/bgp/plugins/rib
		// (rib.go, bgpProtocolID) and internal/plugins/ospf/spf (install.go,
		// ospfProtocolID). RegisterProtocol is idempotent by name, so this test
		// registers them itself rather than linking both producers into the test
		// binary. It has to register them: parseFIBImportConfig builds its
		// permission set over the REGISTERED protocols, so an unregistered "bgp"
		// would take fibPermitted's fail-open branch and hide the gate.
		bgpProtocol := redistevents.RegisterProtocol("bgp")
		ospfProtocol := redistevents.RegisterProtocol("ospf")

		ctx, cancel := context.WithTimeout(context.Background(), withholdWait)
		defer cancel()

		bus := newWithholdBus()

		// The FIB writer, on the namespace-scoped handle. run() (fibkernel.go)
		// makes exactly this subscription. It is not called here because it also
		// starts the routewatch monitor, whose netlink socket would open in
		// whatever namespace its goroutine's thread holds, and withNetNS moves
		// the calling thread alone.
		//
		// The bus stays installed for the rest of the binary, because
		// setEventBus refuses a nil and there is nothing to restore it to. It
		// costs the siblings nothing: the only other publisher in this package
		// is handleExternalChange, and this bus has no subscriber for the event
		// it emits.
		setEventBus(bus)
		writer := newFIBKernel(newTestBackend(handle))
		unsubscribe := sysribevents.BestChange.Subscribe(bus, writer.processEvent)
		defer unsubscribe()

		engine := startSysRIB(ctx, t, bus, withholdSection(t))

		// Two paths enter the Loc-RIB the way a producer enters them, each with
		// its own protocol's distance. BGP is inserted FIRST, so the system RIB
		// has already decided about it by the time the OSPF route appears in the
		// kernel: one worker reads the change channel in order.
		routes := locrib.Default()
		routes.Insert(family.IPv4Unicast, netip.MustParsePrefix(withholdPrefixBGP), locrib.Path{
			Source:        bgpProtocol,
			NextHop:       netip.MustParseAddr(withholdNextHop),
			AdminDistance: 20,
			Metric:        10,
		})
		routes.Insert(family.IPv4Unicast, netip.MustParsePrefix(withholdPrefixOSPF), locrib.Path{
			Source:        ospfProtocol,
			NextHop:       netip.MustParseAddr(withholdNextHop),
			AdminDistance: 110,
			Metric:        10,
		})

		// Assertion 3, and it comes first: the protocol nobody withheld reaches
		// the kernel. It proves the chain from a Loc-RIB insert to netlink works
		// in this environment, so the absence asserted next is the withhold
		// rather than a chain that programmed nothing.
		if !withholdProgrammed(t, handle, withholdPrefixOSPF) {
			t.Fatalf("the kept protocol never programmed %s; ze routes: %v",
				withholdPrefixOSPF, withholdKernelPrefixes(t, handle))
		}

		// Assertion 1: the withheld protocol's prefix is in no kernel route.
		if programmed := withholdKernelPrefixes(t, handle); programmed[withholdPrefixBGP] {
			t.Errorf("the withheld protocol programmed %s; ze routes: %v",
				withholdPrefixBGP, programmed)
		}

		// Assertion 2: the withheld route is still SELECTED. The surface read is
		// the system RIB's own `show rib`, over the plugin connection this test
		// already holds, because that is the answer an operator gets and it is
		// sysrib's state rather than the Loc-RIB the paths were inserted into.
		if winner := withholdRIBWinner(ctx, t, engine, withholdPrefixBGP); winner != "bgp" {
			t.Errorf("system RIB winner for %s is %q, want \"bgp\": a withheld route is still a selected route",
				withholdPrefixBGP, winner)
		}
	})
}

// withholdSection builds the config section the system RIB receives, by driving
// the real producer chain rather than typing the JSON (the reason is written out
// in config_delivery_test.go, deliverSection): every leaf arrives as a string,
// and the section arrives wrapped in its root.
func withholdSection(t *testing.T) rpc.ConfigSection {
	t.Helper()

	result, err := zeconfig.LoadConfig(withholdConfigText, "fib-withhold-test.conf", nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	subtree := zeconfig.ExtractConfigSubtree(result.Tree.ToPluginMap(), sysribRoot)
	if subtree == nil {
		t.Fatalf("ExtractConfigSubtree(%q) returned nil, so the plugin would be handed {}", sysribRoot)
	}
	data, err := json.Marshal(subtree)
	if err != nil {
		t.Fatalf("marshal the %s section: %v", sysribRoot, err)
	}
	t.Logf("delivered section for %s: %s", sysribRoot, data)
	return rpc.ConfigSection{Root: sysribRoot, Data: string(data)}
}

// startSysRIB runs the real system-RIB engine in this process and answers with
// the engine side of its connection, so the caller can put a command to it.
//
// The five stages below are the engine's half of the SDK startup protocol
// (pkg/plugin/sdk/sdk.go, Plugin.Run). Stage 2 is the one this test needs: it
// delivers the `rib` section, which is where parseFIBImportConfig reads
// fib-withhold. The plugin's OnStarted handler runs after stage 5, so the engine
// is selecting routes by the time this returns.
func startSysRIB(ctx context.Context, t *testing.T, bus ze.EventBus, section rpc.ConfigSection) *rpc.MuxConn {
	t.Helper()

	registration := registry.Lookup(sysribRoot)
	if registration == nil {
		t.Fatalf("plugin %q is not registered", sysribRoot)
	}
	registration.ConfigureEventBus(bus)

	pluginSide, engineSide := net.Pipe()
	engine := rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))

	engineDone := make(chan int, 1)
	go func() { engineDone <- registration.RunEngine(pluginSide) }()

	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close the engine connection: %v", err)
		}
		select {
		case <-engineDone:
		case <-time.After(withholdWait):
			t.Error("the system RIB engine did not return after its connection closed")
		}
	})

	awaitCall := func(method string) {
		t.Helper()
		select {
		case request := <-engine.Requests():
			if request.Method != method {
				t.Fatalf("startup: the plugin called %q, want %q", request.Method, method)
			}
			if err := engine.SendOK(ctx, request.ID); err != nil {
				t.Fatalf("startup: answer %s: %v", method, err)
			}
		case <-time.After(withholdWait):
			t.Fatalf("startup: timed out waiting for %s", method)
		}
	}

	awaitCall(rpc.MethodDeclareRegistration)
	if _, err := engine.CallRPC(ctx, "ze-plugin-callback:configure",
		rpc.ConfigureInput{Sections: []rpc.ConfigSection{section}}); err != nil {
		t.Fatalf("startup: configure: %v", err)
	}
	awaitCall("ze-plugin-engine:declare-capabilities")
	if _, err := engine.CallRPC(ctx, "ze-plugin-callback:share-registry",
		rpc.ShareRegistryInput{}); err != nil {
		t.Fatalf("startup: share-registry: %v", err)
	}
	awaitCall("ze-plugin-engine:ready")

	return engine
}

// withholdRIBWinner answers which protocol the system RIB reports as the winner
// of the prefix, and the empty string when the RIB holds no entry for it.
func withholdRIBWinner(ctx context.Context, t *testing.T, engine *rpc.MuxConn, prefix string) string {
	t.Helper()

	answer, err := engine.CallAnswer(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Serial: "1", Command: "show rib"})
	if err != nil {
		t.Fatalf("show rib: %v", err)
	}

	// One document answer: the head, one record carrying the whole reply, the
	// terminator (pkg/plugin/sdk/sdk_callbacks.go, executeCommandAnswer).
	var document json.RawMessage
	for record := range answer.Records {
		document = record.Item
	}
	if answerErr := answer.Err(); answerErr != nil {
		t.Fatalf("show rib answer: %v", answerErr)
	}

	var entries []struct {
		Prefix   string `json:"prefix"`
		Protocol string `json:"protocol"`
	}
	if err := json.Unmarshal(document, &entries); err != nil {
		t.Fatalf("decode show rib %s: %v", document, err)
	}
	for _, entry := range entries {
		if entry.Prefix == prefix {
			return entry.Protocol
		}
	}
	return ""
}

// withholdProgrammed reports whether the kernel holds a Ze-owned route for the
// prefix, polling because the chain crosses two plugin goroutines.
func withholdProgrammed(t *testing.T, handle *netlink.Handle, prefix string) bool {
	t.Helper()

	// 100 attempts at 100ms. The whole chain is in-process, so this is margin
	// for a loaded machine rather than a wait on a network event.
	for attempt := 0; attempt < 100; attempt++ {
		if withholdKernelPrefixes(t, handle)[prefix] {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// withholdKernelPrefixes reads the destination of every route Ze's FIB writer
// owns in this namespace, so a route the kernel made itself is never counted.
func withholdKernelPrefixes(t *testing.T, handle *netlink.Handle) map[string]bool {
	t.Helper()

	prefixes := map[string]bool{}
	for _, route := range zeRoutes(t, handle) {
		if route.Dst == nil {
			continue
		}
		prefixes[route.Dst.String()] = true
	}
	return prefixes
}

var _ ze.EventBus = (*withholdBus)(nil)

// withholdBus is the in-process event bus the system RIB and the FIB writer
// share. The daemon's bus also serves external plugin processes, and none of
// that is reachable from a package test; what this one keeps is the property
// both plugins depend on, which is synchronous delivery to every in-process
// handler.
//
// Safe for concurrent use.
type withholdBus struct {
	mu       sync.Mutex
	nextID   uint64
	handlers map[string]map[uint64]func(any)
}

func newWithholdBus() *withholdBus {
	return &withholdBus{handlers: map[string]map[uint64]func(any){}}
}

func (b *withholdBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	subscribers := make([]func(any), 0, len(b.handlers[namespace+"/"+eventType]))
	for _, handler := range b.handlers[namespace+"/"+eventType] {
		subscribers = append(subscribers, handler)
	}
	b.mu.Unlock()

	for _, handler := range subscribers {
		handler(payload)
	}

	// The count is the number of external plugin PROCESSES that received the
	// event, and this bus has none.
	return 0, nil
}

func (b *withholdBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType

	b.mu.Lock()
	b.nextID++
	id := b.nextID
	if b.handlers[key] == nil {
		b.handlers[key] = map[uint64]func(any){}
	}
	b.handlers[key][id] = handler
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.handlers[key], id)
	}
}
