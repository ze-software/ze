// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- VPP-operation seam for unit tests

//go:build linux

package firewallvpp

import (
	"go.fd.io/govpp/binapi/acl"
	"go.fd.io/govpp/binapi/acl_types"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_types"
)

// vppOps is the narrow VPP-call surface that firewallvpp's Apply path
// depends on. Extracted as an interface so unit tests can substitute a
// scripted fake without a running VPP daemon. The production path uses
// the govppOps adapter in backend_linux.go.
type vppOps interface {
	dumpInterfaces() (map[string]interface_types.InterfaceIndex, error)
	aclAddReplace(req *acl.ACLAddReplace) (uint32, error)
	aclDel(aclIndex uint32) error
	aclDump() ([]aclDumpEntry, error)
	aclInterfaceListDump(swIfIndex interface_types.InterfaceIndex) (ifaceACLList, error)
	aclInterfaceSetACLList(swIfIndex interface_types.InterfaceIndex, nInput uint8, acls []uint32) error

	nat44Enable() error
	nat44AddDelAddressRange(first, last ip_types.IP4Address, isAdd bool) error
	nat44AddDelStaticMapping(m natStaticMapping) error
	nat44AddDelOutputInterface(swIfIndex interface_types.InterfaceIndex, isAdd bool) error
	nat44AddDelInterfaceFeature(swIfIndex interface_types.InterfaceIndex, isInside bool, isAdd bool) error
	nat44StaticMappingDump() ([]natStaticMapping, error)

	classifyAddDelTable(tableIdx uint32, mask []byte, isAdd bool) (uint32, error)
	classifyAddDelSession(tableIdx uint32, match []byte, opaqueIndex uint32, isAdd bool) error
	classifySetInterfaceIPTable(swIfIndex interface_types.InterfaceIndex, tableIdx uint32, isAdd bool) error
	policerClassifySetInterface(swIfIndex interface_types.InterfaceIndex, tableIdx uint32, isAdd bool) error
	policerAddDel(name string, cir uint32, burst uint32, isPackets bool, isAdd bool) (uint32, error)
}

// ifaceACLList holds the current ACL binding for one interface as
// returned by ACLInterfaceListDump.
type ifaceACLList struct {
	nInput uint8
	acls   []uint32
}

// aclDumpEntry holds one ACL's metadata from a dump.
type aclDumpEntry struct {
	Index uint32
	Tag   string
	Rules []acl_types.ACLRule
}

// natStaticMapping holds one DNAT static mapping for add/del/dump.
type natStaticMapping struct {
	IsAdd             bool
	Tag               string
	Protocol          uint8
	LocalAddr         ip_types.IP4Address
	LocalPort         uint16
	ExternalAddr      ip_types.IP4Address
	ExternalPort      uint16
	ExternalSwIfIndex interface_types.InterfaceIndex
}
