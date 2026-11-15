// Design: docs/research/l2tpv2-ze-integration.md -- explicit historical test settings

package ppp

import "time"

// testLCPConfig supplies the settings existing fixtures used before they became
// mandatory inputs. Configured-behavior tests construct their own values.
func testLCPConfig() LCPConfig {
	c, err := NewLCPConfig(3*time.Second, 2, 10, 5)
	if err != nil {
		panic("BUG: fixed valid LCP test settings: " + err.Error())
	}
	return c
}
