package interop_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	interopbgp "github.com/ze-software/ze/internal/le/interoplab/bgp"
)

// TestInteropRunnerFailsClosedWithoutDocker validates the native runner's
// prerequisite polarity without launching an interpreter or a probe program.
func TestInteropRunnerFailsClosedWithoutDocker(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	report := interopbgp.RunAt(t.Context(), root, interopbgp.Options{Scenario: "bgp-ebgp-gobgp"})
	if report.Code == 0 {
		t.Fatalf("native interop runner passed with Docker unavailable: %+v", report)
	}
	text := strings.ToLower(report.SetupError)
	if !strings.Contains(text, "docker") {
		t.Errorf("missing prerequisite must name Docker, got %+v", report)
	}
	if strings.Contains(text, "skip") {
		t.Errorf("missing Docker must be a failure, not a skip: %+v", report)
	}
}

// TestInteropTreeUsesOnlyCompiledHelpers makes the cutover executable: a new
// scenario cannot reintroduce an interpreter path, shell helper, or source probe.
func TestInteropTreeUsesOnlyCompiledHelpers(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	interopRoot := filepath.Join(root, "test", "interop")
	// The lab's own cross-compiled binaries live in this tree because it is the
	// Docker build context (interoplab.LabBinary.Output), and a 84 MB Go binary
	// carries every one of the tokens below as an ordinary string constant. The
	// set is READ from the lab rather than spelled here, so a lab that stages a
	// third personality needs no edit and a file that is NOT declared as a build
	// product is still scanned.
	staged := map[string]bool{}
	for _, binary := range interopbgp.LabBinaries() {
		within, relErr := filepath.Rel(interopRoot, filepath.Join(root, filepath.FromSlash(binary.Output)))
		if relErr != nil {
			t.Fatal(relErr)
		}
		staged[within] = true
	}
	err = filepath.WalkDir(interopRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(interopRoot, path)
		if err != nil {
			return err
		}
		if staged[relative] {
			return nil
		}
		lowerName := strings.ToLower(entry.Name())
		for _, suffix := range []string{"." + "py", "." + "pyc", "." + "sh", "." + "bash", "." + "pl"} {
			if strings.HasSuffix(lowerName, suffix) {
				t.Errorf("interpreted-language artifact remains: %s", relative)
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := strings.ToLower(string(data))
		for _, token := range []string{
			"python" + "3",
			"entrypoint\", \"" + "python",
			"#!/" + "bin/sh",
			"#!/usr/bin/env ba" + "sh",
		} {
			if strings.Contains(text, token) {
				t.Errorf("interpreter launch remains in %s", relative)
			}
		}
		for _, extension := range []string{"." + "sh", "." + "bash", "." + "pl"} {
			if namesScriptExtension(text, extension) {
				t.Errorf("interpreter launch remains in %s", relative)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// namesScriptExtension reports whether text names a file ending in extension:
// an occurrence the next byte does not continue as a name. A bare substring
// match read the sysctl key net.mpls.platform_labels as a Perl script name,
// and would read any dotted word that begins with "sh" as a shell script.
func namesScriptExtension(text, extension string) bool {
	for rest := text; ; {
		at := strings.Index(rest, extension)
		if at < 0 {
			return false
		}
		rest = rest[at+len(extension):]
		if rest == "" || !continuesName(rest[0]) {
			return true
		}
	}
}

// continuesName reports whether b can continue a file-name extension.
func continuesName(b byte) bool {
	return b == '_' || b == '-' || ('a' <= b && b <= 'z') || ('0' <= b && b <= '9')
}

// TestNamesScriptExtension pins the boundary the tree check reads: a script
// name at the end of text, before a quote, a space or a dot, is found, and a
// dotted word that only begins with the extension's letters is not.
func TestNamesScriptExtension(t *testing.T) {
	cases := []struct {
		text, extension string
		want            bool
	}{
		{"cmd [\"/run" + "." + "sh\"]", "." + "sh", true},
		{"entry" + "." + "sh", "." + "sh", true},
		{"helper" + "." + "pl.in", "." + "pl", true},
		{"start" + "." + "bash run", "." + "bash", true},
		{"net.mpls" + "." + "platform_labels=1048575", "." + "pl", false},
		{"lib" + "." + "shared-1", "." + "sh", false},
		{"lib" + "." + "shared then x" + "." + "sh", "." + "sh", true},
	}
	for _, c := range cases {
		if got := namesScriptExtension(c.text, c.extension); got != c.want {
			t.Errorf("namesScriptExtension(%q, %q) = %v, want %v", c.text, c.extension, got, c.want)
		}
	}
}
