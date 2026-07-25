// VALIDATES: RFC 2328 Sections 16.2 / 16.4 -- the routing calculation skips a summary-LSA or
// AS-external-LSA that is MaxAge or self-originated, not only one whose metric is LSInfinity
// (which TestOSPFInterAreaLSInfinityDropped / TestOSPFExternalLSInfinityDropped already cover).
// PREVENTS: a router routing through its own advertisement (a loop) or through an LSA the
// domain is in the middle of flushing.
package spf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc2328NextHop is the reachable next-hop the ABR results below carry.
var rfc2328NextHop = netip.MustParseAddr("10.0.0.2")

// RFC requirement: RFC2328-16.2-2 negative -- a MaxAge summary-LSA is skipped by the inter-area
// calculation, and a summary-LSA advertised by the calculating router itself is skipped, so
// neither installs a route (v4SummaryReader age filter, interarea.go:172-174; self filter,
// ComputeInterAreaWith interarea.go:135-137).
func TestRFC2328InterAreaSkipsMaxAgeAndSelfSummary(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	abr := testRID(t, "2.2.2.2")
	backbone := types.BackboneArea

	maxAged := summaryNetworkLSA(t, "10.50.0.0", "2.2.2.2", 5)
	maxAged.Header.Age = types.LSAge(types.MaxAge)
	src := testSource(t, backbone, maxAged)
	res := map[types.AreaID]*Result{backbone: resultWithRouter(backbone, root, abr, 10, rfc2328NextHop, 0)}
	routes, _ := ComputeInterArea(InterAreaInput{Source: src, Root: root, Areas: []types.AreaID{backbone}, Results: res, MaxPaths: 8})
	assert.Empty(t, routes, "a MaxAge summary-LSA must not enter the routing calculation")

	// Self-originated: the advertising router is the calculating router.
	selfSrc := testSource(t, backbone, summaryNetworkLSA(t, "10.51.0.0", "1.1.1.1", 5))
	selfRes := map[types.AreaID]*Result{backbone: resultWithRouter(backbone, root, root, 0, rfc2328NextHop, 0)}
	selfRoutes, _ := ComputeInterArea(InterAreaInput{Source: selfSrc, Root: root, Areas: []types.AreaID{backbone}, Results: selfRes, MaxPaths: 8})
	assert.Empty(t, selfRoutes, "a self-originated summary-LSA must not install a route back to itself")
}

// RFC requirement: RFC2328-16.2-2 negative -- a MaxAge AS-external-LSA and a self-originated
// AS-external-LSA are both skipped by the AS-external calculation, so neither installs a route
// (ComputeExternalWith age/self filter, external.go:96).
func TestRFC2328ExternalSkipsMaxAgeAndSelf(t *testing.T) {
	root := testRID(t, "1.1.1.1")

	maxAged := externalLSA(t, "10.52.0.0", "2.2.2.2", false, 5, "0.0.0.0")
	maxAged.Header.Age = types.LSAge(types.MaxAge)
	src := testSource(t, types.BackboneArea, maxAged)
	border := []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 10, "10.0.0.2")}
	assert.Empty(t, ComputeExternal(ExternalInput{Source: src, Root: root, BorderRouters: border, MaxPaths: 8}),
		"a MaxAge AS-external-LSA must not enter the routing calculation")

	selfSrc := testSource(t, types.BackboneArea, externalLSA(t, "10.53.0.0", "1.1.1.1", false, 5, "0.0.0.0"))
	selfBorder := []BorderRouterEntry{asbrBorder(t, "1.1.1.1", 10, "10.0.0.2")}
	assert.Empty(t, ComputeExternal(ExternalInput{Source: selfSrc, Root: root, BorderRouters: selfBorder, MaxPaths: 8}),
		"a self-originated AS-external-LSA must not install a route back to itself")
}
