// Design: (none -- build tool)
//
// Detail: report.go -- the answer this file produces
//
// Package inventory is the `ze-inventory` gate: what ze is made of, counted.
// Plugins, address families, capabilities, YANG modules, RPCs and their .ci
// coverage, functional-test files, and Go package statistics.
//
// It blank-imports the product's composition root, which is what makes the
// answer accurate: the plugins and the YANG modules come from the same
// registrations the daemon runs, never from a regular expression over source.
// That import is allowed in exactly this direction
// (plan/spec-le-is-a-ze-binary.md, AC-3): le may link ze to introspect it, ze
// never links le, and le never RUNS a product command (internal/le/leroot/dispatch.go).
//
// EVERY NUMBER HERE IS A COUNT OF WHAT THE WALK SAW, so a walk that ends early
// lowers a published count with nothing said, under a header claiming the
// output is always accurate. A scan error was already fatal before the port;
// a walk error and a file this tool cannot read now are too. See walkFiles, and
// see vanished for the one entry that is neither: a file removed between the
// walk listing it and this tool reaching it was never part of the tree.

package inventory

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
	"strings"
	"time"

	// The blank import triggers every plugin, YANG and RPC registration, so
	// the inventory reports the product rather than a subset of it.
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// generatedLayout is the timestamp the page carries. Minute resolution, UTC,
// as the page has always carried it.
const generatedLayout = "2006-01-02 15:04 UTC"

// codeAreas are the top-level directories the Go statistics cover, in the
// order the table lists them.
var codeAreas = []string{"internal", "pkg", "cmd"}

// walkFiles calls visit for every file under dir whose name ends in suffix.
//
// A walk error is FATAL, with TWO exceptions, and both are facts about the tree
// rather than a scan that fell short. dir itself not existing means the tree has
// no such area. An entry that vanished was never part of the tree either (see
// vanished). Every other error means this walk saw less than the tree holds, and
// every number this tool publishes is a count of what the walk saw.
//
// The script this was ported from returned nil for every walk error and for
// every file it could not open, so a dangling symbolic link under internal/
// lowered the published line count and a directory it could not enter lowered
// all of them, silently.
func walkFiles(dir, suffix string, visit func(path string, entry fs.DirEntry) error) error {
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == dir && errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipAll
			}
			if vanished(path, err) {
				return nil
			}
			return fmt.Errorf("reading %s: %w", path, err)
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			return nil
		}
		return visit(path, entry)
	})
}

// vanished reports whether a path the walk listed has since ceased to exist.
//
// A file can be removed between the walk reading a directory and this tool
// reaching what it listed, and this checkout is shared: another session's
// temporary file under tmp/ disappears mid-walk routinely. Such a file is not
// part of the tree being reported on, so it is skipped rather than counted or
// refused.
//
// It is NOT the same as a file the walk lists and cannot read. A dangling
// symbolic link fails to open with the same error and its DIRECTORY ENTRY is
// still there, so Lstat still succeeds and this answers false. That one makes a
// published count short, and it is refused.
func vanished(path string, err error) bool {
	if !errors.Is(err, fs.ErrNotExist) {
		return false
	}
	_, statErr := os.Lstat(path)
	return errors.Is(statErr, fs.ErrNotExist)
}

