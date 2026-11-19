// Design: docs/contributing/gh-pages.md -- the looking glass replay: the recording file
// Related: record.go -- the run that fills a Recording
// Related: tour.go -- the questions a recording asks
//
// Package sitelgreplay records the looking glass for the website. It asks a
// real looking glass, serving the simulated network, a declared tour of
// questions over HTTP and writes every answer into one JSON file. The site
// build embeds that file in a frame where the player script
// (website/assets/js/lg-replay.js) answers the looking glass's own requests
// from it.
//
// The format, the crawl (Crawler) and the chunk helpers also serve a second
// product, the fleet interface, whose recording states what it needs in the
// optional fields of Recording.
package sitelgreplay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
)

// Schema is the version of the file format. Read and the player both refuse
// another one. Version 2 writes every key in kebab-case (commit-date,
// tree-dirty, content-type), as the answers of the rest of the tooling do.
const Schema = 2

// File is where the recording lives in a checkout, relative to its root. It is
// committed: the site build reads it and never records.
const File = "website/data/lg-replay.json"

// RecorderCommand is the command that records the looking glass. A recording
// names it as its recorder, and a refusal that asks for a new recording names
// it as the fix.
const RecorderCommand = "./le site lg-replay record"

// Recording is one recorded looking glass.
type Recording struct {
	Schema     int        `json:"schema"`
	Provenance Provenance `json:"provenance"`
	// Entry is the key of the document a replay opens on.
	Entry string `json:"entry"`
	// Tour is the declared questions, in the order they were asked.
	Tour []TourStep `json:"tour"`
	// Exchanges is every answer: the tour's first, then the requests the
	// recorded answers themselves name, in the order they were found.
	Exchanges []Exchange `json:"exchanges"`
	// Assets is the looking glass's own stylesheet, scripts and images.
	Assets []Asset `json:"assets"`
	// Stream is a sample of the live event stream. It is nil when no recorded
	// answer names a stream, and the player then answers one with its notice.
	Stream *Stream `json:"stream,omitempty"`

	// The three fields below serve a recording of another product, the fleet
	// interface. Each is optional and omitted when empty, so a looking glass
	// recording holds none of them and means what it meant before they
	// existed: the schema did not change.

	// Player is the product's settings for the player. Nil means the looking
	// glass's own, which are the defaults Player states field by field.
	Player *Player `json:"player,omitempty"`
	// Chunks are the parts of a recording too large to travel in its frame.
	// Each chunk's answers are a file of their own (EncodeChunk), which the
	// host page fetches when the frame first asks one of its keys. An answer a
	// chunk holds is not in Exchanges.
	Chunks []Chunk `json:"chunks,omitempty"`
	// Fallbacks answer a request that neither Exchanges nor a chunk holds: the
	// first rule whose method and path prefix match it answers.
	Fallbacks []Fallback `json:"fallbacks,omitempty"`
}

// NoticeRequestToken stands in Player.NoticeText for the request the
// recording does not hold, as the player names it.
const NoticeRequestToken = "{request}"

