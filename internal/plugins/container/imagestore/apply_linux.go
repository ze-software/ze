// Design: docs/architecture/container/image-store.md -- layer application: one staged root, resolved by descriptor
// Related: apply_rules.go -- the rules and bounds each entry is judged by
// Related: publish.go -- what publishes the root once every layer is applied

//go:build linux

package imagestore

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	pathrs "github.com/cyphar/filepath-securejoin/pathrs-lite"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/container/registry"
)

const (
	// removedFolder, beside the root in a staged folder, holds what a layer
	// replaced or whited out until the applier closes. It is not part of the
	// image.
	removedFolder = "removed"
	// rootMode is the root's mode until an entry for the root sets its own.
	rootMode fs.FileMode = 0o755
	// impliedFolderMode is the mode of a folder created on the way to an
	// entry whose layer holds no entry for it.
	impliedFolderMode fs.FileMode = 0o755
	// createMode is the mode a file, a pipe or a folder is created with,
	// before its entry's owner and mode are set.
	createMode = 0o600
	// createFolderMode is createMode for a folder, which the store must enter.
	createFolderMode = 0o700
	// modeBits are the permission bits with the set-user, set-group and
	// sticky bits: all of an entry's mode a file takes.
	modeBits = 0o7777
)

// layerApplier applies the layers of one image, in order, into the root of
// one staged folder. Every entry is resolved under a descriptor for that
// root, through the path-safety library's in-root calls, and every change
// to a name is a descriptor-relative system call on its folder, so no name
// of the image ever reaches a path-based call.
//
// It is not safe for concurrent use: the store's one worker owns it from
// newLayerApplier to close, and MUST call close once, before it publishes
// the staged folder.
type layerApplier struct {
	store *storeRoot
	// removedPath is the removed folder, under the store root.
	removedPath string
	// root is the applied root, the base every entry is resolved from.
	root *os.File
	// removed is the removed folder.
	removed *os.File
	limits  applyLimits
	counts  applyCounts
	// moved names what moveAway puts in the removed folder.
	moved uint64
}

// newLayerApplier creates the staged folder staged, with an empty root and
// its removed folder, and answers the applier for it. The staged name MUST
// be one the store chose, new in staging.
func (r *storeRoot) newLayerApplier(staged string, limits applyLimits) (*layerApplier, error) {
	if err := checkStagedName(staged); err != nil {
		return nil, err
	}
	stagedPath := filepath.Join(stagingFolder, staged)
	rootPath := filepath.Join(stagedPath, rootfsFolder)
	removedPath := filepath.Join(stagedPath, removedFolder)
	if err := r.root.Mkdir(stagedPath, storeMode); err != nil {
		return nil, storeCannotOpen(r.path, err)
	}
	if err := r.root.Mkdir(rootPath, rootMode); err != nil {
		return nil, storeCannotOpen(r.path, err)
	}
	if err := r.root.Mkdir(removedPath, storeMode); err != nil {
		return nil, storeCannotOpen(r.path, err)
	}
	root, err := r.root.Open(rootPath)
	if err != nil {
		return nil, storeCannotOpen(r.path, err)
	}
	removed, err := r.root.Open(removedPath)
	if err != nil {
		return nil, errors.Join(storeCannotOpen(r.path, err), root.Close())
	}
	return &layerApplier{store: r, removedPath: removedPath, root: root, removed: removed, limits: limits}, nil
}

// close removes what the layers replaced or whited out, and closes the
// handles.
func (a *layerApplier) close() error {
	removeErr := a.store.root.RemoveAll(a.removedPath)
	return errors.Join(removeErr, a.removed.Close(), a.root.Close())
}

// countedFacts copies the counts the completion mark records into facts.
func (a *layerApplier) countedFacts(facts *imageFacts) {
	facts.ExpandedBytes = a.counts.expandedBytes
	facts.Entries = a.counts.entries
	facts.DevicesSkipped = a.counts.devicesSkipped
}

