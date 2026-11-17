// Design: docs/architecture/container/image-store.md -- platform selection and the accepted types, proven against the test registry
// Related: manifest.go -- selectPlatform, parseManifest and imageParts under test

package registry

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// putPlatform stores a one-layer image for architecture and variant, with a
// layer of its own so each platform has its own digest.
func putPlatform(t *testing.T, r *ociregistry.Registry, architecture, variant string) ociregistry.Descriptor {
	t.Helper()
	descriptor, err := r.PutImage("multi", "", ociregistry.Image{
		Architecture: architecture,
		Variant:      variant,
		Layers:       []ociregistry.Layer{{MediaType: ociregistry.MediaOCILayerTar, Content: []byte("layer of " + architecture + variant)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return descriptor
}

// putIndex stores an index of entries under tag.
func putIndex(t *testing.T, r *ociregistry.Registry, tag string, entries ...ociregistry.Descriptor) {
	t.Helper()
	if _, err := r.PutIndex("multi", tag, entries); err != nil {
		t.Fatal(err)
	}
}

// TestIndexSelectsRunningPlatform proves each architecture selects its own
// entry: amd64 and arm64 from one index, arm64 variant v8 as arm64, and an
// amd64 entry above the baseline feature level passed over for the baseline
// one listed after it.
func TestIndexSelectsRunningPlatform(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	amd := putPlatform(t, r, "amd64", "")
	amdV3 := putPlatform(t, r, "amd64", "v3")
	arm := putPlatform(t, r, "arm64", "")
	armV8 := putPlatform(t, r, "arm64", "v8")
	putIndex(t, r, "both", arm, amd)
	putIndex(t, r, "v8", amd, armV8)
	putIndex(t, r, "levels", amdV3, amd)

	for _, tc := range []struct {
		tag  string
		arch Architecture
		want ociregistry.Descriptor
	}{
		{tag: "both", arch: ArchitectureAMD64, want: amd},
		{tag: "both", arch: ArchitectureARM64, want: arm},
		{tag: "v8", arch: ArchitectureARM64, want: armV8},
		{tag: "levels", arch: ArchitectureAMD64, want: amd},
	} {
		c := newClient(t, Config{Registry: r.Host(), Architecture: tc.arch})
		image, err := c.Resolve(t.Context(), mustReference(t, r.Host()+"/multi:"+tc.tag))
		if err != nil {
			t.Errorf("%s for %v: %v", tc.tag, tc.arch, err)
			continue
		}
		if image.Digest().String() != tc.want.Digest {
			t.Errorf("%s for %v selected %s, want %s", tc.tag, tc.arch, image.Digest(), tc.want.Digest)
		}
	}
}

// TestIndexWithoutPlatformNamesOffered proves an index with no entry for the
// running platform is refused, listing the platforms it holds.
func TestIndexWithoutPlatformNamesOffered(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	putIndex(t, r, "other", putPlatform(t, r, "arm64", ""), putPlatform(t, r, "s390x", ""))
	c := newClient(t, Config{Registry: r.Host(), Architecture: ArchitectureAMD64})

	_, err := c.Resolve(t.Context(), mustReference(t, r.Host()+"/multi:other"))
	failure := requireFailure(t, err, FailureRefused)
	for _, offered := range []string{"linux/arm64", "linux/s390x"} {
		if !strings.Contains(failure.Why(), offered) {
			t.Errorf("why = %q, want %s listed", failure.Why(), offered)
		}
	}
}

// TestUnacceptedManifestTypesRefused proves each refused form is refused by
// name: schema 1, a nested index, a foreign layer, an encrypted layer and a
// manifest that is not a container image. A Docker schema 2 manifest built
// the same way is accepted, so the refusals are tied to the types.
func TestUnacceptedManifestTypesRefused(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	config := r.PutBlob("application/vnd.docker.container.image.v1+json", []byte(`{"architecture":"amd64","os":"linux"}`))
	layer := r.PutBlob("application/vnd.docker.image.rootfs.diff.tar.gzip", []byte("docker layer"))
	describe := func(mediaType string, d ociregistry.Descriptor, extra string) string {
		return `{"mediaType":"` + mediaType + `","digest":"` + d.Digest + `","size":` + itoa(d.Size) + extra + `}`
	}
	manifest := func(mediaType, configType, layerJSON string) []byte {
		return []byte(`{"schemaVersion":2,"mediaType":"` + mediaType + `","config":` + describe(configType, config, "") + `,"layers":[` + layerJSON + `]}`)
	}
	docker := "application/vnd.docker.distribution.manifest.v2+json"
	dockerConfig := "application/vnd.docker.container.image.v1+json"

	inner, err := r.PutIndex("multi", "", []ociregistry.Descriptor{putPlatform(t, r, "amd64", "")})
	if err != nil {
		t.Fatal(err)
	}
	inner.Platform = &ociregistry.Platform{Architecture: "amd64", OS: "linux"}
	putIndex(t, r, "nested", inner)
	r.PutManifest("multi", "schema1", "application/vnd.docker.distribution.manifest.v1+prettyjws", []byte(`{"schemaVersion":1,"name":"multi"}`))
	r.PutManifest("multi", "foreign", docker, manifest(docker, dockerConfig, describe("application/vnd.docker.image.rootfs.foreign.diff.tar.gzip", layer, `,"urls":["https://elsewhere.example/layer"]`)))
	r.PutManifest("multi", "encrypted", ociregistry.MediaOCIManifest, manifest(ociregistry.MediaOCIManifest, ociregistry.MediaOCIConfig, describe("application/vnd.oci.image.layer.v1.tar+gzip+encrypted", layer, "")))
	r.PutManifest("multi", "artifact", ociregistry.MediaOCIManifest, manifest(ociregistry.MediaOCIManifest, "application/vnd.example.thing.config.v1+json", describe(ociregistry.MediaOCILayerGzip, layer, "")))
	r.PutManifest("multi", "accepted", docker, manifest(docker, dockerConfig, describe("application/vnd.docker.image.rootfs.diff.tar.gzip", layer, "")))
	c := newClient(t, Config{Registry: r.Host()})

	for _, tc := range []struct{ tag, named string }{
		{tag: "schema1", named: "schema 1"},
		{tag: "nested", named: "itself an index"},
		{tag: "foreign", named: "foreign"},
		{tag: "encrypted", named: "encrypted"},
		{tag: "artifact", named: "not a container image"},
	} {
		_, err := c.Resolve(t.Context(), mustReference(t, r.Host()+"/multi:"+tc.tag))
		failure := requireFailure(t, err, FailureRefused)
		if !strings.Contains(failure.Problem(), tc.named) {
			t.Errorf("%s: problem = %q, want %q", tc.tag, failure.Problem(), tc.named)
		}
	}
	image, err := c.Resolve(t.Context(), mustReference(t, r.Host()+"/multi:accepted"))
	if err != nil {
		t.Fatalf("a Docker schema 2 manifest was refused: %v", err)
	}
	if image.Layers()[0].Compression() != CompressionGzip {
		t.Errorf("the Docker layer reads %v, want gzip", image.Layers()[0].Compression())
	}
}
