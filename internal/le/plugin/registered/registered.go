// Design: docs/architecture/plugin/plugin-system.md -- Registration fields, the runtime list equals the declared list
// Related: registered_linux_test.go -- TestBuiltZeRegistersTheDeclaredPlugins, the check this package serves

// Package registered judges that the plugins a built `ze` registers at run
// time are exactly the plugins the committed source declares.
//
// The declared list is the static walk repoinventory.DeclaredPlugins answers.
// The runtime list is the built binary's own `show plugin list`, read in its
// JSON rendering. The two are produced by unrelated code: the walk reads
// source, the binary runs every init(). A registration the walk cannot see
// (behind a call it treats as opaque) appears on one side only, and so does a
// declared registration the build never runs.
package registered

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
	"github.com/ze-software/ze/internal/test/runner"
)

// buildTimeout bounds the one compile of cmd/ze. A cold build cache compiles
// every gated package, which is the slow case.
const buildTimeout = 5 * time.Minute

// listTimeout bounds one `show plugin list`, which compiles nothing and reads
// a registry the process filled at init.
const listTimeout = time.Minute

// listCommand is the command the built binary answers. `ze cli -c` serves a
// local-data command in its own process before it looks for a daemon
// (client.Run, command.ServeLocal), so a daemon running on the host never
// answers in the built binary's place.
const listCommand = "show plugin list | json"

// Row is one row of `show plugin list` in its JSON rendering: the fields
// pluginRow writes (internal/component/plugin/register.go).
type Row struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	RFCs         []string `json:"rfcs"`
	Capabilities []int    `json:"capabilities"`
	Families     []string `json:"families"`
	Outcome      string   `json:"outcome"`
}

// BuildZe compiles cmd/ze from the checkout at root into dir, for Linux on
// this host's architecture with cgo off, carrying the daemon base and every
// gate feature-gates.txt declares, and answers the binary's path.
//
// The tags are the daemon's (repofeaturetags.DaemonBase), the build a
// customer runs. The caller owns dir and MUST remove it after the binary's
// last run.
func BuildZe(ctx context.Context, root, dir string) (string, error) {
	tags, err := repofeaturetags.DaemonBuildTags(root, repofeaturetags.DaemonBase)
	if err != nil {
		return "", err
	}
	binary := filepath.Join(dir, "ze")

	ctx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()
	// #nosec G204 -- the argv is fixed apart from the tags, which are read
	// from the checkout's own feature-gates.txt, and the output path is ours.
	command := osexec.CommandContext(ctx, "go", "build", "-tags", tags, "-o", binary, "./cmd/ze")
	command.Dir = root
	command.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0")
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := runOwned(ctx, command); err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("compile ./cmd/ze with tags ").Str(tags).Str(": ").
			Str(err.Error()).Str(": ").Str(strings.TrimSpace(output.String())).String())
	}
	return binary, nil
}

// ListPlugins runs the binary's `show plugin list` in its JSON rendering and
// answers the rows it printed, parsed by ParseRows.
func ListPlugins(ctx context.Context, binary string) ([]Row, error) {
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	// #nosec G204 -- the binary is one BuildZe compiled, and the argv is fixed.
	command := osexec.CommandContext(ctx, binary, "cli", "-c", listCommand)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := runOwned(ctx, command); err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("ze cli -c \"").Str(listCommand).Str("\": ").
			Str(err.Error()).Str(": ").Str(strings.TrimSpace(stderr.String())).String())
	}
	return ParseRows(stdout.Bytes())
}

// ParseRows parses the JSON rendering of `show plugin list`.
//
// It refuses what cannot be the binary's answer, so an empty or foreign
// payload never reaches the comparison as an empty runtime list: a payload
// that is not a JSON array of rows, an empty array, a row with no name or no
// outcome (pluginRow writes both on every row), and a name listed twice.
func ParseRows(data []byte) ([]Row, error) {
	var rows []Row
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, errors.New("show plugin list printed no JSON array of plugin rows: " + err.Error())
	}
	if len(rows) == 0 {
		return nil, errors.New("show plugin list printed an empty plugin list, so the binary registered nothing")
	}
	seen := make(map[string]bool, len(rows))
	for index := range rows {
		row := &rows[index]
		if row.Name == "" {
			return nil, errors.New("show plugin list printed row " + strconv.Itoa(index) + " with no name")
		}
		if row.Outcome == "" {
			return nil, errors.New("show plugin list printed plugin " + strconv.Quote(row.Name) + " with no outcome")
		}
		if seen[row.Name] {
			return nil, errors.New("show plugin list printed plugin " + strconv.Quote(row.Name) + " twice")
		}
		seen[row.Name] = true
	}
	return rows, nil
}

