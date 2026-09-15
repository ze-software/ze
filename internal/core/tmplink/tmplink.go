// Design: docs/architecture/testing/qemu-integration.md -- scratch shares
//
// Package tmplink names the checkout tmp paths that can live outside the tree
// and answers the 9p shares a QEMU guest needs to follow them.
//
// One declaration, for the reason internal/core/diskspace exists. `le scratch
// migrate` (internal/le/scratch) MOVES these directories out of a real tmp and
// leaves a symlink behind. Two launchers then hand the guest the checkout over
// 9p, where that symlink stays a symlink: `le qemu run` (internal/le/qemu) and
// `ze appliance kernel --builder qemu` (internal/appliance/kernelbuilder). A
// guest that follows it reaches a path only the host has, so each launcher
// exports the link's target and mounts it at the same absolute path inside the
// guest. internal/appliance is product code and cannot import internal/le, so
// the list and the walk live in this leaf.
package tmplink

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// Migratable is the closed allowlist of build-artifact directories that
// `le scratch migrate` can move out of a real tmp directory. Unclassified
// names stay beside session work.
var Migratable = [...]string{
	"qemu",
	"kernel",
	"gokrazy",
	"golangci-lint-cache",
	"terminal-demos",
}

const (
	tmpName   = "tmp"
	tagPrefix = "zescratch"
)

// Share is one 9p export a guest mounts so one tmp symlink resolves inside it.
type Share struct {
	// Tag is the virtfs mount_tag, the guest's mount source and the device id.
	// It is unique across the shares one tree answers.
	Tag string
	// Host is the resolved host directory QEMU exports.
	Host string
	// Guest is the absolute path the guest mounts the export at. It is the
	// link's own target text, so the symlink the guest reads over 9p resolves
	// into the export without any host lookup.
	Guest string
}

// Shares answers the exports a guest needs once it mounts tree at workspace.
// When tmp itself is a symlink there is one share. When tmp is a real
// directory there is one share for each Migratable child that is a symlink,
// in Migratable order. A relative link target is joined to the guest's view of
// the directory holding the link, so a checkout-relative target keeps its
// workspace-joined path.
func Shares(tree, workspace string) ([]Share, error) {
	tmp := filepath.Join(tree, tmpName)
	info, err := os.Lstat(tmp)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		share, err := shareOf(tmp, workspace, tagPrefix)
		if err != nil {
			return nil, err
		}
		return []Share{share}, nil
	}
	shares := make([]Share, 0, len(Migratable))
	for _, name := range Migratable {
		child := filepath.Join(tmp, name)
		info, err := os.Lstat(child)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		share, err := shareOf(child, path.Join(workspace, tmpName), tagPrefix+"-"+name)
		if err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}
	return shares, nil
}

// shareOf resolves one symlink. guestDir is where the guest sees the
// directory holding the link, which anchors a relative target.
func shareOf(link, guestDir, tag string) (Share, error) {
	target, err := os.Readlink(link)
	if err != nil {
		return Share{}, err
	}
	host, err := filepath.EvalSymlinks(link)
	if err != nil {
		return Share{}, fmt.Errorf("resolve %s: %w", link, err)
	}
	guest := target
	if !filepath.IsAbs(target) {
		guest = path.Clean(path.Join(guestDir, filepath.ToSlash(target)))
	}
	return Share{Tag: tag, Host: host, Guest: guest}, nil
}
