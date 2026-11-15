// Design: docs/research/l2tpv2-ze-integration.md -- explicit external driver fixtures

package ppp_test

import (
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// testLCPConfig supplies the same defaults as the internal driver fixtures.
func testLCPConfig() ppp.LCPConfig {
	c, err := ppp.NewLCPConfig(3*time.Second, 2, 10, 5)
	if err != nil {
		panic("BUG: fixed valid LCP test settings: " + err.Error())
	}
	return c
}
