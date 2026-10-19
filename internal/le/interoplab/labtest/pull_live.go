// Design: docs/architecture/testing/interop.md -- a package test's own server container (labtest)
// Related: pull_docker.go -- the answer outside the live tag.

//go:build live

package labtest

// pullFailureSkips is true under the live tag: a live test needs the internet
// anyway, so an image it cannot pull skips an ad hoc run, and only
// ZE_REQUIRE_DOCKER=1 makes it a failure (owner decision Q-6).
const pullFailureSkips = true
