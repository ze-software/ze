//go:build linux || darwin || freebsd || openbsd || netbsd

// Design: docs/contributing/running-commands.md -- inherited directory capability.
package seedinit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInheritedDirectoryConversion(t *testing.T) {
	if len(os.Args) > 2 && os.Args[len(os.Args)-2] == "--seed-directory-child" {
		root, folder, err := inheritedDirectory("durable-label")
		if os.Args[len(os.Args)-1] == "refuse" {
			if err == nil {
				root.Close()
				folder.Close()
				t.Fatal("invalid directory accepted")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()   //nolint:errcheck // Child test root.
		defer folder.Close() //nolint:errcheck // Child inherited descriptor.
		data, err := root.ReadFile("sentinel")
		if err != nil || string(data) != "held" {
			t.Fatalf("wrong root: %q %v", data, err)
		}
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"renamed", "public", "regular", "missing"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "sentinel"), []byte("held"), 0o600); err != nil {
				t.Fatal(err)
			}
			if kind == "public" {
				if err := os.Chmod(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "regular" {
				path = filepath.Join(path, "sentinel")
			}
			folder, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer folder.Close() //nolint:errcheck // Parent test descriptor.
			expect := "refuse"
			if kind == "renamed" {
				expect = "accept"
				if err := os.Rename(path, path+".held"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "sentinel"), []byte("replacement"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(executable, "-test.run=^TestInheritedDirectoryConversion$", "--", "--seed-directory-child", expect)
			if kind != "missing" {
				cmd.ExtraFiles = []*os.File{folder, folder}
			}
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("child: %v\n%s", err, output)
			}
		})
	}
}
