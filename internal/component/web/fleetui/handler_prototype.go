// Design: docs/architecture/fleet-interface.md -- the prototype's states page and its toast demonstrations
// Related: register_activity_pages.go -- registers the page, its tab and the fragment built here
// Related: page_activity.templ -- statesPage, which draws the page
// Related: component_gallery.go -- NewGalleryView, every component in every state
// Related: handler_events.go -- the step and hold routes the page's script controls post to
//
// The states page is the prototype's: an administration page that draws
// every component in every state of the umbrella's states table, so each can
// be judged in both themes without provoking it. At its top, two controls
// drive the sample script through the shell's prototype-only routes, and
// four buttons each ask for one toast of one level, so lifetimes and
// stacking can be judged.

package fleetui

import (
	"context"
	"strings"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// fragmentToastDemo is the fragment that answers one toast of the level its
// item names, by the level's key.
const fragmentToastDemo = "toast-demo"

// statesEventID is the id of the line of the states page that shows what the
// step and hold routes answered.
const statesEventID = "fleet-activity-event"

// toastDemoLevels are the levels the states page demonstrates, in the order
// of its buttons.
var toastDemoLevels = []ToastLevel{ToastInformational, ToastSuccess, ToastWarning, ToastError}

// statesContent builds the states page.
func statesContent(_ context.Context, request *PageRequest) (templ.Component, error) {
	return statesPage(statesPageView(request)), nil
}

// statesPageView fills the states page. The script controls are drawn only
// over a source that can be stepped, because only then are their routes
// mounted (stepperRegistered).
func statesPageView(request *PageRequest) StatesPageView {
	_, steps := request.Source.(Stepper)

	view := StatesPageView{
		Script: PrototypeScriptView{
			StepAction: stepPath,
			HoldAction: holdPath,
			Output:     statesEventID,
			Steps:      steps,
		},
		Toasts:  make([]ToastDemoView, 0, len(toastDemoLevels)),
		Gallery: NewGalleryView(),
	}

	for _, level := range toastDemoLevels {
		view.Toasts = append(view.Toasts, ToastDemoView{
			Label: level.Word() + " toast",
			Href:  FragmentLink(fragmentToastDemo, request.Address, level.Key()),
		})
	}

	return view
}

// toastDemoFragment answers one toast of the level item names. A level that
// does not exist answers the error toast of the refusal, so the answer always
// lands in the toast area as a toast.
func toastDemoFragment(_ context.Context, request *PageRequest, item string) (templ.Component, error) {
	level, known := enumParse[ToastLevel](toastKeys, item)
	if !known {
		return Toast(refusalToast(toastLevelUnknown(item), statesLink(request))), nil
	}

	return Toast(toastDemoView(request, level)), nil
}

// toastDemoView answers the demonstration toast of one level. It says how
// long it stays, and links back to the states page, so it is not the only
// record of anything.
func toastDemoView(request *PageRequest, level ToastLevel) ToastView {
	lifetime := "It stays until you dismiss it."
	if seconds := level.LifetimeSeconds(); seconds > 0 {
		lifetime = "It leaves after " + activityUnits(int64(seconds), "second") + "."
	}

	return ToastView{
		Level:   level,
		Message: "This is how a toast of the " + strings.ToLower(level.Word()) + " level looks.",
		Detail:  lifetime + " At most four show at once, and the others are counted above them.",
		Link:    statesLink(request),
	}
}

// statesLink answers the link to the states page, which every toast the
// page asks for carries.
func statesLink(request *PageRequest) *LinkView {
	states := Address{Kind: ObjectAdmin, Tab: activityTabStates, View: linkView(request)}

	return &LinkView{Label: "Open the states page", Href: states.String()}
}

// toastLevelUnknown is the refusal of a toast level that does not exist.
func toastLevelUnknown(key string) error {
	problem := textbuf.New().Str("No toast level is named ").Quoted(key).Str(".").String()

	keys := make([]string, 0, len(toastDemoLevels))
	for _, level := range toastDemoLevels {
		keys = append(keys, level.Key())
	}

	return opererr.Wrap(ErrAddressUnknown, problem,
		"A demonstration shows one of the four levels, and this names none, so no toast was drawn.",
		textbuf.New().Str("ask for one of: ").Join(keys, ", ").String())
}
