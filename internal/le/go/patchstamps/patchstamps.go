// Design: docs/contributing/patching-the-standard-library.md -- the review stamp, the check ./le verify runs
//
// Package patchstamps carries `le go patch-stamps check`: every patch of both
// families against what it was reviewed for. The refusals are the ones the
// generator and applyVendorPatches print, read through the same functions, so
// a stamp is parsed once per family.

package patchstamps

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	goversionpin "github.com/ze-software/ze/internal/le/go/versionpin"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/vendorpatch"
)

// stampsArea is the name the check is typed as.
const stampsArea = "go patch-stamps"

// stampsActions is the check's command surface.
var stampsActions = leaction.New(stampsArea,
	leaction.Action{Verb: "check", Summary: "refuse a stdlib patch stamped for another release", Why: "refuse a standard-library patch whose review stamp is not go.mod's pinned release and the activated GOVERSION, or a vendor patch whose stamp is not the version vendor/modules.txt lists",
		Answer: runPatchStampsCheck},
)

// Actions answers the check's command surface as data.
func Actions() leaction.List { return stampsActions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return stampsActions.Subs() }

// Answer is the `le go patch-stamps` command.
func Answer(args []string) (any, int) { return stampsActions.Answer(args) }

// runPatchStampsCheck checks the checkout le runs from, against the version
// of the toolchain the tree is built from.
func runPatchStampsCheck() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	cache, err := gen.CacheDir(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	patches, err := gen.Patches(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	toolchain, err := gen.Resolve(root, cache, patches)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if err := checkPatchStamps(root, toolchain.Version); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return "every patch stamp matches what it patches", 0
}

// checkPatchStamps answers every stale, missing or unknown stamp of both
// families, joined, so one run names them all. Each standard-library stamp
// is compared with go.mod's pinned release and with goVersion, the GOVERSION
// of the toolchain the tree is built from; each vendor stamp with
// vendor/modules.txt.
func checkPatchStamps(root, goVersion string) error {
	var failures []error
	patches, err := gen.Patches(root)
	if err != nil {
		return err
	}
	release, err := goversionpin.DeclaredRelease(root)
	if err != nil {
		return fmt.Errorf("patch-stamps: %w", err)
	}
	for _, version := range []string{"go" + release, goVersion} {
		if err := gen.CheckStamps(patches, version); err != nil {
			failures = append(failures, err)
		}
	}
	paths, err := vendorpatch.Patches(root)
	if err != nil {
		return errors.Join(append(failures, err)...)
	}
	vendored, err := vendorpatch.Vendored(root)
	if err != nil {
		return errors.Join(append(failures, err)...)
	}
	for _, relative := range paths {
		if err := vendorpatch.CheckStamp(root, relative, vendored); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
