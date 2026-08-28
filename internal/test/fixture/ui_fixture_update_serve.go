package fixture

import (
	"fmt"
	"os"
	"syscall"
)

func init() {
	Register("ui/update-serve", uiDriver(runUIUpdateServe))
}

func runUIUpdateServe(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("update-serve fixture requires the listener port")
	}
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return fmt.Errorf("ZE_REPO_ROOT is not set")
	}
	ze, err := uiZEBinary(root)
	if err != nil {
		return err
	}
	port := args[0]

	argv := []string{
		"ze",
		"update",
		"serve",
		"--listen",
		"127.0.0.1:" + port,
	}
	if err := syscall.Exec(ze, argv, os.Environ()); err != nil {
		return fmt.Errorf("exec ze update serve: %w", err)
	}
	return nil
}
