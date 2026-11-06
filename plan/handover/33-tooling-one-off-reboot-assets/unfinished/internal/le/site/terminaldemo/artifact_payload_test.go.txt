// Design: docs/contributing/gh-pages.md -- verified payload ownership survives source mutation.
package siteterminaldemo

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestRetainedAssetPublishesVerifiedBytes changes the same inode or replaces the
// path after acquisition, then verifies the publisher writes the original bytes.
func TestRetainedAssetPublishesVerifiedBytes(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		t.Run(map[bool]string{false: "in-place", true: "path-swap"}[replacement], func(t *testing.T) {
			source := t.TempDir()
			const original = "verified recording\n"
			path := filepath.Join(source, "recording.cast")
			if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte(original))
			asset, err := ReadAsset(source, AssetMetadata{Path: "recording.cast", Bytes: int64(len(original)), SHA256: hex.EncodeToString(digest[:])})
			if err != nil {
				t.Fatal(err)
			}
			if replacement {
				if err := os.Rename(path, path+".old"); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(path, []byte("unverified change!\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			target := t.TempDir()
			root, err := os.OpenRoot(target)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close() //nolint:errcheck // The publication verdict owns the write result.
			if err := asset.Publish(root, "recording.cast"); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(target, "recording.cast"))
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != original {
				t.Fatalf("published unverified payload: %q", content)
			}
		})
	}
}

// TestRetainedAssetRefusesInvalidMetadataAndZero proves acquisition checks size
// and digest, and a zero-value payload cannot silently publish an empty file.
func TestRetainedAssetRefusesInvalidMetadataAndZero(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "asset"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("payload"))
	for _, metadata := range []AssetMetadata{
		{Path: "asset", Bytes: 8, SHA256: hex.EncodeToString(digest[:])},
		{Path: "asset", Bytes: 7, SHA256: "incorrect"},
	} {
		if _, err := ReadAsset(source, metadata); err == nil {
			t.Fatalf("accepted invalid metadata: %+v", metadata)
		}
	}
	if _, err := (Asset{}).Text(); err == nil {
		t.Fatal("uninitialized asset answered text")
	}
	if err := (Asset{}).Publish(nil, "asset"); err == nil {
		t.Fatal("uninitialized asset published")
	}
}
