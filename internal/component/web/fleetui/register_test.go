package fleetui

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/web"
)

// fleetRoutes answers the registered web routes that sit under the fleet
// prefix, whatever their method. It reads the web package's registry, which
// is what the hub mounts.
func fleetRoutes() []web.WebRoute {
	var found []web.WebRoute
	for _, route := range web.RegisteredWebRoutes() {
		_, path, _ := strings.Cut(route.Pattern, " ")
		if strings.HasPrefix(path, fleetPath) {
			found = append(found, route)
		}
	}

	return found
}

// fleetPatterns is every route the fleet interface registers.
var fleetPatterns = []string{
	assetsPattern, fragmentPattern, eventsPattern, stepPattern, holdPattern, shellPattern,
	actionPattern, auditExportPattern, consolePattern,
}

// TestFleetRoutesAbsentWithoutSource reads the fleet routes out of the web
// route registry, in a test binary that links no data source, and asks each
// one whether the hub would mount it.
//
// VALIDATES: AC-7 -- a build with the fleet package and no source mounts no
// fleet route and adds no portal entry, because the hub skips a route whose
// Enabled answers false before it registers the route's portal entry.
// PREVENTS: a standard build serving a fleet page with no estate behind it.
func TestFleetRoutesAbsentWithoutSource(t *testing.T) {
	_, linked := buildSource.registered()
	require.False(t, linked, "this test binary must link no data source")

	routes := fleetRoutes()

	patterns := make([]string, 0, len(routes))
	for _, route := range routes {
		patterns = append(patterns, route.Pattern)
	}
	require.ElementsMatch(t, fleetPatterns, patterns, "the routes that register from init()")

	for _, route := range routes {
		require.NotNil(t, route.Enabled, "%s must be gated on a registered source", route.Pattern)
		assert.False(t, route.Enabled(), "%s must not mount with no source registered", route.Pattern)
	}
}

// TestFleetPortalEntryRidesTheShellRoute reads which fleet route carries the
// portal entry.
//
// VALIDATES: the portal entry belongs to the gated shell route, opens the
// fleet address directly, and no second route carries one.
// PREVENTS: a portal entry on an always-mounted route, which would list Fleet
// in a build that serves no fleet page.
func TestFleetPortalEntryRidesTheShellRoute(t *testing.T) {
	var entries []web.PortalService
	for _, route := range fleetRoutes() {
		if route.Portal == nil {
			continue
		}
		assert.Equal(t, shellPattern, route.Pattern)
		entries = append(entries, *route.Portal)
	}

	require.Len(t, entries, 1)
	assert.Equal(t, fleetPath, entries[0].NavigationURL())
}

// fixedSource is a source that answers one estate. It embeds the interface
// to stand in for a whole source in a test that only stores it: the slot
// never calls it, and any question but Estate would panic on the nil
// embedded value.
type fixedSource struct {
	Source
	name string
}

func (source fixedSource) Estate(context.Context) (Estate, error) {
	return Estate{Name: source.name}, nil
}

// TestSourceSlotHoldsOneSource drives a slot of its own through an empty
// state, a refused nil, a first registration and a refused second one.
//
// VALIDATES: an empty slot answers false, the first source is kept, and a nil
// source or a second source is refused without changing the slot.
// PREVENTS: a second registration silently replacing the estate the pages
// read, and a nil source mounting routes that panic on the first request.
func TestSourceSlotHoldsOneSource(t *testing.T) {
	var slot sourceSlot

	_, ok := slot.registered()
	require.False(t, ok, "a new slot is empty")

	require.ErrorIs(t, slot.register(nil), errSourceAbsent)
	_, ok = slot.registered()
	require.False(t, ok, "a refused nil leaves the slot empty")

	require.NoError(t, slot.register(fixedSource{name: "first"}))
	require.ErrorIs(t, slot.register(fixedSource{name: "second"}), errSourceRegistered)

	kept, ok := slot.registered()
	require.True(t, ok)
	assert.Equal(t, fixedSource{name: "first"}, kept)
}
