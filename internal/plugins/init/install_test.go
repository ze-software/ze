package init

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/pkg/zefs"
)

func TestNativeBootstrapPublishesLoopbackSSHWithoutInterfaceTakeover(t *testing.T) {
	dir := t.TempDir()
	if code := runInstallInit(strings.NewReader("admin\nsecret123\n\n\nedge\n"), nil, dir); code != 0 {
		t.Fatalf("bootstrap=%d", code)
	}
	store, err := storage.OpenReadOnly(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // Test cleanup.
	name, err := resolve.DefaultConfig(store)
	if err != nil {
		t.Fatal(err)
	}
	data, err := store.ReadKey(zefs.KeyFileActive.Key(name))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{"ssh", "enabled true", "127.0.0.1", "2222"} {
		if !strings.Contains(text, required) {
			t.Fatalf("config missing %q: %s", required, text)
		}
	}
	if strings.Contains(text, "interface {") {
		t.Fatalf("installer took over interfaces: %s", text)
	}
}
