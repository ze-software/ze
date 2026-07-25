// VALIDATES: AC-9 (fakeenrich enricher registered for "show test enrich" command)
// PREVENTS: in-process enrichment regression

package fakeenrich

import (
	"testing"

	"github.com/ze-software/ze/internal/core/show"
)

func TestFakeEnrichRegistered(t *testing.T) {
	base := map[string]any{"id": "test-1"}
	show.Enrich(Command, base)

	if base["fakeenrich"] != "present" {
		t.Fatalf("expected fakeenrich=present, got %v", base["fakeenrich"])
	}
	if base["id"] != "test-1" {
		t.Fatal("base key was removed")
	}
}

func TestFakeEnrichBrief(t *testing.T) {
	base := map[string]any{"id": "test-1"}
	show.EnrichBrief(Command, base)

	if base["fakeenrich"] != "present" {
		t.Fatalf("expected fakeenrich=present in brief, got %v", base["fakeenrich"])
	}
}
