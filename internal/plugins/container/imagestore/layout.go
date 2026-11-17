// Design: docs/architecture/container/image-store.md -- the store layout under the data directory
// Related: publish.go -- what goes into the folders this file opens
// Related: digest.go -- the digest that names an image folder

package imagestore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The store layout under the data directory. The names are the field
// contract: an appliance in the field holds folders with these names, so a
// name never changes once shipped.
const (
	// storeFolder holds everything of the feature.
	storeFolder = "container"
	// imagesFolder holds one published folder per image digest.
	imagesFolder = "images"
	// layersFolder holds the writable layers, by container and digest.
	layersFolder = "layers"
	// volumesFolder holds the volumes, by container and volume name.
	volumesFolder = "volumes"
	// blobsFolder holds the checked blobs of the one fetch that stopped
	// before it published.
	blobsFolder = "blobs"
	// stagingFolder holds what is not checked, not applied or not removed
	// yet. It is emptied at every start.
	stagingFolder = "staging"
)

// Folder modes. The store root and every folder the store creates under it
// give no access to group or others: an applied image holds set-user files,
// and no local user may reach one. The data directory above it, when the
// store creates it, is closed to others too: nothing in it is theirs. An
// existing data directory keeps the mode it has.
const (
	storeMode   fs.FileMode = 0o700
	dataDirMode fs.FileMode = 0o750
	// storeOpenBits are the permission bits of group and others, which the
	// store root MUST NOT carry.
	storeOpenBits fs.FileMode = 0o077
)

// storeSubfolders are the folders prepareStoreRoot creates under the store
// root. Staging is last: it is the one emptied after creation.
var storeSubfolders = []string{imagesFolder, layersFolder, volumesFolder, blobsFolder, stagingFolder}

// storeRoot is the store root, opened and checked: a directory reached
// through no symbolic link, owned by the user Ze runs as, and closed to
// group and others. Only openStoreRoot builds one.
//
// Every call goes through root, so a name the store builds can never leave
// the store. dir is the same directory, held for the one call os.Root does
// not offer: a rename that refuses an existing target. Its read methods are
// safe for concurrent use; close MUST be called once, after the last of
// them.
type storeRoot struct {
	path string
	dir  *os.File
	root *os.Root
}

// prepareStoreRoot creates the store root and its folders where they are
// absent, opens it, and empties staging. It MUST run once per run of the
// plugin, before any fetch: emptying staging under a fetch in flight would
// remove its work. The caller MUST close the answer.
func prepareStoreRoot(path string) (*storeRoot, error) {
	if err := os.MkdirAll(filepath.Dir(path), dataDirMode); err != nil {
		return nil, storeCannotOpen(path, err)
	}
	if err := os.Mkdir(path, storeMode); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return nil, storeCannotOpen(path, err)
		}
	}
	root, err := openStoreRoot(path)
	if err != nil {
		return nil, err
	}
	if err := root.makeSubfolders(); err != nil {
		return nil, errors.Join(err, root.close())
	}
	if err := root.emptyStaging(); err != nil {
		return nil, errors.Join(err, root.close())
	}
	return root, nil
}

// openStoreRoot opens the existing store root at path and checks it. A store
// root that fails a check is not used at all: its answer is an operator
// error that names the folder, the check and the fix. The caller MUST close
// the answer.
func openStoreRoot(path string) (*storeRoot, error) {
	linked, err := os.Lstat(path)
	if err != nil {
		return nil, storeCannotOpen(path, err)
	}
	if linked.Mode()&fs.ModeSymlink != 0 {
		return nil, opererr.New(
			"the container store "+path+" is a symbolic link",
			"the store is used only at its own path, so no link can move the images, layers and volumes it holds",
			"rm "+path+" && mkdir -m 0700 "+path,
		)
	}
	dir, err := os.Open(path) //nolint:gosec // G304: the store root resolved from the data directory, never a name a request carries
	if err != nil {
		return nil, storeCannotOpen(path, err)
	}
	opened, err := dir.Stat()
	if err != nil {
		return nil, errors.Join(storeCannotOpen(path, err), dir.Close())
	}
	if err := checkStoreFolder(path, linked, opened); err != nil {
		return nil, errors.Join(err, dir.Close())
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, errors.Join(storeCannotOpen(path, err), dir.Close())
	}
	// The two handles are opened by path, one after the other. The rename
	// through dir and every other call through root MUST reach one folder.
	rooted, err := root.Stat(".")
	if err != nil {
		return nil, errors.Join(storeCannotOpen(path, err), root.Close(), dir.Close())
	}
	if !os.SameFile(opened, rooted) {
		return nil, errors.Join(storeChanged(path), root.Close(), dir.Close())
	}
	return &storeRoot{path: path, dir: dir, root: root}, nil
}

