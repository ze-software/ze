// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package rfc

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, leroot.GroupGate, Answer, registry.Meta{
		Description: "RFC conformance: bind every MUST-level requirement of an enrolled RFC " +
			"to the tests that enforce it, and bound what the summaries missed",
		Mode: "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers ze-perf under it for the same reason.
		Section: registry.SectionTest,
		// Derived from the action table, so help cannot disagree with the
		// listing about which action WRITES (actions.go).
		SubsFunc: Subs,
	})

	// Every answer this command can give carries one row set, so the row
	// operators act on it.
	leroot.RegisterShape(area, command.ShapeMap)

	// The census counts these gates as ported from here, in the same init()
	// that registers the command. A claim whose command never registered is
	// red, so the count cannot fall for a tool nothing can reach.

	// The five generated files are DERIVED, so none of them is tracked and no
	// gate compares a re-render against a committed copy. A write to a file the
	// render reads deletes them, and a command that names one rebuilds all five
	// through one IndexUpdate.
	//
	// rfc/requirements is registered as the DIRECTORY it is. The set of shards
	// is derived too: a summary that stops declaring requirements leaves a file
	// the generator no longer owns, and a directory invalidated whole cannot
	// keep one.
	for _, path := range []string{ledgerRel, shardRelDir, enrolledRel, notEnrolledRel, statusRel} {
		derived.Register(derived.Artifact{
			Path:    path,
			Feeds:   feedsRFCLedger,
			Rebuild: rebuildLedger,
		})
	}
}

// feedsRFCLedger reports whether writing path can change what the RFC generator
// renders.
//
// One predicate for all five artifacts, because one render produces all five.
// The population is the walk NewRenderInput performs: the summaries declare
// every requirement and every Meta row, the audit and discrimination
// directories carry the verdicts and the recorded breaks, and a tag carrier
// holds the evidence. Production Go is in the set because RenderInput.Unscanned
// reports an `RFC requirement:` comment that no carrier claims, and such a
// comment can be written in any Go file.
//
// Broad on purpose. A predicate that misses a source leaves an artifact on disk
// that reads as current, and that silent wrong answer is what the byte
// comparison used to catch. The cost of being broad is a rebuild, which is
// loud, bounded, and correct.
// rebuildLedger renders the five files for one checkout, and answers nothing
// for a tree that holds no summaries.
//
// A rebuild runs on the path of an ordinary command, over whatever root that
// command was pointed at, which is not always this session's checkout. A
// scratch tree with no rfc/short/ has no ledger to be absent: IndexUpdate
// REFUSES that tree, correctly, because an explicit run over an empty corpus
// would prune every shard as an orphan -- and as a rebuild that refusal would
// block a grep in a tree the artifacts never lived in.
//
// The same reasoning the materialize hook already applies to the artifact's own
// directory, applied to the generator's input instead.
func rebuildLedger(root string) error {
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(summaryRel))); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	_, err := IndexUpdate(root)
	return err
}

func feedsRFCLedger(_, path string) bool {
	for _, directory := range [...]string{summaryRel, auditRel, discriminationRel} {
		if strings.HasPrefix(path, directory+"/") {
			return true
		}
	}
	return strings.HasSuffix(path, ".go") || IsTagCarrier(path)
}
