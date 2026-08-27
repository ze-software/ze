//go:build !linux

// Design: docs/architecture/testing/interop.md -- Linux-only AF_PACKET injection.
package bgp

import "errors"

func injectISISPurgeHost(int, string, []byte) error {
	return errors.New("native IS-IS purge injection requires Linux AF_PACKET and network namespaces")
}
