package appliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootVendorContainsGopls(t *testing.T) {
	// VALIDATES: The root vendor tree contains the gopls command that dev setup builds offline.
	// PREVENTS: Vendoring a dependency graph that omits the package named by the setup command.
	vendorRoot := filepath.Join("..", "..", "vendor")
	modules, err := os.ReadFile(filepath.Join(vendorRoot, "modules.txt"))
	if err != nil {
		t.Fatalf("read root vendor metadata: %v", err)
	}
	if !strings.Contains(string(modules), "\n# golang.org/x/tools/gopls ") {
		t.Fatal("root vendor metadata does not contain the gopls module")
	}
	goplsCommand := filepath.Join(vendorRoot, "golang.org", "x", "tools", "gopls", "main.go")
	if _, err := os.Stat(goplsCommand); err != nil {
		t.Fatalf("root vendor does not contain the gopls command: %v", err)
	}
}
