// Design: docs/architecture/third-party-credits.md -- the generated copy
// Overview: credits.go -- the declaration and the record this writes

package credits

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// inputs are what the tool reads besides files: the modules each shipped
// flavor links, and the Go files of the checkout. A test supplies both, so it
// runs neither the go command nor git.
type inputs struct {
	linked  func(root string) ([]flavorLinks, error)
	goFiles func(root string) ([]string, error)
}

// UpdateReport is what `deps credits update` answers: the files it wrote and
// removed under the data directory, relative to it.
type UpdateReport struct {
	Works     int      `json:"works"`
	Written   []string `json:"written"`
	Removed   []string `json:"removed"`
	Unchanged int      `json:"unchanged"`
}

// Text renders the update for a person.
func (r UpdateReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("deps credits update: ").Int(int64(r.Works)).Str(" works credited in ").Str(DataRel).Byte('\n')
	for _, name := range r.Written {
		tb.Str("  wrote   ").Str(name).Byte('\n')
	}
	for _, name := range r.Removed {
		tb.Str("  removed ").Str(name).Byte('\n')
	}
	tb.Str("  ").Int(int64(r.Unchanged)).Str(" files unchanged\n")
	return tb.String()
}

// Update writes the data directory from the declaration and the vendored
// licence files. A declaration it cannot generate from (a shipped module with
// no entry, a work with no licence file) is answered as refusals, and nothing
// is written. Two runs over one tree write the same bytes.
func Update(root string, in inputs) (UpdateReport, []Refusal, error) {
	out, refusals, err := planTree(root, in)
	if err != nil {
		return UpdateReport{}, nil, err
	}
	if len(refusals) != 0 {
		return UpdateReport{}, refusals, nil
	}
	report, err := write(root, out)
	if err != nil {
		return UpdateReport{}, nil, err
	}
	return report, nil, nil
}

// planTree reads the declaration and the linked sets and answers what update
// writes.
func planTree(root string, in inputs) (generated, []Refusal, error) {
	raw, err := readRel(root, DeclarationRel)
	if err != nil {
		return generated{}, nil, err
	}
	links, err := in.linked(root)
	if err != nil {
		return generated{}, nil, err
	}
	return plan(root, raw, links)
}

// plan derives every file update writes, from the declaration's bytes and
// the linked sets: one record per shipped work (a module a flavor links,
// every works entry), each licence file stored once under its digest.
func plan(root string, raw []byte, links []flavorLinks) (generated, []Refusal, error) {
	decl, err := ReadDeclaration(raw)
	if err != nil {
		return generated{}, nil, err
	}
	byModule := shipped(links)
	refusals := undeclaredModules(decl, byModule)
	refusals = append(refusals, declaredTwice(decl)...)
	rec := record{GeneratedBy: updateCommand, Declaration: digest(raw)}
	files := make(map[string][]byte)
	all := decl.entries()
	for index := range all {
		item := &all[index]
		if !ships(*item, byModule) {
			continue
		}
		if _, done := rec.work(item.name); done {
			continue
		}
		origins, problem, err := workFiles(root, *item)
		if err != nil {
			return generated{}, nil, err
		}
		if problem != nil {
			refusals = append(refusals, *problem)
			continue
		}
		work := workRecord{
			Name: item.name, Kind: item.kind, Version: item.version, Toolchain: item.toolchain, Group: item.group,
			Licence: item.licence, Copyright: item.copyright, CopyrightUnstated: item.unstated,
			Why: item.why, Source: item.source, Files: make([]fileRecord, 0, len(origins)),
		}
		for _, origin := range origins {
			content, err := readRel(root, origin)
			if err != nil {
				return generated{}, nil, err
			}
			name := textName(content)
			files[name] = content
			work.Files = append(work.Files, fileRecord{Name: path.Base(origin), Origin: origin, Text: name})
		}
		rec.Works = append(rec.Works, work)
		// entries() walks the declaration in order, so the first shipped work
		// of each group places the group.
		if !slices.Contains(rec.Groups, item.group) {
			rec.Groups = append(rec.Groups, item.group)
		}
	}
	slices.SortFunc(rec.Works, func(a, b workRecord) int { return strings.Compare(a.Name, b.Name) })
	encoded, err := encodeRecord(rec)
	if err != nil {
		return generated{}, nil, err
	}
	files[RecordName] = encoded
	return generated{record: rec, files: files}, refusals, nil
}

// ships answers whether a declared entry is part of the shipped binary: a
// module when a flavor links it, a work always, because the binary decides
// at run time which works it carries.
func ships(item entry, byModule map[string][]string) bool {
	if item.kind == kindWork {
		return true
	}
	_, linked := byModule[item.name]
	return linked
}

