// Design: docs/architecture/exabgp-bridge.md -- an API route naming a watchdog is the watchdog's
// Related: bridge_lines.go -- ScriptLines.dispatch hands a watchdog's route to bgp-watchdog

package bridge

import (
	"errors"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The bgp-watchdog plugin's command for routes a script gives a watchdog
// (internal/component/bgp/plugins/watchdog, server.go). The bridge only names
// it: the plugin owns the groups, their state, sending, and exit cleanup.
const watchdogGiveVerb = "request bgp watchdog give"

// errWatchdogOwner refuses a script name the plugin's command line cannot
// carry as one word.
var errWatchdogOwner = errors.New("the process name holds a space, so the bgp-watchdog plugin cannot name it " +
	"as the owner of a route given to a watchdog; rename the endpoint in the bus config")

// watchdogGiveCommand is the bgp-watchdog command that gives one announce to
// its watchdog, as ExaBGP's announce_route gives the route to
// add_to_rib_watchdog with its owner since c43f90f79 (configuration.py,
// rib/outgoing.py at the pin):
//
//	request bgp watchdog give <name> owner <script> selector <selector> [withdraw]
//	    announce <n> <n words of update text> withdrawal <update text>
//
// The announce is counted rather than ended by a keyword, because update text
// has no word that cannot appear in it. Both texts are the builder's own,
// without their `send bgp <selector>` head: the plugin sends them to the
// selector itself. owner is the script's endpoint name, so bgp-watchdog
// withdraws and forgets the route on participant exit.
func watchdogGiveCommand(owner, selector string, command Command) (string, error) {
	if strings.ContainsAny(owner, " \t") {
		return "", errWatchdogOwner
	}
	head := "send bgp " + selector + " "
	announce, cut := strings.CutPrefix(command.Text, head)
	if !cut {
		panic("BUG: a route given to a watchdog was not written as send bgp <selector>: " + command.Text)
	}
	withdrawal, cut := strings.CutPrefix(command.Withdrawal, head)
	if !cut {
		panic("BUG: the withdrawal of a route given to a watchdog was not written as send bgp <selector>: " + command.Withdrawal)
	}
	words := strings.Fields(announce)

	var tb textbuf.Buffer
	tb.Str(watchdogGiveVerb).Byte(' ').Str(command.Watchdog).
		Str(" owner ").Str(owner).
		Str(" selector ").Str(selector)
	if command.Held {
		tb.Str(" withdraw")
	}
	tb.Str(" announce ").Str(strconv.Itoa(len(words)))
	for _, word := range words {
		tb.Byte(' ').Str(word)
	}
	tb.Str(" withdrawal ").Str(withdrawal)
	return tb.String(), nil
}
