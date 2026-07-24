//go:build !linux

// Design: docs/architecture/testing/ci-format.md -- per-test netns launch mode (Fix B)
// Overview: netns_linux.go -- the real implementation; this stub keeps non-linux builds compiling

package runner

import "errors"

// errNetnsUnsupported is returned by enterTestNetns on non-Linux platforms.
// The runner only calls enterTestNetns when runtime.GOOS == "linux", so this is
// a compile-time stub, not a runtime path; network namespaces and nftables do
// not exist on darwin/BSD, where the netlink suites carry option=skip-os anyway.
var errNetnsUnsupported = errors.New("per-test network namespace mode requires linux")

// enterTestNetns is unsupported off Linux; see the linux build for the contract.
func enterTestNetns(string) (restore func(), hostInode uint64, err error) {
	return nil, 0, errNetnsUnsupported
}

// testNetnsName is unused off Linux (enterTestNetns never succeeds) but kept so
// callers compile without a build tag.
func testNetnsName(string, int) string { return "" }

// provisionNetnsLinks is unsupported off Linux; see the linux build. The runner
// only calls it inside netns mode, which is Linux-only.
func provisionNetnsLinks([]NetnsLinkSpec) error { return errNetnsUnsupported }
