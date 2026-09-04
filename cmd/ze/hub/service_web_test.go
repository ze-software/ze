//go:build ze_web

package hub

// VALIDATES: the ze_web-gated web factory is registered through the service
// construction registry and can build a running web service from generic deps.
// PREVENTS: reintroducing a direct always-on web.NewWebServer construction path.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
)

func TestServiceRegistry_BuildsWeb(t *testing.T) {
	withCleanRegistry(t)
	registerService("web", buildWebService, nil)

	store, err := storage.NewBlob(filepath.Join(t.TempDir(), "database.zefs"), t.TempDir())
	if err != nil {
		t.Fatalf("blob storage: %v", err)
	}

	services := buildServices(&serviceDeps{
		Store:      store,
		ConfigPath: "config.conf",
		Dispatch: func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, nil), nil
		},
		WebEnabled:  true,
		WebAddrs:    []string{"127.0.0.1:0"},
		InsecureWeb: true,
	})
	if len(services) != 1 {
		t.Fatalf("buildServices built %d services, want 1", len(services))
	}
	if services[0].Name() != "web" {
		t.Fatalf("service name = %q, want web", services[0].Name())
	}
	if len(services[0].Addresses()) != 1 {
		t.Fatalf("web service addresses = %v, want one bound address", services[0].Addresses())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := services[0].Shutdown(ctx); err != nil {
		t.Fatalf("shutdown web service: %v", err)
	}
}
