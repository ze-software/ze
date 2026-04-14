// Design: docs/architecture/config/yang-config-design.md -- unified analysis tree
// Related: prefix.go -- prefix collision analysis on sibling groups
// Related: format.go -- output formatting for trees and collisions
// Related: doc.go -- command documentation from RPCs

package yang

import (
	"fmt"
	"sort"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all" // trigger init() registration for all schemas
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"

	// Blank imports trigger init() registration for RPC handlers.
	// These packages call pluginserver.RegisterRPCs in init() but are not
	// included in plugin/all due to import cycles (see plugin_imports.go rpcDirs).
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/rib"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/handler"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cli"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/bfd"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/cache"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/commit"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/del"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/log"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/meta"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/metrics"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/set"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/show"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/subscribe"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/update"
)

// AnalysisNode is a node in the unified analysis tree.
type AnalysisNode struct {
	Name        string
	Source      string // SourceConfig, SourceCommand, or SourceBoth
	Type        string // YANG type name (config nodes) or empty
	Description string
	NodeKind    string // "container", "list", "leaf", "leaf-list", SourceCommand, "branch"
	Mandatory   bool   // YANG mandatory constraint
	Default     string // YANG default value (first element if multiple)
	Range       string // YANG range constraint (e.g., "0..65535")
	Children    map[string]*AnalysisNode
}

// BuildUnifiedTree loads YANG schemas and RPC registrations, then merges
// config entries and command entries into a single analysis tree.
func BuildUnifiedTree() (*AnalysisNode, error) {
	root := &AnalysisNode{
		Name:     "(root)",
		Source:   SourceBoth,
		Children: make(map[string]*AnalysisNode),
	}

	if err := addConfigNodes(root); err != nil {
		return nil, fmt.Errorf("config nodes: %w", err)
	}

	addCommandNodes(root)

	return root, nil
}

// addConfigNodes loads YANG conf modules and walks them into the tree.
func addConfigNodes(root *AnalysisNode) error {
	loader, err := yang.DefaultLoader()
	if err != nil {
		return fmt.Errorf("YANG loader: %w", err)
	}

	for _, modName := range loader.ConfModuleNames() {
		entry := loader.GetEntry(modName)
		if entry == nil || entry.Dir == nil {
			continue
		}
		for name, child := range entry.Dir {
			walkYANGEntry(root, name, child)
		}
	}

	return nil
}

// walkYANGEntry recursively converts a YANG entry into analysis nodes.
func walkYANGEntry(parent *AnalysisNode, name string, entry *gyang.Entry) {
	if entry == nil {
		return
	}

	// Skip RPC entries in config modules (they're handled via command tree).
	if entry.RPC != nil {
		return
	}

	// Skip notification entries.
	if entry.Kind == gyang.NotificationEntry {
		return
	}

	existing, exists := parent.Children[name]
	if exists {
		// Node exists from command tree -- mark as SourceBoth.
		existing.Source = SourceBoth
		if existing.Type == "" {
			existing.Type = yangTypeName(entry)
		}
		if existing.Description == "" {
			existing.Description = entry.Description
		}
		if existing.NodeKind == "" || existing.NodeKind == "branch" || existing.NodeKind == SourceCommand {
			existing.NodeKind = yangNodeKind(entry)
		}
	} else {
		existing = &AnalysisNode{
			Name:        name,
			Source:      SourceConfig,
			Type:        yangTypeName(entry),
			Description: entry.Description,
			NodeKind:    yangNodeKind(entry),
			Mandatory:   entry.Mandatory == gyang.TSTrue,
			Default:     yangDefault(entry),
			Range:       yangRange(entry),
			Children:    make(map[string]*AnalysisNode),
		}
		parent.Children[name] = existing
	}

	// Recurse into children (skip list key leaves).
	if entry.Dir != nil {
		for childName, childEntry := range entry.Dir {
			if entry.IsList() && entry.Key == childName {
				continue // skip list key
			}
			walkYANGEntry(existing, childName, childEntry)
		}
	}
}

// addCommandNodes builds the command tree from YANG -cmd modules and merges it.
func addCommandNodes(root *AnalysisNode) {
	loader, _ := yang.DefaultLoader()
	cmdTree := yang.BuildCommandTree(loader)
	if cmdTree == nil || cmdTree.Children == nil {
		return
	}

	walkCommandNode(root, cmdTree)
}

// walkCommandNode recursively merges command.Node entries into the analysis tree.
func walkCommandNode(parent *AnalysisNode, node *command.Node) {
	if node == nil || node.Children == nil {
		return
	}

	for name, child := range node.Children {
		existing, exists := parent.Children[name]
		if exists {
			// Node exists from config -- mark as SourceBoth.
			if existing.Source == SourceConfig {
				existing.Source = SourceBoth
			}
			if existing.Description == "" && child.Description != "" {
				existing.Description = child.Description
			}
		} else {
			kind := "branch"
			if len(child.Children) == 0 {
				kind = SourceCommand
			}
			existing = &AnalysisNode{
				Name:        name,
				Source:      SourceCommand,
				Description: child.Description,
				NodeKind:    kind,
				Children:    make(map[string]*AnalysisNode),
			}
			parent.Children[name] = existing
		}

		walkCommandNode(existing, child)
	}
}

