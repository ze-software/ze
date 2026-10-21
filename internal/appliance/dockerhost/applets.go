// Design: docs/architecture/appliance/docker-host.md -- the boot sequence before Lima's boot.sh
// Related: guest_linux.go -- installBinaries, which mounts binDirs and links the applets

package dockerhost

import (
	"path"
	"slices"
	"strings"
)

// binDirs are the read-only squashfs directories the boot covers with a
// tmpfs and fills with links: busybox's applets, the shipped binaries and
// the systemctl shim.
var binDirs = []string{"/bin", "/sbin", "/usr/bin", "/usr/sbin"}

// appletLinks answers the absolute link paths for a `busybox --list-full`
// listing, keeping only the applets whose directory is one of binDirs. Any
// other directory (busybox lists `linuxrc` at the root) is on the read-only
// squashfs, where no link can be made.
func appletLinks(list string) []string {
	var links []string
	for line := range strings.Lines(list) {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		link := "/" + name
		if !slices.Contains(binDirs, path.Dir(link)) {
			continue
		}
		links = append(links, link)
	}
	return links
}
