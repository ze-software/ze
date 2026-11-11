// Design: docs/contributing/dependency-approval.md -- reuse declared modules
// while preserving use-specific approval for experimental cryptography.

package deps

import (
	"strings"

	"golang.org/x/mod/modfile"
)

// Policy combines declared modules with exact package/importer approvals.
// Its zero value permits no use. The module list is private and built only
// from a successfully parsed go.mod; the ledger owns its immutable rows.
type Policy struct {
	modules   []string
	providers map[string]string
	ledger    Ledger
}

// ParsePolicy reads the module declarations that may authorize ordinary uses.
// A commit MUST supply HEAD's go.mod, not an uncommitted declaration; a
// whole-tree check supplies the current file it is checking.
func ParsePolicy(goMod, vendorModules string, ledger Ledger) (Policy, error) {
	file, err := modfile.Parse("go.mod", []byte(goMod), nil)
	if err != nil {
		return Policy{}, err
	}
	modules := make([]string, 0, len(file.Require))
	for _, requirement := range file.Require {
		modules = append(modules, requirement.Mod.Path)
	}
	return Policy{modules: modules, providers: packageProviders(vendorModules), ledger: ledger}, nil
}

// Allows admits an exact ledger approval or full use of a declared ordinary
// module. Circl always requires the exact approval, even when it is declared.
func (p Policy) Allows(pkg, importer string) bool {
	if p.ledger.Allows(pkg, importer) {
		return true
	}
	if packageInModule(pkg, "github.com/cloudflare/circl") {
		return false
	}
	provider, present := p.providers[pkg]
	if !present {
		return false
	}
	if !packageInModule(pkg, provider) {
		return false
	}
	for _, module := range p.modules {
		if provider == module {
			return true
		}
	}
	return false
}

// packageProviders reads the exact package inventory emitted by go mod vendor.
// A module header alone does not establish that it provides an imported package.
func packageProviders(vendorModules string) map[string]string {
	providers := make(map[string]string)
	module := ""
	for line := range strings.SplitSeq(vendorModules, "\n") {
		if rest, header := strings.CutPrefix(line, "# "); header {
			module, _, _ = strings.Cut(rest, " ")
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		pkg := strings.TrimSpace(line)
		if pkg == "" {
			continue
		}
		if module != "" {
			providers[pkg] = module
		}
	}
	return providers
}

// packageInModule keeps similarly prefixed module paths distinct.
func packageInModule(pkg, module string) bool {
	return pkg == module || strings.HasPrefix(pkg, module+"/")
}
