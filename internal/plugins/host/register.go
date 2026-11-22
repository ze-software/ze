// Register the offline fallback for `show host` with the command registry.
// `show host [section]` is a daemon command (host-cmd plugin); this fallback
// serves the same hardware inventory in-process (host.DetectSection) when no
// daemon is reachable, so an operator can read it before the daemon is up.
// Blank-imported by internal/component/plugin/all for its side effects: the
// generated composition root names every package that registers a command.

package host

import (
	"github.com/ze-software/ze/internal/component/command"
)

func init() {
	// Offline fallback only: consulted solely when the daemon did not answer,
	// so it never shadows the daemon `show host` command. RunShow takes the
	// section as its first arg, matching the tokens after `show host`.
	command.MustRegisterOfflineFallback("show host", runShowOffline)
	// show host trend is not an inventory section: its state lives only in
	// the daemon, so its fallback says the daemon is needed and why the daemon
	// did not answer (trend_offline.go).
	command.MustRegisterOfflineFallback("show host trend", runShowTrendOffline)
}

// runShowOffline answers show host from this process. The inventory is read
// from this host whoever asks, so the answer needs nothing from the cause: the
// route that ran the fallback tells the operator the answer is this process's
// and why the daemon gave none (command.InvokeOfflineFallback).
func runShowOffline(validated command.ValidatedArgs, _ command.DaemonUnanswered) int {
	return RunShow(validated)
}
