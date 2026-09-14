// Design: docs/architecture/core-design.md -- le links the product to introspect it
//
// Detail: enumeration.go -- the walk that judges a literal against these sets
//
// corpus.go builds the key sets a Go literal can restate. Every set is read
// from the registry that owns it, in this process, behind the blank import of
// the product composition root. Nothing here transcribes a key, because a gate
// carrying its own copy of the set is the defect this gate exists to find
// (ai/rules/principles.md).
//
// Each corpus declares a floor. A registry that answers short answers a set
// that matches almost nothing, and the walk then passes every literal in the
// tree while reporting no finding. The floor turns that silence into a refusal
// (ai/rules/evidence.md).

package enumeration

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	// The blank import runs every product init(), so the registries below hold
	// what the daemon holds rather than a subset of it. This direction is the
	// allowed one: le may link ze, ze never links le
	// (docs/architecture/core-design.md).
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
)

// Group is one set of keys a literal can restate, and it is the unit the
// two-key threshold counts against. A flat registry answers one group. The YANG
// model answers one group for each distinct enumeration, because two values of
// two different enumerations are two facts rather than one copied set.
type Group struct {
	// Name is what a finding calls this group. For a flat registry it repeats
	// the corpus name, so the reader is told the registry once.
	Name string
	Keys []string
}

// Corpus is one registry's key sets, with the floor that says the registry
// answered and the packages that own it.
type Corpus struct {
	// Name is the registry a finding names, in the words a reader greps for.
	Name string
	// Producer is the call this corpus was read from, so a reader can go and
	// check the set rather than trust the finding.
	Producer string
	// Floor is the least distinct keys the registry must answer, across every
	// group, before the gate believes it read the registry at all.
	Floor int
	// Owners are the repository-relative package directories that declare the
	// registry. Code inside one of them is writing the set down for the first
	// time rather than copying it.
	Owners []string
	// Closed says each group of this corpus is a closed enumeration, so a copy
	// restates ALL of it. A literal sharing two words with a twelve-value
	// enumeration shares two words; it does not hold a copy of the set.
	Closed bool
	// WrittenWhole says a key of this registry is written out in full at the
	// place it is registered, which is what lets a const block BE the
	// declaration of one.
	//
	// A family name is not: family.MustRegister(AFIIPv4, SAFIMUP, "ipv4",
	// "mup") composes "ipv4/mup" from two parts (internal/core/family/
	// registry.go), so every Go const holding the joined string is a second
	// spelling of it and none of them is where the key comes from. A diagnostic
	// code is: CodeMeta{Code: codeOSPFRouterIDMissing} carries the const itself
	// into the registry (internal/plugins/ospf/register.go:156).
	WrittenWhole bool
	Groups       []Group
}

// keyCount answers the distinct keys this corpus holds across every group.
func (c Corpus) keyCount() int {
	seen := make(map[string]bool)
	for _, group := range c.Groups {
		for _, key := range group.Keys {
			seen[key] = true
		}
	}
	return len(seen)
}

// owns reports whether the package directory rel declares this corpus's
// registry, in which case its literals are the declaration and not a copy.
func (c Corpus) owns(rel string) bool {
	for _, owner := range c.Owners {
		if rel == owner || strings.HasPrefix(rel, owner+"/") {
			return true
		}
	}
	return false
}

// The floors. Each is well under what this checkout answered on 2026-09-14, so
// the floor fires on a registry that did not answer rather than on one that
// lost a member. The measured counts are in the comment beside each.
const (
	// 89 plugins registered on 2026-09-14.
	floorPlugins = 30
	// 23 families registered on 2026-09-14.
	floorFamilies = 15
	// 13 verbs on 2026-09-14, and the vocabulary is deliberately small.
	floorVerbs = 8
	// 329 distinct enumeration values on 2026-09-14, over 123 distinct
	// enumerations. DefaultLoader discards its LoadRegistered and Resolve
	// errors, so a half-loaded model answers quietly: this floor is the only
	// thing that says so.
	floorYANGEnums = 120
	// 154 diagnostic codes on 2026-09-14, the 141 builtins included.
	floorDiagnosticCodes = 100
)