// apply applies one layer blob, compressed as compression, onto the root.
// The decompressed stream MUST hash to diffID, the digest the image
// configuration declares for it, read to its end. The blob MUST already
// have been checked against its own digest and size.
//
// On an error the root holds part of the layer: the caller removes the
// staged folder and publishes nothing.
func (a *layerApplier) apply(blob io.Reader, compression registry.Compression, diffID imageDigest) error {
	a.counts.layers++
	if a.counts.layers > layersMax {
		return refuseLayer(ruleLayers, textbuf.StringUint(a.counts.layers)+" layers")
	}
	stream, err := openLayerStream(blob, compression)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	expanded := &expandedStream{stream: io.TeeReader(stream, hasher), counts: &a.counts, max: a.limits.expandedBytesMax}
	err = a.applyArchive(tar.NewReader(expanded))
	if err == nil {
		// The end of the archive, and anything after it, are part of the
		// stream the digest covers, and of the bound.
		if _, drainErr := io.Copy(io.Discard, expanded); drainErr != nil {
			err = refuseLayer(ruleCompression, drainErr.Error())
		}
	}
	closeErr := stream.Close()
	if expanded.refusal != nil {
		return expanded.refusal
	}
	if err != nil {
		return err
	}
	if closeErr != nil {
		return refuseLayer(ruleCompression, closeErr.Error())
	}
	received := hex.EncodeToString(hasher.Sum(nil))
	if received != diffID.hex {
		return refuseLayer(ruleDiffDigest, "the stream hashes to "+digestTextPrefix+received+", the configuration declares "+diffID.String())
	}
	return nil
}

// layerState is what one layer applied so far: the names, for its opaque
// folders, and its folders, whose times are set after their children.
type layerState struct {
	// applied holds the key of each entry and of each folder on the way to
	// one. Every key is counted against the entry bound, so it stays bounded.
	applied map[string]struct{}
	folders []folderTimes
}

// folderTimes is a folder of the layer and the times its entry gave it.
type folderTimes struct {
	path     entryPath
	metadata entryMetadata
}

// add records path, and the folders on the way to it, as names the layer
// applied. It answers how many of those folders the layer had not named.
func (l *layerState) add(path entryPath) uint64 {
	if path.isRoot() {
		return 0
	}
	key := path.key()
	l.applied[key] = struct{}{}
	var named uint64
	for {
		parent, _, found := strings.CutLast(key, "/")
		if !found {
			return named
		}
		if _, seen := l.applied[parent]; seen {
			return named
		}
		l.applied[parent] = struct{}{}
		named++
		key = parent
	}
}

// holds reports whether the layer applied the name in folder, or anything
// under it.
func (l *layerState) holds(folder entryPath, name string) bool {
	key := name
	if !folder.isRoot() {
		key = folder.key() + "/" + name
	}
	_, held := l.applied[key]
	return held
}

// applyArchive applies each entry of one layer's archive, then the times of
// its folders.
func (a *layerApplier) applyArchive(archive *tar.Reader) error {
	layer := layerState{applied: make(map[string]struct{})}
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return refuseLayer(ruleArchive, err.Error())
		}
		a.counts.entries++
		if err := a.checkEntryBound(header.Name); err != nil {
			return err
		}
		if err := a.applyEntry(archive, header, &layer); err != nil {
			return err
		}
	}
	return a.setFolderTimes(layer.folders)
}

// checkEntryBound refuses the entry that takes the image past its entry
// bound.
func (a *layerApplier) checkEntryBound(entry string) error {
	if a.counts.entries+a.counts.impliedFolders > a.limits.entriesMax {
		return refuse(entry, ruleEntries, "more than "+textbuf.StringUint(a.limits.entriesMax))
	}
	return nil
}

