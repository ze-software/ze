// Design: docs/contributing/gh-pages.md -- the looking glass replay: one recording run
// Related: recording.go -- the Recording this fills, and the normalizer
// Related: tour.go -- the steps it asks
// Related: actions.go -- runRecord, which starts the network and calls record

package sitelgreplay

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// exchangesMax bounds the requests one recording holds. The simulated
	// network needs a few dozen: the tour, each peer's routes and each route's
	// detail. A run that reaches the bound is refused, never cut short.
	exchangesMax = 400
	// bodyBytesMax bounds one answer. The largest looking glass asset is under
	// a hundred kilobytes.
	bodyBytesMax = 4 << 20
	// streamMessages is how many messages of the event stream are kept. The
	// looking glass sends one every five seconds (handleUIEvents).
	streamMessages = 2
	// streamWait bounds the wait for those messages.
	streamWait = time.Minute
	// secondReadGap is the least time between the two reads of one answer. An
	// uptime or an age counted in seconds differs across it, so a value no
	// rule covers shows up as an answer that varies.
	secondReadGap = 10 * time.Second
	// requestWait bounds one request.
	requestWait = 30 * time.Second
	// lookingGlassAssets is where the looking glass serves its static files.
	lookingGlassAssets = "/lg/assets/"
	// lookingGlassBound is what the error of a walk that reached exchangesMax
	// tells the reader to change.
	lookingGlassBound = "the simulated network grew, so raise exchangesMax in internal/le/site/lgreplay/record.go"

	headerHTMX = "HX-Request"
	// headerHTMXType is what htmx says it will swap: "full" when the target
	// is the body or the element selects part of the answer (hx-select),
	// "partial" otherwise. No recorded page targets the body or selects, so
	// every htmx request is a partial one, and a server that answers a partial
	// request with its region alone (the fleet shell, asksPart) is recorded as
	// the browser sees it.
	headerHTMXType = "HX-Request-Type"
	// headerHTMXCurrent is the address of the page a request leaves, which
	// htmx sends with every request. A follow-up names the answer it was
	// found in, so a server that answers by the page left (the fleet shell,
	// keepsSidebar) is recorded as the browser leaving that page sees it. A
	// step names none: it is asked as from nowhere.
	headerHTMXCurrent = "HX-Current-URL"
	htmxPartial       = "partial"
	headerContentType = "Content-Type"
	formContentType   = "application/x-www-form-urlencoded"
	streamContentType = "text/event-stream"
)

// lookingGlass is the looking glass one run reads.
type lookingGlass struct {
	// base is the looking glass as a URL with no path.
	base string
	// wait returns once d has passed, or with the context's error. A test
	// hands one that returns at once.
	wait func(ctx context.Context, d time.Duration) error
}

