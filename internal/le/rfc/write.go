// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Overview: render.go -- the two pages this driver emits
//
// write.go is `ze-rfc-index-update`. It emits ai/RFC-REQUIREMENTS.md and one
// file per RFC under rfc/requirements/, then DELETES every shard the render no
// longer produces.
//
// The delete is why this driver refuses more than the read-only ones do. An
// incomplete collection is a destructive input, not a partial one: a summary
// that failed to parse renders nothing, drops out of the rendered set, and its
// RFC's tracked file becomes an orphan the prune removes while the run exits 0.
package rfc

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// stemRE is what a name has to look like to BE a summary stem.
//
// It bounds what the prune may delete. A summary stem is lower case, so an
// authored README.md beside a generated tree is safe by the same rule.
var stemRE = regexp.MustCompile(`\A[a-z0-9][a-z0-9._-]*\z`)

// PrunableShards answers every stem under rfc/requirements/ the write would
// delete, given the rendered set `keep`.
//
// The ONE producer of "what the generator owns and no longer wants". PruneShards
// deletes it and the freshness check reports it as an orphan, so the gate can
// never name a file the write would leave, nor stay silent about one the write
// would remove.
//
// Four limits, and each one is a deletion this must never make: only `.md`, so
// a `.gitkeep` beside the files survives; only a name whose stem could BE a
// summary stem, which is what keeps `README.md` out of the candidate set; only
// entries directly in the directory, so a subdirectory is untouched; and only a
// stem the caller did not name, so a file this run has just written is never a
// candidate.
//
// The stem test bounds the claim rather than completing it: nothing enforces the
// lower-case convention, so a summary authored as `RFC7606.md` would render a
// file this never names, neither pruned nor reported as an orphan. It is still
// covered by the stale and missing branches, which iterate the rendered stems
// rather than the directory. Left unguarded on purpose: the failure direction is
// NOT-deleting.
func PrunableShards(tree string, keep map[string]bool) ([]string, error) {
	dir := treePath(tree, shardRelDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(shardRelDir).Str(": cannot read: ").Err(err))
	}
	var stems []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		stem := strings.TrimSuffix(name, ".md")
		if keep[stem] || !stemRE.MatchString(stem) || entry.IsDir() {
			continue
		}
		stems = append(stems, stem)
	}
	// Sorted, so both callers speak about the files in one order.
	sort.Strings(stems)
	return stems, nil
}

// PruneShards deletes every shard PrunableShards names, and answers the deleted
// stems.
//
// A shard the render no longer produces is a page about an RFC that no longer
// declares anything, and it reads as current for as long as it sits there. The
// write owns this directory, so the write removes it.
func PruneShards(tree string, keep map[string]bool) ([]string, error) {
	removed, err := PrunableShards(tree, keep)
	if err != nil {
		return nil, err
	}
	for _, stem := range removed {
		if err := os.Remove(treePath(tree, shardRel(stem))); err != nil {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(shardRel(stem)).Str(": cannot delete: ").Err(err))
		}
	}
	return removed, nil
}

// IndexReport is what one run of the generator wrote and removed.
type IndexReport struct {
	Ledger  string   `json:"ledger"`
	Shards  int      `json:"shards"`
	Deleted []string `json:"deleted"`
	// Files is every ledger file this run wrote beside the index and the
	// shards: the enrolled set, the declared remainder and the public
	// support page, each derived from the summaries.
	Files []string `json:"files,omitempty"`
}

// Text is the two lines the generator printed. It is the DEFAULT rendering:
// `| json` answers the same facts as data.
func (r IndexReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("wrote ").Str(r.Ledger).Str(" and ").Int(int64(r.Shards)).
		Str(" shard(s) under ").Str(shardRelDir).Byte('\n')
	if len(r.Files) > 0 {
		tb.Str("wrote ").Join(r.Files, ", ").Byte('\n')
	}
	if len(r.Deleted) > 0 {
		tb.Str("deleted orphan shard(s): ").Join(r.Deleted, ", ").Byte('\n')
	}
	return tb.String()
}

