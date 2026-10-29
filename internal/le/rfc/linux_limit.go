// Design: docs/contributing/rfc-conformance-gates.md -- the linux-limit annotation
// Related: ../linuxlimit/parse.go -- the shared register parser
package rfc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/le/linuxlimit"
)

const linuxLimitRegisterRel = "docs/architecture/linux-limitations.md"
const linuxLimitKernelRel = "internal/appliance/kernel.version"
const linuxLimitFormat = "Format: {linux-limit: LL-<ID>; <why no Ze code substitutes>}"

// LinuxLimitCandidate is evidence for an owner decision, never an annotation or approval.
type LinuxLimitCandidate struct {
	RID     string `json:"rid"`
	LimitID string `json:"limit-id"`
	Where   string `json:"where"`
}

func parseLinuxLimit(rest, where string) (*Annotation, error) {
	id, reason, separated := strings.Cut(rest, ";")
	id, reason = strings.TrimSpace(id), strings.TrimSpace(reason)
	if !separated {
		return nil, fmt.Errorf("%s: linux-limit needs a register id and a reason. %s", where, linuxLimitFormat)
	}
	if !linuxlimit.ValidID(id) {
		return nil, fmt.Errorf("%s: linux-limit has invalid register id %q. %s", where, id, linuxLimitFormat)
	}
	if reason == "" {
		return nil, fmt.Errorf("%s: linux-limit needs a reason why no Ze code substitutes. %s", where, linuxLimitFormat)
	}
	return &Annotation{Kind: AnnotationLinuxLimit, LimitID: id, Reason: reason}, nil
}

// checkLinuxLimit judges every annotation, enrolled or not. A register without
// annotations is optional: eligible entries can propose candidates, but cannot
// introduce a dependency or force an owner-reserved classification on old corpora.
func checkLinuxLimit(tree string, requirements []Requirement) ([]Finding, []LinuxLimitCandidate) {
	entries, kernel, problem := readLinuxLimitRegister(tree)
	var findings []Finding
	var candidates []LinuxLimitCandidate
	for i := range requirements {
		req := &requirements[i]
		if req.Annotation == nil || req.Annotation.Kind != AnnotationLinuxLimit {
			if problem == "" {
				for j := range entries {
					entry := &entries[j]
					if linuxLimitEntryRefusal(entry, req.RID, kernel) == "" {
						candidates = append(candidates, LinuxLimitCandidate{RID: req.RID, LimitID: entry.ID, Where: requirementWhere(*req)})
					}
				}
			}
			continue
		}
		issue := problem
		if issue == "" {
			index := -1
			for j := range entries {
				if entries[j].ID == req.Annotation.LimitID {
					index = j
					break
				}
			}
			if index < 0 {
				issue = "has no entry " + req.Annotation.LimitID + "; name an existing verified kernel blocks entry or keep {gap}"
			} else {
				issue = linuxLimitEntryRefusal(&entries[index], req.RID, kernel)
			}
		}
		if issue != "" {
			message := requirementWhere(*req) + ": " + req.RID + " {linux-limit: " + req.Annotation.LimitID + "} in " + linuxLimitRegisterRel + ": " + issue
			findings = append(findings, requirementFinding(*req, issue, message))
		}
	}
	return findings, candidates
}

// readLinuxLimitRegister returns the textual entries and shipped kernel pin.
// Read and parse failures remain explicit; checkLinuxLimit decides which rows they refuse.
func readLinuxLimitRegister(tree string) ([]linuxlimit.Entry, string, string) {
	page, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(linuxLimitRegisterRel))) //nolint:gosec // a fixed register path under the checkout root the caller names
	if err != nil {
		return nil, "", "cannot read " + linuxLimitRegisterRel + ": " + err.Error() + "; restore the register before claiming a Linux limit"
	}
	entries, problems := linuxlimit.Parse(string(page))
	if len(problems) != 0 {
		return nil, "", "malformed register: " + strings.Join(problems, "; ")
	}
	kernel, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(linuxLimitKernelRel))) //nolint:gosec // a fixed kernel pin path under the checkout root the caller names
	if err != nil {
		return nil, "", "cannot read " + linuxLimitKernelRel + ": " + err.Error() + "; restore the shipped kernel version before claiming a Linux limit"
	}
	version := strings.TrimSpace(string(kernel))
	if version == "" {
		return nil, "", linuxLimitKernelRel + " is empty; restore the shipped kernel version"
	}
	return entries, version, ""
}

func linuxLimitEntryRefusal(entry *linuxlimit.Entry, rid, kernel string) string {
	if entry.Kind != "kernel" {
		return "entry " + entry.ID + " Kind is " + entry.Kind + "; a library limit is fixed by a carried diff, so keep the row a {gap}"
	}
	if entry.Effect != "blocks" {
		return "entry " + entry.ID + " Effect is " + entry.Effect + "; Ze can implement it in another shape, so keep the row a {gap}"
	}
	if entry.Status != "verified" {
		return "entry " + entry.ID + " Status is " + entry.Status + "; settle its Re-check before annotating: " + entry.ReCheck
	}
	if entry.Kernel != kernel {
		return "entry " + entry.ID + " Checked names Linux " + entry.Kernel + ", but " + linuxLimitKernelRel + " ships " + kernel + "; re-check the entry at the shipped version and update Checked"
	}
	if !linuxLimitBlocks(entry.Blocks, rid) {
		return "entry " + entry.ID + " Blocks does not name the exact requirement " + rid + "; reconcile the row with Blocks (" + entry.Blocks + ") or keep {gap}"
	}
	if strings.TrimSpace(entry.Evidence) == "" {
		return "entry " + entry.ID + " Evidence is empty for " + rid + "; record the source file, function and quoted line before annotating"
	}
	return ""
}

// linuxLimitBlocks compares complete identifier tokens, never an RID prefix.
func linuxLimitBlocks(blocks, rid string) bool {
	for _, token := range strings.FieldsFunc(blocks, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' && r != '.'
	}) {
		// A sentence-ending period is prose; dots inside a section remain part
		// of the identifier, and a longer ordinal or suffix never matches.
		if strings.TrimRight(token, ".") == rid {
			return true
		}
	}
	return false
}
