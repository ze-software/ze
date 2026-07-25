package pppoeclient

import "github.com/ze-software/ze/internal/component/iface"

func init() {
	iface.SetPPPoEDialer(&Dialer{})
}
