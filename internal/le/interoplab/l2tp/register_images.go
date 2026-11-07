// Design: docs/architecture/testing/interop.md -- images registered by name.
// Related: l2tp.go -- imageBuilds, which selects this suite's list from these registrations.
package l2tp

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// imageOwner is the owner this suite's registered images name.
const imageOwner = "interop-l2tp"

// The images this suite builds, named by their tag, which no other suite
// shares, so a topology scenario names one without a collision. The FRR
// image is not registered: its reference comes from the run's environment.
const (
	imageZe       = "ze-l2tp-interop"
	imageLAC      = "ze-l2tp-lac"
	imageRadius   = "ze-l2tp-radius"
	imageAVPRelay = "ze-l2tp-avp-relay"
	imageKnot     = "ze-l2tp-knot"
	imageFRR      = "frr"
)

// l2tpImage registers one built image. under is the Dockerfile's directory
// relative to the checkout, and labContext says the build context is that
// directory rather than the checkout.
func l2tpImage(name, under, dockerfile string, labContext bool) interoplab.RegisteredImage {
	return interoplab.RegisteredImage{Name: name, Owner: imageOwner,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			directory := filepath.Join(context.Root, filepath.FromSlash(under))
			build := interoplab.ImageBuild{Name: name, Tag: name, Dockerfile: filepath.Join(directory, dockerfile),
				Context: context.Root, Required: true}
			if labContext {
				build.Context = directory
			}
			return build
		}}
}

func init() {
	interoplab.RegisterImage(l2tpImage(imageZe, "test/interop-l2tp", "Dockerfile.ze", false))
	interoplab.RegisterImage(l2tpImage(imageLAC, "test/interop-l2tp", "Dockerfile.lac", true))
	interoplab.RegisterImage(l2tpImage(imageRadius, "internal/le/interoplab/l2tp/radiusmock", "Dockerfile", false))
	interoplab.RegisterImage(l2tpImage(imageAVPRelay, "internal/le/interoplab/l2tp/avprelay", "Dockerfile", false))
	interoplab.RegisterImage(l2tpImage(imageKnot, "test/interop", "Dockerfile.knot", true))
}
