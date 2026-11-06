// Design: docs/architecture/plugin/feature-gates.md -- closure and certified assembly results share declared reflection authority.
package repoinventory

import (
	"strings"
	"testing"
)

// TestExternalSignatureReturnedClosureAuthority keeps the closure internal;
// only its typed-nil result reaches opaque execution in the negative case.
func TestExternalSignatureReturnedClosureAuthority(t *testing.T) {
	for _, expose := range []bool{false, true} {
		name := "scalar-control"
		argument := `"scalar copy"`
		if expose {
			name = "returned-value"
			argument = "produce()"
		}
		t.Run(name, func(t *testing.T) {
			declarations := "type root string\nfunc (root) Mutate() { label = \"mutated registration metadata\" }\n"
			statements := "produce := func() func(root) { return nil }\n_ = produce()\nfmt.Print(" + argument + ")"
			effects, file := externalTerminalSelectedSource(t, declarations, statements)
			err := effects.solve()
			if !effects.metadata[file.Scope.Lookup("label")] {
				t.Fatal("closure result fixture lost its actual registration metadata")
			}
			if !expose {
				if err != nil {
					t.Fatalf("discarded closure result exposed reflection authority: %v", err)
				}
				externalTerminalAccepted(t, effects, file)
				return
			}
			if err == nil {
				t.Fatal("typed-nil closure result lost its mutating signature type")
			}
			if !strings.Contains(err.Error(), "metadata") {
				t.Fatalf("closure result refusal lost its metadata cause: %v", err)
			}
			if len(effects.admitted) != 0 {
				t.Fatal("closure result refusal admitted an opaque call")
			}
		})
	}
}

// TestExternalSignatureReturnedAssemblyAuthority selects the real zero-frame
// direct-JMP certificate and BOTH sink witnesses. A named int result shares
// syscall.Getpid's scalar representation; no native reference is fabricated.
// This proves source-certificate/result composition, not a native ABI execution.
func TestExternalSignatureReturnedAssemblyAuthority(t *testing.T) {
	for _, expose := range []bool{false, true} {
		name := "scalar-control"
		argument := `"scalar copy"`
		if expose {
			name = "returned-value"
			argument = "produced"
		}
		t.Run(name, func(t *testing.T) {
			const directory = "internal/reflectionassemblyreturn"
			const filename = directory + "/proof.go"
			selected := typeIdentitySelectedSource(t, directory, map[string]string{
				"go.mod":                                     "module github.com/ze-software/ze\n\ngo 1.26\n",
				"feature-gates.txt":                          "",
				registrationPackage + "/register.go":         "package registry\nimport (_ \"fmt\"; _ \"syscall\")\nfunc Register(label string) {}\n",
				"internal/component/config/yang/register.go": "package yang\nimport (_ \"fmt\"; _ \"syscall\")\n",
				directory + "/proof_linux_amd64.s":           "TEXT ·produce(SB),4,$0-8\n JMP syscall·Getpid(SB)\n",
				filename: `package reflectionassemblyreturn
import (
	"fmt"
	_ "syscall"
	registry "github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/component/config/yang"
)
var label = "retained registration metadata"
type root int
func (root) Mutate() { label = "mutated registration metadata" }
func produce() root
func init() {
	registry.Register(label)
	produced := produce()
	_ = produced
	fmt.Print(` + argument + `)
}
`,
			})
			source, err := newExternalSource(selected.source.snapshot)
			if err != nil {
				t.Fatal(err)
			}
			effects := newExternalEffects(source)
			err = effects.solve()
			file := typeIdentitySelectedFile(t, source.snapshot.packages[directory], filename)
			if !effects.metadata[file.Scope.Lookup("label")] {
				t.Fatal("assembly result fixture lost its actual registration metadata")
			}
			if effects.assembly == nil {
				t.Fatal("assembly result fixture did not reach the source certificate")
			}
			if effects.assemblyErr != nil {
				t.Fatalf("assembly source certificate failed: %v", effects.assemblyErr)
			}
			if !expose {
				if err != nil {
					t.Fatalf("inert assembly result control was refused: %v", err)
				}
				if len(effects.admitted) != 1 {
					t.Fatal("assembly control did not admit its opaque call")
				}
				return
			}
			if err == nil {
				t.Fatal("named scalar assembly result lost its reflected mutator")
			}
			if !strings.Contains(err.Error(), "metadata") {
				t.Fatalf("assembly result refusal lost its metadata cause: %v", err)
			}
			if len(effects.admitted) != 0 {
				t.Fatal("assembly result refusal admitted an opaque call")
			}
		})
	}
}
