// Design: docs/architecture/site-facts.md -- release data is a network fact, committed by a command
// Related: actions.go -- `le site release update`, the one writer of the file
//
// release.go is the model of website/data/github-releases.json and the one
// reach that fills it. The site build reads the file through Read and never
// reaches the forge for release data, so a build stays offline for it and a
// change to the published release is one command and one commit.

package siterelease

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// DataFile is the committed file's name under website/data/, and Path its
// place in the checkout.
const (
	DataFile = "github-releases.json"
	Path     = "website/data/" + DataFile
)

// Endpoint is the forge's releases LIST. The list answers an empty array while
// the project has no release, where /releases/latest answers 404, so "none yet"
// is an answer this command can commit rather than an error (A-2 of
// spec-site-values-from-one-file, observed on 2026-10-30).
const Endpoint = "https://api.github.com/repos/ze-software/ze/releases"

// The bounds of the reach (docs/contributing/ze-go-style.md, "A limit on
// everything"). One context covers every page and every body read. A page
// holds at most pageEntries releases; pagesMax pages is a thousand releases,
// and a forge that states more is refused rather than cut short in silence.
const (
	fetchTimeout = 5 * time.Second
	pageEntries  = 100
	pagesMax     = 10
	pageBytesMax = 8 << 20
)

// File is website/data/github-releases.json: the endpoint read and the
// published releases it stated, newest first.
//
// Fields are declared in the alphabetical order of their JSON keys, so the
// committed file is key-sorted and a refresh that changes nothing changes no
// byte. Key names are the forge's own, so a reader can trace a field to the
// API answer by its name alone.
type File struct {
	Endpoint string    `json:"endpoint"`
	Releases []Release `json:"releases"`
}

// Release is one published release.
type Release struct {
	Assets  []Asset `json:"assets"`
	HTMLURL string  `json:"html_url"`
	// Name is null when the release was published without a title: the forge
	// states none, and the tag is not a title.
	Name        *string `json:"name"`
	Prerelease  bool    `json:"prerelease"`
	PublishedAt string  `json:"published_at"`
	TagName     string  `json:"tag_name"`
}

// Asset is one file a release carries.
type Asset struct {
	BrowserDownloadURL string `json:"browser_download_url"`
	// Digest is "sha256:<hex>", or null for an asset the forge computed no
	// digest for.
	Digest *string `json:"digest"`
	Name   string  `json:"name"`
	Size   int64   `json:"size"`
}

// Latest answers the newest published release, and false when the file
// states none. A prerelease counts: the project publishes what it has, and
// the site shows the newest of it.
func (file *File) Latest() (Release, bool) {
	if len(file.Releases) == 0 {
		return Release{}, false
	}
	return file.Releases[0], true
}

// SHA256 answers the asset's digest without its algorithm prefix, and false
// when the forge stated none.
func (asset *Asset) SHA256() (string, bool) {
	if asset.Digest == nil {
		return "", false
	}
	return strings.TrimPrefix(*asset.Digest, digestPrefix), true
}

// Read answers the committed file under one website source directory, checked
// the way the update checked it before writing: a file edited by hand into a
// state the forge never answers is refused here too (the pair of checks,
// docs/contributing/ze-go-style.md, "Pair the check").
func Read(website string) (File, error) {
	path := filepath.Join(website, "data", DataFile)
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return File{}, fmt.Errorf("read %s: %w; run `./le site release update` and commit the file it writes", Path, err)
	}
	var file File
	if err := json.Unmarshal(content, &file); err != nil {
		return File{}, fmt.Errorf("read %s: %w; run `./le site release update` to rewrite it", Path, err)
	}
	if file.Endpoint != Endpoint {
		return File{}, fmt.Errorf("%s states endpoint %q, not %s; run `./le site release update` to rewrite it", Path, file.Endpoint, Endpoint)
	}
	if file.Releases == nil {
		return File{}, fmt.Errorf("%s states no releases list; run `./le site release update` to rewrite it", Path)
	}
	for index := range file.Releases {
		if err := file.Releases[index].check(); err != nil {
			return File{}, fmt.Errorf("%s: %w; run `./le site release update` to rewrite it", Path, err)
		}
	}
	return file, nil
}

