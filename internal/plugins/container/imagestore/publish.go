// Design: docs/architecture/container/image-store.md -- publication by one rename, the completion mark, the listing
// Related: layout.go -- the store root and its folders
// Related: sync_linux.go -- the filesystem sync before the mark

package imagestore

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/pkg/zefs"
)

// The content of one image folder. The names are the field contract, as the
// folder names of the layout are.
const (
	// rootfsFolder is the applied root of the image.
	rootfsFolder = "rootfs"
	// manifestName and configName are the manifest and the image
	// configuration, byte for byte as fetched.
	manifestName = "manifest.json"
	configName   = "config.json"
	// markName is the completion mark, written last: a folder without it is
	// not an image.
	markName = "complete"
	// markMode is read-only: nothing changes a published image.
	markMode fs.FileMode = 0o400
	// markSizeMax bounds a mark read back. A mark Ze wrote is a few hundred
	// bytes, so a larger one was not written by Ze.
	markSizeMax = 64 << 10
)

// imageFacts is what the fetch that applied an image knows about it, and
// what its completion mark records beside the digest. The folder then
// describes itself: a lost state store costs bindings and generations, not
// images.
type imageFacts struct {
	// Reference is the whole reference the publishing fetch named. A binding
	// may name the same digest under another reference.
	Reference string `json:"reference"`
	// Platform is the operating system and architecture, as linux/arm64.
	Platform string `json:"platform"`
	// Registry is the host that served the bytes: the registry, or its mirror.
	Registry string    `json:"registry"`
	Fetched  time.Time `json:"fetched"`
	// ExpandedBytes is the count of bytes taken while the layers were applied.
	ExpandedBytes uint64 `json:"expanded-bytes"`
	// Entries is the count of archive entries applied.
	Entries uint64 `json:"entries"`
	// DevicesSkipped is the count of device nodes under the image's device
	// folder that were not created.
	DevicesSkipped uint64 `json:"devices-skipped"`
}

// completionMark is the content of the mark file: the digest that names the
// folder, and the facts of the fetch.
type completionMark struct {
	Digest string `json:"digest"`
	imageFacts
}

// errNoMark is the answer for a digest folder that holds no completion mark.
// Such a folder is not an image, and the listing passes over it.
var errNoMark = errors.New("the folder holds no completion mark")

// publish makes the image applied under staging/<staged> an image of the
// store, named by digest. It writes the completion mark last, syncs, and
// renames the folder into images/ in one rename that refuses an existing
// target, so a reader sees either no folder or a whole image, and a second
// publication of one digest never replaces the first.
//
// staged MUST hold the applied root, the manifest and the configuration. On a
// refusal the staged folder stays where it is, and the caller removes it. It
// MUST be called from the store's one worker, which owns every change to the
// store.
func (r *storeRoot) publish(staged string, digest imageDigest, facts imageFacts) error {
	if err := checkStagedName(staged); err != nil {
		return publishFailed(digest, facts.Reference, err)
	}
	stagedPath := filepath.Join(stagingFolder, staged)
	if err := r.checkStagedImage(stagedPath); err != nil {
		return publishFailed(digest, facts.Reference, err)
	}
	target := filepath.Join(imagesFolder, digest.folder())

	// The mark says the root is whole, so the root reaches the disk first.
	if err := syncFilesystem(r.dir); err != nil {
		return syncLostWrite(r.path, digest, facts.Reference, err)
	}
	if err := r.writeMark(stagedPath, completionMark{Digest: digest.String(), imageFacts: facts}); err != nil {
		return publishFailed(digest, facts.Reference, err)
	}
	if err := r.syncFolder(stagedPath); err != nil {
		return publishFailed(digest, facts.Reference, err)
	}
	if err := zefs.RenameNoReplace(r.dir, stagedPath, target); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return opererr.Wrap(err,
				"image "+digest.String()+" is already in the store",
				"a published image is never replaced: the digest proves the folder already holds this content",
				"show container image list",
			)
		}
		return publishFailed(digest, facts.Reference, err)
	}
	if err := r.syncFolder(imagesFolder); err != nil {
		return publishFailed(digest, facts.Reference, err)
	}
	return nil
}

// errStagedName refuses a staged name that is not one segment under staging.
// The store chooses the name, so the refusal is a store defect, answered
// rather than followed out of staging.
var errStagedName = errors.New("the staged folder is not one name under staging")

// checkStagedName refuses a staged name that would leave staging.
func checkStagedName(staged string) error {
	switch staged {
	case "", ".", "..":
		return errStagedName
	}
	if filepath.Base(staged) != staged {
		return errStagedName
	}
	return nil
}

// checkStagedImage refuses to mark a staged folder that does not hold the
// three things the mark vouches for.
func (r *storeRoot) checkStagedImage(stagedPath string) error {
	root, err := r.root.Lstat(filepath.Join(stagedPath, rootfsFolder))
	if err != nil {
		return err
	}
	if !root.IsDir() {
		return errors.New("container image store: " + stagedPath + " holds no applied root")
	}
	for _, name := range []string{manifestName, configName} {
		info, err := r.root.Lstat(filepath.Join(stagedPath, name))
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("container image store: " + stagedPath + " holds no " + name)
		}
	}
	return nil
}

