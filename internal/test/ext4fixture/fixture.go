// Design: docs/architecture/testing/qemu-integration.md -- the ext4 fixtures a guest test attaches
// Detail: fixture_linux.go -- attaching a fixture in the guest through a loop device
//
// Package ext4fixture makes, on the host, the small ext4 filesystems the QEMU
// guest tests of the ext4 and project-quota leaves attach, and attaches them in
// the guest.
//
// The guest runs Ze's own system, which carries no mkfs and no tune2fs, and
// the tests need filesystems they can enlarge and filesystems that differ in
// the quota feature and the stored mount option. None of them may be the
// guest's own persistent partition. So `./le test qemu guest-binaries` makes
// them on the host with the persistent partition's format definition
// (internal/appliance/permformat) and the host's e2fsprogs, into a directory
// the guest sees through the shared checkout. `./le test qemu all` names that
// directory to its children in DirEnv.
//
// Make is host tooling and runs external programs. It is not safe for
// concurrent use on one directory.
package ext4fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ze-software/ze/internal/appliance/permformat"
)

// DirEnv names the variable that holds the absolute directory of the
// fixtures. `./le test qemu all` sets it for every child. A test that finds it
// unset is not in a guest run and skips; a test that finds it set and the
// fixture missing fails.
const DirEnv = "ZE_EXT4_FIXTURES"

// ErrToolNotInstalled is what Make answers when the caller resolved no path
// for one of the two e2fsprogs tools: the host does not carry it, so no
// fixture was written. A caller branches on it to name the install as the fix.
var ErrToolNotInstalled = errors.New("ext4fixture: an e2fsprogs tool is not installed")

// Fixture names one fixture file.
type Fixture string

// The fixtures, each a filesystem of SizeBytes.
const (
	// PermFormat is the persistent partition's whole format: the format
	// step, then the quota step with the stored mount option.
	PermFormat Fixture = "perm-format.img"
	// PermFormatNoMountOption has the format step and the quota feature,
	// without the stored mount option.
	PermFormatNoMountOption Fixture = "perm-format-no-mount-option.img"
	// FormatStepOnly has the format step alone: the project feature and no
	// quota feature, which is the filesystem between the two steps.
	FormatStepOnly Fixture = "perm-format-step-only.img"
)

// SizeBytes is the size of every fixture: four block groups of the format.
// It holds the format's 64 MiB journal, which mke2fs refuses on a filesystem
// whose free space is under twice the journal.
const SizeBytes = 256 << 20

// recipes answers the quota step each fixture takes after the format step;
// nil means none. It is the one list of fixtures.
func recipes() map[Fixture][]string {
	return map[Fixture][]string{
		PermFormat:              permformat.QuotaStepArgs(),
		PermFormatNoMountOption: permformat.QuotaFeatureArgs(),
		FormatStepOnly:          nil,
	}
}

// Make writes every fixture into dir, with formatTool (mkfs.ext4) and
// quotaTool (tune2fs) at the paths the caller resolved; canceling ctx stops
// the e2fsprogs step that is running. A fixture is written
// under a temporary name and renamed into place, so a failed run leaves no
// fixture that a guest test would trust.
func Make(ctx context.Context, dir, formatTool, quotaTool string) error {
	if formatTool == "" {
		return fmt.Errorf("%w: %s", ErrToolNotInstalled, permformat.FormatTool)
	}
	if quotaTool == "" {
		return fmt.Errorf("%w: %s", ErrToolNotInstalled, permformat.QuotaTool)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	for fixture, quotaStep := range recipes() {
		if err := makeFixture(ctx, dir, fixture, formatTool, quotaTool, quotaStep); err != nil {
			return err
		}
	}
	return nil
}

// makeFixture writes one fixture for Make.
func makeFixture(ctx context.Context, dir string, fixture Fixture, formatTool, quotaTool string, quotaStep []string) error {
	target := filepath.Join(dir, string(fixture))
	partial := target + ".partial"
	file, err := os.Create(filepath.Clean(partial))
	if err != nil {
		return err
	}
	if err := file.Truncate(SizeBytes); err != nil {
		file.Close() //nolint:errcheck,gosec // the truncate error is the one reported, and the partial file is rewritten by the next run
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := run(ctx, formatTool, append(permformat.FormatArgs(), partial)...); err != nil {
		return fmt.Errorf("ext4fixture: format %s: %w", fixture, err)
	}
	if quotaStep != nil {
		if err := run(ctx, quotaTool, append(quotaStep, partial)...); err != nil {
			return fmt.Errorf("ext4fixture: quota step of %s: %w", fixture, err)
		}
	}
	return os.Rename(partial, target)
}

// run runs one e2fsprogs step and answers its output with its failure.
func run(ctx context.Context, tool string, args ...string) error {
	output, err := exec.CommandContext(ctx, tool, args...).CombinedOutput() //nolint:gosec // the tool path is resolved by the caller, the arguments are permformat's
	if err != nil {
		return fmt.Errorf("%s %v: %w: %s", tool, args, err, output)
	}
	return nil
}
