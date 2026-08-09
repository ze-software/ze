// Design: docs/architecture/traffic/tc-original-qdisc-restore.md -- tc original-qdisc restore
// Related: backend_linux.go -- tc backend using this operation seam

//go:build linux

package trafficnetlink

import "github.com/vishvananda/netlink"

type tcOps interface {
	linkByName(name string) (netlink.Link, error)
	qdiscList(link netlink.Link) ([]netlink.Qdisc, error)
	qdiscReplace(qdisc netlink.Qdisc) error
	// qdiscDel removes a qdisc. Used only to restore an interface whose original
	// root was `noqueue`: that is the kernel's own representation of "no queueing
	// discipline configured", and deleting the root is how it is re-entered.
	// Adding a qdisc named noqueue is not the inverse operation.
	qdiscDel(qdisc netlink.Qdisc) error
	classList(link netlink.Link, parent uint32) ([]netlink.Class, error)
	classAdd(class netlink.Class) error
	filterList(link netlink.Link, parent uint32) ([]netlink.Filter, error)
	filterAdd(filter netlink.Filter) error
}

type netlinkOps struct{}

func (netlinkOps) linkByName(name string) (netlink.Link, error) { return netlink.LinkByName(name) }
func (netlinkOps) qdiscList(link netlink.Link) ([]netlink.Qdisc, error) {
	return netlink.QdiscList(link)
}
func (netlinkOps) qdiscReplace(qdisc netlink.Qdisc) error { return netlink.QdiscReplace(qdisc) }
func (netlinkOps) qdiscDel(qdisc netlink.Qdisc) error     { return netlink.QdiscDel(qdisc) }
func (netlinkOps) classList(link netlink.Link, parent uint32) ([]netlink.Class, error) {
	return netlink.ClassList(link, parent)
}
func (netlinkOps) classAdd(class netlink.Class) error { return netlink.ClassAdd(class) }
func (netlinkOps) filterList(link netlink.Link, parent uint32) ([]netlink.Filter, error) {
	return netlink.FilterList(link, parent)
}
func (netlinkOps) filterAdd(filter netlink.Filter) error { return netlink.FilterAdd(filter) }
