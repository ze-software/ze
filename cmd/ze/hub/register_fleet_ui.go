// Design: docs/architecture/fleet-interface.md -- ze_fleet_ui hub registration gating
//
// Links the fleet web interface into the hub. The package registers its routes
// with the web route registry from init(), and no generator discovers it, so
// this blank import is the whole hub-side gate. The constraint names ze_web as
// well: the fleet package nests under the web package and imports it, so a
// ze_fleet_ui build without ze_web must drop the fleet package rather than drag
// the web package back in.

//go:build ze_web && ze_fleet_ui

package hub

import (
	_ "github.com/ze-software/ze/internal/component/web/fleetui"
)
