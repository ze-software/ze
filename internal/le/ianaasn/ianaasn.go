// Design: docs/architecture/resolve.md -- the shipped RIR delegation seed
//
// Package ianaasn fetches the five RIR delegation files and rewrites
// internal/component/resolve/irr/rir-delegation.txt, the data file the irr
// package embeds and every ASN-to-registry lookup reads.
//
// It owns the FILE WRITE, and nothing else. The whole recipe above it belongs
// to the irr package: irr.FetchDelegationTable reads the five files, parses
// each one, sorts, collapses and dates the result, and irr.RenderDelegationTable
// writes the bytes the irr parser reads back. `update resolve rir` calls the
// same function, so a guard cannot live on one path and not the other. That is
// exactly what happened while this package held a parser of its own: each copy
// carried a check the other lacked.
//
// It is the one generator here whose input is the NETWORK rather than the tree,
// which is why it has no check twin: there is nothing to compare a checkout
// against without asking five registries what they publish today. The fetch is
// a parameter so a test names its own answers.
//
// TWO THINGS FAIL CLOSED, and irr.FetchDelegationTable owns the first. A run
// that parses no ASN record writes nothing: otherwise five HTTP 200 responses
// carrying no ASN records could commit an empty table, and every lookup would
// then answer that nobody holds any AS number. The second is this package's:
// the table is built in memory and written in one call, so an interrupted
// write cannot leave the irr package holding half a table.
package ianaasn

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/resolve/irr"
)

// outputFile is the delegation table this generator writes, relative to the
// tree. The irr package embeds that path, so a run rewrites what the next
// build ships.
const outputFile = "internal/component/resolve/irr/rir-delegation.txt"

// Write fetches every registry's delegation file and rewrites the table.
//
// The fetch, the parse, the sort, the collapse and the date are
// irr.FetchDelegationTable, which `update resolve rir` also refreshes through.
// This function adds the one thing a generator does and a daemon does not: it
// writes the answer to the file the irr package embeds.
//
// It writes NOTHING unless all five registries answered and the parse took at
// least one ASN record. That guard is irr.FetchDelegationTable's, and it holds
// for the same reason on both paths: an empty table is not a smaller answer,
// it is every AS number becoming unanswerable, and no test of the irr package
// can see it, because the table those tests read is the table that was emptied.
//
// A nil fetch reads the five files over HTTPS.
func Write(ctx context.Context, root string, fetch irr.DelegationFetch) (WriteReport, error) {
	// No source override: a build host carries no operator config, and the
	// table this writes ships to every appliance, so it is read from the
	// registries themselves.
	delegation, records, err := irr.FetchDelegationTable(ctx, nil, fetch)
	if err != nil {
		return WriteReport{}, err
	}

	table, err := irr.RenderDelegationTable(delegation)
	if err != nil {
		return WriteReport{}, err
	}

	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(outputFile)), table, 0o644); err != nil { //nolint:gosec // a data file the tree ships
		return WriteReport{}, err
	}

	return WriteReport{File: outputFile, Ranges: len(delegation.Ranges), Records: records}, nil
}
