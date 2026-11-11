// Design: docs/architecture/site-facts.md -- the build reads recorded Go lines.
package site

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestPublishedGoLineFactsUseRecordedInventory verifies that a site build uses
// the committed sentinels, not a fresh scan of the fixture's different source.
func TestPublishedGoLineFactsUseRecordedInventory(t *testing.T) {
	paths := factsFixture(t)
	writeFixtureFile(t, filepath.Join(paths.Repository, "source.go"), "package source\n")
	facts, err := deriveSiteFacts(paths)
	if err != nil {
		t.Fatal(err)
	}
	if facts.Repo.GoSourceLines != 2_345_678 || facts.Repo.GoSourceLinesDisplay != "2.35 million" {
		t.Errorf("recorded source lines = %d/%q, want 2345678/2.35 million", facts.Repo.GoSourceLines, facts.Repo.GoSourceLinesDisplay)
	}
	if facts.Repo.GoTestLines != 3_456_789 || facts.Repo.GoTestLinesDisplay != "3.46 million" {
		t.Errorf("recorded test lines = %d/%q, want 3456789/3.46 million", facts.Repo.GoTestLines, facts.Repo.GoTestLinesDisplay)
	}
}

// TestRecordedGoLinesDistinguishMissingNegativeAndZero makes zero a valid
// measurement while refusing absent and negative committed measurements.
func TestRecordedGoLinesDistinguishMissingNegativeAndZero(t *testing.T) {
	for _, key := range []string{"repo.go_source_lines", "repo.go_test_lines", "repo.go_vendor_lines"} {
		for _, state := range []string{"missing", "missing-value", "null-value", "negative", "zero"} {
			t.Run(key+"/"+state, func(t *testing.T) {
				paths := factsFixture(t)
				var input struct {
					Facts map[string]any `json:"facts"`
				}
				if err := json.Unmarshal([]byte(readArtifact(t, paths.Source, "data/repo-facts.json")), &input); err != nil {
					t.Fatal(err)
				}
				value := 0
				if state == "negative" {
					value = -1
				}
				input.Facts[key] = map[string]any{"value": value, "source": "fixture"}
				if state == "missing-value" {
					input.Facts[key] = map[string]any{"source": "fixture"}
				}
				if state == "null-value" {
					input.Facts[key] = map[string]any{"value": nil, "source": "fixture"}
				}
				if state == "missing" {
					delete(input.Facts, key)
				}
				encoded, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				writeFixtureFile(t, filepath.Join(paths.Source, "data", repositoryFactsFile), string(encoded))
				var facts siteFacts
				facts.Sources = make(map[string]string)
				err = factsFromRepositoryFile(paths.Source, &facts)
				if state == "zero" {
					if err != nil {
						t.Fatalf("zero is a valid measurement: %v", err)
					}
					if key == "repo.go_source_lines" && facts.Repo.GoSourceLinesDisplay != "0" || key == "repo.go_test_lines" && facts.Repo.GoTestLinesDisplay != "0" {
						t.Error("zero did not publish a readable 0")
					}
					if key == "repo.go_vendor_lines" {
						content, err := json.Marshal(facts)
						if err != nil {
							t.Fatal(err)
						}
						var published map[string]any
						if err := json.Unmarshal(content, &published); err != nil {
							t.Fatal(err)
						}
						if shown, held := factValue(published, key+"_display"); !held || shown != "0" {
							t.Error("vendor zero did not publish a readable 0")
						}
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "./le site facts update") {
					t.Errorf("%s %s: got %v, want named refusal and update action", state, key, err)
				}
			})
		}
	}
}

// TestPublishedVendorGoLineFactsUseRecordedInventory checks the additive JSON
// schema and its raw/display values through the real snapshot producer.
func TestPublishedVendorGoLineFactsUseRecordedInventory(t *testing.T) {
	facts, err := deriveSiteFacts(factsFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	content, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	var published map[string]any
	if err := json.Unmarshal(content, &published); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"repo.go_vendor_lines":         "4567890",
		"repo.go_vendor_lines_display": "4.57 million",
	} {
		if got, held := factValue(published, key); !held || got != want {
			t.Errorf("published %s = %q (present %t), want %q", key, got, held, want)
		}
	}
}
