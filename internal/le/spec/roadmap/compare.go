// Design: docs/contributing/spec-workflow.md -- pinned endpoint queue changes.
package roadmap

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// EndpointLimit explains the population that an endpoint comparison cannot observe.
const EndpointLimit = "Endpoint comparison only: items added and removed entirely between these revisions are absent. Queue changes do not establish delivered work."

// Change preserves both endpoints, including commit-pinned links for removed items.
// A changed item can carry a move and a status transition together.
type Change struct {
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Moved         bool   `json:"moved"`
	StatusChanged bool   `json:"status-changed"`
	Before        *Item  `json:"before,omitempty"`
	After         *Item  `json:"after,omitempty"`
}

// Comparison contains the inventories that produced every count and change.
type Comparison struct {
	SchemaVersion int      `json:"schema-version"`
	Qualification string   `json:"qualification"`
	Limitation    string   `json:"limitation"`
	From          Snapshot `json:"from"`
	To            Snapshot `json:"to"`
	Changes       []Change `json:"changes"`
}

// Compare identifies items by exact stem. Renamed stems are removed plus added.
func Compare(ctx context.Context, root, from, to string) (*Comparison, error) {
	before, err := Collect(ctx, root, from)
	if err != nil {
		return nil, fmt.Errorf("from endpoint: %w", err)
	}
	after, err := Collect(ctx, root, to)
	if err != nil {
		return nil, fmt.Errorf("to endpoint: %w", err)
	}
	report := &Comparison{
		SchemaVersion: 1, Qualification: Qualification, Limitation: EndpointLimit,
		From: before, To: after, Changes: make([]Change, 0),
	}
	old := make(map[string]*Item, len(before.Items))
	current := make(map[string]*Item, len(after.Items))
	names := make([]string, 0, len(before.Items)+len(after.Items))
	for index := range report.From.Items {
		item := &report.From.Items[index]
		old[item.Name] = item
		names = append(names, item.Name)
	}
	for index := range report.To.Items {
		item := &report.To.Items[index]
		current[item.Name] = item
		if old[item.Name] == nil {
			names = append(names, item.Name)
		}
	}
	slices.Sort(names)
	for _, name := range names {
		change := Change{Name: name, Before: old[name], After: current[name]}
		switch {
		case change.Before == nil:
			change.Kind = "added"
		case change.After == nil:
			change.Kind = "removed"
		default:
			change.Moved = change.Before.Bucket != change.After.Bucket
			change.StatusChanged = change.Before.Status != change.After.Status
			if !change.Moved {
				if !change.StatusChanged {
					continue
				}
			}
			change.Kind = "changed"
		}
		report.Changes = append(report.Changes, change)
	}
	return report, nil
}

// Text renders changes before both full inventories so decreases retain their cause.
func (report *Comparison) Text() string {
	var out strings.Builder
	out.WriteString("# Release queue endpoint comparison\n\n")
	out.WriteString(report.Qualification)
	out.WriteString("\n\n")
	out.WriteString(report.Limitation)
	fmt.Fprintf(&out, "\n\nFrom: %s\nTo: %s\n\n", report.From.Revision, report.To.Revision)
	for _, change := range report.Changes {
		fmt.Fprintf(&out, "- %s: %s", escape(change.Name), change.Kind)
		if change.Moved {
			fmt.Fprintf(&out, "; moved %s to %s", escape(change.Before.Bucket), escape(change.After.Bucket))
		}
		if change.StatusChanged {
			fmt.Fprintf(&out, "; status %s to %s", escape(change.Before.Status), escape(change.After.Status))
		}
		if change.Before != nil {
			fmt.Fprintf(&out, " [earlier source](%s)", change.Before.SourceURL)
		}
		if change.After != nil {
			fmt.Fprintf(&out, " [selected source](%s)", change.After.SourceURL)
		}
		out.WriteByte('\n')
	}
	if len(report.Changes) == 0 {
		out.WriteString("No additions, removals, moves, or status transitions between these endpoints.\n")
	}
	out.WriteString("\n## Earlier inventory\n\n")
	out.WriteString(Markdown(&report.From, false))
	out.WriteString("\n## Selected inventory\n\n")
	out.WriteString(Markdown(&report.To, false))
	return out.String()
}
