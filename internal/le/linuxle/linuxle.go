// Design: docs/guide/benchmarking.md -- the linux le the perf sender container runs
// Related: ../site/terminaldemo/actions.go -- the linux le the demo container runs
// Related: ../../test/perfrunner/run.go -- the perf runner that mounts it

// Package linuxle declares, once, how a linux le is cross-built for a container
// or a guest.
//
// Every image and guest that runs le itself takes the recipe from here: the
// perf sender (`le perf send`), the terminal-demo recorder
// (`le site terminal-demo pty`), the BGP interop image, the QEMU guests and the
// VPP evidence container (`le test <name>`). Each builder takes its argv, its
// environment and its tags from this package.
package linuxle

import (
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// Name is the file name a linux le MUST carry. cmd/ze selects its personality
// from the name it was started under (defaultDispatch), so the same build
// named anything else is not le.
const Name = "le"

// Base is the build tag a linux le carries before the daemon feature tags:
// ze_le, which adds le's commands and, with them, every harness command
// under `le test`.
const Base = "ze_le"

// Tags answers the build tags of a linux le: Base and every daemon feature tag, so the container's le carries the same feature
// set as the le on the host.
func Tags(root string) (string, error) {
	return repofeaturetags.DaemonBuildTags(root, Base)
}

// Argv answers the go build command line that writes a linux le to output.
// The caller MUST name output with Name, and MUST run the command with
// Overrides applied over its environment.
func Argv(tags, output string) []string {
	return []string{"go", "build", "-tags", tags, "-o", output, "./cmd/ze"}
}

// Overrides answers the environment entries a linux le is built under: linux,
// the given architecture, and CGO_ENABLED=0 so the binary runs in an image
// with no C library. They go LAST in the environment, because os/exec keeps
// the last value of a repeated key.
func Overrides(goarch string) []string {
	return []string{"GOOS=linux", "GOARCH=" + goarch, "CGO_ENABLED=0"}
}
