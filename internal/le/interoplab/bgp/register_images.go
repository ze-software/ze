// Design: docs/architecture/testing/interop.md -- images registered by name.
// Related: run.go -- the suite derives its image list from these registrations.
package bgp

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// imageOwner is the owner this suite's registered images name.
const imageOwner = "interop"

// bgpImage registers one image this suite builds from test/interop. Context
// root builds from the checkout (the ze image copies the staged binary);
// any other builds from test/interop itself.
func bgpImage(name, tag, dockerfile string, rootContext, required bool, binaries []interoplab.LabBinary) interoplab.RegisteredImage {
	return interoplab.RegisteredImage{Name: name, Owner: imageOwner, Binaries: binaries,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			producer := filepath.Join(context.Root, "test", "interop")
			build := interoplab.ImageBuild{Name: name, Tag: tag, Dockerfile: filepath.Join(producer, dockerfile),
				Context: producer, Required: required}
			if rootContext {
				build.Context = context.Root
			}
			return build
		}}
}

// The images every scenario of this suite may run, and that a topology
// scenario names. The FRR images are not here: their reference comes from
// the run's environment and from each scenario's pin (run.go).
func init() {
	interoplab.RegisterImage(bgpImage("ze", "ze-interop", "Dockerfile.ze", true, true, LabBinaries()))
	interoplab.RegisterImage(bgpImage(peerBIRD, "bird-interop", "Dockerfile.bird", false, true, nil))
	interoplab.RegisterImage(bgpImage(peerGoBGP, "gobgp-interop", "Dockerfile.gobgp", false, false, nil))
	interoplab.RegisterImage(bgpImage(peerKeepalived, "keepalived-interop", "Dockerfile.keepalived", false, true, nil))
	interoplab.RegisterImage(bgpImage(peerDnsmasq, "dnsmasq-interop", "Dockerfile.dnsmasq", false, false, nil))
	interoplab.RegisterImage(bgpImage(peerKea, "kea-interop", "Dockerfile.kea", false, false, nil))
	interoplab.RegisterImage(bgpImage(peerKnot, "knot-interop", "Dockerfile.knot", false, false, nil))
	interoplab.RegisterImage(bgpImage(peerUnbound, "unbound-interop", "Dockerfile.unbound", false, false, nil))
	interoplab.RegisterImage(bgpImage(peerStayRTR, "stayrtr-interop", "Dockerfile.stayrtr", false, false, nil))
	interoplab.RegisterImage(interoplab.RegisteredImage{Name: peerPMACCT, Owner: imageOwner,
		Build: func(interoplab.ImageContext) interoplab.ImageBuild {
			return interoplab.ImageBuild{Name: peerPMACCT, Tag: defaultPMACCTImage, Pull: true}
		}})
}
