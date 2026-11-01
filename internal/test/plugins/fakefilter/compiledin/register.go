// Design: docs/plugin-development/handlers.md -- route filters, the one evaluator both transports call
// Related: ../fakefilter.go -- the SDK-only plugin this registers
//
// Package compiledin registers fakefilter with ze, as a filter plugin compiled
// into the tree (spec-filter-wire-0-umbrella AC-35, R-13 bounds "compiled
// in"). It holds the only code of the plugin that reaches past the SDK: the
// registry entry, its env keys, and its logger. The plugin itself stays in
// package fakefilter, so the ze-fakefilter binary links it without any of
// this.
package compiledin

import (
	"net"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/test/plugins/fakefilter"
	fakefilteryang "github.com/ze-software/ze/internal/test/plugins/fakefilter/compiledin/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// envTypeString is the env.EnvEntry Type of the trigger's text keys.
const envTypeString = "string"

func init() {
	env.MustRegister(env.EnvEntry{Key: fakefilter.EnvRejectPrefix, Type: envTypeString,
		Description: "Test-only: fakefilter rejects the routes that announce this prefix", Private: true})
	env.MustRegister(env.EnvEntry{Key: fakefilter.EnvStopAfter, Type: "int", Default: "0",
		Description: "Test-only: fakefilter stops after judging this many routes (0 never)", Private: true})
	env.MustRegister(env.EnvEntry{Key: fakefilter.EnvStoppedFile, Type: envTypeString,
		Description: "Test-only: file fakefilter creates once it has stopped", Private: true})
	env.MustRegister(env.EnvEntry{Key: fakefilter.EnvRestart, Type: "bool", Default: "false",
		Description: "Test-only: fakefilter declares the restart failure policy", Private: true})
	env.MustRegister(env.EnvEntry{Key: fakefilter.EnvInsert, Type: "bool", Default: "false",
		Description: "Test-only: fakefilter declares its filter insertion at Stage 1", Private: true})
	env.MustRegister(env.EnvEntry{Key: fakefilter.EnvRespawnFile, Type: envTypeString,
		Description: "Test-only: file fakefilter creates once its restarted run has started", Private: true})

	reg := registry.Registration{
		Name:        fakefilter.Name,
		Description: "Test-only SDK filter plugin, compiled in, with a reject prefix and a stop trigger (spec-filter-wire-0-umbrella AC-33, AC-35)",
		RunEngine:   runEngine,
		// The Stage 1 declaration: config load knows fakefilter serves filters.
		Filters: fakefilter.FilterDecls(),
		// The insertion config load applies: its condition names the leaf
		// fakefilteryang adds, so it lands only where a peer sets it.
		FilterInsertions: fakefilter.FilterInsertions(),
		YANG:             fakefilteryang.ZeFakefilterConfYANG,
	}
	reg.CLIHandler = func(args []string) int {
		return cli.RunPlugin(cli.BaseConfig(&reg), args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: " + fakefilter.Name + " registration failed: " + err.Error())
	}
}

// runEngine runs the plugin with the trigger the environment asks for, over
// the connection the daemon gives it (internal) or the TLS connect-back that
// `ze plugin fakefilter` opens (forked).
func runEngine(conn net.Conn) int {
	log := slogutil.Logger(fakefilter.Name)
	tr, err := fakefilter.TriggerFrom(env.Get)
	if err != nil {
		log.Error("fakefilter: read the trigger", "error", err)
		return 1
	}
	return fakefilter.Run(sdk.NewWithConn(fakefilter.Name, conn), tr, log)
}
