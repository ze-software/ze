// Design: docs/architecture/config/yang-config-design.md -- YANG analysis output formatting
// Related: tree.go -- unified analysis tree
// Related: prefix.go -- collision detection

package yang

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// FormatCollisionsText writes collision groups as human-readable text.
func FormatCollisionsText(w io.Writer, groups []CollisionGroup) error {
	if len(groups) == 0 {
		_, err := fmt.Fprintln(w, "No prefix collisions found.")
		return err
	}

	totalAffected := 0
	for _, g := range groups {
		path := strings.Join(g.Path, " > ")
		if path == "" {
			path = "(root)"
		}
		if _, err := fmt.Fprintf(w, "%s > (%d siblings share prefix %q, need %d-%d chars)\n",
			path, len(g.Siblings), g.Prefix, g.MinChars, g.MaxChars); err != nil {
			return err
		}

		for _, s := range g.Siblings {
			disambig := disambigPrefix(s.Name, g.Siblings)
			if _, err := fmt.Fprintf(w, "  %-4s %-20s [%-7s] %-14s %s\n",
				disambig, s.Name, s.Source, s.Type, s.Description); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		totalAffected += len(g.Siblings)
	}

	_, err := fmt.Fprintf(w, "Summary: %d collision groups, %d affected nodes\n", len(groups), totalAffected)
	return err
}

// disambigPrefix returns the minimum prefix needed to distinguish name from its siblings.
func disambigPrefix(name string, siblings []SiblingInfo) string {
	maxLCP := 0
	for _, s := range siblings {
		if s.Name == name {
			continue
		}
		maxLCP = max(maxLCP, longestCommonPrefix(name, s.Name))
	}
	end := min(maxLCP+1, len(name))
	return name[:end]
}

// FormatCollisionsJSON writes collision groups as JSON.
func FormatCollisionsJSON(w io.Writer, groups []CollisionGroup) error {
	type siblingJSON struct {
		Name        string `json:"name"`
		Source      string `json:"source"`
		Type        string `json:"type,omitempty"`
		Description string `json:"description,omitempty"`
	}
	type groupJSON struct {
		Path     []string      `json:"path"`
		Prefix   string        `json:"prefix"`
		MinChars int           `json:"min-chars"`
		MaxChars int           `json:"max-chars"`
		Siblings []siblingJSON `json:"siblings"`
	}
	type outputJSON struct {
		Collisions []groupJSON `json:"collisions"`
		Summary    struct {
			TotalGroups   int `json:"total-groups"`
			TotalAffected int `json:"total-affected"`
		} `json:"summary"`
	}

	out := outputJSON{}
	totalAffected := 0
	for _, g := range groups {
		jg := groupJSON{
			Path:     g.Path,
			Prefix:   g.Prefix,
			MinChars: g.MinChars,
			MaxChars: g.MaxChars,
		}
		if jg.Path == nil {
			jg.Path = []string{}
		}
		for _, s := range g.Siblings {
			jg.Siblings = append(jg.Siblings, siblingJSON(s))
		}
		out.Collisions = append(out.Collisions, jg)
		totalAffected += len(g.Siblings)
	}
	out.Summary.TotalGroups = len(groups)
	out.Summary.TotalAffected = totalAffected

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// formatConstraints returns a constraint annotation string for a tree node.
// Example outputs: "[mandatory]", "[default: 90]", "[0..65535]", "[mandatory] [default: 90]".
func formatConstraints(node *AnalysisNode) string {
	var parts []string
	if node.Mandatory {
		parts = append(parts, "[mandatory]")
	}
	if node.Default != "" {
		parts = append(parts, "[default: "+node.Default+"]")
	}
	if node.Range != "" {
		parts = append(parts, "["+node.Range+"]")
	}
	return strings.Join(parts, " ")
}

// FormatTreeText writes the unified tree as indented text.
func FormatTreeText(w io.Writer, root *AnalysisNode, filter string) error {
	for _, name := range root.SortedChildren() {
		if err := formatTreeNodeText(w, root.Children[name], 0, filter); err != nil {
			return err
		}
	}
	return nil
}

func formatTreeNodeText(w io.Writer, node *AnalysisNode, depth int, filter string) error {
	if filter == SourceConfig && node.Source == SourceCommand {
		return nil
	}
	if filter == FilterCommands && node.Source == SourceConfig {
		return nil
	}

	indent := strings.Repeat("  ", depth)
	typStr := node.Type
	if typStr == "" && node.NodeKind != "" {
		typStr = node.NodeKind
	}

	desc := node.Description
	if len(desc) > 60 {
		desc = desc[:57] + "..."
	}

	// Append constraint annotations.
	constraints := formatConstraints(node)
	if constraints != "" {
		if desc != "" {
			desc = desc + " " + constraints
		} else {
			desc = constraints
		}
	}

	if _, err := fmt.Fprintf(w, "%s%-28s [%-7s] %-14s %s\n",
		indent, node.Name, node.Source, typStr, desc); err != nil {
		return err
	}

	for _, name := range node.SortedChildren() {
		if err := formatTreeNodeText(w, node.Children[name], depth+1, filter); err != nil {
			return err
		}
	}
	return nil
}

// FormatTreeJSON writes the unified tree as JSON.
func FormatTreeJSON(w io.Writer, root *AnalysisNode, filter string) error {
	type nodeJSON struct {
		Name        string      `json:"name"`
		Source      string      `json:"source"`
		Type        string      `json:"type,omitempty"`
		Kind        string      `json:"kind,omitempty"`
		Description string      `json:"description,omitempty"`
		Children    []*nodeJSON `json:"children,omitempty"`
	}

	var convert func(node *AnalysisNode, f string) *nodeJSON
	convert = func(node *AnalysisNode, f string) *nodeJSON {
		if f == SourceConfig && node.Source == SourceCommand {
			return nil
		}
		if f == FilterCommands && node.Source == SourceConfig {
			return nil
		}

		jn := &nodeJSON{
			Name:        node.Name,
			Source:      node.Source,
			Type:        node.Type,
			Kind:        node.NodeKind,
			Description: node.Description,
		}
		for _, name := range node.SortedChildren() {
			child := convert(node.Children[name], f)
			if child != nil {
				jn.Children = append(jn.Children, child)
			}
		}
		return jn
	}

	var nodes []*nodeJSON
	for _, name := range root.SortedChildren() {
		n := convert(root.Children[name], filter)
		if n != nil {
			nodes = append(nodes, n)
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(nodes)
}
