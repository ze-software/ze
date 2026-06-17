package imageserver

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureServerLog() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), &buf
}

func TestLogServedImageWithManifest(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"ze-20260101-000000.img", "ze-20260617-160000.img"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"appliance":"prod","timestamp":"20260617-160000","ze-version":"dev","arch":"amd64","image":"ze-20260617-160000.img","image-sha256":"deadbeef"}`
	if err := os.WriteFile(filepath.Join(dir, "build.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	log, buf := captureServerLog()
	logServedImage(log, dir)
	out := buf.String()

	for _, want := range []string{"ze-20260617-160000.img", "prod", "deadbeef", "image to install"} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "different image") {
		t.Errorf("unexpected mismatch warning:\n%s", out)
	}
}

func TestLogServedImageManifestMismatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ze-20260617-160000.img"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// manifest names an older image than the newest one actually on disk.
	manifest := `{"image":"ze-20260101-000000.img","timestamp":"20260101-000000"}`
	if err := os.WriteFile(filepath.Join(dir, "build.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	log, buf := captureServerLog()
	logServedImage(log, dir)
	if !strings.Contains(buf.String(), "different image") {
		t.Errorf("expected mismatch warning:\n%s", buf.String())
	}
}

func TestLogServedImageNoManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ze-1.img"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	log, buf := captureServerLog()
	logServedImage(log, dir)
	if !strings.Contains(buf.String(), "no build.json") {
		t.Errorf("expected missing-manifest warning:\n%s", buf.String())
	}
}

func TestLogServedImageNoImages(t *testing.T) {
	log, buf := captureServerLog()
	logServedImage(log, t.TempDir())
	if !strings.Contains(buf.String(), "no installable") {
		t.Errorf("expected no-image warning:\n%s", buf.String())
	}
}