// ErrShortCorpus names a registry that answered fewer keys than its floor.
var ErrShortCorpus = errors.New("a corpus answered fewer keys than its floor")

// readCorpora reads all five registries and answers their key sets.
//
// An error here says a registry could not be READ at all, which is a different
// fact from a registry that answered short: checkFloors judges that one, on the
// one path the actions take (actions.go, walkTree), so the floor refusal is
// proved by a test driving the command rather than by a second copy of the rule
// here.
func readCorpora() ([]Corpus, error) {
	enums, err := yangEnumerations()
	if err != nil {
		return nil, err
	}
	return []Corpus{
		pluginNames(),
		familyNames(),
		commandVerbs(),
		enums,
		diagnosticCodes(),
	}, nil
}

// checkFloors refuses the first corpus that answered short, naming it and both
// counts so the reader knows which registry did not answer.
//
// The refusal is the whole point of a floor. A registry that answers nothing
// makes every literal in the tree innocent, and the gate then prints the same
// "OK" it prints over a clean tree.
func checkFloors(corpora []Corpus) error {
	for _, corpus := range corpora {
		count := corpus.keyCount()
		if count < corpus.Floor {
			return fmt.Errorf("%w: %s answered %d keys from %s, below the floor of %d: that registry did not answer",
				ErrShortCorpus, corpus.Name, count, corpus.Producer, corpus.Floor)
		}
	}
	return nil
}

// pluginNames is every registered plugin name.
func pluginNames() Corpus {
	registrations := registry.All()
	names := make([]string, 0, len(registrations))
	for _, registration := range registrations {
		names = append(names, registration.Name)
	}
	return Corpus{
		Name:     "plugin names",
		Producer: "registry.All()",
		Floor:    floorPlugins,
		// The registry package declares the set; each plugin's own package
		// declares one member of it, which one key never reports anyway.
		Owners:       []string{"internal/component/plugin/registry"},
		WrittenWhole: true,
		Groups:       []Group{{Name: "plugin names", Keys: names}},
	}
}

// familyNames is every registered address family name.
func familyNames() Corpus {
	return Corpus{
		Name:     "family names",
		Producer: "family.RegisteredFamilyNames()",
		Floor:    floorFamilies,
		Owners:   []string{"internal/core/family"},
		// Not WrittenWhole: the registrar joins the AFI and SAFI names, so the
		// joined string a Go const holds was never the declaration.
		Groups: []Group{{Name: "family names", Keys: family.RegisteredFamilyNames()}},
	}
}

// commandVerbs is the canonical CLI verb vocabulary.
func commandVerbs() Corpus {
	verbs := make([]string, 0, len(command.Verbs))
	for verb := range command.Verbs {
		verbs = append(verbs, verb)
	}
	return Corpus{
		Name:         "CLI verbs",
		Producer:     "command.Verbs",
		Floor:        floorVerbs,
		Owners:       []string{"internal/component/command"},
		WrittenWhole: true,
		Groups:       []Group{{Name: "CLI verbs", Keys: verbs}},
	}
}

// diagnosticCodes is every registered diagnostic code.
//
// RegisterBuiltinCodes is called here because nothing else calls it in this
// process. Its one non-test caller is cmd/ze/ze_core_dispatch.go, so a tool
// that only blank-imports the composition root reads a registry holding the
// plugin-registered codes and none of the builtins.
func diagnosticCodes() Corpus {
	diagnostic.RegisterBuiltinCodes()
	return Corpus{
		Name:         "diagnostic codes",
		Producer:     "diagnostic.AllCodes()",
		Floor:        floorDiagnosticCodes,
		Owners:       []string{"internal/core/diagnostic"},
		WrittenWhole: true,
		Groups:       []Group{{Name: "diagnostic codes", Keys: diagnostic.AllCodes()}},
	}
}

