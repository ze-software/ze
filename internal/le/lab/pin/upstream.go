// Design: docs/architecture/testing/interop.md -- lab peer pins and their upstream
// Overview: scan.go -- the pins these answers are compared with.
//
// upstream.go asks each pin's upstream for its latest stable release: an OCI
// registry's tag list (Docker Hub, ghcr.io, quay.io), proxy.golang.org, or the
// GitHub API. Every request goes through one fetcher, so a test answers them
// from recorded responses and never reaches the network.

package pin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// responseBytesMax bounds one upstream answer. The largest is a popular
// image's tag list page.
const responseBytesMax = 8 << 20

// tagPagesMax bounds how many pages of one registry tag list are read.
const tagPagesMax = 40

// requestTimeout bounds one HTTP request.
const requestTimeout = 30 * time.Second

// response is one upstream answer.
type response struct {
	Status int
	Header http.Header
	Body   []byte
}

// fetcher is the network boundary.
type fetcher interface {
	Get(ctx context.Context, target string, header http.Header) (response, error)
}

// httpFetcher is the real network.
type httpFetcher struct{}

// Get issues one bounded GET.
func (httpFetcher) Get(ctx context.Context, target string, header http.Header) (response, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return response{}, err
	}
	for name, values := range header {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}
	request.Header.Set("User-Agent", "ze-le-lab-pin")
	answer, err := http.DefaultClient.Do(request)
	if err != nil {
		return response{}, err
	}
	defer answer.Body.Close() //nolint:errcheck // a read-only body; the read below reports a failure
	body, err := io.ReadAll(io.LimitReader(answer.Body, responseBytesMax))
	if err != nil {
		return response{}, err
	}
	return response{Status: answer.StatusCode, Header: answer.Header, Body: body}, nil
}

// upstream asks and remembers: one question per upstream name per run.
// Not safe for concurrent use.
type upstream struct {
	fetch  fetcher
	tags   map[string][]string
	failed map[string]error
}

func newUpstream(fetch fetcher) *upstream {
	return &upstream{fetch: fetch, tags: map[string][]string{}, failed: map[string]error{}}
}

// get fetches target and refuses a status other than 200.
func (u *upstream) get(ctx context.Context, target string, header http.Header) (response, error) {
	answer, err := u.fetch.Get(ctx, target, header)
	if err != nil {
		return answer, errors.New("GET " + target + ": " + err.Error())
	}
	if answer.Status != http.StatusOK {
		return answer, errors.New("GET " + target + " answered HTTP " + strconv.Itoa(answer.Status))
	}
	return answer, nil
}

// registryTags answers every tag of an image repository, through the OCI
// distribution API with the anonymous bearer token the registry offers.
func (u *upstream) registryTags(ctx context.Context, image string) ([]string, error) {
	host, repository := splitImage(image)
	target := "https://" + host + "/v2/" + repository + "/tags/list?n=1000"
	var header http.Header
	var tags []string
	for range tagPagesMax {
		answer, err := u.fetch.Get(ctx, target, header)
		if err != nil {
			return nil, errors.New("GET " + target + ": " + err.Error())
		}
		if answer.Status == http.StatusUnauthorized && header == nil {
			token, err := u.registryToken(ctx, answer.Header.Get("Www-Authenticate"))
			if err != nil {
				return nil, err
			}
			header = http.Header{"Authorization": []string{"Bearer " + token}}
			continue
		}
		if answer.Status != http.StatusOK {
			return nil, errors.New("GET " + target + " answered HTTP " + strconv.Itoa(answer.Status))
		}
		var page struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(answer.Body, &page); err != nil {
			return nil, errors.New(image + ": the tag list is not JSON: " + err.Error())
		}
		tags = append(tags, page.Tags...)
		next := nextLink(answer.Header.Get("Link"))
		if next == "" {
			return tags, nil
		}
		target = "https://" + host + next
	}
	return tags, nil
}

// bearerParameter reads one key="value" of a WWW-Authenticate challenge.
var bearerParameter = regexp.MustCompile(`([a-z]+)="([^"]*)"`)

