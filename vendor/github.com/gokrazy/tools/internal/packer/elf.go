package packer

import (
	"debug/elf"
	"fmt"
)

func fileIsELF(filePath string) error {
	f, err := elf.Open(filePath)
	if err != nil {
		return fmt.Errorf("%s is not an ELF binary: %w (perhaps running into https://github.com/golang/go/issues/53804?)", filePath, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close ELF binary %s: %w", filePath, err)
	}
	return nil
}
