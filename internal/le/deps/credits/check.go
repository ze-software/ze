// Design: docs/architecture/third-party-credits.md -- the check
// Overview: credits.go -- the declaration and the record it judges

package credits

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/deps"
)

// Refusal is one thing the check refuses: the work, what is wrong, why it
// matters, what to change, and the command that regenerates once it is
// changed.
type Refusal struct {
	Work    string `json:"work"`
	Problem string `json:"problem"`
	Why     string `json:"why"`
	Edit    string `json:"edit,omitempty"`
	Fix     string `json:"fix"`
}

// Report is what `deps credits check` answers.
type Report struct {
	// Works is how many works the generated data would hold.
	Works int `json:"works"`
	// Flavors names each flavor and system whose linked modules were read.
	Flavors  []string  `json:"flavors"`
	Refusals []Refusal `json:"refusals"`
}

// Text renders the check for a person: the verdict, then each refusal with
// the problem first and the command alone on its last line.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str("deps credits check: ").Int(int64(r.Works)).Str(" shipped works, flavors ").Str(strings.Join(r.Flavors, ", ")).Byte('\n')
	if len(r.Refusals) == 0 {
		tb.Str("every shipped work is declared, its licence and holder are in its own files, and the generated data is current\n")
		return tb.String()
	}
	tb.Int(int64(len(r.Refusals))).Str(" problem(s):\n")
	for _, refusal := range r.Refusals {
		tb.Byte('\n').Str(refusal.Work).Str(": ").Str(refusal.Problem).Byte('\n')
		tb.Str("  ").Str(refusal.Why).Byte('\n')
		if refusal.Edit != "" {
			tb.Str("  ").Str(refusal.Edit).Str(", then run:\n")
		}
		tb.Str("  ").Str(refusal.Fix).Byte('\n')
	}
	return tb.String()
}

// Check judges the whole tree: the declaration against what the shipped
// flavors link, each work's licence and holder against its own files, the
// presence declarations against the works list, the bundle banners against
// the works, and the generated data against what Update would write.
func Check(root string, in inputs) (Report, error) {
	raw, err := readRel(root, DeclarationRel)
	if err != nil {
		return Report{}, err
	}
	decl, err := readDeclaration(raw)
	if err != nil {
		return Report{}, err
	}
	links, err := in.linked(root)
	if err != nil {
		return Report{}, err
	}
	out, refusals, err := plan(root, raw, links)
	if err != nil {
		return Report{}, err
	}
	byModule := shipped(links)
	report := Report{Works: len(out.record.Works)}
	for _, flavor := range links {
		report.Flavors = append(report.Flavors, flavor.Flavor)
	}
	refusals = append(refusals, unlinkedEntries(decl, byModule)...)
	refusals = append(refusals, toolchainRefusals(decl)...)
	all := decl.entries()
	for index := range all {
		if !ships(all[index], byModule) {
			continue
		}
		problems, err := entryRefusals(root, all[index])
		if err != nil {
			return Report{}, err
		}
		refusals = append(refusals, problems...)
	}
	problems, err := presenceRefusals(root, decl, in)
	if err != nil {
		return Report{}, err
	}
	refusals = append(refusals, problems...)
	problems, err = bannerRefusals(root, decl)
	if err != nil {
		return Report{}, err
	}
	refusals = append(refusals, problems...)
	problems, err = dataRefusals(root, out)
	if err != nil {
		return Report{}, err
	}
	refusals = append(refusals, problems...)
	report.Refusals = refusals
	return report, nil
}

// undeclaredModules refuses each module a flavor links that no entry names
// (AC-10), naming the flavors that link it.
func undeclaredModules(decl declaration, byModule map[string][]string) []Refusal {
	declared := make(map[string]bool)
	all := decl.entries()
	for index := range all {
		if all[index].kind == kindModule {
			declared[all[index].name] = true
		}
	}
	modules := make([]string, 0, len(byModule))
	for module := range byModule {
		modules = append(modules, module)
	}
	slices.Sort(modules)
	var refusals []Refusal
	for _, module := range modules {
		if declared[module] {
			continue
		}
		refusals = append(refusals, Refusal{
			Work:    module,
			Problem: "is linked into the shipped binary by " + strings.Join(byModule[module], ", ") + " and has no entry in " + DeclarationRel,
			Why:     "the binary carries its code, so it must carry its licence and copyright holder, which ze thanks prints",
			Edit: "add {\"module\": \"" + module + "\", \"licence\": ..., \"copyright\": ..., \"source\": ...} to the linked list " +
				"of " + DeclarationRel + ", or to its curated category when Ze imports it directly, reading the licence and holder from " +
				path.Join(vendorRoot, module),
			Fix: updateCommand,
		})
	}
	return refusals
}

