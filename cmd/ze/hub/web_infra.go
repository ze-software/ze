// Design: ai/rules/feature-gate-registration.md -- ze_web compile-out seam

package hub

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/config/storage"
)

type webPortalService struct {
	Key   string
	Title string
	Path  string
	Icon  string
}

// webBuildStandalone starts the web-only daemon mode. It is installed from the
// ze_web-gated registration file and stays nil when web is compiled out.
var webBuildStandalone func(store storage.Storage, listenAddr string, insecureWeb bool) int

func setWebStandalone(start func(store storage.Storage, listenAddr string, insecureWeb bool) int) {
	webBuildStandalone = start
}

func webNotCompiledIn() int {
	fmt.Fprintln(os.Stderr, "error: web UI not compiled in (build with ze_web)")
	return 1
}
