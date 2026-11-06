// Design: docs/architecture/plugin/feature-gates.md -- unknown retained body/linkage must not publish a subset.
package repoinventory

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCommittedAssemblyBodyAndContextRefusals discriminates complete target
// bodies and selected target context. Before certification exists, refusals may
// be masked by the old unavailable-body prerequisite; logs retain that cause.
func TestCommittedAssemblyBodyAndContextRefusals(t *testing.T) {
	cases := map[string]string{
		"missing":                 "",
		"comment-only":            "// TEXT ·inventoryGate(SB),NOSPLIT,$0-56\n// JMP syscall·Syscall(SB)\n",
		"block-comment-only":      "/* TEXT ·inventoryGate(SB),NOSPLIT,$0-56\n JMP syscall·Syscall(SB) */\n",
		"call":                    "TEXT ·inventoryGate(SB),4,$0-56\n CALL syscall·Syscall(SB)\n",
		"extra-instruction":       "TEXT ·inventoryGate(SB),4,$0-56\n MOVQ $0, AX\n JMP syscall·Syscall(SB)\n",
		"conditional-jump":        "TEXT ·inventoryGate(SB),4,$0-56\n JEQ syscall·Syscall(SB)\n",
		"indirect-jump":           "TEXT ·inventoryGate(SB),4,$0-56\n JMP AX\n",
		"local-destination":       "TEXT ·inventoryGate(SB),4,$0-56\n JMP ·Syscall(SB)\n",
		"third-party-destination": "TEXT ·inventoryGate(SB),4,$0-56\n JMP example∕org∕callback·Invoke(SB)\n",
		"dupok":                   "TEXT ·inventoryGate(SB),DUPOK|NOSPLIT,$0-56\n JMP syscall·Syscall(SB)\n",
		"numeric-dupok":           "TEXT ·inventoryGate(SB),6,$0-56\n JMP syscall·Syscall(SB)\n",
		"wrapper-flag":            "TEXT ·inventoryGate(SB),NOSPLIT|WRAPPER,$0-56\n JMP syscall·Syscall(SB)\n",
		"nonzero-frame":           "TEXT ·inventoryGate(SB),4,$8-56\n JMP syscall·Syscall(SB)\n",
		"negative-frame":          "TEXT ·inventoryGate(SB),4,$-8-56\n JMP syscall·Syscall(SB)\n",
		"missing-argsize":         "TEXT ·inventoryGate(SB),4,$0\n JMP syscall·Syscall(SB)\n",
		"overflow-argsize":        "TEXT ·inventoryGate(SB),4,$0-999999999999999999999999999999999999\n JMP syscall·Syscall(SB)\n",
		"abi-selector":            "TEXT ·inventoryGate<ABI0>(SB),4,$0-56\n JMP syscall·Syscall(SB)\n",
		"destination-abi":         "TEXT ·inventoryGate(SB),4,$0-56\n JMP syscall·Syscall<ABIInternal>(SB)\n",
		"static-entry":            "TEXT ·inventoryGate<>(SB),4,$0-56\n JMP syscall·Syscall(SB)\n",
		"entry-offset":            "TEXT ·inventoryGate+8(SB),4,$0-56\n JMP syscall·Syscall(SB)\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newASMInventoryFixture(t)
			fixture.addBoundary(t, "syscall")
			if body == "" {
				if err := os.Remove(filepath.Join(fixture.root, asmTargetFile)); err != nil {
					t.Fatal(err)
				}
			} else {
				write(t, fixture.root, asmTargetFile, body)
			}
			commitInventoryFixture(t, fixture.root)
			asmExpectRefusal(t, fixture)
		})
	}
	// A body that only returns names no symbol and calls nothing, so the leaf
	// proof (assemblyleaf.go) admits it and the report publishes; its zeroed
	// results stay an opaque external value, never a known constant.
	// MUTATION: refuse every leaf in certifyAssembly, and this case refuses.
	t.Run("return-stub", func(t *testing.T) {
		fixture := newASMInventoryFixture(t)
		fixture.addBoundary(t, "syscall")
		write(t, fixture.root, asmTargetFile, "TEXT ·inventoryGate(SB),4,$0-56\n RET\n")
		commitInventoryFixture(t, fixture.root)
		externalEffectsPositive(t, fixture)
	})
	for _, name := range []string{"filename-excluded", "constraint-excluded", "duplicate-selected-entry"} {
		t.Run(name, func(t *testing.T) {
			fixture := newASMInventoryFixture(t)
			fixture.addBoundary(t, "syscall")
			switch name {
			case "filename-excluded":
				if err := os.Remove(filepath.Join(fixture.root, asmTargetFile)); err != nil {
					t.Fatal(err)
				}
				write(t, fixture.root, asmOrdinaryDir+"/boundary_darwin_arm64.s", "TEXT ·inventoryGate(SB),4,$0-56\n JMP syscall·Syscall(SB)\n")
			case "constraint-excluded":
				write(t, fixture.root, asmTargetFile, "//go:build !ze_fixture\n\nTEXT ·inventoryGate(SB),4,$0-56\n JMP syscall·Syscall(SB)\n")
			case "duplicate-selected-entry":
				write(t, fixture.root, asmOrdinaryDir+"/duplicate_linux_amd64.s", "TEXT ·inventoryGate(SB),4,$0-56\n JMP syscall·Syscall(SB)\n")
			}
			commitInventoryFixture(t, fixture.root)
			asmExpectRefusal(t, fixture)
		})
	}
}

