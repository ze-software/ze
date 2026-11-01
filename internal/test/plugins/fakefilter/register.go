// Design: docs/plugin-development/handlers.md -- route filters, the one evaluator both transports call
// Related: fakefilter.go -- the SDK-only plugin this registers

package fakefilter

import (
	"net"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The trigger's keys. Each is read when the plugin starts, so a forked
// `ze plugin fakefilter` reads them from the environment it inherits.
const (
	envRejectPrefix = "ze.fakefilter.reject-prefix"
	envStopAfter    = "ze.fakefilter.stop-after"
	envStoppedFile  = "ze.fakefilter.stopped-file"
)

func init() {
	env.MustRegister(env.EnvEntry{Key: envRejectPrefix, Type: "string",
		Description: "Test-only: fakefilter rejects the routes that announce this prefix", Private: true})
	env.MustRegister(env.EnvEntry{Key: envStopAfter, Type: "int", Default: "0",
		Description: "Test-only: fakefilter stops after judging this many routes (0 never)", Private: true})
	env.MustRegister(env.EnvEntry{Key: envStoppedFile, Type: "string",
		Description: "Test-only: file fakefilter creates once it has stopped", Private: true})

	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only SDK filter plugin, compiled in, with a reject prefix and a stop trigger (spec-filter-wire-0-umbrella AC-33, AC-35)",
		RunEngine:   runEngine,
		// The Stage 1 declaration: config load knows fakefilter serves filters.
		Filters: filterDecls(),
	}
	reg.CLIHandler = func(args []string) int {
		return cli.RunPlugin(cli.BaseConfig(&reg), args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: " + Name + " registration failed: " + err.Error())
	}
}

// runEngine runs the plugin with the trigger the environment asks for.
func runEngine(conn net.Conn) int {
	tr := trigger{
		rejectPrefix: env.Get(envRejectPrefix),
		stopAfter:    env.GetInt64(envStopAfter, 0),
		stoppedFile:  env.Get(envStoppedFile),
	}
	return run(conn, tr, slogutil.Logger(Name))
}
