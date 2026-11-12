// Design: docs/contributing/rfc-conformance-gates.md -- committed audit resealing.
// Related: reseal.go -- the existing writer, whose SHIFTED-only rule applies here.
// Related: freshness.go -- the shared reader and freshness judgment.
// Related: audit_stamp.go -- the rejudgment writer also binds to this snapshot.
package rfc

import (
	"errors"
	"maps"
	"path/filepath"
	"strings"
)

// The optional source selector shared by both audit writers.
const (
	keySource       = "source"
	auditSourceHead = "head"
)

// auditSource holds the inputs one audit writer compares and writes against.
// The committed constructor seals its reader: an absent blob can never fall
// back to another session's working-tree file.
type auditSource struct {
	collected Collected
	audits    map[string]Audit
	reader    *sourceReader
	revision  string
}

func resealInputs(tree string, scope resealScope) (auditSource, error) {
	if scope.head {
		audits, err := loadAudits(tree, scope.stems)
		if err != nil {
			return auditSource{}, err
		}
		paths := map[string]bool{}
		for _, audit := range audits {
			for _, rid := range audit.Order {
				if len(scope.ids) > 0 {
					if !scope.ids[rid] {
						continue
					}
				}
				verdict, held := audit.Verdict(rid)
				if !held {
					continue
				}
				for _, key := range verdictCodeKeys(audit, rid, verdict) {
					paths[keyFile(key)] = true
				}
			}
		}
		snapshot, err := committedAuditSource(tree, scope.stems, sortedSet(paths))
		if err != nil {
			return auditSource{}, err
		}
		snapshot.audits = audits
		return snapshot, nil
	}
	collected, err := Collect(tree)
	if err != nil {
		return auditSource{}, err
	}
	audits, err := loadAudits(tree, collected.Enrolled)
	if err != nil {
		return auditSource{}, err
	}
	return auditSource{collected: collected, audits: audits, reader: newSourceReader(tree)}, nil
}

// committedAuditSource resolves HEAD once, then reads every source input by that object
// id. The caller supplies only the producer paths its operation needs: pending
// citations for a stamp, selected recorded citations for a reseal.
func committedAuditSource(tree string, stems map[string]bool, extraPaths []string) (auditSource, error) {
	raw, known := gitOutput(tree, "rev-parse", "--verify", "HEAD^{commit}")
	if !known {
		return auditSource{}, errors.New("rfc audit source: cannot resolve HEAD to a commit; repair the checkout's Git history before using source head")
	}
	revision := strings.TrimSpace(string(raw))
	const suitesRel = "internal/le/test/functional/suites.go"
	paths := []string{suitesRel}
	for _, stem := range sortedSet(stems) {
		paths = append(paths, summaryRelOf(stem))
	}
	for _, part := range [][2]string{{profileRel, ".md"}, {workflowsRel, ".yml"}, {workflowsRel, ".yaml"}} {
		found, held := gitTreePaths(tree, revision, part[0], part[1])
		if !held {
			return auditSource{}, errors.New("rfc audit source: cannot list " + part[0] + " at " + revision + "; repair the committed source before stamping")
		}
		paths = append(paths, found...)
	}
	blobs, err := resealBlobs(tree, revision, paths)
	if err != nil {
		return auditSource{}, err
	}
	profiles := map[string]string{}
	workflows := map[string]string{}
	for rel, body := range blobs {
		switch filepath.Dir(rel) {
		case profileRel:
			profiles[strings.TrimSuffix(filepath.Base(rel), ".md")] = body
		case workflowsRel:
			workflows[filepath.Base(rel)] = body
		}
	}
	parsedProfiles, problems := ParseProfiles(profiles)
	if len(problems) > 0 {
		return auditSource{}, errors.New("rfc audit source: committed profiles cannot be read: " + strings.Join(problems, "; "))
	}
	collected := Collected{Enrolled: map[string]bool{}}
	for _, stem := range sortedSet(stems) {
		rel := summaryRelOf(stem)
		meta, err := ParseMeta(blobs[rel], stem, rel)
		if err != nil {
			return auditSource{}, err
		}
		if err := checkAggregate(&meta, parsedProfiles.Aggregates, stem, rel); err != nil {
			return auditSource{}, err
		}
		if err := checkUseCases(&meta, parsedProfiles, rel); err != nil {
			return auditSource{}, err
		}
		if meta.Enrolled() {
			collected.Enrolled[stem] = true
		}
		rows, err := ParseSummaryText(blobs[rel], stem, rel)
		if err != nil {
			return auditSource{}, err
		}
		collected.Requirements = append(collected.Requirements, rows...)
	}
	suites, err := functionalSuitesFromGo(blobs[suitesRel], revision+":"+suitesRel)
	if err != nil {
		return auditSource{}, err
	}
	tags, tagBlobs, err := resealHeadTags(tree, revision, carriersFor(suites, scheduledActionsFrom(workflows)))
	if err != nil {
		return auditSource{}, err
	}
	collected.Tags, collected.GapTags = splitGapTags(tags)
	codePaths := map[string]bool{}
	for _, rel := range extraPaths {
		codePaths[rel] = true
	}
	codeBlobs, err := resealBlobs(tree, revision, sortedSet(codePaths))
	if err != nil {
		return auditSource{}, err
	}
	maps.Copy(tagBlobs, codeBlobs)
	return auditSource{collected: collected, reader: newTextReader(tagBlobs), revision: revision}, nil
}