// applyEntry applies one archive entry by the rule of its type.
func (a *layerApplier) applyEntry(archive io.Reader, header *tar.Header, layer *layerState) error {
	if header.Typeflag == tar.TypeXGlobalHeader {
		return nil // Metadata of the archive: it names no file.
	}
	path, err := parseEntryPath(header.Name, header.Typeflag == tar.TypeDir)
	if err != nil {
		return err
	}
	if marker, ok := strings.CutPrefix(path.base(), whiteoutPrefix); ok {
		return a.applyWhiteout(header.Name, path, marker, layer)
	}
	switch header.Typeflag {
	case tar.TypeChar, tar.TypeBlock:
		return a.skipDevice(header.Name, path)
	}
	if header.Typeflag != tar.TypeDir {
		if path.isRoot() {
			return refuse(header.Name, ruleRootEntry, "")
		}
	}
	metadata, err := newEntryMetadata(header)
	if err != nil {
		return err
	}
	a.counts.impliedFolders += layer.add(path)
	if err := a.checkEntryBound(header.Name); err != nil {
		return err
	}
	switch header.Typeflag {
	case tar.TypeReg:
		return a.applyFile(archive, header, path, metadata)
	case tar.TypeDir:
		return a.applyFolder(header.Name, path, metadata, layer)
	case tar.TypeSymlink:
		return a.applySymlink(header, path, metadata)
	case tar.TypeLink:
		return a.applyHardLink(header, path)
	case tar.TypeFifo:
		return a.applyFifo(header.Name, path, metadata)
	}
	return refuse(header.Name, ruleEntryType, "type "+strconv.QuoteRuneToASCII(rune(header.Typeflag)))
}

// entryMetadata is what an entry sets on what it creates, judged.
type entryMetadata struct {
	uid      int
	gid      int
	mode     uint32
	accessed unix.Timespec
	modified unix.Timespec
	// attributes are kept on a file or a folder only.
	attributes []entryAttribute
}

// newEntryMetadata judges the owner and the attributes of an entry and
// answers what it sets.
func newEntryMetadata(header *tar.Header) (entryMetadata, error) {
	uid, gid, err := entryOwner(header.Name, header.Uid, header.Gid)
	if err != nil {
		return entryMetadata{}, err
	}
	attributes, err := entryAttributes(header.Name, header.PAXRecords)
	if err != nil {
		return entryMetadata{}, err
	}
	modified, err := unix.TimeToTimespec(header.ModTime)
	if err != nil {
		return entryMetadata{}, failEntry(header.Name, err)
	}
	accessedAt := header.AccessTime
	if accessedAt.IsZero() {
		accessedAt = header.ModTime
	}
	accessed, err := unix.TimeToTimespec(accessedAt)
	if err != nil {
		return entryMetadata{}, failEntry(header.Name, err)
	}
	return entryMetadata{
		uid:        uid,
		gid:        gid,
		mode:       uint32(header.Mode & modeBits), //nolint:gosec // G115: masked to the twelve mode bits
		accessed:   accessed,
		modified:   modified,
		attributes: attributes,
	}, nil
}

// applyFile creates a regular file: what the name held is moved away, and
// a new file is created by descriptor, so nothing is written through an
// inode that existed before the entry.
func (a *layerApplier) applyFile(archive io.Reader, header *tar.Header, path entryPath, metadata entryMetadata) error {
	a.counts.contentBytes += uint64(max(header.Size, 0)) //nolint:gosec // G115: not negative
	if a.counts.contentBytes > a.limits.expandedBytesMax {
		return refuse(header.Name, ruleExpandedBytes, "the files hold more than "+textbuf.StringUint(a.limits.expandedBytesMax)+" bytes")
	}
	return a.inParent(header.Name, path, func(parent int, name string) error {
		if err := a.moveAway(parent, name); err != nil {
			return err
		}
		fd, err := unix.Openat(parent, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, createMode)
		if err != nil {
			return err
		}
		file := fileFromDescriptor(fd, name)
		_, err = io.Copy(file, archive)
		if err == nil {
			err = setMetadata(fd, metadata)
		}
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		return setTimes(parent, name, metadata)
	})
}

// applyFolder creates a folder, or keeps the folder the name is and updates
// its metadata. Its times are set when the layer ends, after its children.
func (a *layerApplier) applyFolder(entry string, path entryPath, metadata entryMetadata, layer *layerState) error {
	layer.folders = append(layer.folders, folderTimes{path: path, metadata: metadata})
	if path.isRoot() {
		if err := setFolderMetadata(descriptor(a.root), ".", metadata); err != nil {
			return failEntry(entry, err)
		}
		return nil
	}
	return a.inParent(entry, path, func(parent int, name string) error {
		if err := a.makeFolder(parent, name); err != nil {
			return err
		}
		return setFolderMetadata(parent, name, metadata)
	})
}

