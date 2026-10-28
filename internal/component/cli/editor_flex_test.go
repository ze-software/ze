// Design: docs/architecture/config/syntax.md -- Flex file replay and editor replacement.
// Related: editor_commands.go -- SetValue and storeLeaf.

package cli

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestEditorSetReplacesReloadedFlexValues exercises the real editor mutation and
// save after loading each persisted form, retaining only the replacement value.
//
// PREVENTS: Tree.Set changing the scalar while the Flex printer still emits the
// earlier multi-values retained by the file parser.
func TestEditorSetReplacesReloadedFlexValues(t *testing.T) {
	schema := config.NewSchema()
	schema.Define("scope", config.Container(config.Field("action", config.Flex())))
	for _, tc := range []struct {
		name    string
		content string
	}{
		{"set", "set scope action zulu\nset scope action alpha\nset scope action zulu\n"},
		{"set-meta", "#alice set scope action zulu\n#alice set scope action alpha\n#alice set scope action zulu\n"},
		{"hierarchical", "scope { action zulu; action alpha; action zulu; }\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, meta, err := parseConfigWithFormat(tc.content, schema)
			if err != nil {
				t.Fatalf("load persisted input: %v", err)
			}
			scope := tree.GetContainer("scope")
			if scope == nil {
				t.Fatal("scope missing after load")
			}
			if got := scope.GetMultiValues("action"); !slices.Equal(got, []string{"zulu", "alpha", "zulu"}) {
				t.Fatalf("loaded action values = %q, want [zulu alpha zulu]", got)
			}
			ed := &Editor{
				schema:          schema,
				tree:            tree,
				meta:            meta,
				treeValid:       true,
				originalContent: tc.content,
				workingContent:  tc.content,
			}
			if err := ed.SetValue([]string{"scope"}, "action", "replacement value"); err != nil {
				t.Fatalf("replace Flex value: %v", err)
			}
			want := "set scope action \"replacement value\"\n"
			if got := config.SerializeSet(ed.Tree(), schema); got != want {
				t.Errorf("set output after replacement = %q, want %q", got, want)
			}
			var saved bytes.Buffer
			ed.SetStdoutSink(&saved)
			if _, err := ed.Save(); err != nil {
				t.Fatalf("save replacement: %v", err)
			}
			reread, _, err := parseConfigWithFormat(saved.String(), schema)
			if err != nil {
				t.Fatalf("reload saved replacement %q: %v", saved.String(), err)
			}
			if reread.GetContainer("scope") == nil {
				t.Fatalf("scope missing from saved output %q", saved.String())
			}
			if got := reread.GetContainer("scope").GetMultiValues("action"); !slices.Equal(got, []string{"replacement value"}) {
				t.Errorf("saved action values = %q, want [replacement value]; output %q", got, saved.String())
			}
		})
	}
}

// TestEditorSetFlexKeepsInactiveComments guards the existing replacement
// contract by retaining the whole-leaf inactive marker and statement comment.
func TestEditorSetFlexKeepsInactiveComments(t *testing.T) {
	schema := config.NewSchema()
	schema.Define("scope", config.Container(config.Field("action", config.Flex())))
	tree, err := config.NewParser(schema).Parse("scope {\n# keep action comment\naction zulu;\naction alpha;\n}")
	if err != nil {
		t.Fatalf("parse commented Flex values: %v", err)
	}
	scope := tree.GetContainer("scope")
	if scope == nil {
		t.Fatal("scope missing after parse")
	}
	scope.SetLeafInactive("action", true)
	ed := &Editor{schema: schema, tree: tree, treeValid: true}
	if err := ed.SetValue([]string{"scope"}, "action", "replacement"); err != nil {
		t.Fatalf("replace inactive Flex value: %v", err)
	}
	if !scope.IsLeafInactive("action") {
		t.Error("replacement activated the Flex leaf")
	}
	if got := config.Serialize(tree, schema); !strings.Contains(got, "# keep action comment") {
		t.Errorf("replacement lost the statement comment: %q", got)
	}
}

// TestEditorAdoptContestedFlexDoesNotMultiply proves a real draft rewrite keeps
// one assignment per competing owner while moving the adopted session twice.
func TestEditorAdoptContestedFlexDoesNotMultiply(t *testing.T) {
	schema := config.NewSchema()
	schema.Define("action", config.Flex())
	dir := t.TempDir()
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	path := filepath.Join(dir, "config.conf")
	if err := store.WriteFile(path, []byte("set action base\n"), 0o600); err != nil {
		t.Fatalf("write committed config: %v", err)
	}
	input := "#alice @ssh set action first\n#bob @ssh set action second\n"
	if err := store.WriteFile(DraftPath(path), []byte(input), 0o600); err != nil {
		t.Fatalf("write contested draft: %v", err)
	}
	ed := &Editor{schema: schema, store: store, originalPath: path}
	oldSession := "alice@ssh"
	for _, origin := range []string{"reconnected", "again"} {
		ed.session = NewEditSession("alice", origin)
		if err := ed.AdoptSession(oldSession); err != nil {
			t.Fatalf("adopt %s: %v", oldSession, err)
		}
		persisted, err := store.ReadFile(DraftPath(path))
		if err != nil {
			t.Fatalf("read adopted draft: %v", err)
		}
		if got := strings.Count(string(persisted), "set action "); got != 2 {
			t.Errorf("adoption wrote %d assignments, want two: %q", got, persisted)
		}
		_, meta, err := config.NewSetParser(schema).ParseWithMeta(string(persisted))
		if err != nil {
			t.Fatalf("parse adopted draft: %v", err)
		}
		for _, owner := range []struct {
			session string
			value   string
		}{
			{ed.session.ID, "first"},
			{"bob@ssh", "second"},
		} {
			entries := meta.SessionEntries(schema, owner.session)
			if len(entries) != 1 {
				t.Fatalf("owner %s has %d entries, want one", owner.session, len(entries))
			}
			if entries[0].Entry.Value != owner.value {
				t.Errorf("owner %s value = %q, want %q", owner.session, entries[0].Entry.Value, owner.value)
			}
		}
		oldSession = ed.session.ID
	}
}
