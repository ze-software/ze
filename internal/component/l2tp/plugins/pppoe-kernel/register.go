// Design: docs/architecture/l2tp/bng-5-pppoe.md -- kernel session composition
// Related: ../../pppoe/dataplane/kernel/factory.go -- native session constructor

package pppoekernel

import (
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/kernel"
)

func init() {
	if err := dataplane.RegisterSessionDataplane(kernel.Name, kernel.Prepare); err != nil {
		panic("BUG: cannot register netlink PPPoE session backend: " + err.Error())
	}
}