// checkStoreFolder refuses a store root that is not the folder found at path,
// is not a directory, is not owned by the user Ze runs as, or is open to
// group or others. linked is what path named, opened what was opened.
func checkStoreFolder(path string, linked, opened fs.FileInfo) error {
	if !os.SameFile(linked, opened) {
		return storeChanged(path)
	}
	if !opened.IsDir() {
		return opererr.New(
			"the container store "+path+" is not a folder",
			"images, layers and volumes are folders under it",
			"rm "+path+" && mkdir -m 0700 "+path,
		)
	}
	stat, ok := opened.Sys().(*syscall.Stat_t)
	if !ok {
		return storeCannotOpen(path, errors.New("its owner cannot be read on this system"))
	}
	euid := textbuf.StringInt(int64(os.Geteuid()))
	owner := textbuf.StringUint32(stat.Uid)
	if owner != euid {
		return opererr.New(
			"the container store "+path+" belongs to user "+owner+", not to the user ze runs as ("+euid+")",
			"another user could change the images and layers a container runs",
			"chown "+euid+" "+path,
		)
	}
	if opened.Mode().Perm()&storeOpenBits != 0 {
		return opererr.New(
			"the container store "+path+" is open to group or others ("+opened.Mode().Perm().String()+")",
			"an image holds set-user files, and a local user who reached one could run it as its owner",
			"chmod 0700 "+path,
		)
	}
	return nil
}

// makeSubfolders creates each folder of the layout that is absent, and
// refuses a name that holds anything but a folder.
func (r *storeRoot) makeSubfolders() error {
	for _, name := range storeSubfolders {
		err := r.root.Mkdir(name, storeMode)
		if err == nil {
			continue
		}
		if !errors.Is(err, fs.ErrExist) {
			return storeCannotOpen(r.path, err)
		}
		info, err := r.root.Lstat(name)
		if err != nil {
			return storeCannotOpen(r.path, err)
		}
		if !info.IsDir() {
			folder := filepath.Join(r.path, name)
			return opererr.New(
				"the container store's "+folder+" is not a folder",
				"the store keeps its "+name+" there",
				"rm "+folder+" && mkdir -m 0700 "+folder,
			)
		}
	}
	return nil
}

// emptyStaging removes everything under staging and keeps the folder. What
// staging holds is never read as an image: a download not yet checked, a
// root being applied, a folder being removed. A run that stopped left it, so
// it is removed whole. The checked blobs are not in staging and stay.
func (r *storeRoot) emptyStaging() error {
	staging, err := r.root.Open(stagingFolder)
	if err != nil {
		return storeCannotOpen(r.path, err)
	}
	names, err := staging.Readdirnames(-1)
	if closeErr := staging.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return storeCannotOpen(r.path, err)
	}
	for _, name := range names {
		if err := r.root.RemoveAll(filepath.Join(stagingFolder, name)); err != nil {
			folder := filepath.Join(r.path, stagingFolder, name)
			return opererr.Wrap(err,
				"the container store could not remove "+folder+", which a stopped run left in staging",
				"staging is emptied at every start, so a half-applied image never stays on disk",
				"rm -rf "+folder,
			)
		}
	}
	return nil
}

// close closes both handles of the store root.
func (r *storeRoot) close() error {
	return errors.Join(r.root.Close(), r.dir.Close())
}

// storeCannotOpen is the refusal of a store root the store could not create,
// open or read, with the reason the system gave.
func storeCannotOpen(path string, cause error) error {
	return opererr.Wrap(cause,
		"the container store "+path+" cannot be used",
		"images, layers and volumes are kept there, owned by the user ze runs as and closed to others",
		"mkdir -p -m 0700 "+path,
	)
}

// storeChanged is the refusal of a store root that another folder replaced
// while the store opened it.
func storeChanged(path string) error {
	return opererr.New(
		"the container store "+path+" changed while ze opened it",
		"every call of the store MUST reach the one folder it checked",
		"ze show doctor",
	)
}
