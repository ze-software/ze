// VALIDATES: spec-ospf-ext-16 -- the RFC 4552 installer follows each adjacency. A Hello
// arriving through the engine's real receive path installs an outbound SA keyed on the
// neighbor's link-local before the neighbor state machine runs, the dead-interval expiry
// removes it, and an interface state machine restart removes every neighbor's.
// PREVENTS: the installer's neighbor methods being complete and uncalled, which leaves
// every unicast Database Description, Link State Request, Link State Update and
// acknowledgement this router sends with a policy that resolves no state, so the kernel
// drops it and the adjacency never passes ExStart.

package ospf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// ipsecNeighborLinkLocal is the neighbor's link-local, the source of the Hello it sends and
// the destination of every unicast packet this router answers with.
var ipsecNeighborLinkLocal = netip.MustParseAddr("fe80::2")

// ipsecV6Config is an IPv6-family config with ESP on eth0. deadInterval is the interface
// dead interval in seconds: 1 for a test that waits for the expiry, the default otherwise.
func ipsecV6Config(t *testing.T, deadInterval string) ospfConfig {
	t.Helper()
	leaves := `"area":"0","network-type":"point-to-point"`
	if deadInterval != "" {
		leaves += `,"dead-interval":"` + deadInterval + `"`
	}
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},`+
		`"interfaces":{"interface":{"eth0":{`+leaves+`,"ipsec":{"protocol":"esp","spi":256,"algorithm":"sha256","key":"`+hexKey(32)+`"}}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.V6 == nil {
		t.Fatal("address-family ipv6 was not parsed into cfg.V6")
	}
	return *cfg.V6
}

// newIPsecV6Engine builds the IPv6-family engine the way register_multiaf.go spawns it: the
// real ospfv3 transport over an in-memory backend, and the RFC 4552 installer attached
// before any interface opens, with the kernel replaced by fakeDP. It opens eth0, which
// installs the interface's own states, and returns the engine and the fake.
func newIPsecV6Engine(t *testing.T, cfg ospfConfig) (*engine, *fakeDP) {
	t.Helper()
	fake := &fakeDP{}
	v6transport := ospfv3transport.New(&fakeV6Backend{})
	eng := newEngineWithCodecAF(v6transport, v6Codec{}, afIPv6Unicast)
	inst := newIPsecInstaller(nil, nil)
	inst.dpSource = func() (ipsecDataplane, error) { return fake, nil }
	inst.setTransportSource(v6transport.InterfaceSource)
	eng.installIPsecHooks(inst)
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	t.Cleanup(eng.shutdown)
	if _, ok := eng.ipsec.status("eth0"); !ok {
		t.Fatal("eth0 is not protected after openInterfaces")
	}
	if _, ok := fake.installedTo(ipsecNeighborLinkLocal); ok {
		t.Fatal("a neighbor SA exists before any Hello arrived")
	}
	return eng, fake
}

// helloFromNeighbor feeds one OSPFv3 Hello from the neighbor at fe80::2 through the engine's
// receive path, as the ospfv3 transport would deliver it. deadInterval MUST match the
// interface's, or the interface state machine drops the Hello before any neighbor exists.
func helloFromNeighbor(t *testing.T, eng *engine, deadInterval uint16) {
	t.Helper()
	ifindex := 0
	transport, isV3 := eng.transport.(*ospfv3transport.Transport)
	if !isV3 {
		t.Fatalf("engine transport is %T, not the OSPFv3 transport", eng.transport)
	}
	if _, index, ok := transport.InterfaceSource("eth0"); ok {
		ifindex = index
	}
	if ifindex == 0 {
		t.Fatal("eth0 has no ifindex")
	}
	dispatchHelloV6(t, eng, ifindex, ridOf("10.0.0.2"), types.BackboneArea, ipsecNeighborLinkLocal, ospfv3transport.AllSPFRouters, ospfv3packet.Hello{
		InterfaceID:        2,
		Priority:           1,
		Options:            ospfv3types.OptE | ospfv3types.OptV6 | ospfv3types.OptR,
		HelloInterval:      DefaultHelloInterval,
		RouterDeadInterval: deadInterval,
	})
}

// requireNeighborSA asserts the outbound SA keyed on the neighbor's link-local is installed.
func requireNeighborSA(t *testing.T, fake *fakeDP) {
	t.Helper()
	dir, ok := fake.installedTo(ipsecNeighborLinkLocal)
	if !ok {
		t.Fatalf("no SA installed with Dst %s after the neighbor's Hello", ipsecNeighborLinkLocal)
	}
	if dir != dataplane.SADirOut {
		t.Fatalf("neighbor SA Dir = %v, want out", dir)
	}
}

// waitNeighborSARemoved polls for the removal of the neighbor's SA, which the dead-interval
// expiry performs from the interface state machine's own goroutine.
func waitNeighborSARemoved(t *testing.T, fake *fakeDP) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if fake.removedTo(ipsecNeighborLinkLocal) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("SA with Dst %s was not removed", ipsecNeighborLinkLocal)
}

func TestIPsecNeighborSAFollowsHelloAndDeadInterval(t *testing.T) {
	// RFC 4552 §9: "the routing module must install the corresponding SPD/SAD entries
	// before starting these exchanges". The Hello that creates the neighbor installs its
	// outbound SA, keyed on the Hello's source, before the neighbor state machine can send
	// it a Database Description.
	eng, fake := newIPsecV6Engine(t, ipsecV6Config(t, "1"))
	helloFromNeighbor(t, eng, 1)
	requireNeighborSA(t, fake)

	// The interface's dead interval is 1 second. Nothing else arrives from the neighbor,
	// so the state machine drops it and the SA goes with it.
	waitNeighborSARemoved(t, fake)
	if _, ok := eng.ipsec.status("eth0"); !ok {
		t.Fatal("eth0 lost its own protection with the neighbor")
	}
}

func TestIPsecNeighborSAsClearedOnInterfaceRestart(t *testing.T) {
	cfg := ipsecV6Config(t, "")
	eng, fake := newIPsecV6Engine(t, cfg)
	helloFromNeighbor(t, eng, DefaultDeadInterval)
	requireNeighborSA(t, fake)

	// A changed dead interval restarts the interface state machine, which drops every
	// neighbor as one InterfaceDown while the interface's own protection stays installed.
	eng.reconcile(ipsecV6Config(t, "30"))
	if !fake.removedTo(ipsecNeighborLinkLocal) {
		t.Fatalf("SA with Dst %s survived the interface restart", ipsecNeighborLinkLocal)
	}
	if _, ok := eng.ipsec.status("eth0"); !ok {
		t.Fatal("eth0 lost its own protection on the restart")
	}
	if fake.removedTo(netip.MustParseAddr("fe80::1")) {
		t.Fatal("the interface's own inbound SA was removed by the restart")
	}
}
