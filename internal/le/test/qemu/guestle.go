// Design: docs/architecture/testing/qemu-integration.md -- the guest le that runs every harness command
// Related: alltests.go -- the in-guest run that links it into its PATH shim
// Related: netns_linux.go -- the netns launcher that runs `le test <suite>` through it
// Related: ../../linuxle/linuxle.go -- the one recipe for a linux le

package testqemu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/linuxle"
)

// leName is the file name of the guest le and of its PATH link. cmd/ze selects
// its personality from the name it was started under, so the link MUST carry
// this name whatever the file behind it is called.
const leName = linuxle.Name

// leTestWord is the first word of every harness command: `le test <name>`.
const leTestWord = "test"

// guestLeRel answers where the host writes the guest le, relative to the
// checkout the guest mounts at guestWorkspace. It sits under tmp/, never under
// bin/le-*, which is launcher territory.
func guestLeRel(goarch string) string {
	return filepath.Join("tmp", "qemu", "linux-"+goarch, leName)
}

// buildGuestLe cross-builds a linux le for goarch into the checkout, through
// job admission, and answers its path. The guest runs it as `le test ...`, so
// every harness command it runs is the harness of this checkout.
func buildGuestLe(root, goarch string) (string, error) {
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		return "", err
	}
	tags, err := linuxle.Tags(root)
	if err != nil {
		return "", err
	}
	admission, err := job.NewIn(root)
	if err != nil {
		return "", err
	}
	admission.Out = os.Stderr

	output := filepath.Join(root, guestLeRel(goarch))
	environment := append(toolchain.Environment(gotoolchain.EnvOptions{GOOS: "linux", GOARCH: goarch}),
		linuxle.Overrides(goarch)...)
	if _, code := admission.Run("qemu-build-le", linuxle.Argv(tags, output), root, environment); code != 0 {
		return "", fmt.Errorf("build the guest le for linux/%s exited %d", goarch, code)
	}
	return output, nil
}

// guestLeLink writes dir/le as a symlink to the file of the running process,
// which inside a guest is the le that runs this action, and answers the link.
// A previous link at that path is replaced.
func guestLeLink(dir string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", errors.New("the guest le does not know its own executable: " + err.Error())
	}
	link := filepath.Join(dir, leName)
	if err := os.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Symlink(self, link); err != nil {
		return "", err
	}
	return link, nil
}
