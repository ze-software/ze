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

// All of the `show host *` handlers delegate to host.DetectSection,
// which is the single source of truth for valid section names and
// their detector bodies (internal/component/host/inventory.go).
// See rules/derive-not-hardcode.md — this file used to carry a
// parallel `hostHandlers` map; it was removed in favor of deriving.

// handleShowHostCPU returns the CPU section of the host inventory.
func handleShowHostCPU(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return dispatchHostSection("cpu")
}

// handleShowHostNIC returns the NIC section.
func handleShowHostNIC(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return dispatchHostSection("nic")
}

// handleShowHostDMI returns the DMI/SMBIOS section.
func handleShowHostDMI(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return dispatchHostSection("dmi")
}

// handleShowHostMemory returns the memory section.
func handleShowHostMemory(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return dispatchHostSection("memory")
}

// handleShowHostThermal returns the thermal section.
func handleShowHostThermal(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return dispatchHostSection("thermal")
}

// handleShowHostStorage returns the storage section.
func handleShowHostStorage(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return dispatchHostSection("storage")
}

// handleShowHostKernel returns the kernel section.
func handleShowHostKernel(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return dispatchHostSection("kernel")
}

// handleShowHostAll returns the full inventory.
func handleShowHostAll(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return dispatchHostSection("all")
}

// dispatchHostSection is the shared body of every host handler. It
// asks the host package to run the named detector and wraps the
// value in a Response. Detection errors surface as StatusError per
// the exact-or-reject rule: operators get a clear message rather
// than a silent empty response.
//
// The unknown-section branch is defense-in-depth: in normal
// operation pluginserver's WireMethod dispatch rejects unregistered
// methods before the handler runs, so the branch is only reachable
// via a programmer error (typo in the section argument passed from
// a `handleShowHost*` helper). Leaving it in makes that mistake
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
