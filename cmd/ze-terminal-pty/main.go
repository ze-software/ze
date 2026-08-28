package main

import (
	"os"

	"github.com/ze-software/ze/internal/le/terminaldemo"
)

func main() {
	os.Exit(terminaldemo.RunPTY(os.Args[1:], os.Stdout, os.Stderr))
}