// entryDepthMax bounds the walk over the goyang entry tree. The tree is built
// from modules this binary embeds, so the depth is ours rather than a peer's,
// and 64 is far past the deepest Ze module. The bound is here because a
// grouping that refers to itself would otherwise walk forever.
const entryDepthMax = 64

// yangEnumerations is one group for each distinct enumeration the YANG model
// declares.
//
// The group is keyed by the enumeration's VALUE SET rather than by the leaf
// that declares it, so the many leaves that share `enabled`/`disabled` answer
// one group. Two leaves with the same values are one fact about the model.
//
// A loader that fails is an error rather than an empty corpus. The floor would
// catch the empty corpus too, but it would report "that registry did not
// answer" over a reason the loader already gave.
func yangEnumerations() (Corpus, error) {
	corpus := Corpus{
		Name:     "YANG enumerations",
		Producer: "yang.DefaultLoader() entry tree, entry.Type.Enum.Names()",
		Floor:    floorYANGEnums,
		Owners:   []string{"internal/component/config/yang"},
		// An enumeration is closed, so a literal holds a copy of it only by
		// holding all of it. Two shared words are two words: `parseBool` in
		// internal/component/bfd/config.go holds "true" and "false", and the
		// accept leaf of ze-peer-cmd holds them too, and neither took them from
		// the other.
		Closed: true,
	}

	// DefaultLoader discards its own LoadRegistered and Resolve errors, so a
	// half-loaded model comes back here looking whole. The floor is what says
	// so, and it is why this corpus has the largest one.
	loader, err := yang.DefaultLoader()
	if err != nil {
		return corpus, fmt.Errorf("load the YANG model: %w", err)
	}

	// Keyed by the joined value set, so one enumeration is one group however
	// many leaves declare it. The value is the leaf path of the first declaring
	// leaf, which is what the finding cites.
	pathBySet := map[string]string{}
	keysBySet := map[string][]string{}
	for _, module := range loader.ModuleNames() {
		entry := loader.GetEntry(module)
		if entry == nil {
			continue
		}
		collectEnums(entry, module, 0, map[*gyang.Entry]bool{}, pathBySet, keysBySet)
	}

	sets := make([]string, 0, len(keysBySet))
	for set := range keysBySet {
		sets = append(sets, set)
	}
	slices.Sort(sets)
	for _, set := range sets {
		corpus.Groups = append(corpus.Groups, Group{
			Name: "the enumeration at " + pathBySet[set],
			Keys: keysBySet[set],
		})
	}
	return corpus, nil
}

// collectEnums records every enumeration under entry, keyed by its value set.
func collectEnums(entry *gyang.Entry, path string, depth int, seen map[*gyang.Entry]bool, pathBySet map[string]string, keysBySet map[string][]string) {
	if depth > entryDepthMax {
		return
	}
	if seen[entry] {
		return
	}
	seen[entry] = true

	if entry.Type != nil && entry.Type.Kind == gyang.Yenum && entry.Type.Enum != nil {
		names := entry.Type.Enum.Names()
		if len(names) > 1 {
			sorted := slices.Clone(names)
			slices.Sort(sorted)
			// The NUL byte joins the values because a YANG enum value cannot
			// hold one, so two different sets cannot produce one key.
			set := strings.Join(sorted, "\x00")
			if _, known := keysBySet[set]; !known {
				pathBySet[set] = path
				keysBySet[set] = sorted
			}
		}
	}

	children := make([]string, 0, len(entry.Dir))
	for name := range entry.Dir {
		children = append(children, name)
	}
	slices.Sort(children)
	for _, name := range children {
		collectEnums(entry.Dir[name], path+"/"+name, depth+1, seen, pathBySet, keysBySet)
	}
}
