// Design: plan/spec-host-0-inventory.md — hardware inventory detection
// Related: show.go — show verb RPC registration
// Related: system.go — show system cpu/memory/uptime are enriched with
//   a `hardware` subsection sourced from the same host package

package show

import (
	"errors"

	"codeberg.org/thomas-mangin/ze/internal/component/host"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// All `show host *` RPCs are registered programmatically from
// host.SectionNames() at init() time. Adding a new section requires
// one entry in `internal/component/host/inventory.go:sectionDetectors`
// plus one YANG container; both the RPC registration and its handler
// are generated automatically. See `rules/derive-not-hardcode.md` —
// this file used to carry 8 hand-written one-line handlers plus 8
// inline registration entries in show.go; both were collapsed into
// the loop below.
func init() {
	names := host.SectionNames()
	regs := make([]pluginserver.RPCRegistration, 0, len(names))
	for _, name := range names {
		section := name // capture for the closure
		regs = append(regs, pluginserver.RPCRegistration{
			WireMethod: "ze-show:host-" + section,
			Handler: func(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
				return dispatchHostSection(section)
			},
		})
	}
	pluginserver.RegisterRPCs(regs...)
}

// dispatchHostSection is the shared body every generated host handler
// calls. It asks the host package to run the named detector and wraps
// the value in a Response. Detection errors surface as StatusError
// per the exact-or-reject rule: operators get a clear message rather
// than a silent empty response.
//
// The unknown-section branch is defense-in-depth: in normal
// operation pluginserver's WireMethod dispatch rejects unregistered
// methods before the handler runs, so the branch is only reachable
// via a programmer error (typo in the section argument passed from
// the generated closure above). Leaving it in makes that mistake
// fail loudly with the canonical valid-sections list rather than a
// nil-bodied StatusDone.
func dispatchHostSection(section string) (*plugin.Response, error) {
	data, err := host.DetectSection(section)
	if err != nil {
		if errors.Is(err, host.ErrUnknownSection) {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   "unknown host section; valid: " + host.SectionList(),
			}, nil
		}
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error propagated via Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
}
