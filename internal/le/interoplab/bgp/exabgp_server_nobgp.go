// Design: docs/architecture/testing/interop.md -- compiled BGP interop personalities.
// Related: exabgp_server.go -- the personality itself, behind the same gate
//
// The exabgp-server personality reads an UPDATE with the BGP engine's own
// message types, which feature-gates.txt keeps behind ze_bgp. A build without
// that tag has no renderer to run, so the personality says so rather than
// disappearing from the dispatch and leaving "unknown interop-bgp personality"
// as the only clue.

//go:build !ze_bgp

package bgp

import (
	"errors"
	"io"
)

// errExaBGPServerNeedsBGP is what the personality answers without the BGP gate.
var errExaBGPServerNeedsBGP = errors.New("interop-bgp exabgp-server: this binary is built without ze_bgp, so it carries no BGP message renderer")

func runExaBGPServer([]string, io.Writer) error {
	return errExaBGPServerNeedsBGP
}
