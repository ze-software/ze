// Design: docs/architecture/core-design.md -- the le migration census
//
// Package parity measures how much of the Python le has become Go, and it is
// the reason the migration may run two implementations at once. `le` is being
// ported gate by gate while the Python original keeps working, which is a
// duplicate-then-swap route rather than the delete-X-first route
// ai/rules/no-layering.md asks for. That rule exists so two implementations
// cannot drift apart in silence. This census is what removes the silence: it
// reads the Python side for the denominator, reads the Go registry for the
// numerator, NAMES every gate that is still Python, and refuses to answer 0
// while any remain.
//
// It measures two populations, because the migration has two end states and
// neither implies the other.
//
//   - GATES. The 158 gates `./le gates --json` declares are the behavior that
//     must keep working. Each one is in one of THREE states, and the claim is
//     the only thing here anybody types by hand.
//   - FILES. Every code file left under scripts/ is work not yet moved. The
//     owner's end state is that scripts/ holds no code, and that is derived
//     from the filesystem with nothing claimed and nothing to keep in step.
//
// A gate has three states.
// Counting a claimed gate as ported can overstate the migration, so this census distinguishes the states:
//
//   - CONVERTED. A registered le command claims the gate and performs the work in Go.
//   - FORKED. A registered le command claims the gate, but its driver still starts a script.
//     For example, `le deployment vpp-test` runs python3 scripts/evidence/effective-vpp.py.
//     The area is ported, but the work is not. The census counts and names these rows.
//   - UNPORTED. No Go command claims the gate.
//
// The census derives whether a claimed gate is converted or forked.
// leaction.Area.ForkedGates inspects each action argv for a .py or .sh file
// (internal/le/leaction, forksAScript).
//
// Five answers are red, and only the first three shrink as the port proceeds:
//
//   - a gate the Python le declares that no Go command claims;
//   - a gate whose command was written and whose driver is still a script;
//   - a code file still sitting under scripts/;
//   - a claim naming a gate the Python le does not declare, which is what a
//     renamed or deleted gate looks like from this side;
//   - a claim whose command never registered a root handler, which is what a
//     tool that was written but not wired looks like.
package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// gatesTimeout bounds the Python side's answer. `./le gates --json` builds its
// answer from data and forks nothing, and it was measured at 147 ms on this
// machine on 2026-08-26, so a run past this bound is a hung interpreter rather
// than a slow one.
const gatesTimeout = 30 * time.Second

// gatesOutputMax bounds what the census will read back. 156 gates render as
// roughly 90 KB, so this leaves two orders of magnitude of headroom and still
// refuses an interpreter that streams forever.
const gatesOutputMax = 8 << 20

// Gate is one gate the Python le declares. The fields are the ones the census
// needs; `./le gates --json` publishes more, and the decoder ignores them.
type Gate struct {
	Area string `json:"area"`
	Name string `json:"name"`
	Why  string `json:"why"`
}

// Census is what `le parity` answers. Every count is derived: nothing here is
// typed in by hand, which is what "measured, not asserted" means.
type Census struct {
	// Gates is the denominator: how many gates the Python le declares.
	Gates int `json:"gates"`
	// GateAreas is how many areas those gates belong to.
	GateAreas int `json:"gate-areas"`
	// Commands is how many root commands the Go le registers.
	Commands int `json:"commands"`
	// ScriptFiles is how many code files are still under scripts/. The spec's
	// end state is that this reaches zero.
	ScriptFiles int `json:"script-files"`
	// ScriptFilesByLanguage counts those files per extension, which is how the
	// spec's own step table is sized.
	ScriptFilesByLanguage map[string]int `json:"script-files-by-language"`
	// Converted is how many gates a registered Go command claims AND does the
	// work in Go for. It is the only number that says work has moved.
	Converted int `json:"converted"`
	// Forked is how many gates a registered Go command claims while its driver
	// still starts a script. The command exists, the gate is reachable, and the
	// work has not moved, so this is counted apart from Converted rather than
	// added to it.
	Forked int `json:"forked"`
	// Unported is how many gates no Go command claims. Converted, Forked and
	// Unported partition Gates, and the migration drives the last two to zero.
	Unported int `json:"unported"`
	// CommandNames lists every root command the Go le registers.
	CommandNames []string `json:"command-names"`
	// UnportedGates names every gate that is still Python only.
	UnportedGates []string `json:"unported-gates"`
	// ForkedGates names every gate a Go command claims whose driver is still a
	// script. Naming them is what makes the third state actionable: each one is
	// a port somebody still owes.
	ForkedGates []string `json:"forked-gates"`
	// ScriptDirs names every directory under scripts/ that still holds code,
	// with its file count. The whole list is not named: 280 paths is a wall
	// rather than an answer, and the directory is the unit the step table
	// ports.
	ScriptDirs map[string]int `json:"script-dirs"`
	// UnknownClaims names every claim whose gate the Python le does not
	// declare.
	UnknownClaims []string `json:"unknown-claims"`
	// UnwiredClaims names every claim whose command registered no root
	// handler, so the tool exists and nothing can reach it.
	UnwiredClaims []string `json:"unwired-claims"`
}

