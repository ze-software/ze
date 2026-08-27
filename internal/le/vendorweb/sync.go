// Design: docs/architecture/core-design.md -- the vendored-web sync, as a command
//
// sync.go is the writing half. It copies each vendored web asset from
// third_party/web/ into every consumer package that embeds it, because
// //go:embed cannot reach outside its own package. check.go is the read-only
// twin that gates the result.
//
// `make generate` and `make ze-vendor-web-sync` still reach the script this
// replaces: nothing routes here until the swap (plan/spec-le-is-a-ze-binary.md,
// step 14).
//
// Source of truth: third_party/web/. See third_party/web/MANIFEST.md.

package vendorweb

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
)

// consumerCopyMode is the permission a consumer copy is written with. The copies
// are tracked source files a compiler reads, not secrets.
const consumerCopyMode = 0o644

// asset describes one source-to-name mapping.
type asset struct {
	srcDir string // relative to repo root, e.g. "third_party/web/htmx"
	name   string // file name shared between source and consumer
}

var assets = []asset{
	// htmx 4 and its SSE extension. The core keeps the name every page has
	// always loaded; the extension is named as htmx 4 publishes it. htmx 2's
	// core and its separately published sse.js are gone from the tree:
	// see third_party/web/MANIFEST.md.
	{"third_party/web/htmx", "htmx.min.js"},
	{"third_party/web/htmx", "hx-sse.min.js"},
	{"third_party/web/ze", "ze.svg"},
	{"third_party/web/uplot", "uPlot.min.js"},
	{"third_party/web/uplot", "uPlot.min.css"},
}

var consumers = []string{
	"internal/chaos/web/assets",
	"internal/component/lg/assets",
	"internal/component/web/assets",
}

// targetedAsset is synced to a single consumer instead of every consumer.
type targetedAsset struct {
	srcDir string
	name   string
	dest   string
}

var targetedAssets = []targetedAsset{
	{"third_party/web/swagger-ui", "swagger-ui.css", "internal/component/api/rest/assets"},
	{"third_party/web/swagger-ui", "swagger-ui-bundle.js", "internal/component/api/rest/assets"},
}

// Sync copies every vendored asset into every consumer that embeds it and
// answers what it wrote.
//
// A missing consumer directory and a missing source file are warnings the run
// carries on past, which is what lets one absent package leave the rest of the
// tree synced. A run that could read NO source is a different fact and is an
// error: see the fail-closed guard at the end.
func Sync(root string) (SyncReport, error) {
	report := SyncReport{Files: []SyncedFile{}}

	for _, dest := range consumers {
		destDir := filepath.Join(root, dest)

		// One guard per fact. "It is not there" and "it is there and is not a
		// directory" are two different things a reader should not have to hold
		// at once (docs/contributing/ze-go-style.md).
		info, err := os.Stat(destDir)
		if err != nil {
			report.warnMissingConsumer(destDir)
			continue
		}
		if !info.IsDir() {
			report.warnMissingConsumer(destDir)
			continue
		}

		for _, a := range assets {
			src := filepath.Join(root, a.srcDir, a.name)
			dst := filepath.Join(destDir, a.name)

			if err := report.copy(src, dst); err != nil {
				return report, err
			}
		}
	}

	for _, a := range targetedAssets {
		src := filepath.Join(root, a.srcDir, a.name)
		dst := filepath.Join(root, a.dest, a.name)

		if err := report.copy(src, dst); err != nil {
			return report, err
		}
	}

	// FAIL CLOSED. A run that read no source wrote nothing because it found
	// nothing, which is not the tree being up to date. The check half of this
	// pair has had this guard since 2026-08-15 and the sync half never did, so
	// a sync pointed at a tree with no third_party/web/ reported success.
	if report.Readable == 0 {
		return report, fmt.Errorf("no vendored asset was readable under %s/, so nothing could be synced", vendorDir)
	}

	return report, nil
}

// copy writes one consumer copy when it differs from its source, and records
// what it did. A source it cannot read is a warning; a destination it cannot
// write is an error, because the run was told to write it.
func (r *SyncReport) copy(src, dst string) error {
	srcData, err := os.ReadFile(src) //nolint:gosec // the path is built from this package's tables under the checkout root
	if err != nil {
		// A source this tree does not carry is a WARNING the run continues
		// past, so one absent package leaves the rest of the tree synced. The
		// fail-closed guard in Sync is what stops that reading being applied to
		// a tree that carries none of them.
		r.warnMissingSource(src)
		return nil //nolint:nilerr // a missing source is a warning by design; Sync refuses a run that read none
	}

	r.Readable++

	dstData, err := os.ReadFile(dst) //nolint:gosec // the path is built from this package's tables under the checkout root
	if err == nil && bytes.Equal(srcData, dstData) {
		return nil
	}

	if err := os.WriteFile(dst, srcData, consumerCopyMode); err != nil { //nolint:gosec // a tracked source file the compiler reads, not a secret
		return fmt.Errorf("write %s: %w", dst, err)
	}

	r.Files = append(r.Files, SyncedFile{Kind: SyncWritten, Path: dst})
	r.Changed++

	return nil
}

// warnMissingConsumer records a consumer directory the run could not write into.
func (r *SyncReport) warnMissingConsumer(dir string) {
	var tb textbuf.Buffer
	r.warn(dir, tb.Str("consumer directory not found, skipping: ").Str(dir).String())
}

// warnMissingSource records a vendored file the run could not read.
func (r *SyncReport) warnMissingSource(src string) {
	var tb textbuf.Buffer
	r.warn(src, tb.Str("vendor file not found: ").Str(src).String())
}

// warn records one line the run writes to stderr and carries on past.
func (r *SyncReport) warn(path, message string) {
	r.Files = append(r.Files, SyncedFile{Kind: SyncWarned, Path: path, Message: message})
}

// runSync is the `sync` action. It WRITES: every consumer copy that differs
// from its third_party/web/ source is overwritten from it.
func runSync() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}

	report, err := Sync(root)

	// The warnings go to stderr, where the script put them, and they go there
	// whether or not the run failed: a warning is why a later failure happened.
	for _, warning := range report.Warnings() {
		var tb textbuf.Buffer
		fmt.Fprintln(os.Stderr, tb.Str("warning: ").Str(warning).String()) //nolint:errcheck // CLI output
	}

	if err != nil {
		reportError(err)
		return report, 1
	}

	return report, 0
}
