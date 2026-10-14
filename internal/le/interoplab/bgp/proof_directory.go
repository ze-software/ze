// Design: docs/architecture/testing/interop.md -- retained proof under the session scratch.
// Related: check_parsed_empty_mp.go -- retains the parsed-empty MP_UNREACH proof.
// Related: check_srv6_identity.go -- retains the SRv6 next-hop identity proof.
package bgp

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// retainedProofDirectory creates a fresh directory for one checker's
// proof.json under the session scratch of check.Checkout. The scratch path
// lepath answers is relative to the checkout, so it is joined to the checkout
// here: resolved against the working directory instead, a `go test` run wrote
// proof into the package directory inside the source tree. An empty Checkout is
// refused rather than read as the working directory.
func retainedProofDirectory(check *interoplab.CheckContext) (string, error) {
	if check.Checkout == "" {
		return "", errors.New(check.Source.Name + ": no checkout to retain proof under")
	}
	paths, err := lepath.ResolveSession(check.Checkout, true)
	if err != nil {
		return "", err
	}
	return os.MkdirTemp(filepath.Join(check.Checkout, paths.Scratch), check.Source.Name+"-")
}
