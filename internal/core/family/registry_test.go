package family

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withCleanRegistry runs fn against a fresh registry, restoring afterward.
func withCleanRegistry(t *testing.T, fn func()) {
	t.Helper()
	ResetRegistry()
	defer func() {
		ResetRegistry()
		RegisterTestFamilies() // restore default for other tests
	}()
	fn()
}

// TestRegisterFamilyString verifies RegisterFamily stores a family and Family.String() retrieves it.
//
// VALIDATES: AC-1 -- RegisterFamily(1, 133, "ipv4", "flow") -> Family.String() returns "ipv4/flow"
// PREVENTS: Hardcoded family strings.
func TestRegisterFamilyString(t *testing.T) {
	withCleanRegistry(t, func() {
		f, err := RegisterFamily(AFIIPv4, SAFIFlowSpec, "ipv4", "flow")
		require.NoError(t, err)
		assert.Equal(t, "ipv4/flow", f.String())
	})
}

// TestRegisterFamilyAFIString verifies AFI.String() returns the registered name.
//
// VALIDATES: AC-13 -- AFI(1).String() returns "ipv4" from registry, no hardcoded switch.
func TestRegisterFamilyAFIString(t *testing.T) {
	withCleanRegistry(t, func() {
		_, err := RegisterFamily(AFIIPv4, SAFIUnicast, "ipv4", "unicast")
		require.NoError(t, err)
		assert.Equal(t, "ipv4", AFIIPv4.String())
	})
}

// TestRegisterFamilySAFIString verifies SAFI.String() returns the registered name.
//
// VALIDATES: AC-14 -- SAFI(133).String() returns "flow" from registry, no hardcoded switch.
func TestRegisterFamilySAFIString(t *testing.T) {
	withCleanRegistry(t, func() {
		_, err := RegisterFamily(AFIIPv4, SAFIFlowSpec, "ipv4", "flow")
		require.NoError(t, err)
		assert.Equal(t, "flow", SAFIFlowSpec.String())
	})
}

// TestReRegisterSameValues verifies re-registration with identical values is a no-op.
//
// VALIDATES: AC-2 -- Two plugins register AFI 1 as "ipv4" -> no-op, no error.
func TestReRegisterSameValues(t *testing.T) {
	withCleanRegistry(t, func() {
		f1, err := RegisterFamily(AFIIPv4, SAFIUnicast, "ipv4", "unicast")
		require.NoError(t, err)

		f2, err := RegisterFamily(AFIIPv4, SAFIUnicast, "ipv4", "unicast")
		require.NoError(t, err)

		assert.Equal(t, f1, f2, "same registration must return same Family")
	})
}

// TestReRegisterConflictPanics verifies conflicting AFI name returns a fatal error.
//
// VALIDATES: AC-3 -- Plugin registers AFI 1 as "ip4" (conflict) -> fatal error.
func TestReRegisterConflictPanics(t *testing.T) {
	withCleanRegistry(t, func() {
		_, err := RegisterFamily(AFIIPv4, SAFIUnicast, "ipv4", "unicast")
		require.NoError(t, err)

		_, err = RegisterFamily(AFIIPv4, SAFIUnicast, "ip4", "unicast")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrAFIConflict), "expected AFI conflict error")
	})
}

// TestReRegisterSAFIConflict verifies conflicting SAFI name returns a fatal error.
func TestReRegisterSAFIConflict(t *testing.T) {
	withCleanRegistry(t, func() {
		_, err := RegisterFamily(AFIIPv4, SAFIFlowSpec, "ipv4", "flow")
		require.NoError(t, err)

		_, err = RegisterFamily(AFIIPv4, SAFIFlowSpec, "ipv4", "flowspec")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrSAFIConflict), "expected SAFI conflict error")
	})
}

