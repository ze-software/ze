//go:build ze_ospf

// Design: docs/guide/config-editor.md -- load merge and SSH commit.
// Related: cmd_validate.go -- the standalone file validation entry point.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	editorcli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestEmptyOSPFAreaValidationSSHParity loads an empty area into an SSH-owned
// session and commits through the actual Model command, alongside standalone
// file validation. Explicit-normal and undeclared-area controls pin acceptance
// to the area declaration; adjacent nonempty entries must survive publication.
// VALIDATES: spec-web-cli-config-one-off-defects AC-21.
func TestEmptyOSPFAreaValidationSSHParity(t *testing.T) {
	for _, tc := range []struct {
		name  string
		area  string
		valid bool
	}{
		{name: "empty area", area: "area 0.0.0.0 { }", valid: true},
		{name: "explicit normal area", area: "area 0.0.0.0 { area-type normal; }", valid: true},
		{name: "undeclared area", valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const base = "ospf { router-id 192.0.2.1; }\n"
			fragment := `ospf {
	router-id 192.0.2.1;
	areas {
		` + tc.area + `
		area 0.0.0.1 { area-type normal; default-cost 17; }
	}
	interfaces {
		interface ac21-empty { area 0.0.0.0; network-type point-to-point; }
		interface ac21-other { area 0.0.0.1; network-type point-to-point; cost 23; }
	}
}`
			// The file command sees the same full candidate that load-merge
			// constructs from the already committed router-id and this fragment.
			file := filepath.Join(t.TempDir(), "ospf.conf")
			if err := os.WriteFile(file, []byte(fragment), 0o600); err != nil {
				t.Fatal(err)
			}
			wantCode := exitOK
			if !tc.valid {
				wantCode = exitInvalid
			}
			if code := cmdValidate([]string{"-q", file}); code != wantCode {
				t.Fatalf("standalone validation exit = %d, want %d", code, wantCode)
			}
			if !tc.valid {
				refusal := ValidateContent(fragment, file)
				if refusal == nil {
					t.Fatal("undeclared-area control has no validation refusal")
				}
				if !strings.Contains(refusal.Error(), "interface references undeclared area") {
					t.Fatalf("standalone control failed for another reason: %v", refusal)
				}
			}
			store := storage.CreateMemory()
			t.Cleanup(func() {
				if err := store.Close(); err != nil {
					t.Error(err)
				}
			})
			const name = "ospf.conf"
			if err := store.WriteFile(name, []byte(base), 0o600); err != nil {
				t.Fatal(err)
			}
			ed, err := editorcli.NewEditorWithStorage(store, name)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := ed.Close(); err != nil {
					t.Error(err)
				}
			})
			ed.SetSession(editorcli.NewEditSession("ac21", "ssh"))
			loaded, err := ed.ParseLoad(fragment, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := ed.LoadMerge(nil, loaded); err != nil {
				t.Fatal(err)
			}
			model, err := editorcli.NewModel(ed, editorcli.FilesystemAuthorityDaemon)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := model.Close(); err != nil {
					t.Error(err)
				}
			})
			model.SetInput("commit now")
			next, command := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			model, ok := next.(editorcli.Model)
			if !ok {
				t.Fatalf("Enter returned %T", next)
			}
			if command == nil {
				t.Fatal("commit Enter scheduled no command")
			}
			next, _ = model.Update(command())
			model, ok = next.(editorcli.Model)
			if !ok {
				t.Fatalf("commit result returned %T", next)
			}
			if err := model.Error(); err != nil {
				t.Fatal(err)
			}
			committed, err := store.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.valid {
				if !strings.Contains(model.StatusMessage(), "commit blocked:") {
					t.Fatalf("undeclared area was not blocked: %s", model.StatusMessage())
				}
				if !strings.Contains(model.ViewportContent(), "interface references undeclared area") {
					t.Fatalf("wrong refusal: %s", model.ViewportContent())
				}
				if !bytes.Equal(committed, []byte(base)) {
					t.Fatal("refused commit changed committed configuration")
				}
				if !ed.Dirty() {
					t.Fatal("refused commit discarded the candidate")
				}
				return
			}
			if !strings.Contains(model.StatusMessage(), "committed") {
				t.Fatalf("accepted file did not commit through SSH: %s\n%s", model.StatusMessage(), model.ViewportContent())
			}
			if ed.Dirty() {
				t.Fatal("accepted commit left pending changes")
			}
			tree, err := config.ParseTreeForValidation(string(committed))
			if err != nil {
				t.Fatal(err)
			}
			ospf := tree.GetContainer("ospf")
			if ospf == nil {
				t.Fatal("commit lost OSPF")
			}
			areas := ospf.GetContainer("areas")
			if areas == nil {
				t.Fatal("commit lost areas")
			}
			if areas.GetList("area")["0.0.0.0"] == nil {
				t.Fatal("commit lost the accepted area declaration")
			}
			other := areas.GetList("area")["0.0.0.1"]
			if other == nil {
				t.Fatal("commit lost the adjacent nonempty area")
			}
			if cost, _ := other.Get("default-cost"); cost != "17" {
				t.Fatalf("adjacent area default-cost = %q", cost)
			}
			interfaces := ospf.GetContainer("interfaces")
			if interfaces == nil {
				t.Fatal("commit lost interfaces")
			}
			for key, area := range map[string]string{"ac21-empty": "0.0.0.0", "ac21-other": "0.0.0.1"} {
				entry := interfaces.GetList("interface")[key]
				if entry == nil {
					t.Fatalf("commit lost interface %s", key)
				}
				if got, _ := entry.Get("area"); got != area {
					t.Fatalf("interface %s area = %q, want %q", key, got, area)
				}
			}
		})
	}
}
