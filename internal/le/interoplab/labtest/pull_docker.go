// Design: docs/architecture/testing/interop.md -- a package test's own server container (labtest)
// Related: pull_live.go -- the live-tag answer.

//go:build !live

package labtest

// pullFailureSkips is false outside the live tag: a docker-tagged test whose
// image cannot be pulled fails (owner decision Q-6), because its peer is a
// fixed image and a registry outage is a red the run must report.
const pullFailureSkips = false
