// Design: website/AI.md -- one native command surface owns site build tools
package site

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/feature"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	reporewrite "github.com/ze-software/ze/internal/le/repo/rewrite"
)

const area = "site"

// valueDirectory is the value name every action that takes a path spells. Three
// of them do, so it is named once.
const valueDirectory = "directory"

// keywordRefresh asks `site build` to re-record every supported feature
// published on a stale run before it builds (freshness.go).
const keywordRefresh = "refresh"

// keywordPartial asks `site build` to restage sources over the existing full
// artifact rather than seed it from the last published one.
const keywordPartial = "partial"

// keywordProducer asks `site build` to render only the named producers over the
// existing full artifact (buildproducer.go).
const keywordProducer = "producer"

// doesRender is the vocabulary verb the rendering names of this area perform.
const doesRender = "render"

// buildWhy is the page `./le help site build` prints. It carries the review
// loop, because the reader who needs it is editing a page and asks the binary
// how to see it, and the le that answers was built from committed HEAD.
const buildWhy = "stage website sources and refresh the Pages artifact with native renderers.\n" +
	"\n" +
	"To see one page while you edit it, render only the producer that writes it:\n" +
	"  ./le --name session site build producer <name>\n" +
	"It re-renders that producer's pages over the existing full artifact in seconds,\n" +
	"with no commit and no whole-site rebuild. ./le runs an le built from committed\n" +
	"HEAD, which never sees an uncommitted edit to a producer; --name session builds\n" +
	"this session's le from the working tree. Page sources under website/ and docs/\n" +
	"are read from the working tree either way.\n" +
	"The producer names, and the pass each runs in: ./le help site build producer.\n" +
	"The producer of a published route is named in its claim in\n" +
	"tmp/site/producers-<key>.json, which every build writes.\n" +
	"A change to a stylesheet, a script, a copied asset or the code the command\n" +
	"catalog and configuration tree describe needs the full build."

var actions = leaction.New(area,
	leaction.Action{Verb: "build", Heavy: true, Summary: "stage the website and refresh the Pages artifact", Why: buildWhy, Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keywordOutput, Kind: leaction.KindPath, Help: "the directory the Pages artifact is written to", Value: valueDirectory, Requirement: leaction.Optional},
			{Keyword: keywordPartial, Kind: leaction.KindSwitch, Help: "restage sources over the existing full artifact"},
			{Keyword: keywordRefresh, Kind: leaction.KindSwitch, Help: "refresh stale feature evidence before the build"},
			{Keyword: keywordProducer, Kind: leaction.KindLabel, Help: "render only this producer's pages over the artifact", Value: "name", Requirement: leaction.Optional, Repeat: true, Source: producerCandidates},
		}, AnswerArgs: runBuild},
	leaction.Action{Verb: "check", Summary: "check the Pages artifact holds no source inputs", Why: "verify that the Pages artifact contains no source-only website inputs, and that the looking glass replay it carries is not a trial recording and names a commit in this checkout's history",
		Parameters: []leaction.Parameter{
			{Keyword: keywordOutput, Kind: leaction.KindPath, Help: "the Pages artifact directory to check", Value: valueDirectory, Requirement: leaction.Optional},
			{Keyword: publicationReceiptKeyword, Kind: leaction.KindSwitch, Help: "verify the saved source and reviewed artifact bytes"},
			{Keyword: publicationDeploymentKeyword, Kind: leaction.KindRevision, Help: "confirm the artifact commit and live website bytes", Value: "commit", Requirement: leaction.Optional},
		}, AnswerArgs: runCheck},
	leaction.Action{Verb: "record", Heavy: true, Writes: true, Summary: "record the checked website publication bytes", Why: "record the full build's observed source and the reviewed artifact bytes outside the served site; this neither approves nor publishes them",
		Parameters: []leaction.Parameter{
			{Keyword: publicationSourceKeyword, Kind: leaction.KindRevision, Help: "the full source commit ID observed by the full build", Value: "commit", Requirement: leaction.Required},
			{Keyword: keywordOutput, Kind: leaction.KindPath, Help: "the checked Pages artifact directory", Value: valueDirectory, Requirement: leaction.Optional},
		}, AnswerArgs: runPublicationRecord},
	leaction.Action{Verb: "bundle", Does: doesRender, Summary: "turn one deck into a self-contained HTML file", Why: "turn one presentation deck into a self-contained HTML file", Writes: true,
		Parameters: []leaction.Parameter{{Keyword: keywordInput, Kind: leaction.KindPath, Help: "the presentation deck to bundle", Value: "html-file", Requirement: leaction.Required}}, AnswerArgs: runBundle},
	leaction.Action{Verb: "activity", Does: doesRender, Summary: "render repository activity as an HTML page", Why: "render repository line and commit activity as a presentation-ready HTML page", Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keywordOutput, Kind: leaction.KindPath, Help: "the HTML file the page is written to", Value: "html-file", Requirement: leaction.Optional},
			{Keyword: "days", Kind: leaction.KindUint, Help: "how many days of activity to show", Value: "count", Requirement: leaction.Optional},
			{Keyword: "ref", Kind: leaction.KindRevision, Help: "the revision the activity is read at", Value: "revision", Requirement: leaction.Optional},
			{Keyword: "today", Kind: leaction.KindString, Help: "the date the page is drawn for, as YYYY-MM-DD", Value: "date", Requirement: leaction.Optional},
		}, AnswerArgs: runActivity},
	leaction.Action{Verb: "talk", Heavy: true, Does: "update", Summary: "refresh one talk's statistics and deck", Why: "refresh one talk's live statistics, activity page, and standalone deck", Writes: true,
		Parameters: []leaction.Parameter{{Keyword: "talk", Kind: leaction.KindLabel, Help: "the talk to refresh, by its slug", Value: "slug", Requirement: leaction.Required}, {Keyword: "bundle-only", Kind: leaction.KindSwitch, Help: "rebuild only the standalone deck"}}, AnswerArgs: runUpdateTalk},
	leaction.Action{Verb: "config-tree", Heavy: true, Does: doesRender, Summary: "extract the live YANG config tree for the site", Why: "extract the live YANG configuration tree for the public configuration reference", Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: keywordOutput, Kind: leaction.KindPath, Help: "the directory the tree is written to", Value: valueDirectory, Requirement: leaction.Optional},
			{Keyword: "binary", Kind: leaction.KindPath, Help: "the ze binary the tree is extracted from", Value: "ze-binary", Requirement: leaction.Optional},
		}, AnswerArgs: runConfigTree},
)

