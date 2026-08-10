package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestASPACacheAdd verifies adding an ASPA record to the cache.
//
// VALIDATES: AC-1 — ASPA record stored with customer-AS -> provider set.
// PREVENTS: Records silently dropped or provider set incomplete.
func TestASPACacheAdd(t *testing.T) {
	c := newASPACache()
	c.Set(64500, []uint32{100, 200, 300})

	assert.True(t, c.hasRecord(64500))
	assert.True(t, c.isProvider(64500, 100))
	assert.True(t, c.isProvider(64500, 200))
	assert.True(t, c.isProvider(64500, 300))
	assert.False(t, c.isProvider(64500, 400))
	assert.Equal(t, 1, c.count())
}

// TestASPACacheRemove verifies withdrawing an ASPA record.
//
// VALIDATES: AC-5 — withdraw removes the ASPA record entirely.
// PREVENTS: Stale records persisting after withdrawal.
func TestASPACacheRemove(t *testing.T) {
	c := newASPACache()
	c.Set(64500, []uint32{100, 200})
	c.Remove(64500)

	assert.False(t, c.hasRecord(64500))
	assert.False(t, c.isProvider(64500, 100))
	assert.Equal(t, 0, c.count())
}

// TestASPACacheLookup verifies CheckPair returns correct hop results.
//
// VALIDATES: check_pair function per draft-ietf-sidrops-aspa-verification Section 6.
// PREVENTS: Wrong hop classification (Provider+, Not Provider+, No Attestation).
func TestASPACacheLookup(t *testing.T) {
	c := newASPACache()
	c.Set(64500, []uint32{100, 200, 300})

	assert.Equal(t, HopProviderPlus, c.checkPair(100, 64500))
	assert.Equal(t, HopProviderPlus, c.checkPair(200, 64500))
	assert.Equal(t, HopNotProviderPlus, c.checkPair(999, 64500))
	assert.Equal(t, HopNoAttestation, c.checkPair(100, 64501))
}

// TestASPACacheReplace verifies announce replaces the entire provider set.
//
// VALIDATES: RFC 9582 Section 5.12 — announce = full replacement, not delta.
// PREVENTS: Old providers persisting after a new announce.
func TestASPACacheReplace(t *testing.T) {
	c := newASPACache()
	c.Set(64500, []uint32{100, 200, 300})
	c.Set(64500, []uint32{100, 400})

	assert.True(t, c.isProvider(64500, 100))
	assert.False(t, c.isProvider(64500, 200))
	assert.False(t, c.isProvider(64500, 300))
	assert.True(t, c.isProvider(64500, 400))
	assert.Equal(t, 1, c.count())
}

// TestASPACacheApplyDelta verifies atomic delta application.
//
// VALIDATES: AC-5 — End of Data applies ASPA changes atomically.
// PREVENTS: Partial updates visible to concurrent readers.
func TestASPACacheApplyDelta(t *testing.T) {
	c := newASPACache()
	c.Set(64500, []uint32{100, 200})
	c.Set(64501, []uint32{300})

	c.ApplyDelta(
		[]uint32{64500},
		[]ASPARecord{{CustomerAS: 64502, Providers: []uint32{400, 500}}},
	)

	assert.False(t, c.hasRecord(64500))
	assert.True(t, c.hasRecord(64501))
	assert.True(t, c.hasRecord(64502))
	assert.True(t, c.isProvider(64502, 400))
	assert.Equal(t, 2, c.count())
}

// TestASPACacheClear verifies all records removed.
//
// VALIDATES: Clear empties entire cache.
// PREVENTS: Stale records surviving a cache reset.
func TestASPACacheClear(t *testing.T) {
	c := newASPACache()
	c.Set(64500, []uint32{100})
	c.Set(64501, []uint32{200})
	c.clear()

	assert.Equal(t, 0, c.count())
	assert.False(t, c.hasRecord(64500))
}