// Player is what the player needs to know about the product it replays. Every
// field is optional, and an empty one takes the looking glass's value, stated
// on the field.
type Player struct {
	// Title is the frame document's title. Default: "Ze looking glass replay".
	Title string `json:"title,omitempty"`
	// Subject names what the frame document replays, in the paragraph a
	// reader sees when the document is opened outside its page. Default:
	// "Ze looking glass".
	Subject string `json:"subject,omitempty"`
	// HostPage is the address of the page that plays the frame, relative to
	// the frame document, which that paragraph links. Default: "../".
	HostPage string `json:"host-page,omitempty"`
	// AssetPrefixes are the path prefixes of the product's own static files:
	// an image a swapped answer names under one of them is put inline from
	// Assets. Default: "/lg/assets/".
	AssetPrefixes []string `json:"asset-prefixes,omitempty"`
	// NoticeText is the notice that stands in the page in place of an answer
	// the recording does not hold. NoticeRequestToken in it is replaced by the
	// request. Default: "Not in this recording: {request}. The replay holds the
	// answers of the tour and of the rows its pages show."
	NoticeText string `json:"notice-text,omitempty"`
	// NoticeClass is the class of the notice's element, so the product's own
	// stylesheet styles it. Default: "error-banner".
	NoticeClass string `json:"notice-class,omitempty"`
	// SilentEventSource gives the frame an EventSource that opens and never
	// delivers a message, so a page's stream client sees a connected, quiet
	// stream. Default: off, and the frame has the browser's EventSource,
	// which its policy refuses.
	SilentEventSource bool `json:"silent-event-source,omitempty"`
	// KeepPushedAddress keeps the last address the page pushed into the
	// history, and resolves the page's relative addresses against it. Default:
	// off, and a push is dropped.
	KeepPushedAddress bool `json:"keep-pushed-address,omitempty"`
	// PlainForms answers a form that no htmx attribute submits, and whose
	// page's own script leaves its submission alone, as the browser would: a
	// GET form opens its action with its fields as a link to them would, a
	// dialog form closes its dialog, and any other shows the notice. Default:
	// off, and every such form shows the notice.
	PlainForms bool `json:"plain-forms,omitempty"`
}

// Chunk is one part of a recording kept in a file of its own.
//
// A chunk lists its own keys, rather than the recording holding a map from
// key to chunk: each key and each chunk name is then written once, the file
// says which chunk the frame must ask for in the same place it says what the
// chunk holds, and the player builds its map in one pass.
type Chunk struct {
	// Name is how the frame asks the host for the chunk. The host maps it to
	// File itself, so the frame can name a chunk and never a URL.
	Name string `json:"name"`
	// File is the chunk's file, relative to the frame document, in the same
	// directory.
	File string `json:"file"`
	// Size is the file's length in bytes.
	Size int `json:"size"`
	// Keys are the keys of the exchanges the file holds, in its order.
	Keys []string `json:"keys"`
}

// Fallback answers every request of one method under one path prefix that no
// exchange holds.
type Fallback struct {
	Method     string   `json:"method"`
	PathPrefix string   `json:"path-prefix"`
	Exchange   Exchange `json:"exchange"`
}

// chunkNamePattern is what a chunk name may hold. The name becomes the file's
// name, and the host refuses a file name with any other character, so no
// chunk can name a file outside the frame's directory.
var chunkNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// chunkFileExtension ends every chunk file's name.
const chunkFileExtension = ".json"

// ChunkSplit is a list of exchanges divided into what travels in the frame
// and what goes to chunk files.
type ChunkSplit struct {
	// Inline are the exchanges the frame carries, in their order.
	Inline []Exchange
	// Chunks are the chunks, in the order their first exchange came.
	Chunks []Chunk
	// Contents holds each chunk file's bytes by the chunk's name.
	Contents map[string][]byte
}

// SplitChunks divides exchanges by group, which answers the name of the chunk
// an exchange belongs to, or "" for one the frame carries. A chunk keeps its
// exchanges in their order. A name that is not a chunk name, or a key two
// exchanges share, is refused.
func SplitChunks(exchanges []Exchange, group func(Exchange) string) (ChunkSplit, error) {
	split := ChunkSplit{Contents: map[string][]byte{}}
	members := map[string][]Exchange{}
	keys := map[string]bool{}
	for index := range exchanges {
		exchange := exchanges[index]
		if keys[exchange.Key] {
			return ChunkSplit{}, fmt.Errorf("two exchanges have the key %s", exchange.Key)
		}
		keys[exchange.Key] = true
		name := group(exchange)
		if name == "" {
			split.Inline = append(split.Inline, exchange)
			continue
		}
		if !chunkNamePattern.MatchString(name) {
			return ChunkSplit{}, fmt.Errorf("chunk name %q for %s: a chunk name is lower-case letters, digits and hyphens, "+
				"starting with a letter or a digit", name, exchange.Key)
		}
		if _, known := members[name]; !known {
			split.Chunks = append(split.Chunks, Chunk{Name: name, File: name + chunkFileExtension})
		}
		members[name] = append(members[name], exchange)
	}
	for index := range split.Chunks {
		chunk := &split.Chunks[index]
		content, err := EncodeChunk(members[chunk.Name])
		if err != nil {
			return ChunkSplit{}, fmt.Errorf("chunk %s: %w", chunk.Name, err)
		}
		chunk.Size = len(content)
		for _, exchange := range members[chunk.Name] {
			chunk.Keys = append(chunk.Keys, exchange.Key)
		}
		split.Contents[chunk.Name] = content
	}
	return split, nil
}

