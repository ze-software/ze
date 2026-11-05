// Design: docs/architecture/plugin/feature-gates.md -- canonical header presence never becomes a payload read.
package repoinventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommittedAssemblyCanonicalHeaderShadowsRefuse covers every admitted
// spelling and every Git object kind, after its own exact positive prerequisite.
func TestCommittedAssemblyCanonicalHeaderShadowsRefuse(t *testing.T) {
	for _, header := range []string{"textflag.h", "funcdata.h", "go_asm.h"} {
		for _, kind := range []string{"regular", "symlink", "populated-tree", "empty-tree", "gitlink"} {
			t.Run(header+"/"+kind, func(t *testing.T) {
				fixture := asmHeaderFixture(t, asmHeaderEmptyGo, "#include \""+header+"\"\n"+asmHeaderNumericBody)
				file := asmHeaderProbeDir + "/" + header
				switch kind {
				case "regular":
					write(t, fixture.root, file, "#define shadowed 1\n")
					commitInventoryFixture(t, fixture.root)
				case "symlink":
					if err := os.Symlink("unavailable-header", filepath.Join(fixture.root, file)); err != nil {
						t.Fatal(err)
					}
					commitInventoryFixture(t, fixture.root)
				case "populated-tree":
					write(t, fixture.root, file+"/member", "not a header\n")
					commitInventoryFixture(t, fixture.root)
				case "empty-tree":
					empty := asmGitInput(t, fixture.root, "", "mktree")
					asmHeaderCommitEntry(t, fixture, "040000 tree "+empty+"\t"+header)
				case "gitlink":
					head := strings.TrimSpace(committedGit(t, fixture.root, "rev-parse", "HEAD"))
					asmHeaderCommitEntry(t, fixture, "160000 commit "+head+"\t"+header)
				}
				asmHeaderRefusal(t, fixture, header, "shadows the compiler header")
			})
		}
	}
}

// TestCommittedAssemblyCanonicalHeaderPresenceNeedsNoPayload removes only a
// test-owned loose header object. Source capture must succeed without fetching
// it, and public membership must still reject its retained shadow presence.
func TestCommittedAssemblyCanonicalHeaderPresenceNeedsNoPayload(t *testing.T) {
	for _, header := range []string{"textflag.h", "funcdata.h", "go_asm.h"} {
		t.Run(header, func(t *testing.T) {
			fixture := asmHeaderFixture(t, asmHeaderEmptyGo, "#include \""+header+"\"\n"+asmHeaderNumericBody)
			file := asmHeaderProbeDir + "/" + header
			write(t, fixture.root, file, "only fixture-owned missing header payload: "+header+"\n")
			commitInventoryFixture(t, fixture.root)
			object := strings.TrimSpace(committedGit(t, fixture.root, "rev-parse", "HEAD:"+file))
			ownedObject := filepath.Join(fixture.root, ".git", "objects", object[:2], object[2:])
			if err := os.Remove(ownedObject); err != nil {
				t.Fatalf("remove only fixture-owned loose header blob: %v", err)
			}
			snapshot, err := ReadCommitted(t.Context(), fixture.root)
			if err != nil {
				t.Fatalf("header payload was fetched before presence admission: %v", err)
			}
			asmHeaderSelectedPaths(t, snapshot, asmHeaderPaths())
			plugins, err := snapshot.Plugins()
			if plugins != nil {
				t.Fatalf("missing shadow payload leaked partial plugins: %+v", plugins)
			}
			asmHeaderCause(t, err, header, "shadows the compiler header")
			asmHeaderRefusal(t, fixture, header, "shadows the compiler header")
		})
	}
}

// TestCommittedAssemblyCanonicalHeaderPresencePermittedSurvivors ensures the
// recognized basename is neither a global ban nor an eager per-package ban.
func TestCommittedAssemblyCanonicalHeaderPresencePermittedSurvivors(t *testing.T) {
	for _, header := range []string{"textflag.h", "funcdata.h", "go_asm.h"} {
		t.Run(header+"/different-selected-package", func(t *testing.T) {
			fixture := asmHeaderFixture(t, asmHeaderEmptyGo, "#include \""+header+"\"\n"+asmHeaderNumericBody)
			importFile := asmOrdinaryDir + "/header_other_import.go"
			otherFile := "internal/headerother/other.go"
			fixture.goFile(t, importFile, "package ordinary\nimport _ \"github.com/ze-software/ze/internal/headerother\"\n")
			fixture.goFile(t, otherFile, "package headerother\nfunc Dormant() {}\n")
			write(t, fixture.root, "internal/headerother/"+header, "#define unrelated 1\n")
			commitInventoryFixture(t, fixture.root)
			asmHeaderPositive(t, fixture, asmHeaderPaths(importFile, otherFile))
		})
		t.Run(header+"/never-included", func(t *testing.T) {
			fixture := asmHeaderFixture(t, asmHeaderEmptyGo, asmHeaderNumericBody)
			write(t, fixture.root, asmHeaderProbeDir+"/"+header, "#define unused 1\n")
			commitInventoryFixture(t, fixture.root)
			asmHeaderPositive(t, fixture, asmHeaderPaths())
		})
	}
}