// registryToken answers the anonymous pull token a Bearer challenge names.
func (u *upstream) registryToken(ctx context.Context, challenge string) (string, error) {
	values := url.Values{}
	realm := ""
	for _, match := range bearerParameter.FindAllStringSubmatch(challenge, -1) {
		if match[1] == "realm" {
			realm = match[2]
			continue
		}
		values.Set(match[1], match[2])
	}
	if realm == "" {
		return "", errors.New("the registry asked for credentials without a Bearer realm: " + challenge)
	}
	answer, err := u.get(ctx, realm+"?"+values.Encode(), nil)
	if err != nil {
		return "", err
	}
	var token struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(answer.Body, &token); err != nil {
		return "", errors.New("the registry token answer is not JSON: " + err.Error())
	}
	if token.Token != "" {
		return token.Token, nil
	}
	if token.AccessToken != "" {
		return token.AccessToken, nil
	}
	return "", errors.New("the registry token answer holds no token")
}

// nextLink reads the path of a `Link: <path>; rel="next"` header.
func nextLink(link string) string {
	start := strings.Index(link, "<")
	end := strings.Index(link, ">")
	if start < 0 || end <= start || !strings.Contains(link, `rel="next"`) {
		return ""
	}
	return link[start+1 : end]
}

// splitImage answers the registry host and repository path of an image name,
// with Docker Hub's defaults applied.
func splitImage(image string) (string, string) {
	first, rest, found := strings.Cut(image, "/")
	if found && strings.ContainsAny(first, ".:") {
		if first == "docker.io" {
			return dockerHubRegistry, libraryPath(rest)
		}
		return first, rest
	}
	return dockerHubRegistry, libraryPath(image)
}

// dockerHubRegistry is where a docker.io image's API answers.
const dockerHubRegistry = "registry-1.docker.io"

// libraryPath prefixes an official Docker Hub image with library/.
func libraryPath(repository string) string {
	if strings.Contains(repository, "/") {
		return repository
	}
	return "library/" + repository
}

// goLatest answers the latest version of the module holding package path,
// and the newest version of a higher major version line where one exists:
// `@latest` of a v3 path never names v4.
func (u *upstream) goLatest(ctx context.Context, path, pinned string) (string, error) {
	module, latest, err := u.goModule(ctx, path)
	if err != nil {
		return "", err
	}
	major := parseVersion(pinned).numbers
	if len(major) == 0 {
		return latest, nil
	}
	base := module
	if index := strings.LastIndex(module, "/v"); index > 0 {
		if _, err := strconv.Atoi(module[index+2:]); err == nil {
			base = module[:index]
		}
	}
	// A major line has at most a handful of successors; eight bounds the walk.
	for next := major[0] + 1; next < major[0]+8; next++ {
		found, known := u.proxyLatest(ctx, base+"/v"+strconv.Itoa(next))
		if !known {
			break
		}
		latest = found
	}
	return latest, nil
}

// goModule finds the module of a package path by asking the proxy for each
// prefix, longest first.
func (u *upstream) goModule(ctx context.Context, path string) (string, string, error) {
	candidate := path
	for strings.Count(candidate, "/") >= 2 {
		if latest, known := u.proxyLatest(ctx, candidate); known {
			return candidate, latest, nil
		}
		candidate = candidate[:strings.LastIndex(candidate, "/")]
	}
	return "", "", errors.New("proxy.golang.org knows no module holding " + path)
}

// proxyLatest answers the proxy's latest version of module, and false when
// the proxy knows no such module: a missing path is the expected answer for
// every prefix of a package path that is not its module, and for every major
// version past the newest.
func (u *upstream) proxyLatest(ctx context.Context, module string) (string, bool) {
	answer, err := u.fetch.Get(ctx, "https://proxy.golang.org/"+proxyEscape(module)+"/@latest", nil)
	if err != nil {
		return "", false
	}
	if answer.Status != http.StatusOK {
		return "", false
	}
	var info struct {
		Version string `json:"Version"`
	}
	if json.Unmarshal(answer.Body, &info) != nil {
		return "", false
	}
	return info.Version, info.Version != ""
}