// unlinkedEntries refuses each `linked` entry no flavor links.
func unlinkedEntries(decl declaration, byModule map[string][]string) []Refusal {
	var refusals []Refusal
	for index := range decl.Linked {
		module := &decl.Linked[index]
		if _, linked := byModule[module.Module]; linked {
			continue
		}
		refusals = append(refusals, Refusal{
			Work:    module.Module,
			Problem: "is in the linked list of " + DeclarationRel + " but no shipped flavor links it",
			Why:     "the linked list credits what the shipped binary carries, and an entry for a module it does not carry is a false credit",
			Edit:    "remove the entry from the linked list",
			Fix:     updateCommand,
		})
	}
	return refusals
}

// toolchainRefusals refuses a toolchain work that declares a version, which
// the binary would never print because it reads the toolchain's version from
// its build information; one that lists carried-by packages, because every
// binary lists the toolchain work without a carrier, so the list would be a
// second declaration nothing reads; a second toolchain work, because one
// toolchain builds a binary; and a declaration with none, because every
// binary lists Go and refuses a record that lacks it.
func toolchainRefusals(decl declaration) []Refusal {
	var refusals []Refusal
	first := ""
	for index := range decl.Works {
		work := &decl.Works[index]
		if !work.Toolchain {
			continue
		}
		if work.Version != "" {
			refusals = append(refusals, Refusal{
				Work:    work.Name,
				Problem: "is marked toolchain and declares version " + work.Version + " in " + DeclarationRel,
				Why:     "a binary reads the version of the toolchain that built it from its own build information, so a declared one is never printed",
				Edit:    "set its version to \"\"",
				Fix:     updateCommand,
			})
		}
		if len(work.CarriedBy) != 0 {
			refusals = append(refusals, Refusal{
				Work:    work.Name,
				Problem: "is marked toolchain and lists " + strings.Join(work.CarriedBy, ", ") + " under carried-by in " + DeclarationRel,
				Why:     "every binary lists the toolchain work without a carrier, so no package declares it and nothing reads the list",
				Edit:    "remove carried-by from the entry",
				Fix:     updateCommand,
			})
		}
		if first == "" {
			first = work.Name
			continue
		}
		refusals = append(refusals, Refusal{
			Work:    work.Name,
			Problem: "is marked toolchain in " + DeclarationRel + " beside " + first,
			Why:     "a binary is built by one Go toolchain, so one work carries its version",
			Edit:    "remove toolchain from one of the two",
			Fix:     updateCommand,
		})
	}
	if first == "" {
		refusals = append(refusals, Refusal{
			Work:    DeclarationRel,
			Problem: "marks no work as the Go toolchain",
			Why:     "every binary lists Go itself, and the binary refuses a credits record that holds no toolchain work",
			Edit: "add the works entry \"Go standard library and runtime\" with \"toolchain\": true, an empty version, " +
				"its licence, holder, source and files",
			Fix: updateCommand,
		})
	}
	return refusals
}

// declaredTwice refuses each name the declaration holds more than once.
func declaredTwice(decl declaration) []Refusal {
	seen := make(map[string]string)
	var refusals []Refusal
	all := decl.entries()
	for index := range all {
		item := &all[index]
		first, twice := seen[item.name]
		if !twice {
			seen[item.name] = item.list
			continue
		}
		refusals = append(refusals, Refusal{
			Work:    item.name,
			Problem: "is declared in " + first + " and again in " + item.list + " of " + DeclarationRel,
			Why:     "each work is declared once, so its group, licence and holder have one source",
			Edit:    "keep one entry and remove the other",
			Fix:     updateCommand,
		})
	}
	return refusals
}

