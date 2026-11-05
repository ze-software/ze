// Design: docs/architecture/plugin/feature-gates.md -- one declared compiler header boundary.
// Related: assemblygenerated.go -- immutable package-generated emission evidence.
package repoinventory

import (
	"fmt"
	"go/build"
	"path"
	"strings"
)

type assemblyHeader struct {
	name      string
	bindings  []string
	generated bool
}

// assemblyHeaders is the reviewed compiler-language contract, not a host include
// search. Both presence acquisition and include admission MUST use this table.
var assemblyHeaders = [...]assemblyHeader{
	{name: "textflag.h", bindings: []string{"NOPROF", "DUPOK", "NOSPLIT", "RODATA", "NOPTR", "WRAPPER", "NEEDCTXT", "TLSBSS", "NOFRAME", "REFLECTMETHOD", "TOPFRAME", "ABIWRAPPER"}},
	{name: "funcdata.h", bindings: []string{"PCDATA_UnsafePoint", "PCDATA_StackMapIndex", "PCDATA_InlTreeIndex", "PCDATA_ArgLiveIndex", "PCDATA_PanicBounds", "FUNCDATA_ArgsPointerMaps", "FUNCDATA_LocalsPointerMaps", "FUNCDATA_StackObjects", "FUNCDATA_InlTree", "FUNCDATA_OpenCodedDeferInfo", "FUNCDATA_ArgInfo", "FUNCDATA_ArgLiveInfo", "FUNCDATA_WrapInfo", "GO_ARGS", "GO_RESULTS_INITIALIZED", "NO_LOCAL_POINTERS", "ArgsSizeUnknown"}},
	{name: "go_asm.h", generated: true},
}

func assemblyHeaderNamed(name string) *assemblyHeader {
	for index := range assemblyHeaders {
		if assemblyHeaders[index].name == name {
			return &assemblyHeaders[index]
		}
	}
	return nil
}

// assemblyContextBindings derives ordinary-package -D identities from the SAME
// normalized context used for source selection. Special runtime/AllowAsmABI
// packages are outside this Snapshot's selected package roles.
func assemblyContextBindings(target *build.Context) ([]string, error) {
	for _, tag := range target.ToolTags {
		if tag == "amd64.v1" {
			return []string{"GOOS_" + target.GOOS, "GOARCH_" + target.GOARCH, "GOAMD64_" + strings.TrimPrefix(tag, "amd64.")}, nil
		}
	}
	return nil, fmt.Errorf("declared assembly context has no proved amd64 level")
}

type assemblyHeaderReader struct {
	snapshot  *Snapshot
	generated generatedAssembly
}

type assemblyNamespace struct {
	bound      map[string]bool
	expandable map[string]bool
	headers    *assemblyHeaderReader
	pkg        *sourcePackage
}

func (reader *assemblyHeaderReader) unit(pkg *sourcePackage) (*assemblyNamespace, error) {
	if strings.HasPrefix(pkg.dir, "vendor/") {
		identity := strings.TrimPrefix(pkg.dir, "vendor/")
		if !strings.Contains(strings.SplitN(identity, "/", 2)[0], ".") {
			return nil, fmt.Errorf("unproved special-package assembler context for %s", identity)
		}
	}
	unit := &assemblyNamespace{bound: make(map[string]bool), expandable: make(map[string]bool), headers: reader, pkg: pkg}
	for _, name := range reader.snapshot.assemblyPredefines {
		if err := unit.bind(name, false); err != nil {
			return nil, err
		}
	}
	return unit, nil
}

// bind preserves individual emission occurrences, including repeated generated
// names. A fixed compiler binding does not become a local expansion obligation.
func (unit *assemblyNamespace) bind(name string, expandable bool) error {
	if unit.bound[name] {
		return fmt.Errorf("assembly macro %s is redefined", name)
	}
	unit.bound[name] = true
	if expandable {
		unit.expandable[name] = true
	}
	return nil
}

func (unit *assemblyNamespace) include(name string) error {
	header := assemblyHeaderNamed(name)
	if header == nil {
		return fmt.Errorf("unproved assembly include %q", name)
	}
	if unit.headers.snapshot.headerPresence[path.Join(unit.pkg.dir, name)] {
		return fmt.Errorf("committed local %s shadows the compiler header", name)
	}
	bindings := header.bindings
	if header.generated {
		var err error
		bindings, err = generatedAssemblyBindings(&unit.headers.generated, unit.pkg)
		if err != nil {
			return err
		}
	}
	for _, binding := range bindings {
		if err := unit.bind(binding, header.generated); err != nil {
			return err
		}
	}
	return nil
}
