package pppoeclient

import "codeberg.org/thomas-mangin/ze/internal/component/iface"

func init() {
	iface.SetPPPoEDialer(&Dialer{})
}
