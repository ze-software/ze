// Design: docs/architecture/plugin/feature-gates.md -- complete closure before publication.
package repoinventory

import (
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
