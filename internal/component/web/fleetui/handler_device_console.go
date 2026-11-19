// Design: docs/architecture/fleet-interface.md -- the Console tab of a device and its command route
// Related: register_device_pages.go -- registers the page and the command route
// Related: page_device.templ -- deviceConsole, which draws the tab
// Related: component_terminal.templ -- Terminal and TerminalEntry
// Related: assets/fleet-terminal.js -- posts a command and appends the answer
//
// The Console tab is a command line on the device, through the hub. The
// terminal posts each command to the console route, which asks the source and
// answers one TerminalEntry. An offline device has no terminal. A device that
// grants no relay profile shows its own refusal in the terminal, word for
// word, with the line that grants the relay. Configuration mode is not
// offered: the hub writes the device's configuration.
//
// The prototype's source records no audit. The relay of fleet-9 audits every
// command on the hub and on the device.

package fleetui

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// consolePath is the prefix of the console route: a command for the device
// named name is posted to consolePath + name.
const consolePath = fleetPath + "console/"

// The bounds of a posted command.
const (
	// consoleCommandMax is the longest command the console sends on.
	consoleCommandMax = 512
	// consoleFormBytesMax is the most of a posted form the route reads.
	consoleFormBytesMax = 4096
)

// consoleField is the form field a command is posted as.
const consoleField = "command"

// consoleHelp is the command that lists the others. The Console tab asks it
// of a device that grants no relay profile, to show the device's refusal.
const consoleHelp = "help"

// consoleConfigure is the command that enters configuration mode, which the
// console does not offer.
const consoleConfigure = "configure"

// consoleSuggestions are the commands the Console tab suggests under the
// terminal. The sample source answers each, and its tests check that it does.
var consoleSuggestions = []string{
	consoleHelp, "show version", "show health", "show bgp peer list", "show interface brief", "show system cpu",
}

// ConsoleSuggestions answers the commands the Console tab suggests, for the
// test of a source that checks it answers them.
func ConsoleSuggestions() []string {
	return slices.Clone(consoleSuggestions)
}

// consoleURL answers the address a command for the device named name is
// posted to.
func consoleURL(name string) string {
	return consolePath + url.PathEscape(name)
}

// consoleContent builds the Console tab.
func consoleContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	view, err := deviceOf(request)
	if err != nil {
		return nil, err
	}

	device := &view.Device
	connection := &device.Connection
	if !connection.Connected {
		return StateHero(consoleOfflineHero(request, view)), nil
	}

	profile := "none granted"
	if connection.RelayGranted {
		profile = connection.RelayProfile
	}

	console := DeviceConsoleView{
		Terminal: TerminalView{
			Device:     device.Name,
			User:       request.Username,
			Profile:    profile,
			URL:        consoleURL(device.Name),
			ConfigHref: deviceTabAddress(request, tabDeviceConfig).String(),
		},
		InputID:     "fleet-terminal-command-" + device.Name,
		Suggestions: ConsoleSuggestions(),
	}

	if !connection.RelayGranted {
		// The device refuses before it runs anything, so asking it shows its
		// refusal in its own words.
		_, err := request.Source.Console(ctx, device.Name, request.Username, consoleHelp)
		if err != nil && FailureOf(err) != FailureRelayRefused {
			return StateHero(failureHeroView(request, err)), nil
		}
		if err != nil {
			refusal := refusalOf(err)
			console.Terminal.Refusal = refusal.Problem() + "\n" + refusal.Why()
			console.GrantLine = refusal.Fix()
		}
	}

	return deviceConsole(console), nil
}

// consoleOfflineHero answers the state of the Console tab of an offline
// device: no terminal, and when the device was last seen.
func consoleOfflineHero(request *PageRequest, view *DeviceView) StateHeroView {
	connection := &view.Device.Connection

	detail := "The hub has never seen it, so there is no console to open."
	if !connection.LastSeen.IsZero() {
		detail = textbuf.New().Str("It was last seen ").
			Str(RelativeTime(request.Snapshot.Now.Sub(connection.LastSeen))).Str(" (").
			Str(exactTime(connection.LastSeen)).Str("). Commands reach a device only while it is connected.").String()
	}

	return StateHeroView{
		Kind:     HeroOffline,
		Sentence: view.Device.Name + " is offline, so it has no console.",
		Detail:   detail,
		Action:   deviceAction(request, activityTabTasks, "Open the device's tasks", iconTasks),
	}
}

