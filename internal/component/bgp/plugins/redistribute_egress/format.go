// Design: docs/architecture/core-design.md -- canonical update-route command shape

package redistributeegress

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
)

const originIncomplete = "incomplete"

func formatAnnounce(family string, entry *redistevents.RouteChangeEntry) string {
	var sb strings.Builder
	sb.Grow(80)

	sb.WriteString("update text origin ")
	sb.WriteString(originIncomplete)

	sb.WriteString(" nhop ")
	if entry.NextHop.IsValid() {
		sb.WriteString(entry.NextHop.String())
	} else {
		sb.WriteString("self")
	}

	sb.WriteString(" nlri ")
	sb.WriteString(family)
	sb.WriteString(" add ")
	sb.WriteString(entry.Prefix.String())

	return sb.String()
}

func formatWithdraw(family string, entry *redistevents.RouteChangeEntry) string {
	var sb strings.Builder
	sb.Grow(64)

	sb.WriteString("update text nlri ")
	sb.WriteString(family)
	sb.WriteString(" del ")
	sb.WriteString(entry.Prefix.String())

	return sb.String()
}