// IndexUpdate renders both pages and writes them.
//
// It REFUSES before anything is written or removed in the two states where the
// collection is incomplete, because a guard over a destructive operation fails
// closed. An absent rfc/short/ is the same failure at full scale: every stem
// vanishes and every file in the directory becomes an orphan.
func IndexUpdate(tree string) (IndexReport, error) {
	collected, err := Collect(tree)
	if err != nil {
		return IndexReport{}, err
	}
	in, err := NewRenderInput(tree, collected, nil, nil)
	if err != nil {
		return IndexReport{}, err
	}
	index, err := RenderIndex(in)
	if err != nil {
		return IndexReport{}, err
	}
	shards := RenderShards(in)

	if len(collected.ParseErrors) > 0 {
		return IndexReport{}, refuseToWrite(collected.ParseErrors,
			"a summary did not parse, so its requirements are absent from this render "+
				"and the prune would delete that RFC's file. Fix the summary, then re-run")
	}
	if len(shards) == 0 {
		var tb textbuf.Buffer
		return IndexReport{}, refuseToWrite(nil, tb.
			Str("the render produced no requirement rows for any RFC, so every file in ").
			Str(shardRelDir).Str(" would be deleted as an orphan. Check that ").
			Str(summaryRel).Str(" is present").String())
	}

	ledgers, err := LedgerFiles(in.Metas, CoverageRows(in.Requirements, in.Tags, in.Carriers))
	if err != nil {
		return IndexReport{}, err
	}
	if err := writePage(tree, ledgerRel, index); err != nil {
		return IndexReport{}, err
	}
	for _, rel := range LedgerPaths() {
		if err := writeExact(tree, rel, ledgers[rel]); err != nil {
			return IndexReport{}, err
		}
	}
	for _, stem := range sortedKeysOf(shards) {
		if err := writePage(tree, shardRel(stem), shards[stem]); err != nil {
			return IndexReport{}, err
		}
	}
	// After the write, never before: a crash between the two would otherwise
	// leave the directory holding neither the old shard nor the new one.
	keep := map[string]bool{}
	for stem := range shards {
		keep[stem] = true
	}
	removed, err := PruneShards(tree, keep)
	if err != nil {
		return IndexReport{}, err
	}
	return IndexReport{Ledger: ledgerRel, Shards: len(shards), Deleted: removed,
		Files: LedgerPaths()}, nil
}

// refuseToWrite is the destructive-input refusal, with every parse error above
// it so the operator sees which summary to fix rather than only that one did.
func refuseToWrite(errs []string, why string) error {
	var tb textbuf.Buffer
	for _, one := range errs {
		tb.Str("* ").Str(one).Byte('\n')
	}
	return parseErr(tb.Str("rfc-requirements: refusing to write: ").Str(why))
}

// writePage writes one generated page, terminated by the newline both the
// generator and the freshness comparison expect.
func writePage(tree, rel, body string) error {
	path := treePath(tree, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": cannot create directory: ").Err(err))
	}
	var page textbuf.Buffer
	if err := os.WriteFile(path, []byte(page.Str(body).Byte('\n').String()), 0o644); err != nil { //nolint:gosec // a generated page, world-readable by design
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": cannot write: ").Err(err))
	}
	return nil
}

// writeExact writes one generated file with no terminator of its own.
//
// Separate from writePage, which appends the newline ai/RFC-REQUIREMENTS.md and
// the shards are compared with. The three ledger files carry their own trailing
// newline where they end in a row, and the public page ends in an HTML comment
// that must not gain a blank line: what the generator writes and what the
// freshness check compares have to be one string, so the terminator is part of
// the render rather than part of the write.
func writeExact(tree, rel, body string) error {
	path := treePath(tree, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": cannot create directory: ").Err(err))
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil { //nolint:gosec // a generated page, world-readable by design
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": cannot write: ").Err(err))
	}
	return nil
}
