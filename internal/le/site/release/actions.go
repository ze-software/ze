// Design: docs/architecture/site-facts.md -- release data is a network fact, committed by a command
//
// actions.go is the command surface: `le site release update` asks the forge
// for its published releases and writes website/data/github-releases.json, the
// file the site build reads instead of reaching the network.
//
// Related: release.go -- the file's model, the reach and its checks.

package siterelease

import (
	"fmt"
	"os"
	"path/filepath"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as.
const area = "site release"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "update", Summary: "write the published release data", Why: "read the releases " + Endpoint + " states and write " + Path + ", the file the site build reads instead of reaching the network",
		Writes: true,
		Answer: runUpdate},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le site release` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// written is what the update answers: the file it wrote, the endpoint it read,
// how many releases that endpoint states, and the newest one's tag ("" when
// there is none, which Releases being zero says first).
type written struct {
	Endpoint string `json:"endpoint"`
	File     string `json:"file"`
	Latest   string `json:"latest"`
	Releases int    `json:"releases"`
}

// runUpdate is `le site release update`.
func runUpdate() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return update(root)
}

// update is runUpdate with the checkout named, which is what a test can call
// against a fixture rather than against the checkout it runs in.
//
// The file is written only once every page was read and every release passed
// its checks, so a failure leaves the committed file as it was.
func update(root string) (any, int) {
	file, err := fetch()
	if err != nil {
		leaction.ReportError(fmt.Errorf("could not read the published releases from %s: %w\n"+
			"%s is unchanged, so the site keeps the release data it last committed.\n"+
			"When the forge answers again, retry:\n  ./le site release update", Endpoint, err, Path))
		return nil, 1
	}
	content, err := render(&file)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	path := filepath.Join(root, filepath.FromSlash(Path))
	if err := os.WriteFile(path, content, 0o600); err != nil {
		leaction.ReportError(fmt.Errorf("write %s: %w", path, err))
		return nil, 1
	}

	answer := written{Endpoint: Endpoint, File: path, Releases: len(file.Releases)}
	if latest, published := file.Latest(); published {
		answer.Latest = latest.TagName
	}
	return answer, 0
}
