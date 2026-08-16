// Design: docs/features/interfaces.md -- DHCP client plugin
// Detail: dhcp_linux.go -- DHCPClient lifecycle and Start/Stop
// Detail: dhcp_v4_linux.go -- DHCPv4 DORA worker
// Detail: dhcp_v6_linux.go -- DHCPv6 SARR worker
// Detail: resolv_linux.go -- DNS resolv.conf writer

// Package ifacedhcp implements DHCPv4/DHCPv6 client functionality as a
// separate plugin. It publishes lease events on the Bus using topic
// constants from the iface component.
package ifacedhcp
