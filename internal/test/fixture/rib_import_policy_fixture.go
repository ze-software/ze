// Design: docs/guide/route-policy.md -- import chains hide paths from the Loc-RIB, and show rib candidates lists them
// Related: register_rib_import_policy.go -- registers the two drivers below
// Related: ../../component/sysrib/candidates.go -- showRIBCandidates, the answer this reads
// Related: ../../core/rib/locrib/policy.go -- admit, which hides a rejected path and names the chain

package fixture

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// The prefixes both scenarios use. Each judged prefix is offered by BGP (through
// `request bgp rib inject`) and by a static blackhole route, so a hidden path is
// read beside the path that won or beside its sibling hidden one.
const (
	importJudgedPrefix  = "192.0.2.0/24"
	importControlPrefix = "198.51.100.0/24"
	importBGPPrefix     = "203.0.113.0/24"
	importPollAttempts  = 100
	importPollDelay     = 100 * time.Millisecond
	importSourceBGP     = "bgp"
	importChainCentral  = "rib import"
	importChainStatic   = "static import"
	importHiddenCentral = "central"
	importHiddenSource  = "source"
)

// candidateRow is one prefix of `show rib candidates`.
type candidateRow struct {
	Prefix string          `json:"prefix"`
	Paths  []candidateItem `json:"paths"`
}

// candidateItem is one path of a candidateRow.
type candidateItem struct {
	Source   string `json:"source"`
	Selected bool   `json:"selected"`
	Hidden   bool   `json:"hidden"`
	HiddenBy string `json:"hidden-by"`
	Chain    string `json:"chain"`
}

// ribImportCentralBogons proves the central `rib filter import` chain hides a
// prefix from every source. BGP and static both offer 192.0.2.0/24 and the chain
// rejects it, so `show rib` does not list it and `show rib candidates prefix`
// lists both paths hidden by the central chain. The control prefixes, one per
// source, are selected: the gate judged them and let them through.
func ribImportCentralBogons(ctx context.Context, plugin *sdk.Plugin) error {
	if err := injectImportRoutes(ctx, plugin, importJudgedPrefix, importBGPPrefix); err != nil {
		return err
	}
	if err := waitRIBWinner(ctx, plugin, importBGPPrefix, importSourceBGP); err != nil {
		return err
	}
	if err := waitRIBWinner(ctx, plugin, importControlPrefix, protocolStatic); err != nil {
		return err
	}
	if winner, raw := ribWinner02(ctx, plugin, importJudgedPrefix); winner != "" {
		return errors.New("show rib lists " + importJudgedPrefix + " from " + winner + ", but the rib import chain rejects it: " + raw)
	}
	row, err := waitCandidates(ctx, plugin, importJudgedPrefix, func(row candidateRow) bool {
		return len(row.Paths) == 2
	})
	if err != nil {
		return err
	}
	for _, source := range []string{importSourceBGP, protocolStatic} {
		p, ok := candidatePathOf(row, source)
		if !ok {
			return errors.New("show rib candidates prefix " + importJudgedPrefix + " has no " + source + " path")
		}
		if err := requireHidden(p, importHiddenCentral, importChainCentral); err != nil {
			return err
		}
	}
	if err := requireSelectedCandidate(ctx, plugin, importControlPrefix, protocolStatic); err != nil {
		return err
	}
	var ok textbuf.Buffer
	ok.Str("OK: ").Str(importJudgedPrefix).Str(" is absent from show rib, and show rib candidates lists its bgp and static paths hidden by the rib import chain\n")
	return ok.StdErr()
}

// staticImportFilterDistance proves the static `filter import` chain hides a
// static path and lets the next source win. Both prefixes are offered by BGP and
// by static. The chain rejects only 192.0.2.0/24, so BGP holds it and the static
// path is hidden by the source chain; 198.51.100.0/24 meets no reject, so static
// wins it on distance. The two prefixes differ only in the reject, so what the
// test proves is that the reject decides.
func staticImportFilterDistance(ctx context.Context, plugin *sdk.Plugin) error {
	if err := injectImportRoutes(ctx, plugin, importJudgedPrefix, importControlPrefix); err != nil {
		return err
	}
	if err := waitRIBWinner(ctx, plugin, importJudgedPrefix, importSourceBGP); err != nil {
		return err
	}
	if err := waitRIBWinner(ctx, plugin, importControlPrefix, protocolStatic); err != nil {
		return err
	}
	row, err := waitCandidates(ctx, plugin, importJudgedPrefix, func(row candidateRow) bool {
		return len(row.Paths) == 2
	})
	if err != nil {
		return err
	}
	hidden, ok := candidatePathOf(row, protocolStatic)
	if !ok {
		return errors.New("show rib candidates prefix " + importJudgedPrefix + " has no static path")
	}
	if err := requireHidden(hidden, importHiddenSource, importChainStatic); err != nil {
		return err
	}
	winner, ok := candidatePathOf(row, importSourceBGP)
	if !ok {
		return errors.New("show rib candidates prefix " + importJudgedPrefix + " has no bgp path")
	}
	if !winner.Selected {
		return errors.New("show rib candidates prefix " + importJudgedPrefix + ": the bgp path is not selected while the static one is hidden")
	}
	if err := requireSelectedCandidate(ctx, plugin, importControlPrefix, protocolStatic); err != nil {
		return err
	}
	var done textbuf.Buffer
	done.Str("OK: the static import chain hides static ").Str(importJudgedPrefix).Str(" so bgp holds it, and static wins ")
	done.Str(importControlPrefix).Str(" on distance\n")
	return done.StdErr()
}

