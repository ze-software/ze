// Design: docs/architecture/container/image-store.md -- the seam's absent answer

package imagestore

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/plugins/container"
)

// TestResolveAbsentNamesReference proves the store answers a container whose
// image it does not hold with Absent, never an empty value: the whole
// reference the short name completed to, and the reason.
//
// Method: resolve a container that names the short reference "redis" in a
// store that holds nothing. The short name completes to the default
// registry's library namespace with the tag latest.
//
// Red on the wiring stub, whose Resolve answers an error for every container,
// until the store's layout and resolve phases land.
func TestResolveAbsentNamesReference(t *testing.T) {
	store := newStore(nil)
	answer, err := store.Resolve(context.Background(), container.ResolveRequest{Instance: "web", Reference: "redis"})
	if err != nil {
		t.Fatalf("resolve of an image the store does not hold answered an error, want Absent: %v", err)
	}
	absent, ok := answer.(container.Absent)
	if !ok {
		t.Fatalf("resolve answered %T, want container.Absent", answer)
	}
	if absent.Reference != "docker.io/library/redis:latest" {
		t.Errorf("absent reference is %q, want docker.io/library/redis:latest", absent.Reference)
	}
	if absent.Reason != container.AbsentNeverFetched {
		t.Errorf("absent reason is %d, want AbsentNeverFetched", absent.Reason)
	}
}
