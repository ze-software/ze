// Design: docs/architecture/container/image-store.md -- the registry requests: the manifest, the index and the platform
// Related: client.go -- the requests a manifest is fetched through
// Related: blob.go -- the configuration and layer blobs a manifest names

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Bounds of a manifest (the design's "Registry requests" and boundary table).
const (
	manifestSizeMax = 4 << 20
	configSizeMax   = 4 << 20
	layersMax       = 128
	// offeredShownMax bounds the platforms a refusal lists.
	offeredShownMax = 16
	// mediaTypeShownMax bounds a refused media type an error repeats.
	mediaTypeShownMax = 128
)

// The media types the client accepts or refuses by name. These are the
// published names of the OCI image specification and of the Docker schema 2
// specification.
const (
	mediaOCIIndex       = "application/vnd.oci.image.index.v1+json"
	mediaOCIManifest    = "application/vnd.oci.image.manifest.v1+json"
	mediaDockerList     = "application/vnd.docker.distribution.manifest.list.v2+json"
	mediaDockerManifest = "application/vnd.docker.distribution.manifest.v2+json"
	mediaSchema1        = "application/vnd.docker.distribution.manifest.v1+json"
	mediaSchema1Signed  = "application/vnd.docker.distribution.manifest.v1+prettyjws"
	mediaOCIConfig      = "application/vnd.oci.image.config.v1+json"
	mediaDockerConfig   = "application/vnd.docker.container.image.v1+json"
	mediaForeignDocker  = "application/vnd.docker.image.rootfs.foreign.diff.tar.gzip"
	mediaForeignOCI     = "application/vnd.oci.image.layer.nondistributable.v1.tar"
	mediaEncrypted      = "+encrypted"
)

// acceptManifests is the Accept header of a manifest request: the four
// accepted types.
var acceptManifests = strings.Join([]string{mediaOCIIndex, mediaOCIManifest, mediaDockerList, mediaDockerManifest}, ", ")

// layerCompressions is the closed set of accepted layer types.
var layerCompressions = map[string]Compression{
	"application/vnd.oci.image.layer.v1.tar":            CompressionNone,
	"application/vnd.oci.image.layer.v1.tar+gzip":       CompressionGzip,
	"application/vnd.oci.image.layer.v1.tar+zstd":       CompressionZstd,
	"application/vnd.docker.image.rootfs.diff.tar.gzip": CompressionGzip,
}

// manifestKind is what a manifest is.
type manifestKind uint8

const (
	manifestKindUnspecified manifestKind = iota
	manifestKindIndex
	manifestKindImage
)

// wireDescriptor, wirePlatform and wireManifest are the JSON of the OCI and
// Docker formats. Their keys are those formats' own, not Ze's. One struct
// reads an index and a manifest: their fields do not overlap.
type wireDescriptor struct {
	MediaType string        `json:"mediaType"`
	Digest    string        `json:"digest"`
	Size      int64         `json:"size"`
	URLs      []string      `json:"urls"`
	Platform  *wirePlatform `json:"platform"`
}

type wirePlatform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant"`
}

type wireManifest struct {
	SchemaVersion int              `json:"schemaVersion"`
	MediaType     string           `json:"mediaType"`
	ArtifactType  string           `json:"artifactType"`
	Config        wireDescriptor   `json:"config"`
	Layers        []wireDescriptor `json:"layers"`
	Manifests     []wireDescriptor `json:"manifests"`
}

// expected is what a manifest request knows about the answer before it
// arrives: the digest when it asked by digest, the size when a descriptor
// named it (negative for unknown).
type expected struct {
	digest Digest
	size   int64
}

// manifestAnswer is a manifest read under its bound and checked against
// what was expected.
type manifestAnswer struct {
	body      []byte
	mediaType string
	digest    Digest
}

// Image is a reference resolved to the manifest of the running platform:
// the manifest and the configuration as fetched, checked, and the layers the
// manifest names, in order.
type Image struct {
	digest   Digest
	manifest []byte
	config   []byte
	layers   []Layer
}

// Digest answers the digest of the platform's manifest, which names the
// image's folder.
func (i *Image) Digest() Digest { return i.digest }

// Manifest answers the manifest, byte for byte as fetched.
func (i *Image) Manifest() []byte { return i.manifest }

// Config answers the image configuration, byte for byte as fetched.
func (i *Image) Config() []byte { return i.config }

