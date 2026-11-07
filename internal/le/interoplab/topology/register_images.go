// Design: docs/architecture/testing/interop.md -- images registered by name.
// Related: register_access_chain.go -- the scenarios that run this image.
package topology

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// imageOwner is the owner the topology suite's registered images name.
const imageOwner = "interop-topology"

// freeRADIUSLinkImage is the pinned FreeRADIUS release with iproute2 added,
// so the server joins a link: a link member's start wrapper builds its VLAN
// devices with ip before the server starts (interoplab.Link), and the pulled
// image carries no ip. The pin itself stays the core's one declaration
// (interoplab.FreeRADIUSImage), handed to the Dockerfile as a build argument.
const freeRADIUSLinkImage = "ze-topology-freeradius"

// accessChainDirectory holds the access chain's shared lab files: the LAC
// configuration every access scenario runs unchanged, and the FreeRADIUS
// server files and image.
const accessChainDirectory = "test/interop-topology/access-chain"

func init() {
	interoplab.RegisterImage(interoplab.RegisteredImage{Name: freeRADIUSLinkImage, Owner: imageOwner,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			lab := filepath.Join(context.Root, filepath.FromSlash(accessChainDirectory))
			return interoplab.ImageBuild{Name: freeRADIUSLinkImage, Tag: freeRADIUSLinkImage,
				Dockerfile: filepath.Join(lab, "Dockerfile.freeradius"), Context: lab,
				BuildArgs: []string{"FREERADIUS_IMAGE=" + interoplab.FreeRADIUSImage}, Required: true}
		}})
}
