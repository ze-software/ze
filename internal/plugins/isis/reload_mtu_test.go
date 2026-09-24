// Design: docs/guide/isis.md -- node-wide capabilities and live configuration.
// Design: docs/architecture/wire/isis.md -- final authenticated LSP size.

package isis

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

func TestISISPassiveCapabilityReloadUpdatesLSP(t *testing.T) {
	const active = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"eth0":{"level":"l1","hello-interval":"3600"}}}}}`
	const passiveV6 = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"eth0":{"level":"l1","hello-interval":"3600"},"lo":{"passive":"true","address-family":{"ipv6-unicast":{}}}}}}}`
	eng, backend := protocolEngine(t, active)
	id := types.NewLSPID(types.NewSourceID(eng.cfg.SystemID, 0), 0)
	for _, step := range []struct {
		config string
		want   []byte
	}{
		{passiveV6, []byte{packet.NLPIDIPv4, packet.NLPIDIPv6}},
		{active, []byte{packet.NLPIDIPv4}},
	} {
		reconcileTo(t, eng, step.config)
		if err := protocolLiveCircuit(t, eng, "eth0").SendHello(adjacency.Level1); err != nil {
			t.Fatal(err)
		}
		if got := protocolLastHello(t, protocolCaptureFor(t, backend, "eth0")); !bytes.Equal(got, step.want) {
			t.Fatalf("Hello protocols %x, want %x", got, step.want)
		}
		entry := eng.lsdb.Lookup(lsdb.Level1, id)
		if entry == nil {
			t.Fatal("reload omitted the local fragment zero")
		}
		pdu, err := packet.DecodePDU(entry.Raw())
		if err != nil {
			t.Fatal(err)
		}
		got := protocolTLV(t, pdu.LSP.TLVs)
		packet.ReleaseTLVs(pdu.LSP.TLVs)
		if !bytes.Equal(got, step.want) {
			t.Fatalf("reload left LSP protocols %x, want %x", got, step.want)
		}
	}
}

func TestISISFragmentedLSPFitsFinalFrame(t *testing.T) {
	for _, authenticated := range []bool{false, true} {
		name := "unsigned"
		if authenticated {
			name = "authenticated"
		}
		t.Run(name, func(t *testing.T) {
			cfg, err := parseISISConfig(sec(protocolP2PConfig))
			if err != nil {
				t.Fatal(err)
			}
			cfg.Level = LevelL1
			cfg.Hostname = "r"
			cfg.Interfaces[0].Name = "isis-mtu-test"
			key := packet.Key{Algorithm: packet.AuthAlgoHMACSHA256, KeyID: 1, Secret: []byte("fragment-secret")}
			if authenticated {
				cfg.Level1AuthKeyChain = "area"
				cfg.KeyChains = []KeyChainConfig{{Name: "area", Keys: []KeyConfig{{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "fragment-secret"}}}}
			}
			backend := &protocolBackend{circuits: make(map[string]*protocolCapture)}
			eng := newEngine(transport.New(backend))
			eng.setConfig(cfg)
			t.Cleanup(eng.shutdown)
			if err := eng.openCircuits(); err != nil {
				t.Fatal(err)
			}
			prefixes := make([]lsdb.PrefixInfo, 400)
			remaining := make(map[netip.Prefix]bool, len(prefixes))
			for i := range prefixes {
				prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 64, byte(i >> 8), byte(i)}), 32)
				prefixes[i] = lsdb.PrefixInfo{Prefix: prefix, Metric: types.NewPrefixMetric(7)}
				remaining[prefix] = true
			}
			eng.setPrefixes(lsdb.Level1, prefixes)
			eng.originate()
			raw := eng.lsdb.RawSnapshot(lsdb.Level1)
			if len(raw) < 2 {
				t.Fatal("prefix set did not exercise LSP fragmentation")
			}
			for _, wire := range raw {
				if err := eng.transport.SendPDU(cfg.Interfaces[0].Name, transport.Level1, wire); err != nil {
					t.Fatalf("final LSP (%d bytes plus LLC) could not be sent: %v", len(wire), err)
				}
				if authenticated {
					if err := packet.VerifyPDU(wire, []packet.Key{key}); err != nil {
						t.Fatalf("fragment lost authentication: %v", err)
					}
				}
				pdu, err := packet.DecodePDU(wire)
				if err != nil {
					t.Fatal(err)
				}
				for _, tlv := range pdu.LSP.TLVs {
					if tlv.Type != packet.TLVExtendedIPReach {
						continue
					}
					reach, err := packet.DecodeExtendedIPReachTLV(tlv.Value)
					if err != nil {
						t.Fatal(err)
					}
					for _, entry := range reach.Entries {
						delete(remaining, entry.Prefix)
					}
				}
				packet.ReleaseTLVs(pdu.LSP.TLVs)
			}
			if len(remaining) != 0 {
				t.Fatalf("fragmentation dropped %d advertised prefixes", len(remaining))
			}
		})
	}
}

func TestISISReloadActivatesIdleEngine(t *testing.T) {
	backend := &protocolBackend{circuits: make(map[string]*protocolCapture)}
	eng := newEngine(transport.New(backend))
	t.Cleanup(eng.shutdown)
	bus := &bgplsTestBus{handlers: make(map[[2]string]map[int]func(any))}
	eng.setEventSink(newEventSink(bus))
	up := make(chan SessionEvent, 1)
	unsubscribe := SessionUp.Subscribe(bus, func(event *SessionEvent) {
		select {
		case up <- *event:
		default:
		}
	})
	defer unsubscribe()

	// No initial NET means OnStarted leaves the engine idle. A later commit
	// must activate actual transport delivery, not only the Hello sender.
	eng.setConfig(Config{})
	reconcileTo(t, eng, protocolP2PConfig)
	capture := protocolCaptureFor(t, backend, "eth0")
	capture.recv <- transport.RawFrame{
		IfIndex: capture.IfIndex(),
		PDU:     protocolPeerIIH(t, []byte{packet.NLPIDIPv4}),
	}
	select {
	case event := <-up:
		if event.NeighborID != "0000.0000.0002" || event.Level != "l1" || event.State != adjacency.StateUp.String() {
			t.Fatalf("delivered Hello produced unexpected session: %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("configuration activated a circuit but did not deliver its received Hello")
	}
}
