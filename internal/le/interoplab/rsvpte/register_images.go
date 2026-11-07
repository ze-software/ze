// Design: docs/architecture/testing/interop.md -- images registered by name.
// Related: rsvpte.go -- suiteFor, which derives this suite's list from these registrations.
package rsvpte

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// imageOwner is the owner this suite's registered images name.
const imageOwner = "interop-rsvpte"

// The images this suite builds, named by their tag, which no other suite
// shares, so a topology scenario names one without a collision.
const (
	imageZe      = "ze-rsvpte-interop"
	imageFreeRtr = "ze-rsvpte-freertr"
)

// rsvpteImage registers one image built from test/interop-rsvpte. labContext
// builds from that directory rather than the checkout (the ze image copies
// the staged binary from the checkout).
func rsvpteImage(tag, dockerfile string, labContext bool) interoplab.RegisteredImage {
	return interoplab.RegisteredImage{Name: tag, Owner: imageOwner,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			directory := filepath.Join(context.Root, labDirectory)
			build := interoplab.ImageBuild{Name: tag, Tag: tag, Dockerfile: filepath.Join(directory, dockerfile),
				Context: context.Root, Required: true}
			if labContext {
				build.Context = directory
			}
			return build
		}}
}

func init() {
	interoplab.RegisterImage(rsvpteImage(imageZe, "Dockerfile.ze", false))
	interoplab.RegisterImage(rsvpteImage(imageFreeRtr, "Dockerfile.freertr", true))
}
