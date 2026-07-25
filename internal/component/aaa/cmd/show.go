// Design: docs/architecture/api/commands.md -- show aaa accounting handler

// Package cmd registers engine-side RPC handlers for the AAA component's
// CLI surface. The handler reaches the accounting provider through
// aaa.AAAAccountingData() rather than importing the provider directly.
package cmd

import (
	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:aaa-accounting",
			Handler:    handleShowAAAAccounting,
		},
	)
}

func handleShowAAAAccounting(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	data := aaa.AAAAccountingData()
	if data == nil {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"dropped-records": uint64(0)}}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}
