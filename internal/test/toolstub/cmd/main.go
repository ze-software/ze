package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/test/toolstub"
)

func main() {
	name := filepath.Base(os.Args[0])
	code, ok := toolstub.Run(name, os.Args[1:])
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown test tool %q\n", name)
		os.Exit(2)
	}
	os.Exit(code)
}
