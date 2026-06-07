package mrt

import (
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFile_HTTP_PlainMRT(t *testing.T) {
	// Build a minimal MRT record: common header (12 bytes) + empty body
	record := make([]byte, CommonHeaderLen)
	WriteCommonHeader(record, 0, 1000, TypeBGP4MP, BGP4MPMessageAS4, 0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(record) //nolint:errcheck // test
	}))
	defer srv.Close()

	var count int
	err := ReadFile(srv.URL+"/test.mrt", &Handler{
		OnHeader: func(_ Header, _ uint32, _ []byte) error {
			count++
			return nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestReadFile_HTTP_GzipMRT(t *testing.T) {
	record := make([]byte, CommonHeaderLen)
	WriteCommonHeader(record, 0, 2000, TypeBGP4MP, BGP4MPMessageAS4, 0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		gz := gzip.NewWriter(w)
		gz.Write(record) //nolint:errcheck // test
		gz.Close()       //nolint:errcheck // test
	}))
	defer srv.Close()

	var count int
	err := ReadFile(srv.URL+"/test.mrt.gz", &Handler{
		OnHeader: func(_ Header, _ uint32, _ []byte) error {
			count++
			return nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestReadFile_HTTP_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := ReadFile(srv.URL+"/missing.mrt", &Handler{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")
}
