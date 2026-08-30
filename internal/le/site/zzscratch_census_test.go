package site

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestScratchDocsDest is a scratch census probe. It is deleted before the phase
// ends.
func TestScratchDocsDest(t *testing.T) {
	pages, err := docsProducerPages()
	if err != nil {
		t.Fatalf("pages: %v", err)
	}
	var out strings.Builder
	for _, page := range pages {
		out.WriteString("CLAIM /" + strings.TrimSuffix(page.Dest, pageIndexFile) + "\n")
	}
	t.Logf("DOCSDEST count=%d\n%s", len(pages), out.String())
}

// TestScratchProducerCensus runs every registered producer over the real
// checkout and reports what each one claims. It is deleted before the phase
// ends.
func TestScratchProducerCensus(t *testing.T) {
	root := repositoryRoot(t)
	output := t.TempDir()
	published := filepath.Join(root, "tmp", "session",
		"2026-08-29-36ab4fc7-fd4f-4e52-892c-be2eaf79ddcd", "scratch", "p10", "pub")
	copyScratchTree(t, filepath.Join(published, "data"), filepath.Join(output, "data"))
	paths := Paths{Repository: root, Source: filepath.Join(root, "website"), Output: output}
	var out strings.Builder
	for _, producer := range registeredProducers {
		routes, err := producer.Render(paths)
		if err != nil {
			out.WriteString("PRODUCER " + producer.Name + " FAILED " + err.Error() + "\n")
			continue
		}
		out.WriteString("PRODUCER " + producer.Name + " routes=" + itoa(len(routes)) + "\n")
		for _, route := range routes {
			out.WriteString("CENSUS " + producer.Name + " " + route + "\n")
		}
	}
	t.Logf("CENSUS-BEGIN\n%sCENSUS-END", out.String())
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

func copyScratchTree(t *testing.T, source, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := exec.Command("cp", "-R", source, target).Run(); err != nil {
		t.Fatalf("copy %s: %v", source, err)
	}
}