// TestASPACacheMultipleCustomers verifies independent customer-AS records.
//
// VALIDATES: Records are keyed by customer-AS independently.
// PREVENTS: Cross-contamination between customer records.
func TestASPACacheMultipleCustomers(t *testing.T) {
	c := newASPACache()
	c.Set(64500, []uint32{100, 200})
	c.Set(64501, []uint32{300, 400})

	assert.Equal(t, HopProviderPlus, c.checkPair(100, 64500))
	assert.Equal(t, HopNotProviderPlus, c.checkPair(300, 64500))
	assert.Equal(t, HopProviderPlus, c.checkPair(300, 64501))
	assert.Equal(t, HopNotProviderPlus, c.checkPair(100, 64501))
}

// TestASPACacheBoundaryCustomerAS verifies boundary AS values.
//
// VALIDATES: Boundary tests per spec: valid range 1 to 2^32-2.
// PREVENTS: Off-by-one in AS number handling.
func TestASPACacheBoundaryCustomerAS(t *testing.T) {
	c := newASPACache()

	c.Set(1, []uint32{2})
	assert.True(t, c.hasRecord(1))

	c.Set(0xFFFFFFFE, []uint32{1})
	assert.True(t, c.hasRecord(0xFFFFFFFE))
}

// TestASPACacheChangedCustomers verifies delta change tracking.
//
// VALIDATES: ChangedCustomers returns correct set of affected customer ASNs.
// PREVENTS: Missing re-validation for affected routes.
func TestASPACacheChangedCustomers(t *testing.T) {
	c := newASPACache()

	changed := c.changedCustomers(
		[]uint32{64500, 64501},
		[]ASPARecord{{CustomerAS: 64502, Providers: []uint32{100}}},
	)

	assert.Len(t, changed, 3)
	seen := make(map[uint32]bool)
	for _, v := range changed {
		seen[v] = true
	}
	assert.True(t, seen[64500])
	assert.True(t, seen[64501])
	assert.True(t, seen[64502])
}

// TestASPACacheRemoveNonexistent verifies removing a non-existent record is a no-op.
//
// VALIDATES: Remove on absent key does not panic or corrupt state.
// PREVENTS: Panic on withdrawal for unknown customer-AS.
func TestASPACacheRemoveNonexistent(t *testing.T) {
	c := newASPACache()
	c.Remove(99999)
	assert.Equal(t, 0, c.count())
}

// TestASPAApplyDeltaMostRecent verifies verification uses the most recent ASPA data after a delta.
//
// VALIDATES: ApplyDelta atomically replaces a customer's provider set, and a subsequent verifyASPA
// reads the replaced (latest) data rather than the superseded set.
// PREVENTS: Stale ASPA records driving verification after an incremental cache update.
func TestASPAApplyDeltaMostRecent(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-7-2 positive -- verification uses the
	// most recent ASPA data: after ApplyDelta replaces 300's provider set, verifyASPA reflects the
	// new authorization outcome for the same path.
	c := newASPACache()
	// Initial state: 200 authorizes 100, 300 authorizes 200 -> path is Valid.
	c.ApplyDelta(nil, []ASPARecord{
		{CustomerAS: 200, Providers: []uint32{100}},
		{CustomerAS: 300, Providers: []uint32{200}},
	})
	assert.Equal(t, ASPAValid, verifyASPA(c, []uint32{100, 200, 300}))

	// Newer data replaces 300's provider set so that 200 is no longer authorized.
	c.ApplyDelta(nil, []ASPARecord{
		{CustomerAS: 300, Providers: []uint32{999}},
	})
	// verifyASPA must read the latest data and now classify the same path as Invalid.
	assert.Equal(t, ASPAInvalid, verifyASPA(c, []uint32{100, 200, 300}))
	assert.False(t, c.isProvider(300, 200), "superseded provider must be gone after delta")
}