// Layers answers the layers, in the order they are applied.
func (i *Image) Layers() []Layer { return i.layers }

// Resolve fetches the manifest ref names, resolves an index to the entry of
// the client's architecture, and fetches and checks the image
// configuration. Every object is checked against its digest and its size,
// and an index entry against the digest and size the index declares.
func (c *Client) Resolve(ctx context.Context, ref Reference) (*Image, error) {
	if err := c.owns(ref); err != nil {
		return nil, err
	}
	first := expected{size: -1}
	if digest, pinned := ref.Digest(); pinned {
		first.digest = digest
	}
	answer, err := c.fetchManifest(ctx, ref, ref.pin(), first)
	if err != nil {
		return nil, err
	}
	manifest, kind, err := parseManifest(c.registry.host, answer)
	if err != nil {
		return nil, err
	}
	if kind == manifestKindIndex {
		entry, err := selectPlatform(manifest, c.architecture)
		if err != nil {
			return nil, err
		}
		answer, err = c.fetchManifest(ctx, ref, entry.digest.String(), expected(entry))
		if err != nil {
			return nil, err
		}
		manifest, kind, err = parseManifest(c.registry.host, answer)
		if err != nil {
			return nil, err
		}
		if kind != manifestKindImage {
			return nil, refused("the index entry "+entry.digest.String()+" is itself an index", "Ze resolves one index to one image manifest, and never a nested index")
		}
	}
	configBlob, layers, err := imageParts(manifest)
	if err != nil {
		return nil, err
	}
	config, err := c.fetchConfig(ctx, ref, configBlob)
	if err != nil {
		return nil, err
	}
	if err := checkConfigPlatform(config, c.architecture); err != nil {
		return nil, err
	}
	return &Image{digest: answer.digest, manifest: answer.body, config: config, layers: layers}, nil
}

// fetchManifest asks the mirror, then the registry, for the manifest pin
// names in ref's repository.
func (c *Client) fetchManifest(ctx context.Context, ref Reference, pin string, want expected) (manifestAnswer, error) {
	path := "/v2/" + ref.repository + "/manifests/" + pin
	var answer manifestAnswer
	err := c.fromMirrorThenRegistry(ctx, func(o *origin) error {
		got, err := c.manifestOnce(ctx, o, path, ref, want)
		answer = got
		return err
	})
	return answer, err
}

// manifestOnce makes one manifest request and checks the answer: at most
// manifestSizeMax bytes, hashing to the digest asked for and to the digest
// the answer declares, with the size its descriptor gave.
func (c *Client) manifestOnce(ctx context.Context, o *origin, path string, ref Reference, want expected) (manifestAnswer, error) {
	got, err := c.get(ctx, o, path, acceptManifests, pullScope(ref))
	if err != nil {
		return manifestAnswer{}, err
	}
	defer got.close()
	what := "the manifest " + path + " from " + o.host
	body, over, err := readBounded(got.body, manifestSizeMax)
	if err != nil {
		if ctx.Err() != nil {
			return manifestAnswer{}, stopped(ctx)
		}
		return manifestAnswer{}, &Error{failure: FailureUnreachable, problem: "the transfer of " + what + " stopped", why: "the registry stopped sending before the end of the manifest", cause: err, retry: true}
	}
	if over {
		return manifestAnswer{}, refused(what+" is too large", "a manifest or an index is at most 4 MiB")
	}
	digest := digestOf(body)
	if !want.digest.IsZero() && digest != want.digest {
		return manifestAnswer{}, failed(FailureDigestMismatch, what+" does not match its digest", "it hashes to "+digest.String()+", not to "+want.digest.String())
	}
	if want.size >= 0 && int64(len(body)) != want.size {
		return manifestAnswer{}, &Error{failure: FailureDigestMismatch, problem: what + " does not match its descriptor", why: "its size is not the size the index declares", cause: fmt.Errorf("received %d bytes, declared %d", len(body), want.size)}
	}
	if declaredText := got.response.Header.Get("Docker-Content-Digest"); declaredText != "" {
		declared, err := ParseDigest(declaredText)
		if err != nil {
			return manifestAnswer{}, refused(what+" declares a digest Ze does not accept", err.Error())
		}
		if declared != digest {
			return manifestAnswer{}, failed(FailureDigestMismatch, what+" does not match the digest it declares", "it hashes to "+digest.String()+", and the registry declared "+declared.String())
		}
	}
	mediaType, _, _ := strings.Cut(got.response.Header.Get("Content-Type"), ";")
	return manifestAnswer{body: body, mediaType: strings.TrimSpace(mediaType), digest: digest}, nil
}

