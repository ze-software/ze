package aihooks

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestHookSourcesCoverNativeProducers compares discovered source identities with
// direct package membership derived independently from native producer locations
// and fixture mappings, including runtime helpers and the reached summary.
// VALIDATES: every bounded producer package's production source is watched.
// PREVENTS: a helper changing while the source seal still describes old behavior.
func TestHookSourcesCoverNativeProducers(t *testing.T) {
	root := hookCheckout(t)
	paths, err := hookSourceFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	expected := hookAuthorityFiles(t, root, hookAuthorityPackages(t, root))
	if !slices.Equal(paths, expected) {
		t.Errorf("watched source identities = %v, want independently discovered producers %v", paths, expected)
	}
}

func hookProducerSource(t *testing.T, root string, producer any) string {
	t.Helper()
	address := reflect.ValueOf(producer).Pointer()
	source, _ := runtime.FuncForPC(address).FileLine(address)
	if !filepath.IsAbs(source) {
		source = strings.TrimPrefix(source, "github.com/ze-software/ze/")
		source = filepath.Join(root, filepath.FromSlash(source))
	}
	relative, err := filepath.Rel(root, source)
	if err != nil {
		t.Fatalf("locate native producer %s: %v", source, err)
	}
	return filepath.ToSlash(relative)
}

// TestEveryWatchedHookSourceDiscriminates compares each mutation with the
// computed current baseline, never with the stored seal (which may be stale).
// VALIDATES: every watched row changes the digest and a missing row refuses comparison.
// PREVENTS: a stale expected digest making every mutation appear discriminating.
func TestEveryWatchedHookSourceDiscriminates(t *testing.T) {
	root := copyHookSources(t)
	baseline, err := hookSourceContentDigest(root)
	if err != nil {
		t.Fatalf("read intact source baseline: %v", err)
	}
	hookAuthorityAccept(t, root, baseline)
	paths, err := hookSourceFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range paths {
		t.Run(relative, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(relative))
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read watched source: %v", err)
			}
			changed := append(slices.Clone(original), '\n')
			if err := os.WriteFile(path, changed, 0o600); err != nil {
				t.Fatalf("mutate watched source: %v", err)
			}
			digest, err := hookSourceContentDigest(root)
			if err != nil {
				t.Fatalf("read mutated source: %v", err)
			}
			if digest == baseline {
				t.Error("watched source mutation did not change the actual baseline digest")
			}
			hookAuthorityRefuse(t, root, baseline)
			if err := os.Remove(path); err != nil {
				t.Fatalf("remove watched source: %v", err)
			}
			if digest, err := hookSourceContentDigest(root); err == nil && digest == baseline {
				t.Error("missing watched source retained the unchanged source digest")
			}
			hookAuthorityRefuse(t, root, baseline)
			if err := os.WriteFile(path, original, info.Mode().Perm()); err != nil {
				t.Fatalf("restore watched source: %v", err)
			}
			restored, err := hookSourceContentDigest(root)
			if err != nil {
				t.Fatalf("read restored source: %v", err)
			}
			if restored != baseline {
				t.Fatal("restored watched source did not recover the actual baseline digest")
			}
			hookAuthorityAccept(t, root, baseline)
		})
	}
}