func asmHiddenPluginGo(packageName string) string {
	return "package " + packageName + "\nimport r \"github.com/ze-software/ze/internal/component/plugin/registry\"\nfunc hiddenEngine() {}\nfunc installHidden() { _ = r.Register(r.Registration{Name: \"hidden-plugin\", RunEngine: hiddenEngine}) }\n"
}

// TestCommittedAssemblyLinkageCannotRedirectToRegistrations makes an apparent
// stdlib JMP destination repository-owned in another body/file/package or via
// Go linkage directives. Each hidden provider must be retained or refused,
// never silently replaced by a report containing only the ordinary control.
func TestCommittedAssemblyLinkageCannotRedirectToRegistrations(t *testing.T) {
	for _, name := range []string{"foreign-same-file", "foreign-other-file", "foreign-other-package", "foreign-yang-provider", "entry-linkname", "destination-linkname-other-package", "destination-variable-linkname", "entry-one-arg-linkname", "destination-linknamestd", "malformed-selected-linkname", "foreign-unsupported-text", "alias-is-not-linker-prefix", "alias-is-not-stdlib-ownership"} {
		t.Run(name, func(t *testing.T) {
			fixture := newASMInventoryFixture(t)
			fixture.addBoundary(t, "syscall")
			foreign := "TEXT syscall·Syscall(SB),4,$0-56\n JMP ·installHidden(SB)\n"
			switch name {
			case "foreign-same-file":
				fixture.goFile(t, asmOrdinaryDir+"/hidden.go", asmHiddenPluginGo("ordinary"))
				write(t, fixture.root, asmTargetFile, asmTargetBody+"\n"+foreign)
			case "foreign-other-file":
				fixture.goFile(t, asmOrdinaryDir+"/hidden.go", asmHiddenPluginGo("ordinary"))
				write(t, fixture.root, asmOrdinaryDir+"/foreign_linux_amd64.s", foreign)
			case "foreign-other-package", "destination-linkname-other-package", "destination-variable-linkname", "destination-linknamestd":
				fixture.goFile(t, asmOrdinaryDir+"/import_foreign.go", "package ordinary\nimport _ \"github.com/ze-software/ze/internal/foreign\"\n")
				fixture.goFile(t, "internal/foreign/hidden.go", asmHiddenPluginGo("foreign"))
				if name == "foreign-other-package" {
					write(t, fixture.root, "internal/foreign/foreign_linux_amd64.s", foreign)
				} else {
					decl := "func shadowTarget() { installHidden() }\n"
					if name == "destination-variable-linkname" {
						decl = "var shadowTarget = installHidden\n"
					}
					directive := "go:linkname"
					if name == "destination-linknamestd" {
						directive = "go:linknamestd"
					}
					fixture.goFile(t, "internal/foreign/link.go", "package foreign\nimport _ \"unsafe\"\n//"+directive+" shadowTarget syscall.Syscall\n"+decl)
				}
			case "foreign-yang-provider":
				fixture.goFile(t, asmOrdinaryDir+"/hidden.go", "package ordinary\nimport y \"github.com/ze-software/ze/internal/component/config/yang\"\nfunc installHidden() { y.RegisterModule(\"hidden.yang\", \"module hidden { namespace urn:hidden; prefix h; }\") }\n")
				write(t, fixture.root, asmOrdinaryDir+"/yang/hidden.yang", "module hidden { namespace urn:hidden; prefix h; }\n")
				write(t, fixture.root, asmTargetFile, asmTargetBody+"\n"+foreign)
			case "entry-linkname", "entry-one-arg-linkname":
				body := fixture.goSources[asmOrdinaryDir+"/boundary.go"]
				body = strings.Replace(body, "import \"syscall\"", "import (\"syscall\"; _ \"unsafe\")", 1)
				directive := "//go:linkname inventoryGate other.namespace\n"
				if name == "entry-one-arg-linkname" {
					directive = "//go:linkname inventoryGate\n"
				}
				body = strings.Replace(body, "func inventoryGate(", directive+"func inventoryGate(", 1)
				fixture.goFile(t, asmOrdinaryDir+"/boundary.go", body)
			case "malformed-selected-linkname":
				body := fixture.goSources[asmOrdinaryDir+"/boundary.go"]
				body = strings.Replace(body, "import \"syscall\"", "import (\"syscall\"; _ \"unsafe\")", 1)
				body = strings.Replace(body, "func inventoryGate(", "//go:linkname inventoryGate syscall.Syscall extra\nfunc inventoryGate(", 1)
				fixture.goFile(t, asmOrdinaryDir+"/boundary.go", body)
			case "foreign-unsupported-text":
				fixture.goFile(t, asmOrdinaryDir+"/hidden.go", asmHiddenPluginGo("ordinary"))
				write(t, fixture.root, asmOrdinaryDir+"/foreign_linux_amd64.s", "TEXT syscall·Syscall<ABIInternal>(SB),DUPOK|NOSPLIT,$0-56\n JMP ·installHidden(SB)\n")
			case "alias-is-not-linker-prefix":
				fixture.addBoundary(t, "sys")
				write(t, fixture.root, asmTargetFile, strings.Replace(asmTargetBody, "syscall·Syscall", "sys·Syscall", 1))
			case "alias-is-not-stdlib-ownership":
				fixture.goFile(t, "internal/callback/callback.go", "package callback\ntype Errno uintptr\n")
				body := strings.Replace(fixture.goSources[asmOrdinaryDir+"/boundary.go"], "import \"syscall\"", "import syscall \"github.com/ze-software/ze/internal/callback\"", 1)
				fixture.goFile(t, asmOrdinaryDir+"/boundary.go", body)
			}
			commitInventoryFixture(t, fixture.root)
			asmExpectRefusal(t, fixture)
		})
	}
}

