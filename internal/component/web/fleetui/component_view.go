// Design: docs/architecture/fleet-interface.md -- view data shared by the fleet components
// Related: icons.templ -- the marks a tone and a status are drawn with
// Related: component_action.templ -- the links, buttons and menus built from these views

package fleetui

import (
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The view data every fleet component shares. A page handler fills these from
// the data layer's answers, so they hold only what the markup writes: words,
// numbers, addresses and small enums. No component reads a source.

// Tone is the colour family a mark, a bar or a toast is drawn in. The zero
// value is not a tone. Colour never carries a meaning alone: a tone is always
// drawn with its own mark and beside a word.
type Tone uint8

// The tones, each drawn with one theme variable.
const (
	// ToneUnspecified is the invalid zero value.
	ToneUnspecified Tone = iota
	// ToneNeutral is drawn in the secondary text colour: a state not known.
	ToneNeutral
	// ToneMuted is drawn in the muted text colour: offline, canceled, absent.
	ToneMuted
	// ToneAccent is drawn in the accent colour: information, pending.
	ToneAccent
	// ToneSuccess is drawn in the success colour.
	ToneSuccess
	// ToneWarning is drawn in the warning colour.
	ToneWarning
	// ToneError is drawn in the error colour.
	ToneError
	// ToneProgress is drawn in the accent colour with the running mark.
	ToneProgress
)

var (
	toneKeys  = []string{"unspecified", "neutral", "muted", "accent", "success", "warning", "error", "progress"}
	toneMarks = []string{
		"mark-unknown", "mark-unknown", "mark-offline", "mark-info",
		"mark-ok", "mark-warning", "mark-error", "mark-progress",
	}
)

// Key answers the tone's name in a class. A value outside the named set
// answers the name of the invalid zero value, which the stylesheet does not
// colour.
func (tone Tone) Key() string {
	if int(tone) < len(toneKeys) {
		return toneKeys[tone]
	}

	return toneKeys[ToneUnspecified]
}

// Class answers the class that colours an element in the tone.
func (tone Tone) Class() string {
	return "fleet-tone--" + tone.Key()
}

// Mark answers the name of the round icon drawn for the tone.
func (tone Tone) Mark() string {
	if int(tone) < len(toneMarks) {
		return toneMarks[tone]
	}

	return toneMarks[ToneUnspecified]
}

// Status is a state as a component draws it: a tone and the word written
// beside its mark.
type Status struct {
	Tone Tone
	Word string
}

// The tone of each state, indexed by the state. Each slice follows the
// declaration order of its enum in state.go, and TestStatusTones pins every
// entry by its named constant.
var (
	displayStateTones  = []Tone{ToneNeutral, ToneMuted, ToneWarning, ToneError, ToneWarning, ToneSuccess, ToneNeutral}
	healthStateTones   = []Tone{ToneNeutral, ToneSuccess, ToneWarning, ToneError}
	configStateTones   = []Tone{ToneNeutral, ToneSuccess, ToneAccent, ToneError, ToneWarning, ToneMuted}
	alarmSeverityTones = []Tone{ToneNeutral, ToneError, ToneWarning, ToneAccent}
	taskStateTones     = []Tone{
		ToneNeutral, ToneNeutral, ToneProgress, ToneSuccess, ToneError, ToneMuted, ToneWarning,
	}
	rolloutResultTones       = []Tone{ToneNeutral, ToneNeutral, ToneSuccess, ToneError}
	configVersionResultTones = []Tone{ToneNeutral, ToneSuccess, ToneError, ToneMuted}
	auditResultTones         = []Tone{
		ToneNeutral, ToneSuccess, ToneAccent, ToneNeutral, ToneProgress, ToneWarning,
		ToneSuccess, ToneError, ToneMuted, ToneWarning,
	}
)

// toneOf answers the tone of an enum value. A value outside the named set is
// drawn neutral, beside the word the enum itself answers for it.
func toneOf[T ~uint8](tones []Tone, value T) Tone {
	if int(value) < len(tones) {
		return tones[value]
	}

	return ToneNeutral
}

// DisplayStatus answers how a display state is drawn.
func DisplayStatus(state DisplayState) Status {
	return Status{Tone: toneOf(displayStateTones, state), Word: state.String()}
}

// HealthStatus answers how the state of a health component is drawn.
func HealthStatus(state HealthState) Status {
	return Status{Tone: toneOf(healthStateTones, state), Word: state.String()}
}

// ConfigStatus answers how a configuration state is drawn.
func ConfigStatus(state ConfigState) Status {
	return Status{Tone: toneOf(configStateTones, state), Word: state.String()}
}

// AlarmStatus answers how an alarm severity is drawn.
func AlarmStatus(severity AlarmSeverity) Status {
	return Status{Tone: toneOf(alarmSeverityTones, severity), Word: severity.String()}
}

// TaskStatus answers how a task state is drawn.
func TaskStatus(state TaskState) Status {
	return Status{Tone: toneOf(taskStateTones, state), Word: state.String()}
}

// RolloutResultStatus answers how the result of one device of a rollout is
// drawn.
func RolloutResultStatus(result RolloutResult) Status {
	return Status{Tone: toneOf(rolloutResultTones, result), Word: result.String()}
}

// ConfigVersionResultStatus answers how a device's answer to one version of
// its configuration is drawn.
func ConfigVersionResultStatus(result ConfigVersionResult) Status {
	return Status{Tone: toneOf(configVersionResultTones, result), Word: result.String()}
}

// AuditResultStatus answers how the result of an audit entry is drawn.
func AuditResultStatus(result AuditResult) Status {
	return Status{Tone: toneOf(auditResultTones, result), Word: result.String()}
}

// AdjacencyStatus answers how the state of a routing adjacency is drawn: its
// word is the device's, its tone whether the device holds it up.
func AdjacencyStatus(adjacency *Adjacency) Status {
	switch {
	case adjacency.State == "":
		return Status{Tone: ToneNeutral, Word: enumWordUnspecified}
	case adjacency.Up:
		return Status{Tone: ToneSuccess, Word: adjacency.State}
	default:
		return Status{Tone: ToneWarning, Word: adjacency.State}
	}
}

// LinkView is one link: its words and the address it opens.
type LinkView struct {
	Label string
	Href  string
}

// ConfirmView is what the confirm dialog shows before an action runs. Expect
// is the text the operator types before a destructive action is enabled: the
// name of the target, or the count of targets.
type ConfirmView struct {
	Action      string
	Targets     string
	Detail      string
	Expect      string
	Destructive bool
}

// ActionView is one action an operator can take. With Href it is a link. With
// none it is a button that submits the form named by Form, carrying Name and
// Value. A disabled action keeps its place and says why in Reason.
type ActionView struct {
	// ID is the id of the drawn link or button, for a control a script or a
	// test names. Empty draws none.
	ID       string
	Label    string
	Icon     string
	Href     string
	Form     string
	Name     string
	Value    string
	Reason   string
	Disabled bool
	// Bulk marks an action of a table actions row: the script enables it
	// while at least one row is selected.
	Bulk    bool
	Confirm *ConfirmView
}

// MenuView is one menu: the button that opens it and its actions. The
// destructive actions are listed last, apart from the routine ones.
type MenuView struct {
	Label       string
	Icon        string
	Items       []ActionView
	Destructive []ActionView
}

// The icon names the Go files of the components use more than once. Every
// name Icon draws is listed by IconNames.
const (
	glyphPlus     = "plus"
	glyphEdit     = "edit"
	glyphTrash    = "trash"
	glyphTerminal = "terminal"
	glyphRetry    = "retry"
	glyphClose    = "close"
)

// widthPercentMax is the widest a bar is drawn: the whole of its track.
const widthPercentMax = 100

// clampPercent answers a percentage inside 0 to 100.
func clampPercent(percent int) int {
	return min(max(percent, 0), widthPercentMax)
}

// widthClass answers the class that gives a bar its width. The stylesheet
// declares one class per whole percent, because the content security policy
// refuses a style attribute.
func widthClass(percent int) string {
	return "fleet-w-" + strconv.Itoa(clampPercent(percent))
}

// percentText answers a percentage as a page writes it.
func percentText(percent int) string {
	return strconv.Itoa(percent) + "%"
}

// countText answers a count as a page writes it.
func countText(count int) string {
	return strconv.Itoa(count)
}

// Percent answers part of whole as a whole percentage, rounded to the
// nearest, inside 0 to 100. It answers false when whole is not positive: the
// share of nothing is not known, and a bar then shows a dash, never 0%.
func Percent(part, whole float64) (int, bool) {
	if whole <= 0 {
		return 0, false
	}

	return clampPercent(int(part/whole*widthPercentMax + 0.5)), true
}

// The words of a relative time.
const (
	relativeNow    = "now"
	relativeSuffix = " ago"
)

// relativeNowMax is the longest age still written "now".
const relativeNowMax = 5 * time.Second

// RelativeTime answers how long ago something happened, in the largest whole
// unit: "now", "12 seconds ago", "1 minute ago", "59 minutes ago", "2 hours
// ago", "3 days ago". An age below zero, a clock that disagrees, is "now".
func RelativeTime(age time.Duration) string {
	if age < relativeNowMax {
		return relativeNow
	}

	if age < time.Minute {
		return relativeCount(int64(age/time.Second), "second")
	}

	if age < time.Hour {
		return relativeCount(int64(age/time.Minute), "minute")
	}

	if age < 24*time.Hour {
		return relativeCount(int64(age/time.Hour), "hour")
	}

	return relativeCount(int64(age/(24*time.Hour)), "day")
}

// relativeCount writes a count of one unit, plural beyond one.
func relativeCount(count int64, unit string) string {
	text := textbuf.New().Int(count).Byte(' ').Str(unit)
	if count != 1 {
		text.Byte('s')
	}

	return text.Str(relativeSuffix).String()
}
