// Design: docs/architecture/testing/interop.md -- images registered by name.
// Related: pppoe.go -- imageBuilds, this suite's list derived from these registrations.
package pppoe

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// imageOwner is the owner this suite's registered images name.
const imageOwner = "interop-pppoe"

// pppoeImage registers one image this suite builds, named by its tag, which
// no other suite shares. labContext builds from test/interop-pppoe rather
// than the checkout (the ze image copies the staged binary from the checkout).
//
// None of them sets a Timeout, so each takes the machine build budget that
// BUILD_TIMEOUT names (`interoplab.Docker`). That field lengthens a bound for
// an image slower than the machine budget, and none of these three needs it:
// all three are one `apk add` on alpine plus a COPY, because the ze binary is
// cross-compiled on the host before the build rather than compiled inside the
// image.
func pppoeImage(tag, dockerfile string, labContext bool) interoplab.RegisteredImage {
	return interoplab.RegisteredImage{Name: tag, Owner: imageOwner,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			directory := filepath.Join(context.Root, suitePath)
			build := interoplab.ImageBuild{Name: tag, Tag: tag, Dockerfile: filepath.Join(directory, dockerfile),
				Context: context.Root, Required: true}
			if labContext {
				build.Context = directory
			}
			return build
		}}
}

func init() {
	interoplab.RegisterImage(pppoeImage(zeImageTag, "Dockerfile.ze", false))
	interoplab.RegisterImage(pppoeImage(accelImageTag, "Dockerfile.accel", true))
	interoplab.RegisterImage(pppoeImage(clientImageTag, "Dockerfile.client", true))
}