// TestCommittedAssemblyPreprocessingCannotHideDefinitions includes directives
// after the target body and in a separate selected unit. Unclassified source
// cannot become proof that no competing definition exists.
func TestCommittedAssemblyPreprocessingCannotHideDefinitions(t *testing.T) {
	for name, directive := range map[string]string{
		"unknown-include":          "#include \"other.h\"\n",
		"parent-include":           "#include \"../textflag.h\"\n",
		"absolute-include":         "#include \"/compiler/textflag.h\"\n",
		"macro-after-target":       "#define JMP CALL\n",
		"conditional-after-target": "#ifdef GOARCH_amd64\nTEXT syscall·Syscall(SB),4,$0-56\n JMP ·installHidden(SB)\n#endif\n",
		"undef-after-target":       "#undef TEXT\n",
		"line-after-target":        "#line 1 \"other.s\"\n",
		"malformed-text":           "TEXT syscall·Syscall(SB),FLAGS,$0-56\n JMP ·installHidden(SB)\n",
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newASMInventoryFixture(t)
			fixture.addBoundary(t, "syscall")
			fixture.goFile(t, asmOrdinaryDir+"/hidden.go", asmHiddenPluginGo("ordinary"))
			write(t, fixture.root, asmTargetFile, asmTargetBody+"\n"+directive)
			commitInventoryFixture(t, fixture.root)
			asmExpectRefusal(t, fixture)
		})
	}
	t.Run("separate-unit-hidden-definition", func(t *testing.T) {
		fixture := newASMInventoryFixture(t)
		fixture.addBoundary(t, "syscall")
		fixture.goFile(t, asmOrdinaryDir+"/hidden.go", asmHiddenPluginGo("ordinary"))
		write(t, fixture.root, asmOrdinaryDir+"/obscured_linux_amd64.s", "#define ENTRY TEXT syscall·Syscall(SB),4,$0-56\nENTRY\n JMP ·installHidden(SB)\n")
		commitInventoryFixture(t, fixture.root)
		asmExpectRefusal(t, fixture)
	})
}

