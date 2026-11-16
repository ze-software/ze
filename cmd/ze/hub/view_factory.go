//go:build ze_ssh || ze_web

// Design: docs/architecture/command-ownership.md -- shared registered live-view factories.
// Related: session_factory.go, view_web.go -- authenticated surface bindings.

package hub

import (
	"github.com/ze-software/ze/internal/component/cli"
	pingcmd "github.com/ze-software/ze/internal/component/ping/cmd"
	"github.com/ze-software/ze/internal/component/plugin"
	traceroutecmd "github.com/ze-software/ze/internal/component/traceroute/cmd"
)

// cliExecutor retains response completion ownership at the Model writer.
func cliExecutor(executor func(string) (*plugin.RenderedResponse, error)) cli.CommandExecutor {
	return func(input string) (cli.CommandOutput, error) {
		rendered, err := executor(input)
		if rendered == nil {
			return cli.CommandOutput{}, err
		}
		return cli.CommandOutput{
			Text: rendered.Output, Warning: rendered.Response.WarningText(),
			TransportComplete: rendered.TransportComplete,
		}, err
	}
}

// injectViewFactories binds every registered view at this composition root.
// Web and SSH share this inventory; feature packages stay outside both adapters.
func injectViewFactories(m *cli.Model, executor func(string) (*plugin.RenderedResponse, error)) {
	for _, v := range cli.RegisteredViews() {
		switch v.Key {
		case cli.ViewKeyDashboard:
			m.SetViewFactory(v.Key, dashboardFactoryFromExecutor(executor))
		case cli.ViewKeyTraceroute:
			m.SetViewFactory(v.Key, cli.TracerouteFactory(traceroutecmd.NewTracerouteSession))
		case cli.ViewKeyPing:
			m.SetViewFactory(v.Key, cli.PingFactory(pingcmd.NewPingSession))
		}
	}
}

func dashboardFactoryFromExecutor(execute func(string) (*plugin.RenderedResponse, error)) cli.DashboardFactory {
	return func() (func() (string, error), error) {
		return func() (string, error) {
			rendered, err := execute("show bgp")
			if rendered != nil {
				defer rendered.TransportComplete()
				return rendered.Output, err
			}
			return "", err
		}, nil
	}
}
