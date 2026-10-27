// Design: docs/architecture/testing/qemu-integration.md -- the guest le that runs every harness command
// Related: alltests.go -- the in-guest run that links it into its PATH shim
// Related: netns_linux.go -- the netns launcher that runs `le test <suite>` through it
// Related: ../../linuxle/linuxle.go -- the one recipe for a linux le

package testqemu

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/linuxle"
)

// leName is the file name of the guest le and of its PATH link. cmd/ze selects
// its personality from the name it was started under, so the link MUST carry
// this name whatever the file behind it is called.
const leName = linuxle.Name

// leTestWord is the first word of every harness command: `le test <name>`.
const leTestWord = "test"

// GuestLeRel answers where the host writes the guest le, relative to the
// checkout the guest mounts at GuestWorkspace. It sits under tmp/, never under
// bin/le-*, which is launcher territory.
func GuestLeRel(goarch string) string {
	return filepath.Join("tmp", "qemu", "linux-"+goarch, leName)
}

// buildGuestLe cross-builds a linux le for goarch into the checkout, through
// job admission, at GuestLeRel(goarch). The guest runs it as `le test ...`, so
// every harness command it runs is the harness of this checkout.
//
// ctx is the calling verb's signal root, or Background for a verb that holds
// none: a done ctx ends the wait for a slot and the build (RunContext).
func buildGuestLe(ctx context.Context, root, goarch string) error {
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		return err
	}
	tags, err := linuxle.Tags(root)
	if err != nil {
		return err
	}
	admission, err := job.NewIn(root)
	if err != nil {
		return err
	}
	admission.Out = os.Stderr

	output := filepath.Join(root, GuestLeRel(goarch))
	environment := append(toolchain.EnvironmentContext(ctx, gotoolchain.EnvOptions{GOOS: guestOS, GOARCH: goarch}),
		linuxle.Overrides(goarch)...)
	if _, code := admission.RunContext(ctx, "qemu-build-le", linuxle.Argv(tags, output), root, environment); code != 0 {
		if err := ctx.Err(); err != nil {
			// The root ended the build or the wait for its slot: say so, and
			// not the code, which reads as a failed compile.
			return fmt.Errorf("build the guest le for linux/%s: stopped by an interrupt or SIGTERM: %w", goarch, err)
		}
		return fmt.Errorf("build the guest le for linux/%s exited %d", goarch, code)
	}
	return nil
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

// guestJobParentName is the file, in the guest's binary directory, that stands
// in for the host job holding this guest's slot.
const guestJobParentName = "host-job"

// guestJobParentEntry is the body of every stand-in parent entry a guest
// names: the label job.insideParent reports as the slot it runs inside.
const guestJobParentEntry = "LABEL=qemu (held by the host)\n"

// guestRunJobParent is the stand-in the `test qemu run` setup writes in the
// guest's own /tmp, never in the shared checkout, and names as the guest
// command's job parent (guestRunParentSetup).
const guestRunJobParent = "/tmp/ze-" + guestJobParentName

// guestRunParentSetup answers the setup step that writes the stand-in parent
// at path and exports it under job.ParentVariable, so every le the guest
// command starts runs inside the slot the host's `test qemu run` holds
// (admitQEMURun) and never admits itself against the host's registry.
func guestRunParentSetup(path string) string {
	var b textbuf.Buffer
	return b.Str("printf '").Str(strings.TrimSuffix(guestJobParentEntry, "\n")).Str("\\n' > ").Str(shellQuote(path)).
		Str(" && export ").Str(job.ParentVariable).Byte('=').Str(shellQuote(path)).String()
}

// guestJobParent writes the stand-in parent entry into dir and answers the
// environment entry that names it (job.ParentVariable).
//
// A runner command (`le test <suite>`) admits its run through the job
// registry of the checkout it finds, and the guest finds the host's checkout
// at /workspace. The guest MUST NOT take a slot there: the host's run already
// holds one, and the registry judges a holder by a process id the other
// machine cannot see, so each side would reap the other's entry. Named as the
// parent, the entry makes every suite run inside the host's slot (AC-45).
func guestJobParent(dir string) (string, error) {
	path := filepath.Join(dir, guestJobParentName)
	if err := os.WriteFile(path, []byte(guestJobParentEntry), 0o644); err != nil { //nolint:gosec // read by the credential-dropped test group
		return "", err
	}
	return job.ParentVariable + "=" + path, nil
}
