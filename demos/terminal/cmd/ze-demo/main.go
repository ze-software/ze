package main

import (
	"os"

	siteterminaldemo "github.com/ze-software/ze/internal/le/site/terminaldemo"
)

func main() {
	os.Exit(siteterminaldemo.RuntimeMain(os.Args[1:], os.Stdout, os.Stderr))
}