// render answers the committed form of the file, which two runs over one
// forge answer write byte for byte.
func render(file *File) ([]byte, error) {
	content, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", Path, err)
	}
	return append(content, '\n'), nil
}

// apiRelease is one entry of the forge's answer, as the forge states it.
// Pointers mark the fields whose absence is an answer the record must keep
// apart from a zero.
type apiRelease struct {
	Assets      []apiAsset `json:"assets"`
	Draft       bool       `json:"draft"`
	HTMLURL     string     `json:"html_url"`
	Name        *string    `json:"name"`
	Prerelease  bool       `json:"prerelease"`
	PublishedAt *string    `json:"published_at"`
	TagName     string     `json:"tag_name"`
}

type apiAsset struct {
	BrowserDownloadURL string  `json:"browser_download_url"`
	Digest             *string `json:"digest"`
	Name               string  `json:"name"`
	Size               *int64  `json:"size"`
}

// fetchPage answers the body of one page of the releases list.
//
// It is a variable so a test states the forge's answer or its failure, and
// no test reaches the network.
var fetchPage = fetchReleasesPage

// fetch answers every published release the forge states, newest first.
func fetch() (File, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	file := File{Endpoint: Endpoint, Releases: []Release{}}
	for page := 1; page <= pagesMax; page++ {
		pageURL := Endpoint + "?per_page=" + strconv.Itoa(pageEntries) + "&page=" + strconv.Itoa(page)
		body, err := fetchPage(ctx, pageURL)
		if err != nil {
			return File{}, err
		}
		var entries []apiRelease
		if err := json.Unmarshal(body, &entries); err != nil {
			return File{}, fmt.Errorf("read what %s answered: %w", pageURL, err)
		}
		for index := range entries {
			// A draft is not published: the forge shows it to the project's
			// owners only, and no reader can download it.
			if entries[index].Draft {
				continue
			}
			published, err := entries[index].record()
			if err != nil {
				return File{}, fmt.Errorf("%s: %w", pageURL, err)
			}
			file.Releases = append(file.Releases, published)
		}
		if len(entries) < pageEntries {
			sortReleases(file.Releases)
			return file, nil
		}
	}
	return File{}, fmt.Errorf("%s states more than %d releases, the most this command reads", Endpoint, pagesMax*pageEntries)
}

// sortReleases orders the releases newest first, and two releases published
// in the same second by tag, so one forge answer writes one file.
func sortReleases(releases []Release) {
	slices.SortFunc(releases, func(left, right Release) int {
		if order := strings.Compare(right.PublishedAt, left.PublishedAt); order != 0 {
			return order
		}
		return strings.Compare(left.TagName, right.TagName)
	})
}

// record answers the committed form of one release the forge stated, or why
// it cannot be committed.
func (entry *apiRelease) record() (Release, error) {
	if entry.PublishedAt == nil {
		return Release{}, fmt.Errorf("release %q is not a draft and states no published_at", entry.TagName)
	}
	published := Release{
		Assets:      make([]Asset, 0, len(entry.Assets)),
		HTMLURL:     entry.HTMLURL,
		Name:        entry.Name,
		Prerelease:  entry.Prerelease,
		PublishedAt: *entry.PublishedAt,
		TagName:     entry.TagName,
	}
	for index := range entry.Assets {
		asset := &entry.Assets[index]
		if asset.Size == nil {
			return Release{}, fmt.Errorf("release %q asset %q states no size", entry.TagName, asset.Name)
		}
		published.Assets = append(published.Assets, Asset{
			BrowserDownloadURL: asset.BrowserDownloadURL,
			Digest:             asset.Digest,
			Name:               asset.Name,
			Size:               *asset.Size,
		})
	}
	slices.SortFunc(published.Assets, func(left, right Asset) int { return strings.Compare(left.Name, right.Name) })
	if err := published.check(); err != nil {
		return Release{}, err
	}
	return published, nil
}

