// Design: docs/contributing/rfc-conformance-gates.md -- interop evidence tier
// Related: carriers.go -- nativeActionsIn, interopTrees

package rfc

// The interop carriers name actions of `le test integration` and
// `le test deployment`, and the nightly runs them through `le test qemu
// docker-lab`, whose action table says which keyword carries the lab's words.
// nativeActionsIn asks the live registry which leading words of a workflow
// command are the command, and which keyword nests another, so without these
// packages the test binary reads `./le test integration interop` as the
// `integration` verb of a `test` command, and no scheduled workflow credits a
// carrier. The le binary links all three through internal/le/register.go; the
// test binary MUST link them too, or it judges a registry the product does not
// have.
import (
	_ "github.com/ze-software/ze/internal/le/test/deployment"
	_ "github.com/ze-software/ze/internal/le/test/integration"
	_ "github.com/ze-software/ze/internal/le/test/qemu"
)
