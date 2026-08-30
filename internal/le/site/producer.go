// Design: website/AI.md -- every published route is written by one named producer
// Detail: build.go runs the registry; actions.go reports the routes it left out.
package site

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Producer renders one family of published pages into the artifact.
//
// A producer ANSWERS the routes it wrote. It does not declare the routes it
// owns, because a declaration is a second statement of one fact and the two
// drift apart: a producer that stops writing a page keeps declaring it. An
// answer derived from the writing cannot drift from the writing.
type Producer struct {
	// Name identifies this producer in a coverage answer, in lower kebab-case.
	Name string
	// Render writes this producer's pages under paths.Output and answers the
	// route of every page it wrote, spelled as pageRegistry spells one: a
	// leading slash, a trailing slash, and "/" for the site root. A file that
	// is not a public route, a redirect stub or a feed for example, is written
	// and not answered.
	Render func(Paths) ([]string, error)
}

// registeredProducers holds every page producer in registration order, which is
// the order Go runs this package's init functions. A build renders in that
// order, so one checkout renders the same site every time.
var registeredProducers []Producer

// derivedProducers holds the producers that read the FINISHED artifact, and
// every one of them runs after every page producer.
//
// Four producers need that: the search index and llms-full.txt read the Markdown
// mirror of every published page, the sitemap walks every published page, and
// the redirect stubs replace the index.html of a retired route and remove the
// mirror beside it. Init order is file-name order, which no producer can state
// its dependency in, so the two lists say which pass a producer belongs to.
var derivedProducers []Producer

// registerProducer adds one page producer to the registry, from the init() of
// the file that owns it.
//
// A bad registration is a programmer error rather than an operating one: it
// would publish a site with a silent hole, which is the failure this registry
// exists to make impossible. So it panics at init instead.
func registerProducer(producer Producer) {
	checkProducer(producer)
	registeredProducers = append(registeredProducers, producer)
}

// registerDerivedProducer adds one producer that reads the finished artifact.
func registerDerivedProducer(producer Producer) {
	checkProducer(producer)
	derivedProducers = append(derivedProducers, producer)
}

// checkProducer refuses a registration no build could run.
func checkProducer(producer Producer) {
	if producer.Name == "" {
		panic("BUG: site.registerProducer: a producer needs a name; see the init frame above")
	}
	if producer.Render == nil {
		panic("BUG: site.registerProducer: producer " + producer.Name + " has no Render")
	}
	for _, existing := range allProducers() {
		if existing.Name == producer.Name {
			panic("BUG: site.registerProducer: two producers are named " + producer.Name)
		}
	}
}

// allProducers answers every registered producer in the order a build runs
// them: the page producers first, then the producers that read what they wrote.
func allProducers() []Producer {
	producers := make([]Producer, 0, len(registeredProducers)+len(derivedProducers))
	producers = append(producers, registeredProducers...)
	producers = append(producers, derivedProducers...)
	return producers
}

