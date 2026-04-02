// Design: docs/architecture/web-interface.md -- parameterized layered graph layout
// Related: graph.go -- Graph data model and construction
// Related: text.go -- Unicode box-drawing text renderer

package graph

import "sort"

// Layout holds computed positions for graph nodes.
type Layout struct {
	Positions map[uint32]Position // ASN -> position
	Width     int                 // Total width in config units
	Height    int                 // Total height in config units
}

// Position holds the x,y coordinates and dimensions for a graph node.
type Position struct {
	X      int
	Y      int
	Width  int
	Height int
}

// LayoutConfig holds unit-system constants for the layout algorithm.
// SVG uses pixel values; text rendering uses character-unit values.
type LayoutConfig struct {
	NodeHeight    int
	NodeMinWidth  int
	NodeMaxWidth  int
	NodePadding   int
	HorizontalGap int
	VerticalGap   int
	Padding       int
	CharWidth     int // Approximate character width (pixels for SVG, 1 for text)
}

// SVGConfig returns layout constants for SVG rendering (pixel units).
func SVGConfig() LayoutConfig {
	return LayoutConfig{
		NodeHeight:    40,
		NodeMinWidth:  80,
		NodeMaxWidth:  200,
		NodePadding:   16,
		HorizontalGap: 120,
		VerticalGap:   60,
		Padding:       20,
		CharWidth:     7,
	}
}

// TextConfig returns layout constants for text rendering (character units).
func TextConfig() LayoutConfig {
	return LayoutConfig{
		NodeHeight:    3,  // 3 rows: top border, content, bottom border
		NodeMinWidth:  11, // minimum box width (e.g., "| AS100 |" = 9 + borders)
		NodeMaxWidth:  30,
		NodePadding:   2, // padding chars inside box
		HorizontalGap: 6, // gap between columns for edge drawing
		VerticalGap:   1, // gap between rows
		Padding:       0, // no outer padding for text
		CharWidth:     1,
	}
}

// ComputeLayout assigns x,y positions to each graph node using a layered layout.
// Layers are arranged left-to-right: source ASes at left, origin at right.
// The config parameter controls the unit system (pixels for SVG, characters for text).
func ComputeLayout(g *Graph, cfg LayoutConfig) *Layout {
	if len(g.Nodes) == 0 {
		return &Layout{Positions: make(map[uint32]Position)}
	}

	// Group nodes by layer.
	layers := make(map[int][]Node)
	maxLayer := 0
	for _, n := range g.Nodes {
		layers[n.Layer] = append(layers[n.Layer], n)
		if n.Layer > maxLayer {
			maxLayer = n.Layer
		}
	}

	// Sort nodes within each layer by ASN for deterministic layout.
	for layer := range layers {
		sort.Slice(layers[layer], func(i, j int) bool {
			return layers[layer][i].ASN < layers[layer][j].ASN
		})
	}

	positions := make(map[uint32]Position)

	// Compute column widths based on label text.
	colWidths := make(map[int]int)
	for layer, nodes := range layers {
		maxW := cfg.NodeMinWidth
		for _, n := range nodes {
			label := FormatNodeLabel(n)
			w := min(len(label)*cfg.CharWidth+cfg.NodePadding*2, cfg.NodeMaxWidth)
			if w > maxW {
				maxW = w
			}
		}
		colWidths[layer] = maxW
	}

	// Compute x positions (left-to-right, highest layer first).
	colX := make(map[int]int)
	x := cfg.Padding
	for layer := maxLayer; layer >= 0; layer-- {
		colX[layer] = x
		w, ok := colWidths[layer]
		if !ok {
			w = cfg.NodeMinWidth
		}
		x += w + cfg.HorizontalGap
	}

	totalWidth := x - cfg.HorizontalGap + cfg.Padding

	// Compute y positions within each layer.
	maxHeight := 0
	for layer, nodes := range layers {
		for i, n := range nodes {
			y := cfg.Padding + i*(cfg.NodeHeight+cfg.VerticalGap)
			w := colWidths[layer]
			positions[n.ASN] = Position{
				X:      colX[layer],
				Y:      y,
				Width:  w,
				Height: cfg.NodeHeight,
			}
			bottom := y + cfg.NodeHeight
			if bottom > maxHeight {
				maxHeight = bottom
			}
		}
	}

	totalHeight := maxHeight + cfg.Padding

	return &Layout{
		Positions: positions,
		Width:     totalWidth,
		Height:    totalHeight,
	}
}
