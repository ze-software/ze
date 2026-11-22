package trafficnetlink

import "github.com/ze-software/ze/internal/component/traffic"

func init() {
	if err := traffic.RegisterBackend("tc", newBackend); err != nil {
		panic("BUG: trafficnetlink: registration failed: " + err.Error())
	}
}
