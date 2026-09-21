// Design: docs/architecture/config/yang-config-design.md — YANG schema handling

package yang

import (
	"slices"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// RPCMeta describes an RPC extracted from a YANG module.
//
// ShortHelp and Description are the two help texts a command node also declares
// (command.go, mergeYANGEntry): the ze:help extension is the one-line SUMMARY
// that every list row and table cell renders, and the YANG description beside
// it is the LONG explanation a help page prints. Neither is derived from the
// other, and an empty Description means nobody has written the explanation yet.
type RPCMeta struct {
	Module      string     // YANG module name (e.g., "ze-bgp-api")
	Name        string     // RPC name in kebab-case (e.g., "peer-list")
	ShortHelp   string     // One-line summary, from the ze:help extension
	Description string     // Long explanation, from the YANG description
	Input       []LeafMeta // Input parameter leaves
	Output      []LeafMeta // Output parameter leaves
}

// LeafMeta describes a leaf parameter from a YANG RPC input/output or a
// notification body. ShortHelp and Description are the leaf's two declared
// texts; neither is derived from the other, and an empty Description means
// nobody has written the explanation yet.
type LeafMeta struct {
	Name        string // Leaf name
	Type        string // YANG type name
	ShortHelp   string // One-line summary, from the ze:help extension
	Description string // Long explanation, from the YANG description
	Mandatory   bool   // Whether this parameter is required
}

// NotificationMeta describes a notification extracted from a YANG module.
type NotificationMeta struct {
	Module    string     // YANG module name
	Name      string     // Notification name in kebab-case
	ShortHelp string     // One-line summary, from the ze:help extension
	Leaves    []LeafMeta // Notification data leaves
}

// ExtractRPCs extracts RPC metadata from a loaded YANG module.
// Returns nil if the module doesn't exist or has no RPCs.
func ExtractRPCs(loader *Loader, moduleName string) []RPCMeta {
	mod := loader.GetModule(moduleName)
	if mod == nil {
		return nil
	}

	entry := loader.GetEntry(moduleName)

	var rpcs []RPCMeta
	for _, rpc := range mod.RPC {
		meta := RPCMeta{
			Module:      moduleName,
			Name:        rpc.Name,
			ShortHelp:   GetHelpExtension(rpc.Exts()), // the ze:help summary
			Description: valueText(rpc.Description),   // the YANG description explanation
		}

		// Extract input/output from entry tree (has resolved types)
		if entry != nil {
			if rpcEntry, ok := entry.Dir[rpc.Name]; ok && rpcEntry.RPC != nil {
				meta.Input = extractEntryLeaves(rpcEntry.RPC.Input, inputOrder(rpc.Input))
				meta.Output = extractEntryLeaves(rpcEntry.RPC.Output, outputOrder(rpc.Output))
			}
		}

		rpcs = append(rpcs, meta)
	}

	return rpcs
}

// ExtractNotifications extracts notification metadata from a loaded YANG module.
// Returns nil if the module doesn't exist or has no notifications.
func ExtractNotifications(loader *Loader, moduleName string) []NotificationMeta {
	mod := loader.GetModule(moduleName)
	if mod == nil {
		return nil
	}

	entry := loader.GetEntry(moduleName)

	var notifs []NotificationMeta
	for _, notif := range mod.Notification {
		meta := NotificationMeta{
			Module:    moduleName,
			Name:      notif.Name,
			ShortHelp: GetHelpExtension(notif.Exts()), // the ze:help summary
		}

		// Extract leaves from entry tree
		if entry != nil {
			if notifEntry, ok := entry.Dir[notif.Name]; ok {
				meta.Leaves = extractEntryLeaves(notifEntry, declaredOrder(notif.Leaf))
			}
		}

		notifs = append(notifs, meta)
	}

	return notifs
}

// WireModule converts a YANG module name to its wire method prefix.
// Strips "-api" or "-conf" suffix: "ze-bgp-api" → "ze-bgp".
func WireModule(moduleName string) string {
	if base, ok := strings.CutSuffix(moduleName, "-api"); ok {
		return base
	}
	if base, ok := strings.CutSuffix(moduleName, "-conf"); ok {
		return base
	}
	return moduleName
}

// inputOrder and outputOrder answer the declaration order of an RPC's two
// halves. An RPC that declares neither has a nil statement and no order, which
// leaves the entry tree's own leaves to be sorted by name.
func inputOrder(in *gyang.Input) []string {
	if in == nil {
		return nil
	}

	return declaredOrder(in.Leaf)
}

func outputOrder(out *gyang.Output) []string {
	if out == nil {
		return nil
	}

	return declaredOrder(out.Leaf)
}

// declaredOrder answers the names a YANG statement declares, in the order the
// module wrote them. A `uses` of a grouping declares none of its leaves here,
// which is why the caller treats this as an ordering hint rather than as the
// population.
func declaredOrder(declared []*gyang.Leaf) []string {
	names := make([]string, 0, len(declared))
	for _, leaf := range declared {
		names = append(names, leaf.Name)
	}

	return names
}

// extractEntryLeaves extracts leaf metadata from an Entry's direct children,
// in the order the module declared them.
// Used for RPC input/output sections and notification bodies.
//
// The ORDER is load-bearing and the entry tree does not hold it: Entry.Dir is a
// map, so ranging it answers a different order on every process. That order
// reaches an operator -- it is the parameter list of an MCP tool and of the
// generated command help -- so the same schema described its own arguments
// differently on each start, and a test asserting the first parameter passed or
// failed on the toss of Go's map seed.
//
// The declared names come from the AST, which keeps them in source order. They
// are an ordering HINT and never the population: a leaf reached through a `uses`
// is in the entry tree and in no AST list, so what the hint does not name is
// appended by name. Sorting the whole set instead would be deterministic and
// wrong, because a YANG author writes the mandatory argument first.
func extractEntryLeaves(parent *gyang.Entry, declared []string) []LeafMeta {
	if parent == nil || parent.Dir == nil {
		return nil
	}

	rest := make([]string, 0, len(parent.Dir))
	for name, child := range parent.Dir {
		if child.Kind == gyang.LeafEntry && !slices.Contains(declared, name) {
			rest = append(rest, name)
		}
	}
	slices.Sort(rest)

	var leaves []LeafMeta
	for _, name := range slices.Concat(declared, rest) {
		child, held := parent.Dir[name]
		if !held || child.Kind != gyang.LeafEntry {
			continue
		}
		leaf := LeafMeta{
			Name:        name,
			ShortHelp:   GetHelpExtension(child.Exts), // the ze:help summary
			Description: child.Description,            // the YANG description explanation
			Mandatory:   child.Mandatory == gyang.TSTrue,
		}
		if child.Type != nil {
			leaf.Type = child.Type.Name
		}
		leaves = append(leaves, leaf)
	}

	return leaves
}

// valueText extracts text from a YANG Value pointer.
func valueText(v *gyang.Value) string {
	if v == nil {
		return ""
	}
	return v.Name
}