// Complete reports whether the census found no errors. This result alone sets the exit code.
// A forked gate is incomplete, like an unported gate.
// The migration is complete only after both the command and its work are in Go.
func (c Census) Complete() bool {
	return c.Unported == 0 && c.Forked == 0 && c.ScriptFiles == 0 &&
		len(c.UnknownClaims) == 0 && len(c.UnwiredClaims) == 0
}

var (
	claimsMu sync.Mutex
	claims   = make(map[string][]string)
	forked   = make(map[string][]string)
)

// Claim records that the Go command `command` now serves these Python gates.
// A ported tool calls it from the same init() that registers the command, so
// the count falls in the commit that does the porting and in no other.
//
// Safe for concurrent use, though every caller in practice is an init().
func Claim(command string, gates ...string) {
	claimsMu.Lock()
	defer claimsMu.Unlock()
	claims[command] = append(claims[command], gates...)
}

// ClaimForked records that the Go command `command` reaches these gates by
// starting a script rather than by doing the work. They are claimed and they
// are NOT converted, and the census counts and names them apart.
//
// The caller passes leaction.Area.ForkedGates, which derives the list from the
// argv each action starts. Nothing here is a second hand-typed list: an area
// that stops forking a driver stops appearing in this population with no edit
// to its register.go.
//
// A gate named here need not also be passed to Claim. Both are claims, and this
// one carries the extra fact.
func ClaimForked(command string, gates ...string) {
	claimsMu.Lock()
	defer claimsMu.Unlock()
	forked[command] = append(forked[command], gates...)
}

// claimSnapshot copies both claim tables while it holds the lock. A reader then
// uses stable values, and a test compares a snapshot that cannot change.
func claimSnapshot() (claimed, forkedClaims map[string][]string) {
	claimsMu.Lock()
	defer claimsMu.Unlock()
	return cloneClaims(claims), cloneClaims(forked)
}

// cloneClaims copies one claim table. The caller holds the lock.
func cloneClaims(table map[string][]string) map[string][]string {
	out := make(map[string][]string, len(table))
	for command, gates := range table {
		out[command] = slices.Clone(gates)
	}
	return out
}

// Answer is the `le parity` command. It takes no arguments: the rendering is
// the operator's to choose with a pipe operator, so there is no --json flag
// here (ai/rules/cli.md).
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "error: parity takes no arguments, got %q\n", args[0]) //nolint:errcheck // CLI output
		fmt.Fprintln(os.Stderr, "usage: le parity [| json | yaml | table]")           //nolint:errcheck // CLI output
		return nil, 1
	}

	root, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}

	gates, err := ReadGates(context.Background(), root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}

	scripts, err := CountScripts(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}

	census := takeHere(gates, scripts)
	if census.Complete() {
		return census, 0
	}
	return census, 1
}

// takeHere is the census of THIS process: the gates the Python le declares, the
// code files under scripts/, the claims tools made from their init(), and the
// commands le owns.
//
// It exists so there is ONE statement of which registry answers "is this
// command wired". The answer is leroot.Owns rather than
// registry.HasRootHandler, because le LINKS the product: a tool that
// introspects ze loads ze's registry to read it, so ze's root commands are in
// this process too, and a claim must not count as ported because ZE owns the
// name.
func takeHere(gates []Gate, scripts ScriptCount) Census {
	claimed, forkedClaims := claimSnapshot()
	return Take(gates, scripts, claimed, forkedClaims, leroot.Owns, rootNames())
}

