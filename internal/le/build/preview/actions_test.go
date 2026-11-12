// VALIDATES: native preview answers stay useful with a large input population.
// PREVENTS: recursively printing the full manifest during preparation or check.
package buildpreview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

func TestPreviewCommandAnswersSummarizeInsteadOfDumpingInputs(t *testing.T) {
	root, deps := fixture(t)
	initial, err := deps.source(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	initial.Inputs = make([]Input, 10000)
	for index := range initial.Inputs {
		initial.Inputs[index] = Input{Path: fmt.Sprintf("runtime-population/file-%05d.go", index), SHA256: strings.Repeat("a", 64)}
	}
	initial.Digest = sourceDigest(inputDigest(initial.Inputs), initial.Toolchain, initial.Tags)
	deps.source = func(context.Context, string) (Source, error) { return initial, nil }
	output := filepath.Join(t.TempDir(), "preview")
	answers := []struct {
		name string
		run  func() (any, int)
	}{
		{"run", func() (any, int) { return runPreview(t.Context(), root, output, false, deps) }},
		{"check", func() (any, int) { return checkPreview(output, deps.inspect) }},
	}
	var original []byte
	for _, action := range answers {
		t.Run(action.name, func(t *testing.T) {
			answer, code := action.run()
			if code != 0 {
				t.Fatalf("%s command exited %d", action.name, code)
			}
			if _, full := answer.(Manifest); full {
				t.Fatal("command returned the full source manifest")
			}
			body, err := json.Marshal(answer)
			if err != nil {
				t.Fatal(err)
			}
			var data map[string]any
			if err := json.Unmarshal(body, &data); err != nil {
				t.Fatal(err)
			}
			if data["directory"] != output || data["manifest"] != filepath.Join(output, "manifest.json") || data["checksums"] != filepath.Join(output, "SHA256SUMS") {
				t.Fatalf("command omitted usable output paths: %s", body)
			}
			if data["version"] != previewVersion(initial) || data["channel"] != "preview" || data["published"] != false {
				t.Fatalf("command omitted preview identity: %s", body)
			}
			source, ok := data["source"].(map[string]any)
			if !ok || source["commit"] != initial.Commit || source["digest"] != initial.Digest || source["dirty"] != initial.Dirty || source["input-count"] != float64(len(initial.Inputs)) {
				t.Fatalf("command omitted bounded source identity: %s", body)
			}
			artifacts, ok := data["artifacts"].([]any)
			if !ok || len(artifacts) != 4 {
				t.Fatalf("command omitted artifact inventory: %s", body)
			}
			_, format, message := command.ProcessPipesDefaultFormatChecked("le build preview "+action.name, "table")
			if message != "" {
				t.Fatal(message)
			}
			rendered := format(string(body))
			for _, text := range []string{string(body), rendered} {
				if len(text) > 32768 || strings.Count(text, "\n") > 120 || strings.Contains(text, "runtime-population/") {
					t.Fatalf("command expanded source population: bytes=%d lines=%d", len(text), strings.Count(text, "\n"))
				}
				if !strings.Contains(text, previewVersion(initial)) || !strings.Contains(text, "manifest.json") {
					t.Fatal("rendered summary lost version or manifest location")
				}
			}
			stored, err := os.ReadFile(filepath.Join(output, "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			var manifest Manifest
			if err := json.Unmarshal(stored, &manifest); err != nil {
				t.Fatal(err)
			}
			if len(manifest.Source.Inputs) != len(initial.Inputs) {
				t.Fatal("concise answer discarded the full retained provenance")
			}
			if action.name == "run" {
				original = stored
			} else if string(stored) != string(original) {
				t.Fatal("checking rewrote the full manifest")
			}
		})
	}
}
