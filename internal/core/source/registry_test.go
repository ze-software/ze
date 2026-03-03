package source

import (
	"net/netip"
	"sync"
	"testing"
)

func TestRegistryRegisterPeer(t *testing.T) {
	// VALIDATES: Peer registration returns sequential IDs in peer range
	// PREVENTS: Wrong ID range, duplicate IDs

	r := NewRegistry()

	ip1 := netip.MustParseAddr("10.0.0.1")
	ip2 := netip.MustParseAddr("10.0.0.2")

	id1 := r.RegisterPeer(ip1, 65001)
	id2 := r.RegisterPeer(ip2, 65002)

	if id1 == InvalidSourceID {
		t.Error("RegisterPeer returned InvalidSourceID")
	}
	if id2 == InvalidSourceID {
		t.Error("RegisterPeer returned InvalidSourceID")
	}
	if id1 == id2 {
		t.Errorf("RegisterPeer returned same ID for different peers: %d", id1)
	}
	if id1.Type() != SourcePeer {
		t.Errorf("Peer ID %d has wrong type: %v", id1, id1.Type())
	}
	if id1 != SourceIDPeerMin {
		t.Errorf("First peer ID = %d, want %d", id1, SourceIDPeerMin)
	}

	// Re-registering same peer should return same ID
	id1Again := r.RegisterPeer(ip1, 65001)
	if id1Again != id1 {
		t.Errorf("Re-registering peer got different ID: %d vs %d", id1Again, id1)
	}
}

func TestRegistryRegisterAPI(t *testing.T) {
	// VALIDATES: API registration returns sequential IDs in API range
	// PREVENTS: Wrong ID range, empty name acceptance

	r := NewRegistry()

	id1 := r.RegisterAPI("rr-plugin")
	id2 := r.RegisterAPI("rib")

	if id1 == InvalidSourceID {
		t.Error("RegisterAPI returned InvalidSourceID")
	}
	if id2 == InvalidSourceID {
		t.Error("RegisterAPI returned InvalidSourceID")
	}
	if id1 == id2 {
		t.Errorf("RegisterAPI returned same ID for different APIs: %d", id1)
	}
	if !id1.IsAPI() {
		t.Errorf("API ID %d has wrong type: %v", id1, id1.Type())
	}
	if id1 != SourceIDAPIMin {
		t.Errorf("First API ID = %d, want %d (100001)", id1, SourceIDAPIMin)
	}

	// Re-registering same API should return same ID
	id1Again := r.RegisterAPI("rr-plugin")
	if id1Again != id1 {
		t.Errorf("Re-registering API got different ID: %d vs %d", id1Again, id1)
	}

	// Empty name should return InvalidSourceID
	emptyID := r.RegisterAPI("")
	if emptyID != InvalidSourceID {
		t.Errorf("RegisterAPI(\"\") = %d, want InvalidSourceID", emptyID)
	}
}

func TestRegistryConfigID(t *testing.T) {
	// VALIDATES: Config source is pre-registered at ID 0
	// PREVENTS: Missing config source

	r := NewRegistry()

	configID := r.ConfigID()
	if configID != SourceIDConfig {
		t.Errorf("ConfigID() = %d, want %d", configID, SourceIDConfig)
	}
	if configID.Type() != SourceConfig {
		t.Errorf("Config ID has wrong type: %v", configID.Type())
	}

	// Config should be active
	if !r.IsActive(configID) {
		t.Error("Config source should be active")
	}

	// Config should be retrievable
	src, ok := r.Get(configID)
	if !ok {
		t.Fatal("Get(ConfigID) returned false")
	}
	if src.Type() != SourceConfig {
		t.Errorf("Config source has wrong type: %v", src.Type())
	}
}

func TestRegistryGet(t *testing.T) {
	// VALIDATES: Get returns correct source for ID
	// PREVENTS: Wrong source lookup

	r := NewRegistry()

	ip := netip.MustParseAddr("10.0.0.1")
	id := r.RegisterPeer(ip, 65001)

	src, ok := r.Get(id)
	if !ok {
		t.Fatal("Get returned false for registered ID")
	}
	if src.Type() != SourcePeer {
		t.Errorf("Get returned wrong type: %v", src.Type())
	}
	if src.PeerIP != ip {
		t.Errorf("Get returned wrong PeerIP: %v", src.PeerIP)
	}
	if src.PeerAS != 65001 {
		t.Errorf("Get returned wrong PeerAS: %v", src.PeerAS)
	}

	// Get invalid ID
	_, ok = r.Get(InvalidSourceID)
	if ok {
		t.Error("Get returned true for InvalidSourceID")
	}
}

func TestRegistryGetByPeerIP(t *testing.T) {
	// VALIDATES: Reverse lookup by peer IP works
	// PREVENTS: Slow O(n) lookups

	r := NewRegistry()

	ip := netip.MustParseAddr("10.0.0.1")
	expectedID := r.RegisterPeer(ip, 65001)

	gotID, ok := r.GetByPeerIP(ip)
	if !ok {
		t.Fatal("GetByPeerIP returned false for registered peer")
	}
	if gotID != expectedID {
		t.Errorf("GetByPeerIP returned wrong ID: %d vs %d", gotID, expectedID)
	}

	// Lookup unknown IP
	_, ok = r.GetByPeerIP(netip.MustParseAddr("192.168.1.1"))
	if ok {
		t.Error("GetByPeerIP returned true for unknown IP")
	}
}

