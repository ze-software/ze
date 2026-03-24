// Design: docs/architecture/core-design.md — config tree serialization to Ze format
// Overview: migrate.go — migration orchestration and neighbor conversion
// Related: migrate_routes.go — route conversion to update blocks
// Related: migrate_family.go — family syntax conversion helpers

package migration

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// SerializeTree serializes a config tree to ZeBGP config format.
func SerializeTree(tree *config.Tree) string {
	if tree == nil {
		return ""
	}

	var buf strings.Builder
	serializeTreeIndent(tree, &buf, "", true)
	return buf.String()
}

func serializeTreeIndent(tree *config.Tree, buf *strings.Builder, indent string, isRoot bool) {
	// Write simple values (sorted for deterministic output).
	keys := tree.Values()
	sort.Strings(keys)
	for _, key := range keys {
		v, ok := tree.Get(key)
		if !ok {
			continue
		}
		switch {
		case v == "":
			buf.WriteString(indent)
			buf.WriteString(key)
			buf.WriteString("\n")
		case strings.Contains(v, " ") && !strings.HasPrefix(v, "[") && !strings.HasPrefix(v, "\""):
			buf.WriteString(indent)
			buf.WriteString(key)
			buf.WriteString(" \"")
			buf.WriteString(v)
			buf.WriteString("\"\n")
		default: // single word or already bracketed/quoted
			buf.WriteString(indent)
			buf.WriteString(key)
			buf.WriteString(" ")
			buf.WriteString(v)
			buf.WriteString("\n")
		}
	}

	// Write plugin blocks - new syntax: plugin { external NAME { ... } }
	pluginList := tree.GetListOrdered("plugin")
	if len(pluginList) > 0 {
		buf.WriteString(indent)
		buf.WriteString("plugin {\n")
		for _, entry := range pluginList {
			buf.WriteString(indent)
			buf.WriteString("\texternal ")
			buf.WriteString(entry.Key)
			buf.WriteString(" {\n")
			serializeTreeIndent(entry.Value, buf, indent+"\t\t", false)
			buf.WriteString(indent)
			buf.WriteString("\t}\n")
		}
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Write group blocks - wrap in bgp {} at root level.
	// Groups contain nested peer blocks.
	groupList := tree.GetListOrdered("group")
	if len(groupList) > 0 {
		if isRoot {
			buf.WriteString(indent)
			buf.WriteString("bgp {\n")
			indent += "\t"
		}
		for _, groupEntry := range groupList {
			buf.WriteString(indent)
			buf.WriteString("group ")
			buf.WriteString(groupEntry.Key)
			buf.WriteString(" {\n")
			groupIndent := indent + "\t"
			// Write group-level values first.
			serializeGroupValues(groupEntry.Value, buf, groupIndent)
			// Write nested peer blocks.
			for _, peerEntry := range groupEntry.Value.GetListOrdered("peer") {
				buf.WriteString(groupIndent)
				buf.WriteString("peer ")
				buf.WriteString(peerEntry.Key)
				buf.WriteString(" {\n")
				serializeTreeIndent(peerEntry.Value, buf, groupIndent+"\t", false)
				buf.WriteString(groupIndent)
				buf.WriteString("}\n")
			}
			buf.WriteString(indent)
			buf.WriteString("}\n")
		}
		if isRoot {
			indent = indent[:len(indent)-1]
			buf.WriteString(indent)
			buf.WriteString("}\n")
		}
	}

	// Write peer blocks (legacy -- should not appear after migration).
	peerList := tree.GetListOrdered("peer")
	if len(peerList) > 0 {
		if isRoot {
			buf.WriteString(indent)
			buf.WriteString("bgp {\n")
			indent += "\t"
		}
		for _, entry := range peerList {
			buf.WriteString(indent)
			buf.WriteString("peer ")
			buf.WriteString(entry.Key)
			buf.WriteString(" {\n")
			serializeTreeIndent(entry.Value, buf, indent+"\t", false)
			buf.WriteString(indent)
			buf.WriteString("}\n")
		}
		if isRoot {
			indent = indent[:len(indent)-1]
			buf.WriteString(indent)
			buf.WriteString("}\n")
		}
	}

	// Write local container (local > as, local > ip).
	if local := tree.GetContainer("local"); local != nil {
		buf.WriteString(indent)
		buf.WriteString("local {\n")
		serializeTreeIndent(local, buf, indent+"\t", false)
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Write remote container (remote > as, remote > ip).
	if remote := tree.GetContainer("remote"); remote != nil {
		buf.WriteString(indent)
		buf.WriteString("remote {\n")
		serializeTreeIndent(remote, buf, indent+"\t", false)
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Write timer container (timer > receive-hold-time, timer > connect-retry).
	if timer := tree.GetContainer("timer"); timer != nil {
		buf.WriteString(indent)
		buf.WriteString("timer {\n")
		serializeTreeIndent(timer, buf, indent+"\t", false)
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Write nested containers.
	if cap := tree.GetContainer("capability"); cap != nil {
		buf.WriteString(indent)
		buf.WriteString("capability {\n")
		serializeTreeIndent(cap, buf, indent+"\t", false)
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Family entries stored as list entries by convertFamilyToList.
	familyList := tree.GetListOrdered("family")
	if len(familyList) > 0 {
		buf.WriteString(indent)
		buf.WriteString("family {\n")
		for _, entry := range familyList {
			buf.WriteString(indent)
			buf.WriteString("\t")
			buf.WriteString(entry.Key)
			// Serialize family sub-tree (e.g., prefix { maximum N; }) if present.
			if prefix := entry.Value.GetContainer("prefix"); prefix != nil {
				buf.WriteString(" { prefix {")
				if max, ok := prefix.Get("maximum"); ok {
					buf.WriteString(" maximum ")
					buf.WriteString(max)
					buf.WriteString(";")
				}
				buf.WriteString(" } }")
			}
			buf.WriteString("\n")
		}
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Write hostname block (FQDN capability).
	if hostname := tree.GetContainer("hostname"); hostname != nil {
		buf.WriteString(indent)
		buf.WriteString("hostname {\n")
		serializeTreeIndent(hostname, buf, indent+"\t", false)
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Write nexthop block (RFC 8950).
	if nexthop := tree.GetContainer("nexthop"); nexthop != nil {
		buf.WriteString(indent)
		buf.WriteString("nexthop {\n")
		serializeTreeIndent(nexthop, buf, indent+"\t", false)
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Write attribute block (used in update blocks).
	if attr := tree.GetContainer("attribute"); attr != nil {
		buf.WriteString(indent)
		buf.WriteString("attribute {\n")
		serializeTreeIndent(attr, buf, indent+"\t", false)
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Write nlri block (used in update blocks) — stored as list entries.
	nlriEntries := tree.GetListOrdered("nlri")
	if len(nlriEntries) > 0 {
		buf.WriteString(indent)
		buf.WriteString("nlri {\n")
		for _, entry := range nlriEntries {
			family := entry.Key
			// Strip #N duplicate suffix for display
			if idx := strings.LastIndex(family, "#"); idx > 0 {
				family = family[:idx]
			}
			content, _ := entry.Value.Get("content")
			buf.WriteString(indent)
			buf.WriteString("\t")
			buf.WriteString(family)
			if content != "" {
				buf.WriteString(" ")
				buf.WriteString(content)
			}
			buf.WriteString("\n")
		}
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Watchdog block (routes controlled via "bgp watchdog announce/withdraw").
	if wdog := tree.GetContainer("watchdog"); wdog != nil {
		buf.WriteString(indent)
		buf.WriteString("watchdog {\n")
		serializeTreeIndent(wdog, buf, indent+"\t", false)
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Write process bindings.
	for _, entry := range tree.GetListOrdered("process") {
		buf.WriteString(indent)
		buf.WriteString("process ")
		buf.WriteString(entry.Key)
		buf.WriteString(" {\n")
		serializeTreeIndent(entry.Value, buf, indent+"\t", false)
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// send/receive are now bracket leaf-lists (e.g. "receive [ update ];")
	// and are written by the generic value-writing code above.

	// Write update blocks (converted from announce/static/flow).
	for _, entry := range tree.GetListOrdered("update") {
		buf.WriteString(indent)
		buf.WriteString("update {\n")
		serializeTreeIndent(entry.Value, buf, indent+"\t", false)
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Templates are expanded via inherit, not serialized.
}

// serializeGroupValues writes the group-level fields (everything except nested peer list).
// This avoids recursing into serializeTreeIndent which would also write peer blocks.
func serializeGroupValues(tree *config.Tree, buf *strings.Builder, indent string) {
	// Write simple values (sorted for deterministic output).
	keys := tree.Values()
	sort.Strings(keys)
	for _, key := range keys {
		v, ok := tree.Get(key)
		if !ok {
			continue
		}
		buf.WriteString(indent)
		buf.WriteString(key)
		if v != "" {
			buf.WriteString(" ")
			buf.WriteString(v)
		}
		buf.WriteString("\n")
	}
}