// rootNames answers every root command LE OWNS, which is every tool its
// composition root imported.
//
// It reads le's own list rather than the registry's, because le LINKS the
// product: a tool that introspects ze loads ze's registry to read it, so this
// process's registry carries ze's root commands too (five, measured
// 2026-08-26). Counting those would say the migration had ported commands
// nobody wrote, and would let a claim pass because ZE registered the name.
func rootNames() []string {
	names := leroot.Owned()
	sort.Strings(names)
	return names
}

// ReadGates asks the Python le for every gate it declares. That side is the
// denominator for as long as it owns the gates, and it stays the denominator
// until the swap moves the declaration into Go.
func ReadGates(ctx context.Context, root string) ([]Gate, error) {
	ctx, cancel := context.WithTimeout(ctx, gatesTimeout)
	defer cancel()

	shim := filepath.Join(root, "le")
	// The path is the checkout's own `le` shim, built from the root lepath
	// discovered from the markers or named by ZE_REPO_ROOT. le is a build-host
	// tool and its argv is a developer's; nothing here is reachable from a
	// network peer.
	cmd := exec.CommandContext(ctx, shim, "gates", "--json") //nolint:gosec // the executable is <repo>/le, resolved by lepath.Root
	cmd.Dir = root
	cmd.Stderr = os.Stderr

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("parity: read %s gates: %w", shim, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("parity: run %s gates --json: %w", shim, err)
	}
	raw, readErr := io.ReadAll(io.LimitReader(pipe, gatesOutputMax+1))
	waitErr := cmd.Wait()
	switch {
	case readErr != nil:
		return nil, fmt.Errorf("parity: read %s gates: %w", shim, readErr)
	case waitErr != nil:
		return nil, fmt.Errorf("parity: run %s gates --json: %w", shim, waitErr)
	case len(raw) > gatesOutputMax:
		return nil, fmt.Errorf("parity: %s gates --json answered more than %d bytes", shim, gatesOutputMax)
	}

	var gates []Gate
	if err := json.Unmarshal(raw, &gates); err != nil {
		return nil, fmt.Errorf("parity: decode %s gates --json: %w", shim, err)
	}
	if len(gates) == 0 {
		return nil, fmt.Errorf("parity: %s gates --json declared no gate", shim)
	}
	return gates, nil
}

// Take builds the census from its three inputs. It is a pure function so a
// test can drive every red it can produce without a checkout that has those
// faults in it.
//
// registered answers whether a root handler exists for a command name, and
// commands is every root command registered in this process. scripts is the
// filesystem half of the answer, counted by CountScripts.
// claimLine renders one "<command> claims <gate>" row.
//
// textbuf rather than `+`: `performance.md` bans building strings by
// concatenation. This is a cold path -- it runs once per claimed gate when the
// census reports -- so the reason to use textbuf here is one spelling for
// string building across the repository, not speed.
func claimLine(command, gate string) string {
	var tb textbuf.Buffer
	return tb.Str(command).Str(" claims ").Str(gate).String()
}

func Take(gates []Gate, scripts ScriptCount, claimed, forkedClaims map[string][]string, registered func(string) bool, commands []string) Census {
	declared := make(map[string]bool, len(gates))
	areas := make(map[string]bool)
	for _, gate := range gates {
		declared[gate.Name] = true
		areas[gate.Area] = true
	}

	// A claim counts only when the command behind it is reachable. A tool
	// written and never wired leaves the gate unported, which is what the
	// spec's "a step that ports a tool and leaves the count unchanged has not
	// wired it" constraint asks the census to see.
	served := make(map[string]bool, len(gates))
	// stillAScript is the subset of served whose driver has not moved. It is a
	// SUBSET rather than a separate population: the gate is reachable, so it is
	// served, and what it is not is converted work.
	stillAScript := make(map[string]bool)
	// Empty rather than nil: a caller reading `le parity | json` gets [] for
	// "nothing wrong here", never null, so the same key parses the same way in
	// every answer.
	unknown, unwired := []string{}, []string{}
	for _, entry := range mergeClaims(claimed, forkedClaims) {
		wired := registered(entry.command)
		switch {
		case !declared[entry.gate]:
			unknown = append(unknown, claimLine(entry.command, entry.gate))
		case !wired:
			unwired = append(unwired, claimLine(entry.command, entry.gate))
		default:
			served[entry.gate] = true
			if entry.forked {
				stillAScript[entry.gate] = true
			}
		}
	}

	unported, forkedGates := []string{}, []string{}
	for _, gate := range gates {
		switch {
		case !served[gate.Name]:
			unported = append(unported, gate.Name)
		case stillAScript[gate.Name]:
			forkedGates = append(forkedGates, gate.Name)
		}
	}

	sort.Strings(unported)
	sort.Strings(forkedGates)
	sort.Strings(unknown)
	sort.Strings(unwired)

	return Census{
		Gates:                 len(gates),
		GateAreas:             len(areas),
		Commands:              len(commands),
		ScriptFiles:           scripts.Total,
		ScriptFilesByLanguage: scripts.ByLanguage,
		Converted:             len(served) - len(forkedGates),
		Forked:                len(forkedGates),
		Unported:              len(unported),
		CommandNames:          commands,
		ScriptDirs:            scripts.ByDir,
		UnportedGates:         unported,
		ForkedGates:           forkedGates,
		UnknownClaims:         unknown,
		UnwiredClaims:         unwired,
	}
}

