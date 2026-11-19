// Design: docs/contributing/gh-pages.md -- the fleet interface demonstration: its le action
// Related: register.go -- the registration of the area
// Related: render.go -- render, the recording this action writes
// Related: render_absent.go -- the answer of a le built without the fleet interface

package sitefleetdemo

import (
	"context"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/site"
	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

const (
	area = "site fleet-demo"

	keywordOutput = "output"

	// RenderCommand is the action's command line, which its refusals name.
	RenderCommand = "./le site fleet-demo render"

	// demoDirectory is where the demonstration is written, relative to the
	// website output. The output's tmp/ is unpublished: git ignores it, and
	// the page registry, the sitemap, the search index and the publication
	// receipt skip it, while a site build carries it across.
	demoDirectory = "tmp/fleet-demo"

	// renderWait bounds one render. The sample estate is in memory and the
	// handlers answer in process, so a render takes about a second; the bound
	// stops a handler that never answers.
	renderWait = 2 * time.Minute
)

const renderWhy = "write the fleet interface's recorded demonstration, for the owner to review.\n" +
	"\n" +
	"It serves the fleet pages in process over the sample estate, at a clock pinned\n" +
	"to the commit date of HEAD, asks for the estate dashboard, keeps the answer and\n" +
	"every style and script the page loads, and writes the host page, the frame and\n" +
	"the recording under " + demoDirectory + "/ of the website output.\n" +
	"Nothing is published: tmp/ is outside every walk of the site, no page links the\n" +
	"demonstration, and its pages carry a no-index marker. Open\n" +
	"" + demoDirectory + "/index.html from the review server of the output.\n" +
	"./le site build neither creates nor refreshes it. Two renders of one commit\n" +
	"write the same bytes."

var actions = leaction.New(area,
	leaction.Action{
		Verb: "render", Writes: true,
		Summary: "record the fleet interface for website review",
		Why:     renderWhy,
		Parameters: []leaction.Parameter{
			{Keyword: keywordOutput, Kind: leaction.KindPath, Help: "website output; defaults to the sibling gh-pages checkout", Value: "directory", Requirement: leaction.Optional},
		},
		AnswerArgs: runRender,
	},
)

// Actions answers every action with its exact writes metadata.
func Actions() leaction.List { return actions.Actions() }

// Subs answers the action names for the area's usage line.
func Subs() string { return actions.Subs() }

// Answer runs one action of the area.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// demonstration is what one render needs: where to read the player from,
// where to write, and the commit the recording names, whose date is the
// sample estate's clock.
type demonstration struct {
	// repository is the checkout the player's source is read from.
	repository string
	// output is the website output; the demonstration goes to its
	// demoDirectory and nowhere else.
	output     string
	provenance sitelgreplay.Provenance
	// clock is the sample estate's clock, pinned to the commit date.
	clock func() time.Time
}

// writtenFile is one file a render wrote, relative to the website output.
type writtenFile struct {
	path string
	size int
}

// rendered is what one render wrote.
type rendered struct {
	files     []writtenFile
	exchanges int
	assets    int
}

func runRender(arguments leaction.Arguments) (any, int) {
	ctx, cancel := context.WithTimeout(context.Background(), renderWait)
	defer cancel()

	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "Ze cannot find the checkout to record the fleet interface from",
			"The demonstration is recorded from the checkout's own fleet pages. Run the command from the Ze checkout.",
			"cd <ze-checkout> && "+RenderCommand))
		return nil, 1
	}
	paths, err := site.ResolvePaths(root, arguments.One(keywordOutput))
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "The website output to write the demonstration into cannot be used",
			"The demonstration is written under the output's "+demoDirectory+"/, and the output must be a website output outside the website sources.",
			RenderCommand+" "+keywordOutput+" <website-output>"))
		return nil, 2
	}
	demo, err := newDemonstration(ctx, paths)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	written, err := render(ctx, demo)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	files := make([]map[string]any, 0, len(written.files))
	total := 0
	for _, file := range written.files {
		files = append(files, map[string]any{"file": file.path, "bytes": file.size})
		total += file.size
	}

	return map[string]any{
		"output":    paths.Output,
		"files":     files,
		"bytes":     total,
		"exchanges": written.exchanges,
		"assets":    written.assets,
		"commit":    demo.provenance.Commit,
		"trial":     demo.provenance.TreeDirty,
		"published": false,
	}, 0
}

// newDemonstration reads the commit the recording names, and pins the sample
// estate's clock to its date, so two renders of one commit write the same
// bytes.
func newDemonstration(ctx context.Context, paths site.Paths) (demonstration, error) {
	// Nothing the render writes counts as uncommitted here: the output is
	// the gh-pages checkout by default, and a render's directory is a tmp/.
	head, err := sitelgreplay.ReadCheckout(ctx, paths.Repository)
	if err != nil {
		return demonstration{}, err
	}
	pinned, err := time.Parse(time.RFC3339, head.CommitDate)
	if err != nil {
		return demonstration{}, opererr.Wrap(err, "git gave the commit date "+head.CommitDate+", which is not a date",
			"The sample estate's clock is pinned to the commit date, so two renders of one commit write the same bytes.",
			"git -C "+paths.Repository+" show --no-patch --format=%cI HEAD")
	}

	return demonstration{
		repository: paths.Repository,
		output:     paths.Output,
		provenance: sitelgreplay.Provenance{
			Recorder:   RenderCommand,
			Commit:     head.Commit,
			CommitDate: head.CommitDate,
			TreeDirty:  len(head.Uncommitted) > 0,
			Normalized: []sitelgreplay.Normalization{},
			Varies:     []string{},
		},
		clock: func() time.Time { return pinned },
	}, nil
}