// makeFolder creates the folder name in parent, or keeps the folder the name
// already is. A name that holds anything else is moved away first.
func (a *layerApplier) makeFolder(parent int, name string) error {
	err := unix.Mkdirat(parent, name, createFolderMode)
	if err == nil {
		return nil
	}
	if !errors.Is(err, unix.EEXIST) {
		return err
	}
	var stat unix.Stat_t
	if err := unix.Fstatat(parent, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if stat.Mode&unix.S_IFMT == unix.S_IFDIR {
		return nil
	}
	if err := a.moveAway(parent, name); err != nil {
		return err
	}
	return unix.Mkdirat(parent, name, createFolderMode)
}

// applySymlink creates a symbolic link with its target as written. The
// target is not resolved: a later entry under the link is resolved inside
// the root.
func (a *layerApplier) applySymlink(header *tar.Header, path entryPath, metadata entryMetadata) error {
	if err := checkLinkTarget(header.Name, header.Linkname); err != nil {
		return err
	}
	if len(metadata.attributes) > 0 {
		return refuse(header.Name, ruleAttributeOnType, "a symbolic link")
	}
	return a.inParent(header.Name, path, func(parent int, name string) error {
		if err := a.moveAway(parent, name); err != nil {
			return err
		}
		if err := unix.Symlinkat(header.Linkname, parent, name); err != nil {
			return err
		}
		if err := unix.Fchownat(parent, name, metadata.uid, metadata.gid, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return err
		}
		return setTimes(parent, name, metadata)
	})
}

// applyHardLink links a name to a regular file of the root. The target is
// resolved under the root; one that does not exist or is not a regular file
// refuses the image. The new name shares the target's inode, and with it
// the target's owner, mode and attributes.
func (a *layerApplier) applyHardLink(header *tar.Header, path entryPath) error {
	target, err := parseEntryPath(header.Linkname, false)
	if err != nil {
		return err
	}
	if target.isRoot() {
		return refuse(header.Name, ruleHardLinkTarget, "the root")
	}
	targetFolder, err := a.existingFolder(target.parent())
	if nameAbsent(err) {
		return refuse(header.Name, ruleHardLinkTarget, displayName(header.Linkname)+" does not exist")
	}
	if err != nil {
		return failEntry(header.Name, err)
	}
	err = a.linkTo(header, descriptor(targetFolder), target.base(), path)
	if closeErr := targetFolder.Close(); err == nil {
		err = closeErr
	}
	return err
}

// linkTo links path to targetName in targetFolder, once it is checked to
// be a regular file.
func (a *layerApplier) linkTo(header *tar.Header, targetFolder int, targetName string, path entryPath) error {
	var stat unix.Stat_t
	err := unix.Fstatat(targetFolder, targetName, &stat, unix.AT_SYMLINK_NOFOLLOW)
	if nameAbsent(err) {
		return refuse(header.Name, ruleHardLinkTarget, displayName(header.Linkname)+" does not exist")
	}
	if err != nil {
		return failEntry(header.Name, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return refuse(header.Name, ruleHardLinkTarget, displayName(header.Linkname)+" is not a regular file")
	}
	return a.inParent(header.Name, path, func(parent int, name string) error {
		if err := a.moveAway(parent, name); err != nil {
			return err
		}
		return unix.Linkat(targetFolder, targetName, parent, name, 0)
	})
}

// applyFifo creates a named pipe.
func (a *layerApplier) applyFifo(entry string, path entryPath, metadata entryMetadata) error {
	if len(metadata.attributes) > 0 {
		return refuse(entry, ruleAttributeOnType, "a named pipe")
	}
	return a.inParent(entry, path, func(parent int, name string) error {
		if err := a.moveAway(parent, name); err != nil {
			return err
		}
		if err := unix.Mknodat(parent, name, unix.S_IFIFO|createMode, 0); err != nil {
			return err
		}
		if err := unix.Fchownat(parent, name, metadata.uid, metadata.gid, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return err
		}
		// fchmodat cannot refuse to follow a link before Linux 6.6. The name
		// is the pipe just made, and only the store's one worker writes the
		// staged root, so nothing replaced it.
		if err := unix.Fchmodat(parent, name, metadata.mode, 0); err != nil {
			return err
		}
		return setTimes(parent, name, metadata)
	})
}

// skipDevice passes over a device node under the image's device folder and
// counts it: the runtime mounts its own device folder there, so the node
// could never be seen. A node anywhere else refuses the image. No device
// node is ever created.
func (a *layerApplier) skipDevice(entry string, path entryPath) error {
	if !path.underDevices() {
		return refuse(entry, ruleDeviceNode, "")
	}
	a.counts.devicesSkipped++
	return nil
}

// applyWhiteout applies a name of the whiteout family: the opaque marker
// empties its folder of what earlier layers put there, a whiteout removes
// the name it names and everything under it, and any other marker is not
// created.
func (a *layerApplier) applyWhiteout(entry string, path entryPath, marker string, layer *layerState) error {
	folder := path.parent()
	if marker == opaqueMarker {
		return a.applyOpaque(entry, folder, layer)
	}
	if strings.HasPrefix(marker, whiteoutPrefix) {
		return nil // Another marker of the family: not created.
	}
	switch marker {
	case "", ".", "..":
		return refuse(entry, ruleWhiteoutTarget, "")
	}
	handle, err := a.existingFolder(folder)
	if nameAbsent(err) {
		return nil // No folder, so no name to remove.
	}
	if err != nil {
		return failEntry(entry, err)
	}
	err = a.moveAway(descriptor(handle), marker)
	if closeErr := handle.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return failEntry(entry, err)
	}
	return nil
}

// applyOpaque moves away every child the folder had that this layer did not
// apply. A child this layer applies after the marker is created after the
// removal, and one it applied before is held, so the archive's order does
// not matter.
func (a *layerApplier) applyOpaque(entry string, folder entryPath, layer *layerState) error {
	a.counts.impliedFolders += layer.add(folder)
	if err := a.checkEntryBound(entry); err != nil {
		return err
	}
	handle, err := a.existingFolder(folder)
	if nameAbsent(err) {
		return nil // No folder, so no earlier child.
	}
	if err != nil {
		return failEntry(entry, err)
	}
	err = a.emptyOpaque(descriptor(handle), folder, layer)
	if closeErr := handle.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return failEntry(entry, err)
	}
	return nil
}

