package credits

import (
	"path"
	"slices"
	"strings"
	"testing"
)

// TestUpdateWritesWhatCheckAccepts asks whether the generator's output passes
// the check over a valid tree, and whether a second run changes nothing.
// Method: update the fixture, check it, update it again and read what the
// second run wrote and removed.
func TestUpdateWritesWhatCheckAccepts(t *testing.T) {
	root := writeTree(t, fixtureFiles())
	in := fixtureInputs()
	first := generate(t, root, in)
	if len(first.Written) == 0 {
		t.Fatal("the first update wrote nothing")
	}
	if report := check(t, root, in); len(report.Refusals) != 0 {
		t.Fatalf("check refused what update wrote:\n%s", report.Text())
	}
	second := generate(t, root, in)
	if len(second.Written) != 0 {
		t.Errorf("a second update rewrote %v", second.Written)
	}
	if len(second.Removed) != 0 {
		t.Errorf("a second update removed %v", second.Removed)
	}
}

// TestUpdateRemovesAFileNoWorkReferences asks whether a file under the data
// directory that no work references is removed by update. Method: plant a
// text before the first run.
func TestUpdateRemovesAFileNoWorkReferences(t *testing.T) {
	root := writeTree(t, fixtureFiles())
	stale := path.Join(TextsDir, "0000")
	writeFile(t, root, path.Join(DataRel, stale), "an old licence\n")
	report := generate(t, root, fixtureInputs())
	if !slices.Contains(report.Removed, stale) {
		t.Fatalf("update removed %v, want %s among them", report.Removed, stale)
	}
}

// TestUpdateRecordsTheGroupsInDeclarationOrder asks whether the record keeps
// the order in which the declaration names its groups, which the works,
// sorted by name, lose. Method: the fixture declares the curated group
// "Widgets" before the works group "Web assets", the reverse of their sorted
// order, and the record must list them as declared.
func TestUpdateRecordsTheGroupsInDeclarationOrder(t *testing.T) {
	root := writeTree(t, fixtureFiles())
	out, refusals, err := planTree(root, fixtureInputs())
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(refusals) != 0 {
		t.Fatalf("plan refused the fixture: %+v", refusals)
	}
	want := []string{"Widgets", "Web assets"}
	if !slices.Equal(out.record.Groups, want) {
		t.Fatalf("record groups = %q, want %q", out.record.Groups, want)
	}
}

// TestCheckRefusesAToolchainWorkWithAVersion asks whether a work marked as
// the Go toolchain that also declares a version is refused: the binary reads
// that version from its build information, so the declared one would never
// print. Method: mark the fixture's work toolchain and keep its version.
func TestCheckRefusesAToolchainWorkWithAVersion(t *testing.T) {
	files := fixtureFiles()
	files[DeclarationRel] = strings.Replace(declarationJSON("Acme Widgets"), `"version": "1.0",`, `"version": "1.0", "toolchain": true,`, 1)
	root := writeTree(t, files)
	report := check(t, root, fixtureInputs())
	requireRefusal(t, report, thing, "is marked toolchain and declares version 1.0")
}

// toolchainEntry is the end of the fixture declaration's toolchain work, as
// the declaration spells it, for the tests that change it.
const toolchainEntry = `"files": ["` + goLicence + `"]}`

// TestCheckAcceptsAToolchainWorkWithNoCarrier asks whether the toolchain work
// passes the check with no carried-by and no package declaring it: every
// binary lists it without one, so the carrier rule a work owes does not bind
// it. Method: the plain fixture, whose Go work has neither, generated and
// checked.
func TestCheckAcceptsAToolchainWorkWithNoCarrier(t *testing.T) {
	files := fixtureFiles()
	if !strings.Contains(files[DeclarationRel], toolchainEntry) || strings.Contains(files[webFile], `DeclarePresent("`+goWork+`")`) {
		t.Fatal("the fixture's toolchain work has a carrier")
	}
	root := writeTree(t, files)
	in := fixtureInputs()
	generate(t, root, in)
	if report := check(t, root, in); len(report.Refusals) != 0 {
		t.Fatalf("check refused a toolchain work with no carrier:\n%s", report.Text())
	}
}

