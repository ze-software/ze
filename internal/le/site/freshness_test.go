// Related: freshness.go -- the warning and refresh these cases drive
//
// VALIDATES: owner decision 2026-10-08. Before the site build publishes, it
// names every supported feature carrying a stale recorded run, one line per
// run, and says how to re-record; with `refresh` it re-records first, reports
// a refresh that recorded nothing, and warns only for what is still stale.
// Nothing is written when no feature is stale.
// PREVENTS: a site published on old evidence with nobody told, and a build
// that blocks on a prompt or runs tests nobody asked it to.

package site

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/feature"
)

var staleWidget = feature.StaleFeature{Feature: "widget",
	Stale: []string{"S1: test/plugin/widget.ci stale: older than 30 days (recorded 2026-10-07)"}}

func neverRecord(t *testing.T) staleRecorder {
	t.Helper()
	return func([]feature.StaleFeature) []feature.StaleFeature {
		t.Fatal("a build without refresh re-recorded a feature")
		return nil
	}
}

func TestFreshnessWarnsOfEachStaleSupportedRun(t *testing.T) {
	var warn bytes.Buffer
	stale, refreshed, err := freshness(&warn, false,
		func() ([]feature.StaleFeature, error) { return []feature.StaleFeature{staleWidget}, nil }, neverRecord(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 {
		t.Fatalf("stale %v, want widget", stale)
	}
	if refreshed != nil {
		t.Fatalf("refreshed %v without refresh", refreshed)
	}
	text := warn.String()
	want := "warning: supported feature widget publishes on a stale run: " + staleWidget.Stale[0]
	if !strings.Contains(text, want) {
		t.Fatalf("warning %q does not name the stale run", text)
	}
	if !strings.Contains(text, "`./le site build refresh`") {
		t.Fatalf("warning %q does not offer the refresh", text)
	}
}

func TestFreshnessIsSilentWithNothingStale(t *testing.T) {
	var warn bytes.Buffer
	_, _, err := freshness(&warn, true,
		func() ([]feature.StaleFeature, error) { return []feature.StaleFeature{}, nil }, neverRecord(t))
	if err != nil {
		t.Fatal(err)
	}
	if warn.Len() != 0 {
		t.Fatalf("wrote %q with no stale feature", warn.String())
	}
}

func TestFreshnessRefreshReRecordsFirst(t *testing.T) {
	judged := 0
	judge := func() ([]feature.StaleFeature, error) {
		judged++
		if judged == 1 {
			return []feature.StaleFeature{staleWidget, {Feature: "gadget", Stale: []string{"S2: lab/x stale"}}}, nil
		}
		return []feature.StaleFeature{{Feature: "gadget", Stale: []string{"S2: lab/x stale"}}}, nil
	}
	var recorded []string
	record := func(due []feature.StaleFeature) []feature.StaleFeature {
		answered := make([]feature.StaleFeature, 0, len(due))
		for _, entry := range due {
			recorded = append(recorded, entry.Feature)
			entry.Refreshed = entry.Feature == "widget"
			if !entry.Refreshed {
				entry.Refusal = "the scenario failed"
			}
			answered = append(answered, entry)
		}
		return answered
	}
	var warn bytes.Buffer
	stale, refreshed, err := freshness(&warn, true, judge, record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(recorded, ",") != "widget,gadget" {
		t.Fatalf("recorded %v, want widget and gadget", recorded)
	}
	if len(refreshed) != 2 {
		t.Fatalf("refreshed %v, want both", refreshed)
	}
	if len(stale) != 1 {
		t.Fatalf("stale after refresh %v, want gadget alone", stale)
	}
	text := warn.String()
	if strings.Contains(text, "feature widget publishes") {
		t.Fatalf("a refreshed feature is still warned: %q", text)
	}
	if !strings.Contains(text, "refresh of feature gadget recorded nothing: the scenario failed") {
		t.Fatalf("warning %q does not report the refused refresh", text)
	}
	if strings.Contains(text, "`./le site build refresh`") {
		t.Fatalf("after a refresh the warning %q offers the same refresh again", text)
	}
}

func TestFreshnessStopsOnAJudgeError(t *testing.T) {
	var warn bytes.Buffer
	_, _, err := freshness(&warn, false,
		func() ([]feature.StaleFeature, error) { return nil, errors.New("features/x.md: no Meta") }, neverRecord(t))
	if err == nil {
		t.Fatal("a declaration the check cannot read was published past")
	}
}
