// Design: docs/architecture/plugin/feature-gates.md -- complete external static-entry certificates.
package repoinventory

import (
	"fmt"
	"slices"
	"strings"
)

type externalEntryRoute uint8

const (
	externalEntryUnspecified externalEntryRoute = iota
	externalEntryCycles
	externalEntryPublic
)

// externalEntrySet is constructed only by entrySet on a fully selected
// externalSource. Its initializer closure retains internal and vendor imports.
type externalEntrySet struct {
	route        externalEntryRoute
	packages     []string
	initializers []string
	origins      []string
}

// entrySet certifies one exact origin after newExternalSource has completed
// module, provider and import-graph validation. Caches belong to that reader.
func (s *externalSource) entrySet(identity string) (externalEntrySet, error) {
	if entries, found := s.entries[identity]; found {
		return entries, nil
	}
	route := externalEntryCycles
	for _, sink := range []string{registrationPackage, "internal/component/config/yang"} {
		if !s.reaches(sink, identity) {
			route = externalEntryPublic
		}
	}
	if route == externalEntryPublic {
		if err := s.publicEntryOrigin(identity); err != nil {
			return externalEntrySet{}, err
		}
	}
	entries := externalEntrySet{route: route}
	for directory := range s.snapshot.packages {
		if route == externalEntryPublic {
			if externalParentInternal(directory) {
				continue
			}
		}
		if s.reaches(directory, identity) {
			continue
		}
		entries.packages = append(entries.packages, directory)
	}
	slices.Sort(entries.packages)
	seen := make(map[string]bool)
	origins := make(map[string]bool)
	pending := slices.Clone(entries.packages)
	for len(pending) != 0 {
		directory := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[directory] {
			continue
		}
		seen[directory] = true
		if !s.roots[directory] {
			entries.initializers = append(entries.initializers, directory)
		}
		for _, imported := range s.imports[directory] {
			target := externalPotentialImportDirectory(imported)
			if target == "" {
				if imported != "unsafe" {
					origins[imported] = true
				}
				continue
			}
			if _, retained := s.snapshot.packages[target]; !retained {
				return externalEntrySet{}, fmt.Errorf("external entry identity %s imports unavailable retained package %s; repair committed provider selection", directory, imported)
			}
			pending = append(pending, target)
		}
	}
	slices.Sort(entries.initializers)
	for origin := range origins {
		entries.origins = append(entries.origins, origin)
	}
	slices.Sort(entries.origins)
	s.entries[identity] = entries
	return entries, nil
}

func (s *externalSource) publicEntryOrigin(identity string) error {
	parent := strings.TrimSuffix(modulePrefix, "/")
	if identity == "" {
		return fmt.Errorf("external entry identity is empty; retain an exact outside origin")
	}
	if identity == parent {
		return fmt.Errorf("external entry identity %s is inside the parent namespace; retain a cycle certificate", identity)
	}
	if strings.HasPrefix(identity, modulePrefix) {
		return fmt.Errorf("external entry identity %s is inside the parent namespace; retain a cycle certificate", identity)
	}
	for _, sink := range []string{registrationPackage, "internal/component/config/yang"} {
		if !externalParentInternal(sink) {
			return fmt.Errorf("external registration sink %s lacks a parent internal boundary; retain a cycle certificate", sink)
		}
		if _, retained := s.snapshot.packages[sink]; !retained {
			return fmt.Errorf("external registration sink identity %s is unavailable; retain both sink providers", sink)
		}
	}
	return nil
}

// externalParentInternal never excludes vendor internals. Their permitted
// hidden importers are not covered by the closed parent-namespace theorem.
func externalParentInternal(directory string) bool {
	if strings.HasPrefix(directory, "vendor/") {
		return false
	}
	for _, segment := range strings.Split(directory, "/") {
		if segment == "internal" {
			return true
		}
	}
	return false
}
