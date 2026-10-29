// Design: docs/contributing/committing.md -- automatic Linux-limit approval debt
// Related: owed_trailers.go -- only the owner's attestation answers this debt
package commit

import (
	"fmt"
	"path"
	"strings"

	"github.com/ze-software/ze/internal/le/commit/hunks"
	"github.com/ze-software/ze/internal/le/rfc"
)

// linuxLimitSummaryDir is the tracked directory whose summaries carry the
// Linux-limit annotations this debt bills.
const linuxLimitSummaryDir = "rfc/short"

// linuxLimitErrorf names the approval debt in front of every refusal it
// raises, so the operator reads which commit gate refused. The prefix opens
// with the proper noun Linux, which keeps its capital.
func linuxLimitErrorf(format string, args ...any) error {
	return fmt.Errorf("Linux-limit approval: "+format, args...)
}

// linuxLimitTrailers compares the immutable blobs the script will commit with
// the HEAD Create recorded before selecting content. It MUST receive the same
// index entries and removals as commitBlock, so whole files and selected hunks
// owe approval for exactly the annotations the commit carries, never working-tree
// extras. Summary ancestors MUST be trees and summaries MUST be regular files
// in that prospective tree before any blob is parsed.
func linuxLimitTrailers(root, head string, entries, removed []string) ([]string, error) {
	if err := linuxLimitSummaryModes(root, head, entries, removed); err != nil {
		return nil, err
	}
	var trailers []string
	for _, entry := range entries {
		_, name, found := strings.Cut(entry, "\t")
		if !found {
			return nil, linuxLimitErrorf("malformed selected index entry %q", entry)
		}
		if path.Dir(name) != linuxLimitSummaryDir {
			continue
		}
		if path.Ext(name) != ".md" {
			continue
		}
		selected := hunks.Selection{name: {Entry: entry}}
		body, err := selected.Read(root, name)
		if err != nil {
			return nil, err
		}
		stem := strings.TrimSuffix(path.Base(name), ".md")
		rows, err := rfc.ParseSummaryText(string(body), stem, name)
		if err != nil {
			return nil, linuxLimitErrorf("%w", err)
		}
		before, err := linuxLimitHead(root, head, name, stem)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.Annotation == nil {
				continue
			}
			if row.Annotation.Kind != rfc.AnnotationLinuxLimit {
				continue
			}
			old := before[row.RID]
			if old != nil {
				if old.LimitID == row.Annotation.LimitID && old.Reason == row.Annotation.Reason {
					continue
				}
			}
			trailers = append(trailers, trailerLinuxLimitOwed+row.RID+": "+row.Annotation.LimitID)
		}
	}
	return trailers, nil
}

// linuxLimitSummaryModes overlays selected entries and explicit removals on
// recorded HEAD. Selected descendants establish tree ancestors, never links.
// Checking leaves alone misses a linked rfc or rfc/short whose target changes
// elsewhere. No checkout or shared-index read can establish prospective modes.
func linuxLimitSummaryModes(root, head string, entries, removed []string) error {
	replaced := make(map[string]bool, len(entries)+len(removed))
	for _, entry := range entries {
		metadata, name, found := strings.Cut(entry, "\t")
		if !found {
			return linuxLimitErrorf("malformed selected index entry %q", entry)
		}
		if err := linuxLimitSummaryMode(name, metadata); err != nil {
			return err
		}
		replaced[name] = true
		if strings.HasPrefix(name, "rfc/") {
			replaced["rfc"] = true
		}
		if strings.HasPrefix(name, linuxLimitSummaryDir+"/") {
			replaced[linuxLimitSummaryDir] = true
		}
	}
	for _, name := range removed {
		replaced[name] = true
	}
	// Query ancestors separately: Git does not descend into a symlink, unlike
	// the filesystem readers that discover and parse the summary corpus.
	for _, scope := range []string{"rfc", linuxLimitSummaryDir, linuxLimitSummaryDir + "/"} {
		listed, err := gitOutput(root, "ls-tree", "-z", head, "--", scope)
		if err != nil {
			return linuxLimitErrorf("read %s modes at %s: %w", scope, head, err)
		}
		for entry := range strings.SplitSeq(listed, "\x00") {
			if entry == "" {
				continue
			}
			metadata, name, found := strings.Cut(entry, "\t")
			if !found {
				return linuxLimitErrorf("malformed summary tree entry %q", entry)
			}
			if replaced[name] {
				continue
			}
			if err := linuxLimitSummaryMode(name, metadata); err != nil {
				return err
			}
		}
	}
	return nil
}

// linuxLimitSummaryMode refuses links both at summaries and their ancestors.
// Metadata is the prefix of either ls-tree or ls-files -s, both mode-first.
func linuxLimitSummaryMode(name, metadata string) error {
	directory := name == "rfc" || name == linuxLimitSummaryDir
	if !directory {
		if path.Dir(name) != linuxLimitSummaryDir {
			return nil
		}
		if path.Ext(name) != ".md" {
			return nil
		}
	}
	mode, _, found := strings.Cut(metadata, " ")
	if !found {
		return linuxLimitErrorf("malformed summary mode for %s: %q", name, metadata)
	}
	if directory {
		if mode == "040000" {
			return nil
		}
		return linuxLimitErrorf("%s has Git mode %s in the prospective commit, not a directory tree; "+
			"annotation approval cannot follow linked summary directories; include remove %s in this commit "+
			"and select replacement summaries under real directories", name, mode, name)
	}
	if linuxLimitRegularMode(mode) {
		return nil
	}
	return linuxLimitErrorf("%s has Git mode %s in the prospective commit, not a regular file; "+
		"annotation approval cannot follow links or other nonregular entries; replace it with a regular file "+
		"and select that summary, or include remove %s in this commit", name, mode, name)
}

func linuxLimitRegularMode(mode string) bool {
	return mode == "100644" || mode == "100755"
}

// linuxLimitHead distinguishes a new summary from unreadable history: only an
// absent tree entry or a nonregular entry supplies no earlier summary annotation.
// A git or parse failure refuses; replacing a link bills every carried annotation.
func linuxLimitHead(root, head, name, stem string) (map[string]*rfc.Annotation, error) {
	before := make(map[string]*rfc.Annotation)
	entry, err := gitOutput(root, "ls-tree", head, "--", name)
	if err != nil {
		return nil, linuxLimitErrorf("read %s at %s: %w", name, head, err)
	}
	if entry == "" {
		return before, nil
	}
	mode, _, found := strings.Cut(entry, " ")
	if !found {
		return nil, linuxLimitErrorf("malformed summary tree entry %q", entry)
	}
	if !linuxLimitRegularMode(mode) {
		// A link's blob is its target path, not a summary or evidence of prior
		// approval. The prospective entry was already checked as regular.
		return before, nil
	}
	body, err := gitOutput(root, "show", head+":"+name)
	if err != nil {
		return nil, linuxLimitErrorf("read %s at %s: %w", name, head, err)
	}
	rows, err := rfc.ParseSummaryText(body, stem, head+":"+name)
	if err != nil {
		return nil, linuxLimitErrorf("%w", err)
	}
	for _, row := range rows {
		if row.Annotation == nil {
			continue
		}
		if row.Annotation.Kind == rfc.AnnotationLinuxLimit {
			before[row.RID] = row.Annotation
		}
	}
	return before, nil
}
