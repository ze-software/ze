package lg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// manyRouteDispatch returns a dispatcher whose "show bgp rib best" reply carries
// n routes, so pagination behavior can be exercised against a realistic list.
func manyRouteDispatch(n int) CommandDispatcher {
	routes := make([]map[string]any, n)
	for i := range routes {
		routes[i] = map[string]any{
			"prefix":           fmt.Sprintf("10.%d.%d.0/24", i/256, i%256),
			"next-hop":         "10.0.0.1",
			"origin":           "igp",
			"as-path":          []any{float64(65001)},
			"local-preference": float64(100),
		}
	}
	payload, _ := json.Marshal(map[string]any{"routes": routes})
	body := string(payload)
	return func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(body)), nil
	}
}

// startPagServer starts a looking-glass server backed by the given dispatcher.
func startPagServer(t *testing.T, dispatch CommandDispatcher) (string, *http.Client) {
	t.Helper()
	srv, err := NewLGServer(LGConfig{ListenAddrs: []string{"127.0.0.1:0"}, Dispatch: dispatch})
	if err != nil {
		t.Fatalf("NewLGServer: %v", err)
	}
	go func() { _ = srv.ListenAndServe(context.Background()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.WaitReady(ctx); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	return "http://" + srv.Address(), &http.Client{Timeout: 10 * time.Second}
}

// getRoutes fetches url and decodes the birdwatcher routes envelope, closing the body.
func getRoutes(t *testing.T, client *http.Client, url string) map[string]any {
	t.Helper()
	resp := doGet(t, client, url)
	defer resp.Body.Close() //nolint:errcheck // test cleanup
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal %q: %v", string(body), err)
	}
	return env
}

// getStatus fetches url and returns the HTTP status, closing the body.
func getStatus(t *testing.T, client *http.Client, url string) int {
	t.Helper()
	resp := doGet(t, client, url)
	defer resp.Body.Close() //nolint:errcheck // test cleanup
	return resp.StatusCode
}

func routesLen(t *testing.T, env map[string]any) int {
	t.Helper()
	routes, ok := env["routes"].([]any)
	if !ok {
		t.Fatalf("routes not an array: %v", env["routes"])
	}
	return len(routes)
}

func TestRoutesTablePagination(t *testing.T) {
	// VALIDATES: AC-11 -- limit/offset slice route-list responses; default = full.
	// PREVENTS: pagination params being silently ignored (regression to full RIB).
	const total = 50
	base, client := startPagServer(t, manyRouteDispatch(total))
	tableURL := base + "/api/looking-glass/routes/table/ipv4%2Funicast"

	// Default (no params): full result, today's behavior preserved.
	full := getRoutes(t, client, tableURL)
	if got := routesLen(t, full); got != total {
		t.Fatalf("default response: got %d routes, want %d", got, total)
	}
	if _, hasPag := full["pagination"]; hasPag {
		t.Fatalf("default response must not carry a pagination object (byte-identical requirement)")
	}

	// limit=10 caps the page.
	page := getRoutes(t, client, tableURL+"?limit=10")
	if got := routesLen(t, page); got != 10 {
		t.Fatalf("limit=10: got %d routes, want 10", got)
	}
	if got := getNum(page, "routes_count"); int(got) != 10 {
		t.Fatalf("limit=10: routes_count = %v, want 10", page["routes_count"])
	}

	// offset skips into the list; limit+offset windows it.
	windowed := getRoutes(t, client, tableURL+"?limit=5&offset=20")
	if got := routesLen(t, windowed); got != 5 {
		t.Fatalf("limit=5&offset=20: got %d routes, want 5", got)
	}
	first, _ := windowed["routes"].([]any)[0].(map[string]any)
	if got := getStr(first, "network"); got != "10.0.20.0/24" {
		t.Fatalf("limit=5&offset=20: first network = %q, want 10.0.20.0/24 (stable order)", got)
	}

	// offset beyond the end -> empty list, 200.
	if got := getStatus(t, client, tableURL+"?offset=1000"); got != http.StatusOK {
		t.Fatalf("offset beyond end: status %d, want 200", got)
	}
	if got := routesLen(t, getRoutes(t, client, tableURL+"?offset=1000")); got != 0 {
		t.Fatalf("offset beyond end: got %d routes, want 0", got)
	}

	// limit=0 is explicit "unlimited".
	unl := getRoutes(t, client, tableURL+"?limit=0")
	if got := routesLen(t, unl); got != total {
		t.Fatalf("limit=0: got %d routes, want %d (unlimited)", got, total)
	}
}

func TestPaginationParamValidation(t *testing.T) {
	// VALIDATES: AC-11 -- invalid limit/offset params yield HTTP 400.
	// PREVENTS: non-numeric/negative/oversized params being accepted.
	base, client := startPagServer(t, manyRouteDispatch(5))
	tableURL := base + "/api/looking-glass/routes/table/ipv4%2Funicast"

	bad := []string{
		"?limit=abc",
		"?limit=-1",
		"?offset=-1",
		"?offset=xyz",
		"?limit=100001", // above the 100000 ceiling
	}
	for _, q := range bad {
		if got := getStatus(t, client, tableURL+q); got != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", q, got)
		}
	}

	// last valid boundary: limit=100000 accepted.
	if got := getStatus(t, client, tableURL+"?limit=100000"); got != http.StatusOK {
		t.Errorf("limit=100000: status %d, want 200", got)
	}
}

func TestPaginateRoutes(t *testing.T) {
	// VALIDATES: AC-11 boundary table -- paginateRoutes windows [offset, offset+limit).
	// PREVENTS: off-by-one/panic at the offset>=len and limit>len boundaries.
	makeEnv := func(n int) map[string]any {
		routes := make([]any, n)
		for i := range routes {
			routes[i] = map[string]any{"network": fmt.Sprintf("10.0.0.%d/32", i)}
		}
		return map[string]any{"routes": routes, "routes_count": n}
	}

	cases := []struct {
		name          string
		total         int
		limit, offset int
		wantLen       int
	}{
		{"limit below total", 10, 3, 0, 3},
		{"limit above total", 10, 100, 0, 10},
		{"limit zero = unlimited", 10, 0, 0, 10},
		{"offset windows", 10, 4, 2, 4},
		{"offset at end", 10, 5, 10, 0},
		{"offset beyond end", 10, 5, 1000, 0},
		{"empty list", 0, 5, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := makeEnv(tc.total)
			paginateRoutes(env, tc.limit, tc.offset)
			got, _ := env["routes"].([]any)
			if len(got) != tc.wantLen {
				t.Fatalf("page len = %d, want %d", len(got), tc.wantLen)
			}
			if int(getNum(env, "routes_count")) != tc.wantLen {
				t.Fatalf("routes_count = %v, want %d", env["routes_count"], tc.wantLen)
			}
			pag, _ := env["pagination"].(map[string]any)
			if pag == nil {
				t.Fatalf("pagination object missing")
			}
			if int(getNum(pag, "total_results")) != tc.total {
				t.Fatalf("total_results = %v, want %d (pre-slice total)", pag["total_results"], tc.total)
			}
		})
	}
}
