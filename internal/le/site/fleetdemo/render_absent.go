//go:build !ze_web || !ze_fleet_ui

// Design: docs/contributing/gh-pages.md -- the fleet interface demonstration: a le without the fleet interface
// Related: render.go -- render, in a le that links the fleet interface
// Related: render_refusal.go -- the refusal this render answers

package sitefleetdemo

import (
	"context"
)

// render refuses: this le was built without the fleet interface, so there are
// no fleet pages to record. absentRefusal names why and the fix.
func render(_ context.Context, demo demonstration) (rendered, error) {
	return rendered{}, absentRefusal(demo.repository)
}