// entryRefusals judges one shipped entry: its required fields, its holder
// and its licence against the text of its own files.
func entryRefusals(root string, item entry) ([]Refusal, error) {
	refusals := missingFields(item)
	origins, problem, err := workFiles(root, item)
	if err != nil {
		return nil, err
	}
	if problem != nil {
		// plan already refused the missing licence file.
		return refusals, nil
	}
	texts := make(map[string]string, len(origins)+1)
	for _, origin := range origins {
		content, err := readRel(root, origin)
		if err != nil {
			return nil, err
		}
		texts[origin] = string(content)
	}
	read := slices.Clone(origins)
	if item.copyrightFile != "" {
		content, err := readRel(root, item.copyrightFile)
		if err != nil {
			refusals = append(refusals, Refusal{
				Work: item.name, Problem: "names " + item.copyrightFile + " as its copyright-file, which cannot be read: " + err.Error(),
				Why:  "the holder is checked against the file that states it",
				Edit: "correct copyright-file to the repository path of the file stating the holder", Fix: updateCommand,
			})
		} else {
			read = append(read, item.copyrightFile)
			texts[item.copyrightFile] = string(content)
		}
	}
	refusals = append(refusals, copyrightRefusals(item, read, texts)...)
	refusals = append(refusals, licenceRefusals(item, read, texts)...)
	return refusals, nil
}

// missingFields refuses each field a shipped entry leaves empty.
func missingFields(item entry) []Refusal {
	var missing []string
	if item.licence == "" {
		missing = append(missing, "licence")
	}
	if item.source == "" {
		missing = append(missing, "source")
	}
	if item.copyright == "" && !item.unstated {
		missing = append(missing, "copyright")
	}
	if item.kind == kindWork && item.group == "" {
		missing = append(missing, "group")
	}
	// The toolchain work is listed by every binary without a carrier, so it
	// owes none (toolchainRefusals refuses one it declares).
	if item.kind == kindWork && !item.toolchain && len(item.carriedBy) == 0 {
		missing = append(missing, "carried-by")
	}
	if len(missing) == 0 {
		return nil
	}
	return []Refusal{{
		Work:    item.name,
		Problem: "leaves " + strings.Join(missing, ", ") + " empty in the " + item.list + " list of " + DeclarationRel,
		Why:     "ze thanks prints each of them for every work the binary carries",
		Edit:    "fill each from the work's own files and upstream",
		Fix:     updateCommand,
	}}
}

// copyrightRefusals refuses a declared holder found verbatim in none of the
// work's files (AC-12), and a copyright-unstated marker on a work one of
// whose files states a holder line.
func copyrightRefusals(item entry, read []string, texts map[string]string) []Refusal {
	if item.unstated {
		for _, origin := range read {
			line, stated := holderLine(texts[origin])
			if !stated {
				continue
			}
			return []Refusal{{
				Work:    item.name,
				Problem: "is marked copyright-unstated, but " + origin + " states a holder: \"" + line + "\"",
				Why:     "the marker is only for a work whose files name no holder, and here the file names one",
				Edit:    "remove copyright-unstated and declare the holder as the file spells it",
				Fix:     updateCommand,
			}}
		}
		return nil
	}
	var refusals []Refusal
	for _, holder := range holders(item.copyright) {
		if foundIn(holder, read, texts) {
			continue
		}
		refusals = append(refusals, Refusal{
			Work:    item.name,
			Problem: "declares the copyright holder \"" + holder + "\", which none of " + strings.Join(read, ", ") + " contains",
			Why:     "a holder is copied from the work's own files, so a holder they do not state was written from memory",
			Edit:    "spell the holder exactly as one of those files does, or name the source file that states it in copyright-file",
			Fix:     updateCommand,
		})
	}
	return refusals
}

// foundIn answers whether needle appears verbatim in one of the texts.
func foundIn(needle string, read []string, texts map[string]string) bool {
	for _, origin := range read {
		if strings.Contains(texts[origin], needle) {
			return true
		}
	}
	return false
}

