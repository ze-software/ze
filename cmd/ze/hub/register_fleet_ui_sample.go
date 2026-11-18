// Design: docs/architecture/fleet-interface.md -- ze_fleet_ui_sample review-build import
//
// Links the sample data source of the fleet interface, which turns the fleet
// routes on. ze_fleet_ui_sample is not a row of feature-gates.txt, because
// every tag in that file is set by every shipped build: only a review build
// names it. TestShippedZeLinksNoFleetSample (cmd/ze) proves no shipped flavor
// links the package.

//go:build ze_web && ze_fleet_ui && ze_fleet_ui_sample

package hub

import (
	_ "github.com/ze-software/ze/internal/component/web/fleetui/sample"
)
