// Design: docs/guide/pppoe.md -- explicit historical test settings
//go:build linux

package pppoe

import (
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// testLCPConfig supplies the unchanged LCP defaults for direct transport fixtures.
func testLCPConfig() ppp.LCPConfig {
	c, err := ppp.NewLCPConfig(3*time.Second, 2, 10, 5)
	if err != nil {
		panic("BUG: fixed valid LCP test settings: " + err.Error())
	}
	return c
}
