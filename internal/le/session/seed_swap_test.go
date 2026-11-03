// Design: docs/contributing/running-commands.md -- session seed entry confinement.
package session

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestSeedEntryReplacement changes an accepted entry before its first use.
// Real I/O and child launch must neither follow the replacement nor change the
// outside sentinel. Every swapped entry and outside target belongs to this test.
func TestSeedEntryReplacement(t *testing.T) {
	for _, target := range []string{"password", "config", "session", "store", "executable"} {
		t.Run(target, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_SESSION_ID", "seed-swap-owner")
			root := t.TempDir()
			rel := filepath.Join("tmp", "session", "2026-11-03-150000")
			dir := filepath.Join(root, rel)
			config := filepath.Join(dir, "etc", "ze")
			binary := filepath.Join(rel, "bin", "ze")
			for _, path := range []string{config, filepath.Join(dir, "bin")} {
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			for path, content := range map[string]string{
				filepath.Join(dir, ".session-id"):      "seed-swap-owner\n",
				filepath.Join(root, binary):            "#!/bin/sh\nexit 98\n",
				filepath.Join(config, ".dev-password"): "owned-password\n",
			} {
				if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			outside := t.TempDir()
			sentinel := filepath.Join(outside, "sentinel")
			if err := os.WriteFile(sentinel, []byte("outside-password\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(outside, "executed")
			replacement := filepath.Join(outside, "ze")
			if err := os.WriteFile(replacement, []byte("#!/bin/sh\nprintf foreign > '"+marker+"'\nexit 99\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			report, code, err := seedStoreAfterEntry(root, binary, streams{Out: io.Discard, Err: io.Discard}, func() {
				var path, link string
				switch target {
				case "password":
					path, link = filepath.Join(config, ".dev-password"), sentinel
				case "executable":
					path, link = filepath.Join(root, binary), replacement
				case "store":
					path, link = filepath.Join(config, "database"), outside
				case "config":
					path, link = config, outside
				case "session":
					path, link = dir, outside
				}
				if target != "store" {
					if err := os.Rename(path, path+".accepted"); err != nil {
						t.Fatal(err)
					}
				}
				if target == "config" {
					if err := os.Mkdir(filepath.Join(outside, "database"), 0o700); err != nil {
						t.Fatal(err)
					}
				}
				if target == "session" {
					if err := os.MkdirAll(filepath.Join(outside, "etc", "ze", "database"), 0o700); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(link, path); err != nil {
					t.Fatal(err)
				}
			})
			if code == 0 {
				t.Errorf("replacement accepted: report=%+v err=%v", report, err)
			}
			if target == "executable" {
				if report.ChildCode != 98 {
					t.Errorf("original executable did not run: child exit=%d, want 98", report.ChildCode)
				}
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Errorf("replacement executable ran: stat=%v", err)
			}
			content, err := os.ReadFile(sentinel)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != "outside-password\n" {
				t.Errorf("outside bytes changed: %q", content)
			}
			info, err := os.Stat(sentinel)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o640 {
				t.Errorf("outside mode changed: %v", info.Mode())
			}
		})
	}
}
