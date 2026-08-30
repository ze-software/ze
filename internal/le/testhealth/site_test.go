package testhealth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: Render answers both generated artifacts for one checkout, and the
// vocabulary a second renderer needs to read them, while writing neither file.
//
// The website publishes quality/health/ from this package rather than from the
// committed snapshot, so the accessors have to describe the record Render
// answers: a question key no accessor names would render a metric under no
// heading, and a status no accessor names would sort it as calm. The method is
// to read the checkout once and hold every answer against the record.
func TestRenderAnswersTheArtifactsAndTheVocabularyThatReadsThem(t *testing.T) {
	root := checkoutRoot(t)
	pageBefore := readIfPresent(t, filepath.Join(root, filepath.FromSlash(Page)))
	latestBefore := readIfPresent(t, filepath.Join(root, filepath.FromSlash(Latest)))

	rendered, err := Render(root)
	if err != nil {
		t.Fatalf("render the site artifacts: %v", err)
	}

	var record struct {
		Metrics []map[string]any `json:"metrics"`
		History []map[string]any `json:"history"`
	}
	if err := json.Unmarshal([]byte(rendered.Record), &record); err != nil {
		t.Fatalf("the record is not JSON: %v", err)
	}
	if len(record.Metrics) == 0 {
		t.Fatal("the record carries no metric, so the published page would be empty")
	}
	if !strings.HasPrefix(rendered.Page, "# Testing Health") {
		t.Errorf("the page opens with %.20q, want the Testing Health heading", rendered.Page)
	}

	asked := map[string]bool{}
	for _, question := range Questions() {
		asked[question.Key] = true
	}
	known := map[string]bool{}
	for _, status := range Statuses() {
		known[status] = true
	}
	for _, metric := range record.Metrics {
		if !asked[jsonText(metric["question"])] {
			t.Errorf("metric %q asks %q, which Questions() does not name",
				jsonText(metric["key"]), jsonText(metric["question"]))
		}
		if !known[jsonText(metric["status"])] {
			t.Errorf("metric %q is %q, which Statuses() does not name",
				jsonText(metric["key"]), jsonText(metric["status"]))
		}
	}
	if first := Statuses()[0]; first != statusUnknown {
		t.Errorf("Statuses() opens with %q, want %q: a number nobody is computing outranks a bad one",
			first, statusUnknown)
	}
	for _, series := range TrendSeries() {
		if len(record.History) == 0 {
			break
		}
		if _, held := record.History[0][series.Key]; !held {
			t.Errorf("trend series %q is in no history sample", series.Key)
		}
	}

	if readIfPresent(t, filepath.Join(root, filepath.FromSlash(Page))) != pageBefore {
		t.Errorf("Render rewrote %s; it answers the artifacts and writes neither", Page)
	}
	if readIfPresent(t, filepath.Join(root, filepath.FromSlash(Latest))) != latestBefore {
		t.Errorf("Render rewrote %s; it answers the artifacts and writes neither", Latest)
	}
}

// checkoutRoot answers this repository's root, refusing a tree with no go.mod.
func checkoutRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve checkout root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("checkout root %s has no go.mod: %v", root, err)
	}
	return root
}

// readIfPresent answers a file's bytes, and the empty string when it is absent.
func readIfPresent(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path) //nolint:gosec // a test reads the checkout it runs in
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// jsonText answers a decoded JSON value as the string it holds, and the empty
// string for anything else.
func jsonText(value any) string {
	word, ok := value.(string)
	if !ok {
		return ""
	}
	return word
}