// writeMark creates the mark in folder and syncs it. O_EXCL refuses a name
// that already holds anything, a symbolic link included, so the mark is
// never written through another file.
func (r *storeRoot) writeMark(folder string, mark completionMark) error {
	content, err := json.Marshal(mark)
	if err != nil {
		return err
	}
	file, err := r.root.OpenFile(filepath.Join(folder, markName), os.O_WRONLY|os.O_CREATE|os.O_EXCL, markMode)
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
}

// syncFolder syncs one folder of the store, so the names it holds survive a
// crash.
func (r *storeRoot) syncFolder(name string) error {
	folder, err := r.root.Open(name)
	if err != nil {
		return err
	}
	if err := folder.Sync(); err != nil {
		return errors.Join(err, folder.Close())
	}
	return folder.Close()
}

// syncLostWrite is the refusal of a publication whose filesystem sync failed.
//
// The refusal is intended. The sync covers the whole filesystem that holds
// the store, and Linux reports through it, once, any write the filesystem
// lost since the last sync, whichever file it belonged to. The error cannot
// say whether the lost write was one of this image's files, so the store
// does not vouch for the root: it writes no mark and publishes nothing. The
// error is reported once, so a fetch run again after the cause is known
// syncs cleanly unless the disk loses another write.
func syncLostWrite(storePath string, digest imageDigest, reference string, cause error) error {
	return opererr.Wrap(cause,
		"image "+digest.String()+" was not published: the filesystem that holds the container store "+storePath+
			" reported a write it could not complete",
		"the report may concern any file of that filesystem, this image's included, and the store never marks an image whole that may not be on the disk; the kernel log names the device and the failed write",
		"create container image reference "+reference,
	)
}

// publishFailed is the refusal of a publication that did not happen, with
// its cause.
func publishFailed(digest imageDigest, reference string, cause error) error {
	return opererr.Wrap(cause,
		"image "+digest.String()+" could not be published in the container store",
		"an image is published whole or not at all, so the store holds no part of it",
		"create container image reference "+reference,
	)
}

// images answers one row for each published image, in digest order. A
// folder whose name is no digest, and a digest folder with no completion
// mark, are not images: publication renames a folder only once its mark is
// written, so neither came from a whole fetch.
func (r *storeRoot) images() ([]container.ImageRow, error) {
	folder, err := r.root.Open(imagesFolder)
	if err != nil {
		return nil, storeCannotOpen(r.path, err)
	}
	names, err := folder.Readdirnames(-1)
	if closeErr := folder.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, storeCannotOpen(r.path, err)
	}
	rows := make([]container.ImageRow, 0, len(names))
	for _, name := range names {
		digest, err := parseDigestFolder(name)
		if err != nil {
			continue // Not a name publication writes.
		}
		mark, err := r.readMark(digest)
		if errors.Is(err, errNoMark) {
			continue
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, mark.row())
	}
	slices.SortFunc(rows, func(a, b container.ImageRow) int { return strings.Compare(a.Digest, b.Digest) })
	return rows, nil
}

// readMark reads the completion mark of the image folder digest names. It
// answers errNoMark when the folder holds none, and an operator error for a
// mark it cannot read or that names another digest: such a folder claims to
// be whole, so it is reported rather than passed over.
func (r *storeRoot) readMark(digest imageDigest) (completionMark, error) {
	folder := filepath.Join(imagesFolder, digest.folder())
	file, err := r.root.Open(filepath.Join(folder, markName))
	if errors.Is(err, fs.ErrNotExist) {
		return completionMark{}, errNoMark
	}
	if err != nil {
		return completionMark{}, markUnreadable(r.path, folder, err)
	}
	content, err := io.ReadAll(io.LimitReader(file, markSizeMax+1))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return completionMark{}, markUnreadable(r.path, folder, err)
	}
	if len(content) > markSizeMax {
		return completionMark{}, markUnreadable(r.path, folder, errors.New("the mark is larger than 64 KiB"))
	}
	var mark completionMark
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&mark); err != nil {
		return completionMark{}, markUnreadable(r.path, folder, err)
	}
	named, err := parseDigest(mark.Digest)
	if err != nil {
		return completionMark{}, markUnreadable(r.path, folder, err)
	}
	if named != digest {
		return completionMark{}, markUnreadable(r.path, folder, errors.New("the mark names "+named.String()))
	}
	return mark, nil
}

// row answers the listing row of a published image. UsedBy is empty, not
// unknown: a container holds an image only once resolve answered it ready,
// and the store's resolve does not answer ready yet.
func (m completionMark) row() container.ImageRow {
	return container.ImageRow{
		Reference: m.Reference,
		Digest:    m.Digest,
		Size:      m.ExpandedBytes,
		Fetched:   m.Fetched,
		State:     container.ImageStatePresent,
		UsedBy:    []string{},
	}
}

// markUnreadable is the refusal of an image folder whose mark cannot be
// trusted.
func markUnreadable(storePath, folder string, cause error) error {
	path := filepath.Join(storePath, folder)
	return opererr.Wrap(cause,
		"the image folder "+path+" holds a completion mark ze cannot read",
		"the mark is what proves the folder holds a whole image",
		"rm -rf "+path,
	)
}
