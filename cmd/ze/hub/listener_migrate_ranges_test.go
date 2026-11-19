package hub

import (
	"errors"
	"net/netip"
	"testing"
)

// fakeRangesHolder records what the migrator handed it, as the looking glass
// would hold it, and refuses when told to.
type fakeRangesHolder struct {
	ranges  map[netip.Prefix]string
	calls   int
	refusal error
}

func (f *fakeRangesHolder) UpdateNextHopRanges(ranges map[netip.Prefix]string) (restore func(), err error) {
	f.calls++
	if f.refusal != nil {
		return nil, f.refusal
	}
	previous := f.ranges
	f.ranges = ranges
	return func() { f.ranges = previous }, nil
}

// TestUpdateLGNextHopRangesReplacesTheList asks whether a reload hands the
// running looking glass the reloaded list whole, through the seam that names
// no lg type (AC-13, R-6): after the second call the holder has the second
// list only, and a list with no entry clears it.
//
// PREVENTS: names read once at start, so that a reload changes the list and
// the page keeps the old one.
func TestUpdateLGNextHopRangesReplacesTheList(t *testing.T) {
	lan := netip.MustParsePrefix("198.51.100.0/24")
	holder := &fakeRangesHolder{}
	migrator := &listenerMigrator{}
	migrator.setLGNextHopRanges(holder)

	if _, err := migrator.updateLGNextHopRanges(map[netip.Prefix]string{lan: "ixp-lan-a"}); err != nil {
		t.Fatalf("first update: %v", err)
	}
	if _, err := migrator.updateLGNextHopRanges(map[netip.Prefix]string{lan: "ixp-lan-b"}); err != nil {
		t.Fatalf("second update: %v", err)
	}
	if len(holder.ranges) != 1 {
		t.Fatalf("holder has %v, want the one renamed range", holder.ranges)
	}
	if holder.ranges[lan] != "ixp-lan-b" {
		t.Errorf("range %s is named %q after the reload, want ixp-lan-b", lan, holder.ranges[lan])
	}

	if _, err := migrator.updateLGNextHopRanges(nil); err != nil {
		t.Fatalf("update with no range: %v", err)
	}
	if len(holder.ranges) != 0 {
		t.Errorf("holder has %v after a reload that names no range, want none", holder.ranges)
	}
	if holder.calls != 3 {
		t.Errorf("holder was called %d times, want 3", holder.calls)
	}
}

// TestUpdateLGNextHopRangesWithoutLookingGlass asks what the reload does when
// no looking glass runs: nothing, and no error.
func TestUpdateLGNextHopRangesWithoutLookingGlass(t *testing.T) {
	migrator := &listenerMigrator{}
	ranges := map[netip.Prefix]string{netip.MustParsePrefix("198.51.100.0/24"): "ixp-lan-a"}
	if _, err := migrator.updateLGNextHopRanges(ranges); err != nil {
		t.Errorf("update with no looking glass: %v, want no error", err)
	}
}

// TestUpdateLGNextHopRangesRestoresOnRollback asks whether the call the
// migrator answers puts back the list the reload replaced, which is what
// runReloadContext calls when a later step of the reload fails (a candidate
// whose promotion is refused), and whether a migrator with no looking glass
// still answers a call that is safe to run.
//
// PREVENTS: a rolled-back reload leaving the looking glass with the names of
// the configuration that was refused.
func TestUpdateLGNextHopRangesRestoresOnRollback(t *testing.T) {
	lan := netip.MustParsePrefix("198.51.100.0/24")
	holder := &fakeRangesHolder{}
	migrator := &listenerMigrator{}
	migrator.setLGNextHopRanges(holder)

	if _, err := migrator.updateLGNextHopRanges(map[netip.Prefix]string{lan: "ixp-lan-a"}); err != nil {
		t.Fatalf("accepted reload: %v", err)
	}
	restore, err := migrator.updateLGNextHopRanges(map[netip.Prefix]string{lan: "ixp-lan-b"})
	if err != nil {
		t.Fatalf("reload to roll back: %v", err)
	}
	restore()
	if holder.ranges[lan] != "ixp-lan-a" {
		t.Errorf("range %s is named %q after the rollback, want ixp-lan-a", lan, holder.ranges[lan])
	}

	restore, err = (&listenerMigrator{}).updateLGNextHopRanges(map[netip.Prefix]string{lan: "ixp-lan-a"})
	if err != nil {
		t.Fatalf("update with no looking glass: %v", err)
	}
	if restore == nil {
		t.Fatal("a migrator with no looking glass answered no restore call")
	}
	restore()
}

// TestUpdateLGNextHopRangesReportsRefusal asks whether a list the looking
// glass refuses fails the reload with the looking glass's reason kept.
func TestUpdateLGNextHopRangesReportsRefusal(t *testing.T) {
	refusal := errors.New("too many ranges")
	migrator := &listenerMigrator{}
	migrator.setLGNextHopRanges(&fakeRangesHolder{refusal: refusal})

	_, err := migrator.updateLGNextHopRanges(nil)
	if !errors.Is(err, refusal) {
		t.Errorf("error = %v, want one that wraps the looking glass's refusal", err)
	}
}
