package register

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/core/env"
)

// holdCounter is a script that counts the Hold calls it receives.
type holdCounter struct {
	holds int
}

func (*holdCounter) Step() (fleetui.Event, error) { return fleetui.Event{}, nil }

func (counter *holdCounter) Hold() { counter.holds++ }

// TestHeldSettingHoldsTheScript reads each spelling of the held setting and
// gives the script it governs a counter in place of the sample source.
//
// VALIDATES: ze.web.fleet.sample.held true holds the script, once; false, an
// unset key, and a value that is no boolean leave it timed, and the value
// that is no boolean is recorded for ze doctor.
// PREVENTS: a held review build that still runs its timer, and a mistyped
// value taken silently for timed.
func TestHeldSettingHoldsTheScript(t *testing.T) {
	cases := []struct {
		value string
		holds int
	}{
		{value: "true", holds: 1},
		{value: "false", holds: 0},
		{value: "", holds: 0},
		{value: "maybe", holds: 0},
	}

	for _, check := range cases {
		t.Run("value="+check.value, func(t *testing.T) {
			t.Setenv(scriptHeldKey, check.value)
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			counter := &holdCounter{}
			holdWhenAsked(counter)
			assert.Equal(t, check.holds, counter.holds)
		})
	}

	t.Setenv(scriptHeldKey, "maybe")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	holdWhenAsked(&holdCounter{})

	recorded := false
	for _, invalid := range env.InvalidValues() {
		if invalid.Key == scriptHeldKey {
			recorded = true
		}
	}
	assert.True(t, recorded, "a value that is no boolean is reported, not taken silently")
}

// TestHeldSourceStartsOnTheFirstStep builds the source the registration
// builds, with the held setting true, and steps it once.
//
// VALIDATES: the held source answers the first step of the script, the
// device the script takes offline, as the first event its subscription
// carries.
// PREVENTS: a held build whose first Step answers some later step.
func TestHeldSourceStartsOnTheFirstStep(t *testing.T) {
	t.Setenv(scriptHeldKey, "true")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	start := time.Date(2026, time.November, 19, 9, 0, 0, 0, time.UTC)
	source := newSource(func() time.Time { return start })
	t.Cleanup(source.Close)

	subscription, err := source.Subscribe(t.Context())
	require.NoError(t, err)

	event, err := source.Step()
	require.NoError(t, err)
	assert.Equal(t, fleetui.EventDeviceChanged, event.Kind)
	assert.Equal(t, "cpe-lon-03", event.Device, "the first step takes cpe-lon-03 offline")

	select {
	case first := <-subscription.Events():
		assert.Equal(t, event, first, "nothing was published before the stepped event")
	default:
		require.Fail(t, "Step published no event")
	}
}

// fleetPrefix is the path every fleet route pattern starts with, after its
// method.
const fleetPrefix = "/fleet/"

// fleetRouteCount is every fleet route: the shell, the assets, the
// fragments, the event stream, the audit export, the actions and the
// console, and the step and hold routes the sample's script mounts.
const fleetRouteCount = 9

// TestRegisteredSampleMountsTheFleetRoutes mounts the registered fleet routes
// the way the hub does when the web server starts without login: it skips a
// route whose Enabled answers false, builds the others, and wraps each in the
// insecure middleware. Then it asks for the estate dashboard.
//
// VALIDATES: linking this package registers the sample as the build's
// source: every fleet route is enabled, the prototype-only ones included, and
// the shell draws the sample estate.
// PREVENTS: a review build whose registration never reaches the web route
// registry, so its fleet address answers nothing.
func TestRegisteredSampleMountsTheFleetRoutes(t *testing.T) {
	mux := http.NewServeMux()
	mounted := 0

	for _, route := range web.RegisteredWebRoutes() {
		_, path, _ := strings.Cut(route.Pattern, " ")
		if !strings.HasPrefix(path, fleetPrefix) {
			continue
		}
		if route.Enabled != nil && !route.Enabled() {
			continue
		}

		mux.Handle(route.Pattern, web.InsecureMiddleware(route.Build(web.RouteDeps{})))
		mounted++
	}

	require.Equal(t, fleetRouteCount, mounted, "linking the registration mounts every fleet route, the script's included")

	answer := httptest.NewRecorder()
	mux.ServeHTTP(answer, httptest.NewRequestWithContext(t.Context(), http.MethodGet, fleetPrefix, http.NoBody))

	require.Equal(t, http.StatusOK, answer.Code)
	assert.Contains(t, answer.Body.String(), "fleet-dashboard-slot", "the estate dashboard draws its cards")
}
