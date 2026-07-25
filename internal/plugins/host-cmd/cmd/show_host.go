// Design: plan/learned/631-host-0-inventory.md — hardware inventory detection

package cmd

import (
	"encoding/json"
	"errors"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func RegisterShowHost() {
	names := host.SectionNames()
	regs := make([]pluginserver.RPCRegistration, 0, len(names))
	for _, name := range names {
		section := name
		regs = append(regs, pluginserver.RPCRegistration{
			WireMethod: "ze-show:host-" + section,
			Handler: func(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
				return dispatchHostSection(section)
			},
		})
	}
	pluginserver.RegisterRPCs(regs...)
}

func dispatchHostSection(section string) (*plugin.Response, error) {
	data, err := host.DetectSection(section)
	if err != nil {
		if errors.Is(err, host.ErrUnknownSection) {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  "unknown host section; valid: " + host.SectionList(),
			}, nil
		}
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error propagated via Response
	}
	b, jsonErr := json.Marshal(data)
	if jsonErr != nil {
		return nil, jsonErr
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.RawJSON(b)}, nil
}