// waitFor is the wait of a real run.
func waitFor(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// request is one question, ready to ask.
type request struct {
	method string
	path   string
	query  []Field
	form   []Field
	htmx   bool
	via    string
	// current is the address of the page the request leaves, empty for a
	// step. It is not part of the key: the player answers a request by what
	// it asks, wherever it is asked from.
	current string
}

func (r request) key() string {
	return ExchangeKey(r.method, r.path, r.query, r.form, r.htmx)
}

// crawler asks one server and follows what its answers name: the part of a
// run the looking glass recorder and Crawler share. It is not safe for
// concurrent use.
type crawler struct {
	// base is the server as a URL with no path.
	base   string
	client *http.Client
	// server names what answered, in an error: "the looking glass".
	server string
	// assetAttribute finds the static files an answer loads. Nil finds none.
	assetAttribute *regexp.Regexp
	exchangesMax   int
	bodyBytesMax   int
	// bound says, in the error of a walk that reached exchangesMax, what to
	// change.
	bound string
	// normalize rewrites an answer's body. count says the answer is one the
	// recording keeps, so its replacements are counted.
	normalize func(text string, count bool) string
	// follow decides whether a request an answer names is asked. Nil asks
	// every one.
	follow func(parent Exchange, request request) bool
}

// recorder is one run of the looking glass recorder. It is not safe for
// concurrent use.
type recorder struct {
	lookingGlass lookingGlass
	crawler      crawler
	normalizer   *normalizer
}

// record asks lookingGlass every step of steps, then every request
// the answers name, its assets and a sample of its event stream, and answers
// the recording. provenance arrives with what the caller knows (the commit,
// the seed) and leaves with what the run learned (Normalized, Varies).
//
// Every question is asked a second time at least secondReadGap after the
// first pass ended, and an answer that differs after normalization is named
// in Varies.
func record(ctx context.Context, lookingGlass lookingGlass, steps []Step, provenance Provenance) (Recording, error) {
	listener, err := url.Parse(lookingGlass.base)
	if err != nil {
		return Recording{}, fmt.Errorf("looking glass address %q: %w", lookingGlass.base, err)
	}
	normalizer := newNormalizer(listener.Host)
	recorder := recorder{
		lookingGlass: lookingGlass,
		crawler: crawler{
			base: lookingGlass.base,
			// A redirect is an answer like any other: it is recorded, not followed.
			client: &http.Client{
				Timeout:       requestWait,
				Transport:     &http.Transport{Proxy: nil},
				CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			},
			server:         "the looking glass",
			assetAttribute: assetPattern([]string{lookingGlassAssets}),
			exchangesMax:   exchangesMax,
			bodyBytesMax:   bodyBytesMax,
			bound:          lookingGlassBound,
			normalize:      normalizer.apply,
		},
		normalizer: normalizer,
	}
	recording := Recording{Schema: Schema, Provenance: provenance}

	requests, err := recorder.tour(ctx, steps, &recording)
	if err != nil {
		return Recording{}, err
	}
	requests, err = recorder.crawler.followUps(ctx, requests, &recording.Exchanges)
	if err != nil {
		return Recording{}, err
	}
	recording.Assets, err = recorder.crawler.assets(ctx, recording.Exchanges)
	if err != nil {
		return Recording{}, err
	}
	if err := recorder.stream(ctx, &recording); err != nil {
		return Recording{}, err
	}

	if err := lookingGlass.wait(ctx, secondReadGap); err != nil {
		return Recording{}, fmt.Errorf("wait before the second read: %w", err)
	}
	varies, err := recorder.secondRead(ctx, requests, recording.Exchanges)
	if err != nil {
		return Recording{}, err
	}
	recording.Provenance.Normalized = recorder.normalizer.declared()
	recording.Provenance.Varies = varies
	return recording, nil
}

// tour asks every step and answers the requests it made, in order.
func (r *recorder) tour(ctx context.Context, steps []Step, recording *Recording) ([]request, error) {
	if len(steps) == 0 {
		return nil, errors.New("the tour is empty, so there is nothing to record")
	}
	var requests []request
	asked := map[string]bool{}
	for index, step := range steps {
		path, query, err := splitTarget(step.Target)
		if err != nil {
			return nil, fmt.Errorf("tour step %d: %w", index+1, err)
		}
		request := request{method: step.Method, path: path, query: query, form: step.Form, htmx: step.HTMX, via: viaTour}
		key := request.key()
		if asked[key] {
			return nil, fmt.Errorf("tour step %d asks %s, which an earlier step already asks", index+1, key)
		}
		asked[key] = true
		exchange, err := r.crawler.ask(ctx, request, true)
		if err != nil {
			return nil, fmt.Errorf("tour step %d: %w", index+1, err)
		}
		recording.Exchanges = append(recording.Exchanges, exchange)
		recording.Tour = append(recording.Tour, TourStep{Caption: step.Caption, Exchange: key})
		requests = append(requests, request)
	}
	recording.Entry = recording.Tour[0].Exchange
	return requests, nil
}

// followUps asks every request a recorded answer names through hx-get, and the
// requests those answers name in turn, until none is new, appending each
// answer to exchanges and its request to requests. The walk is bounded by
// exchangesMax.
func (c *crawler) followUps(ctx context.Context, requests []request, exchanges *[]Exchange) ([]request, error) {
	asked := map[string]bool{}
	for index := range *exchanges {
		asked[(*exchanges)[index].Key] = true
	}
	// *exchanges grows inside the loop: an answer appended here is read for
	// its own follow-up requests later in the same loop.
	for index := 0; index < len(*exchanges); index++ {
		parent := (*exchanges)[index]
		if !isMarkup(parent.ContentType) {
			continue
		}
		for _, target := range attributeValues(followUpAttribute, parent.Body) {
			path, query, err := splitTarget(target)
			if err != nil {
				return nil, fmt.Errorf("answer to %s: %w", parent.Key, err)
			}
			current := c.base + parent.Path
			if len(parent.Query) > 0 {
				current += "?" + encodeFields(parent.Query)
			}
			request := request{method: methodGet, path: path, query: query, htmx: true, via: viaLink, current: current}
			key := request.key()
			if asked[key] {
				continue
			}
			if c.follow != nil && !c.follow(parent, request) {
				continue
			}
			if len(*exchanges) >= c.exchangesMax {
				return nil, fmt.Errorf("the recording reached %d requests while following the answer to %s: %s",
					c.exchangesMax, parent.Key, c.bound)
			}
			asked[key] = true
			exchange, err := c.ask(ctx, request, true)
			if err != nil {
				return nil, fmt.Errorf("answer to %s names a request that failed: %w", parent.Key, err)
			}
			*exchanges = append(*exchanges, exchange)
			requests = append(requests, request)
		}
	}
	return requests, nil
}

// assets fetches every static file the recorded documents load, in the order
// the documents first name them.
func (c *crawler) assets(ctx context.Context, exchanges []Exchange) ([]Asset, error) {
	if c.assetAttribute == nil {
		return nil, nil
	}
	var assets []Asset
	fetched := map[string]bool{}
	for index := range exchanges {
		exchange := &exchanges[index]
		if !isMarkup(exchange.ContentType) {
			continue
		}
		for _, path := range attributeValues(c.assetAttribute, exchange.Body) {
			if fetched[path] {
				continue
			}
			fetched[path] = true
			answer, err := c.ask(ctx, request{method: methodGet, path: path}, false)
			if err != nil {
				return nil, fmt.Errorf("asset of %s: %w", exchange.Key, err)
			}
			assets = append(assets, Asset{Path: path, ContentType: answer.ContentType, Body: answer.Body})
		}
	}
	return assets, nil
}

// assetPattern answers the pattern that finds a src or href under one of
// prefixes, or nil for no prefix.
func assetPattern(prefixes []string) *regexp.Regexp {
	if len(prefixes) == 0 {
		return nil
	}
	quoted := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		quoted = append(quoted, regexp.QuoteMeta(prefix))
	}
	return regexp.MustCompile(`\b(?:src|href)="((?:` + strings.Join(quoted, "|") + `)[^"]+)"`)
}

