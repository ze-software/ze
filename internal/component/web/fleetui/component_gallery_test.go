package fleetui

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// componentMarkup renders a component to its markup.
func componentMarkup(t *testing.T, component templ.Component) string {
	t.Helper()

	var markup bytes.Buffer
	if err := component.Render(context.Background(), &markup); err != nil {
		t.Fatalf("render: %v", err)
	}

	return markup.String()
}

// galleryRootClasses names the root class of every component the gallery
// must draw. A component added to the set is added here.
var galleryRootClasses = []string{
	"fleet-gallery", "fleet-icon", "fleet-illustration", "fleet-state", "fleet-state-mark",
	"fleet-object-icon", "fleet-object-icon-mark", "fleet-chip", "fleet-copy",
	"fleet-action-link", "fleet-button", "fleet-button--primary", "fleet-icon-button",
	"fleet-menu", "fleet-menu--dots", "fleet-menu-divider", "fleet-action--destructive",
	"fleet-card-grid--12", "fleet-card-grid--8", "fleet-card", "fleet-count-badge",
	"fleet-card-link", "fleet-card-figure", "fleet-card-foot", "fleet-figures--columns",
	"fleet-figure-unit", "fleet-donut", "fleet-donut-arc", "fleet-donut-track", "fleet-legend",
	"fleet-info-mark", "fleet-donut-total", "fleet-usage", "fleet-usage-scale",
	"fleet-usage-track--tall", "fleet-usage-inside", "fleet-usage-group", "fleet-usage-mini",
	"fleet-stacked", "fleet-chart", "fleet-chart-legend", "fleet-quick-info", "fleet-callout",
	"fleet-alarm-list", "fleet-good-news", "fleet-section", "fleet-query",
	"fleet-query-refusal", "fleet-table-actions", "fleet-selection", "fleet-pager",
	"fleet-table", "fleet-table-row--opened", "fleet-table-controls", "fleet-table-empty",
	"fleet-table-sort", "fleet-panel", "fleet-panel-card", "fleet-panel-empty", "fleet-kv",
	"fleet-hero--no-data", "fleet-hero--nothing-configured", "fleet-hero--offline",
	"fleet-hero--not-supported", "fleet-hero--relay-refused", "fleet-hero--unauthorized",
	"fleet-hero--failed", "fleet-skeleton", "fleet-skeleton-card", "fleet-banner--stale",
	"fleet-toast--info", "fleet-toast--success", "fleet-toast--warning", "fleet-toast--error",
	"fleet-confirm--static", "fleet-confirm--destructive", "fleet-task-panel",
	"fleet-task-tab--active", "fleet-task-list", "fleet-task-details", "fleet-progress",
	"fleet-terminal", "fleet-terminal-entry--refused", "fleet-diff", "fleet-diff-row--added",
	"fleet-diff-row--removed", "fleet-diff-row--changed", "fleet-diff-row--same",
}

// markupHasClass reports whether markup holds an element carrying the class.
func markupHasClass(markup, class string) bool {
	pattern := regexp.MustCompile(`class="[^"]*\b` + regexp.QuoteMeta(class) + `(?:[ "])`)

	return pattern.MatchString(markup)
}

// TestComponentGalleryDrawsEveryComponent renders the gallery from its fixed
// data and looks for the root class of every component.
//
// VALIDATES: every component, state and variant of the set renders, and the
// gallery names no icon or illustration the set does not draw.
// PREVENTS: a component that compiles and is never drawn, and a blank
// placeholder where an icon was misspelled.
func TestComponentGalleryDrawsEveryComponent(t *testing.T) {
	markup := componentMarkup(t, ComponentGallery(NewGalleryView()))

	for _, class := range galleryRootClasses {
		if !markupHasClass(markup, class) {
			t.Errorf("the gallery draws no element with class %q", class)
		}
	}

	if strings.Contains(markup, "fleet-icon--unknown") {
		t.Error("the gallery names an icon or an illustration that is not drawn")
	}

	if strings.Contains(markup, "fleet-tone--unspecified") {
		t.Error("the gallery draws a tone that is the invalid zero value")
	}
}

