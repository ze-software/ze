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
	// handlers answer in process, but every answer reads the estate whole:
	// the first full tour asked about 9,000 requests in about seven minutes
	// (some 47 ms each), and the narrowed tour's estimated 3,000 take about
	// two and a half. The bound stops a handler that never answers, with
	// room for a busy machine.
	renderWait = 10 * time.Minute
)

const renderWhy = "write the fleet interface's recorded demonstration, for the owner to review.\n" +
	"\n" +
	"It serves the fleet pages in process over the sample estate, at a clock pinned\n" +
	"to the commit date of HEAD, and records a tour of them: every registered tab of\n" +
	"the estate, of each group, of each device and of the administration, the views\n" +
	"of the devices table, the trees, the charts, the side panels and the console's\n" +
	"suggested commands, with every style and script the pages load. It writes the\n" +
	"host page, the frame, the recording's index and one chunk file per object under\n" +
	"" + demoDirectory + "/ of the website output, and reports each chunk's answers and size,\n" +
	"in the whole and in each chunk by kind of request (page, side panel, fragment, chart, console).\n" +
	"A tour past its bounds fails, naming the bound and what the chunk holds by kind, and writes nothing.\n" +
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
	// scope is how much of the estate the tour opens. The action leaves it
	// the whole tour; only a test narrows it.
	scope tourScope
}

// tourScope is how much of the estate a tour opens. The zero scope is the
// whole tour, which every render of the action records.
type tourScope struct {
	// sampled keeps, beside the estate, the task list and the
	// administration, the first group node of the default tree view with the
	// nodes below it, the first featured device and the first ordinary one
	// by name, and none of the devices table's views but its default one. A
	// test renders it twice to compare the bytes over every kind of object
	// and device the whole tour records, at a fraction of its requests.
	sampled bool
}

// writtenFile is one file a render wrote, relative to the website output.
type writtenFile struct {
	path string
	size int
}

// writtenChunk is one chunk file a render wrote: the answers of one object.
type writtenChunk struct {
	name      string
	exchanges int
	size      int
	kinds     []kindSize
}

// kindSize is what the recorded answers of one kind of request hold: a page
// as htmx or the browser asks it, a side panel, a fragment by its name, a
// chart's data, a console command (exchangeKind).
type kindSize struct {
	kind      string
	exchanges int
	// bytes counts the answers' bodies, which are most of a chunk file.
	bytes int
}

// rendered is what one render wrote. exchanges counts every recorded answer,
// the frame's and every chunk's, and kinds holds them by kind of request,
// the largest first.
type rendered struct {
	files     []writtenFile
	chunks    []writtenChunk
	kinds     []kindSize
	exchanges int
	assets    int
}

// kindReport answers sizes as the report prints them.
func kindReport(sizes []kindSize) []map[string]any {
	report := make([]map[string]any, 0, len(sizes))
	for _, size := range sizes {
		report = append(report, map[string]any{"kind": size.kind, "exchanges": size.exchanges, "bytes": size.bytes})
	}
	return report
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

	chunks := make([]map[string]any, 0, len(written.chunks))
	for _, chunk := range written.chunks {
		chunks = append(chunks, map[string]any{
			"chunk": chunk.name, "exchanges": chunk.exchanges, "bytes": chunk.size, "kinds": kindReport(chunk.kinds),
		})
	}

	return map[string]any{
		"output":    paths.Output,
		"files":     files,
		"chunks":    chunks,
		"kinds":     kindReport(written.kinds),
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
