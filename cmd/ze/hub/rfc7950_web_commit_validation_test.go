//go:build ze_web

// Design: docs/architecture/config/yang-config-design.md -- Validation
// Related: editor_adapter.go -- newEditorFactory, which injects the commit validator

package hub

import (
	"bytes"
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zeconfigcmd "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

// webCommitManager builds the editor manager startWebServer builds: a blob store,
// the real editor factory with zeconfigcmd.ValidateContent as its commit
// validator, and EditorManager. It returns the store and the config path.
func webCommitManager(t *testing.T) (*zeweb.EditorManager, storage.Storage, string) {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("blob storage: %v", err)
	}
	configPath := "config.conf"
	if err := store.WriteFile(configPath, []byte("# ze config\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	schema, err := zeconfig.YANGSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	mgr := zeweb.NewEditorManager(store, configPath, schema,
		newEditorFactory(zeconfigcmd.ValidateContent), newEditSessionFactory())
	return mgr, store, configPath
}

// webCommittedConfig reads the config file a commit writes.
func webCommittedConfig(t *testing.T, store storage.Storage, configPath string) []byte {
	t.Helper()
	content, err := store.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return content
}

// VALIDATES: an edit every constraint accepts is committed and written. The
// method is the web editor's own wiring: SetValue, then CommitNow, then the file.
// PREVENTS: a refusal test that passes because the editor commits nothing at all.
//
// RFC 7950 Section 8.3: "For configuration data, there are three windows when
// constraints MUST be enforced".
//
// RFC requirement: RFC7950-8.3-1 positive -- through the web editor factory the daemon installs, setting system/host to "web-valid-host" and committing returns no error and writes "web-valid-host" to the config file.
func TestRFC7950WebCommitWritesAValidEdit(t *testing.T) {
	mgr, store, configPath := webCommitManager(t)

	if err := mgr.SetValue("bob", []string{"system"}, "host", "web-valid-host"); err != nil {
		t.Fatalf("set value: %v", err)
	}
	if _, err := mgr.CommitNow("bob"); err != nil {
		t.Fatalf("commit of a valid edit refused: %v", err)
	}
	if got := webCommittedConfig(t, store, configPath); !bytes.Contains(got, []byte("web-valid-host")) {
		t.Errorf("committed config does not carry the edit:\n%s", got)
	}
}

// VALIDATES: an edit that a ze:validate rule refuses is refused at commit, and
// the config file keeps what it held. The edit passes the YANG type, so only the
// whole-config validation the commit runs can see it.
// PREVENTS: an edit-config window that writes a config the running datastore
// would then load as invalid. The refusal comes from the commit's transition
// validation (EditorManager.validateTransition, internal/component/web), which
// runs the ConfigValidator the SSH editor's commit runs, before anything is staged.
//
// RFC requirement: RFC7950-8.3-1 negative -- through the web editor factory the daemon installs, setting plugin/internal p1 use to "no-such-plugin-here" and committing returns an error naming the value, and the config file is byte-identical to its content before the commit.
// RFC requirement: RFC7950-8.1-2 negative -- the web editor commit of a config the internal-plugin-name ze:validate rule refuses writes nothing, so the config the daemon loads stays the valid one.
func TestRFC7950WebCommitRefusesAConstraintViolation(t *testing.T) {
	mgr, store, configPath := webCommitManager(t)
	before := webCommittedConfig(t, store, configPath)

	if err := mgr.SetValue("bob", []string{"plugin", "internal", "p1"}, "use", "no-such-plugin-here"); err != nil {
		t.Fatalf("set value: %v", err)
	}
	_, err := mgr.CommitNow("bob")
	if err == nil {
		t.Fatal("the commit wrote an edit the ze:validate rule refuses")
	}
	if !strings.Contains(err.Error(), "no-such-plugin-here") {
		t.Errorf("refusal does not name the refused value: %v", err)
	}
	if after := webCommittedConfig(t, store, configPath); !bytes.Equal(after, before) {
		t.Errorf("a refused commit wrote the config:\nbefore %q\nafter  %q", before, after)
	}
}