// stream keeps the first messages of the event stream the recorded answers
// name. Answers that name no stream leave recording.Stream nil.
func (r *recorder) stream(ctx context.Context, recording *Recording) error {
	var path string
	for index := range recording.Exchanges {
		exchange := &recording.Exchanges[index]
		if !isMarkup(exchange.ContentType) {
			continue
		}
		named := attributeValues(streamAttribute, exchange.Body)
		if len(named) > 0 {
			path = named[0]
			break
		}
	}
	if path == "" {
		return nil
	}
	streamCtx, cancel := context.WithTimeout(ctx, streamWait)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(streamCtx, http.MethodGet, r.crawler.base+path, http.NoBody)
	if err != nil {
		return fmt.Errorf("event stream %s: %w", path, err)
	}
	httpRequest.Header.Set("Accept", streamContentType)
	// The stream has no end, so the client's whole-request timeout does not
	// apply: streamCtx bounds it.
	client := &http.Client{Transport: r.crawler.client.Transport}
	answer, err := client.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("event stream %s: %w", path, err)
	}
	defer answer.Body.Close() //nolint:errcheck // a read-only stream this run is done with
	if answer.StatusCode != http.StatusOK {
		return fmt.Errorf("event stream %s answered %s", path, answer.Status)
	}
	messages, err := readStreamMessages(answer.Body, streamMessages)
	if err != nil {
		return fmt.Errorf("event stream %s: %w", path, err)
	}
	for index := range messages {
		messages[index] = r.normalizer.apply(messages[index], true)
	}
	recording.Stream = &Stream{Path: path, ContentType: answer.Header.Get(headerContentType), Messages: messages}
	return nil
}

// readStreamMessages reads count server-sent messages: each is the lines up to
// a blank line. A stream that ends first is an error, because a recording with
// fewer messages than it declares differs from the next one.
func readStreamMessages(stream io.Reader, count int) ([]string, error) {
	lines := bufio.NewScanner(stream)
	lines.Buffer(make([]byte, 0, 64<<10), bodyBytesMax)
	var messages []string
	var message []string
	for len(messages) < count {
		if !lines.Scan() {
			if err := lines.Err(); err != nil {
				return nil, fmt.Errorf("after %d of %d messages: %w", len(messages), count, err)
			}
			return nil, fmt.Errorf("the stream ended after %d of %d messages", len(messages), count)
		}
		line := lines.Text()
		if line != "" {
			message = append(message, line)
			continue
		}
		if len(message) == 0 {
			continue
		}
		messages = append(messages, strings.Join(message, "\n"))
		message = nil
	}
	return messages, nil
}

