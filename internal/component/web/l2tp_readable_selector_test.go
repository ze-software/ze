// Design: docs/architecture/web-workbench-pages.md -- exact-original L2TP navigation.
//go:build ze_l2tp

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// MUTATION: discard URL qualifiers before the one existing original capture.
// The response must still contain the useful selected original, not a refusal
// that would also pass by rejecting every qualified navigation request.
func TestL2TPDetailHonorsReadableQualifiersAndOriginalGuard(t *testing.T) {
	birth, err := subscriberid.ParseL2TPBirth("l2tp-00000000000000000000000000001917")
	require.NoError(t, err)
	endpoint := netip.MustParseAddrPort("127.0.0.1:1917")
	for _, input := range []struct {
		name     string
		query    url.Values
		selector l2tp.SessionSelector
	}{
		{name: "tunnel", query: url.Values{"tunnel-id": {"17"}}, selector: l2tp.SessionSelector{SID: 7, TID: 17}},
		{name: "listener", query: url.Values{"listener": {endpoint.String()}}, selector: l2tp.SessionSelector{SID: 7, Listener: endpoint}},
		{name: "guard", query: url.Values{"expected-id": {birth.String()}}, selector: l2tp.SessionSelector{SID: 7, Birth: birth}},
		{name: "complete", query: url.Values{"tunnel-id": {"17"}, "listener": {endpoint.String()}, "expected-id": {birth.String()}}, selector: l2tp.SessionSelector{SID: 7, TID: 17, Listener: endpoint, Birth: birth}},
	} {
		t.Run(input.name, func(t *testing.T) {
			capture := &cqmWebCapture{snapshot: l2tp.SessionSnapshot{Birth: birth, SubscriberID: birth.String(), LocalSID: 7, TunnelLocalTID: 17, Listener: endpoint, Username: "original"}}
			service := &cqmWebService{fakeL2TPService: &fakeL2TPService{}, captured: capture}
			publishFakeL2TP(t, service)
			input.query.Set("format", "json")
			request := httptest.NewRequestWithContext(t.Context(), "GET", "/l2tp/7?"+input.query.Encode(), http.NoBody)
			recorder := httptest.NewRecorder()
			(&l2TPHandlers{}).handleL2TPDetail()(recorder, request)
			require.Equal(t, []l2tp.SessionSelector{input.selector}, service.selectors)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Zero(t, service.unsafeReads)
			var answer map[string]any
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &answer))
			snapshot := answer["Session"].(map[string]any)
			require.Equal(t, "original", snapshot["Username"])
			require.Equal(t, birth.String(), snapshot["subscriber-id"])
		})
	}
}

// MUTATION: silently ignore a malformed, duplicate or absent qualifier value
// and then capture the reusable SID. Every rejection precedes original reads.
func TestL2TPDetailRejectsMalformedQualifiersBeforeCapture(t *testing.T) {
	for _, query := range []string{
		"tunnel-id=", "tunnel-id=0", "tunnel-id=65536", "tunnel-id=17&tunnel-id=18",
		"listener=", "listener=not-an-endpoint", "listener=127.0.0.1%3A0",
		"listener=127.0.0.1%3A1917&listener=127.0.0.1%3A1918",
		"expected-id=", "expected-id=invalid", "expected-id=l2tp-00000000000000000000000000000000",
	} {
		t.Run(query, func(t *testing.T) {
			capture := &cqmWebCapture{}
			service := &cqmWebService{fakeL2TPService: &fakeL2TPService{}, captured: capture}
			publishFakeL2TP(t, service)
			request := httptest.NewRequestWithContext(t.Context(), "GET", "/l2tp/7?format=json&"+query, http.NoBody)
			recorder := httptest.NewRecorder()
			(&l2TPHandlers{}).handleL2TPDetail()(recorder, request)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Empty(t, service.selectors)
			require.Zero(t, capture.snapshotReads)
			require.Zero(t, capture.historyReads)
		})
	}
}

// MUTATION: build either list's link or workbench row key from only SID/TID.
// Equal wire coordinates on different listeners must retain two useful links,
// and Disconnect must reach the exact original's guarded reason form.
func TestL2TPSessionListsKeepDistinctOriginalNavigation(t *testing.T) {
	var sessions []l2tp.SessionSnapshot
	for index, reference := range []string{"l2tp-00000000000000000000000000001917", "l2tp-00000000000000000000000000001918"} {
		birth, err := subscriberid.ParseL2TPBirth(reference)
		require.NoError(t, err)
		endpoint := []netip.AddrPort{netip.MustParseAddrPort("127.0.0.1:1917"), netip.MustParseAddrPort("127.0.0.1:1918")}[index]
		sessions = append(sessions, l2tp.SessionSnapshot{Birth: birth, SubscriberID: reference, Listener: endpoint, LocalSID: 7, TunnelLocalTID: 17, Username: reference})
	}
	service := &fakeL2TPService{snapshot: l2tp.Snapshot{SessionCount: 2, TunnelCount: 1, Tunnels: []l2tp.TunnelSnapshot{{LocalTID: 17, Sessions: sessions}}}}
	publishFakeL2TP(t, service)
	table := buildL2TPSessionsTableData()
	require.Len(t, table.Rows, 2)
	require.NotEqual(t, table.Rows[0].Key, table.Rows[1].Key)
	for index, row := range table.Rows {
		checkL2TPOriginalLink(t, row.URL, sessions[index])
		require.Contains(t, row.Cells, sessions[index].Listener.String())
		require.Len(t, row.Actions, 2)
		require.Empty(t, row.Actions[1].HxPost)
		require.Equal(t, row.URL+"#disconnect", row.Actions[1].URL)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), "GET", "/l2tp?format=json", http.NoBody)
	(&l2TPHandlers{}).handleL2TPList()(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	var answer map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &answer))
	rows := answer["Sessions"].([]any)
	require.Len(t, rows, 2)
	for index, row := range rows {
		fields := row.(map[string]any)
		link, ok := fields["detail-url"].(string)
		require.True(t, ok)
		checkL2TPOriginalLink(t, link, sessions[index])
		require.Equal(t, sessions[index].Listener.String(), fields["listener"])
	}
}

func checkL2TPOriginalLink(t *testing.T, link string, original l2tp.SessionSnapshot) {
	t.Helper()
	parsed, err := url.Parse(link)
	require.NoError(t, err)
	require.Equal(t, "/l2tp/7", parsed.Path)
	require.Equal(t, "17", parsed.Query().Get("tunnel-id"))
	require.Equal(t, original.Listener.String(), parsed.Query().Get("listener"))
	require.Equal(t, original.Birth.String(), parsed.Query().Get("expected-id"))
}