// EncodeChunk answers a chunk file's bytes: the exchanges as one JSON list,
// with no indentation, because a chunk is read by the player and never by a
// person, and one trailing newline.
func EncodeChunk(exchanges []Exchange) ([]byte, error) {
	if exchanges == nil {
		exchanges = []Exchange{}
	}
	var out bytes.Buffer
	if err := json.NewEncoder(&out).Encode(exchanges); err != nil {
		return nil, fmt.Errorf("encode a recording chunk: %w", err)
	}
	return out.Bytes(), nil
}

// ChunkFiles answers the value of the host mount's data-lg-replay-chunks
// attribute: a JSON object from each chunk's name to its file. The host page
// fetches only the files this names, so the frame can ask for a chunk by name
// and never make the host fetch an address of its choosing. The caller escapes
// the value for the attribute.
func ChunkFiles(chunks []Chunk) (string, error) {
	files := make(map[string]string, len(chunks))
	for _, chunk := range chunks {
		files[chunk.Name] = chunk.File
	}
	// encoding/json writes a map's keys sorted, so the value is stable.
	encoded, err := json.Marshal(files)
	if err != nil {
		return "", fmt.Errorf("encode the chunk files: %w", err)
	}
	return string(encoded), nil
}

// checkParts refuses a recording whose chunks or fallbacks contradict each
// other or name a file the host would refuse: a chunk name or file that is not
// one, two chunks of one name, a key that two chunks hold or that also travels
// in the frame, and a fallback with no method or a prefix that is no path.
func (r Recording) checkParts() error {
	held := map[string]string{}
	for index := range r.Exchanges {
		held[r.Exchanges[index].Key] = "the frame"
	}
	names := map[string]bool{}
	for _, chunk := range r.Chunks {
		if !chunkNamePattern.MatchString(chunk.Name) {
			return fmt.Errorf("chunk name %q is not a chunk name: lower-case letters, digits and hyphens", chunk.Name)
		}
		if names[chunk.Name] {
			return fmt.Errorf("two chunks are named %s", chunk.Name)
		}
		names[chunk.Name] = true
		if chunk.File != chunk.Name+chunkFileExtension {
			return fmt.Errorf("chunk %s names the file %q, and its file is %q", chunk.Name, chunk.File, chunk.Name+chunkFileExtension)
		}
		for _, key := range chunk.Keys {
			if holder, found := held[key]; found {
				return fmt.Errorf("chunk %s holds %s, which %s also holds", chunk.Name, key, holder)
			}
			held[key] = "chunk " + chunk.Name
		}
	}
	for index, fallback := range r.Fallbacks {
		if fallback.Method == "" {
			return fmt.Errorf("fallback %d names no method", index+1)
		}
		if !strings.HasPrefix(fallback.PathPrefix, "/") {
			return fmt.Errorf("fallback %d has the path prefix %q, which does not start with /", index+1, fallback.PathPrefix)
		}
	}
	return nil
}

