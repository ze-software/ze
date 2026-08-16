// Design: docs/architecture/chaos-web-dashboard.md — the Freeze control
// Related: golden_test.go — chaosGoldenCases, the route population read here

package web

import (
	"regexp"
	"strings"
	"testing"
)

// chaosPollingTag matches the opening tag of an element that polls. htmx spells
// a poll one way, `hx-trigger="every <interval>"`, so the tag is what says an
// element repeats a request on its own.
var chaosPollingTag = regexp.MustCompile(`<[a-zA-Z][^>]*hx-trigger="every[^>]*>`)

// chaosTagID reads an opening tag's id.
var chaosTagID = regexp.MustCompile(`\sid="([^"]*)"`)

// chaosFreezeLeavesAlone names each polling element the Freeze control does NOT
// stop, and why. It is a record of what htmx 2 did, not a fresh decision: under
// htmx 2 only the elements carrying `[!window._frozen]` were frozen, and these
// four never did. A new poll that neither carries freezePoll nor appears here
// fails the check, because "does Freeze own this one" is a decision somebody has
// to make rather than a default to inherit.
var chaosFreezeLeavesAlone = map[string]string{
	"stats":           "the sidebar counters are the run's headline numbers and kept ticking under htmx 2",
	"events":          "the sidebar event feed kept ticking under htmx 2",
	"active-set-info": "the active-set summary kept ticking under htmx 2",
	"peer-grid":       "the peer grid kept ticking under htmx 2",
}

// VALIDATES: every poll the Freeze control owns carries freezePoll, and the
// layout carries the listener that reads it.
// PREVENTS: a dead Freeze checkbox. htmx 2 read a condition inside the trigger
// (`every 500ms [!window._frozen]`); htmx 4 has no trigger filter, parses the
// interval and ignores the rest, so that spelling polls forever and the box
// tells the operator nothing. Measured in a browser against 4.0.0-beta6: the
// route-matrix panel issued 15 requests in 3 seconds with the box ticked, and
// none after the listener landed.
func TestFreezeStopsThePollsItOwns(t *testing.T) {
	t.Parallel()

	for _, c := range chaosGoldenCases {
		body := string(chaosGoldenServe(t, c))

		if strings.Contains(body, "_frozen]") {
			t.Errorf("case %q still carries the htmx 2 trigger condition; htmx 4 parses the interval and ignores it", c.Name)
		}

		for _, tag := range chaosPollingTag.FindAllString(body, -1) {
			if strings.Contains(tag, strings.TrimSpace(freezePoll)) {
				continue
			}

			id := ""
			if m := chaosTagID.FindStringSubmatch(tag); m != nil {
				id = m[1]
			}

			if _, known := chaosFreezeLeavesAlone[id]; !known {
				t.Errorf("case %q: %s polls, carries no %s, and is not listed in chaosFreezeLeavesAlone",
					c.Name, tag, strings.TrimSpace(freezePoll))
			}
		}
	}
}

// VALIDATES: the layout listens for the event that cancels a frozen poll, and
// reads the marker the panels carry.
// PREVENTS: the two halves drifting apart. A marker no listener reads, or a
// listener reading a marker no panel carries, leaves Freeze dead and leaves the
// markup looking converted.
func TestFreezeListenerReadsTheMarker(t *testing.T) {
	t.Parallel()

	var layout string
	for _, c := range chaosGoldenCases {
		if c.Name == "index" {
			layout = string(chaosGoldenServe(t, c))
		}
	}
	if layout == "" {
		t.Fatal("no index case in chaosGoldenCases; this check reads the full page")
	}

	// The listener cancels the request htmx 4 is about to make. Each of these is
	// load-bearing: the event is the one cancelable point before the fetch, the
	// source event tells a poll from a click, and the marker says which polls
	// Freeze owns.
	for _, needle := range []string{
		"htmx:before:request",
		"sourceEvent",
		"'every'",
		"data-freeze-poll",
		"preventDefault",
		"window._frozen",
	} {
		if !strings.Contains(layout, needle) {
			t.Errorf("the layout script does not carry %q", needle)
		}
	}

	if !strings.Contains(layout, `id="freeze-updates"`) {
		t.Error("the layout renders no Freeze control for the listener to read")
	}
}
