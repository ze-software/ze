// Design: docs/architecture/appliance/gokrazy-build-pins.md -- the GPLv2 notice
// every appliance image carries for the Linux kernel it ships.
// Related: driver.go -- writeProvenance writes the record this file reads back.

package kernelbuilder

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Provenance is the source half of the record Build writes beside a kernel
// (ProvenanceName): the kernel.org version, and the tarball URL and SHA-256 the
// worker downloaded and verified. A value comes only from ReadProvenance, which
// refuses a record that lacks any of the three.
type Provenance struct {
	Version      string
	SourceURL    string
	SourceSHA256 string
}

// ReadProvenance reads a provenance record writeProvenance wrote. A record from
// before the build recorded its source carries no source-url or source-sha256,
// and is refused naming the field: the kernel beside it was built from a source
// this record cannot name, so it is rebuilt rather than shipped.
func ReadProvenance(path string) (Provenance, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: the provenance beside a resolved kernel tree
	if err != nil {
		return Provenance{}, fmt.Errorf("read kernel provenance: %w", err)
	}
	fields := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), "=")
		if found {
			fields[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return Provenance{}, fmt.Errorf("read kernel provenance %s: %w", path, err)
	}
	record := Provenance{Version: fields["version"], SourceURL: fields["source-url"], SourceSHA256: fields["source-sha256"]}
	for _, required := range []struct{ key, value string }{
		{"version", record.Version},
		{"source-url", record.SourceURL},
		{"source-sha256", record.SourceSHA256},
	} {
		if required.value == "" {
			return Provenance{}, errProvenanceField(path, required.key)
		}
	}
	return record, nil
}

func errProvenanceField(path, key string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str("kernel provenance ").Str(path).Str(" has no ").Str(key).
		Str(": the kernel was built before the build recorded its source; rebuild it").String())
}

// LinuxNotice is the GPLv2 notice an appliance image carries for its kernel.
// Ze complies by pointing at the published Linux source, so the notice names
// the exact kernel.org version, the tarball URL and its SHA-256, and where the
// configuration, patches and build scripts live.
func (p Provenance) LinuxNotice() string {
	var tb textbuf.Buffer
	return tb.Str("This image contains the Linux kernel, which is licensed under the\n").
		Str("GNU General Public License version 2 (GPLv2).\n\n").
		Str("Version: ").Str(p.Version).Str("\n").
		Str("Source: ").Str(p.SourceURL).Str("\n").
		Str("SHA-256: ").Str(p.SourceSHA256).Str("\n\n").
		Str("The kernel configuration (gokrazy/kernel/), the patches applied to the\n").
		Str("source (gokrazy/kernel/patches/) and the build scripts\n").
		Str("(tools/kernel-builder/) are published at https://github.com/ze-software/ze.\n").
		String()
}
