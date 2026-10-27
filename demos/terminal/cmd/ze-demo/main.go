package main

import (
	"os"

	_ "github.com/ze-software/ze/internal/component/plugin/all" // Configuration checks use the product's schema registrations.
	siteterminaldemo "github.com/ze-software/ze/internal/le/site/terminaldemo"
)

func main() {
	os.Exit(siteterminaldemo.RuntimeMain(os.Args[1:], os.Stdout, os.Stderr))
}
