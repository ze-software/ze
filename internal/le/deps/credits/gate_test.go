package credits

import (
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

// headOf answers the fixture's files after an update, as a parent commit
// holds them: the generated data is read back from the tree update wrote.
func headOf(t *testing.T, files map[string]string, in inputs) map[string]string {
	t.Helper()
	root := writeTree(t, files)
	generate(t, root, in)
	head := maps.Clone(files)
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(DataRel)), func(full string, item os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if item.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, full)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(full) //nolint:gosec // a fixture file this test wrote
		if err != nil {
			return err
		}
		head[filepath.ToSlash(rel)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("read the generated data: %v", err)
	}
	return head
}

// commitOf builds the change a commit makes: head is the parent's files,
// carried the files the commit adds or changes, removed the ones it deletes.
func commitOf(head, carried map[string]string, removed ...string) changecheck.Change {
	side := func(files map[string]string) changecheck.Side {
		return func(rel string) (string, bool, error) {
			text, present := files[rel]
			return text, present, nil
		}
	}
	paths := slices.Sorted(maps.Keys(carried))
	return changecheck.Change{Paths: paths, Removed: removed, Before: side(head), After: side(carried)}
}

// judge runs the gate and fails the test when it could not judge.
func judge(t *testing.T, change changecheck.Change) string {
	t.Helper()
	refusal, err := judgeCommit(change)
	if err != nil {
		t.Fatalf("the gate could not judge: %v", err)
	}
	return refusal
}

// requireGateRefusal fails unless refusal mentions want and ends with the
// fix command alone on its last line.
func requireGateRefusal(t *testing.T, refusal, want string) {
	t.Helper()
	if !strings.Contains(refusal, want) {
		t.Fatalf("the gate's refusal does not mention %q:\n%s", want, refusal)
	}
	lines := strings.Split(refusal, "\n")
	if strings.TrimSpace(lines[len(lines)-1]) != updateCommand {
		t.Errorf("the refusal's last line is %q, want the command alone:\n%s", lines[len(lines)-1], refusal)
	}
}

// importing is webSource with one more import.
func importing(pkg string) string {
	return strings.Replace(webSource, `import "example.com/ze/internal/core/credits"`,
		"import (\n\t\"example.com/ze/internal/core/credits\"\n\t_ \""+pkg+"\"\n)", 1)
}

// TestGateRefusesANewlyLinkedModuleWithNoCredit asks whether a commit whose
// shippable Go file adds an import of a vendored module the data does not
// credit is refused, naming the module (AC-18). Method: the web package
// imports gadget, and the commit carries nothing else.
func TestGateRefusesANewlyLinkedModuleWithNoCredit(t *testing.T) {
	head := headOf(t, fixtureFiles(), fixtureInputs())
	refusal := judge(t, commitOf(head, map[string]string{webFile: importing(gadget)}))
	requireGateRefusal(t, refusal, "import "+gadget+", which it does not credit")
}

// TestGateRefusesAChangedLicenceFileWithStaleData asks whether a commit that
// carries a changed licence file of a credited work, with the data still
// holding the old bytes, is refused naming the file (AC-18).
func TestGateRefusesAChangedLicenceFileWithStaleData(t *testing.T) {
	head := headOf(t, fixtureFiles(), fixtureInputs())
	refusal := judge(t, commitOf(head, map[string]string{widgetLicence: mitText + "An added line.\n"}))
	requireGateRefusal(t, refusal, "changes "+widgetLicence+", a licence file of "+widget)
}

// TestGateRefusesARemovedLicenceFileTheDataStillCopies asks whether a commit
// that removes a licence file of a credited work, while the data it leaves
// still copies that file, is refused naming the file and the work. Method:
// the commit removes widget's LICENSE and carries nothing else.
func TestGateRefusesARemovedLicenceFileTheDataStillCopies(t *testing.T) {
	head := headOf(t, fixtureFiles(), fixtureInputs())
	refusal := judge(t, commitOf(head, map[string]string{}, widgetLicence))
	requireGateRefusal(t, refusal, "removes "+widgetLicence+", which the generated data still copies for "+widget)
}

// TestGatePassesACommitThatOnlyVendorsAModule asks whether the commit that
// adds a module to go.mod, go.sum and vendor/ alone passes: nothing imports
// it yet (AC-18, permitted survivor).
func TestGatePassesACommitThatOnlyVendorsAModule(t *testing.T) {
	files := fixtureFiles()
	delete(files, "vendor/"+gadget+"/LICENSE")
	delete(files, "vendor/"+gadget+"/g.go")
	head := headOf(t, files, fixtureInputs())
	carried := map[string]string{
		"go.mod":                        head["go.mod"] + "\nrequire github.com/acme/gadget v1.0.0\n",
		"go.sum":                        "github.com/acme/gadget v1.0.0 h1:x\n",
		"vendor/modules.txt":            vendorModules,
		"vendor/" + gadget + "/LICENSE": mitText,
		"vendor/" + gadget + "/g.go":    "package gadget\n",
	}
	if refusal := judge(t, commitOf(head, carried)); refusal != "" {
		t.Fatalf("the gate refused a commit that only vendors a module:\n%s", refusal)
	}
}

// TestGateIgnoresAnUncreditedModuleAnUnrelatedCommitLeavesAlone asks whether
// a module already imported at HEAD without a credit refuses a commit that
// does not add an import of it (AC-18, permitted survivor: the tree check
// reports it).
func TestGateIgnoresAnUncreditedModuleAnUnrelatedCommitLeavesAlone(t *testing.T) {
	files := fixtureFiles()
	files[webFile] = importing(gadget)
	head := headOf(t, files, fixtureInputs())
	edited := strings.Replace(head[webFile], "func init() {", "// init declares the work.\nfunc init() {", 1)
	if refusal := judge(t, commitOf(head, map[string]string{webFile: edited})); refusal != "" {
		t.Fatalf("the gate refused a commit that adds no import:\n%s", refusal)
	}
}

// TestGateAcceptsANewModuleOnceDeclaredAndRegenerated asks whether the
// refused commit passes once it carries the declaration naming the module and
// the data regenerated from it: the repair the refusal names works.
func TestGateAcceptsANewModuleOnceDeclaredAndRegenerated(t *testing.T) {
	head := headOf(t, fixtureFiles(), fixtureInputs())
	files := fixtureFiles()
	files[webFile] = importing(gadget)
	files[DeclarationRel] = strings.Replace(files[DeclarationRel], `"linked": []`,
		`"linked": [{"module": "github.com/acme/gadget", "licence": "MIT", "copyright": "Acme Widgets", "source": "https://github.com/acme/gadget"}]`, 1)
	after := headOf(t, files, fixtureInputs(flavorLinks{Flavor: "distro/linux", Modules: []string{gadget, widget}}))
	carried := map[string]string{webFile: after[webFile], DeclarationRel: after[DeclarationRel]}
	for rel, content := range after {
		if strings.HasPrefix(rel, DataRel+"/") {
			carried[rel] = content
		}
	}
	if _, held := carried[path.Join(DataRel, RecordName)]; !held {
		t.Fatal("the regenerated fixture holds no record")
	}
	if refusal := judge(t, commitOf(head, carried)); refusal != "" {
		t.Fatalf("the gate refused a declared and regenerated module:\n%s", refusal)
	}
}
