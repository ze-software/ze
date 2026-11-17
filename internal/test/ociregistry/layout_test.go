// Design: docs/architecture/container/image-store.md -- the test registry serving an image from an OCI layout folder
// Related: layout.go -- the loader under test

package ociregistry_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// The name the tests serve the layout under, and the tag it records.
const (
	layoutRepository = "library/redis"
	layoutTag        = "latest"
	manifestsPath    = "/manifests/"
)

// syntheticLayout is a small OCI layout folder written for one test: an index
// of two platforms and of one attestation entry whose manifest the folder
// does not hold, as the seed leaves the published image.
type syntheticLayout struct {
	dir    string
	index  []byte
	tagged ociregistry.Descriptor
	// manifests are the two platform entries of the index, amd64 then arm64.
	manifests []ociregistry.Descriptor
	// manifestBody, layer and layerBody are those of amd64.
	manifestBody []byte
	layer        ociregistry.Descriptor
	layerBody    []byte
	// absent is the entry whose manifest is not in the folder.
	absent ociregistry.Descriptor
}

// describe answers the descriptor of content.
func describe(mediaType string, content []byte) ociregistry.Descriptor {
	sum := sha256.Sum256(content)
	return ociregistry.Descriptor{MediaType: mediaType, Digest: "sha256:" + hex.EncodeToString(sum[:]), Size: int64(len(content))}
}

// blobFile answers the file of digest's blob in the layout at dir.
func blobFile(t *testing.T, dir, digest string) string {
	t.Helper()
	name, err := ociregistry.LayoutBlobPath(digest)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, name)
}

// writeBlob writes content as a blob of the layout at dir and answers its
// descriptor.
func writeBlob(t *testing.T, dir, mediaType string, content []byte) ociregistry.Descriptor {
	t.Helper()
	descriptor := describe(mediaType, content)
	file := blobFile(t, dir, descriptor.Digest)
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return descriptor
}

