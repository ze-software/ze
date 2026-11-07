// Design: docs/architecture/testing/qemu-integration.md -- real kernel WireGuard ping.
// Related: wireguardproof.go -- the WireGuard proof, which derives its image list from this registration.
package testdeployment

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// imageOwner is the owner this package's registered images name.
const imageOwner = "docker-wireguard"

// The WireGuard proof's image, named by its tag, so a topology scenario runs
// the same WireGuard peer.
func init() {
	interoplab.RegisterImage(interoplab.RegisteredImage{Name: wireguardImage, Owner: imageOwner,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			return interoplab.ImageBuild{Name: wireguardImage, Tag: wireguardImage,
				Dockerfile: filepath.Join(context.Root, "internal", "le", "test", "deployment", "testdata", "Dockerfile.wireguard"),
				Context:    context.Root, Required: true}
		}})
}
