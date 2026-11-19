// Design: docs/architecture/fleet-interface.md -- sample data source registration
// Related: ../source.go -- the sample estate this file registers
// Related: ../script.go -- the script the held setting stops from running on its timer
//
// Linking this package is what turns the fleet routes on: they mount only when
// a data source is registered. cmd/ze/hub/register_fleet_ui_sample.go is the
// only importer, under the ze_fleet_ui_sample tag. The sample package itself
// registers nothing, so a tool that records the pages can link it without
// making it the build's source.

// Package register makes the sample estate the build's fleet data source.
package register

import (
	"time"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/component/web/fleetui/sample"
	"github.com/ze-software/ze/internal/core/env"
)

// scriptHeldKey is the setting that starts the sample script held. It is
// registered here, so only a build with the ze_fleet_ui_sample tag knows it.
const scriptHeldKey = "ze.web.fleet.sample.held"

// The setting is env-only, by the exception ai/rules/config.md keeps for
// debug instrumentation: it exists only in a review build, which no operator
// configures, and a YANG leaf would put it in the schema of every build.
var _ = env.MustRegister(env.EnvEntry{
	Key:     scriptHeldKey,
	Type:    env.TypeBool,
	Default: "false",
	Description: "Review builds only (ze_fleet_ui_sample): start the sample script held, so the estate " +
		"changes only when Step is pressed. Env-only: a development switch no shipped build has, so no YANG leaf.",
})

func init() {
	fleetui.MustRegisterSource(newSource(time.Now))
}

// newSource builds the sample source over clock, held from the start when the
// environment asks for it.
func newSource(clock func() time.Time) sample.Source {
	source := sample.New(clock)
	holdWhenAsked(source)

	return source
}

// holdWhenAsked holds script when ze.web.fleet.sample.held is true. It runs
// before the source is registered, so before any page subscribes and starts
// the timer: a browser test then sees exactly the events it stepped. A value
// that is not a boolean leaves the script timed, and env records it for
// ze doctor (doctor-env-invalid-value).
func holdWhenAsked(script sample.Stepper) {
	if env.GetBool(scriptHeldKey, false) {
		script.Hold()
	}
}
