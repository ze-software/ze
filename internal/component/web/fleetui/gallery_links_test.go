// Design: docs/architecture/fleet-interface.md -- the addresses the component gallery draws
// Related: component_gallery.go -- the gallery's view data, whose addresses this test serves

package fleetui_test

import (
	"bytes"
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/component/web/fleetui/sample"
)

// galleryClock is the pinned clock of the sample source the gallery's
// addresses are served over.
var galleryClock = time.Date(2026, time.November, 18, 15, 0, 0, 0, time.UTC)

// galleryAddressAttribute finds every attribute of the rendered gallery that
// carries an address the browser or a fleet script requests. The leading
// space keeps a data attribute that merely ends in one of the names, such as
// data-confirm-action, which holds a sentence, out of the match.
var galleryAddressAttribute = regexp.MustCompile(`\s(href|hx-get|hx-post|action|data-chart-url|data-terminal-url)="([^"]*)"`)

// The path prefixes of the fleet addresses that are not pages.
const (
	galleryFragmentPrefix = "/fleet/fragment/"
	galleryActionPrefix   = "/fleet/action/"
	galleryConsolePrefix  = "/fleet/console/"
)

// galleryAddress is one address the gallery draws and the attribute that
// carries it.
type galleryAddress struct {
	attribute string
	target    string
}

// galleryAddresses renders the gallery and answers each address it draws,
// once per attribute, in the order drawn.
func galleryAddresses(t *testing.T) []galleryAddress {
	t.Helper()

	var rendered bytes.Buffer
	require.NoError(t, fleetui.ComponentGallery(fleetui.NewGalleryView()).Render(context.Background(), &rendered))

	found := []galleryAddress{}
	for _, match := range galleryAddressAttribute.FindAllStringSubmatch(rendered.String(), -1) {
		address := galleryAddress{attribute: match[1], target: html.UnescapeString(match[2])}
		if !slices.Contains(found, address) {
			found = append(found, address)
		}
	}

	return found
}

// galleryGet requests target from the fleet routes, as htmx does for an
// hx-get attribute and as a link or a script does otherwise.
func galleryGet(t *testing.T, routes http.Handler, target string, htmx bool) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	if htmx {
		request.Header.Set("HX-Request", "true")
	}

	answer := httptest.NewRecorder()
	routes.ServeHTTP(answer, request)

	return answer
}

// requireAnswered asks the routes for target and requires a 2xx answer.
func requireAnswered(t *testing.T, routes http.Handler, address galleryAddress, target string) {
	t.Helper()

	answer := galleryGet(t, routes, target, address.attribute == "hx-get")
	require.GreaterOrEqual(t, answer.Code, http.StatusOK, "%s=%q (requested %s)", address.attribute, address.target, target)
	require.Less(t, answer.Code, http.StatusMultipleChoices, "%s=%q (requested %s) answered %d: %s",
		address.attribute, address.target, target, answer.Code, answer.Body.String())
}

// TestGalleryAddressesAreAnswered renders the component gallery, the States
// page of the prototype, and serves every address it draws through the fleet
// routes over the sample source, the source the prototype and the recorded
// demonstration serve the page from.
//
// VALIDATES: every href, hx-get, hx-post, form action, chart address and
// console address of the gallery parses as the kind of fleet address its
// path names (a page, a fragment, an action, a device's console), and every
// one a browser fetches with GET answers 2xx: a page, a fragment and a chart
// directly, a console through the Console tab of the device it names, which
// the sample estate holds. An action is posted with the form of a selection,
// so it is parsed and not sent.
// PREVENTS: a hand-written gallery address the server refuses, such as a
// task panel tab whose fragment names no page, which shows a broken link on
// the States page and fails the recording of the demonstration.
func TestGalleryAddressesAreAnswered(t *testing.T) {
	source := sample.New(func() time.Time { return galleryClock })
	t.Cleanup(source.Close)
	routes := fleetui.NewRoutes(source, "insecure")

	kinds := map[string]int{}
	for _, address := range galleryAddresses(t) {
		target, err := url.Parse(address.target)
		require.NoError(t, err, "%s=%q", address.attribute, address.target)

		path := target.EscapedPath()
		switch {
		case strings.HasPrefix(path, galleryFragmentPrefix):
			_, err = fleetui.ParseFragment(target)
			require.NoError(t, err, "%s=%q must name a fragment of a page", address.attribute, address.target)
			requireAnswered(t, routes, address, address.target)
			kinds["fragment"]++
		case strings.HasPrefix(path, galleryActionPrefix):
			_, err = fleetui.ParseAction(target)
			require.NoError(t, err, "%s=%q must name an action of a page", address.attribute, address.target)
			require.Contains(t, []string{"hx-post", "action"}, address.attribute, "an action address is posted")
			kinds["action"]++
		case strings.HasPrefix(path, galleryConsolePrefix):
			name, unescapeErr := url.PathUnescape(strings.TrimPrefix(path, galleryConsolePrefix))
			require.NoError(t, unescapeErr, "%s=%q", address.attribute, address.target)
			require.NotEmpty(t, name, "%s=%q must name a device", address.attribute, address.target)
			require.NotContains(t, name, "/", "%s=%q must name one device", address.attribute, address.target)
			console := fleetui.Address{Kind: fleetui.ObjectDevice, Device: name, Tab: "console"}
			requireAnswered(t, routes, address, console.String())
			kinds["console"]++
		default:
			_, err = fleetui.ParseAddress(target)
			require.NoError(t, err, "%s=%q must name a fleet page", address.attribute, address.target)
			requireAnswered(t, routes, address, address.target)
			kinds["page"]++
		}
	}

	// Each kind is drawn by the gallery, so a pattern that stopped matching
	// one fails here rather than passing over nothing.
	for _, kind := range []string{"page", "fragment", "action", "console"} {
		require.Positive(t, kinds[kind], "the gallery draws at least one %s address", kind)
	}
}
