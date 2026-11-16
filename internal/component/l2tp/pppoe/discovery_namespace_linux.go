// Design: docs/architecture/l2tp/bng-5-pppoe.md -- Linux discovery namespace
// Related: discovery_namespace.go -- socket transfer and namespace rollback

//go:build linux

package pppoe

import (
	"runtime"

	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// openDiscoverySocketInNamespace creates the socket where the prepared Linux
// control device lives. The caller MUST close it after all discovery and PPP
// event consumers have joined. It never changes the caller's thread namespace.
func openDiscoverySocketInNamespace(name string) (int, error) {
	return openDiscoveryNamespace(name, linuxDiscoveryNamespaceOperations{})
}

type linuxDiscoveryNamespaceOperations struct{}

func (linuxDiscoveryNamespaceOperations) LockThread()   { runtime.LockOSThread() }
func (linuxDiscoveryNamespaceOperations) UnlockThread() { runtime.UnlockOSThread() }
func (linuxDiscoveryNamespaceOperations) Current() (discoveryNamespace, error) {
	handle, err := netns.Get()
	if err != nil {
		return nil, err
	}
	return linuxDiscoveryNamespace{handle: handle}, nil
}
func (linuxDiscoveryNamespaceOperations) Named(name string) (discoveryNamespace, error) {
	handle, err := netns.GetFromName(name)
	if err != nil {
		return nil, err
	}
	return linuxDiscoveryNamespace{handle: handle}, nil
}
func (linuxDiscoveryNamespaceOperations) OpenSocket() (int, error) { return openDiscoverySocket() }
func (linuxDiscoveryNamespaceOperations) CloseSocket(fd int) error { return unix.Close(fd) }

type linuxDiscoveryNamespace struct{ handle netns.NsHandle }

func (n linuxDiscoveryNamespace) Enter() error { return netns.Set(n.handle) }
func (n linuxDiscoveryNamespace) Close() error { return n.handle.Close() }
