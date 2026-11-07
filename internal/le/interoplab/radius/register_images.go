// Design: docs/architecture/testing/interop.md -- images registered by name.
// Related: radius.go -- suiteFor, which resolves this suite's list from the registry.
package radius

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// imageOwner is the owner this suite's registered images name.
const imageOwner = "interop-radius"

// The Ze image this suite builds, named by its tag. FreeRADIUS is the
// core's registration (interoplab.FreeRADIUSImageName).
func init() {
	interoplab.RegisterImage(interoplab.RegisteredImage{Name: zeImage, Owner: imageOwner,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			return interoplab.ImageBuild{Name: zeImage, Tag: zeImage,
				Dockerfile: filepath.Join(context.Root, labDirectory, "Dockerfile.ze"), Context: context.Root, Required: true}
		}})
}
