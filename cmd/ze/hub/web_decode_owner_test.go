//go:build ze_web && ze_bgp

// Design: docs/architecture/web-interface.md -- BGP tools use the registered command producer.
package hub

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebDecodeUsesTheNativeCommandProducer sends a real browser terminal
// request. The native dispatcher must receive it exactly once and return its
// complete decode envelope; a web-owned decoder bypass cannot satisfy this.
func TestWebDecodeUsesTheNativeCommandProducer(t *testing.T) {
	setAPIConfigDir(t, t.TempDir())
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	commands, native := standaloneCommandFixture(t)
	var calls atomic.Int32
	answers := make(chan []byte, 1)
	dispatch := func(ctx context.Context, caller plugin.CallerIdentity, input string) (*plugin.Response, error) {
		response, err := native(ctx, caller, input)
		if input != "show bgp decode "+keepaliveHex {
			return response, err
		}
		calls.Add(1)
		if response == nil {
			return nil, err
		}
		content, marshalErr := json.Marshal(response.Data)
		if marshalErr != nil {
			return nil, marshalErr
		}
		select {
		case answers <- content:
		default:
			// The first payload is sufficient; the exact call count below
			// separately refuses any duplicate dispatch.
		}
		return response, err
	}
	server, broker, editors := startWebServer(store, "router.conf", []string{"127.0.0.1:0"}, true, "", dispatch, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	require.NotNil(t, server)
	service := newWebService(server, broker, editors)
	t.Cleanup(func() { require.NoError(t, shutdownWebOnly(context.Background(), service, commands)) })
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // Per-test self-signed listener.
	t.Cleanup(client.CloseIdleConnections)
	form := url.Values{"command": {"show bgp decode " + keepaliveHex}, "mode": {"operational"}, "submission": {"native-decode"}}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://"+server.Address()+"/cli/terminal", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	require.NoError(t, err)
	body, readErr := io.ReadAll(response.Body)
	require.NoError(t, response.Body.Close())
	require.NoError(t, readErr)
	require.Equal(t, http.StatusOK, response.StatusCode, string(body))
	require.Contains(t, strings.ToLower(string(body)), "keepalive")
	require.Equal(t, int32(1), calls.Load(), "the browser must use the same registered producer as CLI and daemon commands")
	var data map[string]any
	require.NoError(t, json.Unmarshal(<-answers, &data))
	require.Contains(t, data, "bgp-decode", "the browser must retain the native command envelope")
	encoded, err := json.Marshal(data)
	require.NoError(t, err)
	require.Equal(t, "keepalive", decodedNativeMessageType(t, string(encoded)))
}