// licenceRefusals refuses an identifier the table does not know, and an
// expression none of whose identifiers the work's text states.
func licenceRefusals(item entry, read []string, texts map[string]string) []Refusal {
	if item.licence == "" {
		return nil
	}
	identifiers, err := licenceIdentifiers(item.licence)
	if err != nil {
		return []Refusal{{Work: item.name, Problem: "declares the licence \"" + item.licence + "\": " + err.Error(),
			Why: "the licence is an SPDX expression", Edit: "write the SPDX expression of the work's licence", Fix: updateCommand}}
	}
	var refusals []Refusal
	for _, identifier := range identifiers {
		if _, known := licenceSentences[identifier]; known {
			continue
		}
		refusals = append(refusals, Refusal{
			Work:    item.name,
			Problem: "declares the licence identifier " + identifier + ", which the check's table does not know",
			Why:     "an identifier is accepted only when its title or defining sentence is found in the work's text, and the table holds none for it",
			Edit:    "add " + identifier + " with its title and defining sentence to " + licenceTable + ", or correct the identifier",
			Fix:     updateCommand,
		})
	}
	if len(refusals) != 0 {
		return refusals
	}
	var joined strings.Builder
	for _, origin := range read {
		joined.WriteString(texts[origin])
		joined.WriteByte('\n')
	}
	text := normalized(joined.String())
	// Every identifier must be stated, under OR as under AND: an OR
	// expression records the choice the work's licence offers, so its text
	// states each alternative (licenceIdentifiers, operatorOr).
	for _, identifier := range identifiers {
		if statesIdentifier(identifier, text) {
			continue
		}
		refusals = append(refusals, Refusal{
			Work:    item.name,
			Problem: "declares the licence " + item.licence + ", and none of " + strings.Join(read, ", ") + " states " + identifier,
			Why:     "the identifier is copied from the work's own licence text, so one its text does not state was written from memory",
			Edit:    "read the licence from those files and declare its SPDX expression, or name the file that states it under licence-files",
			Fix:     updateCommand,
		})
	}
	return refusals
}

// presenceRefusals refuses a carried-by package that does not declare its
// work present, a declaration naming no works entry, a declaration in a
// package the entry does not list (R-3), and any declaration of the toolchain
// work, which every binary lists without one.
func presenceRefusals(root string, decl declaration, in inputs) ([]Refusal, error) {
	goMod, err := readRel(root, "go.mod")
	if err != nil {
		return nil, err
	}
	module, err := deps.ModulePath(string(goMod))
	if err != nil {
		return nil, err
	}
	files, err := in.goFiles(root)
	if err != nil {
		return nil, err
	}
	found, refusals, err := presenceDeclarations(root, module, files)
	if err != nil {
		return nil, err
	}
	works := make(map[string]*declaredWork, len(decl.Works))
	for index := range decl.Works {
		work := &decl.Works[index]
		works[work.Name] = work
		// A toolchain work's carried-by is refused whole by toolchainRefusals,
		// whose edit is to remove it, never to declare the work.
		if work.Toolchain {
			continue
		}
		for _, carrier := range work.CarriedBy {
			if declares(found, work.Name, carrier) {
				continue
			}
			refusals = append(refusals, Refusal{
				Work:    work.Name,
				Problem: "lists " + carrier + " under carried-by, and that package does not declare it present",
				Why:     "a build credits a work only when the package that compiles it in declares it, so this one would never be credited",
				Edit:    "call credits.DeclarePresent(\"" + work.Name + "\") from an init() of " + carrier + ", or correct carried-by",
				Fix:     updateCommand,
			})
		}
	}
	for _, call := range found {
		work, declared := works[call.work]
		if !declared {
			refusals = append(refusals, Refusal{
				Work: call.at, Problem: "declares the work \"" + call.work + "\" present, which no works entry of " + DeclarationRel + " names",
				Why:  "the binary refuses at start a presence its generated data does not hold",
				Edit: "add a works entry named \"" + call.work + "\", or correct the name", Fix: updateCommand,
			})
			continue
		}
		if work.Toolchain {
			refusals = append(refusals, Refusal{
				Work: call.at, Problem: "declares the toolchain work \"" + call.work + "\" present",
				Why:  "every binary lists the toolchain work without a declaration, and the binary refuses one at start",
				Edit: "remove the credits.DeclarePresent call", Fix: updateCommand,
			})
			continue
		}
		if slices.Contains(work.CarriedBy, call.pkg) {
			continue
		}
		refusals = append(refusals, Refusal{
			Work: call.work, Problem: "is declared present by " + call.at + ", whose package " + call.pkg + " is not in its carried-by list",
			Why:  "carried-by names every package that compiles the work in, so the check can find a package that stops declaring it",
			Edit: "add " + call.pkg + " to the entry's carried-by", Fix: updateCommand,
		})
	}
	return refusals, nil
}