// resealHeadTags uses the same carrier classification, exclusions and strict
// tag readers as ScanTree, with paths and contents from the pinned commit.
// Baseline readers intentionally tolerate malformed tags; a writer must not.
func resealHeadTags(tree, revision string, carriers []Carrier) ([]Tag, map[string]string, error) {
	var paths []string
	for _, root := range testRoots {
		found, known := gitTreePaths(tree, revision, root, "")
		if !known {
			return nil, nil, errors.New("rfc reseal: cannot list carriers at " + revision + "; repair the committed source before resealing")
		}
		for _, rel := range found {
			if _, held := CarrierFor(rel, carriers); !held {
				continue
			}
			skipped := false
			for part := range strings.SplitSeq(rel, "/") {
				if skipDirs[part] {
					skipped = true
					break
				}
			}
			if !skipped {
				paths = append(paths, rel)
			}
		}
	}
	blobs, err := resealBlobs(tree, revision, paths)
	if err != nil {
		return nil, nil, err
	}
	var tags []Tag
	for _, rel := range paths {
		body := blobs[rel]
		if !strings.Contains(body, tagMarker) {
			continue
		}
		carrier, held := CarrierFor(rel, carriers)
		if !held {
			return nil, nil, errors.New("rfc reseal: committed carrier " + rel + " lost its classification; repair its carrier declaration before resealing")
		}
		found, err := readerFor(carrier.Reader)(body, rel)
		if err != nil {
			return nil, nil, err
		}
		if len(found) > 0 {
			if carrier.Tier == tierUnrun {
				return nil, nil, refuseUnrun(carrier, found[0])
			}
		}
		tags = append(tags, found...)
	}
	return tags, blobs, nil
}

// resealBlobs refuses both a failed Git read and a listed path with no blob.
// An empty map is an answer only when the requested path set itself is empty.
func resealBlobs(tree, revision string, paths []string) (map[string]string, error) {
	blobs, known := gitCatBlobs(tree, revision, paths)
	if !known {
		return nil, errors.New("rfc reseal: cannot read source blobs at " + revision + "; repair the committed source before resealing")
	}
	for _, rel := range paths {
		if _, held := blobs[rel]; !held {
			return nil, errors.New("rfc reseal: " + rel + " has no source blob at " + revision + "; commit the required source before resealing")
		}
	}
	return blobs, nil
}
