package rescueauth

import (
	"runtime"
	"testing"
)

// argonArenaBytes is the memory argon2id is configured to use: argonMemory is
// expressed in KiB.
const argonArenaBytes = argonMemory * 1024

// maxDeriveBytes is the ceiling this package accepts for one derivation. It is
// the configured arena plus 50% headroom for the scratch and the returned
// digest. Anything beyond that means the parameters grew, or argon2 is
// allocating more than its arena.
const maxDeriveBytes = argonArenaBytes * 3 / 2

// VALIDATES: A-2 -- one rescue-token derivation costs about the configured
// arena and nothing pathological, so the memory an installer initrd must have
// spare at the rescue prompt is a known, bounded number rather than a guess.
// PREVENTS: Raising argonMemory without noticing what it costs on the machine
// that runs it. The rescue prompt executes inside the installer initrd on a box
// that has just FAILED to install, which is the worst moment to discover the
// KDF needs more RAM than the machine has: the operator gets an OOM kill
// instead of a shell, on hardware they may not be standing next to.
//
// This measures the allocation, which is the part that is a property of the
// code. It does NOT prove the initrd has that much free on any particular
// appliance: that is a deployment fact, documented alongside the requirement.
func TestDeriveMemoryIsBounded(t *testing.T) {
	salt := testSalt(t)

	// Settle the heap first so the delta reflects this derivation only.
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	value := Value("memory-probe-token", salt)

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	if value == "" {
		t.Fatal("derivation produced no value")
	}

	used := after.TotalAlloc - before.TotalAlloc
	t.Logf("one derivation allocated %d bytes (arena %d, ceiling %d)", used, argonArenaBytes, maxDeriveBytes)

	if used > maxDeriveBytes {
		t.Errorf("one derivation allocated %d bytes, over the %d ceiling: "+
			"the installer initrd must have that much free at the rescue prompt, "+
			"on a machine that has just failed to install",
			used, maxDeriveBytes)
	}
	// Guard the other direction too: an allocation far below the arena would mean
	// argonMemory is not reaching argon2.IDKey, i.e. the memory hardness the
	// public digest relies on is not actually configured.
	if used < argonArenaBytes/2 {
		t.Errorf("one derivation allocated only %d bytes, well under the %d arena: "+
			"argonMemory does not appear to be in effect",
			used, argonArenaBytes)
	}
}
