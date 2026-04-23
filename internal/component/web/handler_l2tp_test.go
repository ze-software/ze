package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
)

type fakeL2TPService struct {
	snapshot     l2tp.Snapshot
	session      l2tp.SessionSnapshot
	sessionOK    bool
	events       []l2tp.ObserverEvent
	samples      []l2tp.CQMBucket
	samplesLogin string
	teardownErr  error
	teardownSID  uint16
	disconnects  []fakeDisconnect
}

type fakeDisconnect struct {
	SID    uint16
	Actor  string
	Reason string
	Cause  uint32
}

func (f *fakeL2TPService) Snapshot() l2tp.Snapshot { return f.snapshot }
func (f *fakeL2TPService) LookupTunnel(_ uint16) (l2tp.TunnelSnapshot, bool) {
	return l2tp.TunnelSnapshot{}, false
}
func (f *fakeL2TPService) LookupSession(_ uint16) (l2tp.SessionSnapshot, bool) {
	return f.session, f.sessionOK
}
func (f *fakeL2TPService) Listeners() []l2tp.ListenerSnapshot   { return nil }
func (f *fakeL2TPService) EffectiveConfig() l2tp.ConfigSnapshot { return l2tp.ConfigSnapshot{} }
func (f *fakeL2TPService) TeardownTunnel(_ uint16) error        { return nil }
func (f *fakeL2TPService) TeardownSession(sid uint16) error {
	f.teardownSID = sid
	return f.teardownErr
}
func (f *fakeL2TPService) TeardownAllTunnels() int  { return 0 }
func (f *fakeL2TPService) TeardownAllSessions() int { return 0 }
func (f *fakeL2TPService) SessionEvents(_ uint16) []l2tp.ObserverEvent {
	return f.events
}
func (f *fakeL2TPService) LoginSamples(login string) []l2tp.CQMBucket {
	if login == f.samplesLogin {
		return f.samples
	}
	return nil
}
func (f *fakeL2TPService) SessionSummaries() []l2tp.SessionSummary    { return nil }
func (f *fakeL2TPService) LoginSummaries() []l2tp.LoginSummary        { return nil }
func (f *fakeL2TPService) EchoState(_ string) *l2tp.LoginEchoState    { return nil }
func (f *fakeL2TPService) ReliableStats(_ uint16) *l2tp.ReliableStats { return nil }
func (f *fakeL2TPService) RecordDisconnect(sid uint16, actor, reason string, cause uint32) {
	f.disconnects = append(f.disconnects, fakeDisconnect{SID: sid, Actor: actor, Reason: reason, Cause: cause})
}

func publishFakeL2TP(t *testing.T, svc l2tp.Service) {
	t.Helper()
	l2tp.PublishService(svc)
	t.Cleanup(func() { l2tp.PublishService(nil) })
}

