// Design: docs/architecture/appliance/builder.md -- named updater identity.

package appliance

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNamedBuildEmbedsStoredUpdateIdentity(t *testing.T) {
	root := writeGokrazyFixture(t)
	if err := os.MkdirAll(filepath.Join(root, "gokrazy", "modcache"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/ze-software/ze\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gokrazy", "ze", "ze.conf"), []byte("set system host fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := initTestAppliance(t, "update-device", nil)
	oldMkfs, oldDebugfs := e2fsMkfs, e2fsDebugfs
	e2fsMkfs, e2fsDebugfs = "fixture-mkfs", "fixture-debugfs"
	t.Cleanup(func() { e2fsMkfs, e2fsDebugfs = oldMkfs, oldDebugfs })
	var prepared string
	oldBuild := gokBuildFn
	gokBuildFn = func(args []string) error {
		prepared = args[1]
		path := filepath.Join(prepared, "ze", "config.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document struct {
			Update struct {
				HTTPPassword string
				HTTPSPort    string
				UseTLS       string
			}
		}
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		paths := strings.Split(document.Update.UseTLS, ",")
		if len(paths) != 2 {
			t.Errorf("prepared Update.UseTLS=%q, want the custom cert,key paths the upstream consumer reads", document.Update.UseTLS)
			return errors.New("fixture cannot inspect absent upstream TLS paths")
		}
		for index, leaf := range []string{"cert.pem", "key.pem"} {
			material, err := os.ReadFile(paths[index])
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(tLSDir(dir, "update-device"), leaf))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(material, want) {
				t.Errorf("upstream custom TLS file %s differs from the appliance's %s", paths[index], leaf)
			}
			info, err := os.Stat(paths[index])
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Errorf("custom TLS material %s has mode %o", leaf, info.Mode().Perm())
			}
		}
		for _, entry := range []struct{ path, got string }{
			{secretFilePath(dir, "update-device", "update.token"), document.Update.HTTPPassword},
		} {
			want, err := os.ReadFile(entry.path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(entry.got), want) {
				t.Errorf("prepared Update omits or changes %s", filepath.Base(entry.path))
			}
		}
		if document.Update.HTTPSPort != "443" {
			t.Errorf("update HTTPS port=%q, want 443", document.Update.HTTPSPort)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("prepared configuration mode=%o exposes update secrets", info.Mode().Perm())
		}
		return errors.New("fixture stops after observing the prepared config")
	}
	t.Cleanup(func() { gokBuildFn = oldBuild })
	if code := buildOne("update-device"); code != exitError {
		t.Fatalf("build code=%d, want fixture stop", code)
	}
	if prepared == "" {
		t.Fatal("builder did not reach the prepared gokrazy configuration")
	}
	if _, err := os.Stat(prepared); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("prepared update secrets survive builder failure: %v", err)
	}
}
