// Design: docs/contributing/rfc-conformance-gates.md -- explicit guest image selection.
// Related: discriminate_guest.go -- placement validation and both native QEMU argv builders.
package rfc

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	stdpatchgen "github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// TestGuestUnitArgvPreservesExplicitImage checks the real unit argv builder with
// paths containing spaces, for both ordinary and root-declared guest units.
// It proves argument propagation, not a guest boot or a protocol observation.
func TestGuestUnitArgvPreservesExplicitImage(t *testing.T) {
	for _, test := range []struct {
		name string
		root bool
	}{
		{name: "ordinary"},
		{name: "root", root: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			runner := observationRunner{self: "/test/le", guest: true, root: test.root,
				kernel: filepath.Join(dir, "accepted kernel", "vmlinuz")}
			image := filepath.Join(dir, "accepted image", "docker-host.img")
			if err := runner.selectGuestImage(image); err != nil {
				t.Fatal(err)
			}
			script := filepath.Join(dir, "unit script.sh")
			want := []string{runner.self, "test", "qemu", "docker-lab", "kernel", runner.kernel,
				testqemu.KeywordImage, image}
			if test.root {
				want = append(want, testqemu.KeywordRoot)
			}
			want = append(want, "command", "sh "+singleQuoted(filepath.ToSlash(script)))
			if got := runner.guestRunArgv(script); !slices.Equal(got, want) {
				t.Fatalf("unit argv = %q, want %q", got, want)
			}
		})
	}
}

// TestGuestCIArgvPreservesExplicitImage checks the functional argv builder
// forwards the same explicit pair while retaining the native CI harness.
func TestGuestCIArgvPreservesExplicitImage(t *testing.T) {
	dir := t.TempDir()
	runner := observationRunner{self: "/test/le", guest: true,
		kernel: filepath.Join(dir, "accepted kernel", "vmlinuz"),
		tag:    Tag{File: "test/qemu/netns.ci"}}
	image := filepath.Join(dir, "accepted image", "docker-host.img")
	if err := runner.selectGuestImage(image); err != nil {
		t.Fatal(err)
	}
	binaries := filepath.Join(dir, "isolated binaries")
	want := []string{runner.self, "test", "qemu", "docker-lab", "kernel", runner.kernel,
		testqemu.KeywordImage, image, testqemu.KeywordHarness, "timeout", guestCICommandTimeout,
		"command", testqemu.OneCICommand(binaries, runner.tag.File, "arm64")}
	if got := runner.guestCIArgv(binaries, "arm64"); !slices.Equal(got, want) {
		t.Fatalf("functional argv = %q, want %q", got, want)
	}
}

// TestGuestUnitArgvKeepsAutomaticImage checks omission leaves the existing unit
// command unchanged, so the native carrier continues to select the image.
func TestGuestUnitArgvKeepsAutomaticImage(t *testing.T) {
	runner := observationRunner{self: "/test/le", guest: true, kernel: "/test/vmlinuz"}
	if err := runner.selectGuestImage(""); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "unit.sh")
	want := []string{runner.self, "test", "qemu", "docker-lab", "kernel", runner.kernel,
		"command", "sh " + singleQuoted(filepath.ToSlash(script))}
	if got := runner.guestRunArgv(script); !slices.Equal(got, want) {
		t.Fatalf("automatic-image unit argv = %q, want %q", got, want)
	}
}

// TestGuestCIArgvKeepsAutomaticImage checks omission retains the existing
// functional command, including the harness and its timeout.
func TestGuestCIArgvKeepsAutomaticImage(t *testing.T) {
	runner := observationRunner{self: "/test/le", guest: true, kernel: "/test/vmlinuz",
		tag: Tag{File: "test/qemu/netns.ci"}}
	binaries := filepath.Join(t.TempDir(), "binaries")
	want := []string{runner.self, "test", "qemu", "docker-lab", "kernel", runner.kernel,
		testqemu.KeywordHarness, "timeout", guestCICommandTimeout,
		"command", testqemu.OneCICommand(binaries, runner.tag.File, "arm64")}
	if got := runner.guestCIArgv(binaries, "arm64"); !slices.Equal(got, want) {
		t.Fatalf("automatic-image functional argv = %q, want %q", got, want)
	}
}

// TestGuestImageRequiresPlacementAndKernel checks each invalid image placement
// is refused before the runner can retain an image no guest would consume.
func TestGuestImageRequiresPlacementAndKernel(t *testing.T) {
	for _, test := range []struct {
		name   string
		guest  bool
		kernel string
		reason string
	}{
		{name: "missing kernel", guest: true, reason: "needs the runtime kernel"},
		{name: "host placement", kernel: "/test/vmlinuz", reason: "runs on this host"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := observationRunner{guest: test.guest, kernel: test.kernel,
				record: DiscriminationRecord{Unit: "test.go::TestUnit"}}
			err := runner.selectGuestImage("/test/docker-host.img")
			if err == nil {
				t.Fatal("image without a consuming guest/kernel was accepted")
			}
			if !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("refusal = %q, want reason %q", err, test.reason)
			}
			if runner.image != "" {
				t.Fatalf("refused image was retained: %q", runner.image)
			}
		})
	}
}

