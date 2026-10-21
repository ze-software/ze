// Design: docs/architecture/appliance/docker-host.md -- the docker-host image's guest program

//go:build linux

// Command ze-docker-host is the program gokrazy starts in Ze's docker-host
// image. Started as systemctl (a link to it), it is the shim Lima's and
// Colima's scripts call; otherwise it boots the guest and supervises it.
// It is a target binary, built by gok for the image's GOARCH.
package main

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/appliance/dockerhost"
)

func main() {
	if filepath.Base(os.Args[0]) == "systemctl" {
		os.Exit(dockerhost.RunShim(dockerhost.ShimSocket, os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(dockerhost.RunGuest())
}
