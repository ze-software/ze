// Design: docs/architecture/container/image-store.md -- the writable layer folder the image store hands a container

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStoredUpperFindsTheOneDigestFolder proves the host side of a fetched
// image's test finds the container's writable layer where the image store
// makes it, and refuses every other finding rather than read a wrong folder.
//
// Method: a data directory per case, holding under container/layers/web the
// folders the case names, judged by storedUpper. Every case holds at least
// one entry, so no case waits.
func TestStoredUpperFindsTheOneDigestFolder(t *testing.T) {
	digest := layerDigestPrefix + strings.Repeat("ab", layerDigestHexLength/2)
	other := layerDigestPrefix + strings.Repeat("cd", layerDigestHexLength/2)
	cases := []struct {
		name    string
		folders []string
		found   bool
	}{
		{"one digest folder", []string{digest}, true},
		{"two digest folders", []string{digest, other}, false},
		{"a folder named by no digest", []string{"upper"}, false},
		{"a short digest", []string{layerDigestPrefix + "ab"}, false},
		{"an upper-case digest", []string{strings.ToUpper(digest)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := t.TempDir()
			for _, folder := range tc.folders {
				if err := os.MkdirAll(filepath.Join(data, storeLayersFolder, "web", folder), 0o750); err != nil {
					t.Fatalf("the test folder could not be made: %v", err)
				}
			}
			upper, found := storedUpper(data, "web")
			if found != tc.found {
				t.Fatalf("storedUpper answered found=%v, want %v", found, tc.found)
			}
			if !found {
				return
			}
			want := filepath.Join(data, storeLayersFolder, "web", digest, "upper")
			if upper != want {
				t.Fatalf("storedUpper answered %q, want %q", upper, want)
			}
		})
	}
}
