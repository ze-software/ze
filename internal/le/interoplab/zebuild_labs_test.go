// zebuild_labs_test.go is the external test package, because these two tests
// read what the five LABS declare and a lab imports interoplab. The producer's
// own tests are zebuild_test.go, inside the package, where the process runner
// seam lives.
package interoplab_test

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/goversion"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/bgp"
	"github.com/ze-software/ze/internal/le/interoplab/ipsec"
	"github.com/ze-software/ze/internal/le/interoplab/l2tp"
	"github.com/ze-software/ze/internal/le/interoplab/pppoe"
	"github.com/ze-software/ze/internal/le/interoplab/radius"
	"github.com/ze-software/ze/internal/le/lepath"
)

// declaredBinaries answers every binary the five interop labs stage, which is
// the one record of what a lab image expects to find in its build context.
func declaredBinaries() []interoplab.LabBinary {
	var all []interoplab.LabBinary
	all = append(all, bgp.LabBinaries()...)
	all = append(all, ipsec.LabBinaries()...)
	all = append(all, l2tp.LabBinaries()...)
	all = append(all, pppoe.LabBinaries()...)
	all = append(all, radius.LabBinaries()...)
	return all
}

// VALIDATES: no lab image carries a Go compiler -- every tracked test/interop*/Dockerfile.ze copies a prebuilt binary in.
// PREVENTS: a fourth lab written back into the shape the kernel killed three times on an idle 31 GiB host, and the 40m39s build it also caused.
func TestZeDockerfilesCarryNoCompiler(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	// The walk finds the files rather than a list naming them, so a lab added
	// later is judged without this test being edited.
	matches, err := filepath.Glob(filepath.Join(root, "test", "interop*", "Dockerfile.ze"))
	if err != nil {
		t.Fatalf("walk the lab Dockerfiles: %v", err)
	}
	if len(matches) < 5 {
		t.Fatalf("the walk found %d lab Dockerfiles, and this repository has five labs: %v", len(matches), matches)
	}

	for _, match := range matches {
		body, readErr := os.ReadFile(match) //nolint:gosec // a test reads the checkout it was pointed at
		if readErr != nil {
			t.Fatalf("read %s: %v", match, readErr)
		}
		number := 0
		for line := range strings.SplitSeq(string(body), "\n") {
			number++
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.Contains(trimmed, "go build") {
				t.Errorf("%s:%d compiles inside the image: %s", match, number, trimmed)
			}
			// The needle comes from the Go version gate rather than from a
			// literal here, which keeps one declaration of the image name and
			// keeps this line off that gate's carrier walk.
			if strings.HasPrefix(strings.ToUpper(trimmed), "FROM ") {
				if strings.Contains(trimmed, goversion.ImagePrefix) {
					t.Errorf("%s:%d names a golang base image: %s", match, number, trimmed)
				}
			}
		}
	}
}

// VALIDATES: .dockerignore admits every staging path the labs declare, and the file's negations name nothing the labs do not.
// PREVENTS: R-3, a `docker build` that fails at the COPY with "file not found" because test/ is excluded and the negation was never written.
func TestDockerIgnoreAdmitsEveryStagedLabBinary(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".dockerignore")) //nolint:gosec // a test reads the checkout it was pointed at
	if err != nil {
		t.Fatalf("read .dockerignore: %v", err)
	}

	var negated []string
	for line := range strings.SplitSeq(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "!") {
			continue
		}
		negated = append(negated, strings.TrimPrefix(trimmed, "!"))
	}

	declared := declaredBinaries()
	if len(declared) == 0 {
		t.Fatal("the five labs declare no staged binary at all")
	}
	for _, binary := range declared {
		if !slices.Contains(negated, binary.Output) {
			t.Errorf(".dockerignore does not admit %s, the path lab binary %q is staged at", binary.Output, binary.Name)
		}
	}

	// The other direction: a negation for a lab path no lab declares is a
	// staging path that moved and left its admission behind.
	for _, admitted := range negated {
		if !strings.HasPrefix(admitted, "test/interop") {
			continue
		}
		if path.Ext(admitted) != "" || strings.HasSuffix(admitted, "/") {
			continue
		}
		if slices.ContainsFunc(declared, func(binary interoplab.LabBinary) bool { return binary.Output == admitted }) {
			continue
		}
		t.Errorf(".dockerignore admits %s, which no lab declares as a staging path", admitted)
	}
}
