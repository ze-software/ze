// Design: docs/architecture/testing/interop.md -- a checker removes lab plumbing that would mask what Ze sends.

package bgp

import (
	"context"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// ipRouteDelete is the iproute2 verb that removes a route.
const ipRouteDelete = "del"

// removeContainerDefaultRoute deletes the default route Docker installs in a
// peer's container, so a default the peer learns by routing protocol is the
// only one it can select. Docker.Exec answers an error for any non-zero exit
// and returns the command's streams with it; a container with no default
// route answers "No such process", which is already the state wanted.
func removeContainerDefaultRoute(ctx context.Context, lab interoplab.CheckerLab, peer string) error {
	result, err := lab.Exec(ctx, peer, []string{"ip", "route", ipRouteDelete, "default"}, nil)
	if err == nil {
		return nil
	}
	if strings.Contains(result.Stderr, "No such process") || strings.Contains(err.Error(), "No such process") {
		return nil
	}
	return fmt.Errorf("remove the Docker default route from %s: %w", peer, err)
}
