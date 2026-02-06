package yang

import (
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// RPCMeta describes an RPC extracted from a YANG module.
type RPCMeta struct {
	Module      string     // YANG module name (e.g., "ze-bgp-api")
	Name        string     // RPC name in kebab-case (e.g., "peer-list")
	Description string     // From YANG description
	Input       []LeafMeta // Input parameter leaves
	Output      []LeafMeta // Output parameter leaves
}

// LeafMeta describes a leaf parameter from a YANG RPC input/output.
type LeafMeta struct {
	Name        string // Leaf name
	Type        string // YANG type name
	Description string // From YANG description
	Mandatory   bool   // Whether this parameter is required
}

// NotificationMeta describes a notification extracted from a YANG module.
type NotificationMeta struct {
	Module      string     // YANG module name
	Name        string     // Notification name in kebab-case
	Description string     // From YANG description
	Leaves      []LeafMeta // Notification data leaves
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
			Description: valueText(rpc.Description),
		}

		// Extract input/output from entry tree (has resolved types)
		if entry != nil {
			if rpcEntry, ok := entry.Dir[rpc.Name]; ok && rpcEntry.RPC != nil {
				meta.Input = extractEntryLeaves(rpcEntry.RPC.Input)
				meta.Output = extractEntryLeaves(rpcEntry.RPC.Output)
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
			Module:      moduleName,
			Name:        notif.Name,
			Description: valueText(notif.Description),
		}

		// Extract leaves from entry tree
		if entry != nil {
			if notifEntry, ok := entry.Dir[notif.Name]; ok {
				meta.Leaves = extractEntryLeaves(notifEntry)
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

// extractEntryLeaves extracts leaf metadata from an Entry's direct children.
// Used for RPC input/output sections and notification bodies.
func extractEntryLeaves(parent *gyang.Entry) []LeafMeta {
	if parent == nil || parent.Dir == nil {
		return nil
	}

	var leaves []LeafMeta
	for name, child := range parent.Dir {
		if child.Kind != gyang.LeafEntry {
			continue
		}
		leaf := LeafMeta{
			Name:        name,
			Description: child.Description,
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
