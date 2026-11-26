// Design: docs/architecture/core-design.md -- service block registration for start-time isolation
// Related: loader.go -- LoadConfigForStart drops invalid registered service blocks
// Related: tree_validators.go -- scoped tree validators attach to a service block

package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const serviceBlockWildcardSuffix = "[*]"

// ExcludedBlock records one service config block that LoadConfigForStart
// dropped from the in-memory tree because its configuration could not be
// served, so the daemon starts without that service and reports it.
type ExcludedBlock struct {
	Path    string // concrete tree path removed, e.g. "environment/web" or "bus/hub/server[main]"
	Service string // human service name, e.g. "web server"
	Cause   error  // the validation refusal that excluded the block
}

type serviceBlock struct {
	path     string
	prefix   string
	wildcard bool
	service  string
}

var serviceBlocks struct {
	sync.RWMutex
	entries []serviceBlock
}

// RegisterServiceBlock registers a service configuration block that
// LoadConfigForStart may drop from the in-memory tree when validation fails
// beneath it. Callers MUST register during init.
//
// path names either a fixed subtree (e.g. "environment/web") or a list-entry
// wildcard ending in "[*]" (e.g. "bus/hub/server[*]"). Any leading "/" is
// stripped before storing. service is the human-readable service name reported
// in ExcludedBlock.Service. Safe for concurrent use.
func RegisterServiceBlock(path, service string) error {
	normalized := strings.TrimLeft(path, PathSep)
	if normalized == "" {
		return errors.New("service block registration requires a non-empty path")
	}
	if strings.TrimSpace(service) == "" {
		return fmt.Errorf("service block %q registration requires a non-empty service name", normalized)
	}

	wildcard := strings.HasSuffix(normalized, serviceBlockWildcardSuffix)
	prefix := normalized
	if wildcard {
		prefix = strings.TrimSuffix(normalized, serviceBlockWildcardSuffix)
		if prefix == "" {
			return fmt.Errorf("service block %q wildcard requires a list path prefix", normalized)
		}
		if strings.HasSuffix(prefix, PathSep) {
			return fmt.Errorf("service block %q wildcard must attach to a list name", normalized)
		}
	}

	serviceBlocks.Lock()
	defer serviceBlocks.Unlock()
	for _, existing := range serviceBlocks.entries {
		if existing.path == normalized {
			return fmt.Errorf("service block %q is already registered", normalized)
		}
	}
	serviceBlocks.entries = append(serviceBlocks.entries, serviceBlock{
		path:     normalized,
		prefix:   prefix,
		wildcard: wildcard,
		service:  service,
	})
	return nil
}

// matchServiceBlock resolves errPath against all registered service blocks and
// returns the longest matching concrete tree path and service name.
//
// A fixed registration matches errPath exactly or at a "/" or "[" boundary.
// A wildcard registration "prefix[*]" matches "prefix[key]" or
// "prefix[key]/..." and returns concretePath "prefix[key]" with service
// "<reg.service> <key>".
func matchServiceBlock(errPath string) (concretePath, service string, matched bool) {
	normalized := strings.TrimLeft(errPath, PathSep)
	if normalized == "" {
		return "", "", false
	}

	serviceBlocks.RLock()
	defer serviceBlocks.RUnlock()

	bestLen := -1
	bestWildcard := false
	for _, reg := range serviceBlocks.entries {
		candPath, candService, ok := matchOneServiceBlock(normalized, reg)
		if !ok {
			continue
		}
		if len(candPath) > bestLen {
			bestLen = len(candPath)
			bestWildcard = reg.wildcard
			concretePath = candPath
			service = candService
			matched = true
			continue
		}
		if len(candPath) == bestLen {
			if !reg.wildcard && bestWildcard {
				bestWildcard = false
				concretePath = candPath
				service = candService
				matched = true
			}
		}
	}
	return concretePath, service, matched
}

func matchOneServiceBlock(errPath string, reg serviceBlock) (concretePath, service string, matched bool) {
	if reg.wildcard {
		return matchWildcardServiceBlock(errPath, reg)
	}
	if errPath == reg.path {
		return reg.path, reg.service, true
	}
	if !strings.HasPrefix(errPath, reg.path) {
		return "", "", false
	}
	next := errPath[len(reg.path)]
	if next == '/' {
		return reg.path, reg.service, true
	}
	if next == '[' {
		return reg.path, reg.service, true
	}
	return "", "", false
}