// emptyOpaque moves away each child of the folder at handle that the layer
// does not hold.
func (a *layerApplier) emptyOpaque(handle int, folder entryPath, layer *layerState) error {
	fd, err := unix.Openat(handle, ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	listing := fileFromDescriptor(fd, folder.key())
	names, err := listing.Readdirnames(-1)
	if closeErr := listing.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	for _, name := range names {
		if layer.holds(folder, name) {
			continue
		}
		if err := a.moveAway(handle, name); err != nil {
			return err
		}
	}
	return nil
}

// setFolderTimes sets the times of the layer's folders, the last one first,
// so a folder's times are set after those of the folders under it.
func (a *layerApplier) setFolderTimes(folders []folderTimes) error {
	for _, folder := range slices.Backward(folders) {
		if err := a.setFolderTime(folder); err != nil {
			return err
		}
	}
	return nil
}

// setFolderTime sets the times of one folder. A folder a later entry of the
// same layer removed has no times to set, so its absence is passed over.
func (a *layerApplier) setFolderTime(folder folderTimes) error {
	name := folder.path.base()
	if folder.path.isRoot() {
		name = "."
	}
	parent, err := a.existingFolder(folder.path.parent())
	if nameAbsent(err) {
		return nil
	}
	if err != nil {
		return failEntry(folder.path.key(), err)
	}
	err = setTimes(descriptor(parent), name, folder.metadata)
	if nameAbsent(err) {
		err = nil
	}
	if closeErr := parent.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return failEntry(folder.path.key(), err)
	}
	return nil
}

