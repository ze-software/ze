// Design: docs/contributing/gh-pages.md -- the receipt binds reviewed bytes, not a source-reproducibility claim.
// Related: publication_actions.go -- registered recording and preflight actions.
// Related: publication_deployment.go -- exact-commit Pages and live-byte confirmation.
package site

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	publicationFormat         = 1
	publicationFilesMax       = 32768
	publicationDepthMax       = 64
	publicationFileBytesMax   = 64 << 20
	publicationBytesMax       = 512 << 20
	publicationRecordBytesMax = 8 << 20
	publicationGitBytesMax    = 2 << 20
	publicationSourceScope    = "website tracked and nonignored untracked files; native code and generated-data inputs are not hashed by this digest"
)

// These structs are the serialized boundary. A receipt is not trusted until
// publicationReceiptRead validates its format, paths, identities and hashes.
type publicationSource struct {
	Revision    string   `json:"revision"`
	Digest      string   `json:"digest"`
	DigestScope string   `json:"digest-scope"`
	DirtyPaths  []string `json:"dirty-paths"`
}

type publicationFile struct {
	Path   string `json:"path"`
	Digest string `json:"sha256"`
	Size   int64  `json:"size"`
}

type publicationProvenance struct {
	Format         int               `json:"format"`
	Source         publicationSource `json:"source"`
	ArtifactDigest string            `json:"artifact-digest"`
	Files          []publicationFile `json:"files"`
	Unavailable    string            `json:"unavailable,omitempty"`
}

type publicationReceipt struct {
	Format         int               `json:"format"`
	Source         publicationSource `json:"source"`
	Artifact       string            `json:"artifact"`
	ArtifactDigest string            `json:"artifact-digest"`
	Files          []publicationFile `json:"files"`
	Changed        []string          `json:"changed"`
	Deleted        []string          `json:"deleted"`
	RecordedAt     string            `json:"recorded-at"`
}

// publicationBuildStart captures only the inputs named in DigestScope. Dirty
// paths disclose other working-tree inputs; they are not content hashes.
func publicationBuildStart(ctx context.Context, paths Paths, digest string) (publicationSource, error) {
	revision, err := publicationGit(ctx, paths.Repository, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return publicationSource{}, err
	}
	if err := publicationRevision(strings.TrimSpace(string(revision))); err != nil {
		return publicationSource{}, err
	}
	dirty, err := publicationGit(ctx, paths.Repository, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return publicationSource{}, err
	}
	return publicationSource{Revision: strings.TrimSpace(string(revision)), Digest: digest,
		DigestScope: publicationSourceScope, DirtyPaths: publicationNames(dirty)}, nil
}

// publicationBuildFinish attaches the final-byte record to the existing route
// record. A changed observed source invalidates attribution, not rendering.
func publicationBuildFinish(ctx context.Context, paths Paths, source publicationSource) error {
	files, err := trackedAndUntrackedSourceFiles(paths)
	if err != nil {
		return err
	}
	digest, err := sourceDigest(paths.Source, files)
	if err != nil {
		return err
	}
	after, err := publicationBuildStart(ctx, paths, digest)
	if err != nil {
		return err
	}
	provenance := publicationProvenance{Format: publicationFormat, Source: source}
	if after.Revision != source.Revision {
		provenance.Unavailable = "source HEAD changed during the build"
	}
	if after.Digest != source.Digest {
		provenance.Unavailable = "website source bytes changed during the build"
	}
	if !slices.Equal(after.DirtyPaths, source.DirtyPaths) {
		provenance.Unavailable = "observed dirty-path qualification changed during the build"
	}
	provenance.Files, provenance.ArtifactDigest, err = publicationSnapshot(paths.Output)
	if err != nil {
		return err
	}
	record, err := publicationProducerRecord(paths)
	if err != nil {
		return err
	}
	record.Provenance = &provenance
	return publicationWrite(paths, producerRecordPath(paths), record)
}

// publicationPartial invalidates whole-build attribution before any partial
// producer can replace bytes. Route claims remain available to site check.
func publicationPartial(paths Paths) error {
	record, err := publicationProducerRecord(paths)
	if err != nil {
		return err
	}
	record.Provenance = nil
	return publicationWrite(paths, producerRecordPath(paths), record)
}

func publicationProducerRecord(paths Paths) (producerRecord, error) {
	var record producerRecord
	if err := publicationRead(paths, producerRecordPath(paths), &record); err != nil {
		return record, err
	}
	return record, nil
}

func publicationReceiptPath(paths Paths) string {
	name := strings.TrimPrefix(filepath.Base(producerRecordPath(paths)), "producers-")
	return filepath.Join(paths.Repository, producerRecordDirectory, "publication-"+name)
}

