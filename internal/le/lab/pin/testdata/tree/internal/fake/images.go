package fake

const gnmic = "ghcr.io/openconfig/gnmic:0.42.1" +
	"@sha256:3e135878f555772653a278742d2c7460b47c20e2907e07f611ecf4cc5a2261cf"

const vpp = "ligato/vpp-base:latest"

const local = "ze-tacplusng:test"

// An official image names no repository path: it is a pin all the same.
const probeImage = "alpine:3.21"

// A name and an address read like an official image; this line never says so.
const peer = "router1:10.0.0.1"

// An address with a port has the image shape and is not an image.
const listen = "localhost:8080"
