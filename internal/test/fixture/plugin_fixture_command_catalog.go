// Design: docs/architecture/api/commands.md -- "Discovery: what each catalog reader can see"
// Related: plugin_fixture_11_alias.go -- startFixtureDaemon and cli11, reused here
// Related: register_command_catalog.go -- the fixture name this driver answers to

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"
)

const (
	catalogCommandRPKI     = "show bgp rpki"
	catalogCommandROA      = "show bgp rpki roa"
	catalogCommandAdjRIBIn = "show bgp adj-rib-in"
	// catalogSourceRPKI is the plugin instance the configuration below names.
	// commandHelp answers cmd.Process.Name() for a command the plugin registry
	// holds, and "builtin" for one the dispatcher's own table holds
	// (handleBgpCommandHelp, internal/plugins/meta/cmd/help.go). So this value
	// is what says a PLUGIN's declaration is the thing being reported.
	catalogSourceRPKI = "rpki"
	catalogShapeMap   = "map"
	catalogKeyShape   = "answer-shape"
	catalogKeyColumns = "column-orders"
	catalogKeyAddress = "address-fields"
	catalogKeyAliases = "pipe-aliases"
)

// catalogROAColumns is the column ORDER `show bgp rpki roa` declares
// (commandDecls, internal/component/bgp/plugins/rpki/rpki.go). Nothing else in
// the answer carries it: it is neither alphabetical (asn, max-length, prefix)
// nor a shape a reader could derive from the rows, so a surface that invented a
// list rather than reading the declaration answers a different one.
var catalogROAColumns = []string{"prefix", "max-length", "asn"}

// catalogAddressFieldsROA is the field `show bgp rpki roa` declares as an IP
// address. Its presence is what admits `| resolve` and `| origin` there.
var catalogAddressFieldsROA = []string{"prefix"}

// catalogConfig starts two in-tree plugins and no peer. bgp-rpki declares an
// answer shape, a column order, an address field and a pipe alias;
// bgp-adj-rib-in declares a shape and deliberately no column order. Neither
// needs a session, a cache server or a peer to declare any of it: a declaration
// reaches the daemon in the plugin's Stage 1 message at startup.
const catalogConfig = `bgp {
    router-id 192.0.2.254
    session {
        asn {
            local 65000
        }
    }
}
plugin {
    internal rpki {
        use bgp-rpki
    }
    internal adj-rib-in {
        use bgp-adj-rib-in
    }
}
system {
    authentication {
        user ci {
            password "$PASSWORD_HASH"
            profile [ admin ]
        }
    }
}
`

// commandCatalogPluginShape proves the command catalog reports what a plugin
// declares, over both channels the declaration travels.
//
// A running daemon answers `show command help "<name>"` from the registries a
// plugin's Stage 1 message filled. A short-lived client answers
// `ze help command --json` from registry.Registration, with no daemon in reach.
// Both are read here for one command and compared, because the contract is that
// they agree by deriving from one declaration rather than by a check
// reconciling two producers.
func commandCatalogPluginShape(ctx context.Context, _ []string) error {
	daemon, cliEnv, err := startFixtureDaemon(ctx, catalogConfig)
	if err != nil {
		return err
	}
	defer daemon.stop() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	// A plugin's commands reach the registry on its Stage 1 message, which is
	// sent after the daemon writes its ready file. Poll the answer rather than
	// the elapsed time.
	var roaHelp map[string]any
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool {
		doc, helpErr := catalogHelp(ctx, cliEnv, catalogCommandROA)
		if helpErr != nil {
			return false
		}
		roaHelp = doc
		return doc["command"] == catalogCommandROA
	}) {
		return fmt.Errorf("`show command help %q` never named the command: %v\n%s", catalogCommandROA, roaHelp, daemon.stderr.String())
	}

	if err := catalogCheckDaemon(ctx, cliEnv, roaHelp); err != nil {
		return err
	}
	published, err := catalogCheckClient(ctx)
	if err != nil {
		return err
	}
	if err := catalogCheckAgreement(roaHelp, published[catalogCommandROA]); err != nil {
		return err
	}

	fmt.Println("OK")
	return nil
}