func Actions() leaction.List          { return actions.Actions() }
func Subs() string                    { return actions.Subs() }
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runBuild(arguments leaction.Arguments) (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	restore := context.AfterFunc(ctx, stop)
	defer restore()
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if arguments.Has(keywordProducer) {
		return runProducerBuild(root, arguments)
	}
	// The warning, and a refresh when asked, come before the build publishes.
	stale, refreshed, err := freshness(os.Stderr, arguments.Has(keywordRefresh),
		func() ([]feature.StaleFeature, error) { return feature.StaleSupported(root) },
		func(due []feature.StaleFeature) []feature.StaleFeature { return feature.RefreshStale(root, due) })
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := Build(ctx, BuildOptions{Repository: root, Output: arguments.One(keywordOutput), Partial: arguments.Has(keywordPartial)})
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report.StaleSupported = stale
	report.Refreshed = refreshed
	return report, 0
}

// runProducerBuild renders the named producers over the existing artifact.
//
// It refuses the two keywords that ask for a whole-site step it does not run:
// partial restages every source, and refresh re-records the feature evidence a
// full build publishes. Each is a usage mistake, so it answers 2, the code a
// parse refusal answers. It reports through leaction.ReportError rather than
// Area.Refuse, because the action table names this handler and a handler that
// names the table back is an initialization cycle.
func runProducerBuild(root string, arguments leaction.Arguments) (any, int) {
	if arguments.Has(keywordPartial) {
		leaction.ReportError(opererr.New("producer and partial cannot be combined",
			"A producer run keeps the existing artifact. A partial build restages every source; omit producer for that build.",
			"./le site build partial"))
		return nil, 2
	}
	if arguments.Has(keywordRefresh) {
		leaction.ReportError(opererr.New("producer and refresh cannot be combined",
			"A refresh records feature evidence that only a full build publishes; omit producer for that build.",
			"./le site build refresh"))
		return nil, 2
	}
	report, err := buildProducers(producerBuildOptions{
		Repository: root, Output: arguments.One(keywordOutput), Producers: arguments.Values(keywordProducer),
	})
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}

type checkReport struct {
	Output string `json:"output"`
	// MissingArtifacts names the published files that are not routes and are
	// absent. The coverage arithmetic answers for pages; nothing else answers
	// for these.
	MissingArtifacts []string `json:"missing-artifacts,omitempty"`
	SourceOnly       []string `json:"source-only,omitempty"`
	MissingMirrors   []string `json:"missing-mirrors,omitempty"`
	// PlainFigures names each snapshot value a page shows outside a
	// data-ze-stat, data-ze-rfc or data-ze-history marker. Such a value is a second copy of
	// data/site-facts.json that site.js cannot update, so it goes stale.
	PlainFigures []plainFigure `json:"plain-figures,omitempty"`
	// StaleRatchet names each page website/data/plain-figure-ratchet.json
	// lists that no longer shows a plain figure, so its entry is owed removal.
	StaleRatchet []string `json:"stale-ratchet,omitempty"`
	// Replay names each reason the looking glass replay the artifact carries
	// is not for publication (checkLGReplay): a trial recording, or a
	// recording whose commit is not in the checkout's history.
	Replay      []replayRefusal    `json:"replay,omitempty"`
	Coverage    Coverage           `json:"coverage"`
	Publication *publicationReport `json:"publication,omitempty"`
}

