// Design: docs/architecture/testing/qemu-integration.md -- the host QEMU harness, scratch shares
// Related: run.go -- ensureScratch and scratchShares, which refuse through danglingScratchLink

package testqemu

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/tmplink"
)

// danglingScratchLink refuses a scratch link of the checkout whose target is
// gone: tmp, each tmplink.Migratable child of a real tmp, and cache. A run
// would otherwise stop on the symptom, `mkdir <tree>/tmp/qemu: file exists`
// from ensureScratch or `resolve <link>: no such file or directory` from
// tmplink.Shares, naming neither the link nor what it points at. Neither
// `./le scratch ensure` nor `./le scratch migrate` creates a missing child
// target (migrate skips a dangling child), so the fix is the directory itself.
// It answers nil when no scratch link dangles.
func danglingScratchLink(tree string) error {
	tmp := filepath.Join(tree, "tmp")
	links := make([]string, 0, len(tmplink.Migratable)+2)
	links = append(links, tmp)
	for _, name := range tmplink.Migratable {
		links = append(links, filepath.Join(tmp, name))
	}
	links = append(links, filepath.Join(tree, "cache"))
	for _, link := range links {
		target, dangling, err := linkTargetMissing(link)
		if err != nil {
			return err
		}
		if dangling {
			return danglingLinkRefusal(link, target)
		}
	}
	return nil
}

// linkTargetMissing answers the absolute target of link and true when link is
// a symbolic link whose target does not exist. A path that is absent, or not a
// link, answers false. A link whose target cannot be read is an error, never
// a pass.
func linkTargetMissing(link string) (string, bool, error) {
	info, err := os.Lstat(link)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", false, nil
	}
	target, err := os.Readlink(link)
	if err != nil {
		return "", false, err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	_, err = os.Stat(link)
	if errors.Is(err, os.ErrNotExist) {
		return target, true, nil
	}
	if err != nil {
		return "", false, err
	}
	return target, false, nil
}

// danglingLinkRefusal names the link, its missing target, and the command that
// recreates the target.
func danglingLinkRefusal(link, target string) error {
	return opererr.New(
		link+" is a scratch link to "+target+", which does not exist",
		"the QEMU run writes its caches and guest shares through this link, so every write through it fails",
		"mkdir -p "+shellQuote(target),
	)
}
