package feature

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
)

// VALIDATES: a .ci this host skips runs in Ze's own system, through `le test qemu
// docker-lab` with the runtime kernel the image boots and the harness `test qemu
// all` needs, and never through the Alpine route.
// PREVENTS: every guest item stopping before any suite, because `test qemu all`
// refuses any guest whose PID 1 is not /gokrazy/init.
//
// Method: the set's le is a script that prints its own argv and a PASS line, so
// the test reads the command feature run chose; the kernel resolver is injected
// and the guest daemons count as built, so nothing compiles.
func TestAGuestOnlyTestRunsInZeSystem(t *testing.T) {
	toolchain, err := gotoolchain.New(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("toolchain: %v", err)
	}
	set := t.TempDir()
	script := "#!/bin/sh\necho \"argv: $*\"\necho \"1.0s  1/1  PASS  a  gated\"\n"
	if err := os.WriteFile(filepath.Join(set, testfunctional.LE), []byte(script), 0o700); err != nil { //nolint:gosec // the fake le must be executable
		t.Fatal(err)
	}
	runner := &repoRunner{tree: t.TempDir(), toolchain: toolchain, prepared: true, guestBuilt: true,
		set:         testfunctional.BinarySet{Dir: set},
		needsGuest:  func(string) (bool, error) { return true, nil },
		guestKernel: func() (string, error) { return "/cache/runtime/vmlinuz", nil }}
	seen, err := runner.run("test/policy/gated.ci")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, want := range []string{
		"argv: test qemu docker-lab kernel /cache/runtime/vmlinuz harness ",
		" test qemu all test test/policy/gated.ci",
	} {
		if !strings.Contains(seen.output, want) {
			t.Errorf("the command feature run chose lacks %q:\n%s", want, seen.output)
		}
	}
	if strings.Contains(seen.output, "test qemu run") {
		t.Errorf("the command feature run chose is the Alpine route:\n%s", seen.output)
	}
	if problem := observedPass("test/policy/gated.ci", seen); problem != "" {
		t.Fatalf("the test's own PASS line was not read: %s", problem)
	}
}

// VALIDATES: a runtime kernel that cannot be resolved stops the guest item with
// the resolver's own error and starts nothing.
// PREVENTS: a guest run with no kernel, which the carrier refuses for a different
// reason, or a pass recorded with nothing observed.
func TestAGuestItemRefusesWithoutARuntimeKernel(t *testing.T) {
	toolchain, err := gotoolchain.New(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("toolchain: %v", err)
	}
	runner := &repoRunner{tree: t.TempDir(), toolchain: toolchain, prepared: true, guestBuilt: true,
		needsGuest:  func(string) (bool, error) { return true, nil },
		guestKernel: func() (string, error) { return "", errors.New("no kernel here") }}
	_, err = runner.run("test/policy/gated.ci")
	if err == nil || !strings.Contains(err.Error(), "no kernel here") {
		t.Fatalf("run answered %v, want the resolver's refusal", err)
	}
}
