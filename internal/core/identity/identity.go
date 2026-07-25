// Design: docs/architecture/core-design.md -- stable machine identity via zefs

package identity

import (
	crand "crypto/rand"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/ze-software/ze/pkg/zefs"
)

const identityUnknown = "unknown"

// Storage is the subset of config/storage.Storage needed by this package.
// Defined here to avoid importing the full storage package from core.
type Storage interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

// Resolve returns a stable machine identity. Resolution order:
//  1. zefs key meta/instance/machine-id (persisted in blob store)
//  2. /etc/machine-id (systemd/gokrazy)
//  3. hostname
//  4. crypto/rand (generated and persisted to zefs)
//
// When the identity is resolved from a filesystem source (steps 2-3),
// it is written back to zefs so subsequent calls return it directly.
// If store is nil, steps 1 and write-back are skipped.
func Resolve(store Storage) string {
	if store != nil {
		if data, err := store.ReadFile(zefs.KeyMachineID.Pattern); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				return id
			}
		}
	}

	if id := readMachineIDFile("/etc/machine-id"); id != "" {
		persist(store, id)
		return id
	}

	if h, err := os.Hostname(); err == nil && h != "" {
		persist(store, h)
		return h
	}

	var buf [16]byte
	if _, err := io.ReadFull(crand.Reader, buf[:]); err == nil {
		id := hex.EncodeToString(buf[:])
		persist(store, id)
		return id
	}
	return identityUnknown
}

var readFile = os.ReadFile

func readMachineIDFile(path string) string {
	data, err := readFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func persist(store Storage, id string) {
	if store == nil {
		return
	}
	_ = store.WriteFile(zefs.KeyMachineID.Pattern, []byte(id), 0o600)
}