// TestCheckRefusesAToolchainWorkWithACarrier asks whether a toolchain work
// that lists carried-by packages is refused: no package declares it, so the
// list is a second declaration nothing reads. Method: give the fixture's Go
// work a carried-by list.
func TestCheckRefusesAToolchainWorkWithACarrier(t *testing.T) {
	files := fixtureFiles()
	files[DeclarationRel] = strings.Replace(files[DeclarationRel], toolchainEntry,
		`"files": ["`+goLicence+`"], "carried-by": ["`+webPackage+`"]}`, 1)
	root := writeTree(t, files)
	in := fixtureInputs()
	generate(t, root, in)
	requireRefusal(t, check(t, root, in), goWork, "is marked toolchain and lists "+webPackage+" under carried-by")
}

// TestCheckRefusesADeclarationOfTheToolchainWork asks whether a package that
// declares the toolchain work present is refused at the call, which the
// binary would refuse at start. Method: the web package declares Go too.
func TestCheckRefusesADeclarationOfTheToolchainWork(t *testing.T) {
	files := fixtureFiles()
	files[webFile] = strings.Replace(webSource, `credits.DeclarePresent("thing")`,
		`credits.DeclarePresent("thing")
	credits.DeclarePresent("`+goWork+`")`, 1)
	root := writeTree(t, files)
	in := fixtureInputs()
	generate(t, root, in)
	report := check(t, root, in)
	for _, refusal := range report.Refusals {
		if strings.Contains(refusal.Problem, "declares the toolchain work \""+goWork+"\" present") && refusal.Fix == updateCommand {
			return
		}
	}
	t.Fatalf("check accepted a declaration of the toolchain work:\n%s", report.Text())
}

// TestCheckRefusesADeclarationWithNoToolchainWork asks whether a declaration
// that marks no work as the Go toolchain is refused: every binary lists Go,
// and the binary refuses a record without it. Method: cut the fixture's Go
// work out of the declaration.
func TestCheckRefusesADeclarationWithNoToolchainWork(t *testing.T) {
	files := fixtureFiles()
	declared := files[DeclarationRel]
	start := strings.Index(declared, `,
    {"name": "`+goWork+`"`)
	end := strings.Index(declared, toolchainEntry)
	if start < 0 || end < start {
		t.Fatal("the fixture declaration no longer holds the Go work last")
	}
	files[DeclarationRel] = declared[:start] + declared[end+len(toolchainEntry):]
	if strings.Contains(files[DeclarationRel], "toolchain") {
		t.Fatal("the cut declaration still marks a toolchain work")
	}
	root := writeTree(t, files)
	in := fixtureInputs()
	generate(t, root, in)
	requireRefusal(t, check(t, root, in), DeclarationRel, "marks no work as the Go toolchain")
}

// TestUpdateStoresASharedTextOnce asks whether two works shipping a
// byte-identical licence share one stored copy while each still lists its
// own file. Method: widget and gadget carry the same MIT text, and both
// ship.
func TestUpdateStoresASharedTextOnce(t *testing.T) {
	files := fixtureFiles()
	files[DeclarationRel] = `{"categories": [{"name": "Widgets", "modules": [
  {"module": "github.com/acme/widget", "why": "Widgets.", "licence": "MIT", "copyright": "Acme Widgets", "source": "https://github.com/acme/widget"}]}],
 "linked": [{"module": "github.com/acme/gadget", "licence": "MIT", "copyright": "Acme Widgets", "source": "https://github.com/acme/gadget"}],
 "works": []}
`
	root := writeTree(t, files)
	out, refusals, err := planTree(root, fixtureInputs(flavorLinks{Flavor: "distro/linux", Modules: []string{gadget, widget}}))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(refusals) != 0 {
		t.Fatalf("plan refused the fixture: %+v", refusals)
	}
	if len(out.files) != 2 {
		t.Errorf("stored %d files, want the record and one shared text", len(out.files))
	}
	for _, name := range []string{gadget, widget} {
		work, held := out.record.work(name)
		if !held {
			t.Fatalf("no record of %s", name)
		}
		if len(work.Files) != 1 || work.Files[0].Origin != "vendor/"+name+"/LICENSE" {
			t.Errorf("%s lists %+v, want its own LICENSE", name, work.Files)
		}
	}
}
