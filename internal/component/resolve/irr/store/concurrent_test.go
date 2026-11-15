// Design: docs/architecture/resolve.md -- shared IRR prefix resolution
package store

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/resolve/irr"
)

// VALIDATES: concurrent Refreshes of distinct names each install their own
// entry: PrefixStore is safe for concurrent use, as its doc comment states, and
// no Refresh loses another's answer. Run with -race.
// PREVENTS: a commit racing another commit on the entries map, dropping an
// entry or leaving it torn. The store's durable write is gone (the irr plugin
// stores from its one worker), so what stays owed is the in-memory half.
// METHOD: six names on one fake IRR, one Refresh each from its own goroutine,
// then Get every name and compare its prefix to the one its server holds.
func TestPrefixStoreConcurrentRefresh(t *testing.T) {
	names := []string{"AS-A", "AS-B", "AS-C", "AS-D", "AS-E", "AS-F"}
	v4 := make(map[string]string, len(names))
	for i, n := range names {
		v4[n] = fmt.Sprintf("10.%d.0.0/24", i)
	}
	s := New(irr.NewIRR(fakeIRR(t, v4, nil)), nil)

	var wg sync.WaitGroup
	for _, n := range names {
		wg.Go(func() {
			if _, err := s.Refresh(context.Background(), n); err != nil {
				t.Errorf("Refresh(%s): %v", n, err)
			}
		})
	}
	wg.Wait()

	for _, n := range names {
		want := netip.MustParsePrefix(v4[n])
		got := s.Get(n)
		if got == nil || len(got.IPv4) != 1 || got.IPv4[0] != want {
			t.Errorf("entry %s after concurrent Refresh = %+v, want %s", n, got, want)
		}
	}
}
