package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNativeImplementationFixture pins the native tables and render decisions
// that the retired cross-runtime oracle compared.
func TestNativeImplementationFixture(t *testing.T) {
	const want = "e475724aaa523d26f47a93cb66da9b4d09c729d46d2a37604d85c81a46c70b9e"
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list rules sources: %v", err)
	}
	digest := sha256.New()
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		digest.Write([]byte(filepath.Base(path)))
		digest.Write([]byte{0})
		digest.Write(content)
		digest.Write([]byte{0})
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != want {
		t.Fatalf("native rules fixture digest = %s, want %s; review the behavior change and update the owned fixture", got, want)
	}
}