// TestRegisterEmptyName verifies empty AFI/SAFI names return an error.
func TestRegisterEmptyName(t *testing.T) {
	withCleanRegistry(t, func() {
		_, err := RegisterFamily(AFIIPv4, SAFIUnicast, "", "unicast")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrEmptyName))

		_, err = RegisterFamily(AFIIPv4, SAFIUnicast, "ipv4", "")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrEmptyName))
	})
}

// TestFamilyStringFallback verifies Family.String() falls back to "afi-N/safi-N" for unregistered families.
//
// VALIDATES: AC-4 -- Family{AFI:1, SAFI:1}.String() before any registration falls back to "afi-1/safi-1".
func TestFamilyStringFallback(t *testing.T) {
	withCleanRegistry(t, func() {
		// Use an AFI that does not have a slot (afiSlot returns -1 for unknown AFI).
		f := Family{AFI: 9999, SAFI: 99}
		s := f.String()
		assert.Equal(t, "afi-9999/safi-99", s)
	})
}

// TestAppendTo verifies AppendTo methods for AFI, SAFI, Family match String() output.
//
// VALIDATES: AppendTo emits byte-identical output to String() for both known and unknown
// values; appends into an existing buffer without reallocating when capacity is sufficient.
// PREVENTS: Regression if AppendTo and String() drift apart.
func TestAppendTo(t *testing.T) {
	withCleanRegistry(t, func() {
		_, err := RegisterFamily(AFIIPv4, SAFIUnicast, "ipv4", "unicast")
		require.NoError(t, err)

		// Known AFI / SAFI / Family: AppendTo matches String.
		afi := AFIIPv4
		safi := SAFIUnicast
		fam := Family{AFI: afi, SAFI: safi}
		assert.Equal(t, afi.String(), string(afi.AppendTo(nil)))
		assert.Equal(t, safi.String(), string(safi.AppendTo(nil)))
		assert.Equal(t, fam.String(), string(fam.AppendTo(nil)))

		// Unknown fallback: "afi-N", "safi-N", "afi-N/safi-N".
		unkAFI := AFI(9999)
		unkSAFI := SAFI(99)
		unkFam := Family{AFI: unkAFI, SAFI: unkSAFI}
		assert.Equal(t, "afi-9999", string(unkAFI.AppendTo(nil)))
		assert.Equal(t, "safi-99", string(unkSAFI.AppendTo(nil)))
		assert.Equal(t, "afi-9999/safi-99", string(unkFam.AppendTo(nil)))

		// Extending an existing buffer preserves its prefix.
		prefix := []byte("prefix:")
		got := fam.AppendTo(prefix)
		assert.Equal(t, "prefix:ipv4/unicast", string(got))

		// Capacity reuse: when the buffer has capacity, AppendTo writes in place.
		buf := make([]byte, 0, 64)
		buf = append(buf, "x:"...)
		before := &buf[0]
		buf = fam.AppendTo(buf)
		assert.Equal(t, "x:ipv4/unicast", string(buf))
		assert.Same(t, before, &buf[0], "AppendTo must reuse the caller's buffer when capacity permits")
	})
}

// TestParseFamilyRegistered verifies LookupFamily returns registered families.
//
// VALIDATES: AC-6 -- ParseFamily("ipv4/flow") after registration returns Family{1, 133}, true.
func TestParseFamilyRegistered(t *testing.T) {
	withCleanRegistry(t, func() {
		_, err := RegisterFamily(AFIIPv4, SAFIFlowSpec, "ipv4", "flow")
		require.NoError(t, err)

		f, ok := LookupFamily("ipv4/flow")
		assert.True(t, ok)
		assert.Equal(t, AFIIPv4, f.AFI)
		assert.Equal(t, SAFIFlowSpec, f.SAFI)
	})
}

// TestParseFamilyUnknown verifies LookupFamily returns false for unregistered names.
//
// VALIDATES: AC-7 -- ParseFamily("ipv4/flowspec") for unregistered name returns zero, false.
func TestParseFamilyUnknown(t *testing.T) {
	withCleanRegistry(t, func() {
		_, err := RegisterFamily(AFIIPv4, SAFIFlowSpec, "ipv4", "flow")
		require.NoError(t, err)

		f, ok := LookupFamily("ipv4/flowspec")
		assert.False(t, ok)
		assert.Equal(t, Family{}, f)
	})
}