// parseManifest reads a manifest answer and judges its type: the type the
// body names, or the answer's content type when the body names none.
func parseManifest(host string, answer manifestAnswer) (wireManifest, manifestKind, error) {
	var manifest wireManifest
	if err := json.Unmarshal(answer.body, &manifest); err != nil {
		return wireManifest{}, manifestKindUnspecified, refused("the manifest "+answer.digest.String()+" from "+host+" is not JSON", err.Error())
	}
	if manifest.SchemaVersion == 1 {
		return wireManifest{}, manifestKindUnspecified, refused("the manifest "+answer.digest.String()+" is of schema 1", "Ze accepts schema 2 manifests and OCI manifests only")
	}
	mediaType := manifest.MediaType
	if mediaType == "" {
		mediaType = answer.mediaType
	}
	switch mediaType {
	case mediaOCIIndex, mediaDockerList:
		return manifest, manifestKindIndex, nil
	case mediaOCIManifest, mediaDockerManifest:
		return manifest, manifestKindImage, nil
	case mediaSchema1, mediaSchema1Signed:
		return wireManifest{}, manifestKindUnspecified, refused("the manifest "+answer.digest.String()+" is of schema 1", "Ze accepts schema 2 manifests and OCI manifests only")
	}
	return wireManifest{}, manifestKindUnspecified, refused("the manifest "+answer.digest.String()+" is of the type "+shownMediaType(mediaType), "Ze accepts an OCI index, an OCI manifest, a Docker manifest list and a Docker manifest")
}

// selectPlatform answers the index entry whose platform is Linux on arch.
// An arm64 entry of variant v8 equals one with no variant; an amd64 entry
// naming a feature level above the baseline is not selected.
func selectPlatform(index wireManifest, arch Architecture) (Blob, error) {
	offered := make([]string, 0, min(len(index.Manifests), offeredShownMax))
	for _, entry := range index.Manifests {
		if entry.Platform == nil {
			continue
		}
		if len(offered) < offeredShownMax {
			offered = append(offered, platformName(entry.Platform))
		}
		if !platformMatches(entry.Platform.OS, entry.Platform.Architecture, entry.Platform.Variant, arch) {
			continue
		}
		switch entry.MediaType {
		case mediaOCIIndex, mediaDockerList:
			return Blob{}, refused("the index entry for linux/"+arch.String()+" is itself an index", "Ze resolves one index to one image manifest, and never a nested index")
		}
		return descriptorBlob("the index entry for linux/"+arch.String(), entry, manifestSizeMax)
	}
	if len(index.Manifests) > len(offered) {
		offered = append(offered, "and "+strconv.Itoa(len(index.Manifests)-len(offered))+" more")
	}
	return Blob{}, refused("the index offers no image for linux/"+arch.String(), "it offers "+strings.Join(offered, ", "))
}

// platformMatches answers whether a platform is Linux on arch, at the
// baseline variant.
func platformMatches(system, architecture, variant string, arch Architecture) bool {
	if system != "linux" {
		return false
	}
	if architecture != arch.String() {
		return false
	}
	switch arch {
	case ArchitectureARM64:
		return variant == "" || variant == "v8"
	case ArchitectureAMD64:
		return variant == "" || variant == "v1"
	case ArchitectureUnspecified:
		return false
	}
	return false
}

// platformName answers os/architecture[/variant] for a refusal.
func platformName(platform *wirePlatform) string {
	name := shownMediaType(platform.OS) + "/" + shownMediaType(platform.Architecture)
	if platform.Variant != "" {
		name += "/" + shownMediaType(platform.Variant)
	}
	return name
}

