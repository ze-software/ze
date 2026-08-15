// Design: docs/architecture/web-interface.md -- the error answer a refused htmx
// request receives
// Related: assets/notification.js -- handleResponseError, the browser side that
// reads the fragment and shows the toast
// Related: auth.go -- serverHandler, the one chain both the daemon and the
// golden capture wrap their mux with

package web

import (
	"net/http"

	"github.com/ze-software/ze/internal/core/errorfragment"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrorData holds the data for rendering an error item via the oob_error template.
type ErrorData struct {
	ID      string
	Path    string
	Message string
}

// WriteOOBError answers a refused action with the error fragment for the
// request's target, followed by an out-of-band swap that appends the same error
// to #error-list and opens the error panel.
//
// The fragment leads deliberately. htmx removes every hx-swap-oob element from
// a response before it swaps what remains, so a body carrying only the
// out-of-band error swaps NOTHING into the target: under htmx 4, which swaps
// every response except 204 and 304, that would empty the element the operator
// was looking at. The fragment is what lands there instead. Under htmx 2 no 4xx
// is swapped at all and only assets/notification.js reads this body.
//
// The fragment is the shared one (internal/core/errorfragment), which is also
// what the middleware writes for the http.Error sites. An operator therefore
// meets one error shape, whichever route refused the action.
func WriteOOBError(w http.ResponseWriter, renderer *Renderer, path, message string, status int) {
	var bID textbuf.Buffer

	data := ErrorData{
		ID:      bID.Reset().Int(int64(len(message) + len(path))).String(),
		Path:    path,
		Message: message,
	}

	html := renderer.renderComponent("oob_error", oobError(data))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if _, err := w.Write(errorfragment.Render(status, message)); err != nil {
		return // client disconnected
	}

	if _, err := w.Write([]byte(html)); err != nil {
		return // client disconnected
	}
}
