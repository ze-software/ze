// Design: website/AI.md -- compact labels distinguish software from evidence.
package site

import "strings"

// Concept classes describe a label's meaning, never support, maturity or health.
const (
	conceptSoftware = "concept-software"
	conceptEvidence = "concept-evidence"
)

// compactConcepts is the curated vocabulary for chips and sidebar labels only.
// Exact names keep protocols, commands, capability claims and mixed labels
// neutral. This is a presentation vocabulary, not an inventory of Ze support.
var compactConcepts = map[string]string{
	"bird":            conceptSoftware,
	"birdwatcher":     conceptSoftware,
	"bio-rd":          conceptSoftware,
	"bgp-performance": conceptSoftware,
	"cisco ios xr":    conceptSoftware,
	"compose":         conceptSoftware,
	"containerlab":    conceptSoftware,
	"docker":          conceptSoftware,
	"dpdk":            conceptSoftware,
	"exabgp":          conceptSoftware,
	"fd.io vpp":       conceptSoftware,
	"freertr":         conceptSoftware,
	"frr":             conceptSoftware,
	"frrouting":       conceptSoftware,
	"gobgp":           conceptSoftware,
	"gokrazy":         conceptSoftware,
	"gokrazy image":   conceptSoftware,
	"govpp":           conceptSoftware,
	"htmx":            conceptSoftware,
	"juniper junos":   conceptSoftware,
	"netdata":         conceptSoftware,
	"netlab":          conceptSoftware,
	"nftables":        conceptSoftware,
	"nokia sr os":     conceptSoftware,
	"openbgpd":        conceptSoftware,
	"prometheus":      conceptSoftware,
	"rustbgpd":        conceptSoftware,
	"rustybgp":        conceptSoftware,
	"systemd":         conceptSoftware,
	"vyos":            conceptSoftware,

	"benchmarking guide":       conceptEvidence,
	"browser and editor tests": conceptEvidence,
	"evidence":                 conceptEvidence,
	"functional ci":            conceptEvidence,
	"fuzz":                     conceptEvidence,
	"interop":                  conceptEvidence,
	"lab evidence":             conceptEvidence,
	"performance":              conceptEvidence,
	"performance results":      conceptEvidence,
	"quality":                  conceptEvidence,
	"quality evidence":         conceptEvidence,
	"release evidence":         conceptEvidence,
	"rfc compliance gate":      conceptEvidence,
	"testing health":           conceptEvidence,
	"unit, fuzz, mutation":     conceptEvidence,
}

// compactLabelClass adds a role only when the complete label has been curated.
// Unclassified labels intentionally retain their existing neutral class.
func compactLabelClass(base, label string) string {
	if concept, known := compactConcepts[strings.ToLower(label)]; known {
		return base + " " + concept
	}
	return base
}
