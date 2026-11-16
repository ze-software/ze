// Design: docs/architecture/update-building.md -- current selected authority.
// Related: selected_write.go -- final writer rechecks this snapshot.
package reactor

import (
	"errors"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errSelectedRevision = errors.New("select-routes: selection revision is no longer current; reconcile the current destination snapshot")

// selectedCurrentRevision checks current desired state, never route history.
// A still-current key can be older than its scope's cut. An absent key keeps no
// historical revision: serialized final writes and the current cut fence it.
func selectedCurrentRevision(route rpc.SelectedRoute, current rpc.CurrentSelectionResult, cut uint64) error {
	if route.Revision > cut {
		return errSelectedRevision
	}
	if route.Kind == rpc.SelectedRouteAbsent {
		if current.Presence != rpc.CurrentSelectionAbsent {
			return errSelectedRevision
		}
		return nil
	}
	if current.Presence != rpc.CurrentSelectionPresent {
		return errSelectedRevision
	}
	if current.CurrentRevision != route.Revision {
		return errSelectedRevision
	}
	return nil
}
