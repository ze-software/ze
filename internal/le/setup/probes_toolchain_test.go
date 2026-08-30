// Related: probes.go — staticcheckBuiltNewEnough, goDirective, goBuildVersion
//
// VALIDATES: a staticcheck built with a Go older than go.mod asks for is
// reported absent rather than present.
// PREVENTS: a stale analyser binary reporting hundreds of import failures in
// files that are correct, which reads as a source defect in someone else's
// package and charges a structural gate red to whoever commits next.
package setup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// setupWithGoMod returns a Setup whose Root holds a go.mod with the given go
// directive, and whose Shell answers `go version -m` with builtWith.
// pinning it here and varying builtWith is what makes the older/newer/equal
// cases readable, and a caller varying it is the natural next case to add.
//
//nolint:unparam // directive is the other half of the comparison under test:
func setupWithGoMod(t *testing.T, directive, builtWith string) *Setup {
	t.Helper()

	root := t.TempDir()
	body := "module example.com/x\n\ngo " + directive + "\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(body), 0o600); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	return &Setup{
		Root: root,
		Shell: &Shell{
			Exec: func(_ context.Context, cmd Cmd) Result {
				if len(cmd.Argv) >= 3 && cmd.Argv[0] == "go" && cmd.Argv[1] == "version" {
					return Result{Code: 0, Out: "/bin/staticcheck: " + builtWith + "\n\tpath\thonnef.co/go/tools\n"}
				}
				return Result{Code: 1}
			},
		},
	}
}

// TestAStaticcheckBuiltWithAnOlderGoIsRefused is the case that cost a gate.
func TestAStaticcheckBuiltWithAnOlderGoIsRefused(t *testing.T) {
	s := setupWithGoMod(t, "1.27.0", "go1.26.6")

	if s.staticcheckBuiltNewEnough("/bin/staticcheck") {
		t.Error("a staticcheck built with go1.26.6 was accepted against a go1.27.0 " +
			"directive. It cannot read the export data the local Go writes, so every " +
			"import it meets is reported as a failure in a file that is correct")
	}
}

// TestAStaticcheckBuiltWithTheSameOrNewerGoIsAccepted is the discrimination
// half. Without it the probe could refuse everything and still pass above.
func TestAStaticcheckBuiltWithTheSameOrNewerGoIsAccepted(t *testing.T) {
	for _, builtWith := range []string{"go1.27.0", "go1.28.1"} {
		s := setupWithGoMod(t, "1.27.0", builtWith)
		if !s.staticcheckBuiltNewEnough("/bin/staticcheck") {
			t.Errorf("a staticcheck built with %s was refused against a go1.27.0 "+
				"directive; only an OLDER build is stale", builtWith)
		}
	}
}

// TestAnUnreadableStampDoesNotFailTheProbe keeps the probe from turning an
// unrelated failure into a missing tool. Absence of evidence is not staleness.
func TestAnUnreadableStampDoesNotFailTheProbe(t *testing.T) {
	s := setupWithGoMod(t, "1.27.0", "not-a-version")
	if !s.staticcheckBuiltNewEnough("/bin/staticcheck") {
		t.Error("an unreadable build stamp was treated as a stale binary")
	}

	noModule := &Setup{Root: t.TempDir(), Shell: &Shell{
		Exec: func(context.Context, Cmd) Result {
			return Result{Code: 0, Out: "/bin/staticcheck: go1.20.0\n"}
		},
	}}
	if !noModule.staticcheckBuiltNewEnough("/bin/staticcheck") {
		t.Error("a missing go.mod was treated as a stale binary; there is no " +
			"directive to hold the build to")
	}
}

// TestTheGoDirectiveIsReadFromGoMod pins the parse, so a reformatted go.mod
// cannot silently disable the check above by reading as no directive at all.
func TestTheGoDirectiveIsReadFromGoMod(t *testing.T) {
	s := setupWithGoMod(t, "1.27.0", "go1.27.0")
	if got := s.goDirective(); got != "go1.27.0" {
		t.Fatalf("goDirective = %q, want go1.27.0", got)
	}
}