// Provenance says where a recording comes from and what was changed in it.
type Provenance struct {
	// Recorder is the command that wrote the file.
	Recorder string `json:"recorder"`
	// Commit and CommitDate name the checkout's last commit. The date is the
	// commit's, never the run's, so two runs on one commit state the same.
	Commit     string `json:"commit"`
	CommitDate string `json:"commit-date"`
	// TreeDirty says the checkout held uncommitted changes, so Commit alone
	// does not name the code that answered.
	TreeDirty bool `json:"tree-dirty"`
	// Seed generated the simulated network.
	Seed uint64 `json:"seed"`
	// Speakers is how many simulated BGP speakers held a session.
	Speakers int `json:"speakers"`
	// Normalized lists every value replaced because it changes run to run.
	Normalized []Normalization `json:"normalized"`
	// Varies lists the key of every answer that still differed between the two
	// reads of one run, after normalization. Empty means two recordings of
	// one commit are byte-identical.
	Varies []string `json:"varies"`
}

// Normalization is one declared replacement and how often it applied.
type Normalization struct {
	Name        string `json:"name"`
	What        string `json:"what"`
	Replacement string `json:"replacement"`
	Count       int    `json:"count"`
}

// TourStep is one step of the tour as the player shows it.
type TourStep struct {
	Caption string `json:"caption"`
	// Exchange is the key of the answer the step asks for.
	Exchange string `json:"exchange"`
}

// Field is one name and value of a query or of a form body.
type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Exchange is one request and the answer the looking glass gave it.
type Exchange struct {
	// Key identifies the request: the player computes the same key for a
	// request the page makes (ExchangeKey).
	Key    string `json:"key"`
	Method string `json:"method"`
	Path   string `json:"path"`
	// Query and Form are the request's fields in the order they were sent.
	Query []Field `json:"query,omitempty"`
	Form  []Field `json:"form,omitempty"`
	// HTMX says the request carried HX-Request, which the looking glass
	// answers with a fragment where a plain request gets the whole page.
	HTMX bool `json:"htmx"`
	// Via is "tour" for a declared question and "link" for a request a
	// recorded answer names.
	Via         string `json:"via"`
	Status      int    `json:"status"`
	ContentType string `json:"content-type"`
	Body        string `json:"body"`
}

// Asset is one static file of the looking glass.
type Asset struct {
	Path        string `json:"path"`
	ContentType string `json:"content-type"`
	Body        string `json:"body"`
}

// Stream is the first messages of the live event stream.
type Stream struct {
	Path        string `json:"path"`
	ContentType string `json:"content-type"`
	// Messages are whole server-sent messages, each without its closing blank
	// line.
	Messages []string `json:"messages"`
}

const (
	viaTour = "tour"
	viaLink = "link"
)

// ExchangeKey answers the identity of one request.
//
// The player computes the same text in the browser, so the rule is stated
// here once and website/assets/js/lg-replay.js follows it: the method, the
// path, the query fields, the form fields, and whether htmx asked. Fields are
// compared decoded, sorted, and without the empty ones, because a browser
// sends every field of a form, in document order, with its own escaping, and
// the looking glass reads an empty field as an absent one.
func ExchangeKey(method, path string, query, form []Field, htmx bool) string {
	var key strings.Builder
	key.WriteString(strings.ToUpper(method))
	key.WriteString(" ")
	key.WriteString(path)
	if fields := canonicalFields(query); fields != "" {
		key.WriteString("?")
		key.WriteString(fields)
	}
	if fields := canonicalFields(form); fields != "" {
		key.WriteString(" form ")
		key.WriteString(fields)
	}
	if htmx {
		key.WriteString(" htmx")
	}
	return key.String()
}

// canonicalFields joins the non-empty fields as name=value, sorted.
func canonicalFields(fields []Field) string {
	pairs := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.Value == "" {
			continue
		}
		pairs = append(pairs, field.Name+"="+field.Value)
	}
	slices.Sort(pairs)
	return strings.Join(pairs, "&")
}

// splitTarget splits a path-and-query, as the looking glass markup writes one,
// into its path and its decoded query fields in order.
func splitTarget(target string) (string, []Field, error) {
	path, raw, _ := strings.Cut(target, "?")
	if !strings.HasPrefix(path, "/") {
		return "", nil, fmt.Errorf("request target %q does not start with a path", target)
	}
	fields, err := decodeFields(raw)
	if err != nil {
		return "", nil, fmt.Errorf("request target %q: %w", target, err)
	}
	return path, fields, nil
}

