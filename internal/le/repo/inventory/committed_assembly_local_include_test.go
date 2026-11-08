// Design: docs/architecture/plugin/feature-gates.md -- a package-local header is read as text at its include line.
// Related: committed_assembly_leaf_macro_test.go -- the local macro rules a header's #define lines follow.
package repoinventory

import "testing"

const (
	asmLocalHeaderName = "leaf_amd64.h"
	asmLocalHeaderFile = asmOrdinaryDir + "/" + asmLocalHeaderName
	// asmLocalHeader has the shape of cloudflare/circl's fp448 fp_amd64.h:
	// comments and a function-like macro continued over several lines.
	asmLocalHeader  = "// load sets dst to three in two steps.\n#define load(dst) \\\n\tMOVQ $1, dst \\\n\tADDQ $2, dst\n"
	asmLocalInclude = "#include \"" + asmLocalHeaderName + "\"\n"
)

// asmLocalIncludeFixture is the leaf fixture with extra committed files, the
// local header among them, landed in the same commit as the unit.
func asmLocalIncludeFixture(t *testing.T, kind, unit string, files map[string]string) *asmInventoryFixture {
	t.Helper()
	fixture := externalEffectsFixture(t, kind)
	write(t, fixture.root, asmLeafFile, unit)
	for name, body := range files {
		write(t, fixture.root, name, body)
	}
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource(asmLeafReadDecl("var leafPrimes = [...]uint64{1, 2, 3}"), asmLeafReadCall))
	commitInventoryFixture(t, fixture.root)
	return fixture
}

// TestCommittedAssemblyLocalIncludePreservesFullReport reproduces the
// actual-root refusal at cloudflare/circl fp448: a .s file includes a header
// committed beside it, and a leaf body uses the multi-line macro the header
// defines. cmd/asm reads the header's lines at the include line, so the macro
// expands as one of the file's own and the full report publishes for both
// registration sinks.
//
// MUTATION: (assemblyheaders.go, `return unit.includeLocal(name)`,
// `return fmt.Errorf("unproved assembly include %q", name)`): every local
// header refuses again, as it did before this proof existed.
// MUTATION: (source.go, `if !localHeader {`, `if true {`): no local header
// payload is read from the commit, and the include refuses as naming no
// committed file.
func TestCommittedAssemblyLocalIncludePreservesFullReport(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := asmLocalIncludeFixture(t, kind, asmLocalInclude+asmLeafReadBody("load(AX)"), map[string]string{asmLocalHeaderFile: asmLocalHeader})
			externalEffectsPositive(t, fixture)
		})
	}
}

// TestCommittedAssemblyLocalIncludeControlsRefuse keeps every include the
// proof cannot read as committed text beside the .s file refusing, each with
// its own cause, and a header-line failure naming the header and its line.
//
// MUTATION (missing, uncommitted, other-directory):
// (assemblylocalinclude.go, `if !unit.headers.snapshot.localHeaders[header] {`,
// `if false {`): an absent header reads as empty text and the include passes,
// so each control publishes or refuses later without "is not a committed
// regular file".
// MUTATION (other-directory): (assemblylocalinclude.go,
// `header := path.Join(unit.pkg.dir, name)`,
// `header := path.Join(path.Dir(unit.pkg.dir), name)`): the header committed
// in the parent directory is read and the report publishes.
// MUTATION (path): (assemblylocalinclude.go,
// `if strings.ContainsAny(name, "/\\") {`, `if false {`): `sub/leaf_amd64.h`
// is joined below the package directory, read, and the report publishes.
// MUTATION (nested): (assemblylocalinclude.go,
// `return fmt.Errorf("nested assembly include in a local header is not proved")`,
// `return nil`): the nested include is skipped and the report publishes.
// MUTATION (nested): (assemblylocalinclude.go,
// `return fmt.Errorf("%s:%d: %w", header, line, err)`, `_ = line; return err`): the
// refusal no longer names the header file and line.
// MUTATION (redefinition): (assemblyheaders.go, `if unit.bound[name] {`,
// `if unit.bound[name] && false {`): the .s file's #define replaces the
// header's and the leaf publishes.
func TestCommittedAssemblyLocalIncludeControlsRefuse(t *testing.T) {
	const notCommitted = "is not a committed regular file beside the including assembly"
	body := asmLeafReadBody("load(AX)")
	for _, test := range []struct {
		name, unit string
		files      map[string]string
		cause      string
	}{
		{"missing", asmLocalInclude + body, nil, notCommitted},
		{"other-directory", asmLocalInclude + body, map[string]string{"internal/plugins/" + asmLocalHeaderName: asmLocalHeader}, notCommitted},
		{"path", "#include \"sub/" + asmLocalHeaderName + "\"\n" + body, map[string]string{asmOrdinaryDir + "/sub/" + asmLocalHeaderName: asmLocalHeader}, "holds no path separator"},
		{"nested", asmLocalInclude + body, map[string]string{asmLocalHeaderFile: "#include \"textflag.h\"\n" + asmLocalHeader}, asmLocalHeaderFile + ":1: nested assembly include in a local header is not proved"},
		{"redefinition", asmLocalInclude + "#define val CX\n" + asmLeafReadBody("MOVQ val, AX"), map[string]string{asmLocalHeaderFile: "#define val BX\n"}, "assembly macro val is redefined"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(test.name+"/"+kind, func(t *testing.T) {
				fixture := asmLocalIncludeFixture(t, kind, test.unit, test.files)
				asmLeafRefusal(t, fixture, test.cause)
			})
		}
	}
	// A header present only in the working tree is not committed evidence.
	for _, kind := range []string{"plugin", "yang"} {
		t.Run("uncommitted/"+kind, func(t *testing.T) {
			fixture := asmLocalIncludeFixture(t, kind, asmLocalInclude+body, nil)
			write(t, fixture.root, asmLocalHeaderFile, asmLocalHeader)
			asmLeafRefusal(t, fixture, notCommitted)
		})
	}
}
