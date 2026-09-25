// Design: docs/architecture/testing/test-health.md -- the requirement model the page counts
// Related: ../../rfc/registry_test.go -- the same link, for the rfc package's tests

package testhealth

// The requirement model reads the RFC carriers, and an interop carrier names
// actions of `le test integration` and `le test deployment`. The carrier check
// asks the live registry which leading words of a workflow command are the
// command, so without these two packages this test binary reads
// `./le test integration interop` as the `integration` verb of a `test` command,
// and no scheduled workflow credits the carrier. The le binary links both
// through internal/le/register.go; this test binary MUST link them too, or it
// judges a registry the product does not have.
import (
	_ "github.com/ze-software/ze/internal/le/test/deployment"
	_ "github.com/ze-software/ze/internal/le/test/integration"
)