// secondRead asks every request again and answers the keys whose answer
// differs from the recorded one, in the recording's order. requests and
// recorded are parallel: record appends to both together.
func (r *recorder) secondRead(ctx context.Context, requests []request, recorded []Exchange) ([]string, error) {
	if len(requests) != len(recorded) {
		panic("BUG: sitelgreplay.secondRead: the requests and their recorded answers are not parallel")
	}
	varies := []string{}
	for index, request := range requests {
		secondAnswer, err := r.crawler.ask(ctx, request, false)
		if err != nil {
			return nil, fmt.Errorf("second read: %w", err)
		}
		firstAnswer := recorded[index]
		if secondAnswer.Status != firstAnswer.Status {
			varies = append(varies, firstAnswer.Key)
			continue
		}
		if secondAnswer.ContentType != firstAnswer.ContentType {
			varies = append(varies, firstAnswer.Key)
			continue
		}
		if secondAnswer.Body != firstAnswer.Body {
			varies = append(varies, firstAnswer.Key)
		}
	}
	return varies, nil
}

// ask sends one request and answers its exchange, normalized. count says
// whether the replacements are counted: they are for an answer the recording
// keeps.
//
// Any status but 200 is refused. A recording is a demonstration of answers,
// and a server that refuses a question the tour or its own page asks is a
// defect to see, not a page to publish.
func (c *crawler) ask(ctx context.Context, request request, count bool) (Exchange, error) {
	key := request.key()
	target := c.base + request.path
	if len(request.query) > 0 {
		target += "?" + encodeFields(request.query)
	}
	var body io.Reader = http.NoBody
	if len(request.form) > 0 {
		body = strings.NewReader(encodeFields(request.form))
	}
	httpRequest, err := http.NewRequestWithContext(ctx, request.method, target, body)
	if err != nil {
		return Exchange{}, fmt.Errorf("%s: %w", key, err)
	}
	if len(request.form) > 0 {
		httpRequest.Header.Set(headerContentType, formContentType)
	}
	if request.htmx {
		httpRequest.Header.Set(headerHTMX, "true")
		httpRequest.Header.Set(headerHTMXType, htmxPartial)
		if request.current != "" {
			httpRequest.Header.Set(headerHTMXCurrent, request.current)
		}
	}
	answer, err := c.client.Do(httpRequest)
	if err != nil {
		return Exchange{}, fmt.Errorf("%s: %w", key, err)
	}
	defer answer.Body.Close() //nolint:errcheck // the body was read whole or the read's error is returned
	content, err := io.ReadAll(io.LimitReader(answer.Body, int64(c.bodyBytesMax)+1))
	if err != nil {
		return Exchange{}, fmt.Errorf("%s: read the answer: %w", key, err)
	}
	if len(content) > c.bodyBytesMax {
		return Exchange{}, fmt.Errorf("%s: the answer is larger than %d bytes", key, c.bodyBytesMax)
	}
	if answer.StatusCode != http.StatusOK {
		return Exchange{}, fmt.Errorf("%s: %s answered %s: %.300q", key, c.server, answer.Status, content)
	}
	if !utf8.Valid(content) {
		return Exchange{}, fmt.Errorf("%s: the answer (%s) is not text, and a recording holds text only",
			key, answer.Header.Get(headerContentType))
	}
	return Exchange{
		Key: key, Method: strings.ToUpper(request.method), Path: request.path, Query: request.query, Form: request.form,
		HTMX: request.htmx, Via: request.via, Status: answer.StatusCode,
		ContentType: answer.Header.Get(headerContentType),
		Body:        c.normalize(string(content), count),
	}, nil
}

// Request is one question a Crawler asks.
type Request struct {
	Method string
	Path   string
	// Query and Form are the fields in the order they are sent.
	Query []Field
	Form  []Field
	// HTMX says the request carries HX-Request and HX-Request-Type partial,
	// as htmx sends them for a swap into an element.
	HTMX bool
	// Current is the path and query of the page an htmx step leaves, sent
	// as HX-Current-URL on the crawled server, as htmx names it. Empty asks
	// the step as from nowhere. It is not part of the key.
	Current string
}

