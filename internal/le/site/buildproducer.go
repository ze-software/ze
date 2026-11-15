// Design: website/AI.md -- one producer re-rendered over a full artifact, for a page under review
// Related: build.go runs every producer; producer.go holds the registry and its record.
// Related: footer.go stamps a page and carries the stamp it was published with.
package site

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/derived"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// ProducerBuildOptions controls one run of named producers over an existing
// full artifact.
type ProducerBuildOptions struct {
	Repository string
	Output     string
	// Producers names the producers to run, as Producer.Name spells them. Each
	// one runs once, in the order a full build runs it, whatever order they were
	// named in.
	Producers []string
}

// ProducerBuildReport identifies what one producer run wrote.
//
// Routes names every route the producers answered, so a reader finds the page
// under review. Coverage is the artifact's coverage after the run, read from the
// record the run merged, so it is what `./le site check` answers next.
type ProducerBuildReport struct {
	Output    string   `json:"output"`
	Producers []string `json:"producers"`
	Routes    []string `json:"routes"`
	Published string   `json:"published"`
	Carried   int      `json:"carried-stamps"`
	Unstamped []string `json:"unstamped,omitempty"`
	Coverage  Coverage `json:"coverage"`
}

// BuildProducers re-renders the pages of the named producers over an existing
// full artifact, so a page under review is published in seconds rather than
// with the whole site.
//
// It keeps every step that decides the bytes of the pages it writes:
//   - the derived checkout artifacts, which a producer reads (derived.EnsureAll)
//   - both producer passes, in the order a full build runs them, so a producer
//     that reads the finished artifact (the search index, llms-full.txt, the
//     sitemap) reads the pages this run wrote
//   - the source-only trim, so `./le site check` judges the artifact as it
//     judges a full build's
//   - the publication stamp on each page the run wrote, with the stamp the page
//     was published with carried back when nothing else changed
//
// It skips the steps that rewrite the rest of the site:
//   - seeding and staging: staging lays the raw source of every page over the
//     page its producer wrote, and only the named producers run again to
//     replace theirs
//   - the terminal-demo retirement, which acts on the media of the whole site
//   - the native surfaces (refreshNativeSurfaces), which compile a daemon. The
//     producers read the command catalog, the configuration tree and the site
//     facts the last full build published. A change to the code those describe,
//     to a stylesheet, a script or a staged asset, needs the full build
//
// The producer record is merged rather than replaced: the claims of the
// producers that ran are replaced by what they wrote now, and every other
// producer's claims stand. A record of this run alone would read every other
// page as unclaimed, and `./le site check` would refuse an artifact that is
// whole.
func BuildProducers(options ProducerBuildOptions) (ProducerBuildReport, error) {
	selection, err := selectProducers(options.Producers)
	if err != nil {
		return ProducerBuildReport{}, err
	}
	paths, err := buildPaths(options.Repository, options.Output)
	if err != nil {
		return ProducerBuildReport{}, err
	}
	recorded, err := fullArtifactClaims(paths, options.Output)
	if err != nil {
		return ProducerBuildReport{}, err
	}
	if err := derived.EnsureAll(paths.Repository); err != nil {
		return ProducerBuildReport{}, err
	}

	// The pages as they were published, read before any producer writes, so a
	// page this run rewrote to the same bytes keeps its publication stamp.
	previous, err := claimedPages(paths.Output, recorded, selection.names)
	if err != nil {
		return ProducerBuildReport{}, err
	}
	byRoute := make(map[string][]string)
	if err := renderInto(byRoute, selection.pages, paths); err != nil {
		return ProducerBuildReport{}, err
	}
	if err := renderInto(byRoute, selection.finished, paths); err != nil {
		return ProducerBuildReport{}, err
	}
	claims := mergeClaims(recorded, selection.names, byRoute)
	if err := writeProducerRecord(paths, claims); err != nil {
		return ProducerBuildReport{}, err
	}
	if err := removeSourceOnly(paths.Output); err != nil {
		return ProducerBuildReport{}, err
	}
	if err := writeArtifactIgnore(paths.Output); err != nil {
		return ProducerBuildReport{}, err
	}

	pages, err := pageRegistry(paths.Output)
	if err != nil {
		return ProducerBuildReport{}, err
	}
	published := buildClock()
	stamps, err := stampWritten(paths.Output, pages, byRoute, previous, published)
	if err != nil {
		return ProducerBuildReport{}, err
	}
	routes := make([]string, 0, len(byRoute))
	for route := range byRoute {
		routes = append(routes, route)
	}
	slices.Sort(routes)
	return ProducerBuildReport{
		Output:    paths.Output,
		Producers: selection.names,
		Routes:    routes,
		Published: publishedDisplay(published),
		Carried:   stamps.carried,
		Unstamped: stamps.unstamped,
		Coverage:  coverageFrom(pages, claims),
	}, nil
}

