//go:build darwin || freebsd || openbsd || netbsd

// Design: docs/contributing/running-commands.md -- inherited directory admission.
package seedinit

// XNU fdesc_open and dupfdopen duplicate the descriptor, not its former name.
// Go OpenRoot checks the duplicated descriptor with fstat before using openat.
const inheritedDirectoryPath = "/dev/fd/4"