// declares answers whether pkg declares work present.
func declares(found []presence, work, pkg string) bool {
	for _, call := range found {
		if call.work == work && call.pkg == pkg {
			return true
		}
	}
	return false
}

// bannerRefusals refuses each banner of a bundle's companion licence file
// that no works entry listing the file claims (R-7), and each declared banner
// phrase no banner of the entry's companion files writes.
func bannerRefusals(root string, decl declaration) ([]Refusal, error) {
	var companions []string
	claimants := make(map[string][]entry)
	all := decl.entries()
	for index := range all {
		item := &all[index]
		for _, origin := range item.declaredFiles {
			if !strings.HasSuffix(origin, companionSuffix) {
				continue
			}
			if !slices.Contains(companions, origin) {
				companions = append(companions, origin)
			}
			claimants[origin] = append(claimants[origin], *item)
		}
	}
	found := make(map[string][]banner, len(companions))
	var refusals []Refusal
	for _, origin := range companions {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(origin))); errors.Is(err, fs.ErrNotExist) {
			// workFiles refuses a listed file that does not exist, naming it.
			continue
		}
		content, err := readRel(root, origin)
		if err != nil {
			return nil, err
		}
		found[origin] = banners(string(content))
		for _, left := range unclaimedBanners(found[origin], claimants[origin]) {
			refusals = append(refusals, Refusal{
				Work:    origin + ":" + strconv.Itoa(left.line),
				Problem: "bundles a library no works entry claims: \"" + left.title() + "\"",
				Why:     "the bundle ships that library to every browser that loads it, so the binary owes it a credit",
				Edit: "add a works entry for the library that lists " + origin + " in its files, named as the banner names it, " +
					"or declaring under banner a phrase the banner writes",
				Fix: updateCommand,
			})
		}
	}
	for index := range all {
		refusals = append(refusals, phraseRefusals(&all[index], found)...)
	}
	return refusals, nil
}

// phraseRefusals refuses a declared banner phrase that no banner of the
// entry's companion files writes: a phrase that claims nothing ties the work
// to no banner, so the banner it was written for would pass unexamined the
// day it changes. found holds the banners of each companion file that
// exists; a listed file that does not exist is refused by workFiles.
func phraseRefusals(item *entry, found map[string][]banner) []Refusal {
	if item.banner == "" {
		return nil
	}
	var read []string
	for _, origin := range item.declaredFiles {
		if !strings.HasSuffix(origin, companionSuffix) {
			continue
		}
		read = append(read, origin)
		if phraseFound(item, found[origin]) {
			return nil
		}
	}
	if len(read) == 0 {
		return []Refusal{{
			Work:    item.name,
			Problem: "declares the banner phrase \"" + item.banner + "\" and lists no bundle companion file (*" + companionSuffix + ") under files",
			Why:     "the phrase ties the work to its banner in a bundle's companion file, so without one it ties the work to nothing",
			Edit:    "list the companion file whose banner writes the phrase under files, or remove banner",
			Fix:     updateCommand,
		}}
	}
	return []Refusal{{
		Work:    item.name,
		Problem: "declares the banner phrase \"" + item.banner + "\", which no banner of " + strings.Join(read, ", ") + " writes",
		Why:     "the phrase ties the work to its banner, so a phrase no banner writes ties the work to nothing",
		Edit:    "copy the phrase verbatim from the work's banner in that file, or remove banner when the banner names the work",
		Fix:     updateCommand,
	}}
}