func TestRegistryGetByAPIName(t *testing.T) {
	// VALIDATES: Reverse lookup by API name works
	// PREVENTS: Slow O(n) lookups

	r := NewRegistry()

	expectedID := r.RegisterAPI("rr-plugin")

	gotID, ok := r.GetByAPIName("rr-plugin")
	if !ok {
		t.Fatal("GetByAPIName returned false for registered API")
	}
	if gotID != expectedID {
		t.Errorf("GetByAPIName returned wrong ID: %d vs %d", gotID, expectedID)
	}

	// Lookup unknown name
	_, ok = r.GetByAPIName("unknown")
	if ok {
		t.Error("GetByAPIName returned true for unknown name")
	}
}

func TestRegistryDeactivate(t *testing.T) {
	// VALIDATES: Deactivation marks source inactive but keeps it
	// PREVENTS: Lost history, incorrect active status

	r := NewRegistry()

	ip := netip.MustParseAddr("10.0.0.1")
	id := r.RegisterPeer(ip, 65001)

	// Should be active initially
	if !r.IsActive(id) {
		t.Error("Newly registered peer should be active")
	}

	// Deactivate
	r.Deactivate(id)

	// Should be inactive now
	if r.IsActive(id) {
		t.Error("Deactivated peer should be inactive")
	}

	// Source should still be retrievable
	src, ok := r.Get(id)
	if !ok {
		t.Fatal("Deactivated source should still be retrievable")
	}
	if src.Active {
		t.Error("Deactivated source should have Active=false")
	}
}

func TestRegistryReactivation(t *testing.T) {
	// VALIDATES: Re-registering deactivated source reactivates it
	// PREVENTS: Stuck inactive sources after reconnection

	r := NewRegistry()

	ip := netip.MustParseAddr("10.0.0.1")
	id := r.RegisterPeer(ip, 65001)

	// Deactivate
	r.Deactivate(id)
	if r.IsActive(id) {
		t.Error("Deactivated peer should be inactive")
	}

	// Re-register should reactivate
	id2 := r.RegisterPeer(ip, 65001)
	if id2 != id {
		t.Errorf("Re-registration returned different ID: %d vs %d", id2, id)
	}
	if !r.IsActive(id) {
		t.Error("Re-registered peer should be active")
	}
}

func TestRegistryPeerASUpdate(t *testing.T) {
	// VALIDATES: Re-registering peer updates AS number
	// PREVENTS: Stale AS metadata after config change

	r := NewRegistry()

	ip := netip.MustParseAddr("10.0.0.1")
	id := r.RegisterPeer(ip, 65001)

	// Re-register with different AS
	id2 := r.RegisterPeer(ip, 65002)
	if id2 != id {
		t.Errorf("Re-registration returned different ID: %d vs %d", id2, id)
	}

	src, ok := r.Get(id)
	if !ok {
		t.Fatal("Get returned false for registered ID")
	}
	if src.PeerAS != 65002 {
		t.Errorf("PeerAS not updated: got %d, want 65002", src.PeerAS)
	}
}

func TestRegistryConcurrent(t *testing.T) {
	// VALIDATES: Registry is thread-safe
	// PREVENTS: Race conditions, data corruption

	r := NewRegistry()
	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent peer registrations
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(n int) {
			defer wg.Done()
			ip := netip.AddrFrom4([4]byte{10, 0, byte(n / 256), byte(n % 256)})
			id := r.RegisterPeer(ip, uint32(65000+n)) //nolint:gosec // G115: test data
			if id == InvalidSourceID {
				t.Errorf("Concurrent RegisterPeer returned InvalidSourceID")
			}
			// Read back
			_, ok := r.Get(id)
			if !ok {
				t.Errorf("Concurrent Get failed for id %d", id)
			}
		}(i)
	}
	wg.Wait()

	// Verify count: 100 peers + 1 config
	count := r.Count()
	if count != numGoroutines+1 {
		t.Errorf("Expected %d sources, got %d", numGoroutines+1, count)
	}
}

func TestRegistryString(t *testing.T) {
	// VALIDATES: String() formats source correctly
	// PREVENTS: Incorrect output in JSON/logs

	r := NewRegistry()

	ip := netip.MustParseAddr("10.0.0.1")
	peerID := r.RegisterPeer(ip, 65001)
	apiID := r.RegisterAPI("rr-plugin")

	peerStr := r.String(peerID)
	if peerStr != "peer:10.0.0.1" {
		t.Errorf("String(peerID) = %q, want %q", peerStr, "peer:10.0.0.1")
	}

	apiStr := r.String(apiID)
	if apiStr != "api:rr-plugin" {
		t.Errorf("String(apiID) = %q, want %q", apiStr, "api:rr-plugin")
	}

	configStr := r.String(SourceIDConfig)
	if configStr != "config:1" {
		t.Errorf("String(ConfigID) = %q, want %q", configStr, "config:1")
	}

	// Invalid ID
	unknownStr := r.String(InvalidSourceID)
	if unknownStr != "unknown" {
		t.Errorf("String(InvalidSourceID) = %q, want %q", unknownStr, "unknown")
	}
}

func TestRegistryPeerIDExhaustion(t *testing.T) {
	// VALIDATES: Returns InvalidSourceID when peer space exhausted
	// PREVENTS: ID overflow or panic

	r := NewRegistry()

	// Set nextPeerID near max to test exhaustion
	r.nextPeerID = SourceIDPeerMax

	ip1 := netip.MustParseAddr("10.0.0.1")
	id1 := r.RegisterPeer(ip1, 65001)
	if id1 != SourceIDPeerMax {
		t.Errorf("Expected last valid peer ID %d, got %d", SourceIDPeerMax, id1)
	}

	// Next registration should fail
	ip2 := netip.MustParseAddr("10.0.0.2")
	id2 := r.RegisterPeer(ip2, 65002)
	if id2 != InvalidSourceID {
		t.Errorf("Expected InvalidSourceID after exhaustion, got %d", id2)
	}
}