// inParent runs change on the folder that holds path and on the path's
// name, creating the folders on the way that are absent. A refusal passes
// through; any other error is the entry's failure.
func (a *layerApplier) inParent(entry string, path entryPath, change func(parent int, name string) error) error {
	parent, err := a.parentFolder(path)
	if err != nil {
		return failEntry(entry, err)
	}
	err = change(descriptor(parent), path.base())
	if closeErr := parent.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		return nil
	}
	if _, refused := errors.AsType[*layerRefusal](err); refused {
		return err
	}
	return failEntry(entry, err)
}

// parentFolder answers a handle on the folder that holds path, creating the
// folders on the way that are absent. Every component is resolved inside
// the root, a symbolic link included, as a process in the container would
// resolve it: an absolute target starts again at the root, and a
// parent-directory step stops at it.
func (a *layerApplier) parentFolder(path entryPath) (*os.File, error) {
	parent := path.parent()
	if parent.isRoot() {
		return a.rootHandle()
	}
	return pathrs.MkdirAllHandle(a.root, parent.key(), impliedFolderMode)
}

// existingFolder answers a handle on folder, resolved inside the root as
// parentFolder resolves, creating nothing.
func (a *layerApplier) existingFolder(folder entryPath) (*os.File, error) {
	if folder.isRoot() {
		return a.rootHandle()
	}
	return pathrs.OpenatInRoot(a.root, folder.key())
}

// rootHandle answers a new handle on the root, which its caller closes as it
// closes any other folder's handle.
func (a *layerApplier) rootHandle() (*os.File, error) {
	fd, err := unix.Openat(descriptor(a.root), ".", unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return fileFromDescriptor(fd, rootfsFolder), nil
}

// moveAway moves what name holds in parent into the removed folder, so the
// entry that takes the name is a new inode and nothing is written through
// what was there. A name that holds nothing is not an error.
func (a *layerApplier) moveAway(parent int, name string) error {
	a.moved++
	err := unix.Renameat(parent, name, descriptor(a.removed), textbuf.StringUint(a.moved))
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	return err
}

// setMetadata gives an open file or folder its owner, then its mode, then
// its attributes: a change of owner clears the set-user and set-group bits
// and the file capability, so each is set after it.
func setMetadata(fd int, metadata entryMetadata) error {
	if err := unix.Fchown(fd, metadata.uid, metadata.gid); err != nil {
		return err
	}
	if err := unix.Fchmod(fd, metadata.mode); err != nil {
		return err
	}
	for _, attribute := range metadata.attributes {
		if err := unix.Fsetxattr(fd, attribute.name, []byte(attribute.value), 0); err != nil {
			return err
		}
	}
	return nil
}

// setFolderMetadata opens the folder name in parent, refusing a link, and
// sets its metadata.
func setFolderMetadata(parent int, name string, metadata entryMetadata) error {
	fd, err := unix.Openat(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	err = setMetadata(fd, metadata)
	if closeErr := unix.Close(fd); err == nil {
		err = closeErr
	}
	return err
}

// setTimes sets the times of name in parent, of a link itself and never of
// what it points to.
func setTimes(parent int, name string, metadata entryMetadata) error {
	return unix.UtimesNanoAt(parent, name, []unix.Timespec{metadata.accessed, metadata.modified}, unix.AT_SYMLINK_NOFOLLOW)
}

// nameAbsent reports whether err says a name, or a folder on the way to it,
// does not exist.
func nameAbsent(err error) bool {
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	return errors.Is(err, unix.ENOTDIR)
}

// descriptor answers the descriptor of an open file. The file MUST stay
// open while the descriptor is used.
func descriptor(file *os.File) int {
	return int(file.Fd()) //nolint:gosec // G115: a descriptor is a small non-negative int
}

// fileFromDescriptor wraps a descriptor the store opened, so it is closed
// as a file.
func fileFromDescriptor(fd int, name string) *os.File {
	return os.NewFile(uintptr(fd), name) //nolint:gosec // G115: a descriptor from a successful open is not negative
}
