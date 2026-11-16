// Design: docs/architecture/l2tp/bng-5-pppoe.md -- non-Linux discovery refusal

//go:build !linux

package pppoe

func openDiscoverySocketInNamespace(string) (int, error) { return -1, errNotLinux }
