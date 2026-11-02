package policyview

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// TestMetadataFieldRegistration proves a registered metadata field reads the
// key of the route's metadata map, writes its registered JSON type, answers
// an absent key absent, refuses a value of another Go type, and is set only
// through an import session's map. Method: the engine's own rebuilt field
// (a flag) through a view over each kind of session.
//
// VALIDATES: spec-filter-wire-6-stage-filters AC-5, D-8.
// PREVENTS: a metadata read answering false for a key nobody set, and an
// export filter writing the map every destination shares.
func TestMetadataFieldRegistration(t *testing.T) {
	rebuilt, ok := MetadataNamed("rebuilt")
	if !ok || rebuilt.Kind() != MetadataFlag {
		t.Fatalf("MetadataNamed(rebuilt) = %+v, %v; want a registered flag", rebuilt, ok)
	}
	meta := map[string]any{"rebuilt": true}
	tests := []struct {
		name    string
		session Session
		want    string
	}{
		{"import map holds the key", Session{}.WithImportMetadata(meta), `{"rebuilt":true}`},
		{"export map holds the key", Session{}.WithExportMetadata(meta), `{"rebuilt":true}`},
		{"absent key is absent", Session{}.WithImportMetadata(map[string]any{}), `{}`},
		{"no map is absent", Session{}, `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var view View
			view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), tt.session)
			got, err := AppendFields(nil, &view, []string{"rebuilt"})
			if err != nil {
				t.Fatalf("AppendFields: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("fields\n got %s\nwant %s", got, tt.want)
			}
		})
	}

	var view View
	view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), Session{}.WithImportMetadata(map[string]any{"rebuilt": "yes"}))
	if _, err := AppendFields(nil, &view, []string{"rebuilt"}); !errors.Is(err, ErrMalformed) {
		t.Errorf("a string under a flag key answered %v, want ErrMalformed", err)
	}

	written := map[string]any{}
	view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), Session{}.WithImportMetadata(written))
	if err := view.SetMetadata(rebuilt, true); err != nil || written["rebuilt"] != true {
		t.Errorf("import SetMetadata = %v, map %v; want the key written", err, written)
	}
	shared := map[string]any{}
	view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), Session{}.WithExportMetadata(shared))
	if err := view.SetMetadata(rebuilt, true); !errors.Is(err, ErrNoMetadataMap) || len(shared) != 0 {
		t.Errorf("export SetMetadata = %v, map %v; want ErrNoMetadataMap and the map untouched", err, shared)
	}
}
