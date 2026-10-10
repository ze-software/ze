// Design: docs/architecture/appliance/gokrazy-build-pins.md -- the GPLv2 notice
// every appliance image carries for the Linux kernel it ships.
// Related: kernelpkg.go -- addLinuxNotice places the notice in the image.
//
// This code runs when an image is assembled, never during a kernel build, so it
// lives here and not in kernelbuilder: every kernelbuilder source is hashed
// into the kernel cache key, and an edit to the notice must not make every
// cached kernel stale.

package instance

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Provenance is the source half of the record Build writes beside a kernel
// (kernelbuilder.ProvenanceName): the kernel.org version, and the tarball URL and SHA-256 the
// worker downloaded and verified. A value comes only from readProvenance, which
// refuses a record that lacks any of the three.
type Provenance struct {
	Version      string
	SourceURL    string
	SourceSHA256 string
}

// readProvenance reads the provenance record kernelbuilder's writeProvenance
// wrote beside a built kernel. A record from
// before the build recorded its source carries no source-url or source-sha256,
// and is refused naming the field: the kernel beside it was built from a source
// this record cannot name, so it is rebuilt rather than shipped.
func readProvenance(path string) (Provenance, error) {
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
	record := Provenance{
		Version:      fields[kernelbuilder.ProvenanceKeyVersion],
		SourceURL:    fields[kernelbuilder.ProvenanceKeySourceURL],
		SourceSHA256: fields[kernelbuilder.ProvenanceKeySourceSHA256],
	}
	for _, required := range []struct{ key, value string }{
		{kernelbuilder.ProvenanceKeyVersion, record.Version},
		{kernelbuilder.ProvenanceKeySourceURL, record.SourceURL},
		{kernelbuilder.ProvenanceKeySourceSHA256, record.SourceSHA256},
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
