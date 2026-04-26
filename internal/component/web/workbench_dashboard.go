// Design: docs/architecture/web-components.md -- Workbench dashboard overview
// Related: handler_workbench.go -- Handler that renders the dashboard at root
// Related: workbench_sections.go -- Navigation model
// Related: render.go -- Fragment rendering
//
// Spec: plan/spec-web-3-foundation.md (Dashboard overview, Phase 6).
//
// DashboardData drives the workbench_dashboard.html template, rendering an
// overview grid with system info, BGP summary, interface summary, and event
// panels. In v1, operational state (BGP session state, warnings, errors) is
// placeholder; subsequent specs populate these via SSE or command dispatch.

package web

import (
	"fmt"
	"runtime"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/version"
)

// DashboardData holds the data for the workbench dashboard overview page.
type DashboardData struct {
	System     DashboardSystemPanel
	BGP        DashboardBGPPanel
	Interfaces DashboardIfacePanel
	Warnings   []DashboardEvent
	Errors     []DashboardEvent
}

// DashboardSystemPanel displays system identity and runtime stats.
type DashboardSystemPanel struct {
	Hostname string
	Uptime   string
	Version  string
	CPUCount int
	MemAlloc string
}

// DashboardBGPPanel summarizes BGP peer state. In v1, only Total and Empty
// are derived from the config tree; session-state counts are populated by
// later specs via operational data.
type DashboardBGPPanel struct {
	Established int
	Active      int
	Idle        int
	Down        int
	Total       int
	Empty       bool
	HintURL     string
}

// DashboardIfacePanel summarizes interface state. In v1, only Total and Empty
// are derived from the config tree; link-state counts are placeholder.
type DashboardIfacePanel struct {
	Up        int
	Down      int
	AdminDown int
	Total     int
	Empty     bool
	HintURL   string
}

// DashboardEvent represents a recent warning or error event.
type DashboardEvent struct {
	Time      string
	Component string
	Message   string
}

// BuildDashboardData assembles dashboard panels from available sources.
// In v1, this uses config tree walking for counts and Go runtime for system
// info. Real-time operational data (BGP session state, warnings, errors) will
// be populated by subsequent specs via SSE or command dispatch.
func BuildDashboardData(tree *config.Tree, schema *config.Schema) DashboardData {
	return DashboardData{
		System:     buildSystemPanel(tree),
		BGP:        buildBGPPanel(tree, schema),
		Interfaces: buildIfacePanel(tree, schema),
	}
}

// buildSystemPanel extracts system identity from the config tree and runtime
// stats from the Go runtime.
func buildSystemPanel(tree *config.Tree) DashboardSystemPanel {
	hostname := ""
	if tree != nil {
		if sys := tree.GetContainer("system"); sys != nil {
			if h, ok := sys.Get("host"); ok {
				hostname = h
			}
		}
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return DashboardSystemPanel{
		Hostname: hostname,
		Uptime:   "-", // Populated by later spec via operational data.
		Version:  version.Short(),
		CPUCount: runtime.NumCPU(),
		MemAlloc: formatBytes(mem.Alloc),
	}
}

// buildBGPPanel counts configured BGP peers from the config tree. Peers can
// appear directly under bgp/peer and inside bgp/group/*/peer.
func buildBGPPanel(tree *config.Tree, schema *config.Schema) DashboardBGPPanel {
	if tree == nil || schema == nil {
		return DashboardBGPPanel{Empty: true, HintURL: "/show/bgp/peer/"}
	}

	total := 0

	bgpTree := tree.GetContainer("bgp")
	if bgpTree != nil {
		// Direct peers: bgp -> peer (list).
		total += countListKeys(bgpTree, "peer")

		// Group peers: bgp -> group -> <key> -> peer (list).
		groupEntries := bgpTree.GetList("group")
		for _, groupEntry := range groupEntries {
			total += countListKeys(groupEntry, "peer")
		}
	}

	if total == 0 {
		return DashboardBGPPanel{Empty: true, HintURL: "/show/bgp/peer/"}
	}

	return DashboardBGPPanel{
		Total:   total,
		HintURL: "/show/bgp/peer/",
	}
}

// buildIfacePanel counts configured interfaces from the config tree.
func buildIfacePanel(tree *config.Tree, schema *config.Schema) DashboardIfacePanel {
	if tree == nil || schema == nil {
		return DashboardIfacePanel{Empty: true, HintURL: "/show/iface/"}
	}

	total := countListKeys(tree, "iface")

	if total == 0 {
		return DashboardIfacePanel{Empty: true, HintURL: "/show/iface/"}
	}

	return DashboardIfacePanel{
		Total:   total,
		HintURL: "/show/iface/",
	}
}

// countListKeys returns the number of entries in a named list on the given tree.
func countListKeys(tree *config.Tree, listName string) int {
	if tree == nil {
		return 0
	}
	entries := tree.GetList(listName)
	return len(entries)
}

// formatBytes formats a byte count into a human-readable string.
func formatBytes(b uint64) string {
	const (
		kib = 1024
		mib = kib * 1024
		gib = mib * 1024
	)

	switch {
	case b >= gib:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(gib))
	case b >= mib:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(mib))
	case b >= kib:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(kib))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
