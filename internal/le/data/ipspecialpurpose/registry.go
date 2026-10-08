// Design: docs/architecture/wire/attributes.md -- deliberate canonical IANA refresh.
// Package dataipspecialpurpose owns fetching and writing, not classification.
// The same core parser validates both snapshots before either file is changed.
package dataipspecialpurpose

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/core/ipregistry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Fetch supplies one fixed canonical source. Tests can supply offline bodies;
// the command exposes no URL override. Write MUST close every returned body.
type Fetch func(context.Context, string) (io.ReadCloser, error)

type source struct {
	name string
	bits int
}

var sources = [...]source{
	{name: "iana-ipv4-special-registry", bits: 32},
	{name: "iana-ipv6-special-registry", bits: 128},
}

func (s source) url() string {
	return "https://www.iana.org/assignments/" + s.name + "/" + s.name + ".xml"
}

func (s source) file() string {
	return "internal/core/ipregistry/" + s.name + ".xml"
}

// WriteReport identifies exactly which validated snapshots were written.
type WriteReport struct {
	Files []SnapshotReport `json:"files"`
}

// SnapshotReport carries source provenance from the canonical XML document.
type SnapshotReport struct {
	File    string `json:"file"`
	Source  string `json:"source"`
	Updated string `json:"updated"`
	Entries int    `json:"entries"`
}

// Text renders the same provenance that the structured report carries.
func (r WriteReport) Text() string {
	var text textbuf.Buffer
	for _, row := range r.Files {
		text.Str("wrote ").Str(row.File).Str(" (").Int(int64(row.Entries)).
			Str(" prefixes, IANA updated ").Str(row.Updated).Str(")\n")
	}
	return text.String()
}

// Write refreshes the two fixed IANA XML sources. A nil fetch uses HTTPS with a
// bounded deadline and response size. Every fetch, read, close and Parse must
// succeed for both documents before either existing snapshot is overwritten.
// The two filesystem writes are not an atomic transaction; filesystem errors
// are returned, never hidden. This action is never called by the runtime.
func Write(ctx context.Context, root string, fetch Fetch) (WriteReport, error) {
	if fetch == nil {
		fetch = fetchHTTP
	}
	var bodies [len(sources)][]byte
	report := WriteReport{Files: make([]SnapshotReport, 0, len(sources))}
	for i, source := range sources {
		body, err := readSource(ctx, source.url(), fetch)
		if err != nil {
			return WriteReport{}, err
		}
		registry, err := ipregistry.Parse(body, source.bits)
		if err != nil {
			return WriteReport{}, fmt.Errorf("validate %s: %w", source.url(), err)
		}
		bodies[i] = body
		report.Files = append(report.Files, SnapshotReport{
			File: source.file(), Source: source.url(),
			Updated: registry.Updated(), Entries: registry.Len(),
		})
	}
	for i, source := range sources {
		path := filepath.Join(root, filepath.FromSlash(source.file()))
		if err := os.WriteFile(path, bodies[i], 0o644); err != nil { //nolint:gosec // canonical source data intentionally readable by the build
			return WriteReport{}, fmt.Errorf("write IANA snapshot %s: %w", path, err)
		}
	}
	return report, nil
}

// readSource MUST close the fetcher's body even after a read failure. The size
// limit is shared with Parse, so oversized upstream XML never enters a file.
func readSource(ctx context.Context, url string, fetch Fetch) ([]byte, error) {
	response, err := fetch(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetch IANA source %s: %w", url, err)
	}
	body, readErr := io.ReadAll(io.LimitReader(response, ipregistry.SizeMax+1))
	closeErr := response.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read IANA source %s: %w", url, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close IANA source %s: %w", url, closeErr)
	}
	if len(body) > ipregistry.SizeMax {
		return nil, fmt.Errorf("IANA source %s exceeds %d octets", url, ipregistry.SizeMax)
	}
	return body, nil
}

var registryHTTP = &http.Client{
	Timeout: 30 * time.Second,
	// Both URLs are canonical HTTPS resources. An unexpected redirect is a
	// failed refresh rather than a change of the source's authority.
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
}

func fetchHTTP(ctx context.Context, url string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	response, err := registryHTTP.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		if err := response.Body.Close(); err != nil {
			return nil, fmt.Errorf("IANA HTTP status %d, close body: %w", response.StatusCode, err)
		}
		return nil, fmt.Errorf("IANA HTTP status %d", response.StatusCode)
	}
	// The successful caller MUST close this body; readSource owns that call.
	return response.Body, nil
}
