// Design: docs/contributing/preview-artifacts.md -- portable image identity.
// Formats: Moby docker-v29.9.0 daemon/containerd/{image_exporter,image_inspect}.go,
// daemon/internal/image/tarexport/save.go; containerd v2.4.1 archive/exporter.go.
// The phase state records their URLs and the differing image-ID producers.
package buildpreview

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
)

const (
	archiveJSONLimit          = 16 << 20
	archiveEntryLimit         = 100000
	archiveGraphLimit         = 4096
	layerSizeLimit      int64 = 16 << 30
	ociManifest               = "application/vnd.oci.image.manifest.v1+json"
	dockerManifest            = "application/vnd.docker.distribution.manifest.v2+json"
	ociIndex                  = "application/vnd.oci.image.index.v1+json"
	dockerIndex               = "application/vnd.docker.distribution.manifest.list.v2+json"
	ociConfig                 = "application/vnd.oci.image.config.v1+json"
	dockerConfig              = "application/vnd.docker.container.image.v1+json"
	attestationManifest       = "application/vnd.docker.attestation.manifest.v1+json"
)

// ociUnknownPlatform is the OS and architecture an attestation manifest and
// its config declare, so no runtime selects it. rootFSTypeLayers is the
// rootfs type both an OCI image config and a Docker image config carry, and
// the only one the OCI image config defines.
const (
	ociUnknownPlatform = "unknown"
	rootFSTypeLayers   = "layers"
)

type archiveEntry struct {
	offset, size int64
	sum          string
	link         string
}

// archiveContent retains offsets and hashes, never whole filesystem layers.
// JSON is read through bounded SectionReaders. Nothing is extracted to disk.
type archiveContent struct {
	file    *os.File
	entries map[string]archiveEntry
	used    map[string]bool
}

type imageDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	Platform  *struct {
		OS   string `json:"os"`
		Arch string `json:"architecture"`
	} `json:"platform,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type savedManifest struct {
	Schema       int               `json:"schemaVersion"`
	MediaType    string            `json:"mediaType"`
	ArtifactType string            `json:"artifactType"`
	Config       imageDescriptor   `json:"config"`
	Layers       []imageDescriptor `json:"layers"`
	Manifests    []imageDescriptor `json:"manifests"`
	Subject      *imageDescriptor  `json:"subject"`
}

type savedConfig struct {
	OS     string `json:"os"`
	Arch   string `json:"architecture"`
	RootFS struct {
		Type    string   `json:"type"`
		DiffIDs []string `json:"diff_ids"`
	} `json:"rootfs"`
}

type compatibilityImage struct {
	Config string
	Layers []string
}

type runtimeImage struct {
	config  string
	layers  []string
	diffIDs []string
}

type imageGraph struct {
	content      *archiveContent
	image        Image
	seen         map[string]string
	children     map[string][]string
	runtime      map[string]runtimeImage
	attestations []string
}

func checkDockerArchive(filename string, image Image) error {
	if !validImageDigest(image.ID) {
		return errors.New("invalid recorded image digest")
	}
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // read-only file
	content, err := scanImageArchive(file)
	if err != nil {
		return err
	}
	var compatible []compatibilityImage
	if err := content.json("manifest.json", &compatible); err != nil {
		return err
	}
	if len(compatible) != 1 {
		return errors.New("preview Docker archive must contain exactly one runnable image")
	}
	if _, present := content.entries["index.json"]; !present {
		if err := content.checkLegacy(image, compatible[0]); err != nil {
			return err
		}
		return content.checkUsed()
	}
	var layout struct {
		Version string `json:"imageLayoutVersion"`
	}
	if err := content.json("oci-layout", &layout); err != nil {
		return err
	}
	if layout.Version != "1.0.0" {
		return errors.New("unsupported OCI archive layout")
	}
	var index savedManifest
	if err := content.json("index.json", &index); err != nil {
		return err
	}
	if index.Schema != 2 || len(index.Manifests) == 0 {
		return errors.New("invalid OCI archive index")
	}
	graph := imageGraph{content: content, image: image, seen: map[string]string{}, children: map[string][]string{}, runtime: map[string]runtimeImage{}}
	matchedTarget := false
	for _, descriptor := range index.Manifests {
		if descriptor.Digest == image.ID {
			matchedTarget = true
		}
		if err := graph.walk(descriptor, 0); err != nil {
			return err
		}
	}
	if len(graph.runtime) != 1 {
		return errors.New("preview archive must bind exactly one runtime platform")
	}
	var runtime runtimeImage
	for digest, found := range graph.runtime {
		runtime = found
		for _, target := range graph.attestations {
			if target != digest {
				return errors.New("attestation does not bind the included runtime image")
			}
		}
	}
	if matchedTarget && !graph.ownsRuntime(image.ID, map[string]bool{}) {
		return errors.New("recorded image target does not own the archive runtime")
	}
	// Graphdriver image IDs name the config even in its newer OCI-shaped
	// export. Containerd image IDs name the top-level manifest/index instead.
	if !matchedTarget {
		config, err := content.entry(runtime.config)
		if err != nil {
			return err
		}
		if len(index.Manifests) != 1 || !isImageManifest(index.Manifests[0].MediaType) || "sha256:"+config.sum != image.ID {
			return errors.New("the Docker archive does not contain its recorded immutable target")
		}
		if err := content.checkV1Configs(runtime.diffIDs, image); err != nil {
			return err
		}
	}
	if compatible[0].Config != runtime.config || !slices.Equal(compatible[0].Layers, runtime.layers) {
		return errors.New("the Docker compatibility manifest differs from OCI config or ordered layers")
	}
	return content.checkUsed()
}

func scanImageArchive(file *os.File) (*archiveContent, error) {
	content := &archiveContent{file: file, entries: map[string]archiveEntry{}, used: map[string]bool{}}
	reader := tar.NewReader(file)
	count := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read Docker archive: %w", err)
		}
		count++
		if count > archiveEntryLimit {
			return nil, errors.New("the Docker archive has too many entries")
		}
		name, err := archivePath(header.Name)
		if err != nil {
			return nil, err
		}
		if header.Typeflag == tar.TypeDir {
			if header.Size != 0 {
				return nil, errors.New("a Docker archive directory has content")
			}
			continue
		}
		if _, duplicate := content.entries[name]; duplicate {
			return nil, fmt.Errorf("repeated Docker archive entry %s", name)
		}
		offset, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		entry := archiveEntry{offset: offset, size: header.Size}
		// tar.Reader.Next already turns a legacy TypeRegA header into TypeReg,
		// or into TypeDir for a name ending in a slash, so TypeReg covers both.
		switch header.Typeflag {
		case tar.TypeReg:
			// Sparse holes expand during hashing and cannot be read back through
			// the physical offsets retained for dense outer archive entries.
			for record := range header.PAXRecords {
				if strings.HasPrefix(record, "GNU.sparse.") {
					return nil, fmt.Errorf("unsupported sparse Docker archive entry %s", name)
				}
			}
			if header.Size > layerSizeLimit {
				return nil, fmt.Errorf("the Docker archive entry %s exceeds the size limit", name)
			}
			sum, err := readerSum(reader)
			if err != nil {
				return nil, err
			}
			entry.sum = sum
		case tar.TypeSymlink, tar.TypeLink:
			if header.Size != 0 || strings.HasPrefix(header.Linkname, "/") {
				return nil, errors.New("invalid Docker archive compatibility link")
			}
			link := header.Linkname
			if header.Typeflag == tar.TypeSymlink {
				link = path.Join(path.Dir(name), link) //nolint:gosec // G305: archivePath below cleans the joined link and refuses one that leaves the archive root; nothing is extracted to disk
			}
			entry.link, err = archivePath(link)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported Docker archive entry type for %s", name)
		}
		content.entries[name] = entry
		if strings.HasPrefix(name, "blobs/") {
			parts := strings.Split(name, "/")
			if len(parts) != 3 || parts[1] != "sha256" || !validImageDigest("sha256:"+parts[2]) || entry.link != "" || entry.sum != parts[2] {
				return nil, fmt.Errorf("the digest of Docker blob %s differs", name)
			}
		}
	}
	// Docker saves an uncompressed tar, with optional zero padding. A second
	// archive or private payload after its end marker is not bound content.
	var tail [32768]byte
	for {
		n, err := file.Read(tail[:])
		for _, value := range tail[:n] {
			if value != 0 {
				return nil, errors.New("the Docker archive has trailing nonzero content")
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return content, nil
}

func archivePath(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, "\\\x00") || strings.HasPrefix(name, "/") {
		return "", errors.New("unsafe Docker archive path")
	}
	clean := path.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("escaping Docker archive path")
	}
	return clean, nil
}

func validImageDigest(digest string) bool {
	body, ok := strings.CutPrefix(digest, "sha256:")
	if !ok || len(body) != 64 {
		return false
	}
	_, err := hex.DecodeString(body)
	return err == nil && strings.ToLower(body) == body
}

func (content *archiveContent) entry(name string) (archiveEntry, error) {
	for range 32 {
		entry, ok := content.entries[name]
		if !ok {
			return archiveEntry{}, fmt.Errorf("the Docker archive is missing %s", name)
		}
		content.used[name] = true
		if entry.link == "" {
			return entry, nil
		}
		name = entry.link
	}
	return archiveEntry{}, errors.New("the Docker archive link chain is too deep")
}

func (content *archiveContent) json(name string, target any) error {
	entry, err := content.entry(name)
	if err != nil {
		return err
	}
	if entry.size < 0 || entry.size > archiveJSONLimit {
		return fmt.Errorf("the Docker JSON %s exceeds the metadata limit", name)
	}
	decoder := json.NewDecoder(io.NewSectionReader(content.file, entry.offset, entry.size))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode Docker JSON %s: %w", name, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("the Docker JSON %s has trailing content", name)
	}
	return nil
}

func (content *archiveContent) blob(descriptor imageDescriptor) (string, error) {
	if !validImageDigest(descriptor.Digest) || descriptor.Size < 0 {
		return "", errors.New("invalid OCI blob descriptor")
	}
	name := "blobs/sha256/" + strings.TrimPrefix(descriptor.Digest, "sha256:")
	entry, err := content.entry(name)
	if err != nil {
		return "", err
	}
	if entry.size != descriptor.Size || "sha256:"+entry.sum != descriptor.Digest {
		return "", fmt.Errorf("OCI descriptor size or digest differs for %s", name)
	}
	return name, nil
}

func isImageManifest(mediaType string) bool {
	return mediaType == ociManifest || mediaType == dockerManifest
}

// Only manifest/index children make a target runnable. Subject descriptors
// and attestation annotations associate metadata; they cannot supply a runtime
// to a recorded root which contains none itself.
func (graph *imageGraph) ownsRuntime(digest string, seen map[string]bool) bool {
	if seen[digest] {
		return false
	}
	seen[digest] = true
	if _, found := graph.runtime[digest]; found {
		return true
	}
	for _, child := range graph.children[digest] {
		if graph.ownsRuntime(child, seen) {
			return true
		}
	}
	return false
}

func (graph *imageGraph) walk(descriptor imageDescriptor, depth int) error {
	if depth > 16 || len(graph.seen) >= archiveGraphLimit {
		return errors.New("OCI descriptor graph exceeds preview bounds")
	}
	if descriptor.Platform != nil {
		attestation := descriptor.Annotations["vnd.docker.reference.type"] == "attestation-manifest"
		if attestation {
			if descriptor.Platform.OS != ociUnknownPlatform || descriptor.Platform.Arch != ociUnknownPlatform {
				return errors.New("attestation descriptor claims a runtime platform")
			}
		} else if descriptor.Platform.OS != graph.image.OS || descriptor.Platform.Arch != graph.image.Arch {
			return errors.New("OCI descriptor platform differs from image record")
		}
	}
	name, err := graph.content.blob(descriptor)
	if err != nil {
		return err
	}
	if mediaType, found := graph.seen[descriptor.Digest]; found {
		if mediaType != descriptor.MediaType {
			return errors.New("OCI blob has conflicting descriptor media types")
		}
		return nil
	}
	graph.seen[descriptor.Digest] = descriptor.MediaType
	var manifest savedManifest
	if err := graph.content.json(name, &manifest); err != nil {
		return err
	}
	if manifest.Schema != 2 {
		return errors.New("unsupported OCI schema version")
	}
	if manifest.MediaType != "" && manifest.MediaType != descriptor.MediaType {
		return errors.New("OCI manifest media type differs from descriptor")
	}
	if descriptor.MediaType == ociIndex || descriptor.MediaType == dockerIndex {
		if len(manifest.Manifests) == 0 {
			return errors.New("empty OCI image index")
		}
		for _, child := range manifest.Manifests {
			if err := graph.walk(child, depth+1); err != nil {
				return err
			}
			graph.children[descriptor.Digest] = append(graph.children[descriptor.Digest], child.Digest)
		}
		return nil
	}
	if !isImageManifest(descriptor.MediaType) {
		return fmt.Errorf("unsupported OCI image media type %s", descriptor.MediaType)
	}
	configName, err := graph.content.blob(manifest.Config)
	if err != nil {
		return err
	}
	attestation := descriptor.Annotations["vnd.docker.reference.type"] == "attestation-manifest" || manifest.ArtifactType == attestationManifest
	if attestation {
		return graph.attestation(descriptor, manifest, configName)
	}
	if manifest.ArtifactType != "" || (manifest.Config.MediaType != ociConfig && manifest.Config.MediaType != dockerConfig) {
		return errors.New("unsupported runtime image config")
	}
	var config savedConfig
	if err := graph.content.json(configName, &config); err != nil {
		return err
	}
	if config.OS != graph.image.OS || config.Arch != graph.image.Arch || (descriptor.Platform != nil && (descriptor.Platform.OS != config.OS || descriptor.Platform.Arch != config.Arch)) {
		return errors.New("archive runtime platform differs from image record")
	}
	if config.RootFS.Type != rootFSTypeLayers || len(config.RootFS.DiffIDs) != len(manifest.Layers) || len(manifest.Layers) == 0 {
		return errors.New("runtime config and layer inventory differ")
	}
	runtime := runtimeImage{config: configName, diffIDs: config.RootFS.DiffIDs}
	for index, layer := range manifest.Layers {
		name, err := graph.content.blob(layer)
		if err != nil {
			return err
		}
		if err := graph.content.layer(name, layer.MediaType, config.RootFS.DiffIDs[index]); err != nil {
			return err
		}
		runtime.layers = append(runtime.layers, name)
	}
	graph.runtime[descriptor.Digest] = runtime
	return nil
}

// Docker29's graphdriver exporter also writes V1 compatibility configs as
// blobs. They do not appear in its OCI descriptors. Bind each such config to
// the actual filesystem ChainID and its preceding V1 config using Moby's
// v1.CreateID algorithm, rather than allowing arbitrary unreferenced JSON.
func (content *archiveContent) checkV1Configs(diffIDs []string, image Image) error {
	type candidate struct {
		fields     map[string]json.RawMessage
		id, parent string
	}
	var configs []candidate
	allowed := map[string]bool{"id": true, "parent": true, "comment": true, "created": true, "container": true, "container_config": true, "docker_version": true, "author": true, "config": true, "architecture": true, "variant": true, "os": true, "Size": true}
	for name := range content.entries {
		if content.used[name] {
			continue
		}
		if !strings.HasPrefix(name, "blobs/sha256/") {
			return fmt.Errorf("unreferenced graphdriver export content: %s", name)
		}
		var fields map[string]json.RawMessage
		if err := content.json(name, &fields); err != nil {
			return err
		}
		for key := range fields {
			if !allowed[key] {
				return errors.New("unexpected V1 compatibility config field")
			}
		}
		var id, parent, osName string
		if err := json.Unmarshal(fields["id"], &id); err != nil {
			return err
		}
		if raw, ok := fields["parent"]; ok {
			if err := json.Unmarshal(raw, &parent); err != nil {
				return err
			}
		}
		if err := json.Unmarshal(fields["os"], &osName); err != nil {
			return err
		}
		if !validImageDigest("sha256:"+id) || (parent != "" && !validImageDigest("sha256:"+parent)) || osName != image.OS {
			return errors.New("invalid V1 compatibility identity or OS")
		}
		delete(fields, "id")
		configs = append(configs, candidate{fields: fields, id: id, parent: parent})
	}
	if len(configs) == 0 {
		return nil
	}
	if len(configs) != len(diffIDs) {
		return errors.New("V1 compatibility config count differs from filesystem layers")
	}
	parent, chain := "", ""
	used := map[int]bool{}
	for layerIndex, diffID := range diffIDs {
		if chain == "" {
			chain = diffID
		} else {
			sum := sha256.Sum256([]byte(chain + " " + diffID))
			chain = "sha256:" + hex.EncodeToString(sum[:])
		}
		matched := -1
		for index, config := range configs {
			if used[index] || config.parent != parent {
				continue
			}
			fields := make(map[string]json.RawMessage, len(config.fields)+1)
			maps.Copy(fields, config.fields)
			if layerIndex == len(diffIDs)-1 {
				// The top layer copies img.V1Image before hashing, including
				// its actual native architecture and OS.
				var arch string
				if err := json.Unmarshal(fields["architecture"], &arch); err != nil || arch != image.Arch {
					continue
				}
			} else {
				// Moby constructs an intermediate V1Image with Created only,
				// computes its ID, and assigns OS afterward for the saved blob.
				delete(fields, "os")
			}
			fields["layer_id"], _ = json.Marshal(chain)
			if parent != "" {
				fields["parent"], _ = json.Marshal("sha256:" + parent)
			}
			body, err := json.Marshal(fields)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(body)
			if hex.EncodeToString(sum[:]) == config.id {
				if matched >= 0 {
					return errors.New("ambiguous V1 compatibility parent chain")
				}
				matched = index
			}
		}
		if matched < 0 {
			return errors.New("V1 compatibility config does not bind filesystem ChainID")
		}
		used[matched] = true
		parent = configs[matched].id
	}
	return nil
}

func (graph *imageGraph) attestation(descriptor imageDescriptor, manifest savedManifest, configName string) error {
	if manifest.ArtifactType != "" && manifest.ArtifactType != attestationManifest {
		return errors.New("unsupported attestation artifact type")
	}
	target := descriptor.Annotations["vnd.docker.reference.digest"]
	if manifest.Subject != nil {
		if target != "" && target != manifest.Subject.Digest {
			return errors.New("attestation subject differs from its reference")
		}
		target = manifest.Subject.Digest
		if _, err := graph.content.blob(*manifest.Subject); err != nil {
			return err
		}
	}
	if !validImageDigest(target) || (descriptor.Platform != nil && (descriptor.Platform.OS != ociUnknownPlatform || descriptor.Platform.Arch != ociUnknownPlatform)) {
		return errors.New("invalid attestation target or platform")
	}
	var config savedConfig
	if err := graph.content.json(configName, &config); err != nil {
		return err
	}
	if manifest.Config.MediaType == "application/vnd.oci.empty.v1+json" {
		var empty map[string]json.RawMessage
		if err := graph.content.json(configName, &empty); err != nil {
			return err
		}
		if len(empty) != 0 {
			return errors.New("OCI empty config contains unexpected fields")
		}
	} else {
		if manifest.Config.MediaType != ociConfig && manifest.Config.MediaType != dockerConfig {
			return errors.New("unsupported attestation config media type")
		}
		if config.OS != ociUnknownPlatform || config.Arch != ociUnknownPlatform || config.RootFS.Type != rootFSTypeLayers || len(config.RootFS.DiffIDs) != len(manifest.Layers) {
			return errors.New("attestation config has an unexpected platform or layer inventory")
		}
	}
	for index, layer := range manifest.Layers {
		if layer.MediaType != "application/vnd.in-toto+json" {
			return errors.New("unsupported attestation payload")
		}
		name, err := graph.content.blob(layer)
		if err != nil {
			return err
		}
		var statement any
		if err := graph.content.json(name, &statement); err != nil {
			return err
		}
		if manifest.Config.MediaType != "application/vnd.oci.empty.v1+json" && config.RootFS.DiffIDs[index] != layer.Digest {
			return errors.New("attestation payload differs from config diffID")
		}
	}
	graph.attestations = append(graph.attestations, target)
	return nil
}

func (content *archiveContent) layer(name, mediaType, diffID string) error {
	if !validImageDigest(diffID) {
		return errors.New("invalid filesystem layer diffID")
	}
	entry, err := content.entry(name)
	if err != nil {
		return err
	}
	var reader io.Reader = io.NewSectionReader(content.file, entry.offset, entry.size)
	switch mediaType {
	case "application/vnd.oci.image.layer.v1.tar", "application/vnd.docker.image.rootfs.diff.tar":
	case "application/vnd.oci.image.layer.v1.tar+gzip", "application/vnd.docker.image.rootfs.diff.tar.gzip":
		compressed, err := gzip.NewReader(reader)
		if err != nil {
			return err
		}
		defer compressed.Close() //nolint:errcheck // read side; the io.Copy to the end of the stream reports any stream error
		reader = compressed
	default:
		return fmt.Errorf("unsupported preview filesystem layer compression: %s", mediaType)
	}
	limited := &io.LimitedReader{R: reader, N: layerSizeLimit + 1}
	sum := sha256.New()
	uncompressed := io.TeeReader(limited, sum)
	tarball := tar.NewReader(uncompressed)
	for {
		header, err := tarball.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid filesystem layer %s: %w", name, err)
		}
		if _, err := archivePath(header.Name); err != nil {
			return err
		}
	}
	if _, err := io.Copy(io.Discard, uncompressed); err != nil {
		return err
	}
	if limited.N <= 0 {
		return errors.New("uncompressed filesystem layer exceeds preview bound")
	}
	if "sha256:"+hex.EncodeToString(sum.Sum(nil)) != diffID {
		return fmt.Errorf("filesystem layer %s differs from its ordered config diffID", name)
	}
	return nil
}

func (content *archiveContent) checkLegacy(image Image, compatible compatibilityImage) error {
	configEntry, err := content.entry(compatible.Config)
	if err != nil {
		return err
	}
	if "sha256:"+configEntry.sum != image.ID {
		return errors.New("legacy Docker config differs from image ID")
	}
	var config savedConfig
	if err := content.json(compatible.Config, &config); err != nil {
		return err
	}
	if config.OS != image.OS || config.Arch != image.Arch || config.RootFS.Type != rootFSTypeLayers || len(config.RootFS.DiffIDs) != len(compatible.Layers) || len(compatible.Layers) == 0 {
		return errors.New("legacy Docker platform or layer inventory differs")
	}
	for index, name := range compatible.Layers {
		if err := content.layer(name, "application/vnd.oci.image.layer.v1.tar", config.RootFS.DiffIDs[index]); err != nil {
			return err
		}
		// Moby's legacy exporter also emits compatibility metadata beside
		// each layer. It is not filesystem content and is never extracted.
		for _, metadata := range []string{"json", "VERSION"} {
			name := path.Join(path.Dir(name), metadata)
			if _, ok := content.entries[name]; !ok {
				continue
			}
			if metadata == "json" {
				var value any
				if err := content.json(name, &value); err != nil {
					return err
				}
			} else {
				entry, err := content.entry(name)
				if err != nil {
					return err
				}
				if entry.size > 32 {
					return errors.New("invalid legacy version metadata")
				}
			}
		}
	}
	if _, ok := content.entries["repositories"]; ok {
		var value any
		if err := content.json("repositories", &value); err != nil {
			return err
		}
	}
	return nil
}

func (content *archiveContent) checkUsed() error {
	for name := range content.entries {
		if !content.used[name] {
			return fmt.Errorf("unreferenced content in Docker archive: %s", name)
		}
	}
	return nil
}
