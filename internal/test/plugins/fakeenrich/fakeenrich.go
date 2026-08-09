// Design: docs/architecture/plugin/show-enrichers.md -- in-process test enricher plugin

package fakeenrich

import (
	"encoding/json"
	"net"

	"github.com/ze-software/ze/internal/core/show"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	Name    = "fakeenrich"
	Command = "show test enrich"
)

func dispatchCommand(_, command string, _ []string, _ string) (string, any, error) {
	if command != Command {
		return "error", nil, nil
	}
	base := map[string]any{"source": "fakeenrich-handler"}
	show.Enrich(Command, base)
	raw, err := json.Marshal(base)
	if err != nil {
		return "error", nil, err
	}
	return "done", json.RawMessage(raw), nil
}

func runPlugin(conn net.Conn) int {
	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	p.OnExecuteCommand(dispatchCommand)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{{Name: Command}},
	}); err != nil {
		return 1
	}
	return 0
}
