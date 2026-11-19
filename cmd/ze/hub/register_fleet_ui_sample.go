// Design: docs/architecture/fleet-interface.md -- ze_fleet_ui_sample review-build import
//
// Links the registration of the fleet interface's sample data source, which
// makes the sample estate the build's source and so turns the fleet routes
// on. ze_fleet_ui_sample is not a row of feature-gates.txt, because every
// tag in that file is set by every shipped build: only a review build
// names it. TestShippedZeLinksNoFleetSample (cmd/ze) proves no shipped flavor
// links the sample package or its registration.

//go:build ze_web && ze_fleet_ui && ze_fleet_ui_sample

package hub

import (
	_ "github.com/ze-software/ze/internal/component/web/fleetui/sample/register"
)
