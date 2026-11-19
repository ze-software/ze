//go:build ze_web && ze_fleet_ui

// Design: docs/contributing/gh-pages.md -- the fleet interface demonstration: the host page
// Related: render.go -- the render that writes this page beside the frame

package sitefleetdemo

import (
	"html"
	"strings"

	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

const (
	// commitShown is how many characters of the commit the host bar shows,
	// as the looking glass replay's provenance line does.
	commitShown = 10
	// dateShown is the length of the calendar date at the start of an ISO
	// 8601 commit date.
	dateShown = len("2006-01-02")

	// The site's player and stylesheet, relative to demoDirectory: the host
	// page is opened from a review server whose root may not be the output's,
	// so it names nothing from the root.
	hostPlayer     = "../../assets/lg-replay.js"
	hostStylesheet = "../../assets/site.css"
)

// hostPage answers the page that mounts the frame: a thin bar that says what
// the demonstration is and where it comes from, above the frame, which fills
// the rest of the window. It carries a no-index marker and none of the site's
// navigation, because it is not part of the site.
func hostPage(provenance sitelgreplay.Provenance) []byte {
	var page strings.Builder
	page.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>Fleet interface: recorded demonstration</title>
<link rel="stylesheet" href="` + hostStylesheet + `">
<style>
html, body { height: 100%; margin: 0; }
body.fleet-demo { display: flex; flex-direction: column; }
.fleet-demo__bar { flex: none; margin: 0; padding: 0.4rem 0.8rem; font-size: 0.9rem; }
.fleet-demo__stage, .fleet-demo__stage .lg-replay__player { flex: 1; display: flex; flex-direction: column; min-height: 0; margin: 0; max-width: none; }
.fleet-demo__stage .lg-replay__frame { flex: 1; width: 100%; height: 100%; border: 0; }
</style>
<script src="` + hostPlayer + `" defer></script>
</head>
<body class="fleet-demo">
<p class="fleet-demo__bar">`)
	page.WriteString(html.EscapeString(hostBarText(provenance)))
	page.WriteString(`</p>
<div class="lg-replay fleet-demo__stage">
<div class="lg-replay__player" data-lg-replay data-lg-replay-fill data-lg-replay-frame="` + frameFile + `">
<p class="lg-replay__fallback">The demonstration runs in your browser and needs JavaScript.</p>
</div>
</div>
</body>
</html>
`)

	return []byte(page.String())
}

// hostBarText answers the sentence of the host bar: what the page is, the
// commit and the date it was recorded from, and what it does not show.
func hostBarText(provenance sitelgreplay.Provenance) string {
	commit := provenance.Commit
	if len(commit) > commitShown {
		commit = commit[:commitShown]
	}
	date := provenance.CommitDate
	if len(date) > dateShown {
		date = date[:dateShown]
	}
	if provenance.TreeDirty {
		commit += " with uncommitted changes"
	}

	return "Fleet interface: recorded demonstration. Sample data, recorded from commit " + commit + " of " + date +
		". Live updates are not recorded. Not published."
}
