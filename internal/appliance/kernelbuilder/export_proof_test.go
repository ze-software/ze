// Design: docs/architecture/appliance/builder.md -- the kernel builder's Docker lifecycle proof
// Related: container_lifetime_proof_test.go -- the external test these helpers serve

package kernelbuilder

import "testing"

// The real-Docker lifetime proof lives in package kernelbuilder_test, because
// it drives the daemon through interoplab.Docker (the one docker route,
// `./le arch tier check`), and interoplab reaches this package through
// internal/appliance, so an in-package test importing it is a cycle. These
// helpers hand that test the in-package fixtures it needs.

// DockerProofRequest answers a Request Build accepts for the Docker backend,
// with its one config fragment written under a fresh root.
func DockerProofRequest(t *testing.T) Request {
	t.Helper()
	return qemuBuildRequest(t, backendDocker)
}

// WriteProofFixture writes content at relative under root.
func WriteProofFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	writeFixture(t, root, relative, content)
}

// PinProofFreeSpace answers the space guard's minimum for every path until
// the test ends, because space policy is not what the proof is about.
func PinProofFreeSpace(t *testing.T) {
	t.Helper()
	old := freeSpace
	freeSpace = func(string) (uint64, error) { return kernelBuildFreeBytesMin, nil }
	t.Cleanup(func() { freeSpace = old })
}