func publicationSnapshot(output string) ([]publicationFile, string, error) {
	root, err := os.OpenRoot(output)
	if err != nil {
		return nil, "", err
	}
	defer root.Close() //nolint:errcheck // every file read is completed before this directory handle closes
	files := make([]publicationFile, 0)
	var total int64
	entries := 0
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		entries++
		if entries > publicationFilesMax*2 {
			return fmt.Errorf("artifact exceeds %d directory entries", publicationFilesMax*2)
		}
		if strings.Count(name, "/") >= publicationDepthMax {
			return fmt.Errorf("artifact exceeds %d path components", publicationDepthMax)
		}
		if publicationIgnored(name) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if err := publicationPath(name); err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("artifact path %s is not a regular file", name)
		}
		if len(files) >= publicationFilesMax {
			return fmt.Errorf("artifact exceeds %d files", publicationFilesMax)
		}
		file, openErr := root.Open(name)
		if openErr != nil {
			return openErr
		}
		hash := sha256.New()
		size, readErr := io.Copy(hash, io.LimitReader(file, publicationFileBytesMax+1))
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if size > publicationFileBytesMax {
			return fmt.Errorf("artifact file %s exceeds %d bytes", name, publicationFileBytesMax)
		}
		total += size
		if total > publicationBytesMax {
			return fmt.Errorf("artifact exceeds %d bytes", publicationBytesMax)
		}
		files = append(files, publicationFile{Path: name, Digest: hex.EncodeToString(hash.Sum(nil)), Size: size})
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	if len(files) == 0 {
		return nil, "", errors.New("the artifact contains no publishable files")
	}
	slices.SortFunc(files, func(first, second publicationFile) int {
		return strings.Compare(first.Path, second.Path)
	})
	return files, publicationFilesDigest(files), nil
}

// publicationIgnored uses the exact ignore declaration the builder writes.
// Tracked files matching it are separately refused at commit confirmation.
func publicationIgnored(name string) bool {
	if name == gitMetadataDir {
		return true
	}
	if strings.HasPrefix(name, gitMetadataDir+"/") {
		return true
	}
	for line := range strings.SplitSeq(artifactIgnoreContent, "\n") {
		if line == "" {
			continue
		}
		if prefix, directory := strings.CutSuffix(line, "/"); directory {
			if name == prefix {
				return true
			}
			if strings.HasPrefix(name, line) {
				return true
			}
			continue
		}
		if filepath.Base(name) == line {
			return true
		}
	}
	return false
}

func publicationFilesDigest(files []publicationFile) string {
	hash := sha256.New()
	for _, file := range files {
		hash.Write([]byte(file.Path))
		hash.Write([]byte{0})
		hash.Write([]byte(file.Digest))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func publicationPath(name string) error {
	if !fs.ValidPath(name) {
		return fmt.Errorf("artifact path %q is not relative and clean", name)
	}
	if strings.ContainsAny(name, "\\\r\n\x00") {
		return fmt.Errorf("artifact path %q contains unsupported characters", name)
	}
	if name == "." {
		return errors.New("an artifact file path cannot name its root")
	}
	return nil
}

func publicationRevision(value string) error {
	if len(value) != 40 {
		return errors.New("publication revision must be a full 40-character commit ID")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return errors.New("publication revision is not hexadecimal")
	}
	return nil
}

func publicationDigest(value string) error {
	if len(value) != sha256.Size*2 {
		return errors.New("publication digest must be a full SHA-256 value")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return errors.New("publication digest is not hexadecimal")
	}
	return nil
}

func publicationGit(ctx context.Context, root string, arguments ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", arguments...) //nolint:gosec // callers supply fixed read-only Git verbs and validated identities
	command.Dir = root
	var output bytes.Buffer
	command.Stdout = &publicationBuffer{buffer: &output, remaining: publicationGitBytesMax}
	var diagnostic bytes.Buffer
	command.Stderr = &publicationBuffer{buffer: &diagnostic, remaining: 4096}
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("read site Git identity: %w: %s", err, strings.TrimSpace(diagnostic.String()))
	}
	return output.Bytes(), nil
}

type publicationBuffer struct {
	buffer    *bytes.Buffer
	remaining int
}

func (writer *publicationBuffer) Write(data []byte) (int, error) {
	if len(data) > writer.remaining {
		return 0, errors.New("publication metadata exceeds its read limit")
	}
	writer.remaining -= len(data)
	return writer.buffer.Write(data)
}

func publicationNames(data []byte) []string {
	names := make([]string, 0)
	for raw := range bytes.SplitSeq(data, []byte{0}) {
		if len(raw) != 0 {
			names = append(names, string(raw))
		}
	}
	slices.Sort(names)
	return names
}

func publicationRead(paths Paths, filename string, value any) error {
	relative, err := filepath.Rel(paths.Repository, filename)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(paths.Repository)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // no directory mutation is pending
	file, err := root.Open(relative)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // a completed read is not changed by a close failure
	content, err := io.ReadAll(io.LimitReader(file, publicationRecordBytesMax+1))
	if err != nil {
		return err
	}
	if len(content) > publicationRecordBytesMax {
		return errors.New("publication record exceeds its read limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("publication record contains trailing data")
	}
	return nil
}

func publicationWrite(paths Paths, filename string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if len(content) > publicationRecordBytesMax {
		return errors.New("publication record exceeds its write limit")
	}
	relative, err := filepath.Rel(paths.Repository, filename)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(paths.Repository)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // each file was closed before publishing its name
	if err := root.MkdirAll(producerRecordDirectory, 0o700); err != nil {
		return err
	}
	temporary := relative + "." + rand.Text()
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary) //nolint:errcheck // a successful rename already removed the task-owned temporary name
	_, writeErr := file.Write(append(content, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return root.Rename(temporary, relative)
}