// consoleHandler answers a command posted from a device's terminal with one
// TerminalEntry. Every answer the operator can act on, a refusal included,
// is an entry under a success status, so the terminal shows it.
type consoleHandler struct {
	source Source
}

// newConsoleHandler builds the console route's handler over source.
func newConsoleHandler(source Source) *consoleHandler {
	return &consoleHandler{source: source}
}

// consoleAddressUnknown is the refusal for a console address that names no
// device.
func consoleAddressUnknown(path string) error {
	problem := textbuf.New().Str("The console address ").Quoted(path).Str(" names no device.").String()

	return opererr.Wrap(ErrAddressUnknown, problem,
		"A command is posted to the console of one device, and this address names none, so nothing ran.",
		"open the device's Console tab and type the command there")
}

func (handler *consoleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name, prefixed := strings.CutPrefix(r.URL.Path, consolePath)
	name = strings.TrimSuffix(name, "/")
	if !prefixed || name == "" || strings.Contains(name, "/") {
		answerConsoleRefusal(ctx, w, consoleAddressUnknown(r.URL.Path))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, consoleFormBytesMax)
	if err := r.ParseForm(); err != nil {
		answerConsoleRefusal(ctx, w, opererr.Wrap(err, "The command could not be read.",
			"The form the terminal posted was malformed or longer than "+textbuf.StringInt(consoleFormBytesMax)+" bytes, so nothing ran.",
			"type a shorter command"))
		return
	}

	address := Address{Kind: ObjectDevice, Device: name, Tab: tabDeviceConsole}
	request, err := NewPageRequest(ctx, handler.source, address, web.GetUsernameFromRequest(r))
	if err != nil {
		answerConsoleRefusal(ctx, w, err)
		return
	}

	command := strings.Join(strings.Fields(r.PostForm.Get(consoleField)), " ")
	renderAnswer(ctx, w, http.StatusOK, TerminalEntry(consoleEntry(ctx, request, command)))
}

// answerConsoleRefusal answers a request the route refused before any command
// ran, as an entry with the refusal's problem, why and fix under a success
// status: the terminal appends only an entry, so a plain text refusal would
// reach the operator as an answer with no output.
func answerConsoleRefusal(ctx context.Context, w http.ResponseWriter, err error) {
	renderAnswer(ctx, w, http.StatusOK, TerminalEntry(refusedEntry("", err)))
}

// consoleEntry runs one command on the request's device and answers the
// entry the terminal appends: the output, or the reason nothing ran.
func consoleEntry(ctx context.Context, request *PageRequest, command string) TerminalEntryView {
	view, err := deviceOf(request)
	if err != nil {
		return refusedEntry(command, err)
	}

	if command == "" {
		return TerminalEntryView{Output: "Type a command, or " + consoleHelp + " for the list of commands."}
	}

	if utf8.RuneCountInString(command) > consoleCommandMax {
		return refusedEntry(string([]rune(command)[:consoleCommandMax]), opererr.New("The command is too long.",
			"The console sends a command of at most "+textbuf.StringInt(consoleCommandMax)+" characters, so nothing ran.",
			"shorten the command"))
	}

	if first, _, _ := strings.Cut(command, " "); first == consoleConfigure {
		config := deviceTabAddress(request, tabDeviceConfig)

		return TerminalEntryView{Command: command, Output: textbuf.New().
			Str("Configuration mode is not offered in the console: the hub writes this device's configuration.\n").
			Str("Open the Config tab: ").Str(config.String()).String()}
	}

	answer, err := request.Source.Console(ctx, view.Device.Name, request.Username, command)
	if err != nil {
		return refusedEntry(command, err)
	}

	output := answer.Output
	if answer.Truncated {
		output = textbuf.New().Str(output).Str("\n[the output was cut at ").
			Int(ConsoleOutputBytesMax).Str(" bytes]").String()
	}

	return TerminalEntryView{Command: command, Output: output}
}

// refusedEntry answers the entry of a command that did not run, in the
// source's own words. A refused relay closes the terminal, because every next
// command would be refused the same way.
func refusedEntry(command string, err error) TerminalEntryView {
	refusal := refusalOf(err)
	output := textbuf.New().Str(refusal.Problem()).Byte('\n').Str(refusal.Why()).Str("\nTry: ").Str(refusal.Fix()).String()

	return TerminalEntryView{Command: command, Output: output, Refused: FailureOf(err) == FailureRelayRefused}
}
