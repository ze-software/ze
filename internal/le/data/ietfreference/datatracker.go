// Design: reference/README.md -- the non-normative IETF reference tree
// Related: refresh.go -- the refresh these reads feed
//
// datatracker.go holds every network read: the datatracker's REST API for
// groups, documents and obsoleting relations, and the two text archives.

package dataietfreference

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Sources names where each fetch goes. The command uses Canonical; a test
// points all three at one local server. Every field MUST be set.
type Sources struct {
	// Datatracker is the datatracker origin, with no trailing slash.
	Datatracker string
	// RFCEditor is the origin that serves /rfc/<stem>.txt.
	RFCEditor string
	// IDArchive is the origin that serves /archive/id/<stem>-<rev>.txt.
	IDArchive string
	// Client performs every request.
	Client *http.Client
}

// Canonical answers the public IETF origins, through a client whose deadline
// bounds each request.
func Canonical() Sources {
	return Sources{
		Datatracker: "https://datatracker.ietf.org",
		RFCEditor:   "https://www.rfc-editor.org",
		IDArchive:   "https://www.ietf.org",
		Client:      &http.Client{Timeout: 60 * time.Second},
	}
}

// Bounds on what the network can make this refresh hold or loop over.
const (
	// pageLimit is the page size asked of the datatracker.
	pageLimit = 200
	// pagesMax bounds a paged listing: 100 pages of 200 is far past any group.
	pagesMax = 100
	// bodyOctetsMax bounds one response, JSON page or document text.
	bodyOctetsMax = 32 << 20
)

// activeDraftState is the datatracker's "draft: active" state id.
const activeDraftState = "1"

// document is the subset of a datatracker document this refresh reads.
type document struct {
	Name             string   `json:"name"`
	StdLevel         *string  `json:"std_level"`
	IntendedStdLevel *string  `json:"intended_std_level"`
	Rev              string   `json:"rev"`
	States           []string `json:"states"`
	Title            string   `json:"title"`
}

// active answers whether the draft carries the "draft: active" state.
func (d *document) active() bool {
	for _, state := range d.States {
		if lastSegment(&state) == activeDraftState {
			return true
		}
	}
	return false
}

// group is the subset of a datatracker group this refresh reads.
type group struct {
	ID int `json:"id"`
}

// relation is the subset of a datatracker related-document row it reads.
type relation struct {
	Source string `json:"source"`
}

// groupID answers the datatracker id of one working group, and refuses an
// acronym that matches no group or more than one.
func (s Sources) groupID(ctx context.Context, wg string) (string, error) {
	query := url.Values{"acronym": {wg}}
	groups, err := listAll[group](ctx, s, "/api/v1/group/group/?"+encode(query, "acronym"))
	if err != nil {
		return "", err
	}
	if len(groups) != 1 {
		return "", fmt.Errorf("working group %q matches %d datatracker groups, want 1: is reference/ietf/%s named for an IETF acronym?", wg, len(groups), wg)
	}
	return strconv.Itoa(groups[0].ID), nil
}

// documents answers every document of one kind the group owns.
func (s Sources) documents(ctx context.Context, groupID, kind string) ([]document, error) {
	query := url.Values{"group": {groupID}, "type": {kind}, "limit": {strconv.Itoa(pageLimit)}}
	return listAll[document](ctx, s, "/api/v1/doc/document/?"+encode(query, "group", "type", "limit"))
}

