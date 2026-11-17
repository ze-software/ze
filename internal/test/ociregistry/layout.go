// Design: docs/architecture/container/image-store.md -- the test registry serving an image from an OCI layout folder
// Related: registry.go -- the registry a layout is loaded into

package ociregistry

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The names of an OCI image layout folder, from the OCI image specification:
// its two files, its blob folders and the annotation that names a tag.
const (
	// LayoutIndexFile lists what the layout holds, each entry with its tag.
	LayoutIndexFile = "index.json"
	// LayoutMarkerFile marks the folder as a layout, and LayoutMarker is its
	// content.
	LayoutMarkerFile = "oci-layout"
	LayoutMarker     = `{"imageLayoutVersion":"1.0.0"}`
	// AnnotationRefName is the annotation of an index.json entry that names
	// the tag the entry is recorded under.
	AnnotationRefName = "org.opencontainers.image.ref.name"

	layoutBlobs     = "blobs"
	layoutAlgorithm = "sha256"
	layoutHexDigits = "0123456789abcdef"
	// layoutManifestSizeMax bounds a manifest, an index and index.json, the
	// only objects of a layout read into memory.
	layoutManifestSizeMax = 4 << 20
)

// layoutManifest reads an index, index.json included, and an image manifest:
// their fields do not overlap.
type layoutManifest struct {
	Manifests []Descriptor `json:"manifests"`
	Config    Descriptor   `json:"config"`
	Layers    []Descriptor `json:"layers"`
}

// SeedCommand is the line that writes the folder SeedLayoutDir names, as a
// failure for a missing seed prints it. The command is registered by
// internal/le/test/containerseed, whose tests hold this text to it.
const SeedCommand = "./le test container-seed fetch"

// SeedLayoutDir answers the layout folder that holds the seed of the published
// Redis image for the checkout at root. The folder sits behind the checkout's
// cache link, which leads to the developer cache outside the tracked tree, and
// which the QEMU guest reaches at the same place under its own mount of the
// checkout. The seed action writes the folder and the registry fixture reads
// it, so both ask here.
func SeedLayoutDir(root string) string {
	return filepath.Join(root, "cache", "container-seed", "redis")
}

// LayoutBlobPath answers the path of digest's blob inside a layout folder,
// blobs/sha256/<hex>. It refuses anything that is not sha256: followed by 64
// lowercase hexadecimal digits, so a digest read from a file never names a
// path outside the folder.
func LayoutBlobPath(digest string) (string, error) {
	hexText, ok := strings.CutPrefix(digest, layoutAlgorithm+":")
	if !ok {
		return "", errors.New("the digest " + strconv.Quote(digest) + " is not a sha256 digest")
	}
	if len(hexText) != sha256.Size*2 {
		return "", errors.New("the digest " + strconv.Quote(digest) + " does not hold 64 hexadecimal digits")
	}
	if strings.Trim(hexText, layoutHexDigits) != "" {
		return "", errors.New("the digest " + strconv.Quote(digest) + " holds a character that is not a lowercase hexadecimal digit")
	}
	return filepath.Join(layoutBlobs, layoutAlgorithm, hexText), nil
}

// PutLayout serves, as repository:tag and by digest, the image index that the
// OCI layout folder dir records under tag. It answers the index's descriptor
// and the platforms of the index whose images the folder holds: a layout MAY
// hold some entries of its index only, and a request for another entry answers
// that the manifest is unknown, so the caller MUST check that the platform it
// needs is in the answer.
//
// The index and the manifests are read into memory and checked against their
// digest and size. A configuration or a layer is checked against its size
// only, and is streamed from its file on each request: the folder MUST stay in
// place while the registry runs.
func (r *Registry) PutLayout(repository, tag, dir string) (Descriptor, []Platform, error) {
	layout, err := os.OpenRoot(dir)
	if err != nil {
		return Descriptor{}, nil, fmt.Errorf("ociregistry: layout: %w", err)
	}
	defer layout.Close() //nolint:errcheck // read-only root
	descriptor, platforms, err := r.putLayoutIndex(layout, dir, repository, tag)
	if err != nil {
		return Descriptor{}, nil, fmt.Errorf("ociregistry: layout %s: %w", dir, err)
	}
	return descriptor, platforms, nil
}