// catalogCheckDaemon reads the declaration keys and both halves of the alias
// barrier from the running daemon.
func catalogCheckDaemon(ctx context.Context, cliEnv []string, roaHelp map[string]any) error {
	if source, _ := roaHelp["source"].(string); source != catalogSourceRPKI {
		return fmt.Errorf("`show command help %q` reports source %q, want the plugin instance %q: the daemon answered about something other than a plugin's command", catalogCommandROA, source, catalogSourceRPKI)
	}
	if err := catalogCheckROAFields(roaHelp, "show command help"); err != nil {
		return err
	}

	// The alias barrier, both halves. bgp-rpki declares `summary` on
	// `show bgp rpki`; `show bgp rpki roa` is a command the same plugin
	// declares below that path, so it inherits nothing (aliasBarriers,
	// internal/component/command/alias.go).
	rpkiHelp, err := catalogHelp(ctx, cliEnv, catalogCommandRPKI)
	if err != nil {
		return err
	}
	alias := catalogSummaryAlias(rpkiHelp)
	if alias == nil {
		return fmt.Errorf("`show command help %q` reports no `%s` alias, only %v", catalogCommandRPKI, aliasSummary, catalogAliasNames(rpkiHelp))
	}
	if _, ok := alias["expansion"].(string); !ok {
		return fmt.Errorf("`show command help %q` reports `%s` with no expansion: %v", catalogCommandRPKI, aliasSummary, alias)
	}
	if catalogSummaryAlias(roaHelp) != nil {
		return fmt.Errorf("`show command help %q` answers to `%s`, which its parent declares and it does not: the alias barrier is gone", catalogCommandROA, aliasSummary)
	}

	// A second plugin, declaring a shape and deliberately no column order, so
	// a reader answering a constant rather than the declaration is caught.
	adjHelp, err := catalogHelp(ctx, cliEnv, catalogCommandAdjRIBIn)
	if err != nil {
		return err
	}
	return catalogCheckAdjRIBInFields(adjHelp, "show command help")
}

// catalogCheckClient reads the same declarations from `ze help command --json`,
// which runs in a short-lived process with no daemon in reach, and asks for the
// whole `show bgp rpki` subtree so the alias barrier is read on this side too.
func catalogCheckClient(ctx context.Context) (map[string]map[string]any, error) {
	entries, err := catalogPublished(ctx, catalogCommandRPKI)
	if err != nil {
		return nil, err
	}
	roa := entries[catalogCommandROA]
	if roa == nil {
		return nil, fmt.Errorf("`ze help command %q --json` names no %q: a plugin's command is absent from the published catalog: %v", catalogCommandRPKI, catalogCommandROA, catalogPaths(entries))
	}
	if err := catalogCheckROAFields(roa, "ze help command --json"); err != nil {
		return nil, err
	}

	parent := entries[catalogCommandRPKI]
	if parent == nil {
		return nil, fmt.Errorf("`ze help command %q --json` names no %q: %v", catalogCommandRPKI, catalogCommandRPKI, catalogPaths(entries))
	}
	if catalogSummaryAlias(parent) == nil {
		return nil, fmt.Errorf("`ze help command --json` reports no `%s` alias on %q, only %v", aliasSummary, catalogCommandRPKI, catalogAliasNames(parent))
	}
	if catalogSummaryAlias(roa) != nil {
		return nil, fmt.Errorf("`ze help command --json` gives %q the `%s` its parent declares: the alias barrier is gone", catalogCommandROA, aliasSummary)
	}

	adjEntries, err := catalogPublished(ctx, catalogCommandAdjRIBIn)
	if err != nil {
		return nil, err
	}
	adj := adjEntries[catalogCommandAdjRIBIn]
	if adj == nil {
		return nil, fmt.Errorf("`ze help command %q --json` names no %q: %v", catalogCommandAdjRIBIn, catalogCommandAdjRIBIn, catalogPaths(adjEntries))
	}
	if err := catalogCheckAdjRIBInFields(adj, "ze help command --json"); err != nil {
		return nil, err
	}
	return entries, nil
}

// catalogCheckROAFields holds one surface to the three values bgp-rpki declares
// for `show bgp rpki roa`. The column order is the load-bearing one: no reader
// published a column order at all before the catalog read the declaration.
func catalogCheckROAFields(entry map[string]any, surface string) error {
	shape, _ := entry[catalogKeyShape].(string)
	if shape != shapeTab {
		return fmt.Errorf("%s reports shape %q for %q, want %s: %v", surface, shape, catalogCommandROA, shapeTab, entry)
	}
	orders := catalogOrders(entry[catalogKeyColumns])
	if len(orders) != 1 || !slices.Equal(orders[0], catalogROAColumns) {
		return fmt.Errorf("%s reports column orders %v for %q, want one order %v", surface, orders, catalogCommandROA, catalogROAColumns)
	}
	fields := catalogStrings(entry[catalogKeyAddress])
	if !slices.Equal(fields, catalogAddressFieldsROA) {
		return fmt.Errorf("%s reports address fields %v for %q, want %v", surface, fields, catalogCommandROA, catalogAddressFieldsROA)
	}
	return nil
}

