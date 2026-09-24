// Design: docs/architecture/ospf/ospf-ext-7-virtual-links.md -- virtual-link database exchange scope.
package ospf

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

type ospfDDRecorder struct {
	mu      sync.Mutex
	packets [][]byte
}

func (r *ospfDDRecorder) SendPacket(_ string, _ netip.Addr, payload []byte) error {
	r.mu.Lock()
	r.packets = append(r.packets, append([]byte(nil), payload...))
	r.mu.Unlock()
	return nil
}

func (r *ospfDDRecorder) headers(t *testing.T) []packet.LSAHeader {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	var headers []packet.LSAHeader
	for _, raw := range r.packets {
		decoded, err := packet.DecodePacket(raw)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.DBDesc != nil && decoded.DBDesc.Flags&packet.DDFlagInit == 0 {
			headers = append(headers, decoded.DBDesc.Headers...)
		}
	}
	return headers
}

func exchangeScopedDD(t *testing.T, virtual bool) []packet.LSAHeader {
	t.Helper()
	eng, backend, result := virtualRouteEngine(t)
	self := eng.cfg.RouterID
	if _, installed, err := eng.lsdb.OriginateExternal(self, [4]byte{203, 0, 113, 0}, [4]byte{255, 255, 255, 0}, types.OptionE, true, 10, [4]byte{}, 0); err != nil || !installed {
		t.Fatalf("originate external: installed=%v error=%v", installed, err)
	}
	for _, scope := range []types.LSType{types.LSTypeOpaqueArea, types.LSTypeOpaqueAS} {
		lsa := packet.LSA{Header: packet.LSAHeader{Type: scope, LinkStateID: packet.OpaqueLinkStateID(99, 1), AdvertisingRouter: self, Sequence: types.InitialSequenceNumber}, Opaque: &packet.OpaqueLSA{Type: scope, Data: []byte{0, 1, 0, 0}}}
		if !eng.lsdb.Install(types.BackboneArea, lsa) {
			t.Fatalf("install opaque scope %d", scope)
		}
	}
	recorder := &ospfDDRecorder{}
	eng.neighbors.SetSender(recorder)
	index := vlIfindex(t, backend, "eth0")
	if virtual {
		dispatchVirtualHello(t, eng, index, result.Neighbor, types.BackboneArea, result.Address, self)
	} else {
		index = vlIfindex(t, backend, "eth1")
		dispatchVirtualHello(t, eng, index, result.Neighbor, types.BackboneArea, result.Address, self)
	}
	dispatchDBDesc(t, eng, index, result.Neighbor, types.BackboneArea, packet.DBDesc{Options: types.OptionE | types.OptionO, Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7})
	return recorder.headers(t)
}

// RFC requirement: RFC5250-3.2-2 positive -- a virtual adjacency's actual transmitted
// Database Description includes area opaque headers but omits both AS-External and
// AS-scope opaque headers installed in the same router's database.
func TestRFC5250VirtualDatabaseExchangeOmitsASScope(t *testing.T) {
	headers := exchangeScopedDD(t, true)
	areaOpaque := false
	for _, header := range headers {
		if header.Type == types.LSTypeASExternal || header.Type == types.LSTypeOpaqueAS {
			t.Fatalf("virtual DD advertises AS scope: %+v", header)
		}
		areaOpaque = areaOpaque || header.Type == types.LSTypeOpaqueArea
	}
	if !areaOpaque {
		t.Fatalf("virtual DD lost permitted area opaque header: %+v", headers)
	}
}

// RFC requirement: RFC5250-3.2-2 negative -- the virtual-link omission does not suppress
// either AS-External or AS-scope opaque headers on an ordinary backbone adjacency.
func TestRFC5250PhysicalDatabaseExchangeRetainsASScope(t *testing.T) {
	headers := exchangeScopedDD(t, false)
	var external, opaque bool
	for _, header := range headers {
		external = external || header.Type == types.LSTypeASExternal
		opaque = opaque || header.Type == types.LSTypeOpaqueAS
	}
	if !external || !opaque {
		t.Fatalf("physical DD missing AS-scoped headers: %+v", headers)
	}
}
