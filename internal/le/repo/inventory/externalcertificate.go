// Design: docs/architecture/plugin/feature-gates.md -- complete closure before publication.
package repoinventory

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// externalProof keeps diagnostic dispatch separate from publication authority.
// A nonnil refusal MUST prevent a reader from escaping readRegistrations.
type externalProof struct {
	sites   map[externalCallSite]bool
	refusal error
}

// externalCertificate is shared by both public report consumers through
// readRegistrations. An incomplete closure may support private diagnostics,
// but its saved refusal prevents either public consumer from publishing it.
func (s *Snapshot) externalCertificate() (externalProof, error) {
	needed := false
	for _, pkg := range s.packages {
		for _, file := range pkg.files {
			for _, imported := range file.File.Imports {
				identity, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return externalProof{}, err
				}
				if strings.HasPrefix(identity, modulePrefix) {
					continue
				}
				first, _, _ := strings.Cut(identity, "/")
				if !strings.Contains(first, ".") {
					needed = true
				}
			}
		}
	}
	if !needed {
		return externalProof{}, nil
	}
	universe, err := newExternalSource(s)
	if err != nil {
		return externalProof{}, err
	}
	analysis := newExternalEffects(universe)
	if err := analysis.solve(); err != nil {
		var issue *externalIssue
		if !errors.As(err, &issue) {
			return externalProof{}, err
		}
		if issue.kind != externalUnresolved {
			return externalProof{}, err
		}
		sites := make(map[externalCallSite]bool, len(analysis.operations))
		for site := range analysis.operations {
			sites[site] = true
		}
		return externalProof{sites: sites, refusal: err}, nil
	}
	return externalProof{sites: analysis.admitted}, nil
}

// Plugins reports the committed Linux le population after the external-effect
// proof certified it. It lives with the proof and goes with it: every reader
// outside this file reads DeclaredPlugins.
func Plugins(root string) ([]Plugin, error) {
	snapshot, err := ReadCommitted(context.Background(), root)
	if err != nil {
		return nil, err
	}
	return snapshot.Plugins()
}

// Plugins answers the declared registrations only once the external-effect
// proof admitted the tree. It returns no partial list when the proof refuses.
func (s *Snapshot) Plugins() ([]Plugin, error) {
	reader, err := s.readRegistrations()
	if err != nil {
		return nil, err
	}
	registrations := sortedRegistrations(reader)
	result := make([]Plugin, 0, len(registrations))
	for index := range registrations {
		result = append(result, registrations[index].plugin)
	}
	return result, nil
}

// readRegistrations runs the declared walk under the external-effect proof.
// A walk refusal is reported first, because it names a retained source; the
// proof's own refusal then prevents publication of a walk that succeeded.
func (s *Snapshot) readRegistrations() (*registrations, error) {
	certificate, err := s.externalCertificate()
	if err != nil {
		return nil, err
	}
	reader, err := s.walkRegistrations()
	if err != nil {
		return nil, err
	}
	if certificate.refusal != nil {
		return nil, certificate.refusal
	}
	if err := s.refuseEmpty(reader); err != nil {
		return nil, err
	}
	return reader, nil
}
