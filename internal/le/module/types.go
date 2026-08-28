// Design: plan/spec-le-is-a-ze-binary.md -- native development-tool actions
package module

import (
	"fmt"
	"strings"
)

const area = "module"

// CountedPath names a text file and the number of matching occurrences it holds.
type CountedPath struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// SkippedPath is a tracked occurrence deliberately left unchanged.
type SkippedPath struct {
	Path   string `json:"path"`
	Count  int    `json:"count"`
	Reason string `json:"reason"`
}

// PathMove is one directory relocation.
type PathMove struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// PluginDirs reports the native plugin generator's search-root edit.
type PluginDirs struct {
	Before  []string `json:"before"`
	After   []string `json:"after"`
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
}

// RegistrationDelta proves that the move did not lose a generated registration.
type RegistrationDelta struct {
	Dropped   []string `json:"dropped"`
	Added     []string `json:"added"`
	Preserved bool     `json:"preserved"`
}

// MoveReport is the structured answer to `le module move`.
type MoveReport struct {
	Apply         bool              `json:"apply"`
	Source        string            `json:"source"`
	Destination   string            `json:"destination"`
	Merge         bool              `json:"merge"`
	Conflicts     []string          `json:"conflicts"`
	ImportEdits   []CountedPath     `json:"import-edits"`
	PluginDirs    PluginDirs        `json:"plugin-dirs"`
	RPCPackages   []string          `json:"rpc-packages"`
	RPCHazard     bool              `json:"rpc-hazard"`
	Residual      []CountedPath     `json:"residual"`
	Goimports     string            `json:"goimports"`
	GeneratorCode int               `json:"generator-code"`
	Registrations RegistrationDelta `json:"registrations"`
	Code          int               `json:"code"`
}

// Text renders a deterministic operator report while pipe renderers retain the
// complete structured answer.
func (r *MoveReport) Text() string {
	var out strings.Builder
	mode := "DRY RUN (no changes)"
	if r.Apply {
		mode = "APPLYING"
	}
	fmt.Fprintf(&out, "%s move %s  ->  %s\n", mode, r.Source, r.Destination)
	if r.Merge {
		fmt.Fprintf(&out, "filesystem merge: %t\n", r.Merge)
	}
	writeCounted(&out, "import rewrites", r.ImportEdits)
	if len(r.PluginDirs.Removed) > 0 || len(r.PluginDirs.Added) > 0 {
		out.WriteString("plugin discovery roots:\n")
		for _, rel := range r.PluginDirs.Removed {
			fmt.Fprintf(&out, "  - %s\n", rel)
		}
		for _, rel := range r.PluginDirs.Added {
			fmt.Fprintf(&out, "  + %s\n", rel)
		}
	}
	if len(r.Conflicts) > 0 {
		out.WriteString("REFUSED file collisions:\n")
		for _, rel := range r.Conflicts {
			fmt.Fprintf(&out, "  ! %s\n", rel)
		}
	}
	if len(r.RPCPackages) > 0 {
		fmt.Fprintf(&out, "RPC packages (hazard=%t):\n", r.RPCHazard)
		for _, rel := range r.RPCPackages {
			fmt.Fprintf(&out, "  %s\n", rel)
		}
	}
	writeCounted(&out, "residual references", r.Residual)
	switch {
	case !r.Apply:
		out.WriteString("dry run -- nothing changed. Re-run with apply.\n")
	case r.Code != 0 && r.Goimports == goimportsNotRun:
		fmt.Fprintf(&out, "REFUSED before mutation (code %d).\n", r.Code)
	default:
		fmt.Fprintf(&out, "generator exit: %d\ngoimports: %s\n", r.GeneratorCode, r.Goimports)
		for _, path := range r.Registrations.Dropped {
			fmt.Fprintf(&out, "generated registration dropped: %s\n", path)
		}
		for _, path := range r.Registrations.Added {
			fmt.Fprintf(&out, "generated registration added: %s\n", path)
		}
		if r.Registrations.Preserved {
			out.WriteString("generated registration set preserved (0 dropped).\n")
		}
		if r.Code == 0 {
			out.WriteString("next: le tier check\n")
		} else {
			fmt.Fprintf(&out, "FAILED after mutation (code %d).\n", r.Code)
		}
	}
	return out.String()
}

// RenameReport is the structured answer to `le module rename`.
type RenameReport struct {
	Old           string        `json:"old"`
	New           string        `json:"new"`
	Apply         bool          `json:"apply"`
	Limit         int           `json:"-"`
	Occurrences   int           `json:"occurrences"`
	Edits         []CountedPath `json:"edits"`
	Moves         []PathMove    `json:"moves"`
	Regenerate    []CountedPath `json:"regenerate"`
	Skipped       []SkippedPath `json:"skipped"`
	ChangedFiles  int           `json:"changed-files"`
	MovedDirs     int           `json:"moved-dirs"`
	Goimports     string        `json:"goimports"`
	Left          []CountedPath `json:"left"`
	Resealed      []string      `json:"resealed"`
	ResealRefused []string      `json:"reseal-refused"`
	ResidualHost  []CountedPath `json:"residual-host"`
	Code          int           `json:"code"`
}

