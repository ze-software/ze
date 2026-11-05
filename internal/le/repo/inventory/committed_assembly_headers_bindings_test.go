// Design: docs/architecture/plugin/feature-gates.md -- ordered header binding collisions and literal ownership.
package repoinventory

import "testing"

// TestCommittedAssemblyCanonicalHeaderBindingsRefuse distinguishes fixed
// numeric and instruction macros from a generic unknown-include refusal.
func TestCommittedAssemblyCanonicalHeaderBindingsRefuse(t *testing.T) {
	for _, name := range []string{"FUNCDATA_ArgsPointerMaps", "GO_ARGS"} {
		for _, order := range []string{"header-first", "local-first"} {
			t.Run(name+"/"+order, func(t *testing.T) {
				header := "#include \"funcdata.h\"\n"
				fixture := asmHeaderFixture(t, asmHeaderEmptyGo, header+asmHeaderNumericBody)
				local := "#define " + name + " 0\n"
				unit := header + local
				if order == "local-first" {
					unit = local + header
				}
				write(t, fixture.root, asmHeaderUnit, unit+asmHeaderNumericBody)
				commitInventoryFixture(t, fixture.root)
				asmHeaderRefusal(t, fixture, name, "redefined")
			})
		}
	}
	for _, test := range []struct {
		name         string
		declarations string
		binding      string
	}{
		{"funcdata.h", asmHeaderEmptyGo, "PCDATA_UnsafePoint"},
		{"go_asm.h", asmHeaderLZ4Go, "const_minMatch"},
	} {
		t.Run("repeated-nonempty/"+test.name, func(t *testing.T) {
			header := "#include \"" + test.name + "\"\n"
			fixture := asmHeaderFixture(t, test.declarations, header+asmHeaderNumericBody)
			write(t, fixture.root, asmHeaderUnit, header+header+asmHeaderNumericBody)
			commitInventoryFixture(t, fixture.root)
			asmHeaderRefusal(t, fixture, test.binding, "redefined")
		})
	}
}

// TestCommittedAssemblyGeneratedHeaderBindingsRefuse covers semantic kinds,
// grouped/alias/embedded fields and nongeneric holders of generic fields. Every
// source definition collides in BOTH orders, independently of literal headers.
func TestCommittedAssemblyGeneratedHeaderBindingsRefuse(t *testing.T) {
	profiles := []struct {
		name         string
		declarations string
		bindings     []string
	}{
		{"lz4", asmHeaderLZ4Go, []string{"const_minMatch", "const_Block256Kb", "const_Fast", "Compressor_table", "Compressor_inUse", "Holder_Box", "Holder_smallDirty", "Holder_largeDirty", "srcReader_p"}},
		{"semantic-kinds", asmHeaderKindsGo, []string{"const_TypedInteger", "const_MixedInteger", "const_ConvertedInteger", "const_Truth", "const_Words", "const_NamedInteger", "const_Second"}},
		{"resolved-shapes", asmHeaderShapesGo, []string{"Base_Left", "Base_Right", "Alias__size", "Alias_Left", "Defined_Right", "Holder_Alias", "Holder_Defined", "Instantiated__size", "Instantiated_Field", "Enclosing_Generic", "Enclosing_First", "Enclosing_Second"}},
	}
	for _, profile := range profiles {
		for _, name := range profile.bindings {
			for _, order := range []string{"header-first", "local-first"} {
				t.Run(profile.name+"/"+name+"/"+order, func(t *testing.T) {
					header := "#include \"go_asm.h\"\n"
					fixture := asmHeaderFixture(t, profile.declarations, header+asmHeaderNumericBody)
					local := "#define " + name + " 0\n"
					unit := header + local
					if order == "local-first" {
						unit = local + header
					}
					write(t, fixture.root, asmHeaderUnit, unit+asmHeaderNumericBody)
					commitInventoryFixture(t, fixture.root)
					asmHeaderRefusal(t, fixture, name, "redefined")
				})
			}
		}
	}
}

// TestCommittedAssemblyGeneratedEmissionCollisionsRefuse prevents set-based
// deduplication and checks generated names against predefines and other headers.
func TestCommittedAssemblyGeneratedEmissionCollisionsRefuse(t *testing.T) {
	for _, test := range []struct {
		name        string
		declaration string
		binding     string
	}{
		{"internal-duplicate", "type T struct { _size int }\n", "T__size"},
		{"predefined-target", "type GOOS struct { linux int }\n", "GOOS_linux"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := asmHeaderFixture(t, asmHeaderEmptyGo, "#include \"go_asm.h\"\n"+asmHeaderNumericBody)
			fixture.goFile(t, asmHeaderProbeFile, asmHeaderEmptyGo+test.declaration)
			commitInventoryFixture(t, fixture.root)
			asmHeaderRefusal(t, fixture, test.binding, "redefined")
		})
	}
	for _, order := range []string{"generated-first", "funcdata-first"} {
		t.Run("cross-header/"+order, func(t *testing.T) {
			headers := "#include \"go_asm.h\"\n#include \"funcdata.h\"\n"
			if order == "funcdata-first" {
				headers = "#include \"funcdata.h\"\n#include \"go_asm.h\"\n"
			}
			fixture := asmHeaderFixture(t, asmHeaderEmptyGo, headers+asmHeaderNumericBody)
			fixture.goFile(t, asmHeaderProbeFile, asmHeaderEmptyGo+"type FUNCDATA struct { ArgsPointerMaps int }\n")
			commitInventoryFixture(t, fixture.root)
			asmHeaderRefusal(t, fixture, "FUNCDATA_ArgsPointerMaps", "redefined")
		})
	}
}

