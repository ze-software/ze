// VALIDATES: the test-only backend-refusal sentinel used by the engine-start
// tests in instance_test.go (TestOpenInterfacesSurvivesOneFailingInterface).
// PREVENTS: nothing on its own -- it exists so the sentinel does not have to be
// added to instance_test.go, which carries an RFC-tagged test (RFC6549-2-1) and
// is therefore change-guarded.

package ospf

import "errors"

// errNoSourceAddress stands in for the real backend refusal that a link with no
// usable source address produces: the ospfv3 transport's ErrNoLinkLocal, wrapped
// with the interface name (internal/plugins/ospf/v3/transport/backend_linux.go,
// interfaceLinkLocal). The engine treats every backend error identically, so a
// v2 fake backend reproduces the v6 failure without the v6 transport.
var errNoSourceAddress = errors.New("test: interface has no usable source address")