// scanLines feeds every line of the file at path to visit.
//
// bufio.Scanner stops on EOF, on a read error and on a line above
// bufio.MaxScanTokenSize alike, and reports only the last two. A partial read
// lowers a published count, so the error is returned rather than absorbed.
//
// A file that vanished contributes no line and is not an error: see vanished.
// It stays COUNTED where the walk counts files, which is what the script does
// and the one number nobody can take twice.
func scanLines(path string, visit func(line string)) error {
	file, err := os.Open(path) //nolint:gosec // path comes from a walk of the tree the caller named
	if err != nil {
		if vanished(path, err) {
			return nil
		}
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer file.Close() //nolint:errcheck // read-only

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		visit(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	return nil
}

// Collect reads the tree at root and answers everything the inventory holds.
// The registry-backed parts come from this process; the counted parts come from
// the tree, so the answer does not change with the working directory the caller
// happens to have.
func Collect(root string) (Inventory, error) {
	inv := Inventory{
		Generated:    time.Now().UTC().Format(generatedLayout),
		Families:     registry.FamilyMap(),
		Capabilities: capabilityCodes(),
	}

	for _, reg := range registry.All() {
		inv.Plugins = append(inv.Plugins, Plugin{
			Name:         reg.Name,
			Description:  reg.Description,
			Families:     reg.Families,
			Capabilities: reg.CapabilityCodes,
			Dependencies: reg.Dependencies,
			ConfigRoots:  reg.ConfigRoots,
			RFCs:         reg.RFCs,
			Features:     reg.Features,
			HasYANG:      reg.YANG != "",
			HasDecoder:   reg.InProcessNLRIDecoder != nil,
			HasEncoder:   reg.InProcessNLRIEncoder != nil,
		})
	}
	inv.FamilySupport = collectFamilySupport()

	yangPaths, err := discoverYANGPaths(root)
	if err != nil {
		return Inventory{}, err
	}
	inv.YANGModules = describeModules(yangPaths)

	inv.RPCsByModule, inv.RPCList, err = extractRPCs(root)
	if err != nil {
		return Inventory{}, err
	}
	for _, count := range inv.RPCsByModule {
		inv.TotalRPCs += count
	}

	testDir := filepath.Join(root, "test")
	ciContent, err := loadCIContent(testDir)
	if err != nil {
		return Inventory{}, err
	}
	for i, rpc := range inv.RPCList {
		inv.RPCList[i].Covered = rpcHasCoverage(rpc.Name, ciContent)
	}

	inv.TestCounts, err = countCITests(testDir)
	if err != nil {
		return Inventory{}, err
	}
	for _, count := range inv.TestCounts {
		inv.TotalTests += count
	}

	for _, area := range codeAreas {
		stats, err := countGoStats(filepath.Join(root, area))
		if err != nil {
			return Inventory{}, err
		}
		var tb textbuf.Buffer
		stats.Area = tb.Str(area).Byte('/').String()
		inv.PackageStats = append(inv.PackageStats, stats)
	}

	return inv, nil
}

// capabilityCodes answers the capability map keyed by the code's decimal
// spelling, which is what the JSON object needs.
func capabilityCodes() map[string]string {
	codes := registry.CapabilityMap()
	out := make(map[string]string, len(codes))
	for code, name := range codes {
		// textbuf.StringUint rather than strconv.FormatUint: `performance.md`
		// names the two as one substitution, and c_sprintf_new enforces it.
		out[textbuf.StringUint(uint64(code))] = name
	}
	return out
}

// describeModules pairs every loaded YANG module with where it came from:
// "plugin:<dir>" when its file sits under a plugin directory, "infrastructure"
// otherwise.
func describeModules(yangPaths map[string]string) []YANGModule {
	loaded := yang.Modules()
	modules := make([]YANGModule, 0, len(loaded))
	for _, module := range loaded {
		source := "infrastructure"
		if path, found := yangPaths[module.Name]; found {
			if dir, under := pluginDirOf(path); under {
				var tb textbuf.Buffer
				source = tb.Str("plugin:").Str(dir).String()
			}
		}
		modules = append(modules, YANGModule{Name: module.Name, Source: source})
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Name < modules[j].Name })
	return modules
}

// pluginDirOf answers the plugin directory a path sits under, and whether it
// sits under one at all.
func pluginDirOf(path string) (dir string, under bool) {
	_, after, found := strings.Cut(path, "/plugins/")
	if !found {
		return "", false
	}
	dir, _, _ = strings.Cut(after, "/")
	return dir, true
}

// discoverYANGPaths maps every .yang file's base name to its path relative to
// root. The base name is the key because that is what a module is named by, and
// two files sharing one name leave the last one walked.
func discoverYANGPaths(root string) (map[string]string, error) {
	paths := make(map[string]string)
	err := walkFiles(root, ".yang", func(path string, entry fs.DirEntry) error {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("locating %s under %s: %w", path, root, err)
		}
		paths[entry.Name()] = rel
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// collectFamilySupport answers one row per address family: which plugin owns it
// and what that plugin can do with it. The first plugin claiming a family owns
// the row.
func collectFamilySupport() []Family {
	var result []Family
	seen := make(map[string]bool)

	for _, reg := range registry.All() {
		for _, family := range reg.Families {
			if seen[family] {
				continue
			}
			seen[family] = true
			result = append(result, Family{
				Family:      family,
				Plugin:      reg.Name,
				Decode:      reg.InProcessNLRIDecoder != nil,
				Encode:      reg.InProcessNLRIEncoder != nil,
				RouteEncode: reg.InProcessRouteEncoder != nil,
				ConfigNLRI:  reg.InProcessConfigNLRIBuilder != nil,
			})
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Family < result[j].Family })
	return result
}

// extractRPCs answers the per-module RPC counts and the flat list of every RPC
// declared by a .yang file under root.
func extractRPCs(root string) (map[string]int, []RPC, error) {
	counts := make(map[string]int)
	var rpcs []RPC

	err := walkFiles(root, ".yang", func(path string, entry fs.DirEntry) error {
		module := entry.Name()
		count := 0
		if err := scanLines(path, func(text string) {
			line := strings.TrimSpace(text)
			if !strings.HasPrefix(line, "rpc ") {
				return
			}
			count++
			rpcs = append(rpcs, RPC{Name: rpcNameIn(line), Module: module})
		}); err != nil {
			return err
		}
		if count > 0 {
			counts[module] = count
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	sort.Slice(rpcs, func(i, j int) bool {
		if rpcs[i].Module != rpcs[j].Module {
			return rpcs[i].Module < rpcs[j].Module
		}
		return rpcs[i].Name < rpcs[j].Name
	})
	return counts, rpcs, nil
}

// rpcNameIn answers the RPC name a declaration line carries: "rpc foo-bar {"
// and "rpc foo { description ... }" both name their RPC in the second word.
func rpcNameIn(line string) string {
	name := strings.TrimPrefix(line, "rpc ")
	if idx := strings.IndexAny(name, " {"); idx >= 0 {
		name = name[:idx]
	}
	return name
}

// loadCIContent reads every .ci file under testDir into one string, which is
// what the coverage search reads.
func loadCIContent(testDir string) (string, error) {
	var buf strings.Builder
	err := walkFiles(testDir, ".ci", func(path string, _ fs.DirEntry) error {
		data, err := os.ReadFile(path) //nolint:gosec // path comes from a walk of the tree the caller named
		if err != nil {
			if vanished(path, err) {
				return nil
			}
			return fmt.Errorf("reading %s: %w", path, err)
		}
		buf.Write(data)     //nolint:errcheck // strings.Builder never fails
		buf.WriteByte('\n') //nolint:errcheck // strings.Builder never fails
		return nil
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// rpcHasCoverage reports whether an RPC name appears in the .ci content, in any
// of the three spellings a test can use it in: the hyphenated wire name, the
// spaced CLI form, and the spaced form with a glob between the first two words.
func rpcHasCoverage(rpcName, ciContent string) bool {
	if strings.Contains(ciContent, rpcName) {
		return true
	}
	spaced := strings.ReplaceAll(rpcName, "-", " ")
	if strings.Contains(ciContent, spaced) {
		return true
	}
	first, rest, found := strings.Cut(spaced, " ")
	if !found {
		return false
	}
	var glob strings.Builder
	glob.WriteString(first) //nolint:errcheck // strings.Builder never fails
	glob.WriteString(" * ") //nolint:errcheck // strings.Builder never fails
	glob.WriteString(rest)  //nolint:errcheck // strings.Builder never fails
	return strings.Contains(ciContent, glob.String())
}

// countCITests counts the .ci files under each immediate subdirectory of
// testDir. A subdirectory holding none is left out of the answer.
func countCITests(testDir string) (map[string]int, error) {
	counts := make(map[string]int)
	entries, err := os.ReadDir(testDir)
	if errors.Is(err, fs.ErrNotExist) {
		return counts, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", testDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		count := 0
		walkErr := walkFiles(filepath.Join(testDir, entry.Name()), ".ci",
			func(string, fs.DirEntry) error {
				count++
				return nil
			})
		if walkErr != nil {
			return nil, walkErr
		}
		if count > 0 {
			counts[entry.Name()] = count
		}
	}
	return counts, nil
}

// countGoStats counts the packages, files and lines of Go under dir. A package
// is a directory holding at least one .go file.
func countGoStats(dir string) (AreaStats, error) {
	var stats AreaStats
	packages := make(map[string]bool)

	err := walkFiles(dir, ".go", func(path string, _ fs.DirEntry) error {
		pkg := filepath.Dir(path)
		if !packages[pkg] {
			packages[pkg] = true
			stats.Packages++
		}
		stats.Files++
		return scanLines(path, func(string) { stats.Lines++ })
	})
	if err != nil {
		return AreaStats{}, err
	}
	return stats, nil
}

// Answer is the `le inventory` command. It takes no arguments: the tree is the
// checkout, and the rendering is the operator's to choose with a pipe operator
// (ai/rules/cli.md).
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "error: inventory takes no arguments, got %q\n", args[0]) //nolint:errcheck // CLI output
		fmt.Fprintln(os.Stderr, "usage: le inventory [| json | yaml | table]")           //nolint:errcheck // CLI output
		return nil, 1
	}

	root, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}

	inv, err := Collect(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}
	return inv, 0
}

// Inventory answers a rendering of itself, so the bare command prints the page
// the gate has always printed.
var _ leroot.Prose = Inventory{}
