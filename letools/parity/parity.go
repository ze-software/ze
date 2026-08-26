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
//   - GATES. The 156 gates `./le gates --json` declares are the behavior that
//     must keep working. A gate is ported when a REGISTERED le command claims
//     it, and the claim is the only thing here anybody types by hand.
//   - FILES. Every code file left under scripts/ is work not yet moved. The
//     owner's end state is that scripts/ holds no code, and that is derived
//     from the filesystem with nothing claimed and nothing to keep in step.
//
// Four answers are red, and only the first two shrink as the port proceeds:
//
//   - a gate the Python le declares that no Go command claims;
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
	"github.com/ze-software/ze/letools/lepath"
	"github.com/ze-software/ze/letools/leroot"
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
	// Ported is how many gates a registered Go command claims.
	Ported int `json:"ported"`
	// Unported is Gates minus Ported, and it is the number the migration is
	// driving to zero.
	Unported int `json:"unported"`
	// CommandNames lists every root command the Go le registers.
	CommandNames []string `json:"command-names"`
	// UnportedGates names every gate that is still Python only.
	UnportedGates []string `json:"unported-gates"`
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

// Complete reports whether the census found nothing wrong. It is the exit
// code's only input.
func (c Census) Complete() bool {
	return c.Unported == 0 && c.ScriptFiles == 0 &&
		len(c.UnknownClaims) == 0 && len(c.UnwiredClaims) == 0
}

var (
	claimsMu sync.Mutex
	claims   = make(map[string][]string)
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

// claimSnapshot copies the claim table so a reader never holds the lock while
// it works, and so a test can compare against a value that cannot move.
func claimSnapshot() map[string][]string {
	claimsMu.Lock()
	defer claimsMu.Unlock()
	out := make(map[string][]string, len(claims))
	for command, gates := range claims {
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
	return Take(gates, scripts, claimSnapshot(), leroot.Owns, rootNames())
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

func Take(gates []Gate, scripts ScriptCount, claimed map[string][]string, registered func(string) bool, commands []string) Census {
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
	// Empty rather than nil: a caller reading `le parity | json` gets [] for
	// "nothing wrong here", never null, so the same key parses the same way in
	// every answer.
	unknown, unwired := []string{}, []string{}
	for command, gateNames := range claimed {
		wired := registered(command)
		for _, name := range gateNames {
			switch {
			case !declared[name]:
				unknown = append(unknown, claimLine(command, name))
			case !wired:
				unwired = append(unwired, claimLine(command, name))
			default:
				served[name] = true
			}
		}
	}

	unported := []string{}
	for _, gate := range gates {
		if !served[gate.Name] {
			unported = append(unported, gate.Name)
		}
	}

	sort.Strings(unported)
	sort.Strings(unknown)
	sort.Strings(unwired)

	return Census{
		Gates:                 len(gates),
		GateAreas:             len(areas),
		Commands:              len(commands),
		ScriptFiles:           scripts.Total,
		ScriptFilesByLanguage: scripts.ByLanguage,
		Ported:                len(served),
		Unported:              len(unported),
		CommandNames:          commands,
		ScriptDirs:            scripts.ByDir,
		UnportedGates:         unported,
		UnknownClaims:         unknown,
		UnwiredClaims:         unwired,
	}
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
