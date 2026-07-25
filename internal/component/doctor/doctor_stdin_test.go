// VALIDATES: loadDoctorConfig (renamed from loadConfigData to end the collision
// with the now-removed config/cli helper, AC-18) reads its config from stdin when
// the path is "-".
// PREVENTS: `ze show doctor -` regressing to a raw os.ReadFile("-") failure.
package doctor

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
)

func TestDoctorLoadRenamed(t *testing.T) {
	const cfg = "router-id 1.2.3.4\nlocal-as 65000\n"
	restore := cliio.SwapStreams(strings.NewReader(cfg), &bytes.Buffer{})
	defer restore()

	data, name, err := loadDoctorConfig(storage.NewFilesystem(), "-")
	if err != nil {
		t.Fatalf("loadDoctorConfig(-): %v", err)
	}
	if string(data) != cfg {
		t.Fatalf("stdin config = %q, want %q", data, cfg)
	}
	if name != "-" {
		t.Fatalf("config name = %q, want %q", name, "-")
	}
}