func TestHandleL2TPList_RendersSessions(t *testing.T) {
	publishFakeL2TP(t, &fakeL2TPService{
		snapshot: l2tp.Snapshot{
			TunnelCount:  1,
			SessionCount: 1,
			Tunnels: []l2tp.TunnelSnapshot{{
				LocalTID: 1,
				PeerAddr: netip.MustParseAddrPort("10.0.0.1:1701"),
				Sessions: []l2tp.SessionSnapshot{{
					LocalSID:     42,
					Username:     "testuser",
					State:        "established",
					AssignedAddr: netip.MustParseAddr("192.168.1.100"),
				}},
			}},
		},
	})

	renderer, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	h := &L2TPHandlers{Renderer: renderer}
	req := httptest.NewRequest("GET", "/l2tp?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleL2TPList()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	var data map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	tc, ok := data["TunnelCount"].(float64)
	if !ok || int(tc) != 1 {
		t.Errorf("TunnelCount=%v, want 1", data["TunnelCount"])
	}
}

func TestHandleL2TPDetail_RendersTimeline(t *testing.T) {
	publishFakeL2TP(t, &fakeL2TPService{
		session: l2tp.SessionSnapshot{
			LocalSID: 42,
			Username: "testuser",
			State:    "established",
		},
		sessionOK: true,
		events: []l2tp.ObserverEvent{
			{Timestamp: time.Now(), Type: l2tp.ObserverEventSessionUp, SessionID: 42},
		},
	})

	renderer, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	h := &L2TPHandlers{Renderer: renderer}
	req := httptest.NewRequest("GET", "/l2tp/42?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleL2TPDetail()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	var data map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	events, ok := data["Events"].([]any)
	if !ok || len(events) != 1 {
		t.Errorf("Events=%v, want 1 element", data["Events"])
	}
}

func TestHandleL2TPSamplesJSON_ColumnarShape(t *testing.T) {
	now := time.Now()
	publishFakeL2TP(t, &fakeL2TPService{
		samplesLogin: "testuser",
		samples: []l2tp.CQMBucket{
			{Start: now, State: l2tp.BucketStateEstablished, EchoCount: 10, MinRTT: time.Millisecond, MaxRTT: 5 * time.Millisecond, SumRTT: 30 * time.Millisecond},
			{Start: now.Add(100 * time.Second), State: l2tp.BucketStateEstablished, EchoCount: 8, MinRTT: 2 * time.Millisecond, MaxRTT: 4 * time.Millisecond, SumRTT: 24 * time.Millisecond},
		},
	})

	req := httptest.NewRequest("GET", "/l2tp/testuser/samples", http.NoBody)
	rec := httptest.NewRecorder()
	HandleL2TPSamplesJSON()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	var data map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"timestamps", "minRTT", "avgRTT", "maxRTT", "states"} {
		arr, ok := data[key].([]any)
		if !ok {
			t.Errorf("missing or wrong type for %s", key)
			continue
		}
		if len(arr) != 2 {
			t.Errorf("%s has %d elements, want 2", key, len(arr))
		}
	}
}

func TestHandleL2TPSamplesJSON_FromToFilter(t *testing.T) {
	base := time.Unix(1000000, 0)
	publishFakeL2TP(t, &fakeL2TPService{
		samplesLogin: "testuser",
		samples: []l2tp.CQMBucket{
			{Start: base},
			{Start: base.Add(100 * time.Second)},
			{Start: base.Add(200 * time.Second)},
		},
	})

	req := httptest.NewRequest("GET", "/l2tp/testuser/samples?from=1000100&to=1000200", http.NoBody)
	rec := httptest.NewRecorder()
	HandleL2TPSamplesJSON()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	var data map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	ts, ok := data["timestamps"].([]any)
	if !ok || len(ts) != 2 {
		t.Errorf("filtered count=%d, want 2", len(ts))
	}
}

func TestHandleL2TPSamplesCSV_Format(t *testing.T) {
	now := time.Now()
	publishFakeL2TP(t, &fakeL2TPService{
		samplesLogin: "testuser",
		samples: []l2tp.CQMBucket{
			{Start: now, State: l2tp.BucketStateEstablished, EchoCount: 5, MinRTT: time.Millisecond, MaxRTT: 3 * time.Millisecond, SumRTT: 10 * time.Millisecond},
		},
	})

	req := httptest.NewRequest("GET", "/l2tp/testuser/samples.csv", http.NoBody)
	rec := httptest.NewRecorder()
	HandleL2TPSamplesCSV()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 2 {
		t.Errorf("CSV lines=%d, want 2 (header + 1 row)", len(lines))
	}
	if !strings.HasPrefix(lines[0], "timestamp,") {
		t.Errorf("header=%q, want to start with 'timestamp,'", lines[0])
	}
}

func TestHandleL2TPDisconnect_DispatchesCommand(t *testing.T) {
	publishFakeL2TP(t, &fakeL2TPService{})

	var dispatched string
	h := &L2TPHandlers{
		Dispatch: func(cmd string) (string, error) {
			dispatched = cmd
			return `{"status":"ok"}`, nil
		},
	}

	form := url.Values{"reason": {"maintenance"}, "cause": {"6"}}
	req := httptest.NewRequest("POST", "/l2tp/42/disconnect?format=json", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleL2TPDisconnect()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	want := `clear l2tp session teardown 42 actor web reason "maintenance" cause 6`
	if dispatched != want {
		t.Errorf("dispatched=%q, want %q", dispatched, want)
	}
}

func TestHandleL2TPDisconnect_ReasonRequired(t *testing.T) {
	h := &L2TPHandlers{
		Dispatch: func(cmd string) (string, error) { return "", nil },
	}
	form := url.Values{"reason": {""}}
	req := httptest.NewRequest("POST", "/l2tp/42/disconnect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleL2TPDisconnect()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec.Code)
	}
}

func TestHandleL2TPDisconnect_ReasonTooLong(t *testing.T) {
	h := &L2TPHandlers{
		Dispatch: func(cmd string) (string, error) { return "", nil },
	}
	form := url.Values{"reason": {strings.Repeat("x", 257)}}
	req := httptest.NewRequest("POST", "/l2tp/42/disconnect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleL2TPDisconnect()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec.Code)
	}
}

func TestHandleL2TPDisconnect_DispatchFailureReturns500JSON(t *testing.T) {
	publishFakeL2TP(t, &fakeL2TPService{})

	h := &L2TPHandlers{
		Dispatch: func(cmd string) (string, error) {
			return "", fmt.Errorf("session not found")
		},
	}

	form := url.Values{"reason": {"maintenance"}}
	req := httptest.NewRequest("POST", "/l2tp/42/disconnect?format=json", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleL2TPDisconnect()(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", rec.Code)
	}
	var data map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	if data["error"] != true {
		t.Errorf("error=%v, want true", data["error"])
	}
}

func TestExtractLogin_Valid(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/l2tp/testuser/samples", "testuser"},
		{"/l2tp/user@realm/samples", "user@realm"},
		{"/l2tp/user.name/samples", "user.name"},
		{"/l2tp/user-name/samples", "user-name"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, http.NoBody)
		got := extractLogin(req)
		if got != tt.want {
			t.Errorf("extractLogin(%q)=%q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestExtractLogin_Rejects(t *testing.T) {
	// Paths safe for httptest.NewRequest (valid URLs).
	httpRejects := []string{
		"/l2tp//samples",
		"/l2tp/../samples",
	}
	for _, path := range httpRejects {
		req := httptest.NewRequest("GET", path, http.NoBody)
		got := extractLogin(req)
		if got != "" {
			t.Errorf("extractLogin(%q)=%q, want empty", path, got)
		}
	}
	// Paths with control/special chars: construct request with raw URL
	// to avoid httptest.NewRequest panicking on invalid URLs.
	rawRejects := []string{
		"/l2tp/user\x00name/samples",
		"/l2tp/user\"name/samples",
		"/l2tp/user;name/samples",
		"/l2tp/user\\name/samples",
		"/l2tp/user\nname/samples",
		"/l2tp/user\rname/samples",
	}
	for _, path := range rawRejects {
		req := &http.Request{URL: &url.URL{Path: path}}
		got := extractLogin(req)
		if got != "" {
			t.Errorf("extractLogin(%q)=%q, want empty", path, got)
		}
	}
}

func TestHandleL2TPDisconnect_ReasonQuoted(t *testing.T) {
	publishFakeL2TP(t, &fakeL2TPService{})

	var dispatched string
	h := &L2TPHandlers{
		Dispatch: func(cmd string) (string, error) {
			dispatched = cmd
			return `{"status":"ok"}`, nil
		},
	}

	form := url.Values{"reason": {"maintenance cause 999"}}
	req := httptest.NewRequest("POST", "/l2tp/42/disconnect?format=json", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleL2TPDisconnect()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	want := `clear l2tp session teardown 42 actor web reason "maintenance cause 999"`
	if dispatched != want {
		t.Errorf("dispatched=%q, want %q", dispatched, want)
	}
}
