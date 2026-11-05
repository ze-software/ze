// Design: docs/architecture/plugin/feature-gates.md -- useful reports survive only proved retained transfers.
package repoinventory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommittedAssemblyTrampolinePreservesFullReport drives public Collect
// through an arbitrary first-party entry, retaining exact report-area controls.
// Distinct same-file instructions, linkage and Go aliases are permitted survivors.
func TestCommittedAssemblyTrampolinePreservesFullReport(t *testing.T) {
	for _, name := range []string{"plain", "same-file-nontrampoline", "go-alias-not-linker-prefix", "distinct-linkname-and-unsafe", "long-irrelevant-assembly", "numeric-nosplit"} {
		t.Run(name, func(t *testing.T) {
			fixture := newASMInventoryFixture(t)
			alias := "syscall"
			if name == "go-alias-not-linker-prefix" {
				alias = "sys"
			}
			fixture.addBoundary(t, alias)
			switch name {
			case "same-file-nontrampoline":
				write(t, fixture.root, asmTargetFile, asmTargetBody+"\nTEXT ·unrelatedBody(SB),NOSPLIT,$0-0\n CALL runtime·entersyscall(SB)\n MOVQ $0, AX\n SYSCALL\n RET\n")
			case "distinct-linkname-and-unsafe":
				fixture.goFile(t, asmOrdinaryDir+"/distinct.go", "package ordinary\nimport _ \"unsafe\"\n//go:linkname distinctLink runtime.fastrand\nfunc distinctLink() uint32\n")
			case "long-irrelevant-assembly":
				write(t, fixture.root, asmOrdinaryDir+"/noise_linux_amd64.s", "TEXT ·unrelatedLong(SB),4,$0-0\n//"+strings.Repeat("x", 70*1024)+"\n RET\n")
			case "numeric-nosplit":
				write(t, fixture.root, asmTargetFile, strings.Replace(asmTargetBody, ",NOSPLIT,", ",4,", 1))
			}
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
			asmCheckSourcesStayGo(t, snapshot, len(fixture.goSources))
		})
	}
}

// TestCommittedAssemblySnapshotIgnoresDirtyAndMovedEvidence uses public APIs
// across dirty assembly/header/Go bytes and a later committed incompatible body.
// Fresh membership must refuse while the original snapshot retains exact metadata.
func TestCommittedAssemblySnapshotIgnoresDirtyAndMovedEvidence(t *testing.T) {
	fixture := newASMInventoryFixture(t)
	fixture.addBoundary(t, "syscall")
	commitInventoryFixture(t, fixture.root)
	snapshot, err := ReadCommitted(context.Background(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	original := snapshot.Revision()
	write(t, fixture.root, asmTargetFile, "TEXT ·inventoryGate(SB),NOSPLIT,$0-56\n CALL ·hidden(SB)\n RET\n")
	write(t, fixture.root, asmOrdinaryDir+"/textflag.h", "#define JMP CALL\n#define TEXT invalid\n")
	write(t, fixture.root, asmOrdinaryDir+"/register.go", "package ordinary\n")
	inv, err := Collect(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	fixture.checkReport(t, inv)
	plugins, err := snapshot.Plugins()
	if err != nil {
		t.Fatal(err)
	}
	fixture.checkPlugins(t, plugins)
	// Restore only the owned Go input; retain and commit the incompatible
	// assembly/header evidence so the fresh failure is not missing membership.
	write(t, fixture.root, asmOrdinaryDir+"/register.go", fixture.goSources[asmOrdinaryDir+"/register.go"])
	commitInventoryFixture(t, fixture.root)
	if snapshot.Revision() != original {
		t.Fatal("captured snapshot revision moved")
	}
	plugins, err = snapshot.Plugins()
	if err != nil {
		t.Fatal(err)
	}
	fixture.checkPlugins(t, plugins)
	asmCheckSourcesStayGo(t, snapshot, len(fixture.goSources))
	asmExpectRefusal(t, fixture)
}

// TestCommittedHeaderShadowNeedsPresenceNotPayload removes only an owned loose
// header blob after committing its tree entry. ReadCommitted must not fetch that
// unused payload; membership must still refuse because the shadow path exists.
func TestCommittedHeaderShadowNeedsPresenceNotPayload(t *testing.T) {
	fixture := newASMInventoryFixture(t)
	fixture.addBoundary(t, "syscall")
	write(t, fixture.root, asmOrdinaryDir+"/textflag.h", "#define JMP CALL\n//unique header presence control\n")
	commitInventoryFixture(t, fixture.root)
	blob := strings.TrimSpace(committedGit(t, fixture.root, "rev-parse", "HEAD:"+asmOrdinaryDir+"/textflag.h"))
	if len(blob) < 3 {
		t.Fatalf("fixture header object ID = %q", blob)
	}
	ownedObject := filepath.Join(fixture.root, ".git", "objects", blob[:2], blob[2:])
	if err := os.Remove(ownedObject); err != nil {
		t.Fatalf("remove only test-owned loose header payload: %v", err)
	}
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatalf("header payload was fetched instead of presence metadata: %v", err)
	}
	plugins, err := snapshot.Plugins()
	if err == nil || plugins != nil {
		t.Fatalf("missing header payload hid committed shadow presence: %+v %v", plugins, err)
	}
	t.Logf("actual shadow refusal (old unavailable-body masking is not header-proof evidence): %v", err)
}
