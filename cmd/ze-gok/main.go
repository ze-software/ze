// Design: docs/guide/appliance.md -- gokrazy build tool wrapper

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gokrazy/tools/gok"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

var _ = env.MustRegister(env.EnvEntry{Key: "ze.gok.debug", Type: "bool", Description: "Print ze-gok debug output (resolved GOMODCACHE path)"})

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

	if env.IsEnabled("ze.gok.debug") {
		fmt.Fprintf(os.Stderr, "ze-gok: GOMODCACHE=%s\n", modcache)
	}

	if err := (gok.Context{}).Execute(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "ze-gok: %v\n", err)
		os.Exit(1)
	}
}