// decodeFields decodes an encoded query or form body, keeping its order.
func decodeFields(raw string) ([]Field, error) {
	if raw == "" {
		return nil, nil
	}
	var fields []Field
	for pair := range strings.SplitSeq(raw, "&") {
		if pair == "" {
			continue
		}
		name, value, _ := strings.Cut(pair, "=")
		decodedName, err := url.QueryUnescape(name)
		if err != nil {
			return nil, fmt.Errorf("field name %q: %w", name, err)
		}
		decodedValue, err := url.QueryUnescape(value)
		if err != nil {
			return nil, fmt.Errorf("field value %q: %w", value, err)
		}
		fields = append(fields, Field{Name: decodedName, Value: decodedValue})
	}
	return fields, nil
}

// encodeFields encodes fields for a query or a form body, in their order.
func encodeFields(fields []Field) string {
	pairs := make([]string, 0, len(fields))
	for _, field := range fields {
		pairs = append(pairs, url.QueryEscape(field.Name)+"="+url.QueryEscape(field.Value))
	}
	return strings.Join(pairs, "&")
}

// rule is one value that changes run to run and the fixed value it becomes.
type rule struct {
	name        string
	what        string
	pattern     *regexp.Regexp
	replacement string
	// replacementText is the fixed value as a reader of the file sees it.
	replacementText string
}

// The fixed values. An uptime of one minute and a change time of the first
// second of 2026 are plainly not measurements, which is the point: the file
// declares them, and nobody reads them as the run's.
const (
	fixedUptime       = "1m 0s"
	fixedStateChanged = "2026-01-01T00:00:00Z"
	fixedListener     = "looking-glass.example"
	// fixedAgeText is what formatElapsed (internal/component/lg/render.go)
	// answers for fixedAgeSeconds: 192 seconds are 3 minutes and 12 seconds.
	fixedAgeSeconds = "192"
	fixedAgeText    = "3m 12s"
)

// markupRules are the values the looking glass markup carries that depend on
// when the run was made. Each pattern names the element that holds the value
// (formatUptime, peersTableBody and peerRoutesContent in
// internal/component/lg), so a number elsewhere on the page is left alone.
var markupRules = []rule{
	{
		name: "uptime", what: "how long a session has been up, in the peers table",
		pattern:     regexp.MustCompile(`<td>(?:\d+d \d+h \d+m|\d+h \d+m \d+s|\d+m \d+s|\d+s)</td>`),
		replacement: "<td>" + fixedUptime + "</td>", replacementText: fixedUptime,
	},
	{
		name: "state-changed", what: "the time of a session's last state change, in the peers table",
		pattern:     regexp.MustCompile(`(<td class="state-changed">)[^<]+(</td>)`),
		replacement: "${1}" + fixedStateChanged + "${2}", replacementText: fixedStateChanged,
	},
	{
		// The age is written twice in one element: the exact seconds in
		// data-seconds and the text formatElapsed makes of them. Both become
		// the one fixed age, so the pair stays consistent. A route with no
		// known age has an empty cell and no attribute, and is left alone.
		name: "route-age", what: "how long ago a route last changed, in a route table and in a route's detail",
		pattern:     regexp.MustCompile(`(class="route-age" data-seconds=")\d+(">)[^<]*(<)`),
		replacement: "${1}" + fixedAgeSeconds + "${2}" + fixedAgeText + "${3}", replacementText: fixedAgeSeconds + " seconds, shown as " + fixedAgeText,
	},
	{
		name: "peer-state-changed", what: "the time of a session's last state change, in a peer's header",
		pattern:     regexp.MustCompile(`(Last change <time>)[^<]+(</time>)`),
		replacement: "${1}" + fixedStateChanged + "${2}", replacementText: fixedStateChanged,
	},
}