// catalogCheckAdjRIBInFields holds one surface to what bgp-adj-rib-in declares
// for `show bgp adj-rib-in`: the `map` shape, and no column order. A row there
// is a peer's route LIST, whose keys sit one level below the row, so the plugin
// declares no order and a surface that reports one is answering something it
// derived rather than something a plugin said.
func catalogCheckAdjRIBInFields(entry map[string]any, surface string) error {
	shape, _ := entry[catalogKeyShape].(string)
	if shape != catalogShapeMap {
		return fmt.Errorf("%s reports shape %q for %q, want %s: %v", surface, shape, catalogCommandAdjRIBIn, catalogShapeMap, entry)
	}
	if _, present := entry[catalogKeyColumns]; present {
		return fmt.Errorf("%s reports a column order %q declares nowhere: %v", surface, catalogCommandAdjRIBIn, entry[catalogKeyColumns])
	}
	return nil
}

// catalogCheckAgreement holds the daemon's answer and the client's answer to
// the same three values. They read two different channels of one declaration,
// so a disagreement means one of them derived its answer somewhere else.
func catalogCheckAgreement(fromDaemon, fromClient map[string]any) error {
	if fromClient == nil {
		return errors.New("the published catalog carries no entry to compare the daemon's answer against")
	}
	for _, key := range []string{catalogKeyShape, catalogKeyColumns, catalogKeyAddress} {
		daemonValue, err := catalogCanonical(fromDaemon[key])
		if err != nil {
			return fmt.Errorf("the daemon's %q is not JSON: %w", key, err)
		}
		clientValue, err := catalogCanonical(fromClient[key])
		if err != nil {
			return fmt.Errorf("the published catalog's %q is not JSON: %w", key, err)
		}
		if daemonValue != clientValue {
			return fmt.Errorf("%q disagrees on %q: the daemon says %s and the published catalog says %s", catalogCommandROA, key, daemonValue, clientValue)
		}
	}
	return nil
}

// catalogHelp asks the running daemon for one command's help, through a second
// process, and decodes the JSON answer.
func catalogHelp(ctx context.Context, cliEnv []string, name string) (map[string]any, error) {
	command := `show command help "` + name + `" | json`
	code, stdout, stderr, err := cli11(ctx, cliEnv, command)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("%s exit=%d: %w %s%s", command, code, err, stdout, stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		return nil, fmt.Errorf("decode %s: %w: %s", command, err, stdout)
	}
	return doc, nil
}

// catalogPublished runs the offline client catalog and keys its entries by
// path. No daemon serves this: `ze help command` is registered
// commandModeOffline (cmd/ze/ze_core_dispatch.go), so it answers from what its
// own process links.
func catalogPublished(ctx context.Context, filter string) (map[string]map[string]any, error) {
	code, stdout, stderr, err := runCaptured(ctx, os.Environ(), "", "ze", "help", "command", filter, "--json")
	if err != nil || code != 0 {
		return nil, fmt.Errorf("ze help command %q --json exit=%d: %w %s%s", filter, code, err, stdout, stderr)
	}
	var list []map[string]any
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		return nil, fmt.Errorf("decode ze help command %q --json: %w: %s", filter, err, stdout)
	}
	entries := make(map[string]map[string]any, len(list))
	for _, entry := range list {
		path, _ := entry["path"].(string)
		if path != "" {
			entries[path] = entry
		}
	}
	return entries, nil
}

// catalogSummaryAlias answers the `summary` pipe alias of one catalog entry, or
// nil when the entry answers to no such name. It is the one alias bgp-rpki
// declares, and both surfaces spell the list the same way.
func catalogSummaryAlias(entry map[string]any) map[string]any {
	rows, _ := entry[catalogKeyAliases].([]any)
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if row["name"] == aliasSummary {
			return row
		}
	}
	return nil
}

// catalogAliasNames names every pipe alias one catalog entry answers to, for
// the report of an alias it does not carry or one it should not.
func catalogAliasNames(entry map[string]any) []string {
	rows, _ := entry[catalogKeyAliases].([]any)
	names := make([]string, 0, len(rows))
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := row["name"].(string); ok {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// catalogStrings reads a JSON string list. A value of another shape answers
// nil, and every caller compares that against the list it wants and reports it.
func catalogStrings(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil
		}
		result = append(result, text)
	}
	return result
}

// catalogOrders reads the list of column orders, one per record shape the
// command renders.
func catalogOrders(value any) [][]string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([][]string, 0, len(items))
	for _, item := range items {
		names := catalogStrings(item)
		if names == nil {
			return nil
		}
		result = append(result, names)
	}
	return result
}

// catalogCanonical writes one decoded JSON value back as text, so two surfaces
// are compared by what they published rather than by the Go shapes they decoded
// to.
func catalogCanonical(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// catalogPaths names what a catalog answer did carry, for the report of an
// entry it did not.
func catalogPaths(entries map[string]map[string]any) []string {
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}
