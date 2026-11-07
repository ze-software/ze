// Design: docs/architecture/testing/interop.md -- images registered by name.
// Related: freeradius.go -- FreeRADIUSImage, the pinned reference registered here.
package interoplab

// imageOwner names the images the core registers: pulled references more
// than one suite runs, so no single suite owns them.
const imageOwner = "interoplab"

// FreeRADIUSImageName is the registered name of the pinned FreeRADIUS image
// both RADIUS labs and the topology suite run.
const FreeRADIUSImageName = "freeradius"

func init() {
	RegisterImage(RegisteredImage{Name: FreeRADIUSImageName, Owner: imageOwner,
		Build: func(ImageContext) ImageBuild {
			return ImageBuild{Name: FreeRADIUSImageName, Tag: FreeRADIUSImage, Pull: true, Required: true}
		}})
}
