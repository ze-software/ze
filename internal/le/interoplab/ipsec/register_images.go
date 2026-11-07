// Design: docs/architecture/testing/interop.md -- images registered by name.
// Related: ipsec.go -- RunAt, which derives this suite's list from these registrations.
package ipsec

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// imageOwner is the owner this suite's registered images name.
const imageOwner = "interop-ipsec"

// ipsecImage registers one image this suite builds, named by its tag, which
// no other suite shares. labContext builds from test/interop-ipsec rather
// than the checkout (the ze image copies the staged binary from the checkout).
func ipsecImage(tag, dockerfile string, labContext bool) interoplab.RegisteredImage {
	return interoplab.RegisteredImage{Name: tag, Owner: imageOwner,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			directory := filepath.Join(context.Root, "test", "interop-ipsec")
			build := interoplab.ImageBuild{Name: tag, Tag: tag, Dockerfile: filepath.Join(directory, dockerfile),
				Context: context.Root, Required: true}
			if labContext {
				build.Context = directory
			}
			return build
		}}
}

// The FRR image is not registered: its reference comes from the run's
// environment.
func init() {
	interoplab.RegisterImage(ipsecImage(zeImage, "Dockerfile.ze", false))
	interoplab.RegisterImage(ipsecImage(swanImage, "Dockerfile.strongswan", true))
	interoplab.RegisterImage(ipsecImage(natImage, "Dockerfile.nat", true))
}
