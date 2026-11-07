// Design: docs/architecture/testing/interop.md -- images registered by name.
// Related: radiussubscriber.go -- suiteFor, which resolves this suite's list from the registry.
package radiussubscriber

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// imageOwner is the owner this suite's registered images name.
const imageOwner = "interop-radius-subscriber"

// The Ze and LAC images this suite builds, named by their tags. FreeRADIUS is
// the core's registration (interoplab.FreeRADIUSImageName).
func init() {
	interoplab.RegisterImage(interoplab.RegisteredImage{Name: zeImage, Owner: imageOwner,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			return interoplab.ImageBuild{Name: zeImage, Tag: zeImage,
				Dockerfile: filepath.Join(context.Root, labDirectory, "Dockerfile.ze"), Context: context.Root, Required: true}
		}})
	interoplab.RegisterImage(interoplab.RegisteredImage{Name: lacImage, Owner: imageOwner,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			lab := filepath.Join(context.Root, labDirectory)
			return interoplab.ImageBuild{Name: lacImage, Tag: lacImage,
				Dockerfile: filepath.Join(lab, "Dockerfile.lac"), Context: lab, Required: true}
		}})
}
