// Design: docs/architecture/fleet-interface.md -- sample data source registration
// Related: ../source.go -- the sample estate this file registers
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
)

func init() {
	fleetui.MustRegisterSource(sample.New(time.Now))
}