// exit answers the status one check reports. An artifact is refused when it
// carries a source-only input, when a public route has no Markdown mirror, when
// a named non-route artifact is absent, when a page shows a published value
// outside its marker, when the plain-figure ratchet lists a page that shows
// none, when its looking glass replay is not for publication, and when a
// published route has no producer or has two.
func (report checkReport) exit() int {
	if len(report.Replay) != 0 {
		return 1
	}
	if len(report.SourceOnly) != 0 || len(report.MissingMirrors) != 0 {
		return 1
	}
	if len(report.PlainFigures) != 0 {
		return 1
	}
	if len(report.StaleRatchet) != 0 {
		return 1
	}
	if len(report.MissingArtifacts) != 0 || report.Coverage.Red() {
		return 1
	}
	return 0
}

// runCheck judges one built artifact.
//
// It takes the SAME `output` keyword the build takes, and defaults to the same
// published tree when the keyword is absent. A check that could only judge the
// default artifact could not answer for the artifact a build was pointed at,
// which is the one a session verifying its own work has just produced.
func runCheck(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	paths, err := ResolvePaths(root, arguments.One(keywordOutput))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := checkSiteArtifact(paths)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	reportReplayRefusals(report.Replay)
	if code := report.exit(); code != 0 {
		return report, code
	}
	publicationRequested := arguments.Has(publicationReceiptKeyword)
	if arguments.Has(publicationDeploymentKeyword) {
		publicationRequested = true
	}
	if publicationRequested {
		receipt, err := checkPublication(context.Background(), paths)
		if err != nil {
			return publicationFailure(err)
		}
		publication := publicationAnswer(paths, receipt, "checked")
		if arguments.Has(publicationDeploymentKeyword) {
			publication, err = confirmPublication(context.Background(), paths, receipt, arguments.One(publicationDeploymentKeyword))
			if err != nil {
				return publicationFailureWithFix(err, "./le site check deployment "+arguments.One(publicationDeploymentKeyword))
			}
		}
		report.Publication = &publication
	}
	return report, report.exit()
}

// checkSiteArtifact keeps one check population for normal checks and receipt
// creation. A failed check never writes a publication receipt.
func checkSiteArtifact(paths Paths) (checkReport, error) {
	report := checkReport{Output: paths.Output}
	var err error
	report.SourceOnly, err = publishedSourceOnly(paths.Output)
	if err != nil {
		return report, err
	}
	report.MissingArtifacts = checkNamedArtifacts(paths.Output)
	report.MissingMirrors, err = checkPageMirrors(paths.Output)
	if err != nil {
		return report, err
	}
	plain, err := checkPlainFigures(paths.Source, paths.Output)
	if err != nil {
		return report, err
	}
	report.PlainFigures = plain.Figures
	report.StaleRatchet = plain.StaleRatchet
	report.Replay, err = checkLGReplay(context.Background(), paths)
	if err != nil {
		return report, err
	}
	report.Coverage, err = checkCoverage(paths)
	if err != nil {
		return report, err
	}
	return report, nil
}

func runBundle(arguments leaction.Arguments) (any, int) {
	output, err := bundlePresentation(arguments.One("input"))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return map[string]string{keywordOutput: output}, 0
}

func runActivity(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	days := reporewrite.ActivityDaysDefault
	if raw := arguments.One("days"); raw != "" {
		days, err = strconv.Atoi(raw)
		if err != nil {
			leaction.ReportError(fmt.Errorf("invalid days %q: %w", raw, err))
			return nil, 1
		}
	}
	today := time.Now().UTC()
	if raw := arguments.One("today"); raw != "" {
		today, err = time.Parse("2006-01-02", raw)
		if err != nil {
			leaction.ReportError(fmt.Errorf("invalid today %q: %w", raw, err))
			return nil, 1
		}
	}
	output := arguments.One(keywordOutput)
	if output == "" {
		output = filepath.Join(root, "tmp", "code-activity.html")
	}
	err = renderActivity(ActivityOptions{Repository: root, Ref: arguments.One("ref"), Output: output, Days: days, Today: today})
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return map[string]string{keywordOutput: output}, 0
}

func runUpdateTalk(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	slug := arguments.One("talk")
	if slug == "" || slug == "." || slug == ".." || filepath.Base(slug) != slug {
		leaction.ReportError(fmt.Errorf("talk must name one directory under website/talks"))
		return nil, 1
	}
	report, err := updateTalk(talkUpdateOptions{
		Repository: root,
		Directory:  filepath.Join(root, "website", "talks", slug),
		BundleOnly: arguments.Has("bundle-only"),
	})
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}

func runConfigTree(arguments leaction.Arguments) (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	restore := context.AfterFunc(ctx, stop)
	defer restore()
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "Ze cannot find the checkout for the configuration reference",
			"This command reads the checkout's YANG models. Run it from the Ze checkout.",
			"cd <ze-checkout> && ./le site config-tree"))
		return nil, 1
	}
	paths, err := ResolvePaths(root, arguments.One(keywordOutput))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	count, err := configTreeFrom(ctx, root, paths.Output, arguments.One("binary"))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return map[string]any{
		"file":  filepath.Join(paths.Output, "data", "yang-config-tree.json"),
		"roots": count,
	}, 0
}
