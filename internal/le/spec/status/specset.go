// Design: docs/contributing/spec-workflow.md -- spec sets
// Related: specstatus.go -- the inventory whose Set column reads the grouping
// Related: closure.go -- the closure detector whose umbrella verdict reads it

package specstatus

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// NoSet is the Set an inventory row carries for a spec that belongs to no set.
// It is an answer, printed in the Set column, never a value left unfilled.
const NoSet = "-"

// umbrellaName is what follows the number 0 in an umbrella's stem:
// `spec-<prefix>-0-umbrella.md`. A longer name after 0, such as
// `0-umbrella-corpus-expansion`, is a child numbered 0, not the umbrella.
const umbrellaName = "umbrella"

// SpecSet is one spec set, as GroupSets answers it from the files in a
// population. It carries the structural fields only: which children have
// closed, who holds each one, the set's state and the child to run next are
// facts about history and claims, and are added beside these by their own
// readers rather than left here as values that read as answers.
type SpecSet struct {
	// Prefix is the set's name: the stem tokens before a member's number.
	Prefix string
	// Umbrella is the umbrella's path, or "" when the population holds no
	// umbrella file for the set. HasUmbrella is the question to ask.
	Umbrella string
	// Open is every numbered child the population holds, by number and then
	// path. The umbrella is never one of them.
	Open []SetChild
}

// SetChild is one open child of a set: a numbered member that is not the
// umbrella.
type SetChild struct {
	Number  int
	Path    string
	Status  string
	Depends string
}

// HasUmbrella reports whether the population holds the set's umbrella file.
func (set SpecSet) HasUmbrella() bool { return set.Umbrella != "" }

// holds reports whether the spec at specPath is the umbrella or an open child.
func (set SpecSet) holds(specPath string) bool {
	if set.Umbrella == specPath {
		return true
	}
	for _, child := range set.Open {
		if child.Path == specPath {
			return true
		}
	}
	return false
}

// SpecSets is every set of one population, sorted by prefix.
type SpecSets []SpecSet

// Of answers the set the spec at specPath belongs to, as its umbrella or as an
// open child, and false when it belongs to none. specPath is the Spec.Path the
// population was grouped from.
func (sets SpecSets) Of(specPath string) (SpecSet, bool) {
	for _, set := range sets {
		if set.holds(specPath) {
			return set, true
		}
	}
	return SpecSet{}, false
}

// Name answers the set name the inventory and the roadmap print for a spec:
// its set's prefix, or NoSet.
func (sets SpecSets) Name(specPath string) string {
	set, member := sets.Of(specPath)
	if !member {
		return NoSet
	}
	return set.Prefix
}

// IsUmbrella reports whether the spec at specPath is the umbrella of a set.
func (sets SpecSets) IsUmbrella(specPath string) bool {
	set, member := sets.Of(specPath)
	if !member {
		return false
	}
	return set.Umbrella == specPath
}

// GroupSets groups a population into spec sets. It is the one reading of the
// naming rule in docs/contributing/spec-workflow.md, "Spec sets": the
// inventory's Set column, the `set` field of the roadmap snapshot and the closure detector's
// umbrella verdict all answer from it, so none of them can disagree with
// another about which files form a set.
//
// A spec is a numbered member when its stem reads `<prefix>-<N>-<name>`, N
// being the first token made only of digits, so a prefix may hold a digit
// inside a token (`bcp194`, `dhcpv6-client`). Members sharing a prefix form a
// set when the group holds an umbrella, or at least two numbered children. The
// umbrella is the member named `<prefix>-0-umbrella`; a group without one takes
// as its umbrella the spec whose whole stem is the prefix, carrying no number
// (`spec-foo.md` beside `spec-foo-1-x.md`). A lone numbered name with no
// umbrella is no set: one file is not a group. An umbrella alone is a set,
// because a set whose children have all closed still owes its umbrella's own
// criteria and its closure.
func GroupSets(specs []Spec) SpecSets {
	paths := make(map[string]string, len(specs))
	groups := make(map[string]*SpecSet)
	for index := range specs {
		spec := &specs[index]
		paths[spec.Name] = spec.Path
		prefix, number, name, numbered := numberedStem(spec.Name)
		if !numbered {
			continue
		}
		group, present := groups[prefix]
		if !present {
			group = &SpecSet{Prefix: prefix}
			groups[prefix] = group
		}
		if number == 0 {
			if name == umbrellaName {
				group.Umbrella = spec.Path
				continue
			}
		}
		group.Open = append(group.Open, SetChild{Number: number, Path: spec.Path, Status: spec.Status, Depends: spec.Depends})
	}

	sets := make(SpecSets, 0, len(groups))
	for prefix, group := range groups {
		if !group.HasUmbrella() {
			// The number-less parent: a spec named for the prefix alone. Its
			// stem holds no number, so the loop above never grouped it.
			if parent, present := paths[prefix]; present {
				group.Umbrella = parent
			}
		}
		if !group.HasUmbrella() {
			if len(group.Open) < 2 {
				continue
			}
		}
		slices.SortFunc(group.Open, func(left, right SetChild) int {
			if order := cmp.Compare(left.Number, right.Number); order != 0 {
				return order
			}
			return cmp.Compare(left.Path, right.Path)
		})
		sets = append(sets, *group)
	}
	slices.SortFunc(sets, func(left, right SpecSet) int { return cmp.Compare(left.Prefix, right.Prefix) })
	return sets
}

// numberedStem reads a stem as `<prefix>-<N>-<name>`. N is the first token made
// only of digits that has a token before it and one after it, so the prefix
// never holds such a token and the name may. A stem with no such token is not
// numbered and answers false.
func numberedStem(stem string) (prefix string, number int, name string, numbered bool) {
	tokens := strings.Split(stem, "-")
	for index := 1; index < len(tokens)-1; index++ {
		value, isNumber := setNumber(tokens[index])
		if !isNumber {
			continue
		}
		return strings.Join(tokens[:index], "-"), value, strings.Join(tokens[index+1:], "-"), true
	}
	return "", 0, "", false
}

// setNumber reads a token made only of ASCII digits. Any other token, the
// empty one included, is not a number. A run of digits too long for an int
// names no set member a person wrote, so it is not one either.
func setNumber(token string) (int, bool) {
	if token == "" {
		return 0, false
	}
	for _, digit := range token {
		if digit < '0' {
			return 0, false
		}
		if digit > '9' {
			return 0, false
		}
	}
	value, err := strconv.Atoi(token)
	if err != nil {
		return 0, false
	}
	return value, true
}