// proxyEscape applies the module proxy's case encoding: an upper-case letter
// becomes '!' and its lower case.
func proxyEscape(path string) string {
	var tb textbuf.Buffer
	for _, character := range path {
		if character >= 'A' && character <= 'Z' {
			tb.Byte('!').Byte(byte(character - 'A' + 'a'))
			continue
		}
		tb.Str(string(character))
	}
	return tb.String()
}

// githubRelease answers a repository's latest release tag.
func (u *upstream) githubRelease(ctx context.Context, repository string) (string, error) {
	answer, err := u.get(ctx, "https://api.github.com/repos/"+repository+"/releases/latest", githubHeader)
	if err != nil {
		return "", err
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(answer.Body, &release); err != nil {
		return "", errors.New(repository + ": the release answer is not JSON: " + err.Error())
	}
	return release.Tag, nil
}

// githubHeader asks the GitHub API for its stable JSON form.
var githubHeader = http.Header{"Accept": []string{"application/vnd.github+json"}}

// commit is one upstream commit and its committer date, RFC 3339.
type commit struct {
	Commit string
	Date   string
}

// latestCommit answers the newest commit of a repository's default branch.
func (u *upstream) latestCommit(ctx context.Context, repository string) (commit, error) {
	answer, err := u.get(ctx, "https://api.github.com/repos/"+repository+"/commits?per_page=1", githubHeader)
	if err != nil {
		return commit{}, err
	}
	var commits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Committer struct {
				Date string `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(answer.Body, &commits); err != nil {
		return commit{}, errors.New(repository + ": the commit answer is not JSON: " + err.Error())
	}
	if len(commits) == 0 {
		return commit{}, errors.New(repository + ": GitHub answered no commit on the default branch")
	}
	return commit{Commit: commits[0].SHA, Date: commits[0].Commit.Committer.Date}, nil
}

// version is a tag split into its numeric run and what surrounds it.
type version struct {
	prefix  string
	numbers []int
	suffix  string
}

// versionShape reads `v1.2.3-alpine` as prefix v, numbers 1 2 3, suffix -alpine.
var versionShape = regexp.MustCompile(`^([A-Za-z]*?)(\d+(?:\.\d+)*)(.*)$`)

func parseVersion(tag string) version {
	match := versionShape.FindStringSubmatch(tag)
	if match == nil {
		return version{}
	}
	parts := strings.Split(match[2], ".")
	numbers := make([]int, 0, len(parts))
	for _, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil {
			return version{}
		}
		numbers = append(numbers, number)
	}
	return version{prefix: match[1], numbers: numbers, suffix: match[3]}
}

// unstableMarker finds a pre-release or build-of-the-day tag.
var unstableMarker = regexp.MustCompile(`(?i)(rc|alpha|beta|dev|pre|snapshot|nightly|edge|test|unstable)`)

// stable reports whether a tag names a release.
func stable(tag string) bool { return !unstableMarker.MatchString(tag) }

// sameShape reports whether candidate can replace pinned: the same prefix,
// the same count of numbers and the same suffix, so golang:1.25-alpine is
// compared with golang:1.27-alpine and never with golang:1.27.1-trixie.
func sameShape(pinned, candidate version) bool {
	return pinned.prefix == candidate.prefix && len(pinned.numbers) == len(candidate.numbers) && pinned.suffix == candidate.suffix
}

// newer reports whether a is a higher version than b.
func newer(a, b []int) bool {
	for index := range min(len(a), len(b)) {
		if a[index] != b[index] {
			return a[index] > b[index]
		}
	}
	return len(a) > len(b)
}

// latestTag answers the highest stable tag of tags in pinned's shape, and for
// a pin that is not a version (latest, alpine) the highest plain release.
func latestTag(pinned string, tags []string) string {
	want := parseVersion(pinned)
	floating := len(want.numbers) == 0
	best := ""
	var bestNumbers []int
	for _, tag := range tags {
		if !stable(tag) {
			continue
		}
		candidate := parseVersion(tag)
		if len(candidate.numbers) == 0 {
			continue
		}
		if floating && (candidate.suffix != "" || len(candidate.numbers) < 2) {
			continue
		}
		if !floating && !sameShape(want, candidate) {
			continue
		}
		if best == "" || newer(candidate.numbers, bestNumbers) {
			best, bestNumbers = tag, candidate.numbers
		}
	}
	return best
}
