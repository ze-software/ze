// Design: plan/spec-fw-6-firewall-vpp.md -- VPP-operation seam for unit tests

//go:build linux

package firewallvpp

import (
	"go.fd.io/govpp/binapi/acl"
	"go.fd.io/govpp/binapi/acl_types"
	"go.fd.io/govpp/binapi/interface_types"
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