// imageParts checks the configuration and the layers an image manifest
// names: a container image configuration, from 1 to 128 layers, each of an
// accepted type, distributed by the registry, unencrypted.
func imageParts(manifest wireManifest) (Blob, []Layer, error) {
	if manifest.ArtifactType != "" {
		return Blob{}, nil, refused("the manifest is not a container image", "it names the artifact type "+shownMediaType(manifest.ArtifactType))
	}
	if manifest.Config.MediaType != mediaOCIConfig && manifest.Config.MediaType != mediaDockerConfig {
		return Blob{}, nil, refused("the manifest is not a container image", "its configuration is of the type "+shownMediaType(manifest.Config.MediaType))
	}
	config, err := descriptorBlob("the image configuration", manifest.Config, configSizeMax)
	if err != nil {
		return Blob{}, nil, err
	}
	if len(manifest.Layers) == 0 {
		return Blob{}, nil, refused("the manifest names no layer", "an image has from 1 to 128 layers")
	}
	if len(manifest.Layers) > layersMax {
		return Blob{}, nil, refused("the manifest names "+strconv.Itoa(len(manifest.Layers))+" layers", "an image has at most 128 layers")
	}
	layers := make([]Layer, 0, len(manifest.Layers))
	for i, descriptor := range manifest.Layers {
		layer, err := checkLayer(i, descriptor)
		if err != nil {
			return Blob{}, nil, err
		}
		layers = append(layers, layer)
	}
	return config, layers, nil
}

// checkLayer judges one layer descriptor.
func checkLayer(index int, descriptor wireDescriptor) (Layer, error) {
	what := "layer " + strconv.Itoa(index+1)
	if strings.HasSuffix(descriptor.MediaType, mediaEncrypted) {
		return Layer{}, refused(what+" is encrypted", "Ze cannot apply an encrypted layer: its type is "+shownMediaType(descriptor.MediaType))
	}
	if len(descriptor.URLs) > 0 {
		return Layer{}, refused(what+" is distributed from a foreign address", "Ze fetches every layer from the registry it named, where its digest is checked")
	}
	foreign := refused(what+" is a foreign layer", "its type "+shownMediaType(descriptor.MediaType)+" is not distributed by the registry")
	if strings.HasPrefix(descriptor.MediaType, mediaForeignOCI) {
		return Layer{}, foreign
	}
	if descriptor.MediaType == mediaForeignDocker {
		return Layer{}, foreign
	}
	compression, accepted := layerCompressions[descriptor.MediaType]
	if !accepted {
		return Layer{}, refused(what+" is of the type "+shownMediaType(descriptor.MediaType), "Ze applies uncompressed, gzip and zstd tar layers only")
	}
	blob, err := descriptorBlob(what, descriptor, -1)
	if err != nil {
		return Layer{}, err
	}
	return Layer{blob: blob, compression: compression}, nil
}

// descriptorBlob checks a descriptor's digest and size, the size bounded by
// sizeMax when sizeMax is not negative.
func descriptorBlob(what string, descriptor wireDescriptor, sizeMax int64) (Blob, error) {
	digest, err := ParseDigest(descriptor.Digest)
	if err != nil {
		return Blob{}, refused(what+" is named by a digest Ze does not accept", err.Error())
	}
	if descriptor.Size < 0 {
		return Blob{}, refused(what+" declares a negative size", "a size is a count of bytes")
	}
	if sizeMax >= 0 && descriptor.Size > sizeMax {
		return Blob{}, &Error{failure: FailureRefused, problem: what + " is too large", why: "it is bounded at 4 MiB", cause: fmt.Errorf("declared %d bytes", descriptor.Size)}
	}
	return Blob{digest: digest, size: descriptor.Size}, nil
}

// checkConfigPlatform refuses an image configuration whose platform is not
// Linux on arch.
func checkConfigPlatform(config []byte, arch Architecture) error {
	var platform wirePlatform
	if err := json.Unmarshal(config, &platform); err != nil {
		return refused("the image configuration is not JSON", err.Error())
	}
	if !platformMatches(platform.OS, platform.Architecture, platform.Variant, arch) {
		return refused("the image is for "+platformName(&platform), "this system runs linux/"+arch.String())
	}
	return nil
}

// refused builds a refusal of an answer Ze does not accept.
func refused(problem, why string) *Error {
	return failed(FailureRefused, problem, why)
}

// shownMediaType bounds a type or a name from outside and keeps its
// printable characters only, before an error repeats it.
func shownMediaType(text string) string {
	var kept strings.Builder
	for _, c := range text {
		if kept.Len() == mediaTypeShownMax {
			break
		}
		if c > ' ' && c < 0x7f {
			kept.WriteRune(c)
		}
	}
	if kept.Len() == 0 {
		return "(none)"
	}
	return kept.String()
}