// producerSelection is the registered producers one run names, split into the
// two passes a build runs, each in registration order.
type producerSelection struct {
	pages    []Producer
	finished []Producer
	names    []string
}

// selectProducers answers the registered producers a run names. A name the
// registry does not hold is refused, and the refusal lists every name it does
// hold, read from the registry.
func selectProducers(requested []string) (producerSelection, error) {
	if len(requested) == 0 {
		return producerSelection{}, errors.New("site build producer names no producer: it renders only the producers it is given; " +
			"run ./le site build producer <name>, with a name from ./le help site build producer")
	}
	for _, name := range requested {
		if !slices.Contains(producerNames(), name) {
			return producerSelection{}, unknownProducer(name)
		}
	}
	var selection producerSelection
	for _, producer := range registeredProducers {
		if slices.Contains(requested, producer.Name) {
			selection.pages = append(selection.pages, producer)
			selection.names = append(selection.names, producer.Name)
		}
	}
	for _, producer := range derivedProducers {
		if slices.Contains(requested, producer.Name) {
			selection.finished = append(selection.finished, producer)
			selection.names = append(selection.names, producer.Name)
		}
	}
	return selection, nil
}

// producerNames answers the name of every registered producer, in the order a
// build runs them.
func producerNames() []string {
	producers := allProducers()
	names := make([]string, 0, len(producers))
	for _, producer := range producers {
		names = append(names, producer.Name)
	}
	return names
}

// unknownProducer is the refusal of a name the producer registry does not hold.
func unknownProducer(name string) error {
	return fmt.Errorf("no site producer is named %q: producer selects a producer the site registry holds, "+
		"and it holds %s; run ./le site build producer <name> with one of those", name, strings.Join(producerNames(), ", "))
}

// producerCandidates answers every producer a run may name, for completion and
// for the keyword's help page, with the pass each one runs in. It reads the
// registry, so it offers exactly the names selectProducers accepts.
func producerCandidates() []leaction.Candidate {
	candidates := make([]leaction.Candidate, 0, len(registeredProducers)+len(derivedProducers))
	for _, producer := range registeredProducers {
		candidates = append(candidates, leaction.Candidate{Value: producer.Name, Summary: "writes its own pages"})
	}
	for _, producer := range derivedProducers {
		candidates = append(candidates, leaction.Candidate{Value: producer.Name, Summary: "reads the finished artifact, after the pages"})
	}
	return candidates
}

// fullArtifactClaims answers the claims recorded for the artifact a producer run
// renders over, and refuses an artifact no full build of this checkout wrote.
//
// The record is the proof: only a full build writes one, and every page this
// run leaves alone is a page that build wrote. A run over a directory without
// one would write a few pages into a tree that holds none of the data files
// they read, and the merged record would claim nothing else.
func fullArtifactClaims(paths Paths, output string) ([]Claim, error) {
	info, err := os.Stat(paths.Output)
	if os.IsNotExist(err) {
		return nil, notAFullArtifact(paths, output, "it does not exist")
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, notAFullArtifact(paths, output, "it is not a directory")
	}
	record := producerRecordPath(paths)
	_, err = os.Stat(record)
	if os.IsNotExist(err) {
		return nil, notAFullArtifact(paths, output, "this checkout holds no record of a full build into it ("+record+")")
	}
	if err != nil {
		return nil, err
	}
	return readProducerRecord(paths)
}