// putLayoutIndex stores the index recorded under tag and every image of it
// the layout holds.
func (r *Registry) putLayoutIndex(layout *os.Root, dir, repository, tag string) (Descriptor, []Platform, error) {
	entry, err := layoutTagged(layout, tag)
	if err != nil {
		return Descriptor{}, nil, err
	}
	body, err := readLayoutManifest(layout, entry)
	if err != nil {
		return Descriptor{}, nil, err
	}
	var index layoutManifest
	if err := json.Unmarshal(body, &index); err != nil {
		return Descriptor{}, nil, fmt.Errorf("the index %s: %w", entry.Digest, err)
	}
	if len(index.Manifests) == 0 {
		return Descriptor{}, nil, errors.New("the object recorded under the tag " + strconv.Quote(tag) + " is not an image index")
	}
	var platforms []Platform
	for _, member := range index.Manifests {
		held, err := r.putLayoutImage(layout, dir, repository, member)
		if err != nil {
			return Descriptor{}, nil, err
		}
		if !held {
			continue
		}
		if member.Platform != nil {
			platforms = append(platforms, *member.Platform)
		}
	}
	if len(platforms) == 0 {
		return Descriptor{}, nil, errors.New("the folder holds no image of the index " + entry.Digest)
	}
	return r.PutManifest(repository, tag, entry.MediaType, body), platforms, nil
}

// layoutTagged answers the entry of index.json recorded under tag.
func layoutTagged(layout *os.Root, tag string) (Descriptor, error) {
	data, err := readLayoutFile(layout, LayoutIndexFile)
	if err != nil {
		return Descriptor{}, err
	}
	var listing layoutManifest
	if err := json.Unmarshal(data, &listing); err != nil {
		return Descriptor{}, fmt.Errorf("%s: %w", LayoutIndexFile, err)
	}
	for _, entry := range listing.Manifests {
		if entry.Annotations[AnnotationRefName] == tag {
			return entry, nil
		}
	}
	return Descriptor{}, errors.New(LayoutIndexFile + " records nothing under the tag " + strconv.Quote(tag))
}

// putLayoutImage stores the image manifest member names, with its
// configuration and its layers, and answers whether the layout holds it. An
// index entry whose manifest file is absent is not held: the seed copies two
// platforms of an index that names more.
func (r *Registry) putLayoutImage(layout *os.Root, dir, repository string, member Descriptor) (bool, error) {
	name, err := LayoutBlobPath(member.Digest)
	if err != nil {
		return false, err
	}
	_, err = layout.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	body, err := readLayoutManifest(layout, member)
	if err != nil {
		return false, err
	}
	var image layoutManifest
	if err := json.Unmarshal(body, &image); err != nil {
		return false, fmt.Errorf("the manifest %s: %w", member.Digest, err)
	}
	if len(image.Manifests) > 0 {
		return false, errors.New("the index entry " + member.Digest + " is itself an index")
	}
	if image.Config.Digest == "" {
		return false, errors.New("the manifest " + member.Digest + " names no configuration")
	}
	if err := r.putLayoutBlob(layout, dir, image.Config); err != nil {
		return false, err
	}
	for _, layer := range image.Layers {
		if err := r.putLayoutBlob(layout, dir, layer); err != nil {
			return false, err
		}
	}
	r.PutManifest(repository, "", member.MediaType, body)
	return true, nil
}

// putLayoutBlob stores a configuration or a layer as its file in the layout,
// checked against the size its descriptor declares.
func (r *Registry) putLayoutBlob(layout *os.Root, dir string, blob Descriptor) error {
	name, err := LayoutBlobPath(blob.Digest)
	if err != nil {
		return err
	}
	info, err := layout.Stat(name)
	if err != nil {
		return fmt.Errorf("the blob %s of a manifest the folder holds: %w", blob.Digest, err)
	}
	if info.Size() != blob.Size {
		return fmt.Errorf("the blob %s holds %d bytes, and its descriptor declares %d", blob.Digest, info.Size(), blob.Size)
	}
	r.mu.Lock()
	r.blobs[blob.Digest] = stored{mediaType: blob.MediaType, path: filepath.Join(dir, name), size: blob.Size}
	r.mu.Unlock()
	return nil
}

// readLayoutManifest reads the manifest or the index descriptor names and
// checks it against the descriptor's size and digest.
func readLayoutManifest(layout *os.Root, descriptor Descriptor) ([]byte, error) {
	name, err := LayoutBlobPath(descriptor.Digest)
	if err != nil {
		return nil, err
	}
	data, err := readLayoutFile(layout, name)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != descriptor.Size {
		return nil, fmt.Errorf("the manifest %s holds %d bytes, and its descriptor declares %d", descriptor.Digest, len(data), descriptor.Size)
	}
	if got := digestOf(data); got != descriptor.Digest {
		return nil, errors.New("the manifest " + descriptor.Digest + " hashes to " + got)
	}
	return data, nil
}

// readLayoutFile reads a file of the layout that is held in memory, refusing
// one over layoutManifestSizeMax.
func readLayoutFile(layout *os.Root, name string) ([]byte, error) {
	file, err := layout.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // read-only file
	data, err := io.ReadAll(io.LimitReader(file, layoutManifestSizeMax+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if len(data) > layoutManifestSizeMax {
		return nil, errors.New(name + " is larger than the 4 MiB a manifest may hold")
	}
	return data, nil
}