// claimEntry is one command's claim on one gate, with the fact that decides
// which of the two claimed states it is in.
type claimEntry struct {
	command string
	gate    string
	forked  bool
}

// mergeClaims returns both claim tables as one stable list.
// Each command-and-gate pair appears once.
//
// Claim and ClaimForked can name the same gate, which is one claim with an extra property.
// A duplicate would report one mistake twice in unknown-claims.
// Stable ordering also prevents rows from moving between census runs.
func mergeClaims(claimed, forkedClaims map[string][]string) []claimEntry {
	commands := make([]string, 0, len(claimed)+len(forkedClaims))
	for command := range claimed {
		commands = append(commands, command)
	}
	for command := range forkedClaims {
		if _, both := claimed[command]; !both {
			commands = append(commands, command)
		}
	}
	sort.Strings(commands)

	entries := make([]claimEntry, 0, len(commands))
	for _, command := range commands {
		isForked := make(map[string]bool, len(forkedClaims[command]))
		for _, gate := range forkedClaims[command] {
			isForked[gate] = true
		}
		seen := make(map[string]bool, len(claimed[command])+len(forkedClaims[command]))
		for _, gate := range slices.Concat(claimed[command], forkedClaims[command]) {
			if seen[gate] {
				continue
			}
			seen[gate] = true
			entries = append(entries, claimEntry{command: command, gate: gate, forked: isForked[gate]})
		}
	}
	return entries
}

// ScriptCount is the filesystem half of the census: how much code is still
// under scripts/, and where.
type ScriptCount struct {
	Total      int
	ByLanguage map[string]int
	ByDir      map[string]int
}

// scriptLanguages are the extensions that count as code. A fixture, a golden
// file or a README is not work to port, so only these three are counted, and
// they are the three the spec's step table sizes itself with.
var scriptLanguages = [...]string{".py", ".go", ".sh"}

// scriptFilesMax bounds the walk. scripts/ held 280 code files on 2026-08-26;
// a tree ten times that size is a mount that does not belong here, and the
// census refuses it rather than walking it.
const scriptFilesMax = 5000

// CountScripts walks scripts/ and counts the code files left in it. Nothing is
// claimed and nothing is declared: the answer is the filesystem, so it cannot
// drift from what the port has actually done.
func CountScripts(root string) (ScriptCount, error) {
	count := ScriptCount{ByLanguage: map[string]int{}, ByDir: map[string]int{}}
	dir := filepath.Join(root, "scripts")

	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		// The end state: the swap removed the tree.
		return count, nil
	}

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "__pycache__" || entry.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !slices.Contains(scriptLanguages[:], ext) {
			return nil
		}
		count.Total++
		if count.Total > scriptFilesMax {
			return fmt.Errorf("parity: more than %d code files under %s", scriptFilesMax, dir)
		}
		count.ByLanguage[ext]++
		if rel, relErr := filepath.Rel(root, filepath.Dir(path)); relErr == nil {
			count.ByDir[filepath.ToSlash(rel)]++
		}
		return nil
	})
	if err != nil {
		return ScriptCount{}, fmt.Errorf("parity: count %s: %w", dir, err)
	}
	return count, nil
}
