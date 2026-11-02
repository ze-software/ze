package hub

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestStoreReadsCannotNameAnotherStore proves the daemon forms of the store
// readers never open a store the caller names.
//
// VALIDATES: `show data cat` and `show data list` read the store their local
// form takes from `--path <store>` (extractPathFlag, config/storage/cli). On
// the daemon route that flag would open any store on the daemon's host with
// the daemon's identity. Through the dispatcher pluginserver.NewServer
// builds, an edit-authorized caller sending `--path <store>` or
// `--path=<store>` is refused before the handler runs, and the answer
// carries no byte of what that store holds.
// PREVENTS: a remote caller reading another store (a backup, a seed blob) by
// naming its path, outside every configuration directory.
func TestStoreReadsCannotNameAnotherStore(t *testing.T) {
	t.Setenv("ze.config.dir", t.TempDir())

	const secret = "ZE-OTHER-STORE-SECRET-77ab"
	other := filepath.Join(t.TempDir(), "other.zefs")
	blob, err := storage.CreateBlobPopulated(other, func(seed storage.Storage) error {
		return seed.WriteKey("meta/web/key", []byte(secret))
	}, false)
	if err != nil {
		t.Fatalf("seed the other store: %v", err)
	}
	if err := blob.Close(); err != nil {
		t.Fatalf("close the other store: %v", err)
	}

	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("daemon dispatcher: %v", err)
	}
	t.Cleanup(server.Stop)
	d := server.Dispatcher()

	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "admin",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Allow},
	})
	store.AssignProfiles("operator", []string{"admin"})
	d.SetAuthorizer(authz.StoreAuthorizer{Store: store})

	for _, input := range []string{
		"show data cat --path " + other + " meta/web/key",
		"show data cat --path=" + other + " meta/web/key",
		"show data cat meta/web/key --path " + other,
		"show data list --path " + other,
	} {
		t.Run(input, func(t *testing.T) {
			resp, dispatchErr := d.Dispatch(&pluginserver.CommandContext{Username: "operator"}, input)
			if dispatchErr == nil {
				t.Fatalf("the daemon answered %q with no refusal: %+v", input, resp)
			}
			if !strings.Contains(dispatchErr.Error(), "unexpected flag") {
				t.Errorf("refusal of %q is not the flag refusal: %v", input, dispatchErr)
			}
			if resp != nil && strings.Contains(resp.Error, secret) {
				t.Errorf("the refusal of %q carries the other store's secret", input)
			}
			if resp != nil && resp.Data != nil {
				t.Errorf("the daemon answered data for %q: %v", input, resp.Data)
			}
		})
	}
}
