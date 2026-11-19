// Design: docs/architecture/fleet-interface.md -- the prototype action route and its registry
// Related: url.go -- ParseAction and ActionAddress, the address of an action
// Related: register.go -- the route, under the mutation wrap
// Related: pages.go -- the page request every action is answered over
//
// An action is what a confirmed control posts: /fleet/action/<name>/<page
// path>, with the page's view in the query and the control's fields in the
// body. A page family registers its actions with MustRegisterAction from its
// own register file. The prototype changes nothing, so every action answers
// a toast, swapped out of band, that says what would have been done; the
// page the operator is on stays as it is.

package fleetui

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"

	"github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// actionPath is the address prefix of every action.
const actionPath = fleetPath + actionSegment + "/"

// actionSelectionDoneHeader is the response header an action answers with
// when it was carried out on the rows its form posted (ToastView's
// SelectionDone). fleet-components.js reads it and unticks the rows of that
// form, which releases the live refresh they held (fleet.js holdsState).
const actionSelectionDoneHeader = "X-Fleet-Selection-Done"

// actionBytesMax bounds the body of an action: the names of DevicesMax
// devices fit in it with room to spare.
const actionBytesMax = 1 << 20

// echoMax is the most characters of posted text a toast or a refusal quotes
// back. The body of an action may hold actionBytesMax bytes, and a toast
// that quoted all of it would fill the page.
const echoMax = 64

// echoEllipsis marks where echoText cut a posted text.
const echoEllipsis = "..."

// echoText answers posted text as a toast or a refusal quotes it: whole when
// it holds at most echoMax characters, else its first echoMax characters and
// echoEllipsis. It cuts between characters, never inside one.
func echoText(text string) string {
	count := 0
	for index := range text {
		if count == echoMax {
			return text[:index] + echoEllipsis
		}
		count++
	}

	return text
}

// The words of a defective action registration.
const (
	bugAction     = "the action"
	actionSubject = "the fleet actions"
)

// ActionBuilder answers the toast of one posted action, for the page the
// request names. form is the posted body. An error is answered as an error
// toast, except one wrapping ErrAddressUnknown, which says the page has no
// such action and is answered 404.
type ActionBuilder func(ctx context.Context, request *PageRequest, form url.Values) (ToastView, error)

// actionRegistry holds the actions of the build by name. The zero value is
// an empty registry, ready for use. Safe for concurrent use.
type actionRegistry struct {
	mu      sync.RWMutex
	actions map[string]ActionBuilder
}

// registeredActions is the action registry of this build.
var registeredActions actionRegistry

// MustRegisterAction adds an action under name, the second segment of its
// address: /fleet/action/<name>/<page path>. Call it from init() in a
// register file. It panics on an empty name, a nil builder and a name
// already registered: each is a defect of the build's composition.
func MustRegisterAction(name string, build ActionBuilder) {
	if name == "" {
		registrationBug(bugAction, name, actionSubject, bugUnnamed)
	}
	if build == nil {
		registrationBug(bugAction, name, actionSubject, bugNoBuilder)
	}

	registeredActions.mu.Lock()
	defer registeredActions.mu.Unlock()

	if _, held := registeredActions.actions[name]; held {
		registrationBug(bugAction, name, actionSubject, bugTwice)
	}
	if registeredActions.actions == nil {
		registeredActions.actions = make(map[string]ActionBuilder)
	}
	registeredActions.actions[name] = build
}

// lookup answers the builder registered under name, or false.
func (registry *actionRegistry) lookup(name string) (ActionBuilder, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	build, held := registry.actions[name]

	return build, held
}

// actionHandler answers the posted actions over one source. It holds no
// state of its own, so it is safe for concurrent use.
type actionHandler struct {
	source Source
}

// newActionHandler answers the action handler over source.
func newActionHandler(source Source) *actionHandler {
	return &actionHandler{source: source}
}

// ServeHTTP answers one action. An address that names no registered action
// or no page is refused with 404; everything after that is answered as a
// toast.
func (handler *actionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action, err := ParseAction(r.URL)
	if err != nil {
		answerRefusal(w, err, http.StatusNotFound)
		return
	}

	build, held := registeredActions.lookup(action.Name)
	if !held {
		answerRefusal(w, actionUnknown(action.Name), http.StatusNotFound)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, actionBytesMax)
	if formErr := r.ParseForm(); formErr != nil {
		renderAnswer(ctx, w, http.StatusOK, ToastOOB(refusalToast(actionFormUnread(formErr), nil)))
		return
	}

	request, err := NewPageRequest(ctx, handler.source, action.Page, web.GetUsernameFromRequest(r))
	if err != nil {
		answerRefusal(w, err, http.StatusServiceUnavailable)
		return
	}

	toast, err := build(ctx, request, r.PostForm)
	if errors.Is(err, ErrAddressUnknown) {
		answerRefusal(w, err, http.StatusNotFound)
		return
	}
	if err != nil {
		toast = refusalToast(err, nil)
	}
	if toast.SelectionDone {
		w.Header().Set(actionSelectionDoneHeader, "true")
	}

	renderAnswer(ctx, w, http.StatusOK, ToastOOB(toast))
}

// actionUnknown is the refusal for an action name nothing registered.
func actionUnknown(name string) error {
	problem := textbuf.New().Str("The fleet interface has no action named ").Quoted(name).Str(".").Slice()
	return opererr.Wrap(ErrAddressUnknown, problem,
		"The form asked for an action that does not exist, so nothing was done.",
		"reload the page to get its current actions")
}

// actionFormUnread is the refusal of an action whose body could not be read.
func actionFormUnread(err error) error {
	return opererr.Wrap(err, "The fields sent with the action could not be read.",
		"The request was malformed or longer than an action accepts, so nothing was done.",
		"select every match instead of many rows, and try again")
}
