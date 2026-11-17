//go:build ze_web

// Design: docs/architecture/web-interface.md -- service cleanup retains borrowed storage.
package hub

import (
	"context"
	"crypto/tls"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/pkg/zefs"
)

type webHistoryStore struct {
	storage.Storage
	entered chan struct{}
	release chan struct{}
	closed  chan struct{}
	first   sync.Once
	atClose map[string]string
}

func (s *webHistoryStore) WriteFile(key string, data []byte, mode fs.FileMode) error {
	if strings.HasPrefix(key, zefs.KeyHistory.Prefix()) {
		s.first.Do(func() { close(s.entered); <-s.release })
	}
	return s.Storage.WriteFile(key, data, mode)
}

func (s *webHistoryStore) Close() error {
	s.atClose = make(map[string]string)
	for _, mode := range []string{"config", "operational"} {
		key := zefs.KeyHistory.Prefix() + "alice/" + mode
		data, err := s.Storage.ReadFile(key)
		if err == nil {
			s.atClose[mode] = string(data)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	err := s.Storage.Close()
	close(s.closed)
	return err
}

// TestWebServiceOverdueHistoryRetainsStoreUntilEveryAcceptedModeDrains uses
// the real manager/history/coalescer and real underlying store. The caller
// immediately closes its owned store after actual service shutdown returns.
// Even after the diagnostic allowance expires, that close cannot race the held
// write or the other accepted mode that has not acquired a store guard yet.
func TestWebServiceOverdueHistoryRetainsStoreUntilEveryAcceptedModeDrains(t *testing.T) {
	cert, key, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)
	underlying, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	synctest.Test(t, func(t *testing.T) {
		store := &webHistoryStore{Storage: underlying, entered: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
		manager := zeweb.NewEditorManager(store, "config.conf", nil, nil, nil)
		server, err := zeweb.NewWebServer(zeweb.WebConfig{ListenAddrs: []string{"127.0.0.1:0"}, CertPEM: cert, KeyPEM: key})
		require.NoError(t, err)
		service := newWebService(server, nil, manager)
		service.drainLimit = time.Second
		require.NoError(t, manager.RecordHistory("alice", cli.ModeConfig, nil, "show config-owned", "one"))
		<-store.entered
		require.NoError(t, manager.RecordHistory("alice", cli.ModeOperational, nil, "show daemon-owned", "two"))
		ended := make(chan error, 1)
		go func() {
			shutdownError := service.Shutdown(context.Background())
			ended <- errors.Join(shutdownError, store.Close())
		}()
		synctest.Wait()
		<-time.After(2 * time.Second)
		synctest.Wait()
		select {
		case <-store.closed:
			t.Fatal("the overdue service released a store still borrowed by accepted history")
		default:
		}
		close(store.release)
		require.ErrorIs(t, <-ended, errWebHistoryDrainOverdue)
		require.Equal(t, map[string]string{"config": "show config-owned", "operational": "show daemon-owned"}, store.atClose)
	})
}

// TestWebServiceShutdownSealsCLIHistoryBeforeOwnedStoreRelease follows a live
// authenticated HTTP submission, the service shutdown and its caller's close.
func TestWebServiceShutdownSealsCLIHistoryBeforeOwnedStoreRelease(t *testing.T) {
	setAPIConfigDir(t, t.TempDir())
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	server, broker, manager := startWebServer(store, "config.conf", []string{"127.0.0.1:0"}, true, "", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	require.NotNil(t, server)
	service := newWebService(server, broker, manager)
	t.Cleanup(func() { require.NoError(t, service.Shutdown(context.Background())) })
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // Per-test self-signed server.
	t.Cleanup(client.CloseIdleConnections)
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://"+server.Address()+"/cli/terminal", strings.NewReader(url.Values{"command": {"show"}, "mode": {"operational"}, "submission": {"owned-one"}}.Encode()))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, response.Body.Close())
	require.NoError(t, service.Shutdown(context.Background()))
	data, err := store.ReadFile(zefs.KeyHistory.Prefix() + "insecure/operational")
	require.NoError(t, err)
	require.Equal(t, "show", string(data))
	_, err = manager.HistorySnapshot("insecure", cli.ModeOperational)
	require.ErrorContains(t, err, "closing")
}

// TestWebStartupFailureReleasesItsBorrowedManager drives the actual late bind
// failure after manager construction. No listener can serve before all bind,
// so this path owns no accepted history; none of its handles may escape.
func TestWebStartupFailureReleasesItsBorrowedManager(t *testing.T) {
	setAPIConfigDir(t, t.TempDir())
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	var listen net.ListenConfig
	occupied, err := listen.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer occupied.Close() //nolint:errcheck // Test-owned occupied port.
	server, broker, manager := startWebServer(store, "config.conf", []string{"127.0.0.1:0", occupied.Addr().String()}, true, "", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	require.Nil(t, server)
	require.Nil(t, broker)
	require.Nil(t, manager)
	require.NoError(t, store.WriteFile("meta/history/after-failed-start/config", []byte("show caller-still-owns-store"), 0o600))
}
