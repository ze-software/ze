// Design: docs/architecture/testing/qemu-integration.md -- running one .ci in a Ze guest
// Related: record_parse_option.go -- the needs-linux skip reasons that name this route
// Related: needs_path.go -- repoRootFrom, the checkout a .ci lives in

package runner

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// guestRouteCommand is the host command that runs one .ci in Ze's QEMU guest,
// `le test qemu ci test <path>` (internal/le/test/qemu, onetest.go). It is a
// literal because this package sits below le and cannot import it; testqemu's
// TestRunnerGuestRouteIsTheOneCIAction parses the line GuestRoute answers
// through that action's own parser, so a renamed verb or keyword goes red there.
const guestRouteCommand = "./le test qemu ci test "

// guestRouteTemplate is the route a skip reason names when it cannot name the
// test's own path: a declaration parsed with no .ci behind it (an .et or .wb
// admission, ParseResourceOption), or a .ci outside a checkout's test/ tree.
const guestRouteTemplate = guestRouteCommand + ciRootDir + "/<dir>/<name>.ci"

// ciRootDir is the checkout directory every functional suite's .ci lives under.
const ciRootDir = "test"

// errNotUnderTestTree refuses a route for a .ci the guest suites do not walk.
var errNotUnderTestTree = errors.New("the file is not under the checkout's test/ directory")

// GuestRoute answers the host command that runs the .ci at ciFile in Ze's own
// QEMU guest. The file is named relative to the checkout that holds it, which
// is where `./le` runs, so the reader can paste the line as printed. A file no
// checkout holds, or one outside its test/ tree, has no such command, and the
// error says which.
func GuestRoute(ciFile string) (string, error) {
	root, err := repoRootFrom(ciFile)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(ciFile)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, ciRootDir+"/") {
		return "", errNotUnderTestTree
	}
	return guestRouteCommand + rel, nil
}

// guestRouteAdvice answers the tail of a needs-linux skip reason: where the
// test does run, and the host command that runs it there, last. A test with no
// route of its own is told to sit under test/<dir>/ in a checkout, the only
// place the guest suites walk, with the command as it then reads.
func guestRouteAdvice(ciFile string) string {
	var tb textbuf.Buffer
	tb.Str("run it in Ze's QEMU guest, which runs it as root: ")
	if ciFile == "" {
		return tb.Str(guestRouteTemplate).String()
	}
	route, err := GuestRoute(ciFile)
	if err != nil {
		return tb.Str("place it under test/<dir>/ in a Ze checkout (").Err(err).Str("), then: ").
			Str(guestRouteTemplate).String()
	}
	return tb.Str(route).String()
}
