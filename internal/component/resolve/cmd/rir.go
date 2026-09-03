// Design: docs/architecture/resolve.md -- RIR delegation lookup and refresh
// Related: register_rir.go -- the two wire methods these handlers answer
// Related: resolve.go -- the resolve component's other RPC handlers
//
// Two commands read and write one table. `show resolve rir <asn>` answers
// which Regional Internet Registry holds an AS number, and `update resolve
// rir` refreshes the table from the five registry delegation files into the
// managed zefs store.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// delegationFetch is how the refresh reads one registry delegation file. It is
// a variable so a test answers from its own fixtures rather than reaching the
// five public files; production leaves it nil, and irr.FetchDelegationTable
// then reads them over HTTPS.
var delegationFetch irr.DelegationFetch

// handleRIRASN answers which registry holds one AS number. An AS number in no
// delegated range and a table that could not be read are two different
// answers, and this handler keeps them apart (ai/rules/principles.md).
func handleRIRASN(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	asn, errResp := requireASN(args)
	if errResp != nil {
		return errResp, nil
	}

	entry, err := irr.RegistryForASN(asn)
	if errors.Is(err, irr.ErrASNUnallocated) {
		var tb textbuf.Buffer
		return errResponse(tb.Str("AS").Uint32(asn).Str(" is in no delegated range").String())
	}
	if err != nil {
		var tb textbuf.Buffer
		return errResponse(tb.Str("RIR delegation table unreadable: ").Err(err).String())
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"asn":         asn,
			"registry":    entry.RIR,
			"whois":       entry.Whois,
			"range-start": entry.Start,
			"range-end":   entry.End,
		},
	}, nil
}

// handleRIRRefresh fetches the five registry delegation files and stores the
// table under the meta/rir/delegation key. The refresh is all-or-nothing: a
// run that stored nothing reports an error and never reports success.
//
// The write goes through statestore, which holds the config system's own zefs
// handle. This process MUST NOT open a second one: the config store's next
// flush would then re-encode from a stale tree and drop every state key
// (internal/core/statestore package doc).
func handleRIRRefresh(cc *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) != 0 {
		return errResponse("update resolve rir: unexpected arguments")
	}

	// The sources an operator committed are read HERE, when the command runs.
	// Ze builds SystemConfig once at startup and never re-applies it to the
	// resolvers, so a source committed after the daemon started would go
	// unread until a restart. prefix_update.go opens the tree at command time
	// for the same reason.
	//
	// A config file that will not open stops the refresh. An operator who
	// named a mirror MUST NOT be served the five published files because the
	// tree could not be read, which is what an empty answer here would mean
	// (ai/rules/principles.md).
	sources, err := configuredDelegationSources(cc)
	if err != nil {
		var tb textbuf.Buffer
		return errResponse(tb.Str("update resolve rir: ").Err(err).String())
	}

	// Every configured URL answers to the fetch rule before a byte is read.
	// The leaf's own ze:validate refuses one at commit time, and this refuses
	// one that reached the tree another way: a config written by an older
	// binary, or restored from a backup taken before the rule existed.
	for registry, source := range sources {
		if err := config.ValidateFetchURL(source); err != nil {
			var tb textbuf.Buffer
			return errResponse(tb.Str("update resolve rir: the ").Str(registry).
				Str(" delegation source is refused: ").Err(err).String())
		}
	}

	// The fetch also answers how many records the five files yielded before the
	// collapse. That number sizes a GENERATION run, and `./le iana-asn write`
	// publishes it for a developer refreshing the shipped seed. What an
	// operator asks of a refresh is what the table now holds, which is the
	// range count below.
	table, _, err := irr.FetchDelegationTable(refreshContext(cc), sources, delegationFetch)
	if err != nil {
		var tb textbuf.Buffer
		return errResponse(tb.Str("update resolve rir: ").Err(err).String())
	}

	blob, err := irr.RenderDelegationTable(table)
	if err != nil {
		var tb textbuf.Buffer
		return errResponse(tb.Str("update resolve rir: ").Err(err).String())
	}

	stored, err := statestore.Put(zefs.KeyRIRDelegation.Pattern, blob)
	if err != nil {
		var tb textbuf.Buffer
		return errResponse(tb.Str("update resolve rir: the delegation table was not stored: ").Err(err).String())
	}
	if !stored {
		return errResponse("update resolve rir: the delegation table was not stored, because no state store is registered")
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"key":       zefs.KeyRIRDelegation.Pattern,
			"ranges":    len(table.Ranges),
			"generated": table.Generated.Format(time.DateOnly),
		},
	}, nil
}

// configuredDelegationSources answers the delegation file URLs an operator
// committed, keyed by registry token, read from the config tree as it stands
// NOW.
//
// The ERROR is what separates two answers no map can tell apart. A process
// that carries no config file has nothing to read and names no source, which
// is the daemon reading the five published files as it always has. A config
// file that exists and will not open answers an error, so the refresh stops
// rather than reading the published files as if that were what the operator
// asked for.
//
// "No source named" is an ANSWER, so it travels as an empty map beside a nil
// error rather than as a nil one. A nil value beside a nil error asks the
// caller to read absence as data, which is the shape `nilnil` refuses.
func configuredDelegationSources(cc *pluginserver.CommandContext) (map[string]string, error) {
	none := map[string]string{}

	if cc == nil || cc.Server == nil {
		return none, nil
	}

	configPath := cc.Server.ConfigPath()
	if configPath == "" {
		return none, nil
	}

	ed, err := cli.NewEditor(configPath)
	if err != nil {
		return nil, fmt.Errorf("read the config at %s: %w", configPath, err)
	}
	defer func() { _ = ed.Close() }()

	return system.ExtractSystemConfig(ed.Tree()).RIRDelegationSources, nil
}

// refreshContext answers the context the fetch runs under. A command arriving
// over the trusted transport carries its own, and a direct call carries none:
// the per-file timeout in the irr package bounds the run either way.
func refreshContext(cc *pluginserver.CommandContext) context.Context {
	if cc != nil && cc.RequestContext != nil {
		return cc.RequestContext
	}
	return context.Background()
}