// TestToastAreaHoldsTheConfirmDialog renders the overlays the shell holds.
//
// VALIDATES: the toast area carries the region toasts are appended to and the
// one confirm dialog, with every slot the script fills.
// PREVENTS: a control carrying data-confirm attributes that submits with no
// confirmation because the page holds no dialog.
func TestToastAreaHoldsTheConfirmDialog(t *testing.T) {
	markup := componentMarkup(t, ToastArea())

	for _, want := range []string{
		`id="fleet-toasts"`, `<dialog`, `id="fleet-confirm"`, `data-confirm-slot="action"`,
		`data-confirm-slot="targets"`, `data-confirm-slot="detail"`, `data-confirm-slot="expect"`,
		`data-confirm-slot="expect-block"`, `data-confirm-slot="input"`, `data-confirm-slot="confirm"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("the toast area holds no %s", want)
		}
	}
}

// TestIconNamesAreAllDrawn renders every name the icon set lists.
//
// VALIDATES: each listed icon and illustration has a drawing, and a name
// outside the list draws the placeholder.
// PREVENTS: a name added to the list and not to the drawings, or the reverse
// going unnoticed.
func TestIconNamesAreAllDrawn(t *testing.T) {
	for _, name := range IconNames() {
		if strings.Contains(componentMarkup(t, Icon(name)), "fleet-icon--unknown") {
			t.Errorf("icon %q is listed and not drawn", name)
		}
	}

	for _, name := range IllustrationNames() {
		if strings.Contains(componentMarkup(t, Illustration(name)), "fleet-icon--unknown") {
			t.Errorf("illustration %q is listed and not drawn", name)
		}
	}

	if !strings.Contains(componentMarkup(t, Icon("no-such-icon")), "fleet-icon--unknown") {
		t.Error("an unlisted icon name draws no placeholder")
	}
}

// templIconFloor is the fewest icon uses the template walk must read, which
// makes a vacuous walk visible.
const templIconFloor = 10

// TestTemplatesNameKnownIcons reads every template of the package for the
// icon names written as literals.
//
// VALIDATES: every @Icon("name") in a fleet template names a listed icon.
// PREVENTS: a misspelled icon in the shell or a page, which renders the
// placeholder and reports success.
func TestTemplatesNameKnownIcons(t *testing.T) {
	files, err := filepath.Glob("*.templ")
	if err != nil {
		t.Fatalf("list the templates: %v", err)
	}

	names := IconNames()
	use := regexp.MustCompile(`@Icon\("([^"]*)"\)`)
	read := 0
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		for _, match := range use.FindAllStringSubmatch(string(source), -1) {
			read++
			if !slices.Contains(names, match[1]) {
				t.Errorf("%s names the icon %q, which IconNames does not list", file, match[1])
			}
		}
	}

	if read < templIconFloor {
		t.Errorf("the walk read %d icon uses, want at least %d", read, templIconFloor)
	}
}

// TestStatusTones pins the tone of every state by its named constant.
//
// VALIDATES: each display, health, configuration, alarm and task state is
// drawn in the color the umbrella gives it, with the state's own word.
// PREVENTS: a tone table that drifted from the order of its enum.
func TestStatusTones(t *testing.T) {
	cases := []struct {
		name string
		got  Status
		tone Tone
	}{
		{"offline", DisplayStatus(DisplayStateOffline), ToneMuted},
		{"diverged", DisplayStatus(DisplayStateDiverged), ToneWarning},
		{"down", DisplayStatus(DisplayStateDown), ToneError},
		{"degraded", DisplayStatus(DisplayStateDegraded), ToneWarning},
		{"healthy", DisplayStatus(DisplayStateHealthy), ToneSuccess},
		{"unknown", DisplayStatus(DisplayStateUnknown), ToneNeutral},
		{"display zero", DisplayStatus(DisplayStateUnspecified), ToneNeutral},
		{"health healthy", HealthStatus(HealthStateHealthy), ToneSuccess},
		{"health degraded", HealthStatus(HealthStateDegraded), ToneWarning},
		{"health down", HealthStatus(HealthStateDown), ToneError},
		{"in sync", ConfigStatus(ConfigStateInSync), ToneSuccess},
		{"pending", ConfigStatus(ConfigStatePending), ToneAccent},
		{"rejected", ConfigStatus(ConfigStateRejected), ToneError},
		{"config diverged", ConfigStatus(ConfigStateDiverged), ToneWarning},
		{"never fetched", ConfigStatus(ConfigStateNeverFetched), ToneMuted},
		{"critical", AlarmStatus(AlarmSeverityCritical), ToneError},
		{"warning", AlarmStatus(AlarmSeverityWarning), ToneWarning},
		{"informational", AlarmStatus(AlarmSeverityInformational), ToneAccent},
		{"task pending", TaskStatus(TaskStatePending), ToneNeutral},
		{"task running", TaskStatus(TaskStateRunning), ToneProgress},
		{"task succeeded", TaskStatus(TaskStateSucceeded), ToneSuccess},
		{"task failed", TaskStatus(TaskStateFailed), ToneError},
		{"task canceled", TaskStatus(TaskStateCanceled), ToneMuted},
		{"task partly failed", TaskStatus(TaskStatePartlyFailed), ToneWarning},
	}

	for _, test := range cases {
		if test.got.Tone != test.tone {
			t.Errorf("%s: tone %q, want %q", test.name, test.got.Tone.Key(), test.tone.Key())
		}

		if test.got.Word == "" {
			t.Errorf("%s: the status has no word", test.name)
		}
	}

	if got := DisplayStatus(DisplayStateDown).Word; got != DisplayStateDown.String() {
		t.Errorf("the word of a display status is %q, want the state's own %q", got, DisplayStateDown.String())
	}
}

// TestToastLifetimes pins how long each level of toast stays.
//
// VALIDATES: an informational and a success toast leave after six seconds; a
// warning and an error stay until dismissed, and interrupt as alerts.
// PREVENTS: an error that disappears before the operator read it.
func TestToastLifetimes(t *testing.T) {
	cases := []struct {
		level   ToastLevel
		seconds int
		role    string
	}{
		{ToastInformational, 6, "status"},
		{ToastSuccess, 6, "status"},
		{ToastWarning, 0, "alert"},
		{ToastError, 0, "alert"},
	}

	for _, test := range cases {
		if got := test.level.LifetimeSeconds(); got != test.seconds {
			t.Errorf("%s: lifetime %d s, want %d", test.level.Word(), got, test.seconds)
		}

		markup := componentMarkup(t, Toast(ToastView{Level: test.level, Message: "message"}))
		if !strings.Contains(markup, `role="`+test.role+`"`) {
			t.Errorf("%s: the toast is not drawn with role %q", test.level.Word(), test.role)
		}
	}
}
