// Design: scripts/dev/gokrazy-updater-upstream.patch -- PoC for upstream response body leaks
//
// VALIDATES: local updater copy closes all HTTP response bodies
// PREVENTS: response body leaks that exhaust file descriptors during multi-device push

package updater_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	upstream "github.com/gokrazy/updater"

	local "codeberg.org/thomas-mangin/ze/internal/appliance/updater"
)

// trackingBody wraps an io.ReadCloser and records whether Close was called.
type trackingBody struct {
	io.ReadCloser
	closed *atomic.Int32
}

func (tb *trackingBody) Close() error {
	tb.closed.Add(1)
	return tb.ReadCloser.Close()
}

// trackingTransport wraps http.RoundTripper, replacing every response body
// with a trackingBody so the test can observe whether the caller closes it.
type trackingTransport struct {
	base     http.RoundTripper
	closures *atomic.Int32
	requests *atomic.Int32
}

func (tt *trackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tt.requests.Add(1)
	resp, err := tt.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Body = &trackingBody{
		ReadCloser: resp.Body,
		closed:     tt.closures,
	}
	return resp, nil
}

// fakeGokrazy returns a test server that mimics the gokrazy update endpoints
// well enough for the updater library to succeed.
func fakeGokrazy(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/update/features", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write([]byte("partuuid,updatehash")); err != nil {
			t.Errorf("write features: %v", err)
		}
	})

	mux.HandleFunc("/update/root", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body) //nolint:errcheck // test handler
		if r.Header.Get("X-Gokrazy-Update-Hash") == "crc32" {
			h := crc32.NewIEEE()
			h.Write(body) //nolint:errcheck // hash.Write never errors
			if _, err := w.Write([]byte(hex.EncodeToString(h.Sum(nil)))); err != nil {
				t.Errorf("write hash: %v", err)
			}
		} else {
			h := sha256.Sum256(body)
			if _, err := w.Write([]byte(hex.EncodeToString(h[:]))); err != nil {
				t.Errorf("write hash: %v", err)
			}
		}
	})

	mux.HandleFunc("/update/switch", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/update/testboot", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/reboot", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestUpstreamLeaksResponseBodies(t *testing.T) {
	srv := fakeGokrazy(t)

	closures := &atomic.Int32{}
	requests := &atomic.Int32{}
	client := &http.Client{
		Transport: &trackingTransport{
			base:     srv.Client().Transport,
			closures: closures,
			requests: requests,
		},
	}

	ctx := context.Background()
	target, err := upstream.NewTarget(ctx, srv.URL+"/", client)
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}

	if err := target.StreamTo(ctx, "root", strings.NewReader("hello")); err != nil {
		t.Fatalf("StreamTo: %v", err)
	}

	if err := target.Switch(ctx); err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if err := target.Testboot(ctx); err != nil {
		t.Fatalf("Testboot: %v", err)
	}

	if err := target.Reboot(ctx); err != nil {
		t.Fatalf("Reboot: %v", err)
	}

	req := requests.Load()
	cls := closures.Load()
	leaked := req - cls

	t.Logf("upstream: %d requests, %d bodies closed, %d leaked", req, cls, leaked)
	if leaked == 0 {
		t.Fatal("expected upstream to leak response bodies, but all were closed (has upstream been fixed?)")
	}
	t.Logf("CONFIRMED: upstream leaks %d response bodies", leaked)
}

func TestLocalClosesAllResponseBodies(t *testing.T) {
	srv := fakeGokrazy(t)

	closures := &atomic.Int32{}
	requests := &atomic.Int32{}
	client := &http.Client{
		Transport: &trackingTransport{
			base:     srv.Client().Transport,
			closures: closures,
			requests: requests,
		},
	}

	ctx := context.Background()
	target, err := local.NewTarget(ctx, srv.URL+"/", client)
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}

	if err := target.StreamTo(ctx, "root", strings.NewReader("hello")); err != nil {
		t.Fatalf("StreamTo: %v", err)
	}

	if err := target.Switch(ctx); err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if err := target.Testboot(ctx); err != nil {
		t.Fatalf("Testboot: %v", err)
	}

	if err := target.Reboot(ctx); err != nil {
		t.Fatalf("Reboot: %v", err)
	}

	req := requests.Load()
	cls := closures.Load()
	leaked := req - cls

	t.Logf("local: %d requests, %d bodies closed, %d leaked", req, cls, leaked)
	if leaked != 0 {
		t.Fatalf("local copy leaked %d response bodies", leaked)
	}
}
