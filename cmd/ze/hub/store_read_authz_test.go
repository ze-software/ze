package hub

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/authz"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestStoreReadsRefusedReadOnly proves the daemon forms of the two commands
// that answer a stored value's bytes are authorized as more than a read.
//
// VALIDATES: the spec's security row "a secret reaches only an
// edit-authorized caller". `show data cat` and `show config cat` read any
// store key, the HTTPS private key (meta/web/key) and the bcrypt hashes
// under meta/auth and meta/ssh included. Through the dispatcher
// pluginserver.NewServer builds, each path is registered not read-only, a
// caller holding the built-in read-only shape (run section allows, edit
// section denies) is refused with ErrUnauthorized before the handler runs,
// and an admin caller reaches the handler (its answer, a refusal for a key
// the empty store lacks, is not ErrUnauthorized).
// PREVENTS: the verb `show` making a store read a run-section command, so a
// read-only operator over SSH or MCP could read the HTTPS private key.
func TestStoreReadsRefusedReadOnly(t *testing.T) {
	t.Setenv("ze.config.dir", t.TempDir())

	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("daemon dispatcher: %v", err)
	}
	t.Cleanup(server.Stop)
	d := server.Dispatcher()

	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "read-only",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AddProfile(authz.Profile{
		Name: "admin",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Allow},
	})
	store.AssignProfiles("viewer", []string{"read-only"})
	store.AssignProfiles("operator", []string{"admin"})
	d.SetAuthorizer(authz.StoreAuthorizer{Store: store})

	for _, path := range []string{"show data cat", "show config cat"} {
		t.Run(path, func(t *testing.T) {
			cmd := d.Lookup(path)
			if cmd == nil {
				t.Fatalf("%q is not served by the dispatcher", path)
			}
			if cmd.ReadOnly {
				t.Fatalf("%q is ReadOnly on the daemon, want not read-only (registry.Meta.NotReadOnly)", path)
			}

			input := path + " meta/web/key"
			_, viewerErr := d.Dispatch(&pluginserver.CommandContext{Username: "viewer"}, input)
			if !errors.Is(viewerErr, pluginserver.ErrUnauthorized) {
				t.Fatalf("read-only caller: got %v, want ErrUnauthorized", viewerErr)
			}

			_, operatorErr := d.Dispatch(&pluginserver.CommandContext{Username: "operator"}, input)
			if errors.Is(operatorErr, pluginserver.ErrUnauthorized) {
				t.Fatalf("admin caller: refused by authorization: %v", operatorErr)
			}
		})
	}
}
