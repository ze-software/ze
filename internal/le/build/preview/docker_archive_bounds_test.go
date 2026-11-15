// Design: docs/contributing/preview-artifacts.md -- bounded image archive scanning.
package buildpreview

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scanDockerArchiveBytes scans a small, possibly incomplete outer tar without
// requiring a valid image graph to reach the entry-reading boundary.
func scanDockerArchiveBytes(t *testing.T, body []byte) error {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "image.docker.tar")
	if err := os.WriteFile(filename, body, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // Read-only fixture.
	_, err = scanImageArchive(file)
	return err
}

// TestDockerArchiveRefusesSparseOuterEntries proves outer files cannot expand
// holes while being hashed or invalidate saved physical offsets for later reads.
// Method: use valid PAX sparse 0.1 and 1.0 entries with only 2 KiB of logical
// zeros. Both must be refused as sparse before the scanner hashes their body.
func TestDockerArchiveRefusesSparseOuterEntries(t *testing.T) {
	for _, version := range []string{"0.1", "1.0"} {
		t.Run(version, func(t *testing.T) {
			records := paxRecord("GNU.sparse.name", "unknown") +
				paxRecord("GNU.sparse.realsize", "2048")
			var body []byte
			if version == "0.1" {
				records += paxRecord("GNU.sparse.major", "0") +
					paxRecord("GNU.sparse.minor", "1") +
					paxRecord("GNU.sparse.numblocks", "0")
			} else {
				records += paxRecord("GNU.sparse.major", "1") +
					paxRecord("GNU.sparse.minor", "0")
				body = padTarBlock([]byte("0\n"))
			}
			archive := rawTarHeader(t, "PaxHeaders.0/unknown", tar.TypeXHeader, 0o644, int64(len(records)))
			archive = append(archive, padTarBlock([]byte(records))...)
			archive = append(archive, rawTarHeader(t, "unknown", tar.TypeReg, 0o644, int64(len(body)))...)
			archive = append(archive, body...)
			archive = append(archive, make([]byte, 1024)...)
			if err := scanDockerArchiveBytes(t, archive); err == nil {
				t.Fatal("the scanner accepted a sparse outer file and hashed its synthesized holes")
			} else if !strings.Contains(err.Error(), "sparse") {
				t.Fatalf("the sparse outer file was refused for another reason: %v", err)
			}
		})
	}
}

// TestDockerArchiveRefusesOversizedOuterEntryBeforeHash proves every outer
// file, including one outside the image graph, has a declared-size bound.
// Method: provide a PAX header declaring one byte beyond the existing layer
// limit but no body. Require the size refusal rather than a hash read's EOF.
func TestDockerArchiveRefusesOversizedOuterEntryBeforeHash(t *testing.T) {
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	if err := writer.WriteHeader(&tar.Header{Name: "unknown", Size: layerSizeLimit + 1, Mode: 0o644, Typeflag: tar.TypeReg, Format: tar.FormatPAX}); err != nil {
		t.Fatal(err)
	}
	// Intentionally omit the body and Close: this is a truncated input, whose
	// impossible declared size must be refused before reading any file content.
	err := scanDockerArchiveBytes(t, archive.Bytes())
	if err == nil {
		t.Fatal("the scanner accepted an outer file beyond the size limit")
	}
	if !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("the oversized outer file was read instead of refused before hashing: %v", err)
	}
}

// TestDockerArchiveAcceptsSparseFilesystemLayer proves the outer archive rule
// preserves legitimate sparse files inside a filesystem layer. Method: put a
// small PAX sparse tar in a dense legacy layer, bind its raw tar diffID through
// the config and immutable image ID, and require the complete image check.
func TestDockerArchiveAcceptsSparseFilesystemLayer(t *testing.T) {
	records := paxRecord("GNU.sparse.major", "1") +
		paxRecord("GNU.sparse.minor", "0") +
		paxRecord("GNU.sparse.name", "sparse-file") +
		paxRecord("GNU.sparse.realsize", "2048")
	layer := rawTarHeader(t, "PaxHeaders.0/sparse-file", tar.TypeXHeader, 0o644, int64(len(records)))
	layer = append(layer, padTarBlock([]byte(records))...)
	layer = append(layer, rawTarHeader(t, "sparse-file", tar.TypeReg, 0o644, 512)...)
	layer = append(layer, padTarBlock([]byte("0\n"))...)
	layer = append(layer, make([]byte, 1024)...)
	fixture := savedDockerFixture(t, "legacy")
	fixture.files[fixture.layers[0]] = layer
	var config savedConfig
	if err := json.Unmarshal(fixture.files[fixture.config], &config); err != nil {
		t.Fatal(err)
	}
	config.RootFS.DiffIDs[0] = digestBytes(layer)
	body := fixtureJSON(t, config)
	delete(fixture.files, fixture.config)
	fixture.image.ID = digestBytes(body)
	fixture.config = strings.TrimPrefix(fixture.image.ID, "sha256:") + ".json"
	fixture.files[fixture.config] = body
	fixture.files["manifest.json"] = fixtureJSON(t, []compatibilityImage{{Config: fixture.config, Layers: fixture.layers}})
	if err := checkDockerArchive(writeDockerFixture(t, fixture), fixture.image); err != nil {
		t.Fatalf("a dense image archive with a sparse filesystem layer was refused: %v", err)
	}
}