// CollectCollisions walks the unified tree and returns all collision groups.
func CollectCollisions(root *AnalysisNode, minPrefix int) []CollisionGroup {
	var groups []CollisionGroup
	collectCollisionsRecursive(root, nil, minPrefix, &groups)
	return groups
}

func collectCollisionsRecursive(node *AnalysisNode, path []string, minPrefix int, groups *[]CollisionGroup) {
	if node == nil || len(node.Children) < 2 {
		return
	}

	siblings := make([]SiblingInfo, 0, len(node.Children))
	for _, child := range node.Children {
		siblings = append(siblings, SiblingInfo{
			Name:        child.Name,
			Source:      child.Source,
			Type:        child.Type,
			Description: child.Description,
		})
	}

	found := FindCollisions(siblings, minPrefix)
	for i := range found {
		found[i].Path = append([]string{}, path...)
	}
	*groups = append(*groups, found...)

	// Sort children for deterministic walk order.
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		child := node.Children[name]
		collectCollisionsRecursive(child, append(path, name), minPrefix, groups)
	}
}

// SortedChildren returns child names in sorted order.
func (n *AnalysisNode) SortedChildren() []string {
	names := make([]string, 0, len(n.Children))
	for name := range n.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// yangTypeName extracts the YANG type name from an entry.
func yangTypeName(entry *gyang.Entry) string {
	if entry.Type == nil {
		return ""
	}
	return entry.Type.Name
}

// yangDefault returns the first default value from a YANG entry, or empty string.
func yangDefault(entry *gyang.Entry) string {
	if len(entry.Default) > 0 {
		return entry.Default[0]
	}
	return ""
}

// yangRange returns the range constraint string from a YANG entry's type, or empty string.
func yangRange(entry *gyang.Entry) string {
	if entry.Type != nil && len(entry.Type.Range) > 0 {
		return entry.Type.Range.String()
	}
	return ""
}

// yangNodeKind returns a human-readable kind string for a YANG entry.
func yangNodeKind(entry *gyang.Entry) string {
	switch {
	case entry.IsList() && entry.ListAttr != nil:
		if entry.Key != "" {
			return "list[" + entry.Key + "]"
		}
		return "list"
	case entry.IsLeaf():
		return "leaf"
	case entry.IsLeafList():
		return "leaf-list"
	case entry.IsContainer():
		return "container"
	default:
		return strings.ToLower(entry.Kind.String())
	}
}

// AllRPCDocs returns documentation for all registered operational commands.
// It loads YANG API modules to extract input/output parameter metadata.
func AllRPCDocs() ([]RPCDoc, error) {
	loader, err := yang.DefaultLoader()
	if err != nil {
		return nil, fmt.Errorf("YANG loader: %w", err)
	}
	wireToPath := yang.WireMethodToPath(loader)

	cmdTree := yang.BuildCommandTree(loader)

	rpcs := pluginserver.AllBuiltinRPCs()
	docs := make([]RPCDoc, 0, len(rpcs))
	for _, reg := range rpcs {
		cliPath := wireToPath[reg.WireMethod]
		if cliPath == "" {
			continue
		}
		docs = append(docs, RPCDoc{
			CLICommand: cliPath,
			Help:       lookupYANGDesc(cmdTree, cliPath),
			ReadOnly:   pluginserver.IsReadOnlyPath(cliPath),
			WireMethod: reg.WireMethod,
		})
	}

	// Load YANG API modules to get parameter info.
	paramIndex, err := loadRPCParams()
	if err != nil {
		return nil, fmt.Errorf("load RPC parameters: %w", err)
	}
	for i := range docs {
		if params, ok := paramIndex[docs[i].WireMethod]; ok {
			docs[i].Input = params.Input
			docs[i].Output = params.Output
		}
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].CLICommand < docs[j].CLICommand
	})
	return docs, nil
}

// rpcParams holds extracted input/output parameter leaves for matching.
type rpcParams struct {
	Input  []yang.LeafMeta
	Output []yang.LeafMeta
}

// loadRPCParams loads YANG API modules and returns a map of wire method to parameters.
func loadRPCParams() (map[string]rpcParams, error) {
	loader, err := yang.DefaultLoader()
	if err != nil {
		return nil, fmt.Errorf("YANG loader: %w", err)
	}

	result := make(map[string]rpcParams)

	for _, moduleName := range loader.APIModuleNames() {
		wireModule := yang.WireModule(moduleName)
		rpcs := yang.ExtractRPCs(loader, moduleName)
		for _, rpc := range rpcs {
			wireMethod := wireModule + ":" + rpc.Name
			result[wireMethod] = rpcParams{
				Input:  rpc.Input,
				Output: rpc.Output,
			}
		}
	}

	return result, nil
}

// lookupYANGDesc walks a command tree by CLI path and returns the leaf description.
func lookupYANGDesc(root *command.Node, cliPath string) string {
	if root == nil {
		return ""
	}
	node := root
	for part := range strings.FieldsSeq(cliPath) {
		if node.Children == nil {
			return ""
		}
		child, ok := node.Children[part]
		if !ok {
			return ""
		}
		node = child
	}
	return node.Description
}

// RPCDoc holds documentation for a single operational command.
type RPCDoc struct {
	CLICommand string
	Help       string
	ReadOnly   bool
	WireMethod string
	Input      []yang.LeafMeta // Input parameter leaves from YANG
	Output     []yang.LeafMeta // Output parameter leaves from YANG
}
