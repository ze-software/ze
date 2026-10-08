// Design: docs/architecture/wire/attributes.md -- deliberate IANA snapshot refresh.
// The input is the network, not a reproducible tree-derived file. Like data
// asn-delegation, this writer has no offline check twin and is not invoked by
// aggregate generators. Runtime only reads the already shipped snapshots.
package dataipspecialpurpose

import (
	"context"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

const area = "data ip-special-purpose"

var actions = leaction.New(area,
	leaction.Action{
		Verb:   "write",
		Why:    "fetch and validate both canonical IANA special-purpose IP registries before rewriting the shipped XML snapshots; deliberately reaches the network",
		Writes: true,
		Answer: runWrite,
	},
)

// Actions answers the registered action surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs derives the help hint from the same action table as dispatch.
func Subs() string { return actions.Subs() }

// Answer dispatches the data ip-special-purpose command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runWrite() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := Write(context.Background(), root, nil)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}
