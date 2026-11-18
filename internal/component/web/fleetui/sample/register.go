// Design: docs/architecture/fleet-interface.md -- sample data source registration
// Related: source.go -- the sample estate this file registers
//
// Linking this package is what turns the fleet routes on: they mount only when
// a data source is registered. cmd/ze/hub/register_fleet_ui_sample.go is the
// only importer, under the ze_fleet_ui_sample tag.

package sample

import "github.com/ze-software/ze/internal/component/web/fleetui"

func init() {
	fleetui.MustRegisterSource(&estate{})
}
