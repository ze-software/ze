// Design: docs/research/vpp-deployment-reference.md -- VPP interface naming and LCP TAP mapping

package ifacevpp

import (
	"maps"
	"sync"
)

// nameMap maintains a bidirectional mapping between ze short names (xe0, loop0)
// and VPP software interface indices (SwIfIndex). VPP's long names
// (TenGigabitEthernet3/0/0) are stored for display but lookup is by SwIfIndex.
type nameMap struct {
	mu       sync.RWMutex
	toIndex  map[string]uint32 // ze name -> SwIfIndex
	toName   map[uint32]string // SwIfIndex -> ze name
	vppNames map[uint32]string // SwIfIndex -> VPP long name
}

func newNameMap() *nameMap {
	return &nameMap{
		toIndex:  make(map[string]uint32),
		toName:   make(map[uint32]string),
		vppNames: make(map[uint32]string),
	}
}

// Add registers a ze name to SwIfIndex mapping.
func (m *nameMap) Add(zeName string, swIfIndex uint32, vppName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toIndex[zeName] = swIfIndex
	m.toName[swIfIndex] = zeName
	m.vppNames[swIfIndex] = vppName
}

// Remove deletes a mapping by ze name.
func (m *nameMap) Remove(zeName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx, ok := m.toIndex[zeName]
	if !ok {
		return
	}
	delete(m.toIndex, zeName)
	delete(m.toName, idx)
	delete(m.vppNames, idx)
}

// LookupIndex returns the SwIfIndex for a ze name, or false if not found.
func (m *nameMap) LookupIndex(zeName string) (uint32, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	idx, ok := m.toIndex[zeName]
	return idx, ok
}

// LookupName returns the ze name for a SwIfIndex, or false if not found.
func (m *nameMap) LookupName(swIfIndex uint32) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	name, ok := m.toName[swIfIndex]
	return name, ok
}

// LookupVPPName returns the VPP long name for a SwIfIndex.
func (m *nameMap) LookupVPPName(swIfIndex uint32) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	name, ok := m.vppNames[swIfIndex]
	return name, ok
}

// All returns all ze name to SwIfIndex mappings.
func (m *nameMap) All() map[string]uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]uint32, len(m.toIndex))
	maps.Copy(result, m.toIndex)
	return result
}

// Len returns the number of mapped interfaces.
func (m *nameMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.toIndex)
}