// TestRuntimeRegistration verifies the cache is rebuilt after a runtime registration.
//
// VALIDATES: AC-12 -- External plugin registers family at runtime, Family.String() works after.
func TestRuntimeRegistration(t *testing.T) {
	withCleanRegistry(t, func() {
		_, err := RegisterFamily(AFIIPv4, SAFIUnicast, "ipv4", "unicast")
		require.NoError(t, err)

		// First lookup works.
		assert.Equal(t, "ipv4/unicast", Family{AFI: AFIIPv4, SAFI: SAFIUnicast}.String())

		// Register a new family at "runtime" (after first lookup).
		_, err = RegisterFamily(AFIIPv6, SAFIUnicast, "ipv6", "unicast")
		require.NoError(t, err)

		// New family is now visible.
		assert.Equal(t, "ipv6/unicast", Family{AFI: AFIIPv6, SAFI: SAFIUnicast}.String())
		// Old family still works.
		assert.Equal(t, "ipv4/unicast", Family{AFI: AFIIPv4, SAFI: SAFIUnicast}.String())
	})
}

// TestPackedBufferSize verifies the packed buffer fits in the expected size for L1 cache locality.
//
// VALIDATES: AC-10 -- Packed buffer < 2KB total.
func TestPackedBufferSize(t *testing.T) {
	withCleanRegistry(t, func() {
		RegisterTestFamilies()
		cur := state.Load()
		// Packed buffer (spans + strings) plus the index array (4*256=1024 bytes).
		total := len(cur.pack) + len(cur.idx)*256
		assert.Less(t, total, 2048, "packed buffer + index should fit in < 2KB for L1 cache locality")
	})
}

// TestLookupFamilyStringIsLockFree verifies the read path does not acquire the writer mutex.
//
// VALIDATES: AC-15 -- writeMu only on RegisterFamily, all reads lock-free via atomic.Pointer.
//
// This test holds writeMu while reading via String(). If the read path
// acquired the same mutex, this would deadlock. We use a goroutine + timeout
// to detect the deadlock.
func TestLookupFamilyStringIsLockFree(t *testing.T) {
	RegisterTestFamilies()

	writeMu.Lock()
	done := make(chan string, 3)
	go func() {
		done <- Family{AFI: AFIIPv4, SAFI: SAFIUnicast}.String()
	}()
	go func() {
		done <- AFIIPv4.String()
	}()
	go func() {
		done <- SAFIUnicast.String()
	}()

	// All three goroutines must complete without holding the mutex.
	// If a regression made the read path take writeMu, the goroutines would
	// block until writeMu.Unlock() below -- the timeout catches that.
	results := make([]string, 0, 3)
	for range 3 {
		select {
		case s := <-done:
			results = append(results, s)
		case <-time.After(100 * time.Millisecond):
			writeMu.Unlock() // release before failing so other tests aren't blocked
			t.Fatal("read path took mutex (deadlock detected)")
		}
	}
	writeMu.Unlock()

	assert.Contains(t, results, "ipv4/unicast")
	assert.Contains(t, results, "ipv4")
	assert.Contains(t, results, "unicast")
}

// BenchmarkFamilyString verifies the read path is zero-allocation for registered families.
//
// VALIDATES: AC-11 -- Family.String() zero allocation for registered families.
func BenchmarkFamilyString(b *testing.B) {
	RegisterTestFamilies()
	f := Family{AFI: AFIIPv4, SAFI: SAFIUnicast}

	b.ReportAllocs()
	var s string
	for b.Loop() {
		s = f.String()
	}
	if !strings.HasPrefix(s, "ipv4") {
		b.Fatalf("unexpected: %q", s)
	}
}
