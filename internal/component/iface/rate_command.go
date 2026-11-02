// Design: docs/features/interfaces.md -- interface rate show + monitor handlers
// Related: rate.go -- the tracker this command reads
// Related: cmd/interface_rate.go -- the daemon handlers that forward here
//
// The rate tracker lives in the iface plugin's own process: runEngine starts
// it, and a plugin run external holds it in the subprocess. A daemon handler
// that read the tracker directly would read the daemon's own, never-started
// copy and answer "rate tracker not running" forever (ai/doors/system.md, "A
// plugin can run in another process"). So the plugin serves its rates as a
// command of its own, and every daemon surface forwards to it through the
// dispatcher, which reaches the plugin in either process.

package iface

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// RateCommand is the plugin command the iface plugin answers with its rate
// tracker's current rates. Without an argument it answers every interface as
// a list sorted by name; with one interface name it answers that interface.
//
// The name is the YANG path of `show interface rate`, so the dispatcher judges
// a forwarded call against the same leaves as the operator's own command.
const RateCommand = "show interface rate"

// errRateTrackerNotRunning is the refusal of a rate command that reaches a
// process where no tracker was started. runEngine starts the tracker before
// it serves any command, so only a call outside the plugin can meet it.
var errRateTrackerNotRunning = errors.New("rate tracker not running: the interface plugin has not started its rate sampling")

// ErrRatePluginAbsent is the refusal of a rate call that finds no process
// serving RateCommand: the iface plugin, which owns the tracker, is not
// running. The engine starts the plugin only for a configuration that holds
// its root section, so the refusal names that section and the commands that
// add it. It is built here, beside the root it names, so the hint cannot
// drift from the section that actually starts the plugin.
var ErrRatePluginAbsent = errors.New("the interface plugin, which measures interface rates, is not running. " +
	"Ze starts it only when the configuration holds an " + configRootInterface + " section. " +
	"To start it, add the section and commit: set " + configRootInterface + " backend netlink")

// commandDecls names the commands the iface plugin serves.
//
// It has two readers and MUST stay one function: init() puts it on the
// registry.Registration, which anything linking the composition root reads,
// and runEngine sends it in the Stage 1 registration a running daemon reads
// (docs/architecture/plugin/plugin-system.md, "The process boundary").
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{
			Name:      RateCommand,
			ShortHelp: "Show per-second traffic rates on your interfaces.",
		},
	}
}

// executeCommand answers a command the dispatcher routed to this plugin.
func executeCommand(command string, args []string) (string, any, error) {
	if !strings.EqualFold(command, RateCommand) {
		var tb textbuf.Buffer
		return "error", nil, errors.New(tb.Str("interface plugin serves no command ").Quoted(command).String())
	}
	data, err := rateAnswer(globalTracker.Load(), args)
	if err != nil {
		return "error", nil, err
	}
	return "done", data, nil
}

// rateAnswer is the answer of RateCommand over tracker: one interface when
// args names one, otherwise every interface sorted by name.
func rateAnswer(tracker *rateTracker, args []string) (any, error) {
	if tracker == nil {
		return nil, errRateTrackerNotRunning
	}
	if len(args) > 0 {
		name := args[0]
		rate, ok := tracker.get(name)
		if !ok {
			return nil, errors.New("interface not found: " + name)
		}
		return rate, nil
	}
	rates := tracker.snapshot()
	result := make([]InterfaceRate, 0, len(rates))
	for _, r := range rates {
		result = append(result, r)
	}
	slices.SortFunc(result, func(a, b InterfaceRate) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}
