// Design: docs/contributing/gh-pages.md -- acquisition, verification and publication share immutable bytes.
package siteterminaldemo

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var errAssetMismatch = errors.New("artifact size or digest mismatch")

// Asset retains an acquired demonstration's immutable bytes. Its zero value is
// uninitialized and its operations refuse it. It is safe for concurrent reading;
// publication to the same destination MUST be serialized by the caller.
// A descriptor alone would still observe writes through another descriptor.
type Asset struct {
	content  string
	metadata AssetMetadata
}

// ReadAsset acquires a file through root and verifies the retained payload,
// not a path that a parser or publisher would have to reopen. The caller opens
// root through the anchor the media must stay inside (OpenMediaRoot), and
// publishes through the same root, so a link that leads out of that anchor is
// refused before any byte is read.
func ReadAsset(root *os.Root, metadata AssetMetadata) (Asset, error) {
	asset, err := acquireAsset(root, metadata.Path, &metadata)
	if err != nil {
		return Asset{}, err
	}
	if asset.metadata.Bytes != metadata.Bytes {
		return Asset{}, fmt.Errorf("%s: %w; restore the recorded media", metadata.Path, errAssetMismatch)
	}
	if asset.metadata.SHA256 != metadata.SHA256 {
		return Asset{}, fmt.Errorf("%s: %w; restore the recorded media", metadata.Path, errAssetMismatch)
	}
	return asset, nil
}

// acquireAsset reads name through root, which refuses a link that leads out
// of it. The caller owns root and closes it.
func acquireAsset(root *os.Root, name string, expected *AssetMetadata) (Asset, error) {
	if root == nil {
		return Asset{}, errors.New("demo acquisition root is absent")
	}
	handle, err := root.Open(name)
	if err != nil {
		return Asset{}, err
	}
	defer handle.Close() //nolint:errcheck // The retained payload owns the read verdict.
	info, err := handle.Stat()
	if err != nil {
		return Asset{}, err
	}
	if !info.Mode().IsRegular() {
		return Asset{}, fmt.Errorf("%s: demo artifact is not a regular file; restore the recorded media", name)
	}
	if info.Size() <= 0 {
		return Asset{}, fmt.Errorf("%s: demo artifact is empty; restore the recorded media", name)
	}
	if uint64(info.Size()) > uint64(^uint(0)>>1) {
		return Asset{}, fmt.Errorf("%s: demo artifact is too large to retain", name)
	}
	if expected != nil {
		if info.Size() != expected.Bytes {
			return Asset{}, fmt.Errorf("%s: %w; restore the recorded media", name, errAssetMismatch)
		}
	}
	// The acquired file size bounds both allocation and reading. Growth after
	// acquisition is refused; a same-size write is checked on the retained bytes.
	var content strings.Builder
	content.Grow(int(info.Size()))
	digest := sha256.New()
	if _, err := io.CopyN(io.MultiWriter(&content, digest), handle, info.Size()); err != nil {
		return Asset{}, err
	}
	var extra [1]byte
	count, err := handle.Read(extra[:])
	if count != 0 {
		return Asset{}, fmt.Errorf("%s: demo artifact grew during acquisition; retry with stable recorded media", name)
	}
	if !errors.Is(err, io.EOF) {
		if err != nil {
			return Asset{}, err
		}
		return Asset{}, fmt.Errorf("%s: demo artifact did not reach EOF during acquisition", name)
	}
	return Asset{content: content.String(), metadata: AssetMetadata{
		Path: name, Bytes: info.Size(), SHA256: hex.EncodeToString(digest.Sum(nil)),
	}}, nil
}

// OpenMediaRoot opens the media directory every acquisition and publication
// goes through. With an anchor, media MUST lie inside it: the anchor is opened
// and media is reached from it, so a link at media or at any directory between
// the two that leads outside the anchor is refused ("path escapes from
// parent") rather than followed. An empty anchor makes media its own boundary,
// as for the render actions' operator-chosen output directory. create makes
// the directory first, through the same anchor. The caller closes the root.
func OpenMediaRoot(anchor, media string, create bool) (*os.Root, error) {
	if anchor == "" {
		if create {
			if err := os.MkdirAll(media, 0o755); err != nil { // #nosec G301 -- Public website artifacts must be traversable by the web server.
				return nil, err
			}
		}
		return os.OpenRoot(media)
	}
	relative, err := filepath.Rel(anchor, media)
	if err != nil {
		return nil, fmt.Errorf("demo media %s is not inside %s: %w", media, anchor, err)
	}
	if !filepath.IsLocal(relative) {
		return nil, fmt.Errorf("demo media %s is not inside %s; set the artifact root inside the site output", media, anchor)
	}
	outer, err := os.OpenRoot(anchor)
	if err != nil {
		return nil, err
	}
	defer outer.Close() //nolint:errcheck // The opened media root holds its own descriptor.
	if create {
		if err := outer.MkdirAll(relative, 0o755); err != nil { // #nosec G301 -- Public website artifacts must be traversable by the web server.
			return nil, err
		}
	}
	return outer.OpenRoot(relative)
}

func assetFromText(name, content string) Asset {
	digest := sha256.New()
	_, _ = io.WriteString(digest, content) //nolint:errcheck // A hash writer cannot fail.
	return Asset{content: content, metadata: AssetMetadata{
		Path: name, Bytes: int64(len(content)), SHA256: hex.EncodeToString(digest.Sum(nil)),
	}}
}

// Text answers immutable content and refuses an uninitialized Asset. No mutable
// alias to the acquired bytes crosses this API.
func (asset Asset) Text() (string, error) {
	if asset.content == "" {
		return "", errors.New("demo asset is uninitialized; acquire the recorded media first")
	}
	return asset.content, nil
}

// Metadata describes the retained bytes, never the current contents of a path.
func (asset Asset) Metadata() AssetMetadata { return asset.metadata }

// Publish atomically replaces one root-relative file with the retained bytes.
// Callers MUST serialize publications to the same destination. A failure before
// replacement preserves that destination; this is not a multi-file transaction.
func (asset Asset) Publish(root *os.Root, name string) error {
	content, err := asset.Text()
	if err != nil {
		return err
	}
	if root == nil {
		return errors.New("demo publication root is absent")
	}
	return replaceArtifact(root, name, strings.NewReader(content))
}

func replaceArtifact(root *os.Root, name string, content io.Reader) error {
	if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	parent, err := root.OpenRoot(filepath.Dir(name))
	if err != nil {
		return err
	}
	defer parent.Close() //nolint:errcheck // The file close and rename own the write verdict.
	temporary := ".demo-" + rand.Text()
	output, err := parent.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer parent.Remove(temporary) //nolint:errcheck // A successful rename already removed the temporary name.
	_, writeErr := io.Copy(output, content)
	modeErr := output.Chmod(0o644) //nolint:gosec // Public website content must be readable by the web server.
	closeErr := output.Close()
	if err := errors.Join(writeErr, modeErr, closeErr); err != nil {
		return err
	}
	return parent.Rename(temporary, filepath.Base(name))
}
