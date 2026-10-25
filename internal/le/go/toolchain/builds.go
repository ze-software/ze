// Design: docs/contributing/running-commands.md -- the launcher and the builds it runs
//
// Related: gotoolchain.go -- NamedLauncherDir, the `./le --name` layout beside this one

package gotoolchain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// buildsEntry is the bin/ directory that holds one directory per build the
// launcher made from a commit: bin/builds/<name>/le. Its name does not start
// with `le`, so a shell completing `bin/le` meets only the dispatcher.
const buildsEntry = "builds"

// buildBinaryName is the file every build holds. The personality is chosen
// from the binary's file name (cmd/ze, binaryName), so it is always `le`.
const buildBinaryName = "le"

// buildTimeLayout is the UTC commit time that opens a build name, fixed width
// so that sorting the names sorts the builds by commit time.
const buildTimeLayout = "20060102T150405Z"

// The short commit hash that follows the time: git's abbreviation grows with
// the repository, so any length from git's minimum to a full SHA-1 is a name.
const (
	buildShaLengthMin = 7
	buildShaLengthMax = 40
)

// The launcher's own entries beside the builds. Each opens with a dot, so
// ParseBuildName never reads one as a build. The `le` script holds the copy
// that cannot ask this file, and TestBuildEntriesMatchTheShellScript pins it.
const (
	// BuildFailedPrefix opens a build moved aside by a rollback
	// (.failed-<build name>, or .failed-<build name>.<pid> when that name was
	// taken), and the record of a HEAD that did not build
	// (.failed-<full commit>.log, and .failed-<full commit>.reported once it
	// was reported).
	BuildFailedPrefix = ".failed-"
	// BuildStagingPrefix opens the directory a build compiles into,
	// .staging-<builder pid>, before one rename publishes it.
	BuildStagingPrefix = ".staging-"
	// BuildLockEntry is the lock one builder holds, a directory with its host and pid.
	BuildLockEntry = ".building"
	// BuildLegacyEntry holds the le binary the layout migration kept from
	// before bin/builds existed, run only while no build does.
	BuildLegacyEntry = ".previous-legacy"
)

// BuildsDir answers bin/builds, where the launcher renames each build in as
// bin/builds/<UTC commit time>-<short sha>[-<goos>-<goarch>]/le. The `le`
// script holds the one copy that cannot ask this function.
func BuildsDir(root string) string {
	return filepath.Join(root, "bin", buildsEntry)
}

// RunnableBuild answers whether a build directory holds the regular le the
// launcher would exec: a missing le, or one that is a symbolic link, is no
// build to run. The `le` script holds the copy that cannot ask this function,
// and TestLauncherAgreesWithTheResolver pins the two to one answer. A build it cannot read is an error naming the file. The
// caller has already read the directory's name with ParseBuildName and seen a
// directory, never a symbolic link.
func RunnableBuild(build string) (bool, error) {
	binary := filepath.Join(build, buildBinaryName)
	info, err := os.Lstat(binary)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("le: cannot read the build %s: %w", binary, err)
	}
	return info.Mode().IsRegular(), nil
}

// BuildName is a parsed bin/builds entry name: the build of one commit for
// one platform. Its fields are private, so outside this package only
// ParseBuildName fills one; the zero value names no build and is never
// answered beside true.
type BuildName struct {
	sha      string
	platform string
}

// Sha answers the short commit hash the build was made from.
func (b BuildName) Sha() string { return b.sha }

// Platform answers "" for a build of the checkout's own platform, and
// <goos>-<goarch> for a build of another one.
func (b BuildName) Platform() string { return b.platform }

// ParseBuildName answers the parts of a build name and whether name is one:
// <UTC time>-<short lowercase sha>, then optionally -<goos>-<goarch>. A name
// that opens with a dot (a rollback, the lock, a staging directory) never is.
func ParseBuildName(name string) (BuildName, bool) {
	stamp, rest, found := strings.Cut(name, "-")
	if !found {
		return BuildName{}, false
	}
	if len(stamp) != len(buildTimeLayout) {
		return BuildName{}, false
	}
	if _, err := time.Parse(buildTimeLayout, stamp); err != nil {
		return BuildName{}, false
	}
	sha, platform, suffixed := strings.Cut(rest, "-")
	if !buildSha(sha) {
		return BuildName{}, false
	}
	if suffixed {
		return parseBuildPlatform(sha, platform)
	}
	// No suffix is a name too: the build of the checkout's own platform.
	return BuildName{sha: sha}, true
}

// parseBuildPlatform answers a build name whose sha is followed by a platform
// suffix, and whether that suffix is <goos>-<goarch> in Go's words.
func parseBuildPlatform(sha, platform string) (BuildName, bool) {
	goos, goarch, paired := strings.Cut(platform, "-")
	if !paired {
		return BuildName{}, false
	}
	if !buildPlatformWord(goos) {
		return BuildName{}, false
	}
	if !buildPlatformWord(goarch) {
		return BuildName{}, false
	}
	return BuildName{sha: sha, platform: platform}, true
}

// buildSha answers whether a name part is a short commit hash as git prints
// it: lowercase hexadecimal, from buildShaLengthMin to buildShaLengthMax long.
func buildSha(sha string) bool {
	if len(sha) < buildShaLengthMin {
		return false
	}
	if len(sha) > buildShaLengthMax {
		return false
	}
	for _, character := range sha {
		if !lowerHexCharacter(character) {
			return false
		}
	}
	return true
}

// buildPlatformWord answers whether a name part is spelled like a GOOS or a
// GOARCH: one or more lowercase letters and digits.
func buildPlatformWord(word string) bool {
	if word == "" {
		return false
	}
	for _, character := range word {
		if !lowerAlphanumeric(character) {
			return false
		}
	}
	return true
}

// lowerHexCharacter answers whether one character is a lowercase
// hexadecimal digit.
func lowerHexCharacter(character rune) bool {
	return character >= '0' && character <= '9' || character >= 'a' && character <= 'f'
}

// lowerAlphanumeric answers whether one character is a lowercase letter or a
// digit.
func lowerAlphanumeric(character rune) bool {
	return character >= '0' && character <= '9' || character >= 'a' && character <= 'z'
}
