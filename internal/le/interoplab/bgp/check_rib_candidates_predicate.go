// Design: docs/architecture/testing/interop.md -- the IGP import-policy scenarios.
// Related: check_import_policy.go -- the checker bodies that query the lab and call these.
// Related: internal/component/sysrib/candidates.go -- showRIBCandidates, the JSON these decode.
package bgp

import (
	"encoding/json"
	"errors"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ribCandidatePath is one path of Ze's `show rib candidates prefix <p>` answer:
// the fields the import-policy checkers judge, under the kebab-case keys
// candidatePath (internal/component/sysrib) writes.
type ribCandidatePath struct {
	Source   string `json:"source"`
	Distance uint8  `json:"distance"`
	Selected bool   `json:"selected"`
	Hidden   bool   `json:"hidden"`
	HiddenBy string `json:"hidden-by"`
	Chain    string `json:"chain"`
}

// ribCandidateEntry is one prefix of the same answer, with all its paths.
type ribCandidateEntry struct {
	Prefix netip.Prefix       `json:"prefix"`
	Paths  []ribCandidatePath `json:"paths"`
}

// ribCandidateWant is what one path of a prefix must be. A path matches only
// when every judged field is equal, so a field of one path can never satisfy
// the expectation of another (the failure the substring checks had).
type ribCandidateWant struct {
	source   string
	selected bool
	hidden   bool
	hiddenBy string
	chain    string
	// distanceJudged says whether distance is compared: the IS-IS paths are
	// judged on their verdict alone, the OSPF ones on the distance the modify
	// set, so a zero distance is never read as "not judged".
	distanceJudged bool
	distance       uint8
}

// ribCandidatesVerdict decides whether output, Ze's JSON answer to
// `show rib candidates prefix <prefix>`, holds exactly one entry for prefix
// whose paths are exactly wants, one path for each want, in any order.
func ribCandidatesVerdict(output string, prefix netip.Prefix, wants []ribCandidateWant) error {
	var entries []ribCandidateEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		var reason textbuf.Buffer
		reason.Str("decode Ze's show rib candidates answer for ").Str(prefix.String()).Str(": ").Str(err.Error())
		return errors.New(reason.String())
	}
	var found *ribCandidateEntry
	for i := range entries {
		if entries[i].Prefix != prefix {
			continue
		}
		if found != nil {
			return errors.New("show rib candidates lists " + prefix.String() + " twice")
		}
		found = &entries[i]
	}
	if found == nil {
		return errors.New("show rib candidates holds no entry for " + prefix.String())
	}
	if len(found.Paths) != len(wants) {
		var reason textbuf.Buffer
		reason.Str("Ze holds ").Str(strconv.Itoa(len(found.Paths))).Str(" paths for ").Str(prefix.String()).
			Str(", want ").Str(strconv.Itoa(len(wants))).Str(": ").Str(describeCandidatePaths(found.Paths))
		return errors.New(reason.String())
	}
	used := make([]bool, len(found.Paths))
	for w := range wants {
		matched := false
		for p := range found.Paths {
			if used[p] {
				continue
			}
			if !candidatePathMatches(&found.Paths[p], &wants[w]) {
				continue
			}
			used[p] = true
			matched = true
			break
		}
		if !matched {
			var reason textbuf.Buffer
			reason.Str("no path of ").Str(prefix.String()).Str(" is ").Str(describeCandidateWant(&wants[w])).
				Str("; Ze holds ").Str(describeCandidatePaths(found.Paths))
			return errors.New(reason.String())
		}
	}
	return nil
}

// candidatePathMatches compares every field want judges.
func candidatePathMatches(path *ribCandidatePath, want *ribCandidateWant) bool {
	if path.Source != want.source {
		return false
	}
	if path.Selected != want.selected {
		return false
	}
	if path.Hidden != want.hidden {
		return false
	}
	if path.HiddenBy != want.hiddenBy {
		return false
	}
	if path.Chain != want.chain {
		return false
	}
	if want.distanceJudged {
		return path.Distance == want.distance
	}
	return true
}

func describeCandidateWant(want *ribCandidateWant) string {
	var text textbuf.Buffer
	text.Str("source=").Str(want.source).Str(" selected=").Str(strconv.FormatBool(want.selected)).
		Str(" hidden=").Str(strconv.FormatBool(want.hidden)).Str(" hidden-by=").Str(want.hiddenBy).
		Str(" chain=").Str(want.chain)
	if want.distanceJudged {
		text.Str(" distance=").Str(strconv.Itoa(int(want.distance)))
	}
	return text.String()
}

func describeCandidatePaths(paths []ribCandidatePath) string {
	var text textbuf.Buffer
	for i := range paths {
		if i > 0 {
			text.Str("; ")
		}
		text.Str("source=").Str(paths[i].Source).Str(" distance=").Str(strconv.Itoa(int(paths[i].Distance))).
			Str(" selected=").Str(strconv.FormatBool(paths[i].Selected)).
			Str(" hidden=").Str(strconv.FormatBool(paths[i].Hidden)).
			Str(" hidden-by=").Str(paths[i].HiddenBy).Str(" chain=").Str(paths[i].Chain)
	}
	return text.String()
}

// frrRouteEntry is one route of FRR's `show ip route <prefix> json`: the
// protocol that installed it and its next hops.
type frrRouteEntry struct {
	Protocol string `json:"protocol"`
	Selected bool   `json:"selected"`
	NextHops []struct {
		IP string `json:"ip"`
	} `json:"nexthops"`
}

// frrISISRouteVerdict decides whether output, FRR's `show ip route <prefix>
// json`, holds a selected IS-IS route for prefix through nextHop. The next hop
// is what proves the route came through Ze: FRR-B has no other IS-IS neighbor.
func frrISISRouteVerdict(output string, prefix netip.Prefix, nextHop netip.Addr) error {
	var routes map[string][]frrRouteEntry
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		return errors.New("decode FRR's show ip route answer for " + prefix.String() + ": " + err.Error())
	}
	entries, ok := routes[prefix.String()]
	if !ok {
		return errors.New("FRR holds no route for " + prefix.String())
	}
	for i := range entries {
		if entries[i].Protocol != "isis" {
			continue
		}
		if !entries[i].Selected {
			continue
		}
		for _, hop := range entries[i].NextHops {
			if hop.IP == nextHop.String() {
				return nil
			}
		}
	}
	return errors.New("FRR holds no selected IS-IS route for " + prefix.String() + " through " + nextHop.String() + ": " + output)
}