// The hosts a committed URL may name. The release page is on github.com, and
// a download URL is github.com's release download path or the object store
// the forge serves assets from. Anything else is not what the forge answers,
// and a page must never link a reader to it.
var (
	pageHosts     = []string{"github.com"}
	downloadHosts = []string{"github.com", "objects.githubusercontent.com"}
)

// digestPrefix is the algorithm the forge states asset digests in.
const digestPrefix = "sha256:"

// check answers why one release cannot be published, or nil.
func (published *Release) check() error {
	if published.TagName == "" {
		return errors.New("a release states no tag_name")
	}
	if _, err := time.Parse(time.RFC3339, published.PublishedAt); err != nil {
		return fmt.Errorf("release %q states published_at %q, which is not an RFC 3339 time", published.TagName, published.PublishedAt)
	}
	if err := checkURL(published.HTMLURL, pageHosts); err != nil {
		return fmt.Errorf("release %q html_url: %w", published.TagName, err)
	}
	for index := range published.Assets {
		if err := published.Assets[index].check(); err != nil {
			return fmt.Errorf("release %q asset %q: %w", published.TagName, published.Assets[index].Name, err)
		}
	}
	return nil
}

// check answers why one asset cannot be published, or nil.
func (asset *Asset) check() error {
	if asset.Name == "" {
		return errors.New("the asset states no name")
	}
	// Zero is a size an empty file has; below zero is no size at all.
	if asset.Size < 0 {
		return fmt.Errorf("size %d is negative", asset.Size)
	}
	if err := checkURL(asset.BrowserDownloadURL, downloadHosts); err != nil {
		return fmt.Errorf("browser_download_url: %w", err)
	}
	if asset.Digest == nil {
		return nil
	}
	sum, isSHA256 := strings.CutPrefix(*asset.Digest, digestPrefix)
	if !isSHA256 {
		return fmt.Errorf("digest %q is not a %s digest", *asset.Digest, strings.TrimSuffix(digestPrefix, ":"))
	}
	decoded, err := hex.DecodeString(sum)
	if err != nil {
		return fmt.Errorf("digest %q is not hexadecimal", *asset.Digest)
	}
	if len(decoded) != 32 {
		return fmt.Errorf("digest %q is not 32 bytes", *asset.Digest)
	}
	return nil
}

// checkURL answers why one URL is not an https URL on one of the hosts, or nil.
func checkURL(raw string, hosts []string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%q is not a URL: %w", raw, err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("%q is not an https URL", raw)
	}
	if parsed.User != nil {
		return fmt.Errorf("%q carries credentials", raw)
	}
	if !slices.Contains(hosts, parsed.Host) {
		return fmt.Errorf("%q is not on %s", raw, strings.Join(hosts, " or "))
	}
	return nil
}

// fetchReleasesPage reads one page of the releases list from the forge.
func fetchReleasesPage(ctx context.Context, pageURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ze-site-release")
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close() //nolint:errcheck // the body is read below and a close error cannot change the answer
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", pageURL, response.Status)
	}
	// One byte past the bound tells a body at the bound from a longer one,
	// which is refused rather than parsed short.
	body, err := io.ReadAll(io.LimitReader(response.Body, pageBytesMax+1))
	if err != nil {
		return nil, fmt.Errorf("read what %s answered: %w", pageURL, err)
	}
	if len(body) > pageBytesMax {
		return nil, fmt.Errorf("%s answered more than %d bytes, the most this command reads", pageURL, pageBytesMax)
	}
	return body, nil
}
