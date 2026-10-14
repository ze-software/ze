package verify

import (
	"errors"
	"testing"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// A verification refuses before it starts when the host cannot run a
// functional stage its piece holds, and a piece that holds none starts on that
// host. The method cuts the full population into one stage per piece, so each
// piece's content is known, and counts the calls to a fake host check that
// always refuses.
func TestHostPreflightChecksOnlyAPieceThatRunsAFunctionalStage(t *testing.T) {
	missing := errors.New("this host is missing loopback addresses")
	calls := 0
	host := func() error {
		calls++
		return missing
	}

	if err := hostPreflight(verifyengine.Mode, verifyengine.Uncut(), host); !errors.Is(err, missing) {
		t.Fatalf("the uncut population started on a host without its addresses: %v", err)
	}
	if calls != 1 {
		t.Fatalf("the host was checked %d times for one run", calls)
	}

	population := verifyengine.StagesForMode(verifyengine.Mode)
	pieces := len(population)
	for index := range population {
		calls = 0
		part := verifyengine.Part{Index: index + 1, Of: pieces}
		err := hostPreflight(verifyengine.Mode, part, host)
		functional := population[index].Identity.Command == verifyengine.FunctionalCommand
		if functional && !errors.Is(err, missing) {
			t.Errorf("piece %d holds %s and started on a host without its addresses: %v",
				index+1, population[index].Identity.Name, err)
		}
		if !functional && (err != nil || calls != 0) {
			t.Errorf("piece %d holds %s and was checked %d times, err %v",
				index+1, population[index].Identity.Name, calls, err)
		}
	}
}

// A mode that names no population is refused, never answered "the host is
// fine": nothing was checked, so nothing may start.
func TestHostPreflightRefusesAnUnknownMode(t *testing.T) {
	host := func() error { return nil }
	if err := hostPreflight("no-such-mode", verifyengine.Uncut(), host); err == nil {
		t.Fatal("an unknown mode passed the preflight")
	}
}
