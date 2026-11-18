// Design: docs/architecture/doctor-and-health-checks.md -- retained resource identity
// Related: resource_warning.go -- immutable original diagnostic context
package report

import (
	"fmt"
	"strings"
	"testing"
)

// Original structured coordinates survive publication and caller mutation.
// Long valid names need not consume the original failure's bounded text.
func TestResourceWarningEpisodeRetainsOriginalDetail(t *testing.T) {
	reset()
	t.Cleanup(reset)
	owner, err := NewResourceWarning("socket-original", "original-attempt")
	if err != nil {
		t.Fatal(err)
	}
	namespace := strings.Repeat("n", 255)
	access := strings.Repeat("a", 255)
	detail := map[string]any{"namespace": namespace, "access": access}
	episode, err := owner.NewEpisode("cleanup", "original close failed", "ze show warnings", detail)
	if err != nil {
		t.Fatal(err)
	}
	detail["namespace"] = "replacement"
	delete(detail, "access")
	if err := episode.Publish(); err != nil {
		t.Fatal(err)
	}
	entries := Warnings()
	if len(entries) != 1 {
		t.Fatalf("published issues=%d, want 1", len(entries))
	}
	if entries[0].Detail["namespace"] != namespace {
		t.Fatal("episode lost its original namespace coordinate")
	}
	if entries[0].Detail["access"] != access {
		t.Fatal("episode lost its original access coordinate")
	}
	entries[0].Detail["namespace"] = "reader replacement"
	if got := Warnings()[0].Detail["namespace"]; got != namespace {
		t.Fatalf("warning reader changed the retained coordinate: %v", got)
	}
}

// Existing field bounds refuse a detail map before acquiring an episode.
// A later valid publication on that same reservation remains useful.
func TestResourceWarningEpisodeRefusesOversizedDetailBeforeRetention(t *testing.T) {
	reset()
	t.Cleanup(reset)
	owner, err := NewResourceWarning("socket-original", "original-bounded-attempt")
	if err != nil {
		t.Fatal(err)
	}
	detail := make(map[string]any, maxDetailKeys+1)
	for index := range maxDetailKeys + 1 {
		detail[fmt.Sprintf("coordinate-%d", index)] = "original"
	}
	if _, err := owner.NewEpisode("cleanup", "original close failed", "ze show warnings", detail); err == nil {
		t.Fatal("oversized original detail acquired an episode")
	}
	if issues := Warnings(); len(issues) != 0 {
		t.Fatalf("refused detail published issues: %+v", issues)
	}
	episode, err := owner.NewEpisode("cleanup", "valid original cause", "ze show warnings", map[string]any{"namespace": "original"})
	if err != nil {
		t.Fatalf("refused detail consumed the original reservation: %v", err)
	}
	if err := episode.Publish(); err != nil {
		t.Fatal(err)
	}
	if got := Warnings()[0].Detail["namespace"]; got != "original" {
		t.Fatalf("valid publication lost original detail: %v", got)
	}
}