// layerTestBound is the uncompressed byte budget the layer tests hand the
// check: far below the production layer limit so the oversized layers stay
// small, and far above the small layers used as controls.
const layerTestBound int64 = 64 << 10

// layerContent stores layer as the only entry, layer.tar, of an outer archive
// whose content the layer check reads by offset, and returns that content.
func layerContent(t *testing.T, layer []byte) *archiveContent {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "layer.tar")
	if err := os.WriteFile(filename, layer, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() }) //nolint:errcheck // Read-only fixture.
	return &archiveContent{
		file:    file,
		entries: map[string]archiveEntry{"layer.tar": {offset: 0, size: int64(len(layer))}},
		used:    map[string]bool{},
	}
}

// boundLayerTar writes a raw filesystem layer holding one regular file of the
// given body, then trailing zero bytes after the tar end marker.
func boundLayerTar(t *testing.T, body []byte, trailing int) []byte {
	t.Helper()
	var layer bytes.Buffer
	writer := tar.NewWriter(&layer)
	if err := writer.WriteHeader(&tar.Header{Name: "file", Size: int64(len(body)), Mode: 0o644, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return append(layer.Bytes(), make([]byte, trailing)...)
}

// TestDockerLayerRefusesStreamBeyondSizeLimit proves that a filesystem layer
// whose uncompressed stream runs past the bound is refused with the size
// limit wherever the budget ran out: inside an entry body the tar reader
// skips, and in the bytes after the tar end marker that the check drains.
// Method: build raw layers of sixteen times a lowered bound at each place,
// bind each to its true diffID so only the bound can refuse it, and require
// an error naming the size limit, never the raw short read. Without the
// budget check right after the tar reader's Next, the skipped-body case
// answers "invalid filesystem layer layer.tar: unexpected EOF"; without the
// one after the drain, the trailing case is refused for the wrong reason: the
// digest covers only the bytes the budget let through, so it answers that the
// layer differs from its ordered config diffID, and never names the size limit.
func TestDockerLayerRefusesStreamBeyondSizeLimit(t *testing.T) {
	oversized := int(16 * layerTestBound)
	cases := []struct {
		name  string
		layer []byte
	}{
		{name: "a skipped entry body", layer: boundLayerTar(t, make([]byte, oversized), 0)},
		{name: "trailing data after the tar end marker", layer: boundLayerTar(t, []byte("file"), oversized)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := layerContent(t, tc.layer)
			err := content.layerWithin("layer.tar", "application/vnd.oci.image.layer.v1.tar", digestBytes(tc.layer), layerTestBound)
			if err == nil {
				t.Fatalf("a layer whose stream runs past a %d byte bound was accepted", layerTestBound)
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("the budget ran out and the raw short read was answered instead of the size limit: %v", err)
			}
			if !strings.Contains(err.Error(), "size limit") {
				t.Fatalf("the oversized layer was refused for another reason: %v", err)
			}
		})
	}
}

// TestDockerLayerAcceptsStreamOfExactlySizeLimit is the control for the
// bound: the same layers, checked with a bound of exactly their length, are
// accepted, so the bound cannot be met by refusing every layer or by counting
// one byte too many.
func TestDockerLayerAcceptsStreamOfExactlySizeLimit(t *testing.T) {
	oversized := int(16 * layerTestBound)
	for name, layer := range map[string][]byte{
		"a large entry body":                     boundLayerTar(t, make([]byte, oversized), 0),
		"trailing data after the tar end marker": boundLayerTar(t, []byte("file"), oversized),
	} {
		t.Run(name, func(t *testing.T) {
			content := layerContent(t, layer)
			if err := content.layerWithin("layer.tar", "application/vnd.oci.image.layer.v1.tar", digestBytes(layer), int64(len(layer))); err != nil {
				t.Fatalf("a layer of exactly the %d byte bound was refused: %v", len(layer), err)
			}
		})
	}
}