// dataRefusals refuses generated data that is not what Update would write
// (AC-11): a missing or different record, a changed text, a file no work
// references.
func dataRefusals(root string, out generated) ([]Refusal, error) {
	recordRel := path.Join(DataRel, RecordName)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(recordRel))); errors.Is(err, fs.ErrNotExist) {
		return []Refusal{staleData(recordRel, "is missing")}, nil
	}
	current, err := readRel(root, recordRel)
	if err != nil {
		return nil, err
	}
	var refusals []Refusal
	written, problem := writtenRecord(recordRel, current)
	if problem != nil {
		return []Refusal{*problem}, nil
	}
	if written.Declaration != out.record.Declaration {
		refusals = append(refusals, staleData(DeclarationRel, "changed since "+recordRel+" was generated from it"))
	}
	if !slices.Equal(written.Groups, out.record.Groups) {
		refusals = append(refusals, staleData(recordRel, "orders its groups as "+strings.Join(written.Groups, ", ")+
			", and the declaration orders them as "+strings.Join(out.record.Groups, ", ")))
	}
	for index := range out.record.Works {
		want := &out.record.Works[index]
		have, held := written.work(want.Name)
		if !held {
			refusals = append(refusals, staleData(want.Name, "ships and has no record in "+recordRel))
			continue
		}
		refusals = append(refusals, workDifference(want, &have)...)
	}
	for index := range written.Works {
		name := written.Works[index].Name
		if _, wanted := out.record.work(name); wanted {
			continue
		}
		refusals = append(refusals, staleData(name, "has a record in "+recordRel+" but is no longer a declared shipped work"))
	}
	names := make([]string, 0, len(out.files))
	for name := range out.files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if name == RecordName {
			continue
		}
		if storedAs(root, name, out.files[name]) {
			continue
		}
		refusals = append(refusals, staleData(path.Join(DataRel, name), "is not the copy of the licence text it is named for"))
	}
	stale, err := unreferenced(root, out)
	if err != nil {
		return nil, err
	}
	for _, name := range stale {
		refusals = append(refusals, staleData(path.Join(DataRel, name), "is referenced by no work"))
	}
	if len(refusals) == 0 && !bytes.Equal(current, out.files[RecordName]) {
		refusals = append(refusals, staleData(recordRel, "is not laid out as update writes it"))
	}
	return refusals, nil
}

// storedAs answers whether the data directory holds name with exactly
// content; a file it cannot read is not stored.
func storedAs(root, name string, content []byte) bool {
	stored, err := readRel(root, path.Join(DataRel, name))
	if err != nil {
		return false
	}
	return bytes.Equal(stored, content)
}

// workDifference refuses a record that differs from the one Update writes,
// naming the licence files whose bytes changed when that is the difference.
func workDifference(want, have *workRecord) []Refusal {
	var changed []string
	for _, file := range want.Files {
		if slices.Contains(have.Files, file) {
			continue
		}
		changed = append(changed, file.Origin)
	}
	if len(changed) != 0 {
		return []Refusal{staleData(want.Name, "has licence files whose generated copy differs from "+strings.Join(changed, ", "))}
	}
	if !slices.Equal(want.Files, have.Files) {
		return []Refusal{staleData(want.Name, "has licence files in its generated record that it no longer ships")}
	}
	if sameFields(want, have) {
		return nil
	}
	return []Refusal{staleData(want.Name, "has a generated record that differs from its declaration")}
}

// sameFields compares every field of two records but the files.
func sameFields(want, have *workRecord) bool {
	return want.Name == have.Name && want.Kind == have.Kind && want.Version == have.Version && want.Toolchain == have.Toolchain &&
		want.Group == have.Group && want.Licence == have.Licence && want.Copyright == have.Copyright &&
		want.CopyrightUnstated == have.CopyrightUnstated && want.Why == have.Why && want.Source == have.Source
}

// writtenRecord parses the record the data directory holds, or answers the
// refusal of one that does not parse: a record update would never write.
func writtenRecord(recordRel string, raw []byte) (record, *Refusal) {
	rec, err := decodeRecord(raw)
	if err != nil {
		problem := staleData(recordRel, "does not parse")
		return record{}, &problem
	}
	return rec, nil
}

// staleData is the refusal of generated data that update would write
// differently.
func staleData(work, problem string) Refusal {
	return Refusal{
		Work:    work,
		Problem: problem,
		Why:     "the binary embeds " + DataRel + ", so data that differs from the declaration and the vendored licence files ships a wrong credit",
		Fix:     updateCommand,
	}
}
