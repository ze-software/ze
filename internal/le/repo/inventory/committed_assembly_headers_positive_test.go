// Design: docs/architecture/plugin/feature-gates.md -- canonical headers do not change committed report membership.
package repoinventory

import "testing"

const asmHeaderLZ4Go = `package headerprobe
const (
 minMatch = 4
 winSizeLog = 16
 winSize = 1 << winSizeLog
 mask = winSize - 1
)
const (
 Block64Kb uint32 = 1 << (4 + iota*2)
 Block256Kb
 Block1Mb
 Block4Mb
 Block8Mb = 2 * Block4Mb
 bound = Block8Mb + Block8Mb/255 + 16
)
type CompressionLevel uint32
const Fast CompressionLevel = 0
type Compressor struct { table, inUse [4]uint16 }
type Box[T any] struct { Field T }
type Holder struct { Box Box[int]; smallDirty, largeDirty bool }
type Constraint interface { uint16 | int32 }
type srcReader struct { p *byte }
func Dormant() {}
`

const asmHeaderKindsGo = `package headerprobe
const UntypedFloat = 1.0
const UntypedComplex = 1i
const TypedFloat float64 = 1
const ComplexExpression = 1 + 2i
const TypedInteger int = 1.0
const MixedInteger = TypedInteger + 1.0
const ConvertedInteger = int(1.0)
const Truth = true
const Words = "TEXT GLOBL DATA"
type Scalar = int
const NamedInteger Scalar = 2
const (
 First uint32 = 1 << iota
 Second
 Third
)
`

const asmHeaderShapesGo = `package headerprobe
type Base struct { Left, Right int; _ int }
type Alias = Base
type Defined Base
type Holder struct { Alias; *Defined }
type Box[T any] struct { Field T }
type Instantiated = Box[int]
type Enclosing struct { Generic Box[int]; First, Second bool }
type _ struct { Ignored int }
const _ = 7
`

// TestCommittedAssemblyCanonicalHeadersPreserveFullReport establishes each
// admitted header's positive prerequisite through both public committed APIs.
func TestCommittedAssemblyCanonicalHeadersPreserveFullReport(t *testing.T) {
	cases := []struct {
		name string
		unit string
	}{
		{"funcdata", "#include \"funcdata.h\"\n" + asmHeaderNumericBody},
		{"funcdata-metadata", "#include \"funcdata.h\"\nTEXT ·unentered(SB),4,$0-0\n GO_ARGS\n GO_RESULTS_INITIALIZED\n NO_LOCAL_POINTERS\n RET\n"},
		{"empty-generated", "#include \"go_asm.h\"\n" + asmHeaderNumericBody},
		{"repeated-empty-generated", "#include \"go_asm.h\"\n#include \"go_asm.h\"\n" + asmHeaderNumericBody},
		{"all-three", "#include \"go_asm.h\"\n#include \"funcdata.h\"\n#include \"textflag.h\"\n" + asmHeaderNumericBody},
		{"whole-token-symbol", "#include \"textflag.h\"\nTEXT ·NOSPLIT(SB),4,$0-0\n RET\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			declarations := asmHeaderEmptyGo
			if test.name == "funcdata-metadata" {
				declarations += "func unentered()\n"
			}
			asmHeaderFixture(t, declarations, test.unit)
		})
	}
}

// TestCommittedAssemblyGeneratedHeadersPreserveFullReport distinguishes a
// source-derived nonempty namespace from cpuinfo-shaped structural emptiness.
// Excluded semantic kinds, generic declarations and blank names may be defined
// locally after the include: inventing generated bindings would break positives.
func TestCommittedAssemblyGeneratedHeadersPreserveFullReport(t *testing.T) {
	cases := []struct {
		name         string
		declarations string
		macros       string
		body         string
	}{
		{"lz4-shape", asmHeaderLZ4Go, "#define Box__size 7\n#define Box_Field 7\n", "TEXT ·unentered(SB),4,$0-0\n MOVQ $const_minMatch, AX\n MOVQ $Holder_Box, BX\n RET\n"},
		{"semantic-kind-exclusions", asmHeaderKindsGo, "#define const_UntypedFloat 7\n#define const_UntypedComplex 7\n#define const_TypedFloat 7\n#define const_ComplexExpression 7\n", asmHeaderNumericBody},
		{"aliases-embedded-and-blank", asmHeaderShapesGo, "#define Base__ 7\n#define const__ 7\n#define ___size 7\n#define __Ignored 7\n#define Box__size 7\n#define Box_Field 7\n", asmHeaderNumericBody},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			unit := "#include \"go_asm.h\"\n#include \"textflag.h\"\n" + test.macros + test.body
			fixture := asmHeaderFixture(t, test.declarations, unit)
			// A second unit in the SAME package has independent include bindings.
			write(t, fixture.root, asmHeaderProbeDir+"/independent_linux_amd64.s", "#include \"go_asm.h\"\n#include \"textflag.h\"\nTEXT ·secondUnit(SB),4,$0-0\n RET\n")
			commitInventoryFixture(t, fixture.root)
			asmHeaderPositive(t, fixture, asmHeaderPaths())
		})
	}
}

// TestCommittedAssemblyGeneratedMembershipRefusesUnknown proves derivation is
// lazy: selected but unused unsupported declarations are not global blockers.
// Once go_asm.h is requested, unknown membership must refuse by its OWN cause.
func TestCommittedAssemblyGeneratedMembershipRefusesUnknown(t *testing.T) {
	cases := []struct {
		name         string
		declarations string
		cause        string
		origin       string
	}{
		{"external-constant-kind", "package headerprobe\nimport \"math\"\nconst Unknown = math.Pi\n", "unsupported generated assembly constant kind", "math"},
		{"external-struct-shape", "package headerprobe\nimport \"time\"\ntype Unknown = time.Time\n", "unsupported generated assembly type", "time"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := asmHeaderFixture(t, test.declarations, "#include \"textflag.h\"\n"+asmHeaderNumericBody, test.origin)
			// A DIFFERENT package needs generated evidence. That must not
			// eagerly classify this probe's unused unsupported declarations.
			write(t, fixture.root, asmTargetFile, "#include \"go_asm.h\"\n"+asmTargetBody)
			commitInventoryFixture(t, fixture.root)
			asmHeaderPositive(t, fixture, asmHeaderPaths())
			write(t, fixture.root, asmHeaderUnit, "#include \"go_asm.h\"\n#include \"textflag.h\"\n"+asmHeaderNumericBody)
			commitInventoryFixture(t, fixture.root)
			asmHeaderRefusal(t, fixture, test.cause, "Unknown")
		})
	}
}