// workFiles answers the repository paths of a work's licence and notice
// files, or the refusal of a work that has none.
func workFiles(root string, item entry) ([]string, *Refusal, error) {
	if item.kind == kindModule {
		origins, problem := moduleLicenceFiles(root, item)
		if problem != nil {
			return nil, problem, nil
		}
		// A module's declared licence-files follow the files at its root.
		missing, err := firstMissing(root, item.licenceFiles)
		if err != nil {
			return nil, nil, err
		}
		if missing != "" {
			return nil, noLicence(item, "lists "+missing+" under licence-files, which does not exist"), nil
		}
		return append(origins, item.licenceFiles...), nil, nil
	}
	if len(item.declaredFiles) == 0 {
		return nil, noLicence(item, "lists no licence file under files"), nil
	}
	missing, err := firstMissing(root, item.declaredFiles)
	if err != nil {
		return nil, nil, err
	}
	if missing != "" {
		return nil, noLicence(item, "lists "+missing+" under files, which does not exist"), nil
	}
	return item.declaredFiles, nil, nil
}

// moduleLicenceFiles answers the licence and notice files at the root of a
// vendored module, or the refusal of a module that has none or no vendored
// copy.
func moduleLicenceFiles(root string, item entry) ([]string, *Refusal) {
	origins, err := moduleFiles(root, item.name)
	if err != nil {
		return nil, noLicence(item, "has no vendored copy under "+path.Join(vendorRoot, item.name)+": "+err.Error())
	}
	if len(origins) == 0 {
		return nil, noLicence(item, "has no licence or notice file at the root of "+path.Join(vendorRoot, item.name))
	}
	return origins, nil
}

// firstMissing answers the first of the repository paths that does not
// exist, "" when all do, and an error when one cannot be looked at.
func firstMissing(root string, origins []string) (string, error) {
	for _, origin := range origins {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(origin)))
		if err == nil {
			continue
		}
		if errors.Is(err, fs.ErrNotExist) {
			return origin, nil
		}
		return "", errors.New("read " + origin + ": " + err.Error())
	}
	return "", nil
}

// noLicence is the refusal of a shipped work whose licence text is missing.
func noLicence(item entry, problem string) *Refusal {
	return &Refusal{
		Work:    item.name,
		Problem: problem,
		Why:     "the binary must carry the licence text of every work it ships, and ze thanks licence prints it",
		Edit:    "fetch the licence text from the work's upstream at the version Ze carries, commit it under third_party/, and list it in the entry's files",
		Fix:     updateCommand,
	}
}

// write replaces the data directory's files with out, removes every file no
// work references, and answers what it changed.
func write(root string, out generated) (UpdateReport, error) {
	dataDir := filepath.Join(root, filepath.FromSlash(DataRel))
	if err := os.MkdirAll(filepath.Join(dataDir, TextsDir), 0o750); err != nil {
		return UpdateReport{}, errors.New("create " + DataRel + ": " + err.Error())
	}
	report := UpdateReport{Works: len(out.record.Works)}
	names := make([]string, 0, len(out.files))
	for name := range out.files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		target := filepath.Join(dataDir, filepath.FromSlash(name))
		current, err := os.ReadFile(target) //nolint:gosec // a file of the data directory this tool owns
		if err == nil && bytes.Equal(current, out.files[name]) {
			report.Unchanged++
			continue
		}
		if err := os.WriteFile(target, out.files[name], 0o600); err != nil {
			return UpdateReport{}, errors.New("write " + path.Join(DataRel, name) + ": " + err.Error())
		}
		report.Written = append(report.Written, name)
	}
	stale, err := unreferenced(root, out)
	if err != nil {
		return UpdateReport{}, err
	}
	for _, name := range stale {
		if err := os.Remove(filepath.Join(dataDir, filepath.FromSlash(name))); err != nil {
			return UpdateReport{}, errors.New("remove " + path.Join(DataRel, name) + ": " + err.Error())
		}
		report.Removed = append(report.Removed, name)
	}
	return report, nil
}

// unreferenced answers the files under the data directory, relative to it,
// that out does not hold: what update removes and check refuses. A data
// directory that does not exist holds none.
func unreferenced(root string, out generated) ([]string, error) {
	dataDir := filepath.Join(root, filepath.FromSlash(DataRel))
	var stale []string
	err := filepath.WalkDir(dataDir, func(full string, item fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) && full == dataDir {
			return filepath.SkipAll
		}
		if err != nil {
			return err
		}
		if item.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dataDir, full)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if _, held := out.files[name]; held {
			return nil
		}
		stale = append(stale, name)
		return nil
	})
	if err != nil {
		return nil, errors.New("read " + DataRel + ": " + err.Error())
	}
	slices.Sort(stale)
	return stale, nil
}
