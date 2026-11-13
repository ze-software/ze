// Design: docs/contributing/preview-artifacts.md -- bounded image archive scanning.
package buildpreview

import (
	"archive/tar"
	"bytes"
	"encoding/json"
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
