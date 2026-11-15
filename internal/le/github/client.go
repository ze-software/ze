// Design: docs/contributing/github-integration.md -- fixed-repository read-only API boundary.

package github

import (
	"context"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/opererr"
)

// Client owns a read-only HTTP client. Construct it with NewClient. Its zero
// value is uninitialized. It is safe for concurrent use when its transport is.
type Client struct {
	http   *http.Client
	states apiStates
}

const apiRoot = "https://api.github.com/repos/" + Repository + "/"

const apiSecureScheme = "https"
const apiPageSizeKey = "per_page"

const (
	operationTimeout  = 45 * time.Second
	pageBytesMax      = 4 << 20
	operationBytesMax = 32 << 20
	requestsMax       = 200
	pagesMax          = 10
	pageEntries       = 100
)

// apiStatesData is the complete external vocabulary, kept as wire data so its
// spellings remain exact rather than passing through prose correction.
//
//go:embed api-states.json
var apiStatesData []byte

type stateClass uint8

const (
	stateUnspecified stateClass = iota
	stateActive
	stateFinished
)

// apiStates is private, read-only after construction and shared by API reads.
// Its zero value is uninitialized; only NewClient supplies a populated value.
type apiStates struct {
	Status     map[string]stateClass `json:"status"`
	Conclusion []string              `json:"conclusion"`
	Deployment []string              `json:"deployment"`
}

var errInput = errors.New("invalid GitHub report selector")

// NewClient selects the caller's transport and fixes the timeout and redirect
// policy. The transport MUST be non-nil and safe for its callers' concurrency.
func NewClient(transport http.RoundTripper) (*Client, error) {
	if transport == nil {
		return nil, opererr.New("GitHub reporting has no HTTP transport",
			"No repository result can be read without its configured transport.", "./le help github report")
	}
	var states apiStates
	if err := json.Unmarshal(apiStatesData, &states); err != nil {
		return nil, opererr.Wrap(err, "GitHub reporting cannot read its API vocabulary",
			"Unknown API states could otherwise be mistaken for complete evidence.", "./le help github report")
	}
	if len(states.Status) == 0 {
		return nil, errors.New("GitHub status vocabulary is empty")
	}
	for _, class := range states.Status {
		switch class {
		case stateActive, stateFinished:
		case stateUnspecified:
			return nil, errors.New("GitHub status vocabulary has an unspecified class")
		default:
			return nil, errors.New("GitHub status vocabulary has an unknown class")
		}
	}
	if len(states.Conclusion) == 0 {
		return nil, errors.New("GitHub conclusion vocabulary is empty")
	}
	if len(states.Deployment) == 0 {
		return nil, errors.New("GitHub deployment vocabulary is empty")
	}
	return &Client{states: states, http: &http.Client{Transport: transport, Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// reach bounds one complete report, rather than resetting its budget per page.
// It is not safe for concurrent use; each public operation owns a fresh reach.
type reach struct {
	client   *Client
	ctx      context.Context
	requests int
	bytes    int64
}

func (client *Client) start(ctx context.Context) (*reach, context.CancelFunc, error) {
	if client == nil {
		return nil, nil, errors.New("GitHub client was not constructed")
	}
	if client.http == nil {
		return nil, nil, errors.New("GitHub client has no HTTP configuration")
	}
	bounded, cancel := context.WithTimeout(ctx, operationTimeout)
	return &reach{client: client, ctx: bounded}, cancel, nil
}

// get reads only a fixed-repository GET. Link targets are checked but never
// followed: collection pagination constructs its own bounded page numbers.
func (read *reach) get(path string, query url.Values, target any) error {
	if read.requests >= requestsMax {
		return errors.New("GitHub report exceeded its request bound")
	}
	read.requests++
	endpoint := apiRoot + path
	if len(query) != 0 {
		endpoint += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(read.ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "ze-development-evidence")
	response, err := read.client.http.Do(request)
	if err != nil {
		return fmt.Errorf("read GitHub API: %w", err)
	}
	defer response.Body.Close() //nolint:errcheck // read-only response; no buffered output remains
	if response.StatusCode != http.StatusOK {
		// API bodies may contain private data. Only the status and safe quota
		// headers are evidence about an unavailable public read.
		return fmt.Errorf("GitHub API answered HTTP %d (retry-after %q, rate-reset %q)", response.StatusCode,
			response.Header.Get("Retry-After"), response.Header.Get("X-RateLimit-Reset"))
	}
	if err := checkLinks(response.Header.Values("Link")); err != nil {
		return err
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, pageBytesMax+1))
	if err != nil {
		return fmt.Errorf("read GitHub response: %w", err)
	}
	if len(body) > pageBytesMax {
		return errors.New("GitHub API response exceeded the page byte bound")
	}
	read.bytes += int64(len(body))
	if read.bytes > operationBytesMax {
		return errors.New("GitHub report exceeded its total byte bound")
	}
	if !utf8.Valid(body) {
		return errors.New("GitHub API response is not valid UTF-8")
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}

func checkLinks(headers []string) error {
	for _, header := range headers {
		for part := range strings.SplitSeq(header, ",") {
			part = strings.TrimSpace(part)
			if !strings.HasPrefix(part, "<") {
				return errors.New("GitHub pagination header is malformed")
			}
			end := strings.IndexByte(part, '>')
			if end < 0 {
				return errors.New("GitHub pagination URL is unterminated")
			}
			parsed, err := url.Parse(part[1:end])
			if err != nil {
				return errors.New("GitHub pagination URL is malformed")
			}
			if parsed.Scheme != apiSecureScheme {
				return errors.New("GitHub pagination URL is not HTTPS")
			}
			if parsed.Host != "api.github.com" {
				return errors.New("GitHub pagination URL leaves the API origin")
			}
			if parsed.User != nil {
				return errors.New("GitHub pagination URL contains credentials")
			}
		}
	}
	return nil
}

func fullRevision(raw string) (string, error) {
	if len(raw) != 40 {
		return "", fmt.Errorf("%w: a full 40-character commit ID is required", errInput)
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("%w: commit ID contains a non-hexadecimal character", errInput)
	}
	return hex.EncodeToString(decoded), nil
}

func pageQuery(page int) url.Values {
	return url.Values{"page": {strconv.Itoa(page)}, apiPageSizeKey: {strconv.Itoa(pageEntries)}}
}