// Crawler records a server other than the looking glass: the walk the
// looking glass recorder makes, with the server, the asset prefixes and the
// bounds given rather than fixed. It asks every step, then every request the
// answers name through hx-get and the requests those answers name in turn,
// then the static files the answers load under AssetPrefixes. It changes no
// answer: a value that varies run to run is the caller's to fix at its source.
type Crawler struct {
	// Base is the server as a URL with no path.
	Base string
	// Client asks every request. A redirect is recorded as an answer only when
	// the client does not follow it.
	Client *http.Client
	// AssetPrefixes are the path prefixes of the server's static files. An
	// answer's src or href under one of them is fetched as an asset.
	AssetPrefixes []string
	// ExchangesMax bounds the requests one crawl holds, the steps included. A
	// crawl that reaches it is refused, never cut short.
	ExchangesMax int
	// BodyBytesMax bounds one answer.
	BodyBytesMax int
	// Follow, when set, decides whether a request an answer names through
	// hx-get is asked: parent is the answer that names it. A product whose
	// pages link every view of every table needs it, since following every
	// link would record every combination of them. Nil asks every one.
	Follow func(parent Exchange, request Request) bool
}

// Crawled is what one crawl recorded: the steps' answers first, in their
// order, then the answers they named, in the order they were found.
type Crawled struct {
	Exchanges []Exchange
	Assets    []Asset
}

// Crawl asks steps and what their answers name. Any answer but a 200 in text,
// a step that repeats an earlier one, and a walk past either bound is refused
// with an error that names the request.
func (c Crawler) Crawl(ctx context.Context, steps []Request) (Crawled, error) {
	walk, err := c.crawler()
	if err != nil {
		return Crawled{}, err
	}
	if len(steps) == 0 {
		return Crawled{}, errors.New("the crawl has no step, so there is nothing to record")
	}
	if len(steps) > c.ExchangesMax {
		return Crawled{}, fmt.Errorf("the crawl has %d steps, more than its bound of %d requests", len(steps), c.ExchangesMax)
	}
	var crawled Crawled
	requests := make([]request, 0, len(steps))
	asked := map[string]bool{}
	for index, step := range steps {
		request := request{
			method: strings.ToUpper(step.Method), path: step.Path, query: step.Query, form: step.Form,
			htmx: step.HTMX, via: viaTour,
		}
		if step.Current != "" {
			request.current = c.Base + step.Current
		}
		key := request.key()
		if asked[key] {
			return Crawled{}, fmt.Errorf("step %d asks %s, which an earlier step already asks", index+1, key)
		}
		asked[key] = true
		exchange, err := walk.ask(ctx, request, true)
		if err != nil {
			return Crawled{}, fmt.Errorf("step %d: %w", index+1, err)
		}
		crawled.Exchanges = append(crawled.Exchanges, exchange)
		requests = append(requests, request)
	}
	if _, err := walk.followUps(ctx, requests, &crawled.Exchanges); err != nil {
		return Crawled{}, err
	}
	crawled.Assets, err = walk.assets(ctx, crawled.Exchanges)
	if err != nil {
		return Crawled{}, err
	}
	return crawled, nil
}

// crawler answers the shared walk with c's settings, or why they cannot
// drive one.
func (c Crawler) crawler() (crawler, error) {
	if _, err := url.Parse(c.Base); err != nil {
		return crawler{}, fmt.Errorf("crawl address %q: %w", c.Base, err)
	}
	if c.Base == "" {
		return crawler{}, errors.New("the crawl names no server address")
	}
	if c.Client == nil {
		return crawler{}, errors.New("the crawl has no HTTP client")
	}
	if c.ExchangesMax < 1 {
		return crawler{}, fmt.Errorf("the crawl's exchange bound is %d, and it must allow one request", c.ExchangesMax)
	}
	if c.BodyBytesMax < 1 {
		return crawler{}, fmt.Errorf("the crawl's answer size bound is %d bytes, and it must allow one", c.BodyBytesMax)
	}
	for _, prefix := range c.AssetPrefixes {
		if !strings.HasPrefix(prefix, "/") {
			return crawler{}, fmt.Errorf("asset prefix %q does not start with /", prefix)
		}
	}
	walk := crawler{
		base: c.Base, client: c.Client, server: "the server",
		assetAttribute: assetPattern(c.AssetPrefixes),
		exchangesMax:   c.ExchangesMax, bodyBytesMax: c.BodyBytesMax,
		bound:     "raise the crawl's ExchangesMax, or narrow its steps",
		normalize: func(text string, _ bool) string { return text },
	}
	if c.Follow != nil {
		walk.follow = func(parent Exchange, asked request) bool {
			return c.Follow(parent, Request{
				Method: asked.method, Path: asked.path, Query: asked.query, Form: asked.form, HTMX: asked.htmx,
			})
		}
	}
	return walk, nil
}
