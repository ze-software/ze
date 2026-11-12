// VALIDATES: portable image checks bind payloads, identity and platform.
// PREVENTS: refreshed outer checksums blessing an unrelated Docker archive.
package buildpreview

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Shapes follow Moby docker-v29.9.0 image_exporter/ImageInspect and
// tarexport.save, containerd v2.4.1 archive.manifestsRecord/ociIndexRecord,
// and Moby v24.0.9 tarexport.save. URLs are retained in the phase state.
type dockerFixture struct {
	files            map[string][]byte
	image            Image
	config, manifest string
	layers           []string
}

// The non-omitempty zero fields of Moby29 api/types/container.Config.
type fixtureContainerConfig struct {
	Hostname, Domainname, User              string
	AttachStdin, AttachStdout, AttachStderr bool
	Tty, OpenStdin, StdinOnce               bool
	Env, Cmd                                []string
	Image                                   string
	Volumes                                 map[string]struct{}
	WorkingDir                              string
	Entrypoint                              []string
	Labels                                  map[string]string
}

type fixtureV1Config struct {
	ID           string                 `json:"id,omitempty"`
	Parent       string                 `json:"parent,omitempty"`
	Created      *string                `json:"created"`
	Container    fixtureContainerConfig `json:"container_config"`
	Architecture string                 `json:"architecture,omitempty"`
	OS           string                 `json:"os,omitempty"`
}

func digestBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func fixtureJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func fixtureLayer(t *testing.T, name, text string) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := tar.NewWriter(&out)
	if err := writer.WriteHeader(&tar.Header{Name: name, Size: int64(len(text)), Mode: 0o755, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// mustString returns a decoded fixture value as a string, and fails the test
// when the fixture holds another shape there. It stops the test on a mismatch,
// so it runs only on the test's own goroutine.
func mustString(t *testing.T, value any, what string) string {
	t.Helper()
	text, ok := value.(string)
	if !ok {
		t.Fatalf("%s is %T, want string", what, value)
	}
	return text
}

// mustMap returns a decoded fixture value as a JSON object, and fails the
// test when the fixture holds another shape there. It stops the test on a
// mismatch, so it runs only on the test's own goroutine.
func mustMap(t *testing.T, value any, what string) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, want map[string]any", what, value)
	}
	return object
}

// mustSlice returns a decoded fixture value as a JSON array of at least
// entriesMin entries, and fails the test when the fixture holds another shape
// or a shorter array there. It stops the test on a mismatch, so it runs only
// on the test's own goroutine.
func mustSlice(t *testing.T, value any, entriesMin int, what string) []any {
	t.Helper()
	array, ok := value.([]any)
	if !ok {
		t.Fatalf("%s is %T, want []any", what, value)
	}
	if len(array) < entriesMin {
		t.Fatalf("%s holds %d entries, want at least %d", what, len(array), entriesMin)
	}
	return array
}

