// Design: docs/architecture/fleet-config.md -- managed start path tests

//go:build ze_core

package main

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/internal/core/env"
)

const managedClientConfigText = `bus {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-plugin-secret-of-32-characters";
        }
        client edge-01 {
            host 10.0.0.1;
            port 1791;
            secret "edge-01-secret-of-at-least-32-chars";
        }
    }
}
`

const standaloneConfigText = `bus {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-plugin-secret-of-32-characters";
        }
    }
}
`

// setManagedOverride sets one ze.managed override for the test and clears it
// after, so the next test reads the config block alone.
func setManagedOverride(t *testing.T, key, value string) {
	t.Helper()
	if err := env.Set(key, value); err != nil {
		t.Fatalf("set %s: %v", key, err)
	}
	t.Cleanup(func() {
		if err := env.Set(key, ""); err != nil {
			t.Errorf("clear %s: %v", key, err)
		}
	})
}

// TestHubClientEndpointReadsTheClientBlock: the hub connection of a config is
// its hub-level client block. This is the reading the running client applies
// to every config the hub pushes, so it is the one a push changes.
func TestHubClientEndpointReadsTheClientBlock(t *testing.T) {
	got, err := hubClientEndpoint([]byte(managedClientConfigText))
	if err != nil {
		t.Fatalf("hubClientEndpoint: %v", err)
	}
	want := managed.Endpoint{
		Name:   "edge-01",
		Server: "10.0.0.1:1791",
		Token:  "edge-01-secret-of-at-least-32-chars",
	}
	if got != want {
		t.Errorf("endpoint = %+v, want %+v", got, want)
	}
}

// TestHubClientEndpointWithoutClientBlock: a config with no hub client block
// answers managed.ErrNoHubClient, never an empty endpoint a client would dial.
func TestHubClientEndpointWithoutClientBlock(t *testing.T) {
	got, err := hubClientEndpoint([]byte(standaloneConfigText))
	if !errors.Is(err, managed.ErrNoHubClient) {
		t.Fatalf("err = %v (endpoint %+v), want managed.ErrNoHubClient", err, got)
	}
}

// TestHubClientEndpointOverrides: --server, --name and --token (the
// ze.managed keys they set) replace the values of the config block, as
// docs/architecture/fleet-config.md states under "CLI flag overrides". Before,
// they were read on a first boot only, so a client holding a cached config
// ignored them.
//
// Method: each override is set alone and MUST change its own field and no
// other; an override with no client block to replace still answers
// managed.ErrNoHubClient.
func TestHubClientEndpointOverrides(t *testing.T) {
	t.Run("server", func(t *testing.T) {
		setManagedOverride(t, "ze.managed.server", "192.0.2.9:1799")
		got, err := hubClientEndpoint([]byte(managedClientConfigText))
		if err != nil {
			t.Fatalf("hubClientEndpoint: %v", err)
		}
		if got.Server != "192.0.2.9:1799" || got.Name != "edge-01" {
			t.Errorf("endpoint = %+v", got)
		}
	})
	t.Run("name", func(t *testing.T) {
		setManagedOverride(t, "ze.managed.name", "edge-99")
		got, err := hubClientEndpoint([]byte(managedClientConfigText))
		if err != nil {
			t.Fatalf("hubClientEndpoint: %v", err)
		}
		if got.Name != "edge-99" || got.Server != "10.0.0.1:1791" {
			t.Errorf("endpoint = %+v", got)
		}
	})
	t.Run("token", func(t *testing.T) {
		setManagedOverride(t, "ze.managed.token", "another-token-of-at-least-32-chars")
		got, err := hubClientEndpoint([]byte(managedClientConfigText))
		if err != nil {
			t.Fatalf("hubClientEndpoint: %v", err)
		}
		if got.Token != "another-token-of-at-least-32-chars" || got.Name != "edge-01" {
			t.Errorf("endpoint = %+v", got)
		}
	})
	t.Run("no block to override", func(t *testing.T) {
		setManagedOverride(t, "ze.managed.server", "192.0.2.9:1799")
		if _, err := hubClientEndpoint([]byte(standaloneConfigText)); !errors.Is(err, managed.ErrNoHubClient) {
			t.Errorf("err = %v, want managed.ErrNoHubClient", err)
		}
	})
}

// TestConnectTimeoutFlag: --connect-timeout sets ze.managed.connect.timeout,
// the bound on reaching the hub, and refuses a value that is not a duration
// rather than falling back to the default.
func TestConnectTimeoutFlag(t *testing.T) {
	t.Cleanup(func() {
		if err := env.Set("ze.managed.connect.timeout", ""); err != nil {
			t.Errorf("clear ze.managed.connect.timeout: %v", err)
		}
	})

	rest, code := zeParseGlobalFlags([]string{"--connect-timeout", "7s", "start"})
	if code != 0 || len(rest) != 1 || rest[0] != "start" {
		t.Fatalf("rest = %v, code = %d", rest, code)
	}
	if got := env.Get("ze.managed.connect.timeout"); got != "7s" {
		t.Errorf("ze.managed.connect.timeout = %q, want 7s", got)
	}

	if _, bad := zeParseGlobalFlags([]string{"--connect-timeout", "soon", "start"}); bad == 0 {
		t.Error("a value that is not a duration was accepted")
	}
	if _, missing := zeParseGlobalFlags([]string{"--connect-timeout"}); missing == 0 {
		t.Error("the flag with no value was accepted")
	}
}

// TestManagedClientBuiltFromThePointedVersion proves that the daemon's builder
// reads the configuration the device runs, the version the active pointer
// names, and not the plain mirror: the pointed version names hub client
// edge-01, and the mirror, rewritten raw after it, names no hub client.
//
// VALIDATES: the hub client is built from the version that `request fleet
// enable` checked and that the fleet status reports.
// PREVENTS: an enable that passes its check and then builds no client, or a
// client built for another hub than the one the device runs.
func TestManagedClientBuiltFromThePointedVersion(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	const configName = "ze.conf"
	pointed := []byte(managedClientConfigText)
	if writeErr := store.WriteFile(configName, pointed, 0); writeErr != nil {
		t.Fatalf("write config: %v", writeErr)
	}
	if _, _, ensureErr := storage.EnsureActiveVersion(store, configName, pointed, time.Now().Add(-time.Minute)); ensureErr != nil {
		t.Fatalf("EnsureActiveVersion: %v", ensureErr)
	}
	if writeErr := store.WriteFile(configName, []byte(standaloneConfigText), 0); writeErr != nil {
		t.Fatalf("rewrite the mirror: %v", writeErr)
	}
	if active, readErr := storage.ReadActiveConfig(store, configName); readErr != nil || !bytes.Equal(active, pointed) {
		t.Fatalf("the raw write moved the active pointer: %q, %v", active, readErr)
	}

	client, err := extractManagedClientConfig(store, configName)

	if err != nil {
		t.Fatalf("the builder built no hub client from a pointed version that names one: %v", err)
	}
	if client.Name != "edge-01" {
		t.Errorf("hub client %q, want edge-01 from the pointed version", client.Name)
	}
}
