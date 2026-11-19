// Design: docs/architecture/storage-backends.md -- stored configuration name admission.
// Related: restore.go -- RestoreConfig commits a config under the name judged here.
// Related: rename_config.go, rollback_config.go -- shared owner transactions.

package storage

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// DefaultConfigName is the config filename a store with no usable instance
// name answers with. InstanceConfigName is the only producer, so the name is
// written once here rather than at each arm that falls back to it.
const DefaultConfigName = "ze.conf"

// validInstanceName matches alphanumeric names with hyphens, max 64 chars.
// Prevents path traversal in blob keys.
var validInstanceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`)

// InstanceConfigName answers the filename of the configuration that is the
// device's own, from the bytes stored under meta/instance/name. It is the one
// rule both readers of that key apply: the launcher that picks the config to
// boot (internal/core/resolve DefaultConfig) and the freeze that scopes itself
// to that config for a tool holding no source binding (freeze.go). The rule
// lives here because resolve imports this package.
//
// A blank name, and a name that is not a valid file stem, answer
// DefaultConfigName. The caller reads the key and MUST decide what a failed
// read means before it calls: a missing key is a blank name, and any other
// read failure is not a name at all. Safe for concurrent use.
func InstanceConfigName(instanceName []byte) string {
	name := strings.TrimSpace(string(instanceName))
	if name == "" {
		return DefaultConfigName
	}
	// Guard: `ze init` stores the host name as the instance name, and a host
	// name such as "edge.example" is not a valid file stem, so such a store
	// names its config "ze.conf". The regex also keeps "../evil" out of a key.
	if !validInstanceName.MatchString(name) {
		return DefaultConfigName
	}
	var tb textbuf.Buffer
	return tb.Str(name).Str(".conf").String()
}

// StoredConfigName validates a configuration filename against an open source
// handle and returns its basename. Raw storage keys, control characters and
// paths outside that store are refused. Safe for concurrent use.
func StoredConfigName(source Storage, name string) (string, error) {
	if isNamespaced(name) {
		return "", fmt.Errorf("stored configuration name %q is a storage key, not a config filename", name)
	}
	if err := source.CheckName(name); err != nil {
		return "", err
	}
	base := filepath.Base(name)
	if !PlainConfigName(base) {
		return "", fmt.Errorf("stored configuration name %q is not a config filename", name)
	}
	for _, character := range base {
		if unicode.IsControl(character) {
			return "", fmt.Errorf("stored configuration name %q contains a control character", name)
		}
	}
	return base, nil
}

// PlainConfigName reports whether name names a config in the store folder
// itself: not a directory step, not a path, and not the stdin marker. Every
// command that names a config by its file name alone judges the name here
// (`ze config import`, and `update data config`, which the local `ze data
// import` sends while a daemon owns the store), so they agree on what a config
// name is.
func PlainConfigName(name string) bool {
	switch name {
	case ".", "..":
		return false
	}
	if strings.ContainsAny(name, "/\\") {
		return false
	}
	return !cliio.IsStdin(name)
}
