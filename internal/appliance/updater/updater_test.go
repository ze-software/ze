package updater

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestTarget(t *testing.T, h http.Handler) *Target {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	tgt, err := NewTarget(t.Context(), srv.URL+"/", srv.Client())
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	return tgt
}

func TestNewTargetParsesTextFeatures(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/update/features", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("partuuid,updatehash")) //nolint:errcheck // test
	})

	tgt := newTestTarget(t, mux)
	if !tgt.Supports(ProtocolFeaturePARTUUID) {
		t.Error("expected partuuid support")
	}
	if !tgt.Supports(ProtocolFeatureUpdateHash) {
		t.Error("expected updatehash support")
	}
}

func TestNewTargetNoFeatures(t *testing.T) {
	// 404 on /update/features means a legacy target with no advertised features.
	tgt := newTestTarget(t, http.NotFoundHandler())
	if tgt.Supports(ProtocolFeatureUpdateHash) {
		t.Error("legacy target should not advertise updatehash")
	}
}

func TestStreamToSHA256(t *testing.T) {
	payload := []byte("disk-image-contents")

	mux := http.NewServeMux()
	mux.HandleFunc("/update/features", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("")) //nolint:errcheck // no updatehash -> SHA-256 path
	})
	mux.HandleFunc("/update/root", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gokrazy-Update-Hash") != "" {
			t.Errorf("SHA-256 path must not set the crc32 header")
		}
		body, _ := io.ReadAll(r.Body)
		sum := sha256.Sum256(body)
		w.Write([]byte(hex.EncodeToString(sum[:]))) //nolint:errcheck // test
	})

	tgt := newTestTarget(t, mux)
	if err := tgt.StreamTo(t.Context(), "root", bytes.NewReader(payload)); err != nil {
		t.Fatalf("StreamTo (sha256): %v", err)
	}
}

func TestStreamToCRC32(t *testing.T) {
	payload := []byte("disk-image-contents")

	mux := http.NewServeMux()
	mux.HandleFunc("/update/features", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("updatehash")) //nolint:errcheck // crc32 path
	})
	mux.HandleFunc("/update/root", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gokrazy-Update-Hash") != "crc32" {
			t.Errorf("crc32 path must set X-Gokrazy-Update-Hash: crc32")
		}
		body, _ := io.ReadAll(r.Body)
		h := crc32.NewIEEE()
		h.Write(body)
		w.Write([]byte(hex.EncodeToString(h.Sum(nil)))) //nolint:errcheck // test
	})

	tgt := newTestTarget(t, mux)
	if err := tgt.StreamTo(t.Context(), "root", bytes.NewReader(payload)); err != nil {
		t.Fatalf("StreamTo (crc32): %v", err)
	}
}

func TestStreamToChecksumMismatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/update/features", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("")) //nolint:errcheck // test
	})
	mux.HandleFunc("/update/root", func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)                         //nolint:errcheck // test
		w.Write([]byte(hex.EncodeToString([]byte("nope")))) //nolint:errcheck // wrong hash
	})

	tgt := newTestTarget(t, mux)
	if err := tgt.StreamTo(t.Context(), "root", bytes.NewReader([]byte("data"))); err == nil {
		t.Fatal("StreamTo accepted a mismatched checksum; expected error")
	}
}
