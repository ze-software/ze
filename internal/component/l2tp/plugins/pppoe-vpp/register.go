// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP session composition
// Related: ../../pppoe/dataplane/vpp/factory.go -- VPP session constructor

package pppoevpp

import (
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	vppdataplane "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp"
)

func init() {
	if err := dataplane.RegisterSessionDataplane(vppdataplane.Name, vppdataplane.Prepare); err != nil {
		panic("BUG: cannot register VPP PPPoE session backend: " + err.Error())
	}
	registerStartup()
}
