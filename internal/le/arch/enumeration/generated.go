// Design: docs/architecture/plugin/feature-gates.md -- derived plugin tables retain their source declarations
// Related: actions.go -- the native gated-table check supplies the proof before the walk

package archenumeration

import (
	"errors"
	pathpkg "path"
	"slices"

	"github.com/ze-software/ze/internal/le/plugin/gatedtable"
)

// verifiedPluginCorpora admits only the three tables whose complete source the
// gated-table check reproduced from committed declarations. It exempts no other
// symbol in their file or package. The live registry cannot replace these
// tables: a stripped build must still identify plugins that it does not link.
func verifiedPluginCorpora(corpora []Corpus, proof gatedtable.CheckReport) ([]Corpus, error) {
	if proof.Reason != "" {
		return nil, errors.New(proof.Text())
	}
	if proof.File == "" {
		return nil, errors.New("enumeration: the gated-table check named no verified file; correct plugin gated-table check before running arch enumeration check")
	}
	out := slices.Clone(corpora)
	for i := range out {
		if out[i].Name != "plugin names" {
			continue
		}
		declarations := slices.Clone(out[i].Declarations)
		for _, symbol := range []string{"gatedPluginTags", "gatedConfigRoots", "gatedDataPlaneWriters"} {
			declarations = append(declarations, Declaration{Package: pathpkg.Dir(proof.File), Symbol: symbol})
		}
		out[i].Declarations = declarations
		return out, nil
	}
	return nil, errors.New("enumeration: the plugin-name corpus is absent; restore registry.All() in readCorpora before running arch enumeration check")
}