// Text renders the old producer's report from the structured data.
func (r *RenameReport) Text() string {
	var out strings.Builder
	fmt.Fprintf(&out, "rename %s\n    -> %s\n", r.Old, r.New)
	fmt.Fprintf(&out, "%d occurrence(s) in %d file(s), %d directory move(s)\n", r.Occurrences, len(r.Edits), len(r.Moves))
	writeCountedLimited(&out, "rewrite", r.Edits, r.Limit)
	if len(r.Moves) > 0 {
		out.WriteString("move:\n")
		limit := limitedLength(len(r.Moves), r.Limit)
		for _, move := range r.Moves[:limit] {
			fmt.Fprintf(&out, "  %s  ->  %s\n", move.From, move.To)
		}
		writeMore(&out, len(r.Moves)-limit)
	}
	writeCountedLimited(&out, "REGENERATE (length-prefixed, not rewritten)", r.Regenerate, r.Limit)
	if len(r.Skipped) > 0 {
		out.WriteString("skipped (reported, not rewritten):\n")
		limit := limitedLength(len(r.Skipped), r.Limit)
		for _, row := range r.Skipped[:limit] {
			fmt.Fprintf(&out, "  %s  %d  %s\n", row.Path, row.Count, row.Reason)
		}
		writeMore(&out, len(r.Skipped)-limit)
	}
	if !r.Apply {
		out.WriteString("dry run -- nothing changed. Re-run with apply.\n")
		return out.String()
	}
	if r.Code != 0 && r.ChangedFiles == 0 && r.MovedDirs == 0 && r.Goimports == goimportsNotRun {
		fmt.Fprintf(&out, "REFUSED before mutation (code %d).\n", r.Code)
		return out.String()
	}
	fmt.Fprintf(&out, "rewrote %d file(s)\nmoved %d director(y|ies)\ngoimports: %s\n", r.ChangedFiles, r.MovedDirs, r.Goimports)
	if r.Code != 0 {
		fmt.Fprintf(&out, "FAILED after mutation (code %d).\n", r.Code)
	}
	writeCountedLimited(&out, "STILL CONTAIN THE OLD MODULE PATH", r.Left, r.Limit)
	if len(r.Regenerate) > 0 {
		out.WriteString("regenerate these, then verify:\n")
		for _, row := range r.Regenerate {
			fmt.Fprintf(&out, "  %s\n", row.Path)
		}
		out.WriteString("  ./le setup proto-generate\n")
	}
	if len(r.Resealed) > 0 {
		out.WriteString("re-sealed RFC audit verdicts:\n")
		limit := limitedLength(len(r.Resealed), r.Limit)
		for _, row := range r.Resealed[:limit] {
			fmt.Fprintf(&out, "  %s\n", row)
		}
		writeMore(&out, len(r.Resealed)-limit)
	}
	if len(r.ResealRefused) > 0 {
		out.WriteString("RFC audit verdicts REFUSED (left stale):\n")
		limit := limitedLength(len(r.ResealRefused), r.Limit)
		for _, row := range r.ResealRefused[:limit] {
			fmt.Fprintf(&out, "  %s\n", row)
		}
		writeMore(&out, len(r.ResealRefused)-limit)
	}
	writeCountedLimited(&out, "residual old-host references", r.ResidualHost, r.Limit)
	return out.String()
}

func writeCounted(out *strings.Builder, title string, rows []CountedPath) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(out, "%s (%d):\n", title, len(rows))
	for _, row := range rows {
		fmt.Fprintf(out, "  %d  %s\n", row.Count, row.Path)
	}
}

func writeCountedLimited(out *strings.Builder, title string, rows []CountedPath, requested int) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(out, "%s (%d):\n", title, len(rows))
	limit := limitedLength(len(rows), requested)
	for _, row := range rows[:limit] {
		fmt.Fprintf(out, "  %d  %s\n", row.Count, row.Path)
	}
	writeMore(out, len(rows)-limit)
}

func limitedLength(length, requested int) int {
	if requested < 0 {
		return 0
	}
	if requested < length {
		return requested
	}
	return length
}

func writeMore(out *strings.Builder, count int) {
	if count > 0 {
		fmt.Fprintf(out, "  ... and %d more\n", count)
	}
}

// The Go file suffix, the component tree this mover special-cases, and the
// goimports state before it runs.
const (
	goSuffix        = ".go"
	componentTree   = "internal/component"
	goimportsNotRun = "not run"
)

// coreTree is the third source area this mover moves packages between.
const (
	coreTree = "internal/core"
)

// moduleDirective is the go.mod line that declares the module path.
const moduleDirective = "module"

// pluginsTree is the plugin source area this mover moves packages between.
const (
	pluginsTree = "internal/plugins"
)