// normalizer replaces what changes run to run and counts each replacement.
// It is not safe for concurrent use: one recording run owns one.
type normalizer struct {
	rules  []rule
	counts []int
}

// newNormalizer answers the markup rules plus one for listener, the host and
// port the looking glass listened on in this run.
func newNormalizer(listener string) *normalizer {
	rules := append([]rule{}, markupRules...)
	rules = append(rules, rule{
		name: "listener", what: "the address the recorded looking glass listened on",
		pattern:     regexp.MustCompile(regexp.QuoteMeta(listener)),
		replacement: fixedListener, replacementText: fixedListener,
	})
	return &normalizer{rules: rules, counts: make([]int, len(rules))}
}

// apply answers text with every rule applied, counting when count is set. The
// second read of a run is normalized without counting, so a count states what
// the file holds.
func (n *normalizer) apply(text string, count bool) string {
	for index, rule := range n.rules {
		if count {
			n.counts[index] += len(rule.pattern.FindAllStringIndex(text, -1))
		}
		text = rule.pattern.ReplaceAllString(text, rule.replacement)
	}
	return text
}

// declared answers every rule with its count, a rule that never applied
// included: a zero says the run held no such value.
func (n *normalizer) declared() []Normalization {
	declared := make([]Normalization, 0, len(n.rules))
	for index, rule := range n.rules {
		declared = append(declared, Normalization{
			Name: rule.name, What: rule.what, Replacement: rule.replacementText, Count: n.counts[index],
		})
	}
	return declared
}

var (
	followUpAttribute = regexp.MustCompile(`\bhx-get="([^"]+)"`)
	streamAttribute   = regexp.MustCompile(`\bhx-sse:connect="([^"]+)"`)
)

// attributeValues answers the distinct values of one attribute in markup, in
// the order they first appear, with character references decoded.
func attributeValues(pattern *regexp.Regexp, markup string) []string {
	var values []string
	seen := map[string]bool{}
	for _, match := range pattern.FindAllStringSubmatch(markup, -1) {
		value := html.UnescapeString(match[1])
		if seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	return values
}

// isMarkup reports whether an answer of this content type is HTML, the only
// kind that names further requests.
func isMarkup(contentType string) bool {
	return strings.HasPrefix(contentType, "text/html")
}

// Encode answers the file's bytes: indented JSON, one trailing newline. The
// encoding escapes <, > and &, so the same bytes are safe inside a script
// element of the frame document.
func (r Recording) Encode() ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetIndent("", " ")
	if err := encoder.Encode(r); err != nil {
		return nil, fmt.Errorf("encode the looking glass recording: %w", err)
	}
	return out.Bytes(), nil
}

// Read answers the recording at path. A missing file is the caller's to tell
// apart with errors.Is(err, fs.ErrNotExist): a checkout that never recorded.
func Read(path string) (Recording, error) {
	content, err := os.ReadFile(path) //nolint:gosec // the recording of the checkout the caller named
	if err != nil {
		return Recording{}, err
	}
	// The version is read first and alone. A file of another version holds
	// other keys, which the strict read below would refuse by naming a key,
	// and the reader needs to be told to record again instead.
	var version struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(content, &version); err != nil {
		return Recording{}, fmt.Errorf("%s: %w", path, err)
	}
	if version.Schema != Schema {
		return Recording{}, opererr.New(
			fmt.Sprintf("%s has schema %d, and this build reads schema %d", path, version.Schema, Schema),
			"The file was written by another version of the recorder, so its keys are not the ones this build reads. Record it again",
			RecorderCommand)
	}
	var recording Recording
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&recording); err != nil {
		return Recording{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := recording.checkParts(); err != nil {
		return Recording{}, fmt.Errorf("%s: %w", path, err)
	}
	return recording, nil
}

// Exchange answers the exchange recorded under key.
func (r Recording) Exchange(key string) (Exchange, bool) {
	for index := range r.Exchanges {
		if r.Exchanges[index].Key == key {
			return r.Exchanges[index], true
		}
	}
	return Exchange{}, false
}
