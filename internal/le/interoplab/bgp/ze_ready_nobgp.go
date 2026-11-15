//go:build !ze_bgp

// Design: docs/architecture/testing/interop.md -- a missing BGP resolver cannot report a passive daemon ready.
// Related: ze_ready_bgp.go -- the resolver-backed implementation.

package bgp

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
)

// zePassiveListenPorts refuses BGP configuration when this le cannot resolve
// its inheritance. No BGP block means no BGP listeners are owed.
func zePassiveListenPorts(tree *config.Tree, name string) ([]string, error) {
	if tree.GetContainer("bgp") == nil {
		return nil, nil
	}
	return nil, fmt.Errorf("resolve %s for the Ze readiness probe: this le was built without ze_bgp, so it cannot resolve passive BGP listeners; rebuild with the launcher's default feature tags and run ./le test integration interop", name)
}
