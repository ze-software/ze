// Design: plan/learned/1024-installer-initrd-pure-go.md -- PID-1 installer initrd binary

//go:build linux && ze_installer

package main

import "github.com/ze-software/ze/internal/install/disk"

func main() {
	disk.RunInitrd()
}
