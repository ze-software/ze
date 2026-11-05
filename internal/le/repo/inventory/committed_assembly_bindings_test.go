// Design: docs/architecture/plugin/feature-gates.md -- compiler and source macro binding ownership.
package repoinventory

import "testing"

const asmCompilerBindingFile = asmOrdinaryDir + "/compilerbindings_linux_amd64.s"
const asmCompilerBindingHeader = "#include \"textflag.h\"\n"
const asmCompilerBindingBody = "TEXT ·bindingControl(SB),4,$0-0\n RET\n"

// asmCompilerBindingFixture proves a canonical include in each of two selected
// units preserves every ordinary report field before a collision is committed.
// Numeric flags in the second unit ensure local header expansion cannot mask
// the missing compiler/header binding collision discriminator.
func asmCompilerBindingFixture(t *testing.T) *asmInventoryFixture {
	t.Helper()
	fixture := newASMInventoryFixture(t)
	fixture.addBoundary(t, "syscall")
	write(t, fixture.root, asmCompilerBindingFile, asmCompilerBindingHeader+asmCompilerBindingBody)
	commitInventoryFixture(t, fixture.root)
	inv, err := Collect(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	fixture.checkReport(t, inv)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	plugins, err := snapshot.Plugins()
	if err != nil {
		t.Fatal(err)
	}
	fixture.checkPlugins(t, plugins)
	asmCheckSourcesStayGo(t, snapshot, len(fixture.goSources))
	return fixture
}

// TestCommittedAssemblyCompilerHeaderBindingsRefuse retains canonical include-
// only positives and rejects source/header collisions in BOTH orders. The
// canonical compiler header is unguarded, so repeating it also redefines names.
func TestCommittedAssemblyCompilerHeaderBindingsRefuse(t *testing.T) {
	cases := []struct {
		name string
		unit string
	}{
		{"header-then-local", asmCompilerBindingHeader + "#define NOSPLIT 4\n" + asmCompilerBindingBody},
		{"local-then-header", "#define NOSPLIT 4\n" + asmCompilerBindingHeader + asmCompilerBindingBody},
		{"repeated-header", asmCompilerBindingHeader + asmCompilerBindingHeader + asmCompilerBindingBody},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := asmCompilerBindingFixture(t)
			write(t, fixture.root, asmCompilerBindingFile, test.unit)
			commitInventoryFixture(t, fixture.root)
			asmExpectRefusal(t, fixture)
		})
	}
}

// TestCommittedAssemblyContextPredefinesRefuse isolates target -D bindings from
// header ownership: the changed unit has no include and uses only numeric flags.
// These names are supplied by the declared linux/amd64/v1 compiler invocation,
// not by the host running this test or by a source token used in a header.
func TestCommittedAssemblyContextPredefinesRefuse(t *testing.T) {
	for _, name := range []string{"GOOS_linux", "GOARCH_amd64", "GOAMD64_v1"} {
		t.Run(name, func(t *testing.T) {
			fixture := asmCompilerBindingFixture(t)
			write(t, fixture.root, asmCompilerBindingFile, "#define "+name+" 1\n"+asmCompilerBindingBody)
			commitInventoryFixture(t, fixture.root)
			asmExpectRefusal(t, fixture)
		})
	}
}