// notAFullArtifact is the refusal of an output a producer run cannot render
// over, naming the full build that makes one.
func notAFullArtifact(paths Paths, output, reason string) error {
	build := "./le site build"
	if output != "" {
		build += " output " + output
	}
	return fmt.Errorf("site build producer renders over an existing full artifact, and %s is not one: %s; run %s first, then name the producer again",
		paths.Output, reason, build)
}

// claimedPages answers the published bytes of every page the record says one of
// the named producers wrote, keyed by the page's artifact-relative HTML path. A
// page the artifact no longer holds answers nil, which carries no stamp.
func claimedPages(output string, recorded []Claim, names []string) (map[string][]byte, error) {
	pages := make(map[string][]byte)
	for _, claim := range recorded {
		if !slices.ContainsFunc(claim.Producers, func(name string) bool { return slices.Contains(names, name) }) {
			continue
		}
		page := routeHTML(claim.Route)
		content, err := publishedPage(filepath.Join(output, filepath.FromSlash(page)))
		if err != nil {
			return nil, err
		}
		pages[page] = content
	}
	return pages, nil
}

// routeHTML answers the artifact-relative HTML path of one route, as
// pageRegistry spells it: "/" is index.html and "/labs/" is labs/index.html.
func routeHTML(route string) string {
	return strings.TrimPrefix(route, "/") + pageIndexFile
}

// mergeClaims answers the record of an artifact after the named producers ran:
// every recorded claim of a producer that did not run, and every route the ones
// that ran answered now. A route only the producers that ran had claimed, and
// that they no longer write, leaves the record, so the check reads the page
// they left behind as unclaimed.
func mergeClaims(recorded []Claim, ran []string, byRoute map[string][]string) []Claim {
	merged := make(map[string][]string, len(recorded)+len(byRoute))
	for _, claim := range recorded {
		for _, name := range claim.Producers {
			if slices.Contains(ran, name) {
				continue
			}
			merged[claim.Route] = append(merged[claim.Route], name)
		}
	}
	for route, names := range byRoute {
		merged[route] = append(merged[route], names...)
	}
	order := make(map[string]int)
	for index, name := range producerNames() {
		order[name] = index
	}
	claims := make([]Claim, 0, len(merged))
	for route, names := range merged {
		// A full build lists each route's producers in registration order, so
		// the merged record does too. A name this binary does not register,
		// from a record an older build wrote, sorts last.
		slices.SortStableFunc(names, func(left, right string) int {
			return producerRank(order, left) - producerRank(order, right)
		})
		claims = append(claims, Claim{Route: route, Producers: names})
	}
	slices.SortFunc(claims, func(left, right Claim) int { return strings.Compare(left.Route, right.Route) })
	return claims
}

// producerRank answers where one producer runs in a build, and after every
// registered producer for a name the registry does not hold.
func producerRank(order map[string]int, name string) int {
	if rank, registered := order[name]; registered {
		return rank
	}
	return len(order)
}

// stampCount is what stamping the pages of one producer run did.
type stampCount struct {
	carried   int
	unstamped []string
}

// stampWritten writes the publication stamp into every published page the run
// wrote, as stampArtifact does for a full build, then gives a page whose bytes
// changed nowhere else the stamp it was published with, as
// carryPublicationStamps does. A page the artifact does not publish is left
// alone, because a full build stamps only the pages pageRegistry answers.
func stampWritten(output string, pages []Page, byRoute map[string][]string, previous map[string][]byte, published time.Time) (stampCount, error) {
	var count stampCount
	for _, page := range pages {
		if len(byRoute[page.Route]) == 0 {
			continue
		}
		name := filepath.Join(output, filepath.FromSlash(page.HTML))
		content, err := os.ReadFile(name) //nolint:gosec // a site build reads the artifact it just wrote
		if err != nil {
			return stampCount{}, err
		}
		patched, found := patchFooter(content, pageRoot(page.HTML), published)
		if !found {
			count.unstamped = append(count.unstamped, page.HTML)
			continue
		}
		if restored, carried := restoreStamp(previous[page.HTML], patched); carried {
			patched = restored
			count.carried++
		}
		if bytes.Equal(patched, content) {
			continue
		}
		if err := os.WriteFile(name, patched, 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
			return stampCount{}, fmt.Errorf("stamp %s: %w", name, err)
		}
	}
	return count, nil
}
