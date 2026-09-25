// Design: docs/contributing/spec-workflow.md -- shared repository and site presentation.
package roadmap

import (
	"cmp"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	specpath "github.com/ze-software/ze/internal/le/spec/path"
	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// Text supplies the default CLI view; explicit pipes still receive the typed data.
func (snapshot *Snapshot) Text() string { return Markdown(snapshot, false) }

// Markdown renders a snapshot without reading the filesystem or the clock.
// localLinks selects links relative to plan/roadmap.md; site links pin each commit.
func Markdown(snapshot *Snapshot, localLinks bool) string {
	var out strings.Builder
	out.WriteString("# Release roadmap\n\nGenerated inventory; do not edit this output.\n\n")
	out.WriteString(escape(snapshot.Qualification))
	out.WriteString("\n\n<!-- roadmap-revision: ")
	out.WriteString(snapshot.Revision)
	out.WriteString(" -->\nSource revision: [")
	out.WriteString(escape(snapshot.Revision))
	out.WriteString("](https://github.com/ze-software/ze/commit/")
	out.WriteString(snapshot.Revision)
	out.WriteString(").\n\nInput digest (SHA-256): ")
	out.WriteString(escape(snapshot.InputDigest))
	out.WriteString(".\n\nPending edits appear after commit and regeneration. Counts measure spec work items, including umbrella and child files separately. They do not measure effort, completion percentage, or release readiness.\n\n")
	fmt.Fprintf(&out, "Required work items: **%d**. Nice-to-have work items: **%d**. Total: **%d**.\n\n",
		snapshot.Counts.Required, snapshot.Counts.NiceToHave, snapshot.Counts.Total)
	out.WriteString("## Status breakdown\n\n| Declared status | Work items |\n|---|---:|\n")
	statuses := make([]string, 0, len(snapshot.Counts.Statuses))
	for status := range snapshot.Counts.Statuses {
		statuses = append(statuses, status)
	}
	slices.SortFunc(statuses, func(left, right string) int {
		if order := cmp.Compare(specstatus.StatusOrder(left), specstatus.StatusOrder(right)); order != 0 {
			return order
		}
		return cmp.Compare(left, right)
	})
	for _, status := range statuses {
		fmt.Fprintf(&out, "| %s | %d |\n", escape(status), snapshot.Counts.Statuses[status])
	}
	if len(snapshot.Items) == 0 {
		out.WriteString("\nNo spec work items exist in this selected tree. This empty inventory does not establish release readiness.\n")
	}
	out.WriteString("\n## Required before release\n\n")
	group(&out, snapshot, specpath.Immediate, "Operator-visible defects and missing behavior", localLinks)
	group(&out, snapshot, specpath.PreRelease, "Other release obligations", localLinks)
	out.WriteString("## Nice-to-haves\n\nThe first release can ship without these items; no later delivery date is implied.\n\n")
	group(&out, snapshot, specpath.After, "Optional work", localLinks)
	out.WriteString("## Status legend\n\n| Status | Meaning |\n|---|---|\n" +
		"| skeleton | Captured task; design has not started |\n" +
		"| design | Research and design in progress |\n" +
		"| ready | Design complete; ready for implementation |\n" +
		"| in-progress | Implementation in progress |\n" +
		"| verification | Committed implementation awaiting independent review and closure; still open |\n" +
		"| blocked | Waiting on a declared prerequisite |\n" +
		"| deferred | Explicitly postponed; still counted |\n" +
		"| unparsed / unknown / other values | Metadata needs attention; still counted |\n\n" +
		"The report records declared status only. Phase text and checkboxes do not establish completion. Removed specs require source verification before a delivery claim.\n")
	return out.String()
}

func group(out *strings.Builder, snapshot *Snapshot, bucket, title string, localLinks bool) {
	fmt.Fprintf(out, "### %s (%d)\n\n", title, snapshot.Counts.Buckets[bucket])
	if snapshot.Counts.Buckets[bucket] == 0 {
		out.WriteString("No work items in this bucket.\n\n")
		return
	}
	out.WriteString("| Work item | Status | Updated | Phase | Dependencies | Diagnostics |\n|---|---|---|---|---|---|\n")
	for index := range snapshot.Items {
		item := &snapshot.Items[index]
		if item.Bucket != bucket {
			continue
		}
		title := item.Title
		if title == "" {
			title = "spec-" + item.Name + " (title unavailable)"
		}
		date := item.Updated
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			date = "unavailable"
		}
		fmt.Fprintf(out, "| [%s](%s) (%s) | %s | %s | %s | %s | %s |\n",
			escape(title), itemLink(item, localLinks), escape("spec-"+item.Name),
			escape(item.Status), escape(date), escape(item.Phase),
			dependencies(snapshot, item.Depends, localLinks), escape(strings.Join(item.Diagnostics, "; ")))
	}
	out.WriteByte('\n')
}

func itemLink(item *Item, localLinks bool) string {
	if localLinks {
		return escapedPath(strings.TrimPrefix(item.Path, specpath.Root+"/"))
	}
	return item.SourceURL
}

var dependencyRE = regexp.MustCompile(`(?:plan/(?:immediate/|pre-release/)?)?spec-[a-zA-Z0-9][a-zA-Z0-9._-]*\.md`)

func dependencies(snapshot *Snapshot, declared string, localLinks bool) string {
	var out strings.Builder
	previous := 0
	for _, match := range dependencyRE.FindAllStringIndex(declared, -1) {
		out.WriteString(escape(declared[previous:match[0]]))
		reference := declared[match[0]:match[1]]
		var target *Item
		boundary := true
		if match[0] > 0 {
			boundary = !referenceByte(declared[match[0]-1])
		}
		if match[1] < len(declared) {
			if referenceByte(declared[match[1]]) {
				boundary = false
			}
		}
		if boundary {
			for index := range snapshot.Items {
				item := &snapshot.Items[index]
				candidate := path.Base(item.Path)
				if strings.Contains(reference, "/") {
					candidate = item.Path
				}
				if candidate != reference {
					continue
				}
				if target != nil {
					target = nil
					break
				}
				target = item
			}
		}
		if target == nil {
			out.WriteString(escape(reference))
		} else {
			fmt.Fprintf(&out, "[%s](%s)", escape(reference), itemLink(target, localLinks))
		}
		previous = match[1]
	}
	out.WriteString(escape(declared[previous:]))
	return out.String()
}

func referenceByte(value byte) bool {
	if value >= 'a' {
		if value <= 'z' {
			return true
		}
	}
	if value >= 'A' {
		if value <= 'Z' {
			return true
		}
	}
	if value >= '0' {
		if value <= '9' {
			return true
		}
	}
	return strings.ContainsRune("/._-", rune(value))
}

// Entities keep metadata literal in Markdown cells and in the site's HTML renderer.
func escape(text string) string {
	var out strings.Builder
	for _, char := range text {
		switch char {
		case '&', '<', '>', '|', '[', ']', '(', ')', '\\', '`', '*', '_', '{', '}', '!', '#', '~':
			fmt.Fprintf(&out, "&#%d;", char)
		case '\n', '\r', '\t':
			out.WriteByte(' ')
		default:
			out.WriteRune(char)
		}
	}
	return out.String()
}
