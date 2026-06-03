package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-firewall-conf.yang", ZeFirewallConfYANG)
	yang.RegisterModule("ze-firewall-cmd.yang", ZeFirewallCmdYANG)
}
