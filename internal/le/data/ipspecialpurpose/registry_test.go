// Design: docs/architecture/wire/attributes.md -- deliberate IANA snapshot refresh.
package dataipspecialpurpose

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/ipregistry"
)

// TestWriteRefreshesCanonicalSnapshots observes both fixed sources and the exact
// bytes embedded by the runtime, rather than a separately maintained CIDR table.
func TestWriteRefreshesCanonicalSnapshots(t *testing.T) {
	root := refreshTree(t)
	asked := 0
	report, err := Write(context.Background(), root, func(_ context.Context, url string) (io.ReadCloser, error) {
		if !strings.HasPrefix(url, "https://www.iana.org/assignments/") {
			t.Fatalf("noncanonical source %s", url)
		}
		asked++
		return io.NopCloser(bytes.NewReader(refreshSnapshot(t, url))), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if asked != 2 || len(report.Files) != 2 {
		t.Fatalf("sources=%d report=%+v", asked, report)
	}
	for _, row := range report.Files {
		got, err := os.ReadFile(filepath.Join(root, row.File))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, refreshSnapshot(t, row.Source)) {
			t.Fatalf("snapshot %s changed during refresh", row.File)
		}
		if row.Entries == 0 || row.Updated == "" {
			t.Fatalf("missing provenance %+v", row)
		}
	}
}

// TestWriteFailurePreservesBothSnapshots requires the second source's fetch,
// parse and body-close errors to leave even the first existing file untouched.
func TestWriteFailurePreservesBothSnapshots(t *testing.T) {
	for _, failure := range []string{"fetch", "parse", "read", "close", "oversized"} {
		t.Run(failure, func(t *testing.T) {
			root := refreshTree(t)
			original := []byte("previous shipped snapshot")
			for _, source := range sources {
				if err := os.WriteFile(filepath.Join(root, source.file()), original, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			asked := 0
			_, err := Write(context.Background(), root, func(_ context.Context, url string) (io.ReadCloser, error) {
				asked++
				if asked == 1 {
					return io.NopCloser(bytes.NewReader(refreshSnapshot(t, url))), nil
				}
				switch failure {
				case "fetch":
					return nil, errors.New("source unavailable")
				case "parse":
					return io.NopCloser(strings.NewReader("<registry/>")), nil
				case "read":
					return refreshFailureReader{}, nil
				case "close":
					return refreshCloseFailure{bytes.NewReader(refreshSnapshot(t, url))}, nil
				case "oversized":
					return io.NopCloser(strings.NewReader(strings.Repeat("x", ipregistry.SizeMax+1))), nil
				default:
					t.Fatalf("unknown fixture %s", failure)
					return nil, errors.New("unknown fixture")
				}
			})
			if err == nil {
				t.Fatal("failed registry refresh succeeded")
			}
			for _, source := range sources {
				got, err := os.ReadFile(filepath.Join(root, source.file()))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, original) {
					t.Fatalf("failed refresh changed %s", source.file())
				}
			}
		})
	}
}

func refreshTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal/core/ipregistry"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func refreshSnapshot(t *testing.T, url string) []byte {
	t.Helper()
	for _, source := range sources {
		if url == source.url() {
			data, err := os.ReadFile(filepath.Join("../../../..", source.file()))
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
	}
	t.Fatalf("unexpected source %s", url)
	return nil
}

type refreshFailureReader struct{}

func (refreshFailureReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (refreshFailureReader) Close() error             { return nil }

type refreshCloseFailure struct{ *bytes.Reader }

func (refreshCloseFailure) Close() error { return errors.New("close failed") }
