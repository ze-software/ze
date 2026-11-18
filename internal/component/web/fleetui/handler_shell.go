// Design: docs/architecture/fleet-interface.md -- the fleet shell page and its assets
// Related: shell.templ -- the markup this handler renders
// Related: source.go -- the data source this handler reads
// Related: register.go -- the routes that reach these handlers

package fleetui

import (
	"bytes"
	"embed"
	"net/http"

	"github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/opererr"
)

// The addresses the fleet interface is served at. Every fleet route sits under
// fleetPath, which no other web route uses.
const (
	fleetPath       = "/fleet/"
	fleetAssetsPath = "/fleet/assets/"
)

// assetsFS holds the fleet styles and scripts. The package embeds its own, so
// a build without the fleet package carries none of them.
//
//go:embed assets
var assetsFS embed.FS

// ShellView is everything the shell renders for one request.
type ShellView struct {
	// EstateName is the name of the estate, shown in the header and as the
	// name of the object the main pane shows.
	EstateName string
	// Username is the account the request runs as.
	Username string
	// Devices holds every device of the estate.
	Devices []Device
}

// newShellHandler answers the estate page from source. It takes the source
// itself, so it cannot be built before one is registered.
func newShellHandler(source Source) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The route owns the whole /fleet/ subtree, and the estate page is
		// its only page so far.
		if r.URL.Path != fleetPath {
			http.NotFound(w, r)
			return
		}

		estate, err := source.Estate(r.Context())
		if err != nil {
			refusal := opererr.Wrap(err, "The estate could not be read.",
				"The fleet data source did not answer, so the page has nothing to show.",
				"ze show doctor")
			http.Error(w, refusal.Error(), http.StatusServiceUnavailable)
			return
		}

		devices, err := source.Devices(r.Context())
		if err != nil {
			refusal := opererr.Wrap(err, "The devices of the estate could not be read.",
				"The fleet data source did not answer, so the page has nothing to show.",
				"ze show doctor")
			http.Error(w, refusal.Error(), http.StatusServiceUnavailable)
			return
		}

		view := ShellView{
			EstateName: estate.Name,
			Username:   web.GetUsernameFromRequest(r),
			Devices:    devices,
		}

		// Render to memory first, so a failed render answers an error
		// instead of half a page under a 200.
		var page bytes.Buffer
		if err := Shell(view).Render(r.Context(), &page); err != nil {
			refusal := opererr.Wrap(err, "The fleet page could not be rendered.",
				"The estate was read, but its page failed to build, so nothing can be shown.",
				"ze show doctor")
			http.Error(w, refusal.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(page.Bytes()) //nolint:errcheck // the answer is committed; a failed write means the browser left
	})
}

// newAssetHandler serves the embedded fleet assets at fleetAssetsPath. Assets
// use no-cache, as the web package's own do, so a browser picks up a new
// binary's files without a hard refresh.
func newAssetHandler() http.Handler {
	// Stripping fleetPath leaves "assets/<file>", which is the file's path
	// in the embedded tree.
	files := http.StripPrefix(fleetPath, http.FileServerFS(assetsFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		files.ServeHTTP(w, r)
	})
}
