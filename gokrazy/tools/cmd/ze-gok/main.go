// Design: docs/guide/appliance.md -- gokrazy build tool wrapper

// ze-gok wraps the gokrazy gok tool with a repo-local GOMODCACHE.
//
// gok hardcodes -mod=mod in all go build/list commands, so it always
// resolves packages from the module cache. By setting GOMODCACHE to
// gokrazy/modcache/ (relative to the repo root), we redirect all
// resolution to a directory we control:
//
//   - Go source (gokrazy init, dhcp, ntp, etc.) is committed to git
//   - Linux kernel binary is fetched once and .gitignored
//   - No network access after initial setup
//
// This avoids patching any gokrazy code.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gokrazy/tools/gok"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ze-gok: %v\n", err)
		os.Exit(1)
	}
	modcache := filepath.Join(wd, "gokrazy", "modcache")
	if err := os.MkdirAll(modcache, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "ze-gok: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv("GOMODCACHE", modcache); err != nil {
		fmt.Fprintf(os.Stderr, "ze-gok: setenv: %v\n", err)
		os.Exit(1)
	}

	if os.Getenv("ZE_GOK_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "ze-gok: GOMODCACHE=%s\n", modcache)
	}

	if err := (gok.Context{}).Execute(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "ze-gok: %v\n", err)
		os.Exit(1)
	}
}