// encode answers value as JSON.
func encode(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// writeLayout writes the synthetic layout into a new temporary folder.
func writeLayout(t *testing.T) syntheticLayout {
	t.Helper()
	layout := syntheticLayout{dir: t.TempDir()}
	for _, architecture := range []string{"amd64", "arm64"} {
		layerBody := []byte("the layer of " + architecture)
		layer := writeBlob(t, layout.dir, ociregistry.MediaOCILayerTar, layerBody)
		config := writeBlob(t, layout.dir, ociregistry.MediaOCIConfig, []byte(`{"os":"linux","architecture":"`+architecture+`"}`))
		manifestBody := encode(t, map[string]any{
			"schemaVersion": 2,
			"mediaType":     ociregistry.MediaOCIManifest,
			"config":        config,
			"layers":        []ociregistry.Descriptor{layer},
		})
		manifest := writeBlob(t, layout.dir, ociregistry.MediaOCIManifest, manifestBody)
		manifest.Platform = &ociregistry.Platform{Architecture: architecture, OS: "linux"}
		if len(layout.manifests) == 0 {
			layout.manifestBody, layout.layer, layout.layerBody = manifestBody, layer, layerBody
		}
		layout.manifests = append(layout.manifests, manifest)
	}
	layout.absent = describe(ociregistry.MediaOCIManifest, []byte(`{"attestation":true}`))
	layout.absent.Platform = &ociregistry.Platform{Architecture: "unknown", OS: "unknown"}
	layout.index = encode(t, map[string]any{
		"schemaVersion": 2,
		"mediaType":     ociregistry.MediaOCIIndex,
		"manifests":     append(append([]ociregistry.Descriptor{}, layout.manifests...), layout.absent),
	})
	layout.tagged = writeBlob(t, layout.dir, ociregistry.MediaOCIIndex, layout.index)
	layout.tagged.Annotations = map[string]string{ociregistry.AnnotationRefName: layoutTag}
	listing := encode(t, map[string]any{"schemaVersion": 2, "manifests": []ociregistry.Descriptor{layout.tagged}})
	if err := os.WriteFile(filepath.Join(layout.dir, ociregistry.LayoutIndexFile), listing, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.dir, ociregistry.LayoutMarkerFile), []byte(ociregistry.LayoutMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	return layout
}

// startBearer starts a registry with the hub's anonymous bearer challenge,
// stopped when the test ends.
func startBearer(t *testing.T) *ociregistry.Registry {
	t.Helper()
	registry, err := ociregistry.Start(ociregistry.Options{Auth: ociregistry.AuthBearer})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(registry.Close)
	return registry
}

// answer is what one GET answered.
type answer struct {
	status int
	header http.Header
	body   []byte
}

// get sends one GET with authorization, which is empty for none.
func get(t *testing.T, url, authorization string) answer {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	if closeErr := response.Body.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	return answer{status: response.StatusCode, header: response.Header, body: body}
}

// bearer answers the authorization header of the registry's token.
func bearer(registry *ociregistry.Registry) string {
	return "Bearer " + registry.Token()
}

// TestPutLayoutServesTagAndDigest proves a loaded layout is served as a
// registry serves an image: the index under its tag and its digest, a held
// platform manifest and its layer by digest, and an index entry the folder
// does not hold as an unknown manifest.
//
// Method: load the synthetic layout, then read each object over HTTP and
// compare it byte for byte with what the folder holds.
func TestPutLayoutServesTagAndDigest(t *testing.T) {
	layout := writeLayout(t)
	registry := startBearer(t)
	descriptor, platforms, err := registry.PutLayout(layoutRepository, layoutTag, layout.dir)
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.Digest != layout.tagged.Digest {
		t.Fatalf("PutLayout answered the digest %s, want the index %s", descriptor.Digest, layout.tagged.Digest)
	}
	if len(platforms) != 2 {
		t.Fatalf("PutLayout answered the platforms %v, want the two the folder holds", platforms)
	}
	if platforms[0].Architecture != "amd64" {
		t.Fatalf("first platform is %v, want amd64", platforms[0])
	}
	if platforms[1].Architecture != "arm64" {
		t.Fatalf("second platform is %v, want arm64", platforms[1])
	}

	base := registry.URL() + "/v2/" + layoutRepository
	for _, name := range []string{layoutTag, layout.tagged.Digest} {
		got := get(t, base+manifestsPath+name, bearer(registry))
		if got.status != http.StatusOK {
			t.Fatalf("index by %s: status %d", name, got.status)
		}
		if !bytes.Equal(got.body, layout.index) {
			t.Fatalf("index by %s: the body is not the index of the folder", name)
		}
		if declared := got.header.Get("Docker-Content-Digest"); declared != layout.tagged.Digest {
			t.Fatalf("index by %s: declared digest %s, want %s", name, declared, layout.tagged.Digest)
		}
		if mediaType := got.header.Get("Content-Type"); mediaType != ociregistry.MediaOCIIndex {
			t.Fatalf("index by %s: content type %s", name, mediaType)
		}
	}

	manifest := get(t, base+manifestsPath+layout.manifests[0].Digest, bearer(registry))
	if manifest.status != http.StatusOK {
		t.Fatalf("platform manifest: status %d", manifest.status)
	}
	if !bytes.Equal(manifest.body, layout.manifestBody) {
		t.Fatal("platform manifest: the body is not the manifest of the folder")
	}

	layer := get(t, base+"/blobs/"+layout.layer.Digest, bearer(registry))
	if layer.status != http.StatusOK {
		t.Fatalf("layer: status %d", layer.status)
	}
	if !bytes.Equal(layer.body, layout.layerBody) {
		t.Fatal("layer: the body is not the layer of the folder")
	}
	if mediaType := layer.header.Get("Content-Type"); mediaType != ociregistry.MediaOCILayerTar {
		t.Fatalf("layer: content type %s", mediaType)
	}

	absent := get(t, base+manifestsPath+layout.absent.Digest, bearer(registry))
	if absent.status != http.StatusNotFound {
		t.Fatalf("entry the folder does not hold: status %d, want 404", absent.status)
	}
}

// TestPutLayoutAnswersBearerChallenge proves a loaded layout sits behind the
// hub's behavior: no token is a bearer challenge naming a realm, and the realm
// grants a token to anyone.
//
// Method: ask for the tag with no authorization, read the challenge, ask the
// registry's token path with no credentials, and ask again with that token.
func TestPutLayoutAnswersBearerChallenge(t *testing.T) {
	layout := writeLayout(t)
	registry := startBearer(t)
	if _, _, err := registry.PutLayout(layoutRepository, layoutTag, layout.dir); err != nil {
		t.Fatal(err)
	}
	target := registry.URL() + "/v2/" + layoutRepository + manifestsPath + layoutTag

	refused := get(t, target, "")
	if refused.status != http.StatusUnauthorized {
		t.Fatalf("no token: status %d, want 401", refused.status)
	}
	challenge := refused.header.Get("WWW-Authenticate")
	if !strings.HasPrefix(challenge, `Bearer realm="`+registry.URL()+`/token"`) {
		t.Fatalf("no token: challenge %q names no bearer realm of the registry", challenge)
	}
	if !strings.Contains(challenge, `scope="repository:`+layoutRepository+`:pull"`) {
		t.Fatalf("no token: challenge %q names no pull scope of the repository", challenge)
	}

	granted := get(t, registry.URL()+"/token", "")
	if granted.status != http.StatusOK {
		t.Fatalf("token: status %d", granted.status)
	}
	var token struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(granted.body, &token); err != nil {
		t.Fatal(err)
	}
	if token.Token == "" {
		t.Fatal("token: the realm granted no token to an anonymous caller")
	}

	served := get(t, target, "Bearer "+token.Token)
	if served.status != http.StatusOK {
		t.Fatalf("with the granted token: status %d", served.status)
	}
}

// TestPutLayoutStreamsBlobFromDisk proves a layer is read from its file when
// it is asked for, and is not a copy taken when the layout was loaded.
//
// Method: load the layout, replace the layer file with other bytes of the
// same length, and fetch the layer. A loader that read the layer into memory
// answers the old bytes.
func TestPutLayoutStreamsBlobFromDisk(t *testing.T) {
	layout := writeLayout(t)
	registry := startBearer(t)
	if _, _, err := registry.PutLayout(layoutRepository, layoutTag, layout.dir); err != nil {
		t.Fatal(err)
	}
	replaced := bytes.Repeat([]byte{'x'}, len(layout.layerBody))
	if err := os.WriteFile(blobFile(t, layout.dir, layout.layer.Digest), replaced, 0o600); err != nil {
		t.Fatal(err)
	}
	layer := get(t, registry.URL()+"/v2/"+layoutRepository+"/blobs/"+layout.layer.Digest, bearer(registry))
	if layer.status != http.StatusOK {
		t.Fatalf("layer: status %d", layer.status)
	}
	if !bytes.Equal(layer.body, replaced) {
		t.Fatalf("layer: got %q, want the bytes now in the file, %q", layer.body, replaced)
	}
}

// TestPutLayoutRefusesBrokenLayout proves each way a folder fails to be the
// layout of the tag is an error that names what is wrong, never an image
// served in part.
//
// Method: break the synthetic layout one way per case, load it, and read the
// error.
func TestPutLayoutRefusesBrokenLayout(t *testing.T) {
	cases := []struct {
		name string
		tag  string
		harm func(t *testing.T, layout syntheticLayout)
		want string
	}{
		{
			name: "no index file",
			tag:  layoutTag,
			harm: func(t *testing.T, layout syntheticLayout) {
				t.Helper()
				if err := os.Remove(filepath.Join(layout.dir, ociregistry.LayoutIndexFile)); err != nil {
					t.Fatal(err)
				}
			},
			want: ociregistry.LayoutIndexFile,
		},
		{
			name: "tag not recorded",
			tag:  "other",
			harm: func(*testing.T, syntheticLayout) {},
			want: `records nothing under the tag "other"`,
		},
		{
			name: "layer shorter than declared",
			tag:  layoutTag,
			harm: func(t *testing.T, layout syntheticLayout) {
				t.Helper()
				if err := os.Truncate(blobFile(t, layout.dir, layout.layer.Digest), 1); err != nil {
					t.Fatal(err)
				}
			},
			want: "holds 1 bytes, and its descriptor declares",
		},
		{
			name: "layer of a held manifest missing",
			tag:  layoutTag,
			harm: func(t *testing.T, layout syntheticLayout) {
				t.Helper()
				if err := os.Remove(blobFile(t, layout.dir, layout.layer.Digest)); err != nil {
					t.Fatal(err)
				}
			},
			want: "of a manifest the folder holds",
		},
		{
			name: "manifest with other bytes",
			tag:  layoutTag,
			harm: func(t *testing.T, layout syntheticLayout) {
				t.Helper()
				other := bytes.Repeat([]byte{' '}, len(layout.manifestBody))
				if err := os.WriteFile(blobFile(t, layout.dir, layout.manifests[0].Digest), other, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "hashes to",
		},
		{
			name: "no image of the index held",
			tag:  layoutTag,
			harm: func(t *testing.T, layout syntheticLayout) {
				t.Helper()
				for _, manifest := range layout.manifests {
					if err := os.Remove(blobFile(t, layout.dir, manifest.Digest)); err != nil {
						t.Fatal(err)
					}
				}
			},
			want: "holds no image of the index",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			layout := writeLayout(t)
			c.harm(t, layout)
			registry := startBearer(t)
			_, _, err := registry.PutLayout(layoutRepository, c.tag, layout.dir)
			if err == nil {
				t.Fatal("PutLayout accepted the broken layout")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("PutLayout answered %q, want it to hold %q", err, c.want)
			}
			refused := get(t, registry.URL()+"/v2/"+layoutRepository+manifestsPath+c.tag, bearer(registry))
			if refused.status != http.StatusNotFound {
				t.Fatalf("after the refusal the tag answers status %d, want 404", refused.status)
			}
		})
	}
}

// TestLayoutBlobPathRefusesForeignDigest proves a digest read from a file
// cannot name a path outside the blob folder.
//
// Method: ask for the path of digests that are not sha256 with 64 lowercase
// hexadecimal digits, and of one that is.
func TestLayoutBlobPathRefusesForeignDigest(t *testing.T) {
	valid := "sha256:" + strings.Repeat("a", 64)
	name, err := ociregistry.LayoutBlobPath(valid)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("blobs", "sha256", strings.Repeat("a", 64)); name != want {
		t.Fatalf("LayoutBlobPath(%s) = %s, want %s", valid, name, want)
	}
	for _, digest := range []string{
		"",
		"sha512:" + strings.Repeat("a", 64),
		"sha256:" + strings.Repeat("a", 63),
		"sha256:" + strings.Repeat("A", 64),
		"sha256:../../" + strings.Repeat("a", 58),
	} {
		if got, err := ociregistry.LayoutBlobPath(digest); err == nil {
			t.Fatalf("LayoutBlobPath(%q) answered %s, want a refusal", digest, got)
		}
	}
}