// Compare answers nil when the runtime rows name exactly the declared plugins
// with the same description, RFCs, capability codes and families, and an
// error naming every difference otherwise.
//
// A plugin on one side only is named with the side it is on. A plugin that
// recorded a setup outcome and never registered reaches the rows with a
// substituted description and no metadata, so it is reported as a
// description difference. Either list empty is refused: two empty lists are
// equal and prove nothing.
func Compare(declared []repoinventory.Plugin, rows []Row) error {
	if len(declared) == 0 {
		return errors.New("the declared plugin list is empty, so there is nothing to compare the binary with")
	}
	if len(rows) == 0 {
		return errors.New("the runtime plugin list is empty, so there is nothing to compare the source with")
	}

	byName := make(map[string]*repoinventory.Plugin, len(declared))
	for index := range declared {
		byName[declared[index].Name] = &declared[index]
	}
	var differences []string
	registered := make(map[string]bool, len(rows))
	for index := range rows {
		row := &rows[index]
		registered[row.Name] = true
		plugin, isDeclared := byName[row.Name]
		if !isDeclared {
			differences = append(differences, "plugin "+strconv.Quote(row.Name)+
				" is registered at run time and absent from the declared list")
			continue
		}
		differences = append(differences, fieldDifferences(plugin, row)...)
	}
	for index := range declared {
		if !registered[declared[index].Name] {
			differences = append(differences, "plugin "+strconv.Quote(declared[index].Name)+
				" is in the declared list and not registered at run time")
		}
	}
	if len(differences) == 0 {
		return nil
	}
	slices.Sort(differences)
	return errors.New("the built ze and the declared source disagree:\n  " + strings.Join(differences, "\n  "))
}

// fieldDifferences answers one line for each field of a plugin both sides
// name whose values differ.
func fieldDifferences(plugin *repoinventory.Plugin, row *Row) []string {
	prefix := "plugin " + strconv.Quote(row.Name) + " "
	var differences []string
	if plugin.Description != row.Description {
		differences = append(differences, prefix+"description: declared "+strconv.Quote(plugin.Description)+
			", registered "+strconv.Quote(row.Description))
	}
	if !slices.Equal(plugin.RFCs, row.RFCs) {
		differences = append(differences, prefix+"rfcs: declared "+list(plugin.RFCs)+", registered "+list(row.RFCs))
	}
	if !slices.Equal(plugin.Families, row.Families) {
		differences = append(differences, prefix+"families: declared "+list(plugin.Families)+
			", registered "+list(row.Families))
	}
	declaredCodes := make([]string, len(plugin.Capabilities))
	for index, code := range plugin.Capabilities {
		declaredCodes[index] = strconv.Itoa(int(code))
	}
	registeredCodes := make([]string, len(row.Capabilities))
	for index, code := range row.Capabilities {
		registeredCodes[index] = strconv.Itoa(code)
	}
	if !slices.Equal(declaredCodes, registeredCodes) {
		differences = append(differences, prefix+"capabilities: declared "+list(declaredCodes)+
			", registered "+list(registeredCodes))
	}
	return differences
}

// list renders values for a difference line, so an empty list reads as one
// rather than as nothing.
func list(values []string) string {
	return "[" + strings.Join(values, " ") + "]"
}

// runOwned runs command with its process group owned until its sole reaper
// joins, so a stopped compile leaves no compiler running. The deferred Close
// MUST finish before the caller removes the build directory.
func runOwned(ctx context.Context, command *osexec.Cmd) (err error) {
	command.WaitDelay = 2 * time.Second
	owner, err := runner.StartServerProcess(command)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, owner.Close()) }()
	return errors.Join(owner.Wait(), ctx.Err())
}
