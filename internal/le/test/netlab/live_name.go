// Design: docs/guide/netlab.md -- generated live topology identity.
package testnetlab

import (
	"crypto/rand"
	"errors"
	"os"
	"strings"
)

// A Ze prefix plus fourteen base32 characters fits netlab's sixteen-character
// identifier limit and retains seventy random bits to isolate concurrent runs.
func writeLiveTopologyName(path string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	name := "ze" + strings.ToLower(rand.Text()[:14])
	_, writeErr := file.WriteString("\nname: " + name + "\n")
	return errors.Join(writeErr, file.Close())
}