// obsoletedBy answers the RFCs that obsolete rfc, sorted and comma-joined.
func (s Sources) obsoletedBy(ctx context.Context, rfc string) (string, error) {
	query := url.Values{"target__name": {rfc}, "relationship": {"obs"}}
	relations, err := listAll[relation](ctx, s, "/api/v1/doc/relateddocument/?"+encode(query, "target__name", "relationship"))
	if err != nil {
		return "", err
	}
	var names []string
	for i := range relations {
		name := lastSegment(&relations[i].Source)
		if name == "" {
			return "", fmt.Errorf("relation obsoleting %s names no source document", rfc)
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return strings.Join(slices.Compact(names), ","), nil
}

// text fetches one document's plain text.
func (s Sources) text(ctx context.Context, kind, stem, revision string) ([]byte, error) {
	target := s.RFCEditor + "/rfc/" + stem + ".txt"
	if kind == kindDraft {
		target = s.IDArchive + "/archive/id/" + stem + "-" + revision + ".txt"
	}
	body, err := s.get(ctx, target)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("fetch %s: empty body", target)
	}
	return body, nil
}

// encode renders query in the given key order, then format=json, so the URL
// a run requests is the same on every run.
func encode(query url.Values, keys ...string) string {
	var parts []string
	for _, key := range keys {
		parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(query.Get(key)))
	}
	return strings.Join(append(parts, "format=json"), "&")
}

// listing is one page of a datatracker list answer.
type listing[T any] struct {
	Meta struct {
		Next *string `json:"next"`
	} `json:"meta"`
	Objects []T `json:"objects"`
}

// listAll follows meta.next from path until the listing ends. A next link
// that leaves the datatracker origin is refused rather than followed.
func listAll[T any](ctx context.Context, s Sources, path string) ([]T, error) {
	var all []T
	for range pagesMax {
		body, err := s.get(ctx, s.Datatracker+path)
		if err != nil {
			return nil, err
		}
		var page listing[T]
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode %s%s: %w", s.Datatracker, path, err)
		}
		all = append(all, page.Objects...)
		if page.Meta.Next == nil {
			return all, nil
		}
		path = *page.Meta.Next
		if !strings.HasPrefix(path, "/") {
			return nil, fmt.Errorf("datatracker next link %q is not a path on %s", path, s.Datatracker)
		}
	}
	return nil, fmt.Errorf("datatracker listing %s ran past %d pages", path, pagesMax)
}

// get answers the body of one successful GET, bounded in size.
func (s Sources) get(ctx context.Context, target string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", target, err)
	}
	response, err := s.Client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", target, err)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, bodyOctetsMax+1))
	closeErr := response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP status %d", target, response.StatusCode)
	}
	if readErr != nil {
		return nil, fmt.Errorf("read %s: %w", target, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close %s: %w", target, closeErr)
	}
	if len(body) > bodyOctetsMax {
		return nil, fmt.Errorf("fetch %s: body exceeds %d octets", target, bodyOctetsMax)
	}
	return body, nil
}

// lastSegment answers the final path segment of a datatracker resource URI,
// "/api/v1/name/stdlevelname/bcp/" to "bcp". A nil URI, which the datatracker
// sends for an unset level, answers "".
func lastSegment(uri *string) string {
	if uri == nil {
		return ""
	}
	trimmed := strings.TrimSuffix(*uri, "/")
	return trimmed[strings.LastIndexByte(trimmed, '/')+1:]
}

// categoryNames maps each datatracker standard-level code to the name the
// index carries. It is the datatracker's whole stdlevelname set.
var categoryNames = map[string]string{
	"bcp":  "BCP",
	"ds":   "Draft Standard",
	"exp":  "Experimental",
	"hist": "Historic",
	"inf":  "Informational",
	"ps":   "Proposed Standard",
	"std":  "Internet Standard",
	"unkn": "Unknown",
	"":     "unset",
}

// categoryName answers the index name of a standard-level code, and refuses
// a code the datatracker did not define when this was written rather than
// guessing a name for it.
func categoryName(code string) (string, error) {
	name, ok := categoryNames[code]
	if !ok {
		return "", fmt.Errorf("unknown datatracker standard level %q: add it to categoryNames in internal/le/data/ietfreference", code)
	}
	return name, nil
}

// collapse turns every run of whitespace in a title into one space, so a
// title the datatracker wrapped over lines can never break a TSV row.
func collapse(title string) string {
	return strings.Join(strings.Fields(title), " ")
}