func savedDockerFixture(t *testing.T, format string) dockerFixture {
	t.Helper()
	fixture := dockerFixture{files: map[string][]byte{}, image: Image{Role: "deployment", OS: "linux", Arch: "amd64"}}
	add := func(body []byte) map[string]any {
		digest := digestBytes(body)
		fixture.files["blobs/sha256/"+strings.TrimPrefix(digest, "sha256:")] = body
		return map[string]any{"digest": digest, "size": len(body)}
	}
	var diffs []string
	var layers []map[string]any
	for index, name := range []string{"ze", "etc/ze-release"} {
		raw := fixtureLayer(t, name, "layer "+name)
		diffs = append(diffs, digestBytes(raw))
		payload := raw
		mediaType := "application/vnd.oci.image.layer.v1.tar"
		if strings.HasPrefix(format, "containerd") {
			var compressed bytes.Buffer
			writer := gzip.NewWriter(&compressed)
			if _, err := writer.Write(raw); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			payload, mediaType = compressed.Bytes(), mediaType+"+gzip"
		}
		desc := add(payload)
		desc["mediaType"] = mediaType
		blob := "blobs/sha256/" + strings.TrimPrefix(mustString(t, desc["digest"], "layer descriptor digest"), "sha256:")
		if format == "legacy" {
			legacy := strings.Repeat(string(rune('a'+index)), 64) + "/layer.tar"
			fixture.files[legacy] = fixture.files[blob]
			delete(fixture.files, blob)
			blob = legacy
		}
		fixture.layers = append(fixture.layers, blob)
		layers = append(layers, desc)
	}
	config := fixtureJSON(t, map[string]any{"architecture": "amd64", "os": "linux", "rootfs": map[string]any{"type": "layers", "diff_ids": diffs}})
	configDesc := add(config)
	configDesc["mediaType"] = "application/vnd.oci.image.config.v1+json"
	configDigest := mustString(t, configDesc["digest"], "config descriptor digest")
	fixture.config = "blobs/sha256/" + strings.TrimPrefix(configDigest, "sha256:")
	fixture.image.ID = configDigest
	if format == "legacy" {
		legacy := strings.TrimPrefix(fixture.image.ID, "sha256:") + ".json"
		fixture.files[legacy] = config
		delete(fixture.files, fixture.config)
		fixture.config = legacy
	} else {
		manifest := fixtureJSON(t, map[string]any{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json", "config": configDesc, "layers": layers})
		desc := add(manifest)
		desc["mediaType"] = "application/vnd.oci.image.manifest.v1+json"
		fixture.manifest = "blobs/sha256/" + strings.TrimPrefix(mustString(t, desc["digest"], "manifest descriptor digest"), "sha256:")
		if strings.HasPrefix(format, "containerd-index") {
			desc["platform"] = map[string]string{"os": "linux", "architecture": "amd64"}
			manifests := []any{desc}
			if format == "containerd-index-attestation" {
				payload := fixtureJSON(t, map[string]any{"_type": "https://in-toto.io/Statement/v0.1", "predicateType": "https://slsa.dev/provenance/v0.2"})
				layer := add(payload)
				layer["mediaType"] = "application/vnd.in-toto+json"
				config := add(fixtureJSON(t, map[string]any{"architecture": "unknown", "os": "unknown", "rootfs": map[string]any{"type": "layers", "diff_ids": []any{layer["digest"]}}}))
				config["mediaType"] = "application/vnd.oci.image.config.v1+json"
				attestation := add(fixtureJSON(t, map[string]any{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json", "config": config, "layers": []any{layer}}))
				attestation["mediaType"] = "application/vnd.oci.image.manifest.v1+json"
				attestation["platform"] = map[string]string{"os": "unknown", "architecture": "unknown"}
				attestation["annotations"] = map[string]any{"vnd.docker.reference.type": "attestation-manifest", "vnd.docker.reference.digest": desc["digest"]}
				manifests = append(manifests, attestation)
			}
			index := fixtureJSON(t, map[string]any{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.index.v1+json", "manifests": manifests})
			desc = add(index)
			desc["mediaType"] = "application/vnd.oci.image.index.v1+json"
		}
		if strings.HasPrefix(format, "containerd") {
			fixture.image.ID = mustString(t, desc["digest"], "top-level descriptor digest")
		}
		fixture.files["index.json"] = fixtureJSON(t, map[string]any{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.index.v1+json", "manifests": []any{desc}})
		fixture.files["oci-layout"] = []byte(`{"imageLayoutVersion":"1.0.0"}`)
	}
	fixture.files["manifest.json"] = fixtureJSON(t, []any{map[string]any{"Config": fixture.config, "Layers": fixture.layers, "RepoTags": []string{}}})
	if strings.HasPrefix(format, "graphdriver") {
		parent, chain := "", ""
		for index, diffID := range diffs {
			if chain == "" {
				chain = diffID
			} else {
				chain = digestBytes([]byte(chain + " " + diffID))
			}
			epoch := "1970-01-01T00:00:00Z"
			// Moby saveImage hashes the intermediate V1Image before assigning
			// OS; only the top layer already carries the image's V1 fields.
			config := fixtureV1Config{Created: &epoch}
			if format == "graphdriver-saved-os" {
				config.OS = "linux"
			}
			if index == len(diffs)-1 {
				config.Created = nil
				config.Architecture = "amd64"
				config.OS = "linux"
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(fixtureJSON(t, config), &fields); err != nil {
				t.Fatal(err)
			}
			fields["layer_id"] = fixtureJSON(t, chain)
			if parent != "" {
				fields["parent"] = fixtureJSON(t, "sha256:"+parent)
			}
			config.ID = strings.TrimPrefix(digestBytes(fixtureJSON(t, fields)), "sha256:")
			if index != len(diffs)-1 && format == "graphdriver" {
				fields["os"] = fixtureJSON(t, "linux")
				if strings.TrimPrefix(digestBytes(fixtureJSON(t, fields)), "sha256:") == config.ID {
					t.Fatal("intermediate V1 identity incorrectly includes the later saved OS")
				}
			}
			config.Parent = parent
			config.OS = "linux"
			body := fixtureJSON(t, config)
			fixture.files["blobs/sha256/"+strings.TrimPrefix(digestBytes(body), "sha256:")] = body
			parent = config.ID
		}
	}
	return fixture
}

func writeDockerFixture(t *testing.T, fixture dockerFixture) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "image.docker.tar")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := tar.NewWriter(file)
	var names []string
	for name := range fixture.files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		body := fixture.files[name]
		if err := writer.WriteHeader(&tar.Header{Name: name, Size: int64(len(body)), Mode: 0o644, Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDockerArchiveMatchesEngineIdentityAndLayerFormats(t *testing.T) {
	for _, format := range []string{"legacy", "graphdriver", "containerd-manifest", "containerd-index", "containerd-index-attestation"} {
		t.Run(format, func(t *testing.T) {
			fixture := savedDockerFixture(t, format)
			if err := checkDockerArchive(writeDockerFixture(t, fixture), fixture.image); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDockerGraphdriverRejectsPostHashOSInIntermediateID(t *testing.T) {
	fixture := savedDockerFixture(t, "graphdriver-saved-os")
	if err := checkDockerArchive(writeDockerFixture(t, fixture), fixture.image); err == nil {
		t.Fatal("accepted an intermediate V1 ID hashed with the later saved OS")
	}
}

func TestDockerArchiveRejectsUnboundOrChangedContent(t *testing.T) {
	for _, attack := range []string{"wrong-id", "wrong-platform", "missing-layer", "changed-layer", "changed-config", "compatibility-mismatch", "extra-blob", "traversal", "malformed-manifest"} {
		t.Run(attack, func(t *testing.T) {
			fixture := savedDockerFixture(t, "containerd-index")
			switch attack {
			case "wrong-id":
				fixture.image.ID = "sha256:" + strings.Repeat("f", 64)
			case "wrong-platform":
				fixture.image.Arch = "arm64"
			case "missing-layer":
				delete(fixture.files, fixture.layers[0])
			case "changed-layer":
				fixture.files[fixture.layers[0]] = fixtureLayer(t, "ze", "unrelated payload")
			case "changed-config":
				fixture.files[fixture.config] = []byte(`{"architecture":"arm64","os":"linux"}`)
			case "compatibility-mismatch":
				fixture.files["manifest.json"] = fixtureJSON(t, []any{map[string]any{"Config": fixture.config, "Layers": []string{fixture.layers[1], fixture.layers[0]}}})
			case "extra-blob":
				body := []byte("private unused blob")
				fixture.files["blobs/sha256/"+strings.TrimPrefix(digestBytes(body), "sha256:")] = body
			case "traversal":
				fixture.files["../private"] = []byte("private payload")
			case "malformed-manifest":
				fixture.files["manifest.json"] = []byte("not JSON")
			}
			if err := checkDockerArchive(writeDockerFixture(t, fixture), fixture.image); err == nil {
				t.Fatalf("accepted %s", attack)
			}
		})
	}
}

func TestDockerArchiveRejectsFreshInnerHashesWithStaleDiffIDs(t *testing.T) {
	fixture := savedDockerFixture(t, "containerd-manifest")
	// Replace a complete gzip layer, update its blob filename/descriptor,
	// compatibility manifest, OCI manifest digest and recorded image ID.
	// The config's ordered uncompressed DiffIDs still bind the original layer.
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(fixtureLayer(t, "ze", "different executable")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	body := compressed.Bytes()
	digest := digestBytes(body)
	name := "blobs/sha256/" + strings.TrimPrefix(digest, "sha256:")
	delete(fixture.files, fixture.layers[0])
	fixture.files[name], fixture.layers[0] = body, name
	var manifest map[string]any
	if err := json.Unmarshal(fixture.files[fixture.manifest], &manifest); err != nil {
		t.Fatal(err)
	}
	layer := mustMap(t, mustSlice(t, manifest["layers"], 1, "manifest layers")[0], "first manifest layer")
	layer["digest"], layer["size"] = digest, len(body)
	body = fixtureJSON(t, manifest)
	digest = digestBytes(body)
	delete(fixture.files, fixture.manifest)
	fixture.manifest = "blobs/sha256/" + strings.TrimPrefix(digest, "sha256:")
	fixture.files[fixture.manifest] = body
	fixture.image.ID = digest
	fixture.files["index.json"] = fixtureJSON(t, map[string]any{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.index.v1+json", "manifests": []any{map[string]any{"mediaType": "application/vnd.oci.image.manifest.v1+json", "digest": digest, "size": len(body)}}})
	fixture.files["manifest.json"] = fixtureJSON(t, []any{map[string]any{"Config": fixture.config, "Layers": fixture.layers}})
	if err := checkDockerArchive(writeDockerFixture(t, fixture), fixture.image); err == nil || !strings.Contains(err.Error(), "diffID") {
		t.Fatalf("changed layer escaped config binding: %v", err)
	}
}

func TestDockerArchiveRecordedTargetMustOwnItsRuntime(t *testing.T) {
	fixture := savedDockerFixture(t, "containerd-index-attestation")
	// Keep all descriptors and bytes valid, but split the native manifest
	// and attestation into separate top-level roots. The recorded ID now
	// selects only the attestation, which cannot start the reported runtime.
	oldIndex := "blobs/sha256/" + strings.TrimPrefix(fixture.image.ID, "sha256:")
	var index map[string]any
	if err := json.Unmarshal(fixture.files[oldIndex], &index); err != nil {
		t.Fatal(err)
	}
	manifests := mustSlice(t, index["manifests"], 2, "index manifests")
	attestation := mustMap(t, manifests[1], "attestation manifest descriptor")
	fixture.image.ID = mustString(t, attestation["digest"], "attestation manifest digest")
	fixture.files["index.json"] = fixtureJSON(t, map[string]any{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.index.v1+json", "manifests": []any{attestation, manifests[0]}})
	delete(fixture.files, oldIndex)
	if err := checkDockerArchive(writeDockerFixture(t, fixture), fixture.image); err == nil {
		t.Fatal("metadata-only recorded target borrowed an unrelated runtime root")
	}
}

func TestDockerArchiveBoundsMetadataAndRefusesRepeatedEntries(t *testing.T) {
	fixture := savedDockerFixture(t, "containerd-index")
	fixture.files["manifest.json"] = bytes.Repeat([]byte(" "), (16<<20)+1)
	if err := checkDockerArchive(writeDockerFixture(t, fixture), fixture.image); err == nil || !strings.Contains(err.Error(), "metadata limit") {
		t.Fatalf("oversized JSON was accepted: %v", err)
	}
	fixture = savedDockerFixture(t, "legacy")
	filename := writeDockerFixture(t, fixture)
	file, err := os.OpenFile(filename, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	// Replace the final two zero blocks with a second entry of the same name.
	if _, err := file.Seek(info.Size()-1024, 0); err != nil {
		t.Fatal(err)
	}
	archive := tar.NewWriter(file)
	if err := archive.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o644, Typeflag: tar.TypeReg, Size: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte("[]")); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := checkDockerArchive(filename, fixture.image); err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Fatalf("repeated image entry accepted: %v", err)
	}
	filename = writeDockerFixture(t, fixture)
	file, err = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("unbound private trailing payload"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := checkDockerArchive(filename, fixture.image); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing private content accepted: %v", err)
	}
}

func TestCheckBindsEachDockerArtifactToOneImageRecord(t *testing.T) {
	for _, attack := range []string{"none", "duplicate-file", "unrelated-tar-with-fresh-checksums"} {
		t.Run(attack, func(t *testing.T) {
			root, deps := fixture(t)
			output := filepath.Join(t.TempDir(), "preview")
			manifest, err := prepare(t.Context(), root, output, false, deps)
			if err != nil {
				t.Fatal(err)
			}
			for _, role := range []string{"deployment", "lab"} {
				archive := savedDockerFixture(t, "containerd-index")
				path := writeDockerFixture(t, archive)
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				name := role + ".docker.tar"
				if attack == "unrelated-tar-with-fresh-checksums" && role == "lab" {
					body = []byte("not a Docker archive")
				}
				put(t, output, name, string(body))
				sum, err := fileSum(filepath.Join(output, name))
				if err != nil {
					t.Fatal(err)
				}
				manifest.Artifacts = append(manifest.Artifacts, Artifact{File: name, Kind: "docker", OS: "linux", Arch: "amd64", SHA256: sum})
				archive.image.File, archive.image.Role, archive.image.VersionOutput = name, role, manifest.Version
				manifest.Images = append(manifest.Images, archive.image)
			}
			if attack == "duplicate-file" {
				manifest.Images[1].File = manifest.Images[0].File
			}
			rewriteManifest(t, output, manifest)
			_, err = check(output, deps.inspect)
			if attack == "none" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatalf("accepted %s", attack)
			}
		})
	}
}
