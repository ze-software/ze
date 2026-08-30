// Design: website/AI.md -- the site searches itself in the reader's browser
package site

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// searchArtifact is the artifact the index tests walk: one page mirror at the
// site root, two under it, the configuration reference that is indexed section
// by section instead, and one Markdown file under each directory the walk skips.
var searchArtifact = map[string]string{
	"index.md":                         "# Ze\n\nZe is a **network operating system**.\n",
	"docs/index.md":                    "# Documentation\n\nEvery guide, in one hub.\n",
	"guides/quickstart/index.md":       "# Quickstart\n\nPeer with a neighbor in ten minutes.\n",
	"reference/configuration/index.md": "# Configuration Reference\n\nThe whole YANG tree, dumped.\n",
	"assets/index.md":                  "# Assets\n\nA stylesheet is not a page.\n",
	"data/index.md":                    "# Data\n\nA data file is not a page.\n",
	"tmp/index.md":                     "# Scratch\n\nA build's own scratch is not a page.\n",
}

// searchConfigTree is the published configuration tree the index reads, with
// two top-level sections so the tie between them has a stated order.
const searchConfigTree = `{"bgp":{"kind":"container","description":"BGP speaker configuration.",` +
	`"children":[{"name":"neighbor","kind":"list[name]","description":"One peering session."}]},` +
	`"interface":{"kind":"container","description":"Interface configuration.","children":[]}}`

// searchPaths lays out an artifact carrying searchArtifact and the published
// configuration tree, with a source tree that states no sidebar.
func searchPaths(t *testing.T) Paths {
	t.Helper()
	output := t.TempDir()
	for name, content := range searchArtifact {
		writeArtifactFile(t, output, name, content)
	}
	writeArtifactFile(t, output, configTreeFile, searchConfigTree)
	return Paths{Repository: t.TempDir(), Source: t.TempDir(), Output: output}
}

// publishedSearchIndex answers the index one render wrote, decoded.
func publishedSearchIndex(t *testing.T, output string) []searchRecord {
	t.Helper()
	var records []searchRecord
	if err := json.Unmarshal([]byte(readArtifact(t, output, searchIndexFile)), &records); err != nil {
		t.Fatalf("the published index is not JSON: %v", err)
	}
	return records
}

// VALIDATES: AC-13 -- the index is built from the ARTIFACT this build wrote:
// one record for every published Markdown mirror, and nothing for a directory
// that publishes no page.
//
// The method seeds one mirror under each directory the walk must skip. A record
// for any of them would mean the index carries a file no reader can open, and
// an index carried forward from the previous build would carry records for
// pages this artifact does not hold at all.
func TestTheSearchIndexCarriesOneRecordForEveryPublishedMirror(t *testing.T) {
	paths := searchPaths(t)

	routes, err := renderSearch(paths)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(routes, []string{"/" + searchDirectory + "/"}) {
		t.Errorf("the search producer claimed %v, want the one page it writes", routes)
	}
	records := publishedSearchIndex(t, paths.Output)
	urls := make([]string, 0, len(records))
	for _, record := range records {
		urls = append(urls, record.URL)
	}
	// The search page is written BEFORE the walk, so it is in its own index.
	want := []string{
		"", "docs/", "guides/quickstart/",
		configurationReferenceDirectory + "/",
		configurationReferenceDirectory + "/#bgp",
		configurationReferenceDirectory + "/#interface",
		searchDirectory + "/",
	}
	if !slices.Equal(urls, want) {
		t.Fatalf("the index answers %v, want %v", urls, want)
	}
	for _, record := range records {
		if record.DisplayTitle != record.Title || record.DisplaySection != record.Section {
			t.Errorf("%s states two titles or two sections: %#v", record.URL, record)
		}
	}
	root := records[0]
	if root.Title != "Ze" || root.Section != "Home" {
		t.Errorf("the site root is indexed as %q under %q, want its heading under Home", root.Title, root.Section)
	}
	if !strings.Contains(root.Text, "network operating system") {
		t.Errorf("the site root's body is %q, want the words its mirror states", root.Text)
	}
	if strings.ContainsAny(root.Text, "#*") {
		t.Errorf("the site root's body is %q, want the words a reader types rather than the Markdown", root.Text)
	}
	if section := records[2].Section; section != "guides" {
		t.Errorf("guides/quickstart/ is indexed under %q, want its first path segment", section)
	}
}