// TestCommittedAssemblyHeaderShadowsRefuse exercises all Git entry kinds at an
// earlier include candidate. Trees/gitlinks are committed object metadata, not
// followed paths or header blobs to feed the batch reader.
func TestCommittedAssemblyHeaderShadowsRefuse(t *testing.T) {
	for _, name := range []string{"regular", "symlink", "populated-tree", "empty-tree", "gitlink"} {
		t.Run(name, func(t *testing.T) {
			fixture := newASMInventoryFixture(t)
			fixture.addBoundary(t, "syscall")
			commitInventoryFixture(t, fixture.root)
			header := asmOrdinaryDir + "/textflag.h"
			switch name {
			case "regular":
				write(t, fixture.root, header, "#define JMP CALL\n#define TEXT invalid\n")
				commitInventoryFixture(t, fixture.root)
			case "symlink":
				if err := os.Symlink("unavailable-header", filepath.Join(fixture.root, header)); err != nil {
					t.Fatal(err)
				}
				commitInventoryFixture(t, fixture.root)
			case "populated-tree":
				write(t, fixture.root, header+"/member", "not a header\n")
				commitInventoryFixture(t, fixture.root)
			case "empty-tree":
				empty := asmGitInput(t, fixture.root, "", "mktree")
				asmCommitHeaderEntry(t, fixture.root, "040000 tree "+empty+"\ttextflag.h")
			case "gitlink":
				head := strings.TrimSpace(committedGit(t, fixture.root, "rev-parse", "HEAD"))
				asmCommitHeaderEntry(t, fixture.root, "160000 commit "+head+"\ttextflag.h")
			}
			asmExpectRefusal(t, fixture)
		})
	}
}

// TestCommittedNoImportCallbackCapabilitiesCannotDisappear guards against
// replacing a body certificate with import-only pruning. The helper itself has
// no registry import, but invoking its argument installs a plugin or YANG module.
func TestCommittedNoImportCallbackCapabilitiesCannotDisappear(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		for _, capability := range []string{"function", "interface"} {
			t.Run(kind+"/"+capability, func(t *testing.T) {
				fixture := newASMInventoryFixture(t)
				helper := "package callback\nfunc Invoke(callback func()) { callback() }\n"
				if capability == "interface" {
					helper = "package callback\ntype Hook interface { Run() }\nfunc Invoke(callback Hook) { callback.Run() }\n"
				}
				fixture.goFile(t, "internal/callback/callback.go", helper)
				targetImport := "r \"github.com/ze-software/ze/internal/component/plugin/registry\""
				install := "_ = r.Register(r.Registration{Name: \"callback-plugin\", RunEngine: engine})"
				if kind == "yang" {
					targetImport = "y \"github.com/ze-software/ze/internal/component/config/yang\""
					install = "y.RegisterModule(\"callback.yang\", \"module callback { namespace urn:callback; prefix c; }\")"
					write(t, fixture.root, asmOrdinaryDir+"/yang/callback.yang", "module callback { namespace urn:callback; prefix c; }\n")
				}
				body := "package ordinary\nimport (\n c \"github.com/ze-software/ze/internal/callback\"\n " + targetImport + "\n)\n"
				if capability == "function" {
					body += "func init() { c.Invoke(func() { " + install + " }) }\n"
				} else {
					body += "type callbackHook struct{}\nfunc (callbackHook) Run() { " + install + " }\nfunc init() { c.Invoke(callbackHook{}) }\n"
				}
				fixture.goFile(t, asmOrdinaryDir+"/callback.go", body)
				commitInventoryFixture(t, fixture.root)
				inv, err := Collect(fixture.root)
				if err != nil {
					if inv.Plugins != nil {
						t.Fatalf("callback refusal leaked partial plugins: %+v", inv.Plugins)
					}
					t.Logf("explicit callback refusal: %v", err)
					return
				}
				if kind == "plugin" {
					var names []string
					for _, plugin := range inv.Plugins {
						names = append(names, plugin.Name)
					}
					if !slices.Equal(names, []string{"callback-plugin", "ordinary"}) {
						t.Fatalf("no-import callback plugin identities = %v", names)
					}
				} else {
					var names []string
					for _, module := range inv.YANGModules {
						names = append(names, module.Name)
					}
					if !slices.Equal(names, []string{"callback.yang", "extra.yang", "fixture"}) {
						t.Fatalf("no-import callback schema identities = %v", names)
					}
				}
				if inv.TotalRPCs != 2 || inv.TotalTests != 2 || inv.Families["ipv4/unicast"] != "ordinary" {
					t.Fatalf("callback changed independent report controls: RPCs %d tests %d families %v", inv.TotalRPCs, inv.TotalTests, inv.Families)
				}
			})
		}
	}
}
