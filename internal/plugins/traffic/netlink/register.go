package trafficnetlink

import "github.com/ze-software/ze/internal/component/traffic"

// backendName is the name this backend registers under, which the traffic
// control backend leaf selects.
const backendName = "kernel"

func init() {
	if err := traffic.RegisterBackend(backendName, newBackend); err != nil {
		panic("BUG: trafficnetlink: registration failed: " + err.Error())
	}
}
