// Helpers the cases share. Nothing here asserts; each function answers a fact
// a case then judges.

package testhealth

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeFixtureFile puts one file into a temporary tree.
func writeFixtureFile(t *testing.T, root, rel, body string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("fixture file %s: %v", rel, err)
	}
}

// writeFixtureBaseline puts a sensitivity baseline into a temporary tree.
func writeFixtureBaseline(t *testing.T, root string, assertNothing, tagOrphan int) {
	t.Helper()

	floors := object{}
	floors.set("assert-nothing", assertNothing)
	floors.set("tag-orphan", tagOrphan)
	body, err := dumpIndented(floors)
	if err != nil {
		t.Fatalf("encoding the baseline: %v", err)
	}
	writeFixtureFile(t, root, Baseline, body+"\n")
}

// ratioData builds the data one quality metric ratchets on.
func ratioData(key string, num, den int) object {
	data := object{}
	data.set(key, ratio(num, den))
	return data
}

// readFixtureFile answers one file of a temporary tree.
func readFixtureFile(t *testing.T, root, rel string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- a fixture path
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(body)
}

// readFixtureBaseline answers the baseline a temporary tree now holds.
func readFixtureBaseline(t *testing.T, root string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Baseline))) // #nosec G304 -- a fixture path
	if err != nil {
		t.Fatalf("reading the baseline: %v", err)
	}
	return string(body)
}

// fieldTags answers the JSON key of every field of a payload that declares one.
//
// It reads the STRUCT rather than an encoded document, because a field the
// encoder omitted when empty would be invisible in the document and is still a
// key `| json` can render.
func fieldTags(payload any) []string {
	shape := reflect.TypeOf(payload)
	tags := make([]string, 0, shape.NumField())
	for field := range shape.Fields() {
		tag := field.Tag.Get("json")
		name := tag
		for position, char := range tag {
			if char == ',' {
				name = tag[:position]
				break
			}
		}
		if name == "" || name == "-" {
			continue
		}
		tags = append(tags, name)
	}
	return tags
}