// TestCommittedAssemblyGeneratedLiteralOwnershipRefuse does not expand generated
// names in any entered definition component, opcode or destination.
func TestCommittedAssemblyGeneratedLiteralOwnershipRefuse(t *testing.T) {
	cases := []struct {
		name    string
		binding string
		body    string
	}{
		{"symbol", "const_Entry", "TEXT const_Entry(SB),4,$0-56\n JMP syscall·Syscall(SB)\n"},
		{"address", "const_Register", "TEXT ·inventoryGate(const_Register),4,$0-56\n JMP syscall·Syscall(SB)\n"},
		{"flags", "const_Flags", "TEXT ·inventoryGate(SB),const_Flags,$0-56\n JMP syscall·Syscall(SB)\n"},
		{"frame", "const_Frame", "TEXT ·inventoryGate(SB),4,$const_Frame-56\n JMP syscall·Syscall(SB)\n"},
		{"opcode", "const_Opcode", "TEXT ·inventoryGate(SB),4,$0-56\n const_Opcode syscall·Syscall(SB)\n"},
		{"destination", "const_Destination", "TEXT ·inventoryGate(SB),4,$0-56\n JMP const_Destination(SB)\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := asmHeaderFixture(t, asmHeaderEmptyGo, "#include \"textflag.h\"\n"+asmHeaderNumericBody)
			file := asmOrdinaryDir + "/generated_entry.go"
			fixture.goFile(t, file, "package ordinary\nconst Entry = 0\nconst Register = 0\nconst Flags = 4\nconst Frame = 0\nconst Opcode = \"JMP\"\nconst Destination = \"syscall.Syscall\"\n")
			headers := "#include \"go_asm.h\"\n#include \"textflag.h\"\n"
			write(t, fixture.root, asmTargetFile, headers+asmMacroEntry)
			commitInventoryFixture(t, fixture.root)
			asmHeaderPositive(t, fixture, asmHeaderPaths(file))
			write(t, fixture.root, asmTargetFile, headers+test.body)
			commitInventoryFixture(t, fixture.root)
			asmHeaderRefusal(t, fixture, test.binding, "literal header/body ownership")
		})
	}
}

// TestCommittedAssemblyMetadataInstructionsRemainUnsupported keeps namespace
// admission separate from entered instruction execution certification.
func TestCommittedAssemblyMetadataInstructionsRemainUnsupported(t *testing.T) {
	for _, instruction := range []string{"GO_ARGS", "FUNCDATA $0, go_args_stackmap(SB)", "PCDATA $0, $1"} {
		t.Run(instruction, func(t *testing.T) {
			fixture := asmHeaderFixture(t, asmHeaderEmptyGo, "#include \"funcdata.h\"\n"+asmHeaderNumericBody)
			headers := "#include \"textflag.h\"\n#include \"funcdata.h\"\n"
			write(t, fixture.root, asmTargetFile, headers+asmMacroEntry)
			commitInventoryFixture(t, fixture.root)
			asmHeaderPositive(t, fixture, asmHeaderPaths())
			write(t, fixture.root, asmTargetFile, headers+"TEXT ·inventoryGate(SB),4,$0-56\n "+instruction+"\n JMP syscall·Syscall(SB)\n")
			commitInventoryFixture(t, fixture.root)
			asmHeaderRefusal(t, fixture, "assembly body is not a direct JMP")
		})
	}
}

// TestCommittedAssemblyImportedGeneratedShapesUseDefiningPackage checks imported
// type/constant identity in the selected defining package, not caller spellings.
func TestCommittedAssemblyImportedGeneratedShapesUseDefiningPackage(t *testing.T) {
	for _, name := range []string{"Alias_First", "Alias_Second", "Defined__size", "Embedded_Base", "const_Imported"} {
		for _, order := range []string{"header-first", "local-first"} {
			t.Run(name+"/"+order, func(t *testing.T) {
				fixture := asmHeaderFixture(t, asmHeaderEmptyGo, "#include \"textflag.h\"\n"+asmHeaderNumericBody)
				defining := "internal/headerdefs/definitions.go"
				fixture.goFile(t, defining, "package definitions\ntype Scalar = int\nconst Kind Scalar = 1.0\ntype Base struct { First, Second int }\n")
				fixture.goFile(t, asmHeaderProbeFile, "package headerprobe\nimport d \"github.com/ze-software/ze/internal/headerdefs\"\nconst Imported = d.Kind\ntype Alias = d.Base\ntype Defined d.Base\ntype Embedded struct { *d.Base }\n")
				header := "#include \"go_asm.h\"\n"
				write(t, fixture.root, asmHeaderUnit, header+asmHeaderNumericBody)
				commitInventoryFixture(t, fixture.root)
				asmHeaderPositive(t, fixture, asmHeaderPaths(defining))
				local := "#define " + name + " 0\n"
				unit := header + local
				if order == "local-first" {
					unit = local + header
				}
				write(t, fixture.root, asmHeaderUnit, unit+asmHeaderNumericBody)
				commitInventoryFixture(t, fixture.root)
				asmHeaderRefusal(t, fixture, name, "redefined")
			})
		}
	}
}
