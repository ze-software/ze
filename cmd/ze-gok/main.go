// Design: docs/guide/appliance.md -- gokrazy build tool wrapper

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gokrazy/tools/gok"
)

func main() {
	modcache := os.Getenv("GOMODCACHE")
	if modcache == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ze-gok: %v\n", err)
			os.Exit(1)
		}
		modcache = filepath.Join(wd, "gokrazy", "modcache")
	}
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
