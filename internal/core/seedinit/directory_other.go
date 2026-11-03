//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd

// Design: docs/contributing/running-commands.md -- inherited directory admission.
package seedinit

import (
	"fmt"
	"os"
)

func inheritedDirectory(_ string) (*os.Root, *os.File, error) {
	return nil, nil, fmt.Errorf("inherited session directory conversion is unavailable on this host; run le session seed-store on a Linux, Darwin or BSD development host")
}
