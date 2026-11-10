// Design: docs/architecture/plugin/plugin-system.md -- the registry seals after start-up
// Related: register.go -- the init() registration that must still succeed

// Package lateregister is the test-only plugin behind
// test/plugin/registry-sealed-after-start.ci. Once every daemon plugin is
// ready, its engine calls registry.Register for a second name, the run-time
// registration the seal exists to refuse, then asks the daemon for
// `show plugin list` and requests a shutdown. Each outcome is one stderr line
// the .ci reads; a line starting "lateregister: FAIL" is a failed assertion.
// Its init registers the plugin, so only a ze built with the zetest tag
// imports this package.
package lateregister

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Name is the plugin's registry name, registered from init().
const Name = "lateregister"

// lateName is the name the engine tries to register after start-up. The
// daemon's registry MUST NOT carry it.
const lateName = "lateregister-after-start"

// dispatchTimeout bounds each command the probe sends to the daemon.
const dispatchTimeout = 10 * time.Second

// runPlugin is the in-process entry point. It completes the five startup
// stages and runs the probe from OnAllPluginsReady, when the daemon has
// dispatched, and so sealed, long before.
func runPlugin(conn net.Conn) int {
	p := sdk.NewWithConn(Name, conn)
	defer p.Close() //nolint:errcheck // test plugin teardown, so a close failure changes no assertion

	p.OnAllPluginsReady(func() error {
		// The probe dispatches commands, whose answers arrive through this
		// event loop, so it runs on its own goroutine: one per plugin
		// lifetime, ended when the probe returns.
		go probe(p)
		return nil
	})
	if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
		logLine("lateregister: plugin run ended: " + err.Error())
		return 1
	}
	return 0
}

// probe attempts the late registration, reads the daemon's plugin list, and
// asks the daemon to stop, reporting each step on stderr.
func probe(p *sdk.Plugin) {
	attemptLateRegistration()
	checkPluginList(p)

	ctx, cancel := context.WithTimeout(context.Background(), dispatchTimeout)
	defer cancel()
	if _, _, err := p.DispatchCommand(ctx, "request shutdown"); err != nil {
		logLine("lateregister: FAIL request shutdown: " + err.Error())
	}
}

// attemptLateRegistration calls registry.Register after start-up and reports
// whether the seal refused it. The registration offered is a renamed copy of
// this plugin's own (Lookup answers a clone), so it is complete and valid in
// every field: the only reason left to refuse it is the seal. Building it from
// the copy rather than a second Registration literal also keeps this package
// at one plugin's declaration, which the plugin-declarations gate pairs.
func attemptLateRegistration() {
	own := registry.Lookup(Name)
	if own == nil {
		logLine("lateregister: FAIL its own registration is not in the registry")
		return
	}
	reg := *own
	reg.Name = lateName
	reg.Description = "registered after start-up; the seal MUST refuse it"
	err := registry.Register(reg)
	if err == nil {
		logLine("lateregister: FAIL registered " + lateName + " after start-up")
		return
	}
	if !errors.Is(err, registry.ErrRegistrySealed) {
		logLine("lateregister: FAIL late registration refused for another reason: " + err.Error())
		return
	}
	logLine("lateregister: late registration refused: " + err.Error())
}

// pluginList is the part of the `show plugin list` answer the probe reads.
type pluginList struct {
	Plugins []struct {
		Name string `json:"name"`
	} `json:"plugins"`
}

// checkPluginList asks the daemon for `show plugin list` and reports whether
// it carries the init() name and not the late one, so the daemon still
// answers after the refusal and the refusal changed nothing it lists.
func checkPluginList(p *sdk.Plugin) {
	ctx, cancel := context.WithTimeout(context.Background(), dispatchTimeout)
	defer cancel()
	status, data, err := p.DispatchCommand(ctx, "show plugin list")
	if err != nil {
		logLine("lateregister: FAIL show plugin list: " + err.Error())
		return
	}
	var list pluginList
	if err := json.Unmarshal(data, &list); err != nil {
		logLine("lateregister: FAIL show plugin list answered " + status + " with unreadable data: " + err.Error())
		return
	}
	carriesName, carriesLate := false, false
	for _, row := range list.Plugins {
		if row.Name == Name {
			carriesName = true
		}
		if row.Name == lateName {
			carriesLate = true
		}
	}
	if !carriesName {
		logLine("lateregister: FAIL show plugin list does not list " + Name)
		return
	}
	if carriesLate {
		logLine("lateregister: FAIL show plugin list lists " + lateName)
		return
	}
	logLine("lateregister: show plugin list lists " + Name + " and not " + lateName)
}

// logLine writes one line to stderr, which the daemon carries to the .ci.
func logLine(line string) {
	os.Stderr.WriteString(line + "\n") //nolint:errcheck // a lost line fails the .ci expectation that reads it
}