// TestRecordNoBreakRejectsExplicitGuestImage takes the real observation entry
// point's no-break branch; an escape must not silently ignore an explicit image.
func TestRecordNoBreakRejectsExplicitGuestImage(t *testing.T) {
	record := DiscriminationRecord{Route: RouteNoBreak}
	_, err := observeFor("", nil, nil, Carrier{}, Tag{}, &record,
		recordRequest{kernel: "/test/vmlinuz", image: "/test/docker-host.img"}, nil)
	if err == nil {
		t.Fatal("no-break route accepted an image without observing a guest")
	}
	if !strings.Contains(err.Error(), "route no-break observes no guest") {
		t.Fatalf("no-break image refusal = %q", err)
	}
}

// TestRecordAnswerRetainsExplicitGuestImage follows the registered action into
// the real fixture corpus. Dropping image from the parsed request must lose this
// exact refusal for the existing declaration-only escape fixture.
// No compiler or VM is needed, and the refused action must write no record.
func TestRecordAnswerRetainsExplicitGuestImage(t *testing.T) {
	files := selftestDiscriminationSources()
	files[selftestSummaryRel] = selftestSummary
	files[FixtureAggregateRel] = FixtureAggregateProfile
	files[selftestWorkflowRel] = selftestWorkflow
	root := discriminationTree(t, files)
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	stderr, err := os.CreateTemp(t.TempDir(), "record-image-refusal-")
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = stderr
	t.Cleanup(func() {
		os.Stderr = saved
		if err := stderr.Close(); err != nil {
			t.Error(err)
		}
	})
	answer, code := actions.Answer([]string{
		"record", "id", selftestRIDDrop, "polarity", PolarityNegative,
		"unit", selftestCIPath, "route", RouteNoBreak,
		"producer", selftestCITablePath, "reason", escapeDeclaration,
		"image", filepath.Join(root, "accepted image", "docker-host.img"),
	})
	if code != 2 {
		t.Fatalf("record answered %d (%v), want image refusal 2", code, answer)
	}
	output, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "route no-break observes no guest") {
		t.Fatalf("record action lost the explicit image before observation: %s", output)
	}
	path := filepath.Join(root, filepath.FromSlash(discriminationRecordRel(selftestStem, selftestRIDDrop)))
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("refused action's record exists or cannot be checked: %v", err)
	}
}

// TestRecordObservationBindsExplicitGuestImage follows observeFor through the
// actual toy producer and host-unit placement. The image needs a kernel, so the
// real runner must reject it before observeBreaks can start a Go command.
// Removing only the image-binding call must lose this specific refusal.
func TestRecordObservationBindsExplicitGuestImage(t *testing.T) {
	files := selftestDiscriminationSources()
	files[selftestTestPath] = "package sample\n\nimport \"testing\"\n\n" +
		"// RFC requirement: RFC9999-2-1 positive -- SendWidget retains the requested count.\n" +
		"func TestWidget(t *testing.T) {\n\tif SendWidget(1) != 1 {\n" +
		"\t\tt.Fatal(\"the widget was not sent\")\n\t}\n}\n"
	checkout := checkoutRoot(t)
	pin, err := stdpatchgen.Pin(checkout)
	if err != nil {
		t.Fatal(err)
	}
	version, pinned := strings.CutPrefix(pin, "go")
	if !pinned {
		t.Fatalf("checkout's native Go toolchain has no version pin: %q", pin)
	}
	// The toy imports only testing. Its module carries the real toolchain pin,
	// not Ze's external requirements without their vendor population.
	files["go.mod"] = "module example.com/widget\n\ngo " + version + "\n"
	files["feature-gates.txt"] = "ze_widget internal/sample\n"
	root := discriminationTree(t, files)
	// A binding-loss cut really runs the toy package. Reuse the native cache
	// just as checkFixtureTree does, without copying source or build products.
	cache := filepath.Join(checkout, "cache")
	if err := os.MkdirAll(cache, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cache, filepath.Join(root, "cache")); err != nil {
		t.Fatal(err)
	}
	record := DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestDiscriminationUnit, Route: RouteRevert,
		Producer: selftestProducerUnit, Source: selftestDiscriminationRel}
	_, err = observeFor(root, newSourceReader(root), newScopeIndex(),
		Carrier{Kind: kindUnit}, Tag{File: selftestTestPath}, &record,
		recordRequest{image: filepath.Join(root, "docker-host.img")}, nil)
	if err == nil {
		t.Fatal("record observation accepted an explicit image without a kernel")
	}
	if !strings.Contains(err.Error(), "an explicit guest image needs the runtime kernel") {
		t.Fatalf("observation did not bind its image before execution: %v", err)
	}
	path := filepath.Join(root, filepath.FromSlash(selftestDiscriminationRel))
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("refused observation's record exists or cannot be checked: %v", err)
	}
}
