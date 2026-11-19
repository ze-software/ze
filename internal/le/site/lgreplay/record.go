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

	headerHTMX        = "HX-Request"
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
}

func (r request) key() string {
	return ExchangeKey(r.method, r.path, r.query, r.form, r.htmx)
}

// recorder is one run. It is not safe for concurrent use.
type recorder struct {
	lookingGlass lookingGlass
	client       *http.Client
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
	recorder := recorder{
		lookingGlass: lookingGlass,
		// A redirect is an answer like any other: it is recorded, not followed.
		client: &http.Client{
			Timeout:       requestWait,
			Transport:     &http.Transport{Proxy: nil},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		normalizer: newNormalizer(listener.Host),
	}
	recording := Recording{Schema: Schema, Provenance: provenance}

	requests, err := recorder.tour(ctx, steps, &recording)
	if err != nil {
		return Recording{}, err
	}
	requests, err = recorder.followUps(ctx, requests, &recording)
	if err != nil {
		return Recording{}, err
	}
	if err := recorder.assets(ctx, &recording); err != nil {
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
		exchange, err := r.ask(ctx, request, true)
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
// requests those answers name in turn, until none is new. The walk is bounded
// by exchangesMax.
func (r *recorder) followUps(ctx context.Context, requests []request, recording *Recording) ([]request, error) {
	asked := map[string]bool{}
	for index := range recording.Exchanges {
		asked[recording.Exchanges[index].Key] = true
	}
	// recording.Exchanges grows inside the loop: an answer appended here is
	// read for its own follow-up requests later in the same loop.
	for index := 0; index < len(recording.Exchanges); index++ {
		parent := recording.Exchanges[index]
		if !isMarkup(parent.ContentType) {
			continue
		}
		for _, target := range attributeValues(followUpAttribute, parent.Body) {
			path, query, err := splitTarget(target)
			if err != nil {
				return nil, fmt.Errorf("answer to %s: %w", parent.Key, err)
			}
			request := request{method: methodGet, path: path, query: query, htmx: true, via: viaLink}
			key := request.key()
			if asked[key] {
				continue
			}
			if len(recording.Exchanges) == exchangesMax {
				return nil, fmt.Errorf("the recording reached %d requests while following the answer to %s: "+
					"the simulated network grew, so raise exchangesMax in internal/le/site/lgreplay/record.go",
					exchangesMax, parent.Key)
			}
			asked[key] = true
			exchange, err := r.ask(ctx, request, true)
			if err != nil {
				return nil, fmt.Errorf("answer to %s names a request that failed: %w", parent.Key, err)
			}
			recording.Exchanges = append(recording.Exchanges, exchange)
			requests = append(requests, request)
		}
	}
	return requests, nil
}

// assets fetches every static file the recorded documents load.
func (r *recorder) assets(ctx context.Context, recording *Recording) error {
	fetched := map[string]bool{}
	for index := range recording.Exchanges {
		exchange := &recording.Exchanges[index]
		if !isMarkup(exchange.ContentType) {
			continue
		}
		for _, path := range attributeValues(assetAttribute, exchange.Body) {
			if fetched[path] {
				continue
			}
			fetched[path] = true
			answer, err := r.ask(ctx, request{method: methodGet, path: path}, false)
			if err != nil {
				return fmt.Errorf("asset of %s: %w", exchange.Key, err)
			}
			recording.Assets = append(recording.Assets, Asset{Path: path, ContentType: answer.ContentType, Body: answer.Body})
		}
	}
	return nil
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
	httpRequest, err := http.NewRequestWithContext(streamCtx, http.MethodGet, r.lookingGlass.base+path, http.NoBody)
	if err != nil {
		return fmt.Errorf("event stream %s: %w", path, err)
	}
	httpRequest.Header.Set("Accept", streamContentType)
	// The stream has no end, so the client's whole-request timeout does not
	// apply: streamCtx bounds it.
	client := &http.Client{Transport: r.client.Transport}
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
		secondAnswer, err := r.ask(ctx, request, false)
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
// and a looking glass that refuses a question the tour or its own page asks
// is a defect to see, not a page to publish.
func (r *recorder) ask(ctx context.Context, request request, count bool) (Exchange, error) {
	key := request.key()
	target := r.lookingGlass.base + request.path
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
	}
	answer, err := r.client.Do(httpRequest)
	if err != nil {
		return Exchange{}, fmt.Errorf("%s: %w", key, err)
	}
	defer answer.Body.Close() //nolint:errcheck // the body was read whole or the read's error is returned
	content, err := io.ReadAll(io.LimitReader(answer.Body, bodyBytesMax+1))
	if err != nil {
		return Exchange{}, fmt.Errorf("%s: read the answer: %w", key, err)
	}
	if len(content) > bodyBytesMax {
		return Exchange{}, fmt.Errorf("%s: the answer is larger than %d bytes", key, bodyBytesMax)
	}
	if answer.StatusCode != http.StatusOK {
		return Exchange{}, fmt.Errorf("%s: the looking glass answered %s: %.300q", key, answer.Status, content)
	}
	if !utf8.Valid(content) {
		return Exchange{}, fmt.Errorf("%s: the answer (%s) is not text, and a recording holds text only",
			key, answer.Header.Get(headerContentType))
	}
	return Exchange{
		Key: key, Method: strings.ToUpper(request.method), Path: request.path, Query: request.query, Form: request.form,
		HTMX: request.htmx, Via: request.via, Status: answer.StatusCode,
		ContentType: answer.Header.Get(headerContentType),
		Body:        r.normalizer.apply(string(content), count),
	}, nil
}
