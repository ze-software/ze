package pppoeclient

import (
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func init() {
	iface.SetPPPoEDialer(&Dialer{})
	registry.InjectPluginMetrics("l2tp-pppoe-client", bindClientMetrics)
}
