package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadImageManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build.json")
	manifest := `{"appliance":"prod","timestamp":"20260617-160000","ze-version":"dev","arch":"amd64","image":"ze-20260617-160000.img","image-sha256":"abc123"}`
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	old := imageManifestPath
	imageManifestPath = path
	t.Cleanup(func() { imageManifestPath = old })

	got, ok := readImageManifest()
	if !ok {
		t.Fatal("expected manifest to load")
	}
	if got.Image != "ze-20260617-160000.img" || got.Timestamp != "20260617-160000" || got.Appliance != "prod" {
		t.Errorf("unexpected manifest: %+v", got)
	}

	out := Extended()
	for _, want := range []string{"ze-20260617-160000.img", "20260617-160000", "prod", "abc123"} {
		if !strings.Contains(out, want) {
			t.Errorf("Extended() missing %q:\n%s", want, out)
		}
	}
}

func TestReadImageManifestAbsent(t *testing.T) {
	old := imageManifestPath
	imageManifestPath = filepath.Join(t.TempDir(), "nope.json")
	t.Cleanup(func() { imageManifestPath = old })

	if _, ok := readImageManifest(); ok {
		t.Error("expected ok=false when manifest missing")
	}
	if strings.Contains(Extended(), "image:") {
		t.Error("Extended() should omit the image section when no manifest is present")
	}
}

func TestReadImageManifestMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "build.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := imageManifestPath
	imageManifestPath = path
	t.Cleanup(func() { imageManifestPath = old })

	if _, ok := readImageManifest(); ok {
		t.Error("expected ok=false on malformed manifest")
	}
}