// injectImportRoutes waits for the BGP RIB and injects one BGP route per prefix.
func injectImportRoutes(ctx context.Context, plugin *sdk.Plugin, prefixes ...string) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	for _, prefix := range prefixes {
		var inject textbuf.Buffer
		inject.Str("request bgp rib inject 10.0.0.99 ipv4/unicast ").Str(prefix)
		inject.Str(" origin igp aspath 64500 nexthop 198.51.100.1")
		if _, err := requireDone02(ctx, plugin, inject.String()); err != nil {
			return err
		}
	}
	return nil
}

// waitRIBWinner polls `show rib` until protocol holds prefix.
func waitRIBWinner(ctx context.Context, plugin *sdk.Plugin, prefix, protocol string) error {
	var winner, seen string
	if Poll(ctx, importPollAttempts, importPollDelay, func() bool {
		winner, seen = ribWinner02(ctx, plugin, prefix)
		return winner == protocol
	}) {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str(prefix).Str(": system RIB winner is ").Str(winner).Str(", want ").Str(protocol)
	tb.Str("; show rib: ").Str(seen)
	return errors.New(tb.String())
}

// waitCandidates polls `show rib candidates prefix <prefix>` until ready holds
// for the prefix's row. The selector form is the one an operator types, so the
// dispatcher must route the longer command name with its arguments.
func waitCandidates(ctx context.Context, plugin *sdk.Plugin, prefix string, ready func(candidateRow) bool) (candidateRow, error) {
	command := "show rib candidates prefix " + prefix
	var row candidateRow
	var seen string
	if Poll(ctx, importPollAttempts, importPollDelay, func() bool {
		data, err := requireDone02(ctx, plugin, command)
		if err != nil {
			seen = err.Error()
			return false
		}
		seen = text02(data)
		var rows []candidateRow
		if decode02(data, &rows) != nil {
			return false
		}
		if len(rows) != 1 {
			return false
		}
		if rows[0].Prefix != prefix {
			return false
		}
		row = rows[0]
		return ready(row)
	}) {
		return row, nil
	}
	return candidateRow{}, errors.New(command + ": the answer never held the expected paths: " + seen)
}

// requireSelectedCandidate reads the full `show rib candidates` listing and
// requires prefix to carry a selected, unhidden path from source.
func requireSelectedCandidate(ctx context.Context, plugin *sdk.Plugin, prefix, source string) error {
	data, err := requireDone02(ctx, plugin, "show rib candidates")
	if err != nil {
		return err
	}
	var rows []candidateRow
	if err := decode02(data, &rows); err != nil {
		return errors.New("show rib candidates: " + err.Error() + ": " + text02(data))
	}
	for _, row := range rows {
		if row.Prefix != prefix {
			continue
		}
		p, ok := candidatePathOf(row, source)
		if !ok {
			return errors.New("show rib candidates: " + prefix + " has no " + source + " path: " + text02(data))
		}
		if p.Hidden {
			return errors.New("show rib candidates: the " + source + " path of " + prefix + " is hidden, but no chain rejects it")
		}
		if !p.Selected {
			return errors.New("show rib candidates: the " + source + " path of " + prefix + " is not selected")
		}
		return nil
	}
	return errors.New("show rib candidates does not list " + prefix + ": " + text02(data))
}

// candidatePathOf returns the path source offered in row.
func candidatePathOf(row candidateRow, source string) (candidateItem, bool) {
	for _, p := range row.Paths {
		if p.Source == source {
			return p, true
		}
	}
	return candidateItem{}, false
}

// requireHidden checks a path is hidden, unselected, and names the chain.
func requireHidden(p candidateItem, hiddenBy, chain string) error {
	if !p.Hidden {
		return errors.New("the " + p.Source + " path is not hidden")
	}
	if p.Selected {
		return errors.New("the " + p.Source + " path is hidden and selected")
	}
	if p.HiddenBy != hiddenBy {
		return errors.New("the " + p.Source + " path is hidden by " + p.HiddenBy + ", want " + hiddenBy)
	}
	if p.Chain != chain {
		return errors.New("the " + p.Source + " path names chain " + p.Chain + ", want " + chain)
	}
	return nil
}
