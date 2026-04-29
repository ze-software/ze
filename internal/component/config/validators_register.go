// Design: docs/architecture/config/yang-config-design.md — custom validator registration
// Overview: validators.go — custom validator function definitions

package config

import "codeberg.org/thomas-mangin/ze/internal/component/config/yang"

// RegisterValidators registers all custom validators into the given registry.
// Called during startup after YANG modules are loaded.
func RegisterValidators(reg *yang.ValidatorRegistry) {
	reg.Register("registered-address-family", AddressFamilyValidator())
	reg.Register("nonzero-ipv4", NonzeroIPv4Validator())
	reg.Register("literal-self", LiteralSelfValidator())
	reg.Register("community-range", CommunityRangeValidator())
	reg.Register("receive-event-type", ReceiveEventValidator())
	reg.Register("send-message-type", SendMessageValidator())
	reg.Register("mac-address", MACAddressValidator())
	reg.Register("redistribute-source", RedistributeSourceValidator())
	reg.Register("ipv4-address", IPv4AddressValidator())
	reg.Register("ipv6-address", IPv6AddressValidator())
	reg.Register("ipv4-prefix", IPv4PrefixValidator())
	reg.Register("ipv6-prefix", IPv6PrefixValidator())
	reg.Register("set-ref", SetRefValidator())
	reg.Register("port-spec", PortSpecValidator())
}