// writeNamedArtifact writes one published file that is not a route: a feed, a
// data file, a machine-reader answer.
//
// A producer answers routes, and none of these is one, so the coverage
// arithmetic cannot see them. They are named in the check instead, which is
// what AC-16 of plan/spec-site-renderers-in-go.md arms.
func writeNamedArtifact(output, name, content string) error {
	path := filepath.Join(output, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// namedArtifacts are the published files that are not routes.
//
// A producer answers routes, so the coverage arithmetic cannot see any of
// these: each one could stop being written and every check the artifact carries
// would still pass. llms.txt is why this list exists. It lost seventeen of its
// eighteen sections when a second writer took the path, and the site published
// the shorter file for a day with nothing to say so.
//
// The list is the whole population, not a sample. A file added here that no
// build writes turns the check red, which is the correct answer: either the
// producer is missing or the name is wrong.
var namedArtifacts = []string{
	"assets/header.html",
	blogFeedDest,
	catalogFile,
	changesFeedDest,
	changesIndexFile,
	changesLegacyFeedDest,
	configTreeFile,
	factsFile,
	llmsFile,
	llmsFullFile,
	pluginFile,
	rfcComplianceSnapshot,
	robotsFile,
	searchIndexFile,
	sitemapFile,
}

// checkNamedArtifacts answers every named artifact the artifact does not carry
// as a file with content, in the order the list states them.
//
// An empty file counts as absent. A producer that wrote nothing and a producer
// that never ran leave a reader the same blank page, so the check answers the
// same way for both.
func checkNamedArtifacts(output string) []string {
	var missing []string
	for _, name := range namedArtifacts {
		info, err := os.Stat(filepath.Join(output, filepath.FromSlash(name)))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			missing = append(missing, name)
		}
	}
	return missing
}

// Claim names one route and every producer that wrote it.
type Claim struct {
	Route     string   `json:"route"`
	Producers []string `json:"producers"`
}

// Coverage answers how the routes an artifact publishes and the routes its
// producers wrote disagree.
//
// Unclaimed is a published route no producer wrote. It survives from the seed
// the previous build laid down, so its content is frozen while its mtime is
// fresh, and every other check the artifact carries passes it.
//
// Doubled is a route two producers wrote. The second write won and nothing said
// so, so which content a reader sees depends on registration order alone. It is
// as red as an unclaimed route.
//
// Producers counts the registry this binary carries, and Written counts the
// routes the build that made the artifact claimed. A check reads the two from
// different places, so they answer different questions.
type Coverage struct {
	Producers int      `json:"producers"`
	Published int      `json:"published"`
	Written   int      `json:"written"`
	Unclaimed []string `json:"unclaimed,omitempty"`
	Doubled   []Claim  `json:"doubly-claimed,omitempty"`
	// Spec names the work that owns this red while that work is open, so a
	// session sharing this checkout does not diagnose the red as its own
	// breakage. It is empty when the coverage is green.
	Spec string `json:"spec,omitempty"`
}

// coverageSpec is the open work that makes this coverage green. Its phase 1
// arms the check and its phase 10 empties the unclaimed list, so the red stands
// for the duration and states the defect rather than a new breakage.
const coverageSpec = "plan/spec-site-renderers-in-go.md"

// Red reports whether this coverage refuses the artifact.
func (coverage Coverage) Red() bool {
	return len(coverage.Unclaimed) != 0 || len(coverage.Doubled) != 0
}

// renderProducers runs every registered producer against one artifact and
// answers what they wrote, one entry for each route.
//
// The three passes are ordered here, because the order is a property of the
// build rather than of any one producer. The page producers write the pages and
// their Markdown mirrors. The legacy-URL rewrite then replaces every retired
// absolute address those pages carry. Only then do the producers that READ the
// finished artifact run, so the search index, llms-full.txt and the sitemap
// carry the addresses a reader reaches rather than the ones that moved.
//
// A producer that fails stops the build. A site published with one family of
// pages missing is the failure this registry exists to expose, and a warning on
// the way past does not expose it.
func renderProducers(paths Paths) ([]Claim, error) {
	byRoute := make(map[string][]string)
	if err := renderInto(byRoute, registeredProducers, paths); err != nil {
		return nil, err
	}
	// The legacy-URL rewrite ran here until 2026-08-30 and does not run now.
	// Owner decision: the site maps nothing to an old page before the first
	// release, so neither the redirect stubs nor this rewrite of addresses
	// inside published pages happens. `rewriteArtifactLegacyURLs` in
	// redirect.go is what returns, together with the producer that file
	// registers, when redirects are reconsidered after release.
	if err := renderInto(byRoute, derivedProducers, paths); err != nil {
		return nil, err
	}
	claims := make([]Claim, 0, len(byRoute))
	for route, names := range byRoute {
		claims = append(claims, Claim{Route: route, Producers: names})
	}
	// The map above answers in a random order and a route is a unique key, so
	// one sort over the routes makes the whole answer deterministic. Every
	// producer list is already in registration order.
	sort.Slice(claims, func(left, right int) bool { return claims[left].Route < claims[right].Route })
	return claims, nil
}

// renderInto runs one pass of producers and records the route each one wrote.
func renderInto(byRoute map[string][]string, producers []Producer, paths Paths) error {
	for _, producer := range producers {
		routes, err := producer.Render(paths)
		if err != nil {
			return fmt.Errorf("site producer %s: %w", producer.Name, err)
		}
		for _, route := range routes {
			byRoute[route] = append(byRoute[route], producer.Name)
		}
	}
	return nil
}

// coverageOf compares the routes one artifact publishes against the claims its
// producers made.
func coverageOf(output string, claims []Claim) (Coverage, error) {
	pages, err := pageRegistry(output)
	if err != nil {
		return Coverage{}, err
	}
	coverage := Coverage{Producers: len(allProducers()), Published: len(pages), Written: len(claims)}
	writers := make(map[string]int, len(claims))
	for _, claim := range claims {
		writers[claim.Route] = len(claim.Producers)
		if len(claim.Producers) > 1 {
			coverage.Doubled = append(coverage.Doubled, claim)
		}
	}
	for _, page := range pages {
		if writers[page.Route] == 0 {
			coverage.Unclaimed = append(coverage.Unclaimed, page.Route)
		}
	}
	if coverage.Red() {
		coverage.Spec = coverageSpec
	}
	return coverage, nil
}

// checkCoverage answers the coverage of an artifact this process did not build.
func checkCoverage(paths Paths) (Coverage, error) {
	claims, err := readProducerRecord(paths)
	if err != nil {
		return Coverage{}, err
	}
	return coverageOf(paths.Output, claims)
}

// producerRecordDirectory holds one record for each artifact this checkout has
// built, under the checkout's own scratch area.
//
// The record exists because a producer answers what it WROTE, and only a build
// runs a producer. `./le site check` reads an artifact it did not build, so a
// build states its claim where the check can read it back.
//
// It lives in the CHECKOUT and never in the artifact, because the artifact is
// published: a bookkeeping file written there is served to a reader of the
// public site. A verification-debt shard reached the live site for exactly this
// reason, and the artifact is trimmed of source-only paths AFTER a producer
// runs, so the exclusion list cannot fix it either.
const producerRecordDirectory = "tmp/site"

// producerRecordPath names the record describing one artifact.
//
// The name is keyed by the artifact's own path, so a build into gh-pages and a
// build into a scratch tree keep separate records rather than overwriting each
// other's. The key is a digest because an artifact path is not a file name.
func producerRecordPath(paths Paths) string {
	key := sha256.Sum256([]byte(filepath.Clean(paths.Output)))
	name := "producers-" + hex.EncodeToString(key[:8]) + ".json"
	return filepath.Join(paths.Repository, filepath.FromSlash(producerRecordDirectory), name)
}

// producerRecord is the on-disk form of one build's claims. It names the
// artifact it describes, so a reader of the scratch directory can tell the
// records apart without recomputing the key.
type producerRecord struct {
	Artifact string  `json:"artifact"`
	Claims   []Claim `json:"claims"`
}

// writeProducerRecord states which producer wrote which route of one artifact.
func writeProducerRecord(paths Paths, claims []Claim) error {
	if claims == nil {
		claims = []Claim{}
	}
	content, err := json.MarshalIndent(producerRecord{Artifact: paths.Output, Claims: claims}, "", "  ")
	if err != nil {
		return err
	}
	path := producerRecordPath(paths)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// readProducerRecord answers the claims recorded for one artifact.
//
// An artifact with no record has had no producer run over it, so every page it
// publishes is unclaimed. That is the honest answer and it is red, so the
// absent file fails closed rather than reading as full coverage.
func readProducerRecord(paths Paths) ([]Claim, error) {
	path := producerRecordPath(paths)
	content, err := os.ReadFile(path) //nolint:gosec // the path is this checkout's own record of the artifact it was pointed at
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record producerRecord
	if err := json.Unmarshal(content, &record); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return record.Claims, nil
}
