// Design: docs/architecture/api/process-protocol.md -- Filter Declaration (Stage 1)
// Related: external_filters.go -- the judge registered here

package bgpconfig

import "github.com/ze-software/ze/internal/component/plugin/registry"

// BGP runs the policy filters, so it judges what a plugin declares it reads
// and sets at Stage 1; the plugin server cannot import BGP.
func init() {
	registry.SetFilterDeclarationCheck(checkFilterDeclarations)
}