// VALIDATES: AC-13 -- the configuration reference is indexed section by
// section, deep-linked to the hash the in-page explorer routes on.
//
// Its Markdown mirror is one dump of the whole YANG tree, well past the page
// cap, so one capped record buried every configuration term in it and a search
// for a section name found nothing.
func TestTheConfigurationReferenceIsIndexedSectionBySection(t *testing.T) {
	paths := searchPaths(t)
	if _, err := renderSearch(paths); err != nil {
		t.Fatal(err)
	}
	records := publishedSearchIndex(t, paths.Output)

	byURL := make(map[string]searchRecord, len(records))
	for _, record := range records {
		byURL[record.URL] = record
	}
	landing := byURL[configurationReferenceDirectory+"/"]
	if landing.Title != "Configuration Reference" {
		t.Errorf("the configuration landing record is titled %q", landing.Title)
	}
	if strings.Contains(landing.Text, "dumped") {
		t.Errorf("the landing record carries the mirror's own body: %q", landing.Text)
	}
	for _, name := range []string{"bgp", "interface"} {
		section := byURL[configurationReferenceDirectory+"/#"+name]
		if section.Title != name {
			t.Errorf("the %s section is titled %q", name, section.Title)
		}
		if !strings.Contains(landing.Text, name) {
			t.Errorf("the landing record does not name the %s section: %q", name, landing.Text)
		}
	}
	if body := byURL[configurationReferenceDirectory+"/#bgp"].Text; !strings.Contains(body, "neighbor <name>") {
		t.Errorf("the bgp section is indexed as %q, want the names its subtree states", body)
	}
}

// VALIDATES: AC-13 -- two records sharing one URL keep the order they were
// discovered in.
//
// No pair of published records shares a URL today: a page mirror is one for
// each directory and a configuration section is one for each name, so the
// artifact cannot produce the tie. That is exactly why the property needs a
// constructed case. sort.Slice is NOT stable, it is one word away from the sort
// this index uses, and nothing derived from the artifact would fail if somebody
// wrote it.
//
// The fixture is forty records because Go sorts twelve or fewer by insertion,
// which is stable by accident: a smaller case would pass against sort.Slice and
// prove nothing. Measured 2026-08-30 -- thirteen records is the smallest size
// at which the two sorts disagree.
func TestTwoSearchRecordsSharingAURLKeepTheDiscoveryOrder(t *testing.T) {
	const records, urls = 40, 4
	discovered := make([]searchRecord, 0, records)
	for position := range records {
		discovered = append(discovered, searchRecord{
			Title: "page " + strconv.Itoa(position),
			URL:   "section-" + strconv.Itoa(position%urls) + "/",
		})
	}
	want := slices.Clone(discovered)

	sortSearchRecords(discovered)

	slices.SortStableFunc(want, func(left, right searchRecord) int { return strings.Compare(left.URL, right.URL) })
	for position := range want {
		if discovered[position] != want[position] {
			t.Fatalf("record %d is %q at %s, want %q; the tie order the walk stated was discarded",
				position, discovered[position].Title, discovered[position].URL, want[position].Title)
		}
	}
}

// VALIDATES: AC-13 -- the index is published as one record per line, in
// ascending URL order.
//
// One record per line keeps a content change to the lines that changed. A
// minified single line would make every edit read as a full-file replacement,
// which is what the site's own diff would then show for a one-word fix.
func TestTheSearchIndexIsOnePublishedRecordPerLine(t *testing.T) {
	paths := searchPaths(t)
	if _, err := renderSearch(paths); err != nil {
		t.Fatal(err)
	}
	published := readArtifact(t, paths.Output, searchIndexFile)
	records := publishedSearchIndex(t, paths.Output)

	lines := strings.Split(strings.TrimSuffix(published, "\n"), "\n")
	if len(lines) != len(records)+2 {
		t.Fatalf("the index carries %d lines for %d records, want one record per line inside a bracket pair",
			len(lines), len(records))
	}
	for position := 1; position < len(records); position++ {
		if records[position-1].URL > records[position].URL {
			t.Errorf("record %d is at %s and record %d at %s; the index is ordered by URL",
				position-1, records[position-1].URL, position, records[position].URL)
		}
	}
	// A data file, not a page: the angle bracket a keyed list's head carries
	// must read as itself rather than as an escape a reader has to decode.
	if !strings.Contains(published, "neighbor <name>") {
		t.Error("the index does not carry the head a keyed list states")
	}
	if strings.Contains(published, `\u003c`) {
		t.Error("the index escapes HTML; it is a data file the page reads, not markup")
	}
}
