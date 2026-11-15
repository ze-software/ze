// Design: docs/architecture/route-selection.md -- prospective next-hop resolution
// Related: entry.go, policy.go, distance.go -- the same rules used by insertion
package locrib

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// PreviewSelected returns the selected path if offered replaced its contribution
// at prefix. It applies the normal import and distance rules without storing a
// path, accounting an offer or emitting an event. The returned path and its slices
// are read-only. Nil, an invalid prefix or an invalid offer answers no selection.
// Safe for concurrent use; a caller needing a stable view MUST check the RIB revision.
func (r *RIB) PreviewSelected(fam family.Family, prefix netip.Prefix, offered Path) (Path, bool) {
	if r == nil {
		return Path{}, false
	}
	if !prefix.IsValid() {
		return Path{}, false
	}
	if !offered.Valid() {
		return Path{}, false
	}
	offered = admit(ribpolicy.Current(), fam, prefix, offered)
	offered.AdminDistance = resolvedDistance(offered)
	var best Path
	found := false
	consider := func(path Path) {
		pair := [2]Path{best, path}
		index := selectBest(pair[:])
		if index >= 0 {
			best, found = pair[index], true
		}
	}
	replaced := false
	r.Inspect(fam, prefix, func(group PathGroup) {
		for _, path := range group.Paths {
			if path.key() == offered.key() {
				consider(offered)
				replaced = true
				continue
			}
			consider(path)
		}
	})
	if !replaced {
		consider(offered)
	}
	return best, found
}
