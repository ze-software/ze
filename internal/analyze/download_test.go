package analyze

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFileRecompressesGzip(t *testing.T) {
	const payload = "ripe fixture\nwith two records\n"
	var encoded bytes.Buffer
	writer := gzip.NewWriter(&encoded)
	if _, err := io.WriteString(writer, payload); err != nil {
		t.Fatalf("write source gzip: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close source gzip: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Write(encoded.Bytes()) //nolint:errcheck // test server records transport failures
	}))
	t.Cleanup(server.Close)

	output := filepath.Join(t.TempDir(), "ripe-updates.gz")
	if err := downloadFile(server.URL+"/updates.gz", output); err != nil {
		t.Fatalf("downloadFile: %v", err)
	}
	if got := readGzip(t, output); got != payload {
		t.Fatalf("decoded output = %q, want %q", got, payload)
	}
}

func TestDownloadFileConvertsBzip2ToGzip(t *testing.T) {
	const payload = "routeviews fixture\n"
	encoded, err := base64.StdEncoding.DecodeString("QlpoOTFBWSZTWazh5/sAAAVRgAAQQAADIJ/AIAAijTGjTQgGgBgKEmoN4ytLmaLuSKcKEhWcPP9g")
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Write(encoded) //nolint:errcheck // test server records transport failures
	}))
	t.Cleanup(server.Close)

	output := filepath.Join(t.TempDir(), "routeviews-rib.gz")
	if err := downloadFile(server.URL+"/rib.bz2", output); err != nil {
		t.Fatalf("downloadFile: %v", err)
	}
	if got := readGzip(t, output); got != payload {
		t.Fatalf("decoded output = %q, want %q", got, payload)
	}
}

func TestDownloadFileRemovesCorruptTransfer(t *testing.T) {
	var encoded bytes.Buffer
	writer := gzip.NewWriter(&encoded)
	if _, err := io.WriteString(writer, "truncated fixture"); err != nil {
		t.Fatalf("write source gzip: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close source gzip: %v", err)
	}
	truncated := encoded.Bytes()[:encoded.Len()-8]
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Write(truncated) //nolint:errcheck // test server records transport failures
	}))
	t.Cleanup(server.Close)

	output := filepath.Join(t.TempDir(), "corrupt.gz")
	if err := downloadFile(server.URL+"/updates.gz", output); err == nil {
		t.Fatal("downloadFile accepted corrupt gzip")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("partial output stat error = %v, want not-exist", err)
	}
}

func TestDownloadFileRejectsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "unavailable", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	output := filepath.Join(t.TempDir(), "missing.gz")
	if err := downloadFile(server.URL+"/updates.gz", output); err == nil {
		t.Fatal("downloadFile accepted HTTP 404")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output stat error = %v, want not-exist", err)
	}
}

func readGzip(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	defer file.Close() //nolint:errcheck // read error is asserted below
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("new gzip reader: %v", err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close gzip reader: %v", err)
	}
	return string(decoded)
}