func matchWildcardServiceBlock(errPath string, reg serviceBlock) (concretePath, service string, matched bool) {
	if !strings.HasPrefix(errPath, reg.prefix) {
		return "", "", false
	}
	rest := errPath[len(reg.prefix):]
	if len(rest) == 0 {
		return "", "", false
	}
	if rest[0] != '[' {
		return "", "", false
	}
	closeIdx := strings.IndexByte(rest[1:], ']')
	if closeIdx <= 0 {
		return "", "", false
	}
	key := rest[1 : 1+closeIdx]
	after := rest[2+closeIdx:]
	if after != "" {
		if after[0] != '/' {
			return "", "", false
		}
	}
	var pathBuf textbuf.Buffer
	concretePath = pathBuf.Str(reg.prefix).Byte('[').Str(key).Byte(']').String()
	var svcBuf textbuf.Buffer
	service = svcBuf.Str(reg.service).Byte(' ').Str(key).String()
	return concretePath, service, true
}

// snapshotServiceBlocks saves the current service block registry and returns a
// function that restores it, so unit tests never leak registrations.
func snapshotServiceBlocks() func() {
	serviceBlocks.Lock()
	saved := slices.Clone(serviceBlocks.entries)
	serviceBlocks.Unlock()
	return func() {
		serviceBlocks.Lock()
		serviceBlocks.entries = saved
		serviceBlocks.Unlock()
	}
}

const maxPrunePasses = 16

type pendingExclusion struct {
	path    string
	service string
	causes  []error
}

// excludeInvalidServiceBlocks validates tree for daemon startup. Every failure
// scoped to a registered service block (or a scoped original-tree validator)
// removes only that concrete block from tree and records it in ExcludedBlock.
// Any unscoped failure, a block whose path cannot be removed, or a tree that
// still fails after maxPrunePasses returns the original strict validation error.
func excludeInvalidServiceBlocks(tree *Tree) ([]ExcludedBlock, error) {
	var excluded []ExcludedBlock
	var originalErr error

	for range maxPrunePasses {
		treeFailures := validateOriginalTreeAll(tree)
		sectionFailures, err := validateCustomSectionsSchema(tree)
		if err != nil {
			return nil, err
		}
		if len(treeFailures) == 0 {
			if len(sectionFailures) == 0 {
				return excluded, nil
			}
		}
		if originalErr == nil {
			originalErr = strictValidationError(treeFailures, sectionFailures)
		}

		pending, scoped := groupScopedFailures(treeFailures, sectionFailures)
		if !scoped {
			return nil, originalErr
		}

		for _, entry := range pending {
			if !tree.RemovePath(entry.path) {
				return nil, originalErr
			}
			excluded = append(excluded, ExcludedBlock{
				Path:    entry.path,
				Service: entry.service,
				Cause:   joinExclusionCauses(entry.causes),
			})
		}
	}

	treeFailures := validateOriginalTreeAll(tree)
	sectionFailures, err := validateCustomSectionsSchema(tree)
	if err != nil {
		return nil, err
	}
	if len(treeFailures) == 0 {
		if len(sectionFailures) == 0 {
			return excluded, nil
		}
	}
	return nil, originalErr
}

func strictValidationError(treeFailures []treeValidatorFailure, sectionFailures []SectionValidationError) error {
	if len(treeFailures) > 0 {
		return fmt.Errorf("%w: %w", ErrCustomValidation, treeFailures[0].err)
	}
	return joinSectionRefusals(sectionFailures)
}

func groupScopedFailures(treeFailures []treeValidatorFailure, sectionFailures []SectionValidationError) ([]*pendingExclusion, bool) {
	byPath := make(map[string]*pendingExclusion)
	add := func(blockPath, service string, cause error) {
		if existing, ok := byPath[blockPath]; ok {
			existing.causes = append(existing.causes, cause)
			return
		}
		byPath[blockPath] = &pendingExclusion{
			path:    blockPath,
			service: service,
			causes:  []error{cause},
		}
	}

	for _, f := range treeFailures {
		if f.blockPath == "" {
			return nil, false
		}
		service := f.name
		if _, matchedService, matched := matchServiceBlock(f.blockPath); matched {
			service = matchedService
		}
		add(f.blockPath, service, f.err)
	}

	for _, ve := range sectionFailures {
		concretePath, service, matched := matchServiceBlock(ve.Err.Path)
		if !matched {
			return nil, false
		}
		// Use SectionValidationError.Message so sensitive leaf values stay redacted.
		add(concretePath, service, errors.New(ve.Message()))
	}

	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	slices.Sort(paths)

	kept := make([]*pendingExclusion, 0, len(paths))
	for _, p := range paths {
		entry := byPath[p]
		merged := false
		for _, parent := range kept {
			if isAncestorPath(parent.path, p) {
				parent.causes = append(parent.causes, entry.causes...)
				merged = true
				break
			}
		}
		if !merged {
			kept = append(kept, entry)
		}
	}
	return kept, true
}

func isAncestorPath(parent, child string) bool {
	if len(child) <= len(parent) {
		return false
	}
	if !strings.HasPrefix(child, parent) {
		return false
	}
	next := child[len(parent)]
	return next == '/' || next == '['
}

func joinExclusionCauses(causes []error) error {
	if len(causes) == 1 {
		return causes[0]
	}
	return errors.Join(causes...)
}
