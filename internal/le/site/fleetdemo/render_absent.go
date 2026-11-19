//go:build !ze_web || !ze_fleet_ui

// Design: docs/contributing/gh-pages.md -- the fleet interface demonstration: a le without the fleet interface
// Related: render.go -- render, in a le that links the fleet interface

package sitefleetdemo

import (
	"context"

	"github.com/ze-software/ze/internal/core/opererr"
)

// render refuses: this le was built without the fleet interface, so there are
// no fleet pages to record. le is built with every tag of feature-gates.txt,
// so only a build made by hand without ze_web or ze_fleet_ui answers this.
func render(context.Context, demonstration) (rendered, error) {
	return rendered{}, opererr.New("This le was built without the fleet interface, so it has no fleet pages to record",
		"The demonstration records the fleet package's own handlers, which the ze_web and ze_fleet_ui tags link, and this le was built without them.",
		"./le site fleet-demo render")
}
