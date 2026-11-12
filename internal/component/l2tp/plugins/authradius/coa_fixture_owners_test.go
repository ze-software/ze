package l2tpauthradius

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/textbuf"
	cosplugin "github.com/ze-software/ze/internal/plugins/cos"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// coaFixtureMaps implements the interface operations the real CoS owner uses.
// Embedding the remaining backend methods makes an unexpected operation fail
// immediately, rather than silently succeed in a fixture.
type coaFixtureMaps struct {
	iface.Backend
	mu         sync.Mutex
	links      map[string]*iface.InterfaceInfo
	failWrites int
}

func (b *coaFixtureMaps) GetLink(name string) (*iface.InterfaceInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	link, found := b.links[name]
	if !found {
		link = &iface.InterfaceInfo{Name: name, OsName: name, Type: "vlan"}
		b.links[name] = link
	}
	copy := *link
	copy.IngressQoSMap = maps.Clone(link.IngressQoSMap)
	copy.EgressQoSMap = maps.Clone(link.EgressQoSMap)
	return &copy, nil
}

func (b *coaFixtureMaps) GetInterface(name string) (*iface.InterfaceInfo, error) {
	return b.GetLink(name)
}

func (b *coaFixtureMaps) UpdateVLANQoSMap(name string, ingress, egress map[uint32]uint32) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.links[name] = &iface.InterfaceInfo{Name: name, OsName: name, Type: "vlan",
		IngressQoSMap: maps.Clone(ingress), EgressQoSMap: maps.Clone(egress)}
	if b.failWrites > 0 {
		b.failWrites--
		return errors.New("test VLAN backend failed after writing the new maps")
	}
	return nil
}

func (*coaFixtureMaps) Close() error { return nil }

type coaFixtureOwners struct {
	bus         *coaApplyBus
	rate        *rpc.DirectBridge
	cos         *rpc.DirectBridge
	rateBackend *coaApplyBackend
	mapsBackend *coaFixtureMaps
}

var coaFixtureOwnersByTest sync.Map
var coaFixtureMapRegistration sync.Once
var coaFixtureMapsCurrent *coaFixtureMaps

func coaFixtureInterface(tunnel, session uint16) string {
	var name textbuf.Buffer
	return name.Str("ppp-coa-").Uint(uint64(tunnel)).Byte('-').Uint(uint64(session)).String()
}

// coaFixtureDispatch starts the registered owners once per test and seeds their
// actual successful backend state from the service the test publishes. The
// listener's recording bus remains an observer of committed notifications.
func coaFixtureDispatch(t *testing.T) coaCommandDispatch {
	t.Helper()
	value, found := coaFixtureOwnersByTest.Load(t)
	if !found {
		bus, rateBackend, rateBridge := startCoARateOwner(t)
		backend := &coaFixtureMaps{links: make(map[string]*iface.InterfaceInfo)}
		coaFixtureMapsCurrent = backend
		coaFixtureMapRegistration.Do(func() {
			if err := iface.RegisterBackend("test-coa-maps", func() (iface.Backend, error) { return coaFixtureMapsCurrent, nil }); err != nil {
				t.Fatal(err)
			}
		})
		if err := iface.LoadBackend("test-coa-maps"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = iface.CloseBackend() })
		reg := registry.Lookup(cosplugin.Name)
		if reg == nil {
			t.Fatal("CoS owner is not registered")
		}
		reg.ConfigureEventBus(bus)
		_, cosBridge := startAuthProvider(t, cosplugin.Name, "")
		previous := coreCos.All()
		coreCos.Replace(map[string]coreCos.Profile{"gold": {IngressMap: map[uint32]uint32{1: 5}, EgressMap: map[uint32]uint32{5: 1}}})
		t.Cleanup(func() { coreCos.Replace(previous) })
		owners := &coaFixtureOwners{bus: bus, rate: rateBridge, cos: cosBridge, rateBackend: rateBackend, mapsBackend: backend}
		coaFixtureOwnersByTest.Store(t, owners)
		t.Cleanup(func() { coaFixtureOwnersByTest.Delete(t) })
		value = owners
	}
	owners := value.(*coaFixtureOwners)
	if service := l2tp.LookupService(); service != nil {
		snapshot := service.Snapshot()
		for i := range snapshot.Tunnels {
			for j := range snapshot.Tunnels[i].Sessions {
				session := &snapshot.Tunnels[i].Sessions[j]
				name := session.PppInterface
				if name == "" {
					name = coaFixtureInterface(snapshot.Tunnels[i].LocalTID, session.LocalSID)
				}
				if _, err := l2tpevents.SessionUp.Emit(owners.bus, &l2tpevents.SessionUpPayload{
					TunnelID: snapshot.Tunnels[i].LocalTID, SessionID: session.LocalSID, Interface: name,
				}); err != nil {
					t.Fatal(err)
				}
				tunnelID, sessionID := snapshot.Tunnels[i].LocalTID, session.LocalSID
				t.Cleanup(func() {
					if _, err := l2tpevents.SessionDown.Emit(owners.bus, &l2tpevents.SessionDownPayload{TunnelID: tunnelID, SessionID: sessionID}); err != nil {
						t.Error(err)
					}
				})
			}
		}
	}
	return func(ctx context.Context, command string, args []string, peer string) (string, json.RawMessage, error) {
		bridge := owners.rate
		if command == "request class-of-service session" {
			bridge = owners.cos
		}
		out, err := bridge.ExecuteCommand(ctx, "coa-fixture", command, args, peer)
		if err != nil {
			return "error", nil, err
		}
		return out.Status, out.Data, nil
	}
}
